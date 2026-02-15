# PROJECT KNOWLEDGE BASE

**Generated:** 2026-02-15
**Branch:** mh/s3

## OVERVIEW

DocShare: document sharing platform with file storage, sharing, groups, previews. Monorepo (Go backend + Next.js frontend + Go CLI).

## BUILD / LINT / TEST COMMANDS

```bash
# Backend
cd backend && go build ./...                          # Build all packages
cd backend && go test ./...                           # Run all tests
cd backend && go test -v ./internal/handlers          # Run specific package tests
cd backend && go test -v -run TestAuthHandler ./...   # Run single test by name
cd backend && go vet ./...                            # Static analysis
cd backend && go mod tidy                             # Clean dependencies

# Frontend
cd frontend && npm run dev                            # Development server
cd frontend && npm run build                          # Production build
cd frontend && npm run lint                           # ESLint

# CLI
cd cli && go build -o docshare .                      # Build binary
cd cli && go test ./...                               # Run all tests

# Docker Development
docker-compose -f docker-compose.dev.yml up -d        # Full dev stack (includes MinIO)
docker-compose -f docker-compose.dev.yml up -d backend postgres  # Minimal dev

# Helm
helm dependency update charts/docshare                # Update chart deps
helm template release charts/docshare                 # Render templates locally
```

## STRUCTURE

```
./
├── backend/              # Go Fiber REST API
│   ├── cmd/server/       # Entry point
│   ├── internal/         # Handlers, models, services, middleware
│   └── pkg/              # Public utilities (logger, utils)
├── frontend/             # Next.js 16 App
│   └── src/
│       ├── app/          # Routes (App Router)
│       ├── components/   # Feature + UI components
│       ├── lib/          # API client, types, utilities
│       └── hooks/        # React hooks
├── cli/                  # Go CLI (docshare)
│   ├── cmd/              # Cobra commands
│   └── internal/         # API client, config, output
├── docs/                 # API, CLI, deployment docs
├── examples/             # Docker Compose + Helm examples
└── charts/               # Helm charts
```

## GO CODE STYLE

### Imports
```go
import (
    // Standard library (grouped first)
    "context"
    "fmt"
    "net/http"
    
    // External packages (blank line separator)
    "github.com/gofiber/fiber/v2"
    "gorm.io/gorm"
    
    // Internal packages (blank line separator)
    "github.com/docshare/backend/internal/models"
    "github.com/docshare/backend/pkg/logger"
)
```

### Naming
- **Packages**: lowercase, single word (`handlers`, `models`, `services`)
- **Types**: PascalCase (`AuthHandler`, `User`, `SharePermission`)
- **Interfaces**: `-er` suffix (`Reader`, `Writer`) or descriptive (`StorageClient`)
- **Constants**: PascalCase for public, camelCase for private (`UserRoleAdmin`, `defaultTimeout`)
- **Request structs**: `*Request` suffix (`registerRequest`, `loginRequest`)

### Error Handling
```go
// Always check errors, never ignore
if err := db.Create(&user).Error; err != nil {
    return utils.Error(c, fiber.StatusInternalServerError, "failed creating user")
}

// Wrap errors with context when appropriate
return fmt.Errorf("failed to upload file: %w", err)

// Use utils.Error for HTTP responses
return utils.Error(c, fiber.StatusBadRequest, "invalid request")
return utils.Success(c, fiber.StatusOK, user)
```

### Handlers
```go
type AuthHandler struct {
    DB    *gorm.DB
    Audit *services.AuditService
}

func NewAuthHandler(db *gorm.DB, audit *services.AuditService) *AuthHandler {
    return &AuthHandler{DB: db, Audit: audit}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
    var req registerRequest
    if err := c.BodyParser(&req); err != nil {
        return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
    }
    // ... validation and processing
}
```

### Testing
```go
// Test file naming: <name>_test.go, colocated with source
// Test function naming: Test<Struct>_<Method> or Test<Function>

func TestAuthHandler_Register(t *testing.T) {
    env := setupTestEnv(t)
    defer env.cleanup()  // Use t.Cleanup for teardown
    
    // Use table-driven tests for multiple cases
    tests := []struct {
        name    string
        payload map[string]interface{}
        want    int
    }{...}
}
```

## TYPESCRIPT CODE STYLE

### Imports
```typescript
// React imports first
import { useState, useEffect, useCallback, useMemo } from 'react';

// Third-party imports (blank line)
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';

// Local imports (blank line)
import { apiMethods } from '@/lib/api';
import { User } from '@/lib/types';
```

### Naming
- **Components**: PascalCase (`ShareDialog`, `FileViewer`)
- **Hooks**: `use` prefix (`useActivityToast`, `useFileUpload`)
- **Types/Interfaces**: PascalCase (`User`, `ApiResponse`, `FileViewerProps`)
- **Constants**: UPPER_SNAKE_CASE or PascalCase for enums

### Patterns
```typescript
// Use useMemo for derived state
const resolvedIds = useMemo(() => fileIds ?? (fileId ? [fileId] : []), [fileIds, fileId]);

// Use useCallback for functions in dependencies
const fetchShares = useCallback(async () => {
    // ...
}, [dependency]);

// API calls use apiMethods wrapper
const res = await apiMethods.get<User[]>('/api/users');
if (res.success) {
    setUsers(res.data);
}

// Client components must have 'use client' directive
'use client';
```

### Styling
- Use Tailwind CSS classes exclusively
- Use `cn()` utility for conditional classes: `cn(baseClass, conditional && conditionalClass)`
- Icons from `lucide-react`

## ANTI-PATTERNS

- **No comments**: Code should be self-documenting; no `TODO`/`FIXME`
- **No `.env.example`**: Environment via Docker/compose
- **No Makefile**: Use `docker-compose` or direct commands
- **No `any` type**: Use proper TypeScript types
- **No direct `fetch`**: Use `apiMethods` wrapper
- **No hardcoded status codes**: Use `fiber.Status...` constants

## ENVIRONMENT

| Service | Port | Note |
|---------|------|------|
| Frontend | 3001 | Not 3000 |
| Backend | 8080 | - |
| MinIO (dev) | 9000/9001 | Console at 9001 |
| PostgreSQL | 5432 | - |
| Gotenberg | 3000 | Document preview |

## ENVIRONMENT VARIABLES

```bash
# S3 Storage (required for production)
S3_REGION=us-east-1
S3_ENDPOINT=              # Auto: s3.$REGION.amazonaws.com
S3_ACCESS_KEY=            # Empty = IAM role auth
S3_SECRET_KEY=
S3_BUCKET=docshare

# Database
DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE

# Auth
JWT_SECRET, JWT_EXPIRATION_HOURS
```

## TESTING NOTES

- Go tests use in-memory SQLite with `gorm.Open(sqlite.Open(":memory:"), ...)`
- Use `t.Helper()` for test helper functions
- Use `t.Cleanup()` for resource cleanup
- Run single test: `go test -v -run TestName ./path/to/package`