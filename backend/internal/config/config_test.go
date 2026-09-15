package config

import "testing"

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
