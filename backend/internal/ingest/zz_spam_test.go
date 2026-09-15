package ingest

import "testing"

func TestIsReferrerSpam(t *testing.T) {
	// The embedded matomo list contains these host fragments.
	if !isReferrerSpam("buttons-for-website.com") {
		t.Fatal("spam host not detected")
	}
	if !isReferrerSpam("www.buttons-for-website.com") {
		t.Fatal("subdomain of spam host not detected")
	}
	if isReferrerSpam("NOT-SPAM.example.com") {
		t.Fatal("clean host flagged as spam")
	}
	if isReferrerSpam("") {
		t.Fatal("empty host flagged")
	}
	if isReferrerSpam("example.com.") {
		t.Fatal("trailing-dot host flagged")
	}
}

func TestNormalizeDropsSpamReferrer(t *testing.T) {
	// Direct call of the normalization path: a spammy referrer must be
	// blanked before it reaches the record (and therefore the DB).
	// Normalize hits the settings cache (DB), so reuse the shared test buffer.
	b := testBuffer(t)
	rec := b.Normalize(map[string]any{
		"site_id":  "s",
		"path":     "/",
		"referrer": "https://www.buttons-for-website.com/whatever",
	}, "8.8.8.8")
	if rec.Referrer != "" {
		t.Fatalf("spam referrer survived: %q", rec.Referrer)
	}
	rec2 := b.Normalize(map[string]any{
		"site_id":  "s",
		"path":     "/",
		"referrer": "https://news.ycombinator.com/item?id=1",
	}, "8.8.8.8")
	if rec2.Referrer == "" {
		t.Fatal("clean referrer was blanked")
	}
}
