package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
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
		c.Locals("uid", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("claims", claims)
		siteID := strings.Clone(c.Params("id"))
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("X-Accel-Buffering", "no")

		userID := strings.Clone(claims.UserID)
		conn := c.Context().Conn()
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			defer conn.SetWriteDeadline(time.Time{})
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			send := func() bool {
				queryCtx, queryCancel := context.WithTimeout(ctx, 5*time.Second)
				defer queryCancel()
				out, err := analytics.Q.Realtime(queryCtx, db, userID, siteID)
				if err != nil {
					return true // transient DB error: keep the stream open
				}
				b, err := json.Marshal(out)
				if err != nil {
					return true
				}
				if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
					return false
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
				case <-ctx.Done():
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
