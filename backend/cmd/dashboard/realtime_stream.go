package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// realtimeStreamHandler pushes the realtime panel over Server-Sent Events so
// the dashboard no longer polls every 30 seconds. EventSource cannot send
// Authorization headers, so the token is accepted as a query parameter and
// validated exactly like the middleware does (issuer + server-side session).
func realtimeStreamHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := m.ValidateTokenCtx(c.Context(), auth.BearerToken(c.Query("token")))
		if !ok {
			return errJSON(c, fiber.StatusUnauthorized, "invalid token")
		}
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("X-Accel-Buffering", "no")

		userID := claims.UserID
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			send := func() bool {
				out, err := analytics.Q.Realtime(context.Background(), db, userID, siteID)
				if err != nil {
					return true // transient DB error: keep the stream open
				}
				b, err := json.Marshal(out)
				if err != nil {
					return true
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
					return false
				}
				return w.Flush() == nil
			}
			if !send() {
				return
			}
			for {
				select {
				case <-c.Context().Done():
					return
				case <-ticker.C:
					if !send() {
						return
					}
				}
			}
		}))
		return nil
	}
}
