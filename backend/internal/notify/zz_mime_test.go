package notify

import (
	"strings"
	"testing"
)

func TestMimeHeaderASCIIUnchanged(t *testing.T) {
	if got := mimeHeader("Site is down: example.com"); got != "Site is down: example.com" {
		t.Fatalf("ascii header changed: %q", got)
	}
}

func TestMimeHeaderNonASCIIEncoded(t *testing.T) {
	got := mimeHeader("Résumé site down")
	if got == "Résumé site down" {
		t.Fatal("non-ASCII header not encoded")
	}
	if !strings.HasPrefix(got, "=?utf-8?q?") && !strings.HasPrefix(got, "=?UTF-8?q?") {
		t.Fatalf("not RFC 2047 encoded: %q", got)
	}
}

func TestMimeHeaderCRLFStripped(t *testing.T) {
	got := mimeHeader("a\r\nb")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("CRLF survived: %q", got)
	}
}

func TestEmailTemplateEscapesPayload(t *testing.T) {
	html := Email(AlertPayload{
		Event:    "site_down",
		SiteName: `<script>alert("x")</script>`,
		Domain:   `"><img src=x onerror=alert(1)>`,
		Status:   "<b>down</b>",
		Time:     "2026-01-01T00:00:00Z",
	})
	if strings.Contains(html, "<script>") {
		t.Fatal("script tag survived in email body")
	}
	if strings.Contains(html, "<img src=x") {
		t.Fatal("img tag survived in email body")
	}
	if strings.Contains(html, "<b>down</b>") {
		t.Fatal("status HTML survived")
	}
}
