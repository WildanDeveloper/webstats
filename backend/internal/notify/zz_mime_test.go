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

func TestHtmlToTextPreservesLinks(t *testing.T) {
	out := htmlToText(`<p>See <a href="https://example.com/invite/abc">the invite</a> now.</p>`)
	if !strings.Contains(out, "the invite") {
		t.Fatalf("link text lost: %q", out)
	}
	if !strings.Contains(out, "https://example.com/invite/abc") {
		t.Fatalf("link href lost: %q", out)
	}
}

func TestHtmlToTextEntitiesDecoded(t *testing.T) {
	out := htmlToText(`<p>a &amp; b &lt;c&gt; &quot;d&quot; &#39;e&#39;</p>`)
	if strings.Contains(out, "&amp;") || strings.Contains(out, "&lt;") {
		t.Fatalf("entities not decoded: %q", out)
	}
	if !strings.Contains(out, `a & b <c> "d" 'e'`) {
		t.Fatalf("decoded text wrong: %q", out)
	}
	if strings.Contains(out, "&#39;") {
		t.Fatalf("numeric entity not decoded: %q", out)
	}
}

func TestHtmlToTextKeepsLineStructure(t *testing.T) {
	out := htmlToText(`<div>Line1</div><div>Line2</div><p>Para</p><br/>End`)
	if got := out; got != "Line1\n\nLine2\n\nPara\n\nEnd" {
		t.Fatalf("line structure wrong: %q", got)
	}
}

func TestHtmlToTextSkipsScriptStyleAndAttributeTags(t *testing.T) {
	out := htmlToText(`<script>evil()</script><style>.x{}</style><a href="https://e.com/x?a=1&b=2" title="y">link</a>`)
	if strings.Contains(out, "evil") || strings.Contains(out, ".x{") {
		t.Fatalf("script/style content survived: %q", out)
	}
	if !strings.Contains(out, "https://e.com/x?a=1") {
		t.Fatalf("href with quotes lost: %q", out)
	}
}
