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

(If you prefer separate subdomains for API/tracker, use the commented
variant in `deploy/Caddyfile` and the 3-record layout in `deploy/README.md`.)

## 2. Prerequisites

```bash
sudo apt update && sudo apt install -y caddy postgresql nodejs npm golang
```

## 3. Database

```bash
sudo systemctl start postgresql && sudo systemctl enable postgresql
sudo -u postgres psql -c "CREATE USER webstats WITH PASSWORD 'webstats';"
sudo -u postgres psql -c "CREATE DATABASE webstats OWNER webstats;"
./db/migrate.sh        # run from the repo root (uses psql)
```

## 4. Build backend

```bash
cd backend && go build -o bin/dashboard ./cmd/dashboard
cd backend && go build -o bin/ingest ./cmd/ingest
```

## 5. Environment file

```bash
sudo cp deploy/webstats.env.example /etc/webstats.env
sudo nano /etc/webstats.env     # fill JWT_SECRET, IP_HASH_SALT, your domain
```

## 6. Systemd services

```bash
sudo cp deploy/webstats-dashboard.service deploy/webstats-ingest.service deploy/webstats-frontend.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now webstats-dashboard webstats-ingest webstats-frontend
sudo systemctl status webstats-dashboard webstats-ingest webstats-frontend
```

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

```bash
cd /root/webstats && git pull
cd backend && go build -o bin/dashboard ./cmd/dashboard && go build -o bin/ingest ./cmd/ingest
sudo systemctl restart webstats-dashboard webstats-ingest webstats-frontend
```