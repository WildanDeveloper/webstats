package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ipapiResult struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Isp         string  `json:"isp"`
	Org         string  `json:"org"`
	As          string  `json:"as"`
	Timezone    string  `json:"timezone"`
}

var geoHTTP = &http.Client{Timeout: 8 * time.Second}

// ip-api.com free tier allows ~45 requests/minute, so results are cached
// per IP. Failures are cached too (shorter) so a dead upstream doesn't get
// hammered by every dashboard view.
type geoCacheEntry struct {
	result ipapiResult
	exp    time.Time
}

const (
	geoCachePosTTL = time.Hour
	geoCacheNegTTL = 5 * time.Minute
	geoCacheMax    = 4096
)

var (
	geoCacheMu sync.Mutex
	geoCache   = map[string]geoCacheEntry{}
)

func geoCacheGet(ip string) (ipapiResult, bool) {
	geoCacheMu.Lock()
	defer geoCacheMu.Unlock()
	e, ok := geoCache[ip]
	if !ok {
		return ipapiResult{}, false
	}
	if time.Now().After(e.exp) {
		delete(geoCache, ip)
		return ipapiResult{}, false
	}
	return e.result, true
}

func geoCachePut(ip string, r ipapiResult, success bool) {
	ttl := geoCacheNegTTL
	if success {
		ttl = geoCachePosTTL
	}
	geoCacheMu.Lock()
	defer geoCacheMu.Unlock()
	if len(geoCache) >= geoCacheMax {
		now := time.Now()
		for k, v := range geoCache {
			if now.After(v.exp) || len(geoCache) >= geoCacheMax {
				delete(geoCache, k)
			}
		}
	}
	geoCache[ip] = geoCacheEntry{result: r, exp: time.Now().Add(ttl)}
}

var proxyProviders = []string{
	"cloudflare", "google", "amazon", "microsoft", "oracle", "akamai", "fastly", "incapsula", "imperva", "stackpath",
}

func isProxyProvider(org string) (bool, string) {
	if org == "" {
		return false, ""
	}
	lower := strings.ToLower(org)
	for _, p := range proxyProviders {
		if strings.Contains(lower, p) {
			return true, p
		}
	}
	return false, ""
}

// enrichVisitorIP resolves city/region/coords/ISP for an IP via ip-api.com
// and caches the result back into the pageviews rows for that visitor.
func enrichVisitorIP(ctx context.Context, db *pgxpool.Pool, ip, siteID string) ipapiResult {
	var r ipapiResult
	// Only real IPs can be enriched. With IP hashing enabled the identifier
	// is a salted hash and must never be sent to a third party.
	if net.ParseIP(ip) == nil {
		return r
	}
	if cached, ok := geoCacheGet(ip); ok {
		return cached
	}
	u := "http://ip-api.com/json/" + url.PathEscape(ip) +
		"?fields=status,country,countryCode,regionName,city,lat,lon,isp,org,as,timezone,query"
	resp, err := geoHTTP.Get(u)
	if err != nil {
		return r
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return r
	}
	success := r.Status == "success"
	geoCachePut(ip, r, success)
	if !success || r.Lat == 0 && r.Lon == 0 {
		return r
	}
	_, err = db.Exec(ctx, `UPDATE pageviews
		SET isp = CASE WHEN isp = '' THEN $2 ELSE isp END,
		    country = $7,
		    region = $3, city = $4, lat = $5, lon = $6
		WHERE ip = $1 AND site_id = $8`, ip, r.Isp, r.RegionName, r.City, r.Lat, r.Lon, r.CountryCode, siteID)
	if err != nil {
		return r
	}
	return r
}