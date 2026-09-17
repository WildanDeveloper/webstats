package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

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
	defer rdb.Close()

	// Records arriving from Redis were already geo-enriched by the ingest
	// service, so the worker uses empty resolvers.
	workerCfg := *cfg
	workerCfg.RedisURL = ""
	buf := ingest.NewBuffer(&workerCfg, pool, &geo.Resolver{}, &geo.ASNResolver{})

	log.Printf("worker started, draining %s (batch=%d)", ingest.RedisList, cfg.BatchSize)

	// Reliable queue: records are moved to a processing list before decode,
	// acknowledged (removed) only after a successful commit, and stranded
	// entries from a previous crash are recovered on startup.
	for {
		n, err := rdb.RPopLPush(ctx, ingest.RedisProcessing, ingest.RedisList).Result()
		if err != nil && err != redis.Nil {
			log.Printf("processing recovery error: %v", err)
			break
		}
		if err == redis.Nil {
			break
		}
		_ = n
	}

	var batch []ingest.Record
	var raws []string
	flush := func() {
		if len(batch) == 0 {
			return
		}
		writeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := buf.FlushNow(writeCtx, batch)
		cancel()
		pipe := rdb.Pipeline()
		if err != nil {
			// Keep the batch: push it back to the head of the queue so it is
			// retried before newer records, then drop the claims.
			log.Printf("persist failed, requeueing %d records: %v", len(batch), err)
			for _, raw := range raws {
				pipe.LPush(ctx, ingest.RedisList, raw)
			}
		}
		for _, raw := range raws {
			pipe.LRem(ctx, ingest.RedisProcessing, 1, raw)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("ack pipeline error: %v", err)
		}
		batch = batch[:0]
		raws = raws[:0]
	}

	for {
		if ctx.Err() != nil {
			break
		}
		res, err := rdb.BLMove(ctx, ingest.RedisList, ingest.RedisProcessing, "RIGHT", "LEFT", 5*time.Second).Result()
		if err == redis.Nil {
			flush()
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf("blmove error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var rec ingest.Record
		if err := json.Unmarshal([]byte(res), &rec); err != nil {
			// Poison message: it can never parse, so acknowledge it instead
			// of looping on it forever.
			log.Printf("bad record: %v", err)
			if err := rdb.LRem(ctx, ingest.RedisProcessing, 1, res).Err(); err != nil {
				log.Printf("ack error: %v", err)
			}
			continue
		}
		batch = append(batch, rec)
		raws = append(raws, res)
		if len(batch) >= cfg.BatchSize {
			flush()
		}
	}
	flush()
}
