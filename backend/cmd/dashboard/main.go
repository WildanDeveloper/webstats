package main

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	authMgr := auth.NewManager(cfg.JWTSecret)

	app := fiber.New()
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.AllowOrigins}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	api := app.Group("/api")
	api.Post("/auth/register", registerHandler(pool, authMgr))
	api.Post("/auth/login", loginHandler(pool, authMgr))
	api.Post("/auth/logout", authMgr.Middleware(), logoutHandler(pool, authMgr))

	authed := api.Group("", authMgr.Middleware())
	authed.Get("/auth/me", meHandler(pool))
	authed.Get("/overview", overviewHandler(pool))

	authed.Get("/sites", listSitesHandler(pool))
	authed.Post("/sites", createSiteHandler(pool))
	authed.Get("/sites/:id", getSiteHandler(pool))
	authed.Patch("/sites/:id", updateSiteHandler(pool))
	authed.Delete("/sites/:id", deleteSiteHandler(pool))
	authed.Get("/sites/:id/ssl-check", sslCheckHandler(pool))

	stats := authed.Group("/sites/:id")
	stats.Get("/overview", siteOverviewHandler(pool))
	stats.Get("/timeseries", timeseriesHandler(pool))
	stats.Get("/pages", topHandler(pool, "path"))
	stats.Get("/referrers", topHandler(pool, "referrer"))
	stats.Get("/devices", topHandler(pool, "device"))
	stats.Get("/browsers", topHandler(pool, "browser"))
	stats.Get("/os", topHandler(pool, "os"))
	stats.Get("/countries", topHandler(pool, "country"))
	stats.Get("/events", eventsHandler(pool))

	admin := authed.Group("/admin", authMgr.AdminOnly())
	admin.Get("/users", listUsersHandler(pool))
	admin.Post("/users", createUserHandler(pool, authMgr))
	admin.Patch("/users/:id", updateUserHandler(pool, authMgr))
	admin.Delete("/users/:id", deleteUserHandler(pool))
	admin.Get("/stats", adminStatsHandler(pool))

	log.Printf("dashboard API listening on :%s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}
