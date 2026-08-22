# WebStats

Open-source web analytics — lightweight tracking script (<5kb), Go backend
(Fiber + PostgreSQL), Next.js dashboard (Tailwind + Recharts).

<p align="center">
  <img src=".github/assets/dashboard.png" alt="WebStats dashboard" width="820">
</p>

## ✨ Highlights

- **One-line tracking script** — SPA-aware (pushState/popstate/**hashchange**), bot filtering, and opt-in auto events (`data-outbound`, `data-download`, `data-scroll`)
- **Multi-site dashboard** — traffic trends with previous-period comparison, realtime panel over **SSE**, top pages/referrers/geo/device breakdowns
- **Uptime monitoring & public status page** — parallel checks, 90-day uptime bars
- **Alerts & scheduled reports** — webhook or email (SMTP, Resend, SendGrid, Mailgun, Postmark, Brevo), one-click unsubscribe links
- **Goals, funnels (ordered steps) and UTM campaign tracking**
- **Team features** — invites by email, per-site members, admin panel
- **Privacy-first** — salted IP hashing toggle, retention windows, visitor opt-out cookie
- **Versioned releases** — the dashboard shows the installed version and notifies you when a newer release is published on GitHub
- Light/dark mode · CSV export · SSL certificate checker

| Dashboard | Site stats |
|---|---|
| ![Dashboard](.github/assets/dashboard.png) | ![Site stats](.github/assets/site-stats.png) |

| Public status page | Login |
|---|---|
| ![Status page](.github/assets/status-page.png) | ![Login](.github/assets/login.png) |

## Architecture

| Component | Technology | Location |
|---|---|---|
| Tracking script | Vanilla JS, ~3kb minified | `tracker/track.js` → `backend/internal/static/track.min.js` |
| Ingestion API | Go + Fiber + pgx (COPY-based bulk inserts) | `backend/cmd/ingest` (port 8085) |
| Worker (optional queue) | Go, BRPOP Redis | `backend/cmd/worker` |
| Dashboard API | Go + Fiber + JWT + server-side sessions | `backend/cmd/dashboard` (port 8086) |
| Database | PostgreSQL (pgcrypto), monthly-partitioned pageviews | `db/migrations` |
| Dashboard Web | Next.js 14 + Tailwind + Recharts + NextAuth | `frontend` (port 3000) |

```
Websites → track.js → POST /api/collect → Ingestion API → PostgreSQL
                                  └──→ Redis queue → Worker → PostgreSQL   (optional)
Dashboard Web ←→ Dashboard API :8086 ←→ PostgreSQL
```

## Getting started (development)

```bash
# 1. Database
docker compose up -d db
./db/migrate.sh

# 2. Backend
cd backend
go run ./cmd/dashboard     # API       :8086
go run ./cmd/ingest        # collector :8085

# 3. Frontend
cd frontend
npm install
NEXT_PUBLIC_API_URL=http://localhost:8086 npm run dev   # :3000

# 4. Open http://localhost:3000 and sign in
#    default seed: admin@webstats.dev / admin123 (dev only!)
```

### Production with Docker

```bash
docker compose --profile full up -d --build
# starts: db → migrate → ingest(:8085) + dashboard(:8086) + web(:3000)
# add the Redis worker instead of direct writes:
docker compose --profile full --profile queue up -d --build
```

Put a reverse proxy (Caddy/Traefik/Nginx) in front for TLS and route:

| Path | Upstream |
|---|---|
| `/track.js`, `/api/collect`, `/api/event` | `ingest:8085` |
| everything else | `web:3000` (which calls `dashboard:8086`) |

### Configuration

| Variable | Default | Used by |
|---|---|---|
| `DATABASE_URL` | `postgres://webstats:webstats@localhost:5432/webstats` | all |
| `JWT_SECRET` | dev fallback (**set in prod!**) | dashboard |
| `PORT` / `BIND` | `8086`/`8085`, bind empty | APIs |
| `GEO_CSV`, `GEO_ASN_CSV` | unset (GeoIP off) | ingest |
| `REDIS_URL` | unset (direct writes) | ingest/worker |
| `IP_HASH_SALT` | dev fallback | ingest |
| `ALLOW_ORIGINS` | `*` | dashboard |
| `APP_PUBLIC_URL` | `http://localhost:3000` | dashboard (links inside emails) |
| `API_PUBLIC_URL` | `http://localhost:8086` | dashboard (unsubscribe links) |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8086` | frontend (build time) |
| `NEXT_PUBLIC_TRACKER_URL` | same origin | frontend (install snippet) |

## Releases & updates

The backend serves its version at `GET /api/version`. The sidebar shows it,
and once every few hours compares it against the latest GitHub release — if a
newer version exists you get an "Update available" notice with a link.

Releasing a new version:

1. Bump `Version` in `backend/internal/version/version.go`
2. Tag & publish: `git tag vX.Y.Z && git push origin vX.Y.Z`, then create the
   GitHub release for that tag

## Development notes

- `make tracker` regenerates the minified script after editing `tracker/track.js`
- `make build` builds all three Go binaries; CI runs build/vet/test + frontend typecheck/build
- Migrations are idempotent and run in order from `db/migrations/`

---

Created by [WildanDev](https://wildandev.tech) · licensed open source.
