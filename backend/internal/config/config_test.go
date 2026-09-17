package config

import (
	"testing"
	"time"
)

func TestProxyList(t *testing.T) {
	got := proxyList("127.0.0.1, ::1 ,,10.0.0.0/8,")
	if len(got) != 3 || got[0] != "127.0.0.1" || got[1] != "::1" || got[2] != "10.0.0.0/8" {
		t.Fatalf("proxyList = %v", got)
	}
	if len(proxyList("")) != 0 {
		t.Fatal("empty list should be empty")
	}
}

func TestLoadDefaultsTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	c := Load()
	if len(c.TrustedProxies) == 0 {
		t.Fatal("TrustedProxies default is empty")
	}
	if c.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("first trusted proxy = %q", c.TrustedProxies[0])
	}
}

func TestLoadClampsSizes(t *testing.T) {
	t.Setenv("BUFFER_SIZE", "0")
	t.Setenv("BATCH_SIZE", "0")
	c := Load()
	if c.BufferSize != 4096 {
		t.Fatalf("BufferSize = %d, want 4096", c.BufferSize)
	}
	if c.BatchSize != 100 {
		t.Fatalf("BatchSize = %d, want 100", c.BatchSize)
	}
	t.Setenv("BUFFER_SIZE", "1")
	t.Setenv("BATCH_SIZE", "1")
	c = Load()
	if c.BufferSize != 1 || c.BatchSize != 1 {
		t.Fatalf("clamps changed valid values: %d %d", c.BufferSize, c.BatchSize)
	}
}

// Verifies B38: nonpositive FLUSH_EVERY is rejected at startup instead of
// panicking later in NewTicker.
func TestLoadRejectsNonPositiveFlushEvery(t *testing.T) {
	for _, v := range []string{"0", "-5s", "bogus"} {
		t.Setenv("FLUSH_EVERY", v)
		if _, err := LoadChecked(); err == nil {
			t.Fatalf("FLUSH_EVERY=%q accepted", v)
		}
	}
	t.Setenv("FLUSH_EVERY", "250ms")
	c, err := LoadChecked()
	if err != nil || c.FlushEvery != 250*time.Millisecond {
		t.Fatalf("valid FLUSH_EVERY rejected: %v %v", c, err)
	}
}

// Verifies B42: env integers are parsed with bounds checks; overflowing
// values are rejected instead of wrapping to a small/negative number.
func TestEnvIntRejectsOverflow(t *testing.T) {
	if got := envInt("X", 7); got != 7 {
		t.Fatalf("empty env: got %d", got)
	}
	t.Setenv("BUFFER_SIZE", "99999999999999999999999")
	if _, err := LoadChecked(); err == nil {
		t.Fatal("overflowing BUFFER_SIZE accepted")
	}
	t.Setenv("BUFFER_SIZE", "5000")
	c, err := LoadChecked()
	if err != nil || c.BufferSize != 5000 {
		t.Fatalf("valid value rejected: %v %v", c, err)
	}
	t.Setenv("BUFFER_SIZE", "-3")
	if _, err := LoadChecked(); err == nil {
		t.Fatal("negative BUFFER_SIZE accepted")
	}
	t.Setenv("BUFFER_SIZE", "not-a-number")
	if _, err := LoadChecked(); err == nil {
		t.Fatal("non-numeric BUFFER_SIZE accepted")
	}
	// envInt fallback path stays lenient for other callers.
	t.Setenv("BUFFER_SIZE", "")
	t.Setenv("RATE_LIMIT_PER_MIN", "12x")
	if got := envInt("RATE_LIMIT_PER_MIN", 5); got != 5 {
		t.Fatalf("envInt fallback = %d, want 5", got)
	}
}
