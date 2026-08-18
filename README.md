# WebStats

Open-source web analytics — tracking script vanilla JS (<5kb), backend Go
(Fiber), PostgreSQL, dashboard Next.js (Tailwind + Recharts).

```
flowchart TD
    A1..A3[Web A/B/C] -->|1 line script| S[track.js <5kb]
    S -->|POST /api/collect| B[Ingestion API Go/Fiber]
    B -->|push| Q[Redis (optional)]
    B -->|direct write| D2[(PostgreSQL)]
    Q --> W[Worker Go - batch insert]
    W --> D2
    F[Dashboard API Go] --> D1[(PostgreSQL: users, sites)]
    F --> D2[(PostgreSQL: pageviews, events)]
    G[Next.js dashboard] <--> F
    H[User/Admin] --> G
```

## Features

- Tracking script that installs with a single line of HTML
- Per-site dashboards with visitor trends, top pages, referrers, devices,
  browsers, OS, countries and custom events
- Multi-site root dashboard with a colored area chart per website
- Admin panel: manage users and roles (admin/user), reset passwords, delete
  accounts
- Per-site settings: rename, change domain, pick a chart color, check the
  SSL certificate of the domain, and copy the install script
- Light / dark mode toggle
- IP hashing (SHA-256 + salt), opt-out support, optional Geo lookup

## Architecture

| Component | Technology | Location |
|---|---|---|
| Tracking script | Vanilla JS, 2.3kb minified | `tracker/track.js` → `backend/internal/static/track.min.js` |
| Ingestion API | Go + Fiber + pgx | `backend/cmd/ingest` (port 8085) |
| Worker (batch insert) | Go, BRPOP Redis, required with queue | `backend/cmd/worker` |
| Dashboard API | Go + Fiber + JWT (golang-jwt) | `backend/cmd/dashboard` (port 8086) |
| Database | PostgreSQL (pgcrypto) | `db/migrations` |
| Dashboard Web | Next.js 14 + Tailwind + Recharts + NextAuth | `frontend` (port 3000) |

## Getting started (development)

```bash
# 1. Database
docker compose up -d db
./db/migrate.sh

# 2. Backend (terminal 1: ingestion, terminal 2: dashboard)
cd backend && go run ./cmd/ingest       # :8085
cd backend && go run ./cmd/dashboard    # :8086

# 3. Frontend
cd frontend && npm install && npm run dev   # :3000
```

Open http://localhost:3000 and sign in. A default admin account is seeded by
the migrations:

```
Email:    admin@webstats.dev
Password: admin123
```

Create a site, copy the one-line install script into your website:

```html
<script async src="http://localhost:8085/track.js" data-site="SITE_KEY"></script>
```

## Queue mode (optional, for scale)

```bash
docker compose --profile queue up -d        # postgres + redis
REDIS_URL=redis://localhost:6379 PORT=8085 go run ./cmd/ingest
REDIS_URL=redis://localhost:6379 go run ./cmd/worker
```

Without `REDIS_URL`, ingestion buffers in memory (goroutine channel) and
flushes batches every 5 seconds / 100 records — the `direct write` path.

## API overview

- `POST /api/collect`, `POST /api/event` — called by the tracker, no auth (site_key is the key)
- `POST /api/auth/login|logout`, `GET /api/auth/me` — JWT
- `GET/POST/PATCH/DELETE /api/sites`, `GET /api/sites/:id/ssl-check`
- `GET /api/sites/:id/overview|timeseries|pages|referrers|devices|browsers|os|countries|events?period=24h|7d|30d|all`
- `GET /api/overview` — root dashboard, multi-site series
- Admin only: `GET/POST /api/admin/users`, `PATCH/DELETE /api/admin/users/:id`, `GET /api/admin/stats`

## Configuration (env)

`DATABASE_URL`, `JWT_SECRET`, `REDIS_URL`, `GEO_CSV` (GeoLite2-Country CSV,
optional), `IP_HASH_SALT`, `PORT`, `FLUSH_EVERY`, `BATCH_SIZE`, `ALLOW_ORIGINS`.
Frontend: `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_TRACKER_URL`, `NEXTAUTH_SECRET`, `NEXTAUTH_URL`.

## Privacy

IPs are hashed (SHA-256 + salt), never stored raw. Geo lookup only runs if a
CSV file is provided (`GEO_CSV`). Users can opt out with
`localStorage.setItem('_wst_optout','1')`.

## Regenerate track.min.js

```bash
make tracker   # uses terser, required after editing tracker/track.js
```

---

Created by [WildanDev](https://wildandev.tech)