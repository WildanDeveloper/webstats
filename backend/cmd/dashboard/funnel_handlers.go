package main

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
)

func siteAccessByUser(c *fiber.Ctx, db *pgxpool.Pool, siteID string) bool {
	var ok bool
	err := db.QueryRow(c.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM sites s
			LEFT JOIN site_members m ON m.site_id = s.id
			WHERE s.id = $1 AND (s.user_id = $2 OR m.user_id = $2)
		)`, siteID, auth.UserID(c)).Scan(&ok)
	return err == nil && ok
}

func funnelPaths(ctx context.Context, db *pgxpool.Pool, siteID string) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT label FROM funnel_steps WHERE site_id = $1 ORDER BY position`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func listFunnelsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, site_id, position, label FROM funnel_steps WHERE site_id = $1 ORDER BY position`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.FunnelStep
		for rows.Next() {
			var s model.FunnelStep
			if err := rows.Scan(&s.ID, &s.SiteID, &s.Position, &s.Label); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, s)
		}
		if out == nil {
			out = []model.FunnelStep{}
		}
		return c.JSON(out)
	}
}

func replaceFunnelHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		var in struct {
			Paths []string `json:"paths"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if len(in.Paths) < 2 {
			return errJSON(c, 400, "at least 2 funnel steps required")
		}
		for i := range in.Paths {
			in.Paths[i] = strings.TrimSpace(in.Paths[i])
		}
		tx, err := db.Begin(c.Context())
		if err != nil {
			return errJSON(c, 500, "transaction failed")
		}
		defer tx.Rollback(c.Context())
		if _, err := tx.Exec(c.Context(), `DELETE FROM funnel_steps WHERE site_id = $1`, siteID); err != nil {
			return errJSON(c, 500, "delete failed")
		}
		for i, p := range in.Paths {
			if _, err := tx.Exec(c.Context(), `
				INSERT INTO funnel_steps (site_id, position, label) VALUES ($1, $2, $3)`, siteID, i, p); err != nil {
				return errJSON(c, 500, "insert failed")
			}
		}
		if err := tx.Commit(c.Context()); err != nil {
			return errJSON(c, 500, "commit failed")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func funnelDataHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		paths, err := funnelPaths(c.Context(), db, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		out, err := analytics.Q.Funnel(c.Context(), db, auth.UserID(c), siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), paths, filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(fiber.Map{"steps": paths, "report": out})
	}
}
