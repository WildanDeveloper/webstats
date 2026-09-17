package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port         string
	Bind         string
	DBURL        string
	JWTSecret    string
	RedisURL     string
	GeoCSV       string
	ASNCSV       string
	IPHashSalt   string
	BufferSize   int
	FlushEvery   time.Duration
	BatchSize    int
	AllowOrigins string
	// PublicURL is the browser-facing base of the dashboard (frontend),
	// used to build absolute links inside emails (invites, unsubscribe).
	PublicURL string
	// APIPublicURL is the browser-facing base of this API, used in emails.
	APIPublicURL string
	// TrustedProxies is the allowlist of proxies whose X-Forwarded-For
	// header may be trusted when resolving client IPs.
	TrustedProxies []string
	// RateLimitPerMin caps ingest requests per site per minute (token
	// bucket). 0 disables the limiter.
	RateLimitPerMin int
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	c, err := LoadChecked()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	return c
}

func LoadChecked() (*Config, error) {
	c := &Config{
		Port:            getenv("PORT", "8086"),
		Bind:            os.Getenv("BIND"),
		DBURL:           getenv("DATABASE_URL", "postgres://webstats:webstats@localhost:5432/webstats"),
		JWTSecret:       getenv("JWT_SECRET", "webstats-dev-secret-change-me"),
		RedisURL:        os.Getenv("REDIS_URL"),
		GeoCSV:          os.Getenv("GEO_CSV"),
		ASNCSV:          os.Getenv("GEO_ASN_CSV"),
		IPHashSalt:      getenv("IP_HASH_SALT", "webstats-salt"),
		BufferSize:      envInt("BUFFER_SIZE", 4096),
		FlushEvery:      envDur("FLUSH_EVERY", 5*time.Second),
		BatchSize:       envInt("BATCH_SIZE", 100),
		AllowOrigins:    getenv("ALLOW_ORIGINS", "*"),
		PublicURL:       getenv("APP_PUBLIC_URL", "http://localhost:3000"),
		APIPublicURL:    getenv("API_PUBLIC_URL", "http://localhost:8086"),
		TrustedProxies:  proxyList(getenv("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")),
		RateLimitPerMin: envInt("RATE_LIMIT_PER_MIN", 600),
	}
	if c.BufferSize <= 0 {
		c.BufferSize = 4096
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if v := os.Getenv("FLUSH_EVERY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("FLUSH_EVERY must be a positive duration")
		}
		c.FlushEvery = d
	}
	for _, bound := range []struct {
		key string
		max int
	}{
		{"BUFFER_SIZE", 1000000},
		{"BATCH_SIZE", 10000},
		{"RATE_LIMIT_PER_MIN", 10000000},
	} {
		if v := os.Getenv(bound.key); v != "" {
			n, err := strconv.ParseUint(v, 10, strconv.IntSize)
			if err != nil || n > uint64(bound.max) {
				return nil, fmt.Errorf("%s must be an integer between 0 and %d", bound.key, bound.max)
			}
		}
	}
	return c, nil
}

func proxyList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, strconv.IntSize-1)
	if err != nil {
		return def
	}
	return int(n)
}

func envDur(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return def
}
