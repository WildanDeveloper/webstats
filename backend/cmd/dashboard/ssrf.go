package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// isPrivateIP reports whether ip points into a network scope that must never
// be probed by server-side fetches (loopback, RFC1918, link-local, CGNAT,
// link-local multicast and the unspecified address).
func isPrivateIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// dialControl runs after DNS resolution and before the socket connects, so it
// closes the DNS-rebinding window: whatever IP is actually dialed is checked.
func dialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("ssrf: refusing non-IP dial target %q", host)
	}
	if isPrivateIP(ip) {
		return fmt.Errorf("ssrf: refusing connection to private address %q", host)
	}
	return nil
}

// ssrfSafeClient returns an HTTP client that refuses to connect to private
// or loopback addresses regardless of the URL host resolution.
func ssrfSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: dialControl}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
		},
	}
}

// isBlockedIPHost resolves host and reports whether any of its addresses is
// private/loopback. Used for TLS-only probes that bypass http.Transport.
func isBlockedIPHost(host string) bool {
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		// Fail closed: unresolvable hosts are not probed either.
		return true
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}
	return false
}

// validateMonitorURL normalizes a monitor URL and rejects anything that is
// not a plain public http(s) endpoint. The returned URL is what gets stored;
// runtime dials are additionally guarded by ssrfSafeClient.
func validateMonitorURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err = url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("invalid URL")
		}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("URL host required")
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if isPrivateIP(ip) {
			return "", fmt.Errorf("URL must not point at a private address")
		}
	} else if isBlockedIPHost(host) {
		return "", fmt.Errorf("URL must not point at a private address")
	}
	return u.String(), nil
}

var ssrfMonitorClient = ssrfSafeClient(5 * time.Second)
var ssrfUptimeClient = ssrfSafeClient(10 * time.Second)
