package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
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
// and caches the result back into the pageviews rows for that IP.
func enrichVisitorIP(ctx context.Context, db *pgxpool.Pool, ip string) ipapiResult {
	var r ipapiResult
	if ip == "" {
		return r
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
	if r.Status != "success" || r.Lat == 0 && r.Lon == 0 {
		return r
	}
	_, err = db.Exec(ctx, `UPDATE pageviews
		SET isp = CASE WHEN isp = '' THEN $2 ELSE isp END,
		    country = $7,
		    region = $3, city = $4, lat = $5, lon = $6
		WHERE ip = $1`, ip, r.Isp, r.RegionName, r.City, r.Lat, r.Lon, r.CountryCode)
	if err != nil {
		return r
	}
	return r
}