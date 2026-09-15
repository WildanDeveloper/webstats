package ingest

import (
	_ "embed"
	"strings"
)

//go:embed referrerspam.txt
var referrerSpamList string

var referrerSpam = buildSpamList(referrerSpamList)

// buildSpamList normalizes the embedded blocklist: lowercase host
// fragments, one per line, comments (#) and blanks ignored.
func buildSpamList(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// isReferrerSpam reports whether the referrer host matches the embedded
// spam blocklist. Matching is on host fragments so subdomains of spam
// domains are caught too.
func isReferrerSpam(host string) bool {
	if host == "" {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, frag := range referrerSpam {
		if host == frag || strings.HasSuffix(host, "."+frag) {
			return true
		}
	}
	return false
}
