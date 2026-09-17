package egress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

var ErrDenied = errors.New("egress destination denied")

type Policy struct {
	allowed []netip.Prefix
}

func ParseAllowlist(raw string) (*Policy, error) {
	p := &Policy{}
	if strings.TrimSpace(raw) == "" {
		return p, nil
	}
	for _, entry := range strings.Split(raw, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil || prefix.Addr().Is4In6() {
			return nil, errors.New("EGRESS_ALLOW_CIDRS must contain valid IPv4 or IPv6 CIDRs")
		}
		p.allowed = append(p.allowed, prefix.Masked())
	}
	return p, nil
}

func FromEnv() (*Policy, error) {
	return ParseAllowlist(os.Getenv("EGRESS_ALLOW_CIDRS"))
}

func IsNonPublic(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return !addr.IsGlobalUnicast()
}

func (p *Policy) Allows(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if p != nil {
		for _, prefix := range p.allowed {
			if prefix.Contains(addr) {
				return true
			}
		}
	}
	return !IsNonPublic(ip)
}

func (p *Policy) Control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || !p.Allows(net.ParseIP(host)) {
		return ErrDenied
	}
	return nil
}

func (p *Policy) Dialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: p.Control}
}

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	p, err := FromEnv()
	if err != nil {
		return nil, err
	}
	return p.Dialer(15*time.Second).DialContext(ctx, network, address)
}

func (p *Policy) ValidateHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if p.Allows(ip) {
			return nil
		}
		return ErrDenied
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return ErrDenied
	}
	for _, ip := range ips {
		if ip.Zone != "" || !p.Allows(ip.IP) {
			return ErrDenied
		}
	}
	return nil
}

func ValidateURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return errors.New("invalid egress URL")
	}
	return nil
}

func origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

func CheckRedirect(req *http.Request, via []*http.Request) error {
	if err := ValidateURL(req.URL); err != nil {
		return err
	}
	if len(via) >= 10 {
		return errors.New("egress redirect limit exceeded")
	}
	if len(via) == 0 || origin(req.URL) != origin(via[0].URL) {
		return errors.New("cross-origin egress redirect denied")
	}
	return nil
}

type transport struct{ base *http.Transport }

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := ValidateURL(req.URL); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func (t *transport) CloseIdleConnections() { t.base.CloseIdleConnections() }

func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: CheckRedirect,
		Transport: &transport{base: &http.Transport{
			DialContext:           DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          100,
		}},
	}
}
