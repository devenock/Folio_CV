# CLAUDE.md — FolioCV
> "Your CV is the source. Your portfolio is the story."

---

## What This File Is

This is the complete agent instruction file for FolioCV. Read every section before writing a single line of code. This file takes the project from an empty directory to a fully working, containerized, production-ready portfolio builder. Follow the build order exactly. Do not skip phases. Do not implement features ahead of their phase.

---

## Project Overview

FolioCV is a self-hosted portfolio builder. A user uploads their CV (PDF or DOCX), the app sends it to the Claude API which parses it into structured data and infers their career type, then instantly generates a beautiful public portfolio page. The user can then edit every section inline, drag sections to reorder them, toggle sections on or off, switch themes, and share a clean public URL with anyone.

It works for everyone — developers, nurses, teachers, designers, marketers, accountants, photographers. The AI detects the career type from the CV and tailors the layout, section prominence, and theme suggestion accordingly.

**Everything runs in Docker. Postgres runs as a container. The Go app runs as a container. One command starts the full stack.**

---

## Core Principles

1. **CV is the single source of truth.** The user uploads once. Claude does the hard work. Everything is derived from the CV — the user only edits to refine, not to rebuild from scratch.
2. **Universal, not developer-only.** Section types, themes, and UI language must work for any profession.
3. **Self-hosted, data-owned.** No third-party analytics, no data leaving the server except for the one-time Claude API call.
4. **Parse once, never re-parse.** After the initial Claude call, everything lives in Postgres. Re-rendering and re-editing never call the API again.
5. **Public URL is the product.** The portfolio page at `/p/{slug}` is what gets shared with recruiters. It must look exceptional with zero login required.

---

## Full Tech Stack

| Layer | Tool | Version | Why |
|---|---|---|---|
| Language | Go | 1.22 | Single binary, excellent stdlib HTTP, goroutines |
| Router | `chi` | v5 | Lightweight, idiomatic, middleware support |
| Templates | `html/template` stdlib | — | Server-side rendering, multiple themes, no build step |
| Frontend interactivity | HTMX | 2.x CDN | Inline editing, form swaps, polling |
| Drag-and-drop | SortableJS | latest CDN | Official HTMX integration, one script tag |
| Styling | Tailwind CSS | CDN | Theme switching via CSS classes |
| Database | PostgreSQL | 16-alpine | Relational, normalized section data, concurrent-safe |
| DB driver | `pgx/v5` | v5 | Best-in-class Postgres driver for Go |
| DB migrations | `golang-migrate/migrate` | v4 | File-based SQL migrations, runs on startup |
| AI parsing | Anthropic Claude API | claude-sonnet-4-6 | PDF understanding + structured JSON output |
| HTTP client | Go stdlib `net/http` | — | Claude API calls (no Python SDK — pure Go HTTP) |
| Auth | bcrypt + session cookie | — | `golang.org/x/crypto/bcrypt`, no JWT |
| UUID | `github.com/google/uuid` | v1 | Portfolio and section IDs |
| Env loading | `github.com/joho/godotenv` | v1 | `.env` file support |
| Slug | Custom Go function | — | `slugify(name) + random suffix` |
| Container | Docker + Docker Compose | — | App + Postgres as services |
| Dev reload | `cosmtrek/air` | latest | Hot reload inside container |
| Build | Makefile | — | Single entry point for all commands |

---

## Complete Project Structure

Create every file and directory listed. Do not skip any. Empty files are fine as placeholders — they will be filled in during the build phases.

```
foliocv/
├── CLAUDE.md                            # This file
├── Makefile                             # All developer commands
├── Dockerfile                           # Multi-stage production build
├── Dockerfile.dev                       # Development image with air
├── docker-compose.yml                   # Production: app + postgres
├── docker-compose.dev.yml               # Development: app + postgres + volumes
├── .env.example                         # Template — copy to .env
├── .env                                 # Actual env vars (gitignored)
├── .air.toml                            # Air live reload config
├── .gitignore
├── go.mod
├── go.sum
│
├── migrations/                          # SQL migration files
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   ├── 000002_create_sessions.up.sql
│   ├── 000002_create_sessions.down.sql
│   ├── 000003_create_portfolios.up.sql
│   ├── 000003_create_portfolios.down.sql
│   ├── 000004_create_sections.up.sql
│   ├── 000004_create_sections.down.sql
│   ├── 000005_create_section_items.up.sql
│   └── 000005_create_section_items.down.sql
│
├── main.go                              # Entry point
│
├── db/
│   └── db.go                            # Postgres connection + migration runner
│
├── models/
│   ├── user.go                          # User + Session structs + DB methods
│   ├── portfolio.go                     # Portfolio struct + DB methods
│   ├── section.go                       # Section struct + DB methods
│   └── item.go                          # SectionItem struct + DB methods
│
├── handlers/
│   ├── auth.go                          # Register, login, logout
│   ├── upload.go                        # CV upload + Claude parse trigger
│   ├── portfolio.go                     # Dashboard, public view, settings
│   ├── section.go                       # Section CRUD + reorder + toggle
│   ├── item.go                          # Item CRUD (add, edit, delete)
│   └── theme.go                         # Theme switching
│
├── middleware/
│   ├── auth.go                          # Session validation
│   └── logger.go                        # Request logger
│
├── services/
│   ├── claude.go                        # Claude API client + PDF parsing
│   └── parser.go                        # ResumeData struct + career type inference
│
├── templates/
│   ├── base.html                        # Editor base layout (authenticated)
│   ├── landing.html                     # Public landing page
│   ├── 404.html
│   ├── error.html                       # Generic error page
│   ├── auth/
│   │   ├── login.html
│   │   └── register.html
│   ├── upload/
│   │   ├── index.html                   # Upload page (first time + re-upload)
│   │   └── processing.html              # Parsing in progress (HTMX polling)
│   ├── dashboard/
│   │   └── index.html                   # Portfolio editor dashboard
│   ├── editor/
│   │   ├── section_card.html            # Draggable section card (partial)
│   │   ├── section_editor.html          # Expanded section editor (partial)
│   │   ├── item_row.html                # Single item row (partial)
│   │   ├── item_form.html               # Add/edit item form (partial)
│   │   └── theme_picker.html            # Theme switcher (partial)
│   └── portfolio/
│       ├── professional.html            # Professional theme public view
│       ├── creative.html                # Creative theme public view
│       └── minimal.html                 # Minimal theme public view
│
└── static/
    ├── favicon.ico
    └── css/
        └── themes.css                   # Custom theme overrides (minimal)
```

---

## Docker Setup

### `Dockerfile` (production — multi-stage)

```dockerfile
# Stage 1: build Go binary
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o foliocv .

# Stage 2: minimal runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/foliocv .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/migrations ./migrations

RUN mkdir -p /app/data/uploads

EXPOSE 8080
CMD ["./foliocv"]
```

Note: CGO_ENABLED=0 because we use `pgx` (pure Go) instead of `mattn/go-sqlite3`. No gcc needed.

### `Dockerfile.dev`

```dockerfile
FROM golang:1.22-alpine

RUN apk add --no-cache git curl

RUN go install github.com/cosmtrek/air@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

EXPOSE 8080
CMD ["air", "-c", ".air.toml"]
```

### `docker-compose.yml` (production)

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    container_name: foliocv_postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - foliocv_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5

  app:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: foliocv_app
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - foliocv_uploads:/app/data/uploads
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s

volumes:
  foliocv_pgdata:
  foliocv_uploads:
```

### `docker-compose.dev.yml` (development)

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    container_name: foliocv_postgres_dev
    restart: unless-stopped
    ports:
      - "5432:5432"      # Expose for local DB tools (TablePlus, DBeaver)
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - foliocv_pgdata_dev:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5

  app:
    build:
      context: .
      dockerfile: Dockerfile.dev
    container_name: foliocv_app_dev
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - .:/app                              # Mount source for live reload
      - foliocv_uploads_dev:/app/data/uploads
      - go_mod_cache:/go/pkg/mod            # Cache Go modules across rebuilds
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  foliocv_pgdata_dev:
  foliocv_uploads_dev:
  go_mod_cache:
```

### `.air.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/foliocv ."
  bin = "./tmp/foliocv"
  include_ext = ["go", "html", "css", "sql"]
  exclude_dir = ["tmp", "data", "static", "migrations"]
  delay = 500
  kill_delay = "0s"

[log]
  time = true

[color]
  main = "cyan"
  watcher = "blue"
  build = "yellow"
  runner = "green"

[misc]
  clean_on_exit = true
```

---

## Makefile

```makefile
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
```

---

## Environment Variables

### `.env.example`

```env
# ── Server ────────────────────────────────────────────────────────────────────
PORT=8080
BASE_URL=http://localhost:8080

# ── Security ──────────────────────────────────────────────────────────────────
# Generate with: openssl rand -hex 32
SECRET_KEY=replace-with-64-char-hex-string

# ── Database ──────────────────────────────────────────────────────────────────
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_DB=foliocv
POSTGRES_USER=foliocv
POSTGRES_PASSWORD=foliocv_dev_password
DATABASE_URL=postgres://foliocv:foliocv_dev_password@postgres:5432/foliocv?sslmode=disable

# ── Claude API ────────────────────────────────────────────────────────────────
# Get from: https://console.anthropic.com/
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-sonnet-4-6

# ── File Storage ──────────────────────────────────────────────────────────────
UPLOADS_DIR=/app/data/uploads
MAX_UPLOAD_BYTES=10485760

# ── App ───────────────────────────────────────────────────────────────────────
APP_ENV=development
APP_NAME=FolioCV
```

### `.gitignore`

```gitignore
.env
data/
tmp/
*.db
uploads/
__debug_bin
```

---

## Database Migrations

All SQL files live in `migrations/`. The app runs them automatically on startup using `golang-migrate`. Never edit a migration file after it has been committed — always create a new one.

### `migrations/000001_create_users.up.sql`

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT UNIQUE NOT NULL,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
```

### `migrations/000001_create_users.down.sql`

```sql
DROP TABLE IF EXISTS users;
```

### `migrations/000002_create_sessions.up.sql`

```sql
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### `migrations/000002_create_sessions.down.sql`

```sql
DROP TABLE IF EXISTS sessions;
```

### `migrations/000003_create_portfolios.up.sql`

```sql
CREATE TYPE career_type AS ENUM (
    'developer', 'designer', 'creative', 'corporate',
    'academic', 'healthcare', 'education', 'hospitality',
    'legal', 'finance', 'marketing', 'general'
);

CREATE TYPE portfolio_theme AS ENUM (
    'professional', 'creative', 'minimal'
);

CREATE TABLE portfolios (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug            TEXT UNIQUE NOT NULL,
    full_name       TEXT NOT NULL,
    headline        TEXT,
    summary         TEXT,
    email           TEXT,
    phone           TEXT,
    location        TEXT,
    linkedin_url    TEXT,
    github_url      TEXT,
    website_url     TEXT,
    avatar_url      TEXT,
    career_type     career_type NOT NULL DEFAULT 'general',
    theme           portfolio_theme NOT NULL DEFAULT 'professional',
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    raw_json        JSONB,
    cv_filename     TEXT,
    cv_parsed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolios_user_id ON portfolios(user_id);
CREATE INDEX idx_portfolios_slug ON portfolios(slug);
```

### `migrations/000003_create_portfolios.down.sql`

```sql
DROP TABLE IF EXISTS portfolios;
DROP TYPE IF EXISTS portfolio_theme;
DROP TYPE IF EXISTS career_type;
```

### `migrations/000004_create_sections.up.sql`

```sql
CREATE TYPE section_type AS ENUM (
    'experience', 'education', 'skills', 'projects',
    'certifications', 'awards', 'publications',
    'volunteer', 'languages', 'interests', 'custom'
);

CREATE TABLE sections (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    type         section_type NOT NULL,
    title        TEXT NOT NULL,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    is_visible   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sections_portfolio_id ON sections(portfolio_id);
CREATE INDEX idx_sections_sort_order ON sections(portfolio_id, sort_order);
```

### `migrations/000004_create_sections.down.sql`

```sql
DROP TABLE IF EXISTS sections;
DROP TYPE IF EXISTS section_type;
```

### `migrations/000005_create_section_items.up.sql`

```sql
CREATE TABLE section_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    section_id   UUID NOT NULL REFERENCES sections(id) ON DELETE CASCADE,
    sort_order   INTEGER NOT NULL DEFAULT 0,

    -- Common fields (used across section types)
    title        TEXT,           -- job title, degree name, skill category, project name
    subtitle     TEXT,           -- company, institution, issuer
    location     TEXT,
    start_date   TEXT,           -- free text: "Jan 2022", "2020", etc
    end_date     TEXT,           -- free text or "Present"
    description  TEXT,           -- main body text
    url          TEXT,           -- project link, certificate URL

    -- Structured extras stored as JSONB for flexibility
    -- skills: {"items": ["Go", "PostgreSQL", "Docker"]}
    -- bullets: {"items": ["Built X", "Achieved Y"]}
    -- tags: {"items": ["React", "TypeScript"]}
    meta         JSONB,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_section_items_section_id ON section_items(section_id);
CREATE INDEX idx_section_items_sort_order ON section_items(section_id, sort_order);
```

### `migrations/000005_create_section_items.down.sql`

```sql
DROP TABLE IF EXISTS section_items;
```

---

## `go.mod`

```
module foliocv

go 1.22

require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/golang-migrate/migrate/v4 v4.17.1
    github.com/google/uuid v1.6.0
    github.com/jackc/pgx/v5 v5.6.0
    github.com/joho/godotenv v1.5.1
    golang.org/x/crypto v0.23.0
)
```

Run `go mod tidy` inside the dev container after creating this file.

---

## `db/db.go` — Full Implementation

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/jackc/pgx/v5/stdlib"
)

// Connect creates a pgxpool connection and runs all pending migrations.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        return nil, fmt.Errorf("DATABASE_URL is not set")
    }

    pool, err := pgxpool.New(ctx, dsn)
    if err != nil {
        return nil, fmt.Errorf("pgxpool.New: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("db ping failed: %w", err)
    }

    log.Println("Database connected")

    if err := runMigrations(dsn); err != nil {
        return nil, fmt.Errorf("migrations failed: %w", err)
    }

    return pool, nil
}

func runMigrations(dsn string) error {
    m, err := migrate.New("file://migrations", dsn)
    if err != nil {
        return fmt.Errorf("migrate.New: %w", err)
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migrate up: %w", err)
    }

    log.Println("Migrations applied")
    return nil
}
```

---

## `services/claude.go` — Claude API Client

This is the most important service file. It handles the entire Claude API interaction — uploading the PDF, sending the structured extraction prompt, and returning a typed `ResumeData` struct.

```go
package services

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "time"
)

// ClaudeClient handles all Claude API interactions.
type ClaudeClient struct {
    apiKey  string
    model   string
    client  *http.Client
    baseURL string
}

func NewClaudeClient() *ClaudeClient {
    return &ClaudeClient{
        apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
        model:   os.Getenv("ANTHROPIC_MODEL"),
        client:  &http.Client{Timeout: 120 * time.Second},
        baseURL: "https://api.anthropic.com/v1",
    }
}

// ParseCV sends a PDF file to Claude and returns structured resume data.
// fileBytes: raw PDF bytes
// mimeType: "application/pdf" or "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
func (c *ClaudeClient) ParseCV(ctx context.Context, fileBytes []byte, mimeType string) (*ResumeData, error) {
    encoded := base64.StdEncoding.EncodeToString(fileBytes)

    prompt := buildExtractionPrompt()

    requestBody := map[string]any{
        "model":      c.model,
        "max_tokens": 8192,
        "messages": []map[string]any{
            {
                "role": "user",
                "content": []map[string]any{
                    {
                        "type": "document",
                        "source": map[string]any{
                            "type":       "base64",
                            "media_type": mimeType,
                            "data":       encoded,
                        },
                    },
                    {
                        "type": "text",
                        "text": prompt,
                    },
                },
            },
        },
        "system": systemPrompt(),
    }

    bodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/messages", bytes.NewReader(bodyBytes))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    req.Header.Set("anthropic-beta", "pdfs-2024-09-25")

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("api request: %w", err)
    }
    defer resp.Body.Close()

    respBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("claude api error %d: %s", resp.StatusCode, string(respBytes))
    }

    // Parse Claude's response envelope
    var claudeResp struct {
        Content []struct {
            Type string `json:"type"`
            Text string `json:"text"`
        } `json:"content"`
    }
    if err := json.Unmarshal(respBytes, &claudeResp); err != nil {
        return nil, fmt.Errorf("unmarshal claude response: %w", err)
    }

    if len(claudeResp.Content) == 0 {
        return nil, fmt.Errorf("empty response from claude")
    }

    // Extract the JSON from the text response
    rawJSON := claudeResp.Content[0].Text

    var resume ResumeData
    if err := json.Unmarshal([]byte(rawJSON), &resume); err != nil {
        return nil, fmt.Errorf("unmarshal resume JSON: %w\nraw: %s", err, rawJSON)
    }

    return &resume, nil
}

func systemPrompt() string {
    return `You are a precise CV/resume parser. Your job is to extract structured data from any CV or resume document, regardless of format, language, layout, or profession. You always return valid JSON matching the exact schema requested. You never add commentary, explanations, or markdown formatting. You return only the JSON object.`
}

func buildExtractionPrompt() string {
    return `Extract all information from this CV/resume and return it as a single JSON object matching this exact schema. Include every section that exists in the document, even if not listed below. For missing fields, use null or empty arrays — never omit keys.

Return ONLY valid JSON, no explanation, no markdown, no code blocks.

{
  "full_name": "string",
  "headline": "string — current job title or professional tagline",
  "summary": "string — professional summary or bio, full text",
  "email": "string or null",
  "phone": "string or null",
  "location": "string or null — city, country",
  "linkedin_url": "string or null",
  "github_url": "string or null",
  "website_url": "string or null",
  "career_type": "one of: developer, designer, creative, corporate, academic, healthcare, education, hospitality, legal, finance, marketing, general",
  "suggested_theme": "one of: professional, creative, minimal",
  "experience": [
    {
      "title": "string — job title",
      "company": "string",
      "location": "string or null",
      "start_date": "string e.g. Jan 2020",
      "end_date": "string e.g. Dec 2023 or Present",
      "description": "string or null — full description paragraph if present",
      "bullets": ["string", "string"],
      "url": "string or null"
    }
  ],
  "education": [
    {
      "degree": "string e.g. BSc Computer Science",
      "institution": "string",
      "location": "string or null",
      "start_date": "string or null",
      "end_date": "string e.g. 2022",
      "description": "string or null",
      "grade": "string or null"
    }
  ],
  "skills": [
    {
      "category": "string e.g. Programming Languages, Tools, Soft Skills",
      "items": ["string", "string"]
    }
  ],
  "projects": [
    {
      "name": "string",
      "description": "string",
      "url": "string or null",
      "start_date": "string or null",
      "end_date": "string or null",
      "technologies": ["string"],
      "bullets": ["string"]
    }
  ],
  "certifications": [
    {
      "name": "string",
      "issuer": "string or null",
      "date": "string or null",
      "url": "string or null",
      "credential_id": "string or null"
    }
  ],
  "awards": [
    {
      "title": "string",
      "issuer": "string or null",
      "date": "string or null",
      "description": "string or null"
    }
  ],
  "publications": [
    {
      "title": "string",
      "publisher": "string or null",
      "date": "string or null",
      "url": "string or null",
      "description": "string or null"
    }
  ],
  "volunteer": [
    {
      "role": "string",
      "organization": "string",
      "start_date": "string or null",
      "end_date": "string or null",
      "description": "string or null"
    }
  ],
  "languages": [
    {
      "language": "string",
      "proficiency": "string e.g. Native, Fluent, Intermediate"
    }
  ],
  "interests": ["string"],
  "custom_sections": [
    {
      "title": "string — the section heading as it appears in the CV",
      "items": [
        {
          "title": "string or null",
          "description": "string or null"
        }
      ]
    }
  ]
}`
}
```

---

## `services/parser.go` — ResumeData Struct + DB Mapper

```go
package services

import "foliocv/models"

// ResumeData is the typed Go struct that mirrors the Claude extraction JSON schema.
// Every field maps directly to what Claude returns.
type ResumeData struct {
    FullName      string             `json:"full_name"`
    Headline      string             `json:"headline"`
    Summary       string             `json:"summary"`
    Email         *string            `json:"email"`
    Phone         *string            `json:"phone"`
    Location      *string            `json:"location"`
    LinkedInURL   *string            `json:"linkedin_url"`
    GithubURL     *string            `json:"github_url"`
    WebsiteURL    *string            `json:"website_url"`
    CareerType    string             `json:"career_type"`
    SuggestedTheme string            `json:"suggested_theme"`
    Experience    []ExperienceItem   `json:"experience"`
    Education     []EducationItem    `json:"education"`
    Skills        []SkillGroup       `json:"skills"`
    Projects      []ProjectItem      `json:"projects"`
    Certifications []CertItem        `json:"certifications"`
    Awards        []AwardItem        `json:"awards"`
    Publications  []PublicationItem  `json:"publications"`
    Volunteer     []VolunteerItem    `json:"volunteer"`
    Languages     []LanguageItem     `json:"languages"`
    Interests     []string           `json:"interests"`
    CustomSections []CustomSection   `json:"custom_sections"`
}

type ExperienceItem struct {
    Title       string   `json:"title"`
    Company     string   `json:"company"`
    Location    *string  `json:"location"`
    StartDate   *string  `json:"start_date"`
    EndDate     *string  `json:"end_date"`
    Description *string  `json:"description"`
    Bullets     []string `json:"bullets"`
    URL         *string  `json:"url"`
}

type EducationItem struct {
    Degree      string  `json:"degree"`
    Institution string  `json:"institution"`
    Location    *string `json:"location"`
    StartDate   *string `json:"start_date"`
    EndDate     *string `json:"end_date"`
    Description *string `json:"description"`
    Grade       *string `json:"grade"`
}

type SkillGroup struct {
    Category string   `json:"category"`
    Items    []string `json:"items"`
}

type ProjectItem struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    URL          *string  `json:"url"`
    StartDate    *string  `json:"start_date"`
    EndDate      *string  `json:"end_date"`
    Technologies []string `json:"technologies"`
    Bullets      []string `json:"bullets"`
}

type CertItem struct {
    Name         string  `json:"name"`
    Issuer       *string `json:"issuer"`
    Date         *string `json:"date"`
    URL          *string `json:"url"`
    CredentialID *string `json:"credential_id"`
}

type AwardItem struct {
    Title       string  `json:"title"`
    Issuer      *string `json:"issuer"`
    Date        *string `json:"date"`
    Description *string `json:"description"`
}

type PublicationItem struct {
    Title       string  `json:"title"`
    Publisher   *string `json:"publisher"`
    Date        *string `json:"date"`
    URL         *string `json:"url"`
    Description *string `json:"description"`
}

type VolunteerItem struct {
    Role         string  `json:"role"`
    Organization string  `json:"organization"`
    StartDate    *string `json:"start_date"`
    EndDate      *string `json:"end_date"`
    Description  *string `json:"description"`
}

type LanguageItem struct {
    Language    string `json:"language"`
    Proficiency string `json:"proficiency"`
}

type CustomSection struct {
    Title string `json:"title"`
    Items []struct {
        Title       *string `json:"title"`
        Description *string `json:"description"`
    } `json:"items"`
}

// SectionOrder defines the default display order for each career type.
// Sections not listed here appear at the end in the order Claude returned them.
var DefaultSectionOrder = map[string][]string{
    "developer": {"experience", "skills", "projects", "education", "certifications", "awards"},
    "designer":  {"experience", "projects", "skills", "education", "certifications", "awards"},
    "creative":  {"experience", "projects", "awards", "skills", "education"},
    "corporate": {"experience", "education", "skills", "certifications", "awards", "publications"},
    "academic":  {"education", "publications", "experience", "awards", "certifications"},
    "healthcare":{"experience", "education", "certifications", "skills", "awards"},
    "education": {"experience", "education", "certifications", "publications", "skills"},
    "marketing": {"experience", "projects", "skills", "education", "certifications", "awards"},
    "finance":   {"experience", "education", "certifications", "skills", "awards"},
    "legal":     {"experience", "education", "publications", "awards", "certifications"},
    "general":   {"experience", "education", "skills", "projects", "certifications", "awards"},
}
```

---

## Implementation Steps — In Strict Order

### Step 1 — Bootstrap & Docker

```
1.  Create all directories from project structure
2.  Write Dockerfile, Dockerfile.dev
3.  Write docker-compose.yml, docker-compose.dev.yml
4.  Write .air.toml, .gitignore, .env.example
5.  Write Makefile
6.  Write go.mod (module: foliocv)
7.  Run: make setup → creates .env
8.  Fill in: ANTHROPIC_API_KEY, SECRET_KEY (openssl rand -hex 32),
    POSTGRES_PASSWORD (any string for dev)
9.  Write db/db.go (Connect + runMigrations)
10. Write all migration SQL files
11. Write main.go skeleton — just env load + db connect + log.Fatal if fail + /health endpoint
12. Run: make dev
    → Postgres starts first (healthcheck)
    → App starts, connects to Postgres, runs migrations
    → "Migrations applied" in logs
    → GET http://localhost:8080/health → 200 OK
13. Run: make psql → \dt → should show 5 tables
```

### Step 2 — Auth

```
14. Write models/user.go:
    Structs: User{ID, Email, Name, PasswordHash, CreatedAt, UpdatedAt}
             Session{ID, UserID, ExpiresAt, CreatedAt}
    Methods:
      CreateUser(ctx, pool, email, name, hash) (*User, error)
      FindUserByEmail(ctx, pool, email) (*User, error)
      FindUserByID(ctx, pool, id) (*User, error)
      CreateSession(ctx, pool, userID, expiresAt) (*Session, error)
      FindSession(ctx, pool, id) (*Session, error)
      DeleteSession(ctx, pool, id) error

15. Write middleware/auth.go — RequireAuth(pool):
    - Read session_id cookie
    - FindSession → check ExpiresAt
    - FindUserByID
    - Attach user to context via context.WithValue
    - Redirect /login if invalid

16. Write middleware/logger.go — simple request logger

17. Write handlers/auth.go:
    Register: GET /register → form | POST /register → bcrypt(12) → CreateUser → CreateSession → cookie → redirect /dashboard
    Login:    GET /login → form | POST /login → FindUserByEmail → bcrypt compare → CreateSession → cookie → redirect /dashboard
    Logout:   POST /logout → DeleteSession → clear cookie → redirect /login

18. Write templates/base.html (editor layout — authenticated pages)
19. Write templates/landing.html
20. Write templates/auth/login.html
21. Write templates/auth/register.html
22. Write templates/404.html
23. Wire routes in main.go

24. Test: register → login → /dashboard returns 404 (fine) → logout → redirected to /login
    Run: make psql → SELECT * FROM users; → row exists with bcrypt hash
```

### Step 3 — CV Upload + Claude Parsing

```
25. Write services/claude.go (full implementation above)
26. Write services/parser.go (ResumeData structs + DefaultSectionOrder)

27. Write models/portfolio.go:
    Struct: Portfolio{ID, UserID, Slug, FullName, Headline, Summary,
                      Email, Phone, Location, LinkedInURL, GithubURL, WebsiteURL,
                      CareerType, Theme, IsPublic, RawJSON, CVFilename, CVParsedAt,
                      CreatedAt, UpdatedAt}
    Methods:
      CreatePortfolio(ctx, pool, userID, slug, data ResumeData) (*Portfolio, error)
      FindPortfolioByUserID(ctx, pool, userID) (*Portfolio, error)
      FindPortfolioBySlug(ctx, pool, slug) (*Portfolio, error)
      UpdatePortfolioField(ctx, pool, id, field, value) error  -- generic single-field update
      UpdateTheme(ctx, pool, id, theme) error

28. Write models/section.go:
    Struct: Section{ID, PortfolioID, Type, Title, SortOrder, IsVisible, CreatedAt, UpdatedAt}
    Methods:
      CreateSection(ctx, pool, portfolioID, stype, title, order) (*Section, error)
      ListSectionsByPortfolio(ctx, pool, portfolioID) ([]Section, error)
      UpdateSectionOrder(ctx, pool, portfolioID, orderedIDs []string) error
        -- UPDATE sections SET sort_order = $1 WHERE id = $2 AND portfolio_id = $3
        -- Run in a transaction, loop over orderedIDs with index as sort_order
      ToggleSectionVisibility(ctx, pool, id, portfolioID) error
      UpdateSectionTitle(ctx, pool, id, portfolioID, title) error

29. Write models/item.go:
    Struct: SectionItem{ID, SectionID, SortOrder, Title, Subtitle, Location,
                        StartDate, EndDate, Description, URL, Meta, CreatedAt, UpdatedAt}
    Methods:
      CreateItem(ctx, pool, sectionID, item SectionItem) (*SectionItem, error)
      ListItemsBySection(ctx, pool, sectionID) ([]SectionItem, error)
      UpdateItem(ctx, pool, item SectionItem) error
      DeleteItem(ctx, pool, id, sectionID) error
      UpdateItemOrder(ctx, pool, sectionID, orderedIDs []string) error

30. Write handlers/upload.go:
    GET /upload → render upload/index.html
    POST /upload:
      1. Parse multipart form (max 10MB)
      2. Accept only: application/pdf,
         application/vnd.openxmlformats-officedocument.wordprocessingml.document
      3. Validate file size vs MAX_UPLOAD_BYTES
      4. Read file bytes into memory
      5. Save original file to UPLOADS_DIR/{user_id}_{filename}
      6. Set portfolio processing status in session or temporary DB flag
      7. Return HTMX redirect to GET /upload/processing

    GET /upload/processing → render upload/processing.html
      - Page polls GET /upload/status every 3s via HTMX

    GET /upload/status → HTMX partial:
      - Check if portfolio exists for user with cv_parsed_at NOT NULL
      - If ready: return HX-Redirect header to /dashboard
      - If not ready: return "still processing" spinner HTML

    Background goroutine triggered from POST /upload:
      1. Call claude.ParseCV(ctx, fileBytes, mimeType) → ResumeData
      2. Generate slug: slugify(resume.FullName) + "-" + randomString(6)
         e.g. "enock-waweru-x7k2m3"
      3. Create portfolio record
      4. Create sections + items from ResumeData (see Populate Portfolio below)
      5. Update portfolio.cv_parsed_at = now

31. Write populatePortfolio(ctx, pool, portfolioID, resume ResumeData):
    This function creates all sections and items from the parsed resume data.
    Order sections by DefaultSectionOrder[resume.CareerType].

    For each section type that has data:
      a. CreateSection(portfolioID, type, title, sortOrder)
      b. For each item in that section's data:
         - Map ResumeData fields to SectionItem fields
         - Store bullets/tags/technologies as Meta JSONB: {"items": [...]}
         - CreateItem(sectionID, item)

    Section → Item mapping:
    experience → title=JobTitle, subtitle=Company, location, start_date, end_date, description, meta={"bullets":[...]}
    education  → title=Degree, subtitle=Institution, location, start_date, end_date, description, meta={"grade":"..."}
    skills     → title=Category, meta={"items":["Go","PostgreSQL",...]}
    projects   → title=Name, description, url, start_date, end_date, meta={"technologies":[...],"bullets":[...]}
    certifications → title=Name, subtitle=Issuer, start_date=Date, url, meta={"credential_id":"..."}
    awards     → title, subtitle=Issuer, start_date=Date, description
    publications → title, subtitle=Publisher, start_date=Date, url, description
    volunteer  → title=Role, subtitle=Organization, start_date, end_date, description
    languages  → title=Language, subtitle=Proficiency
    interests  → one item per interest, title=interest string
    custom     → title=SectionTitle, items mapped as title+description

32. Test full upload flow:
    - Upload a real PDF CV
    - Check logs for "Claude API call started" and "Claude API call complete"
    - GET /upload/status should eventually redirect to /dashboard
    - make psql → SELECT * FROM portfolios; → should show 1 row
    - SELECT * FROM sections; → should show multiple sections
    - SELECT * FROM section_items LIMIT 10; → should show items
```

### Step 4 — Portfolio Editor (Dashboard)

```
33. Write handlers/portfolio.go:
    GET /dashboard:
      - Require auth
      - FindPortfolioByUserID
      - If no portfolio → redirect /upload
      - ListSectionsByPortfolio (ordered by sort_order)
      - For each section: ListItemsBySection
      - Render dashboard/index.html with full data

34. Write templates/dashboard/index.html:
    Key elements:
    - Portfolio header (editable inline: name, headline, summary, links)
    - Theme picker (3 buttons: Professional / Creative / Minimal)
    - Drag-and-drop sections list (.sortable class, SortableJS initialized)
    - "View public portfolio" link → /p/{slug} (opens in new tab)
    - "Re-upload CV" link → /upload

    SortableJS integration:
    <script src="https://cdn.jsdelivr.net/npm/sortablejs@latest/Sortable.min.js"></script>
    <form class="sortable"
          hx-post="/dashboard/sections/reorder"
          hx-trigger="end"
          hx-swap="none">
      {{range .Sections}}
        <div data-id="{{.ID}}">
          <input type="hidden" name="id" value="{{.ID}}">
          {{template "section_card" .}}
        </div>
      {{end}}
    </form>

35. Write templates/editor/section_card.html:
    Each card shows:
    - Drag handle (cursor: grab icon)
    - Section title (inline-editable via HTMX click-to-edit)
    - Toggle visibility button (eye icon, HTMX POST /dashboard/sections/{id}/toggle)
    - Expand button → loads section_editor.html partial via hx-get

36. Write handlers/section.go:
    POST /dashboard/sections/reorder:
      - Read []id from form values (order of hidden inputs after drag)
      - UpdateSectionOrder(portfolioID, ids)
      - Return 200 (hx-swap="none" — no HTML update needed)

    POST /dashboard/sections/{id}/toggle:
      - ToggleSectionVisibility
      - Return updated section_card.html partial

    GET /dashboard/sections/{id}/edit:
      - ListItemsBySection
      - Return section_editor.html partial (hx-swap="innerHTML" on card body)

    POST /dashboard/sections/{id}/title:
      - UpdateSectionTitle
      - Return updated title text

37. Write templates/editor/section_editor.html:
    Shows all items in the section as draggable rows.
    Each item row: title | subtitle | dates | edit button | delete button
    "Add item" button → loads item_form.html partial inline
    Items are also SortableJS draggable → POST /dashboard/sections/{id}/items/reorder

38. Write handlers/item.go:
    GET  /dashboard/sections/{sectionID}/items/new → item_form.html (blank)
    POST /dashboard/sections/{sectionID}/items → CreateItem → return item_row.html partial
    GET  /dashboard/sections/{sectionID}/items/{id}/edit → item_form.html (prefilled)
    PUT  /dashboard/sections/{sectionID}/items/{id} → UpdateItem → return item_row.html partial
    DELETE /dashboard/sections/{sectionID}/items/{id}:
      - DeleteItem
      - Return empty 200 with HX-Trigger to remove the row
    POST /dashboard/sections/{sectionID}/items/reorder → UpdateItemOrder → 200

39. Write templates/editor/item_row.html (partial — one item in edit list)
40. Write templates/editor/item_form.html (partial — add/edit form, fields vary by section type)

    item_form.html logic:
    The form fields shown depend on section type, passed as template data.
    experience → title (job title), subtitle (company), location, start_date, end_date, description, bullets textarea
    education  → title (degree), subtitle (institution), location, start_date, end_date, grade
    skills     → title (category), items (comma-separated or tag input)
    projects   → title (name), description, url, technologies (comma-separated), bullets textarea
    certifications → title (name), subtitle (issuer), start_date (date), url, credential_id
    awards, publications, volunteer, languages, interests, custom → appropriate subset
```

### Step 5 — Theme Switcher

```
41. Write handlers/theme.go:
    POST /dashboard/theme:
      - Read theme from form: "professional" | "creative" | "minimal"
      - UpdateTheme(portfolioID, theme)
      - Return HTMX response:
        HX-Trigger: {"themeChanged": "professional"}
        plus updated theme_picker.html partial showing new active state

42. Write templates/editor/theme_picker.html:
    Three buttons, active state on current theme.
    Each POSTs to /dashboard/theme with hx-post, hx-swap="outerHTML", hx-target="#theme-picker"

43. Inline portfolio header editing:
    Each field (name, headline, summary, email, phone, location, linkedin, github, website)
    uses HTMX click-to-edit pattern:
    - Display: <span hx-get="/dashboard/profile/{field}/edit" hx-trigger="click" hx-target="this" hx-swap="outerHTML">{{value}}</span>
    - Edit: <input hx-put="/dashboard/profile/{field}" hx-trigger="blur, keyup[key=='Enter']" hx-target="this" hx-swap="outerHTML">
    Handler: PUT /dashboard/profile/{field} → UpdatePortfolioField → return updated <span>
```

### Step 6 — Public Portfolio Page

```
44. Write handlers/portfolio.go:
    GET /p/{slug} — public, no auth:
      - FindPortfolioBySlug
      - If not found or !IsPublic → 404
      - ListSectionsByPortfolio where IsVisible = true
      - For each visible section: ListItemsBySection
      - Switch on portfolio.Theme → render the matching template

45. Write templates/portfolio/professional.html:
    Design: Clean white background, serif headings, structured columns.
    Layout: Left sidebar (contact, skills, languages) | Right main (experience, education, projects, etc.)
    Color: Navy/charcoal text, thin ruled section dividers, subtle hover on links.
    Mobile: Single column, sidebar sections appear above main content.

46. Write templates/portfolio/creative.html:
    Design: Bold, expressive, asymmetric. Color accent (indigo or teal) on section headers.
    Layout: Full width, large name at top with colorful underline, sections as cards.
    Color: Dark background option, gradient name treatment, pill-style skill tags.
    Mobile: Cards stack cleanly, padding-heavy, touch-friendly.

47. Write templates/portfolio/minimal.html:
    Design: Ultra-clean, generous whitespace, system font, everything left-aligned.
    Layout: Single column, clear typographic hierarchy, no decorative elements.
    Color: Black and white only, blue for links, thin gray dividers.
    Mobile: Same layout, just narrower — minimal adapts naturally.

    All three templates share this structure:
    - Render portfolio.FullName, portfolio.Headline, portfolio.Summary
    - Render contact links (email, phone, location, LinkedIn, GitHub, website)
    - For each visible section (ordered by sort_order):
        - Render section.Title as a heading
        - For each item in section: render based on section.Type
          (use item.Meta JSONB for bullets, technologies, tags, items)
    - No nav, no login button, no editor chrome — clean and shareable

48. Test public page: /p/{your-slug} renders correctly with no auth
    Test across all three themes by switching in dashboard
    Test on mobile viewport
```

### Step 7 — Polish + Re-upload

```
49. Write re-upload flow (same as upload but merges, not overwrites):
    POST /upload with existing portfolio:
      - Parse new CV via Claude
      - Update portfolio header fields (name, headline, summary, contact info)
      - For each section in new parse:
          - If section type already exists: UPDATE items (add new ones, don't delete existing)
          - If section type is new: CREATE section + items
      - Update portfolio.cv_filename, portfolio.cv_parsed_at
      - Redirect to /dashboard

50. Add portfolio public/private toggle:
    POST /dashboard/visibility:
      - Toggle portfolio.IsPublic
      - If private: /p/{slug} returns 404
      - HTMX updates the toggle button state

51. Add copy-link button on dashboard:
    Show: BASE_URL + "/p/" + slug
    Button: "Copy link" → clipboard JS (2 lines, no HTMX needed)

52. Add section deletion:
    DELETE /dashboard/sections/{id}:
      - Delete section + all items (CASCADE handles items)
      - HTMX removes the section card from DOM

53. Add manual section creation:
    GET /dashboard/sections/new → modal/inline form to pick section type + title
    POST /dashboard/sections → CreateSection → return new section_card.html partial

54. Error handling:
    - Claude API timeout (120s) → show friendly error, allow retry
    - Claude returns invalid JSON → log raw response, show "parsing failed" with retry button
    - File too large → show size limit message before API call
    - Unsupported file type → reject at handler level
```

### Step 8 — Production Build

```
55. Run: make build → production images build cleanly
56. Run: make up → stack starts, /health returns 200
57. Test full flow in production mode: register → upload → portfolio → edit → public URL
58. Run: make down
```

---

## `main.go` — Full Bootstrap

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"
    "github.com/joho/godotenv"

    "foliocv/db"
    "foliocv/handlers"
    "foliocv/middleware"
    "foliocv/services"
)

func main() {
    _ = godotenv.Load()

    // Validate required env vars
    required := []string{"DATABASE_URL", "SECRET_KEY", "ANTHROPIC_API_KEY"}
    for _, key := range required {
        if os.Getenv(key) == "" {
            log.Fatalf("Required env var %s is not set", key)
        }
    }

    ctx := context.Background()

    // Connect Postgres + run migrations
    pool, err := db.Connect(ctx)
    if err != nil {
        log.Fatalf("Database setup failed: %v", err)
    }
    defer pool.Close()

    // Ensure uploads dir
    uploadsDir := os.Getenv("UPLOADS_DIR")
    if uploadsDir == "" {
        uploadsDir = "./data/uploads"
    }
    if err := os.MkdirAll(uploadsDir, 0755); err != nil {
        log.Fatalf("Failed to create uploads dir: %v", err)
    }

    // Init services
    claude := services.NewClaudeClient()

    // Init handlers
    authHandler := handlers.NewAuthHandler(pool)
    uploadHandler := handlers.NewUploadHandler(pool, claude)
    portfolioHandler := handlers.NewPortfolioHandler(pool)
    sectionHandler := handlers.NewSectionHandler(pool)
    itemHandler := handlers.NewItemHandler(pool)
    themeHandler := handlers.NewThemeHandler(pool)

    // Router
    r := chi.NewRouter()
    r.Use(chimiddleware.Logger)
    r.Use(chimiddleware.Recoverer)
    r.Use(chimiddleware.RealIP)
    r.Use(chimiddleware.Timeout(30 * time.Second))

    // Static files
    r.Handle("/static/*", http.StripPrefix("/static/",
        http.FileServer(http.Dir("static"))))

    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })

    // Public routes
    r.Get("/", handlers.LandingHandler)
    r.Get("/login", authHandler.LoginPage)
    r.Post("/login", authHandler.Login)
    r.Get("/register", authHandler.RegisterPage)
    r.Post("/register", authHandler.Register)
    r.Post("/logout", authHandler.Logout)

    // Public portfolio view
    r.Get("/p/{slug}", portfolioHandler.PublicView)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(middleware.RequireAuth(pool))

        // Dashboard
        r.Get("/dashboard", portfolioHandler.Dashboard)

        // Upload
        r.Get("/upload", uploadHandler.Page)
        r.Post("/upload", uploadHandler.Handle)
        r.Get("/upload/processing", uploadHandler.ProcessingPage)
        r.Get("/upload/status", uploadHandler.Status)

        // Portfolio settings
        r.Put("/dashboard/profile/{field}", portfolioHandler.UpdateField)
        r.Post("/dashboard/visibility", portfolioHandler.ToggleVisibility)
        r.Post("/dashboard/theme", themeHandler.Switch)

        // Sections
        r.Get("/dashboard/sections/new", sectionHandler.NewPage)
        r.Post("/dashboard/sections", sectionHandler.Create)
        r.Post("/dashboard/sections/reorder", sectionHandler.Reorder)
        r.Get("/dashboard/sections/{id}/edit", sectionHandler.Edit)
        r.Post("/dashboard/sections/{id}/title", sectionHandler.UpdateTitle)
        r.Post("/dashboard/sections/{id}/toggle", sectionHandler.Toggle)
        r.Delete("/dashboard/sections/{id}", sectionHandler.Delete)

        // Items
        r.Get("/dashboard/sections/{sectionID}/items/new", itemHandler.NewForm)
        r.Post("/dashboard/sections/{sectionID}/items", itemHandler.Create)
        r.Post("/dashboard/sections/{sectionID}/items/reorder", itemHandler.Reorder)
        r.Get("/dashboard/sections/{sectionID}/items/{id}/edit", itemHandler.EditForm)
        r.Put("/dashboard/sections/{sectionID}/items/{id}", itemHandler.Update)
        r.Delete("/dashboard/sections/{sectionID}/items/{id}", itemHandler.Delete)
    })

    // Start server with graceful shutdown
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    srv := &http.Server{
        Addr:         ":" + port,
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 150 * time.Second, // Long for Claude API calls
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        log.Printf("FolioCV running on :%s", port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down...")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    srv.Shutdown(shutdownCtx)
}
```

---

## UI Design System

### Color Palette (Tailwind config — applied via CDN script config)

```javascript
tailwind.config = {
  theme: {
    extend: {
      colors: {
        // Editor UI (dashboard) — dark
        editor: {
          bg:      '#0d0d0d',
          surface: '#171717',
          card:    '#1f1f1f',
          border:  '#2d2d2d',
          muted:   '#404040',
        },
        // Accent
        brand: '#6366f1',       // indigo
        success: '#22c55e',
        warn: '#f59e0b',
        danger: '#ef4444',
      }
    }
  }
}
```

### Three Portfolio Themes (Public Pages)

**Professional** — used for: corporate, finance, legal, healthcare, education
- White background, `font-family: Georgia, serif` for headings
- Navy `#1e3a5f` headings, charcoal `#374151` body
- Two-column layout (30% sidebar, 70% main)
- Thin `border-b border-gray-200` section dividers
- Skill pills: `bg-gray-100 text-gray-700 rounded px-2 py-0.5 text-sm`

**Creative** — used for: designer, creative, marketing
- Off-white `#fafaf9` background
- Bold accent color `#6366f1` on section titles
- Full-width single column, large name with gradient underline
- Project cards with colored left border
- Skill pills: `bg-indigo-50 text-indigo-700 rounded-full px-3 py-1`

**Minimal** — used for: developer, academic, general
- Pure white background, system font stack
- Black headings, `#6b7280` muted text
- Single column, strict left alignment
- No decorative elements, only typographic hierarchy
- Skill pills: `border border-gray-300 text-gray-600 rounded px-2 py-0.5 text-xs`

### HTMX Patterns Used

```
Click-to-edit inline field:
  Display: hx-get="/dashboard/profile/{field}/edit" hx-trigger="click" hx-swap="outerHTML" hx-target="this"
  Input:   hx-put="/dashboard/profile/{field}" hx-trigger="blur, keyup[key=='Enter']" hx-target="this" hx-swap="outerHTML"

Section drag-and-drop reorder:
  SortableJS end event → hx-post="/dashboard/sections/reorder" hx-trigger="end" hx-swap="none"
  Hidden inputs inside each draggable div carry the ID

Item add (inline, no page reload):
  hx-get="/dashboard/sections/{id}/items/new" hx-target="#items-{id}" hx-swap="beforeend"

Item delete:
  hx-delete="/dashboard/sections/{sectionID}/items/{id}"
  hx-target="closest .item-row" hx-swap="outerHTML"
  hx-confirm="Remove this item?"

Upload processing poll:
  hx-get="/upload/status" hx-trigger="every 3s" hx-swap="outerHTML" hx-target="#status-container"

Theme switch:
  hx-post="/dashboard/theme" hx-swap="outerHTML" hx-target="#theme-picker"
```

---

## Handling the Async Parse (Critical Detail)

The Claude API call takes 5-30 seconds depending on CV length. The upload handler must not block the HTTP response waiting for it. Here is the correct pattern:

```go
func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // ... validate file, save to disk ...

    // Store parse-in-progress flag in DB
    // Simple approach: set portfolio.cv_parsed_at = NULL, portfolio exists = true
    // Create a skeleton portfolio record immediately (with empty fields)
    skeletonID := uuid.New().String()
    h.db.Exec(ctx, `
        INSERT INTO portfolios (id, user_id, slug, full_name, career_type, theme)
        VALUES ($1, $2, $3, $4, 'general', 'professional')
    `, skeletonID, userID, "processing-"+skeletonID[:8], "Processing...")

    // Store portfolio ID in session so /upload/status can check it
    setSessionValue(w, r, "pending_portfolio_id", skeletonID)

    // Fire Claude call in goroutine
    go func() {
        ctx := context.Background() // Fresh context — request context will be cancelled
        resume, err := h.claude.ParseCV(ctx, fileBytes, mimeType)
        if err != nil {
            log.Printf("Claude parse failed: %v", err)
            // Update portfolio with error state
            h.db.Exec(ctx, `UPDATE portfolios SET full_name = 'Parse failed' WHERE id = $1`, skeletonID)
            return
        }
        // Populate all sections and items
        populatePortfolio(ctx, h.db, skeletonID, *resume)
        // Mark as done
        now := time.Now()
        h.db.Exec(ctx, `
            UPDATE portfolios SET full_name=$1, headline=$2, summary=$3,
            career_type=$4, theme=$5, cv_parsed_at=$6, slug=$7 WHERE id=$8
        `, resume.FullName, resume.Headline, resume.Summary,
           resume.CareerType, resume.SuggestedTheme, now,
           generateSlug(resume.FullName), skeletonID)
    }()

    // Redirect immediately — don't wait
    http.Redirect(w, r, "/upload/processing", http.StatusFound)
}
```

The `/upload/status` HTMX polling endpoint checks if `cv_parsed_at IS NOT NULL` for the pending portfolio. When it is, it returns `HX-Redirect: /dashboard`.

---

## Slug Generation

```go
import (
    "crypto/rand"
    "encoding/hex"
    "regexp"
    "strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
    s = strings.ToLower(strings.TrimSpace(s))
    s = nonAlphaNum.ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    if len(s) > 40 {
        s = s[:40]
    }
    return s
}

func generateSlug(fullName string) string {
    b := make([]byte, 4)
    rand.Read(b)
    suffix := hex.EncodeToString(b)[:6]
    return slugify(fullName) + "-" + suffix
}
// e.g. "enock-waweru-a3f9c2"
```

---

## Template Helper Functions

Register these via `template.FuncMap` in main.go before parsing templates:

```go
funcMap := template.FuncMap{
    "lower":       strings.ToLower,
    "upper":       strings.ToUpper,
    "title":       strings.Title,
    "timeAgo":     timeAgo,          // "2 minutes ago", "3 days ago"
    "formatDate":  formatDate,       // "Jan 2020" → "January 2020" (optional)
    "joinStrings": strings.Join,
    "deref":       func(s *string) string { if s == nil { return "" }; return *s },
    "isNil":       func(s *string) bool { return s == nil },
    "safeURL":     func(s string) template.URL { return template.URL(s) },
    "add":         func(a, b int) int { return a + b },
    "json":        func(v any) string { b, _ := json.Marshal(v); return string(b) },
}
```

---

## What NOT to Build in MVP

- No email verification or password reset
- No custom domain for /p/{slug}
- No portfolio analytics (who viewed, how long) — that's PeekCV's job
- No PDF export of the portfolio
- No multiple portfolios per user (one per user for MVP)
- No image upload for portfolio avatar (URL link only)
- No rich text editor (plain textarea for descriptions)
- No portfolio templates marketplace
- No team/agency accounts
- No social sharing meta tags (add in v2)
- No contact form on public page (add in v2)

---

## Definition of Done

Every item must pass before the project is considered complete:

- [ ] `make setup` creates `.env` from `.env.example`
- [ ] `make dev` starts Postgres + App with live reload in under 60 seconds
- [ ] `make psql` connects to Postgres and shows all 5 tables
- [ ] `make build && make up` starts the production stack
- [ ] `make clean` removes all containers and volumes
- [ ] `/health` returns 200 OK
- [ ] User can register and log in
- [ ] User can upload a PDF CV (max 10MB)
- [ ] Claude API parses the CV into structured data within 30 seconds
- [ ] Processing page polls and auto-redirects to dashboard when done
- [ ] Dashboard shows all extracted sections and items in correct order
- [ ] User can drag sections to reorder — new order persists after page reload
- [ ] User can click any item to edit it inline
- [ ] User can add new items to any section
- [ ] User can delete items and sections
- [ ] User can toggle section visibility (hide from public page)
- [ ] Theme switcher changes public portfolio appearance instantly
- [ ] Public page `/p/{slug}` renders with correct theme, no auth required
- [ ] All three themes render correctly on mobile (responsive)
- [ ] Re-uploading a new CV merges data without destroying edits
- [ ] Uploading a non-PDF/DOCX file is rejected with a clear error
- [ ] Claude API errors are caught and shown as user-friendly messages
- [ ] All data persists across container restarts (Postgres volume)
- [ ] No hardcoded secrets — all config via `.env`
- [ ] App starts cleanly with just `make setup && make dev`