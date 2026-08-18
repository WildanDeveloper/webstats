package main

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ingest"
	"github.com/webstats/backend/internal/static"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	g, _ := geo.Load(cfg.GeoCSV)
	if g.Loaded() {
		log.Printf("geo database loaded")
	}

	buf := ingest.NewBuffer(cfg, pool, g)
	buf.Run(ctx)
	defer buf.Stop()

	app := fiber.New(fiber.Config{ProxyHeader: "X-Forwarded-For"})
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.AllowOrigins, AllowMethods: "GET,POST,OPTIONS", AllowHeaders: "Content-Type"}))

	// The tracking script itself is served here so sites only need one line.
	app.Get("/track.js", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/javascript; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.Send(static.TrackJS)
	})

	app.Post("/api/collect", buf.CollectHandler)
	app.Post("/api/event", buf.CollectHandler)

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	log.Printf("ingestion API listening on :%s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}
