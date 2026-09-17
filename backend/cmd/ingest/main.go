package main

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ingest"
	"github.com/webstats/backend/internal/static"
	"github.com/webstats/backend/internal/version"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.LoadChecked()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	g, err := geo.Load(cfg.GeoCSV)
	if err != nil {
		log.Fatalf("geo database: %v", err)
	}
	if g.Loaded() {
		log.Printf("geo database loaded")
	} else {
		log.Printf("geo database disabled")
	}
	asn, err := geo.LoadASN(cfg.ASNCSV)
	if err != nil {
		log.Fatalf("asn database: %v", err)
	}
	if asn.Loaded() {
		log.Printf("asn database loaded")
	} else {
		log.Printf("asn database disabled")
	}

	buf := ingest.NewBuffer(cfg, pool, g, asn)
	buf.Run(ctx)

	app := fiber.New(fiber.Config{
		ProxyHeader:             "X-Forwarded-For",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.TrustedProxies,
	})
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

	// On SIGINT/SIGTERM: stop accepting connections and wait for in-flight
	// requests. The queue drain happens below, after Listen returns.
	go func() {
		<-ctx.Done()
		log.Printf("shutting down ingestion API…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx)
	}()

	log.Printf("ingestion API %s listening on :%s", version.Version, cfg.Port)
	if err := app.Listen(cfg.Bind + ":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}

	// Listen only returns after ShutdownWithContext above: in-flight requests
	// are done and no new records arrive. Drain the buffer with a fresh
	// bounded context (the signal context is already canceled), wait for the
	// flusher, then close the pool.
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelDrain()
	if err := buf.Shutdown(drainCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	pool.Close()
}
