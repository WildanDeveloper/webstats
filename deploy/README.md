# Deploy WebStats on a VPS with your own domain

This setup serves the dashboard on your own domain with automatic SSL (Let's
Encrypt) using Caddy as reverse proxy, and runs the Go services as systemd
units. The backend only listens on 127.0.0.1 — only Caddy is exposed to the
internet.

## Architecture (single domain — one DNS record only)

```
Internet ──> Caddy :80/:443 ──> https://stats.yourdomain.com          ──> Next.js :3000
                              ├─> https://stats.yourdomain.com/_api/* ──> dashboard API :8086
                              └─> https://stats.yourdomain.com/_tracker/* ─> ingest API :8085
```

The tracker script on your website resolves the ingestion host from its own
URL, so the one-line install script stays exactly the same.

## 1. DNS

One A record:

| Host                   | Type | Value     |
|------------------------|------|-----------|
| `stats.yourdomain.com` | A    | server IP |

For separate API/tracker subdomains, use the optional variant in `deploy/Caddyfile` and create an A record for each subdomain.

## 2. Prerequisites and working directory

Install Caddy, PostgreSQL (including `psql`), Node.js 20 with npm, and the Go version specified in `backend/go.mod`. Distribution packages may need upgrading to those versions.

Place a checkout at `/opt/webstats`, owned by your deployment account, not the service user. All commands below start from that repository root unless an explicit directory is shown. Do not deploy under `/root`, which the non-root services cannot traverse.

```bash
cd /opt/webstats
sudo useradd --system --user-group --home-dir /opt/webstats --no-create-home --shell /usr/sbin/nologin webstats
sudo chgrp -R webstats /opt/webstats
sudo chmod -R g+rX /opt/webstats
```

Create the `webstats` user only once. Keep source, binaries, and dependencies writable only by your deployment account; grant the service group read/traverse access after updates.

## 3. Environment file

```bash
sudo install -m 0600 -o root -g root deploy/webstats.env.example /etc/webstats.env
sudo nano /etc/webstats.env
openssl rand -hex 32
```

Run `openssl rand -hex 32` separately for the database password, `JWT_SECRET`, `IP_HASH_SALT`, and `NEXTAUTH_SECRET`, and replace every `REPLACE_WITH_*` value. Never reuse secrets or use development fallbacks in production. Preserve these values across updates. Use a hex database password so it is safe in `DATABASE_URL` without escaping. The example placeholders are not usable secrets; backend fail-closed validation must also reject them.

Set `APP_PUBLIC_URL` and `NEXTAUTH_URL` to your public dashboard origin, and `API_PUBLIC_URL` and `NEXT_PUBLIC_API_URL` to that origin plus `/_api`. Set `NEXT_PUBLIC_TRACKER_URL` to the origin plus `/_tracker`. `API_SERVER_URL=http://127.0.0.1:8086` is for server-side requests, not browsers. Keep this file root-readable only; systemd reads it before switching to `User=webstats`.

## 4. Database

```bash
sudo systemctl enable --now postgresql
sudo -u postgres createuser --pwprompt webstats
sudo -u postgres createdb --owner=webstats webstats
sudo sh -c 'set -a; . /etc/webstats.env; set +a; exec /opt/webstats/db/migrate.sh'
```

Enter the same unique database password used in `DATABASE_URL` at the password prompt. PostgreSQL must listen only on loopback, and its host authentication must require a password. Migrations use the explicit `DATABASE_URL`; do not run them against an unrelated database.

## 5. Build backend and frontend

From `/opt/webstats`, as your deployment account:

```bash
(cd backend && go build -o bin/dashboard ./cmd/dashboard && go build -o bin/ingest ./cmd/ingest)
(cd frontend && npm ci && ./node_modules/.bin/tsc --noEmit --incremental false && NEXT_PUBLIC_API_URL=https://stats.yourdomain.com/_api NEXT_PUBLIC_TRACKER_URL=https://stats.yourdomain.com/_tracker npm run build)
sudo chgrp -R webstats /opt/webstats
sudo chmod -R g+rX /opt/webstats
sudo chmod -R g+rwX /opt/webstats/frontend/.next/cache
```

Replace the public URLs in the build command with the values chosen in `/etc/webstats.env`. `NEXT_PUBLIC_*` is inlined during `next build`; the runtime environment file cannot change existing browser bundles. Only Next.js's runtime cache needs service-user write access. `npm run start` requires this production build; it does not build the frontend. No secrets are needed in the build command.

## 6. Systemd services

```bash
sudo cp deploy/webstats-dashboard.service deploy/webstats-ingest.service deploy/webstats-frontend.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now webstats-dashboard webstats-ingest webstats-frontend
sudo systemctl status webstats-dashboard webstats-ingest webstats-frontend
```

All three units run as `webstats`. Both Go APIs and `next start` listen on loopback only. If you change the installation directory, update the units and `GEO_CSV` together.

## 7. Caddy (reverse proxy + automatic SSL)

```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo sed -i 's/yourdomain.com/YOUR-DOMAIN/g' /etc/caddy/Caddyfile   # or edit manually
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Caddy will fetch SSL certificates automatically. Open
`https://stats.yourdomain.com` — done.

## 8. Verify

```bash
curl -s https://stats.yourdomain.com/_api/healthz
curl -s https://stats.yourdomain.com/_tracker/track.js | head -c 80
```

## Troubleshooting

- **502 from Caddy**: a backend unit is down — `systemctl status webstats-*`, check `journalctl -u webstats-dashboard -n 30`.
- **Login redirects to localhost**: `NEXTAUTH_URL` in `/etc/webstats.env` is wrong.
- **Tracker data not arriving**: check the site install code uses `tracker.yourdomain.com/track.js` or `https://stats.yourdomain.com/_tracker/track.js`, and `ALLOW_ORIGINS` includes `https://stats.yourdomain.com`.
- **Ports open**: ensure only 80/443 are public; backend binds 127.0.0.1 already.

## Updating

Back up the database first. For this in-place deployment, stop the services before replacing binaries or `.next` so active requests do not mix old and new build artifacts. Run as your deployment account:

```bash
cd /opt/webstats
git pull
sudo systemctl stop webstats-frontend webstats-dashboard webstats-ingest
sudo sh -c 'set -a; . /etc/webstats.env; set +a; exec /opt/webstats/db/migrate.sh'
(cd backend && go build -o bin/dashboard ./cmd/dashboard && go build -o bin/ingest ./cmd/ingest)
(cd frontend && npm ci && ./node_modules/.bin/tsc --noEmit --incremental false && NEXT_PUBLIC_API_URL=https://stats.yourdomain.com/_api NEXT_PUBLIC_TRACKER_URL=https://stats.yourdomain.com/_tracker npm run build)
sudo chgrp -R webstats /opt/webstats
sudo chmod -R g+rX /opt/webstats
sudo chmod -R g+rwX /opt/webstats/frontend/.next/cache
sudo cp deploy/webstats-dashboard.service deploy/webstats-ingest.service deploy/webstats-frontend.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl restart webstats-dashboard webstats-ingest webstats-frontend
```

Use your configured public URLs in every build, including updates that change only environment values. Stop on any migration, typecheck, or build error; restart only after all steps succeed. Review new environment settings without overwriting your existing secrets.