.PHONY: help setup dev dev-bg build up down logs logs-dev shell db \
        migrate migrate-down test clean psql

## ─── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "  FolioCV — Developer Commands"
	@echo ""
	@echo "  make setup         Copy .env.example → .env (first time only)"
	@echo "  make dev           Start full stack in dev mode (live reload)"
	@echo "  make dev-bg        Start dev stack in background"
	@echo "  make build         Build production Docker images"
	@echo "  make up            Start production stack (detached)"
	@echo "  make down          Stop all containers"
	@echo "  make logs          Tail production logs"
	@echo "  make logs-dev      Tail dev logs"
	@echo "  make shell         Shell into dev app container"
	@echo "  make psql          Open psql inside dev postgres container"
	@echo "  make db            Alias for psql"
	@echo "  make test          Run Go tests inside dev container"
	@echo "  make clean         Remove all containers, volumes, images"
	@echo ""

## ─── Setup ────────────────────────────────────────────────────────────────────

setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✅  .env created — fill in ANTHROPIC_API_KEY and SECRET_KEY before starting"; \
	else \
		echo "⚠️   .env already exists — skipping"; \
	fi

## ─── Development ──────────────────────────────────────────────────────────────

dev: check-env
	@echo "🚀  Starting FolioCV dev stack (Postgres + App with live reload)..."
	docker compose -f docker-compose.dev.yml up --build

dev-bg: check-env
	docker compose -f docker-compose.dev.yml up --build -d
	@echo "✅  Dev stack running at http://localhost:8080"

## ─── Production ───────────────────────────────────────────────────────────────

build:
	@echo "🔨  Building production images..."
	docker compose build --no-cache

up: check-env
	@echo "🚀  Starting FolioCV production stack..."
	docker compose up -d
	@echo "✅  Running at http://localhost:8080"

down:
	docker compose down
	docker compose -f docker-compose.dev.yml down

## ─── Utilities ────────────────────────────────────────────────────────────────

logs:
	docker compose logs -f

logs-dev:
	docker compose -f docker-compose.dev.yml logs -f

shell:
	docker compose -f docker-compose.dev.yml exec app sh

psql:
	@echo "📦  Opening psql (type \\dt to list tables, \\q to quit)..."
	docker compose -f docker-compose.dev.yml exec postgres \
		psql -U $${POSTGRES_USER} -d $${POSTGRES_DB}

db: psql

test:
	docker compose -f docker-compose.dev.yml run --rm app \
		go test ./... -v -count=1

## ─── Cleanup ──────────────────────────────────────────────────────────────────

clean:
	@echo "🧹  Removing all containers, volumes, and images..."
	docker compose down -v --remove-orphans
	docker compose -f docker-compose.dev.yml down -v --remove-orphans
	docker rmi foliocv-app foliocv-app-dev 2>/dev/null || true
	rm -rf tmp/
	@echo "✅  Clean complete"

## ─── Guards ───────────────────────────────────────────────────────────────────

check-env:
	@if [ ! -f .env ]; then \
		echo "❌  .env not found. Run: make setup"; \
		exit 1; \
	fi
