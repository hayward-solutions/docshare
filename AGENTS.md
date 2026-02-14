# PROJECT KNOWLEDGE BASE

**Generated:** 2026-02-14
**Commit:** 98c0d84
**Branch:** main

## OVERVIEW

DocShare: document sharing platform with file storage, sharing, groups, previews. Monorepo (Go backend + Next.js frontend + Go CLI).

## STRUCTURE

```
./
├── backend/          # Go Fiber REST API
│   ├── cmd/server/   # Entry point
│   ├── internal/     # Handlers, models, services, middleware
│   └── pkg/          # Utilities (logger, utils)
├── frontend/         # Next.js 16 App
│   └── src/
│       ├── app/      # Routes (App Router)
│       ├── components/ui/  # shadcn/ui
│       ├── lib/     # Utilities
│       └── hooks/   # React hooks
├── cli/              # Go CLI (docshare)
│   ├── cmd/          # Commands
│   └── internal/    # API client, config
├── docs/             # API, CLI, deployment docs
└── charts/           # Helm charts
```

## WHERE TO LOOK

| Task | Location |
|------|----------|
| Backend API handlers | `backend/internal/handlers/` |
| Data models | `backend/internal/models/` |
| Auth middleware | `backend/internal/middleware/auth.go` |
| Frontend routes | `frontend/src/app/` |
| UI components | `frontend/src/components/ui/` |
| CLI commands | `cli/cmd/` |

## CODE MAP

| Component | Type | Location |
|-----------|------|----------|
| Fiber server | main | `backend/cmd/server/main.go` |
| Auth handlers | handler | `backend/internal/handlers/auth.go` |
| File handlers | handler | `backend/internal/handlers/files.go` |
| User model | model | `backend/internal/models/user.go` |
| File model | model | `backend/internal/models/file.go` |
| Root layout | page | `frontend/src/app/layout.tsx` |
| Dashboard | route | `frontend/src/app/(dashboard)/files/` |

## CONVENTIONS

- **Go**: `cmd/<name>/main.go` entry, `internal/` for private, `pkg/` for public utils
- **Frontend**: Next.js App Router in `src/app/`, route groups `(dashboard)/`, `(public)/`
- **Tests**: Go `*_test.go` colocated; frontend has NO test framework
- **Linting**: ESLint flat config (`frontend/eslint.config.mjs`)
- **Path alias**: `@/*` → `./frontend/src/*`

## ANTI-PATTERNS (THIS PROJECT)

- No `TODO`/`FIXME` comments (clean!)
- No `.env.example` committed (env via Docker/compose)
- No Makefile (use `docker-compose`)
- Frontend: no Jest/Vitest configured

## UNIQUE STYLES

- Go uses Fiber web framework (not Gin/Echo)
- Frontend uses shadcn/ui + Radix primitives + Tailwind 4
- CLI uses Cobra + OAuth2 device flow
- MinIO for object storage, SQLite for dev, PostgreSQL for prod

## COMMANDS

```bash
# Dev (Docker)
docker-compose -f docker-compose.dev.yml up -d

# Backend tests
cd backend && go test ./...

# Frontend
cd frontend && npm run dev

# CLI build
cd cli && go build -o docshare .
```

## NOTES

- Frontend runs on :3001 (not :3000)
- Backend API at :8080
- MinIO console at :9001
- JWT secret must be set in production
- Go versions: CI uses 1.24, Dockerfile uses 1.25 (future)
