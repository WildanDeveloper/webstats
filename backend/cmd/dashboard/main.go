package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/db"
	"github.com/webstats/backend/internal/version"
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
	if cfg.JWTSecret == "webstats-dev-secret-change-me" {
		log.Printf("WARNING: using the default JWT_SECRET — set JWT_SECRET before exposing this service")
	}
	// JWTs are only accepted while a matching, non-expired row exists in the
	// sessions table. This makes logout and password changes revoke tokens.
	authMgr.UseSessionCheck(func(ctx context.Context, tokenHash string) bool {
		var ok bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash = $1 AND expires_at > now())`,
			tokenHash).Scan(&ok)
		return err == nil && ok
	})

	app := fiber.New()
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.AllowOrigins}))

	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	// Public so the frontend can show the installed version and check for
	// updates even before signing in.
	app.Get("/api/version", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": version.Version})
	})

	// Brute-force protection for credential endpoints: 10 attempts per
	// minute per IP (in-memory limiter, no external dependency).
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many attempts, slow down"})
		},
	})

	api := app.Group("/api")
	api.Post("/auth/register", authLimiter, registerHandler(pool, authMgr))
	api.Post("/auth/login", authLimiter, loginHandler(pool, authMgr))
	api.Post("/auth/logout", authMgr.Middleware(), logoutHandler(pool, authMgr))
	api.Get("/invites/:token", inviteInfoHandler(pool))
	api.Post("/invites/:token", acceptInviteHandler(pool, authMgr))
	api.Get("/unsubscribe/:token", unsubscribeHandler(pool))

	pub := api.Group("/public")
	pub.Get("/:token/overview", publicOverviewHandler(pool))
	pub.Get("/:token/timeseries", publicTimeseriesHandler(pool))
	pub.Get("/:token/pages", publicTopHandler(pool, "path"))
	pub.Get("/:token/referrers", publicTopHandler(pool, "referrer"))
	pub.Get("/:token/countries", publicTopHandler(pool, "country"))
	pub.Get("/:token/world", publicWorldHandler(pool))
	pub.Get("/:token/campaigns", publicCampaignsHandler(pool))
	pub.Get("/:token/goals", publicGoalSummariesHandler(pool))
	pub.Get("/:token/funnel", publicFunnelHandler(pool))
	pub.Get("/:token/events/detail", publicEventDetailsHandler(pool))
	pub.Get("/:token/events/:name", publicEventOccurrencesHandler(pool))
	pub.Get("/:token/status", publicStatusHandler(pool))
	pub.Get("/:token/insights", publicInsightsHandler(pool))

	api.Use(apiKeyFallback(pool))
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
	stats.Get("/realtime", realtimeHandler(pool))
	stats.Get("/realtime/stream", realtimeStreamHandler(pool, authMgr))
	stats.Get("/visitors", visitorsHandler(pool))
	stats.Get("/visitors/:ip", visitorDetailHandler(pool))
	stats.Get("/checks", checksHandler(pool))
	stats.Get("/world", worldHandler(pool))
	stats.Get("/export", exportHandler(pool))
	stats.Get("/campaigns", campaignsHandler(pool))
	stats.Get("/goals", listGoalsHandler(pool))
	stats.Post("/goals", createGoalHandler(pool))
	stats.Delete("/goals/:goal_id", deleteGoalHandler(pool))
	stats.Get("/goals/summary", goalSummariesHandler(pool))
	stats.Post("/funnel", funnelHandler(pool))
	stats.Get("/events/detail", eventDetailsHandler(pool))
	stats.Get("/events/:name", eventOccurrencesHandler(pool))
	stats.Get("/members", membersHandler(pool))
	stats.Delete("/members/:user_id", removeMemberHandler(pool))
	stats.Post("/invites", createInviteHandler(pool, cfg))
	stats.Get("/invites", listInvitesHandler(pool))
	stats.Delete("/invites/:invite_id", deleteInviteHandler(pool))
	stats.Get("/settings", getSettingsHandler(pool))
	stats.Patch("/settings", updateSettingsHandler(pool))
	stats.Put("/funnels", replaceFunnelHandler(pool))
	stats.Get("/funnels", listFunnelsHandler(pool))
	stats.Get("/funnel/data", funnelDataHandler(pool))
	stats.Get("/insights", insightsHandler(pool))
	stats.Get("/monitors", listMonitorsHandler(pool))
	stats.Post("/monitors", createMonitorHandler(pool))
	stats.Patch("/monitors/:mid", updateMonitorHandler(pool))
	stats.Delete("/monitors/:mid", deleteMonitorHandler(pool))
	stats.Get("/monitors/:mid/checks", monitorChecksHandler(pool))

	account := authed.Group("/account")
	account.Post("/password", changePasswordHandler(pool, authMgr))
	account.Get("/api-keys", listApiKeysHandler(pool))
	account.Post("/api-keys", createApiKeyHandler(pool))
	account.Delete("/api-keys/:kid", deleteApiKeyHandler(pool))

	notif := authed.Group("/notifications")
	notif.Get("/providers", listProvidersHandler(pool))
	notif.Post("/providers", createProviderHandler(pool))
	notif.Patch("/providers/:id", updateProviderHandler(pool))
	notif.Delete("/providers/:id", deleteProviderHandler(pool))
	notif.Post("/providers/:id/test", testProviderHandler(pool))
	notif.Get("/rules", listRulesHandler(pool))
	notif.Post("/rules", createRuleHandler(pool))
	notif.Patch("/rules/:id", updateRuleHandler(pool))
	notif.Delete("/rules/:id", deleteRuleHandler(pool))
	notif.Post("/rules/:id/test", testRuleHandler(pool))
	notif.Get("/logs", logsHandler(pool))
	notif.Delete("/logs", clearLogsHandler(pool))
	notif.Get("/reports", listReportsHandler(pool))
	notif.Post("/reports", createReportHandler(pool))
	notif.Patch("/reports/:id", updateReportHandler(pool))
	notif.Delete("/reports/:id", deleteReportHandler(pool))
	notif.Post("/reports/:id/test", testReportHandler(pool, cfg))

	admin := authed.Group("/admin", authMgr.AdminOnly())
	admin.Get("/users", listUsersHandler(pool))
	admin.Post("/users", createUserHandler(pool, authMgr))
	admin.Patch("/users/:id", updateUserHandler(pool, authMgr))
	admin.Delete("/users/:id", deleteUserHandler(pool))
	admin.Get("/stats", adminStatsHandler(pool))

	go uptimeLoop(ctx, pool)
	go alertLoop(ctx, pool)
	go monitorLoop(ctx, pool)
	go anomalyLoop(ctx, pool)
	go retentionLoop(ctx, pool)
	go reportLoop(ctx, pool, cfg)

	log.Printf("dashboard %s listening on :%s", version.Version, cfg.Port)
	log.Fatal(app.Listen(cfg.Bind + ":" + cfg.Port))
}

func uptimeLoop(ctx context.Context, pool *pgxpool.Pool) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	ticker := time.NewTicker(60 * time.Second)
	run := func() {
		rows, err := pool.Query(ctx, `SELECT id, domain FROM sites WHERE domain <> ''`)
		if err != nil {
			return
		}
		type site struct{ id, domain string }
		var sites []site
		for rows.Next() {
			var s site
			if rows.Scan(&s.id, &s.domain) == nil {
				sites = append(sites, s)
			}
		}
		rows.Close()
		// Check sites concurrently so one slow domain can't delay the rest.
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for _, s := range sites {
			wg.Add(1)
			sem <- struct{}{}
			go func(s site) {
				defer wg.Done()
				defer func() { <-sem }()
				url := "https://" + s.domain
				if !strings.Contains(s.domain, ".") {
					url = "http://" + s.domain
				}
				start := time.Now()
				resp, err := client.Get(url)
				status := "down"
				latency := time.Since(start).Milliseconds()
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode < 500 {
						status = "up"
					}
				}
				pool.Exec(ctx, `INSERT INTO site_checks (site_id, status, latency_ms) VALUES ($1, $2, $3)`,
					s.id, status, latency)
			}(s)
		}
		wg.Wait()
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
