package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/notify"
)

// sslExpiryLoop warns site owners before their TLS certificates expire.
// Rules use event 'cert_expiry'; params.days (default 14) is how many days
// before expiry the reminder fires. Once fired, a rule stays quiet for 20
// hours, so a long-running expiry keeps reminding roughly once a day.
func sslExpiryLoop(ctx context.Context, pool *pgxpool.Pool) {
	sslExpiryRun(ctx, pool)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sslExpiryRun(ctx, pool)
		}
	}
}

func sslExpiryRun(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.user_id, r.site_id, r.channel, r.target, r.provider_id, r.params, s.domain, s.name
		FROM notif_rules r
		JOIN sites s ON s.id = r.site_id AND s.domain <> ''
		WHERE r.event = 'cert_expiry' AND r.enabled
		  AND (r.last_sent_at IS NULL OR r.last_sent_at < now() - interval '20 hours')`)
	if err != nil {
		return
	}
	type rule struct {
		id, userID, siteID, channel, target, domain, name string
		providerID                                        *string
		params                                            map[string]any
	}
	var rules []rule
	for rows.Next() {
		var r rule
		if rows.Scan(&r.id, &r.userID, &r.siteID, &r.channel, &r.target, &r.providerID, &r.params, &r.domain, &r.name) == nil {
			rules = append(rules, r)
		}
	}
	rows.Close()
	for _, r := range rules {
		notAfter, err := certNotAfter(r.domain)
		if err != nil {
			// Unreachable/broken TLS is the uptime monitor's job; here we
			// only care about certificates we can read.
			continue
		}
		days := int(time.Until(notAfter).Hours() / 24)
		threshold := 14
		if v, ok := r.params["days"].(float64); ok && v >= 1 && v <= 365 {
			threshold = int(v)
		}
		if days > threshold {
			continue
		}
		payload := notify.AlertPayload{
			Event: "cert_expiry", SiteID: r.siteID, SiteName: r.name, Domain: r.domain,
			Status: fmt.Sprintf("%d days left", days), Time: time.Now().Format(time.RFC3339),
		}
		_, _ = deliverRule(ctx, pool, r.userID, r.channel, r.target, r.providerID, r.params, payload, false)
		_, _ = pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, r.id)
	}
}

// certNotAfter fetches the leaf certificate's expiry for a public host.
// Private/loopback targets are refused, same as the other SSRF guards.
func certNotAfter(rawHost string) (time.Time, error) {
	host := strings.TrimSpace(rawHost)
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	host = strings.TrimSuffix(host, "/")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return time.Time{}, errors.New("empty host")
	}
	if isBlockedIPHost(host) {
		return time.Time{}, errors.New("host resolves to a private address")
	}
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(context.Background(), "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return time.Time{}, err
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
	defer tlsConn.Close()
	if err := tlsConn.Handshake(); err != nil {
		return time.Time{}, err
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, errors.New("no peer certificate")
	}
	return certs[0].NotAfter, nil
}
