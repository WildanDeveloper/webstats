package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ingest"
	"github.com/webstats/backend/internal/static"
	"github.com/webstats/backend/internal/version"
)

func main() {
	cfg := config.Load()

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	g, _ := geo.Load(cfg.GeoCSV)
	if g.Loaded() {
		log.Printf("geo database loaded")
	}
	asn, _ := geo.LoadASN(cfg.ASNCSV)
	if asn.Loaded() {
		log.Printf("asn database loaded")
	}

	buf := ingest.NewBuffer(cfg, pool, g, asn)
	buf.Run(ctx)

	app := fiber.New(fiber.Config{ProxyHeader: "X-Forwarded-For"})
	// The tracker never sends credentials, so keep CORS permissive but do NOT
	// combine arbitrary reflected origins with AllowCredentials.
	app.Use(cors.New(cors.Config{
		AllowMethods:     "GET,POST,OPTIONS",
		AllowHeaders:     "Content-Type",
		AllowOriginsFunc: func(origin string) bool { return true },
	}))

	app.Get("/track.js", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/javascript; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.Send(static.TrackJS)
	})

	app.Post("/api/collect", buf.CollectHandler)
	app.Post("/api/event", buf.CollectHandler)

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "version": version.Version})
	})

	// On SIGINT/SIGTERM: stop accepting connections, then give the flusher a
	// chance to drain the queue so buffered events are not lost.
	go func() {
		<-ctx.Done()
		log.Printf("shutting down ingestion API…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx)
	}()

	log.Printf("ingestion API %s listening on :%s", version.Version, cfg.Port)
	if err := app.Listen(cfg.Bind+":"+cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}

	// Flush anything still buffered before exit (must not be deferred: it
	// closes a channel and must run exactly once).
	buf.Stop()
}
