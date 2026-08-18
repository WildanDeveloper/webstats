# WebStats

Open-source web analytics — tracking script vanilla JS (<5kb), backend Go
(Fiber), PostgreSQL, dashboard Next.js (Tailwind + Recharts).

```
flowchart TD
    A1..A3[Web A/B/C] -->|1 baris script| S[track.js <5kb]
    S -->|POST /api/collect| B[Ingestion API Go/Fiber]
    B -->|push| Q[Redis (opsional)]
    B -->|direct write| D2[(PostgreSQL)]
    Q --> W[Worker Go - batch insert]
    W --> D2
    F[Dashboard API Go] --> D1[(PostgreSQL: users, sites)]
    F --> D2[(PostgreSQL: pageviews, events)]
    G[Next.js dashboard] <--> F
    H[User/Admin] --> G
```

## Arsitektur

| Komponen | Teknologi | Lokasi |
|---|---|---|
| Tracking script | Vanilla JS, 2.3kb minified | `tracker/track.js` → `backend/internal/static/track.min.js` |
| Ingestion API | Go + Fiber + pgx | `backend/cmd/ingest` (port 8085) |
| Worker (batch insert) | Go, BRPOP Redis, wajib saat pakai queue | `backend/cmd/worker` |
| Dashboard API | Go + Fiber + JWT (golang-jwt) | `backend/cmd/dashboard` (port 8086) |
| Database | PostgreSQL (pgcrypto) | `db/migrations` |
| Dashboard Web | Next.js 14 + Tailwind + Recharts + NextAuth | `frontend` (port 3000) |

## Cara jalan (development)

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

Buka http://localhost:3000 → daftar akun → buat situs → salin 1 baris script:

```html
<script async src="http://localhost:8085/track.js" data-site="SITE_KEY"></script>
```

## Mode queue (opsional, skala besar)

```bash
docker compose --profile queue up -d        # postgres + redis
REDIS_URL=redis://localhost:6379 PORT=8085 go run ./cmd/ingest
REDIS_URL=redis://localhost:6379 go run ./cmd/worker
```

Tanpa `REDIS_URL`, ingestion mem-buffer di memori (goroutine channel) dan
flush batch setiap 5 detik / 100 record — sesuai diagram `direct write`.

## API ringkas

- `POST /api/collect`, `POST /api/event` — diterima tracker, tanpa auth (site_key sebagai kunci)
- `POST /api/auth/register|login|logout`, `GET /api/auth/me` — JWT
- `GET/POST/DELETE /api/sites`
- `GET /api/sites/:id/overview|timeseries|pages|referrers|devices|browsers|os|countries|events?period=24h|7d|30d|all`

## Konfigurasi (env)

`DATABASE_URL`, `JWT_SECRET`, `REDIS_URL`, `GEO_CSV` (GeoLite2-Country CSV,
opsional), `IP_HASH_SALT`, `PORT`, `FLUSH_EVERY`, `BATCH_SIZE`, `ALLOW_ORIGINS`.
Frontend: `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_TRACKER_URL`, `NEXTAUTH_SECRET`, `NEXTAUTH_URL`.

## Privasi

IP di-hash (SHA-256 + salt), tidak disimpan mentah. Geo lookup hanya dipakai
jika file CSV disediakan (`GEO_CSV`). User bisa opt-out via
`localStorage.setItem('_wst_optout','1')`.

## Regenerate track.min.js

```bash
make tracker   # pakai terser, wajib jika edit tracker/track.js
```
