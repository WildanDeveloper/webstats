package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ingest"
)

func main() {
	cfg := config.Load()
	if cfg.RedisURL == "" {
		log.Fatal("REDIS_URL is required for the standalone worker")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opt)

	// Records arriving from Redis were already geo-enriched by the ingest
	// service, so the worker uses empty resolvers.
	buf := ingest.NewBuffer(cfg, pool, &geo.Resolver{}, &geo.ASNResolver{})

	log.Printf("worker started, draining %s (batch=%d)", ingest.RedisList, cfg.BatchSize)

	for {

		res, err := rdb.BRPop(ctx, 5*time.Second, ingest.RedisList).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			log.Printf("brpop error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		raw := res[1]
		var rec ingest.Record
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			log.Printf("bad record: %v", err)
			continue
		}
		if err := buf.FlushNow(ctx, []ingest.Record{rec}); err != nil {
			log.Printf("flush failed: %v", err)
			time.Sleep(time.Second)
		}
	}
}
