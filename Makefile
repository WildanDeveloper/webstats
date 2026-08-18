.PHONY: all db-up db-down migrate build run-ingest run-dashboard run-worker run-frontend clean

all: build

db-up:
	docker compose up -d db

db-down:
	docker compose down

db-up-queue:
	docker compose --profile queue up -d

migrate:
	./db/migrate.sh

build:
	cd backend && go build -o bin/ingest ./cmd/ingest
	cd backend && go build -o bin/dashboard ./cmd/dashboard
	cd backend && go build -o bin/worker ./cmd/worker

run-ingest:
	cd backend && go run ./cmd/ingest

run-dashboard:
	cd backend && go run ./cmd/dashboard

run-worker:
	cd backend && go run ./cmd/worker

run-frontend:
	cd frontend && npm run dev

tracker:
	npx terser tracker/track.js -c -m -o backend/internal/static/track.min.js
	@echo "track.min.js: $$(wc -c < backend/internal/static/track.min.js) bytes"

clean:
	rm -rf backend/bin frontend/.next frontend/node_modules
