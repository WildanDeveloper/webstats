package config

import (
	"os"
	"time"
)

type Config struct {
	Port         string
	Bind         string
	DBURL        string
	JWTSecret    string
	RedisURL     string
	GeoCSV       string
	IPHashSalt   string
	BufferSize   int
	FlushEvery   time.Duration
	BatchSize    int
	AllowOrigins string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	return &Config{
		Port:         getenv("PORT", "8080"),
		Bind:         os.Getenv("BIND"),
		DBURL:        getenv("DATABASE_URL", "postgres://webstats:webstats@localhost:5432/webstats"),
		JWTSecret:    getenv("JWT_SECRET", "webstats-dev-secret-change-me"),
		RedisURL:     os.Getenv("REDIS_URL"),
		GeoCSV:       os.Getenv("GEO_CSV"),
		IPHashSalt:   getenv("IP_HASH_SALT", "webstats-salt"),
		BufferSize:   envInt("BUFFER_SIZE", 4096),
		FlushEvery:   envDur("FLUSH_EVERY", 5*time.Second),
		BatchSize:    envInt("BATCH_SIZE", 100),
		AllowOrigins: getenv("ALLOW_ORIGINS", "*"),
	}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
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
