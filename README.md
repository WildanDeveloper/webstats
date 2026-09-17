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

From the repository root, generate the secrets described below and export them in each backend terminal. Start PostgreSQL and migrate:

```bash
docker compose up -d db
PGPASSWORD="$POSTGRES_PASSWORD" ./db/migrate.sh
export DATABASE_URL="postgres://webstats:$POSTGRES_PASSWORD@localhost:5432/webstats"
```

Run each service in a separate terminal, starting from the repository root:

```bash
(cd backend && PORT=8086 go run ./cmd/dashboard)
(cd backend && PORT=8085 go run ./cmd/ingest)
(cd frontend && npm ci && NEXTAUTH_URL=http://localhost:3000 NEXT_PUBLIC_API_URL=http://localhost:8086 npm run dev)
```

Open `http://localhost:3000`, register via "Sign up", then promote your account with SQL:

```sql
UPDATE users SET role='admin' WHERE email='you@example.com';
```

No default admin is seeded; older installs rotate the seeded `admin123` password via migration 011.

### Production with Docker

Compose requires `POSTGRES_PASSWORD`, `JWT_SECRET`, `IP_HASH_SALT`, and `NEXTAUTH_SECRET`; missing or empty values stop configuration. Generate each independently with at least 32 random bytes:

```bash
export POSTGRES_PASSWORD="$(openssl rand -hex 32)"
export JWT_SECRET="$(openssl rand -hex 32)"
export IP_HASH_SALT="$(openssl rand -hex 32)"
export NEXTAUTH_SECRET="$(openssl rand -hex 32)"
```

Persist these values securely (for example, a repository-root `.env` with mode `0600`, never committed). Do not regenerate them on each update or use the example placeholders. Use a hex database password to avoid URL-encoding issues. Changing `POSTGRES_PASSWORD` does not rotate the password in an existing database volume; update that database role separately. Compose checks presence, not entropy or placeholder values; never rely on backend development fallbacks in production.

Set public URLs before building, using the single-domain layout in `deploy/Caddyfile`:

```bash
export APP_PUBLIC_URL=https://stats.yourdomain.com
export API_PUBLIC_URL=https://stats.yourdomain.com/_api
export NEXT_PUBLIC_TRACKER_URL=https://stats.yourdomain.com/_tracker
docker compose --profile full up -d --build
```

This starts db, migrations, ingest, dashboard, and web. For local-only testing, omit the public URL overrides to use localhost ports 3000, 8086, and 8085.

To enable queue mode, override the ingest environment as well as enabling the worker profile:

```bash
REDIS_URL=redis://redis:6379 docker compose --profile full --profile queue up -d --build
```

Persist `REDIS_URL=redis://redis:6379` in the same `.env` for subsequent queue deployments. The `queue` profile alone only adds Redis/worker; it cannot override ingest's environment. Ingest and worker use the same `REDIS_URL` override. To return to direct writes, drain the queue, remove `REDIS_URL`, then run `docker compose --profile full up -d --build --remove-orphans`.

All published ports bind to `127.0.0.1`. Run Caddy on the host for TLS and route:

| Path | Upstream |
|---|---|
| `/_api/*` (strip `/_api`) | `127.0.0.1:8086` |
| `/_tracker/*` (strip `/_tracker`) | `127.0.0.1:8085` |
| everything else | `127.0.0.1:3000` |

The web container uses runtime `API_SERVER_URL=http://dashboard:8086` for authentication/SSR, runtime `NEXTAUTH_URL` from `APP_PUBLIC_URL`, and a separate runtime `NEXTAUTH_SECRET`. Browser URLs (`NEXT_PUBLIC_*`) are build arguments, not runtime settings: rebuild the web image whenever public API/tracker URLs change. Never supply secrets as build arguments.

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
| `NEXT_PUBLIC_TRACKER_URL` | same origin | frontend (build time, install snippet) |
| `API_SERVER_URL` | public API fallback | frontend server (runtime; Compose uses `http://dashboard:8086`) |
| `NEXTAUTH_URL` | unset | frontend (runtime; public dashboard origin) |
| `NEXTAUTH_SECRET` | unset (**required in production**) | frontend (runtime; unique random secret) |

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

Created by [WildanDev](https://wildandev.tech) · licensed under the [MIT License](LICENSE).
