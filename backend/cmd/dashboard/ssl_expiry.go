package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
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
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, r := range rules {
		sem <- struct{}{}
		wg.Add(1)
		go func(r rule) {
			defer wg.Done()
			defer func() { <-sem }()
			conn, err := dialSiteTLS(ctx, r.domain)
			if err != nil {
				return
			}
			notAfter := conn.ConnectionState().PeerCertificates[0].NotAfter
			conn.Close()
			days := int(time.Until(notAfter).Hours() / 24)
			threshold := 14
			if v, ok := r.params["days"].(float64); ok && v >= 1 && v <= 365 {
				threshold = int(v)
			}
			if days > threshold {
				return
			}
			payload := notify.AlertPayload{
				Event: "cert_expiry", SiteID: r.siteID, SiteName: r.name, Domain: r.domain,
				Status: fmt.Sprintf("%d days left", days), Time: time.Now().Format(time.RFC3339),
			}
			_, _ = deliverRule(ctx, pool, r.userID, r.channel, r.target, r.providerID, r.params, payload, false)
			_, _ = pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, r.id)
		}(r)
	}
	wg.Wait()
}

// certNotAfter fetches the leaf certificate's expiry for a public host.
// Private/loopback targets are refused, same as the other SSRF guards.
func certNotAfter(rawHost string) (time.Time, error) {
	conn, err := dialSiteTLS(context.Background(), rawHost)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates[0].NotAfter, nil
}

func dialSiteTLS(parent context.Context, rawHost string) (*tls.Conn, error) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	rawHost = strings.TrimSpace(rawHost)
	if !strings.Contains(rawHost, "://") {
		rawHost = "https://" + rawHost
	}
	u, err := url.Parse(rawHost)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid TLS host")
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, Control: dialControl}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	deadline, _ := ctx.Deadline()
	if err := tlsConn.SetDeadline(deadline); err != nil {
		tlsConn.Close()
		return nil, err
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, err
	}
	if len(tlsConn.ConnectionState().PeerCertificates) == 0 {
		tlsConn.Close()
		return nil, errors.New("no peer certificate")
	}
	return tlsConn, nil
}
