# DocShare Architecture

This document describes the architecture, design decisions, and technical implementation details of DocShare.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Technology Stack](#technology-stack)
3. [Project Structure](#project-structure)
4. [Backend Architecture](#backend-architecture)
5. [Frontend Architecture](#frontend-architecture)
6. [Data Models](#data-models)
7. [Authentication & Authorization](#authentication--authorization)
8. [File Storage Strategy](#file-storage-strategy)
9. [Permission System](#permission-system)
10. [Preview Generation](#preview-generation)
11. [Security Considerations](#security-considerations)
12. [Design Decisions](#design-decisions)

## System Architecture

### High-Level Overview

DocShare follows a traditional client-server architecture with the following components:

```
┌──────────────────────────────────────────────────────────┐
│                        Client                            │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Next.js Frontend (SSR + Client Components)        │  │
│  │  - Pages: App Router                               │  │
│  │  - State: Zustand                                  │  │
│  │  - UI: Radix + shadcn/ui                           │  │
│  └────────────────────────────────────────────────────┘  │
└───────────────────────────┬──────────────────────────────┘
                            │ HTTPS/REST
                            │
┌───────────────────────────▼──────────────────────────────┐
│                    API Gateway Layer                     │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Go Fiber HTTP Server                              │  │
│  │  - Middleware: Auth, CORS, Logging                 │  │
│  │  - Routes: RESTful endpoints                       │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────┬─────────────────────┬─────────────────────┘
               │                     │
┌──────────────▼──────────┐   ┌──────▼─────────────────────┐
│   Business Logic        │   │   Storage Layer            │
│  ┌──────────────────┐   │   │  ┌──────────────────────┐  │
│  │   Handlers       │   │   │  │  Storage Service     │  │
│  │  - Auth          │   │   │  │  (S3 Client)         │  │
│  │  - Files         │   │   │  └──────────────────────┘  │
│  │  - Shares        │   │   │                            │
│  │  - Groups        │   │   │  ┌──────────────────────┐  │
│  │  - Users         │   │   │  │  Preview Service     │  │
│  └──────────────────┘   │   │  │  (Gotenberg Client)  │  │
│  ┌──────────────────┐   │   │  └──────────────────────┘  │
│  │   Services       │   │   │                            │
│  │  - Access        │   │   └────────────────────────────┘
│  │  - Preview       │   │
│  └──────────────────┘   │
└──────────┬──────────────┘
           │
┌──────────▼──────────────────────────────────────────────┐
│                   Data Layer                            │
│  ┌─────────────────┐      ┌─────────────────────────┐   │
│  │  PostgreSQL     │      │  AWS S3                 │   │
│  │  - User Data    │      │  - File Objects         │   │
│  │  - Metadata     │      │  - Preview Cache        │   │
│  │  - Permissions  │      └─────────────────────────┘   │
│  └─────────────────┘                                    │
└─────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Technology |
|-----------|----------------|------------|
| **Frontend** | User interface, client-side rendering, form validation | Next.js 16, React 19, TypeScript |
| **API Server** | Request routing, authentication, authorization, business logic | Go, Fiber framework |
| **Database** | Persistent storage of metadata, users, permissions | PostgreSQL 16 |
| **Object Storage** | Binary file storage, scalability | AWS S3 |
| **Preview Service** | Document conversion (Office → PDF) | Gotenberg (LibreOffice) |
| **Audit Service** | Audit logging, activity feed, and S3 log export | Go |

## Technology Stack

### Backend

- **Language**: Go 1.24+
- **Web Framework**: Fiber v2
- **ORM**: GORM
- **Database**: PostgreSQL 16
- **Storage**: AWS S3
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **Password Hashing**: bcrypt (golang.org/x/crypto)

### Frontend

- **Framework**: Next.js 16.1.6
- **Runtime**: React 19.2.3
- **Language**: TypeScript 5
- **Styling**: TailwindCSS 4
- **UI Components**: Radix UI + shadcn/ui
- **State Management**: Zustand
- **Icons**: Lucide React

### Infrastructure

- **Container Runtime**: Docker + Docker Compose
- **Reverse Proxy**: (Not included - use Nginx, Traefik, or Caddy)
- **Document Conversion**: Gotenberg 8

## Project Structure

```
docshare/
├── backend/
│   ├── cmd/
│   │   └── server/          # Application entry point
│   ├── internal/
│   │   ├── config/          # Configuration management
│   │   ├── database/        # Database connection & migrations
│   │   ├── handlers/        # HTTP request handlers (controllers)
│   │   ├── middleware/      # HTTP middleware (auth, logging, CORS)
│   │   ├── models/          # Database models & entities
│   │   ├── services/        # Business logic services
│   │   └── storage/         # Storage abstraction (S3)
│   ├── pkg/
│   │   ├── logger/          # Structured logging utilities
│   │   ├── previewtoken/    # Preview token generation
│   │   └── utils/           # Shared utilities (JWT, validation)
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── cli/
│   ├── cmd/                 # CLI command definitions (Cobra)
│   ├── internal/
│   │   ├── api/             # HTTP client & API response types
│   │   ├── config/          # Config file persistence (~/.config/docshare/)
│   │   ├── output/          # Table formatting, JSON output
│   │   └── pathutil/        # Path-to-UUID resolution
│   ├── install.sh           # Curl-able install script
│   ├── main.go
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── app/             # Next.js app router pages
│   │   │   ├── (auth)/      # Authentication pages (login, register)
│   │   │   └── (dashboard)/ # Protected dashboard pages (files, shared, activity, settings, admin)
│   │   ├── components/      # React components
│   │   │   └── ui/          # shadcn/ui components
│   │   └── lib/             # Utilities, API client, types
│   ├── public/              # Static assets
│   ├── Dockerfile
│   ├── package.json
│   └── tsconfig.json
├── docker-compose.yml       # Multi-service orchestration
├── README.md                # Project overview
├── ARCHITECTURE.md          # This file
├── API.md                   # API reference documentation
├── CLI.md                   # CLI installation & command reference
├── DEPLOYMENT.md            # Deployment guide
└── CONTRIBUTING.md           # Development guidelines
```

## Backend Architecture

### Layer Separation

The backend follows a clean layered architecture:

```
cmd/
  server/main.go          # Entry point, dependency injection

internal/
  handlers/               # HTTP request handlers (Presentation Layer)
    ├── auth.go          # Authentication endpoints
    ├── api_tokens.go    # API token management
    ├── device_auth.go   # Device flow endpoints
    ├── files.go         # File management endpoints
    ├── shares.go        # Sharing endpoints
    ├── groups.go        # Group management endpoints
    ├── users.go         # User management endpoints
    ├── activities.go    # Activity feed endpoints
    └── audit.go         # Audit log endpoints

  services/              # Business logic (Service Layer)
    ├── access.go        # Permission checking service
    ├── preview.go       # Preview generation service
    └── audit.go         # Audit logging and activity service

  models/                # Domain entities (Domain Layer)
    ├── user.go
    ├── file.go
    ├── share.go
    ├── group.go
    ├── group_membership.go
    ├── audit_log.go
    └── activity.go

  storage/               # Storage abstraction (Infrastructure Layer)
    └── s3.go            # S3 client wrapper

  database/              # Database management (Infrastructure Layer)
    └── database.go      # Connection, migrations

  middleware/            # HTTP middleware
    ├── auth.go          # JWT authentication
    └── logging.go       # Request logging

  config/                # Configuration management
    └── config.go        # Environment variable loading (includes AuditConfig)

pkg/                     # Shared utilities
  ├── logger/            # Structured logging
  ├── utils/             # JWT, validation helpers
  └── previewtoken/      # Preview token generation
```

### Request Flow

1. **Request Reception**: Fiber receives HTTP request
2. **Middleware Chain**: 
   - CORS check
   - Request logging
   - Authentication (if required)
   - Authorization (if required)
3. **Handler Execution**: Route-specific handler processes request
4. **Service Call**: Handler delegates business logic to services
5. **Data Access**: Services interact with database/storage
6. **Response**: JSON response returned to client

### Dependency Injection

Dependencies are injected at application startup in `cmd/server/main.go`:

```go
// Initialize infrastructure
db := database.Connect(cfg.DB)
storageClient := storage.NewS3Client(cfg.S3)

// Create services
accessService := services.NewAccessService(db)
auditService := services.NewAuditService(db, storageClient, cfg.Audit)
previewService := services.NewPreviewService(db, storageClient, cfg.Gotenberg)

// Create handlers with injected dependencies
filesHandler := handlers.NewFilesHandler(db, storageClient, accessService, previewService, auditService)
```

This approach:
- Makes testing easier (mock dependencies)
- Reduces coupling between layers
- Improves code organization

## Frontend Architecture

### App Router Structure

Next.js 14+ App Router with route groups:

```
src/app/
  layout.tsx              # Root layout (providers, metadata)
  page.tsx                # Landing page
  
  (auth)/                 # Auth route group (special layout)
    layout.tsx            # Auth layout (centered, no nav)
    login/
      page.tsx            # Login page
    register/
      page.tsx            # Registration page
  
  (dashboard)/            # Dashboard route group (requires auth)
    layout.tsx            # Dashboard layout (sidebar, nav)
    files/
      page.tsx            # Root files list
      [id]/
        page.tsx          # Folder contents
    shared/
      page.tsx            # Files shared with me
    groups/
      page.tsx            # Groups list
      [id]/
        page.tsx          # Group details & members
    activity/
      page.tsx            # User activity feed & notifications
    settings/
      page.tsx            # Account settings & audit log tab
    admin/
      page.tsx            # Admin user management
```

### State Management Strategy

**Zustand** for minimal global state:
- User authentication state
- Current file path/breadcrumbs
- UI state (modals, loading)

**Server Components** for data fetching:
- Files list
- Group membership
- User details

**Client Components** for interactivity:
- File upload with progress
- Drag-and-drop
- Share dialogs
- Context menus

### Component Architecture

```
components/
  ui/                    # shadcn/ui primitives (button, dialog, etc.)
  file-icon.tsx          # File type icon resolver
  file-viewer.tsx        # File preview modal
  file-inspector.tsx     # File metadata sidebar
  upload-zone.tsx        # Drag-drop upload component
  share-dialog.tsx       # Sharing modal
  move-dialog.tsx        # Move file dialog
  create-folder-dialog.tsx
  loading.tsx            # Loading states
  providers.tsx          # Context providers wrapper
```

### API Client Pattern

Centralized API client in `lib/api.ts`:

```typescript
// Automatic token injection
// Automatic error handling
// Automatic redirect on 401

export const apiMethods = {
  get: <T>(endpoint, params) => api<T>(endpoint, { method: 'GET', params }),
  post: <T>(endpoint, body) => api<T>(endpoint, { method: 'POST', body }),
  put: <T>(endpoint, body) => api<T>(endpoint, { method: 'PUT', body }),
  delete: <T>(endpoint) => api<T>(endpoint, { method: 'DELETE' }),
  upload: <T>(endpoint, formData) => // Special handling for multipart
}
```

### API URL Strategy

`NEXT_PUBLIC_API_URL` is embedded at build time, not runtime:

- **Production build**: Set to empty string, making all API calls relative (`/api/...`)
- **Development**: Set via shell: `NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev`

**Production architecture**:
- Reverse proxy (Nginx/Traefik/Ingress) routes `/api` to backend
- Frontend and backend appear as a single origin (no CORS issues)
- No environment-specific API URL configuration needed

**Development architecture**:
- Frontend dev server proxies API calls to backend
- Or set `NEXT_PUBLIC_API_URL` to bypass proxy for direct backend access

## Data Models

### Entity Relationship Diagram

```
┌─────────────────┐       │
│      User       │       │
│─────────────────│       │
│ ID (PK)         │◄──────┼──────────┐
│ Email (unique)  │       │          │
│ PasswordHash    │       │          │
│ FirstName       │       │OwnerID   │
│ LastName        │       │          │
│ Role            │       │          │
│ AvatarURL       │       │          │
└─────────────────┘       │          │
         │                │          │
         │                │          │
         │CreatedByID     │          │UserID
         │                │          │
         ▼                │          │
┌─────────────────┐       │   ┌──────┴──────────┐
│     Group       │       │   │    APIToken     │
│─────────────────│       │   │─────────────────│
│ ID (PK)         │       │   │ ID (PK)         │
│ Name            │       │   │ UserID (FK)     │
│ Description     │       │   │ Name            │
│ CreatedByID (FK)│       │   │ TokenHash       │
└─────────────────┘       │   │ Prefix          │
         │                │   │ ExpiresAt       │
         │                │   │ LastUsedAt      │
         │GroupID         │   └─────────────────┘
         │                │
         ▼                │   ┌─────────────────┐
┌─────────────────┐       │   │   DeviceCode    │
│GroupMembership  │       │   │─────────────────│
│─────────────────│       │   │ ID (PK)         │
│ ID (PK)         │       │   │ DeviceCodeHash  │
│ GroupID (FK)    │       │   │ UserCode        │
│ UserID (FK)     │───────┤   │ ExpiresAt       │
│ Role            │       │   │ Interval        │
└─────────────────┘       │   │ Status          │
                          │   │ UserID (FK)     │
                          │   └─────────────────┘
                          │
┌─────────────────┐       │
│      File       │       │
│─────────────────│       │
│ ID (PK)         │───────┘
│ Name            │
│ MimeType        │
│ Size            │
│ IsDirectory     │
│ ParentID (FK)   │───┐
│ OwnerID (FK)    │   │
│ StoragePath     │   │
│ ThumbnailPath   │   │
└─────────────────┘   │
         │            │
         │            │(self-reference)
         │ParentID    │
         └────────────┘
         │
         │FileID
         │
         ▼
┌─────────────────┐
│     Share       │
│─────────────────│
│ ID (PK)         │
│ FileID (FK)     │
│ SharedByID (FK) │─────────┐
│ SharedWithUserID│         │
│ SharedWithGroupID         │
│ Permission      │         │
│ ExpiresAt       │         │
└─────────────────┘         │
                            │
                            ▼
                     ┌─────────────┐
                     │    User     │
                     └─────────────┘
```

### Authentication Models

#### 1. APIToken
Stores long-lived personal access tokens.
- **UserID**: The owner of the token.
- **TokenHash**: SHA-256 hash of the raw token.
- **Prefix**: The first few characters of the token (e.g., `dsh_7f8e9d`) for display.
- **ExpiresAt**: Optional expiration timestamp.
- **LastUsedAt**: Updated on every authenticated request using this token.

#### 2. DeviceCode
Temporary storage for OAuth2 Device Authorization Flow (RFC 8628).
- **DeviceCodeHash**: SHA-256 hash of the device code used for polling.
- **UserCode**: Consonant-only code (e.g., `BCDF-GHJK`) shown to the user.
- **Status**: `pending`, `approved`, or `denied`.
- **UserID**: The user who approved the code (NULL until approved).

### Audit & Activity Models

```
┌─────────────────┐       ┌─────────────────┐
│    AuditLog     │       │    Activity     │
│─────────────────│       │─────────────────│
│ ID (PK)         │       │ ID (PK)         │
│ UserID (FK)     │───────┤ UserID (FK)     │
│ Action          │       │ ActorID (FK)    │
│ ResourceType    │       │ Message         │
│ ResourceID      │       │ IsRead          │
│ Details (JSONB) │       │ ResourceType    │
│ IPAddress       │       │ ResourceID      │
│ RequestID       │       │ CreatedAt       │
│ CreatedAt       │       └─────────────────┘
└─────────────────┘

┌───────────────────┐
│ AuditExportCursor │
│───────────────────│
│ ID (PK)           │
│ LastExportAt      │
└───────────────────┘
```

#### 1. AuditLog
Append-only table (no soft-delete) that tracks all system actions.
- **UserID**: The user who performed the action.
- **Action**: The specific action (e.g., `file.upload`, `share.create`).
- **ResourceType/ID**: The entity affected by the action.
- **Details**: JSONB field containing action-specific metadata.
- **IPAddress/RequestID**: Traceability metadata for security auditing.

#### 2. Activity
User-facing notifications and personal activity feed.
- **UserID**: The recipient of the activity/notification.
- **ActorID**: The user who triggered the activity.
- **IsRead**: Tracks whether the user has seen the notification.
- **ResourceType/ID**: Links to the relevant entity for frontend navigation.

#### 3. AuditExportCursor
A singleton table used to track the timestamp of the last successful S3 audit log export.

### Key Model Decisions

#### 1. File Model

**Unified File & Folder Model**: Both files and folders are stored in the same table with an `IsDirectory` boolean flag.

**Rationale**:
- Simplifies permission inheritance (folders can be shared like files)
- Enables recursive operations (move, delete, share)
- Cleaner API (single endpoint for both types)

**Trade-offs**:
- Slightly more complex queries (always need `WHERE is_directory = ?`)
- NULL fields for directories (size, mimeType = "application/directory")

#### 2. Share Model

**Flexible Recipient**: Share can target either a user (`SharedWithUserID`) or a group (`SharedWithGroupID`).

**Constraint**: Exactly one must be non-NULL (enforced at application level).

**Rationale**:
- Single table for both share types
- Simpler querying ("give me all shares for file X")

**Permission Levels**:
- `view`: Can see metadata and preview
- `download`: Can download content
- `edit`: Can modify and reshare

**Permission Hierarchy**: `edit` > `download` > `view`

#### 3. Group Membership

**Three Roles**:
- `owner`: Can delete group, modify all settings
- `admin`: Can add/remove members, change roles
- `member`: Standard membership

**Multiple Owners**: System allows multiple owners (not enforced uniqueness).

## Authentication & Authorization

### Authentication Flow

DocShare supports three authentication paths:

#### 1. JWT Flow (Web)
Standard login/register flow returning a 24h JWT.

#### 2. API Token Flow (CLI/Programmatic)
1. User generates a token in Settings (`dsh_` prefix + 48 random hex chars).
2. Client sends `Authorization: Bearer dsh_...`.
3. Middleware detects the `dsh_` prefix.
4. Server hashes the token and looks it up in the `api_tokens` table.
5. If valid, `last_used_at` is updated and user is attached to context.

#### 3. Device Flow (OAuth2 RFC 8628)
For CLI tools that cannot open a browser.

```
┌─────────┐          ┌─────────┐          ┌─────────┐
│   CLI   │          │ Backend │          │ Browser │
└────┬────┘          └────┬────┘          └────┬────┘
     │                    │                    │
     │ 1. POST /device/code                    │
     │───────────────────>│                    │
     │                    │                    │
     │ 2. Return codes    │                    │
     │<───────────────────│                    │
     │                    │                    │
     │ 3. Poll /token     │ 4. User visits /device   │
     │───────────────────>│<───────────────────│
     │                    │                    │
     │                    │ 5. Approve code    │
     │                    │<───────────────────│
     │                    │                    │
     │ 6. Return JWT      │                    │
     │<───────────────────│                    │
     │                    │                    │
```

### JWT Token Structure

```json
{
  "user_id": "uuid-here",
  "email": "user@example.com",
  "role": "user",
  "exp": 1234567890
}
```

**Token Lifetime**: Configurable via `JWT_EXPIRATION_HOURS` (default: 24 hours)

**Storage**: localStorage (client-side)
- ⚠️ Vulnerable to XSS
- ✅ Survives page refresh
- Alternative: HttpOnly cookies (more secure, requires CSRF protection)

### Authorization Middleware

**RequireAuth Middleware**:
1. Extract `Authorization: Bearer <token>` header.
2. Determine token type:
   - If prefix is `dsh_`: Route to API token lookup (hash and check DB).
   - Otherwise: Validate as JWT signature and expiration.
3. Load user from database and update `last_used_at` (for API tokens).
4. Attach user to request context.
5. Proceed to handler.

**AdminOnly Middleware**:
1. Check if user exists in context
2. Verify `user.Role == "admin"`
3. Return 403 if not admin

### Permission Checking

Implemented in `services/access.go`:

```go
func (a *AccessService) HasAccess(
    ctx context.Context, 
    userID uuid.UUID, 
    fileID uuid.UUID, 
    requiredPermission SharePermission
) bool
```

**Algorithm**:
1. Check if user is file owner → grant access
2. Check for direct share to user with sufficient permission
3. Check for share to user's groups with sufficient permission
4. If file has parent, recursively check parent permissions
5. Repeat until root or access granted

**Permission Inheritance**: 
- Share on folder applies to all descendants
- Implemented via recursive parent traversal

## File Storage Strategy

### Hybrid Storage Model

**Metadata** → PostgreSQL
- File name, size, MIME type
- Ownership, timestamps
- Relationships (parent, shares)

**Binary Content** → AWS S3
- Actual file bytes
- Scalable, distributed storage
- Presigned URLs for direct client access

### Storage Path Strategy

Files are stored in S3 with the following path structure:

```
files/<uuid>/<filename>
previews/<uuid>.pdf
thumbnails/<uuid>.jpg
```

**UUID-based paths**:
- Prevents name collisions
- Obscures file structure
- Enables secure direct URLs

### Upload Flow

```
┌────────┐                 ┌─────────┐               ┌─────────┐
│ Client │                 │  Backend│               │   S3    │
└───┬────┘                 └────┬────┘               └────┬────┘
    │                           │                         │
    │ 1. POST /api/files/upload │                         │
    │    (multipart/form-data)  │                         │
    │──────────────────────────>│                         │
    │                           │                         │
    │                           │ 2. Generate UUID        │
    │                           │ 3. Stream to S3         │
    │                           │────────────────────────>│
    │                           │                         │
    │                           │ 4. Store metadata in DB │
    │                           │                         │
    │                           │ 5. Trigger preview gen  │
    │                           │    (background)         │
    │                           │                         │
    │ 6. Return file metadata   │                         │
    │<──────────────────────────│                         │
    │                           │                         │
```

### Download Flow

**Option 1: Presigned URLs** (preferred for large files)
```
Client → Backend: GET /api/files/{id}/download-url
Backend → Client: { url: "https://s3/..." }
Client → S3: Direct download (no backend involved)
```

**Option 2: Proxied Download** (for small files or access logging)
```
Client → Backend: GET /api/files/{id}/download
Backend → S3: Fetch file
Backend → Client: Stream file bytes
```

## Preview Generation

### Supported Formats

| Format                    | Method    | Output           |
|---------------------------|-----------|------------------|
| PDF                       | Direct    | Original PDF     |
| Images (JPEG, PNG, GIF)   | Direct    | Original image   |
| Office (DOCX, XLSX, PPTX) | Gotenberg | Converted to PDF |
| Text (TXT, MD)            | Direct    | Plain text       |

### Preview Generation Flow

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│   Backend    │         │  Gotenberg   │         │      S3      │
└──────┬───────┘         └──────┬───────┘         └──────┬───────┘
       │                        │                        │
       │ 1. User uploads DOCX   │                        │
       │ 2. File stored in S3   │                        │
       │────────────────────────┼───────────────────────>│
       │                        │                        │
       │ 3. Trigger preview gen │                        │
       │    (background job)    │                        │
       │                        │                        │
       │ 4. Fetch DOCX from     │                        │
       │    S3                  │                        │
       │<───────────────────────┼────────────────────────│
       │                        │                        │
       │ 5. Send to Gotenberg   │                        │
       │    POST /forms/libreoffice/convert              │
       │───────────────────────>│                        │
       │                        │                        │
       │                        │ 6. Convert with        │
       │                        │    LibreOffice         │
       │                        │                        │
       │ 7. Return PDF bytes    │                        │
       │<───────────────────────│                        │
       │                        │                        │
       │ 8. Store preview in    │                        │
       │    S3 (previews/)      │                        │
       │────────────────────────┼───────────────────────>│
       │                        │                        │
       │ 9. Update DB with      │                        │
       │    preview path        │                        │
       │                        │                        │
```

### Preview Tokens

For security, preview URLs include time-limited tokens:

```
GET /api/files/{id}/preview
→ { url: "http://backend/api/files/{id}/proxy?token=..." }
```

**Token Generation**:
```go
token := GeneratePreviewToken(fileID, userID, expiresAt)
// HMAC-SHA256(fileID || userID || expiresAt, secret)
```

**Token Validation**:
- Verify signature
- Check expiration
- Verify user has access to file

## Audit Log & Activity System

### Purpose
The system provides a dual-purpose tracking mechanism:
1.  **Audit Trail**: A comprehensive, append-only record of all system actions for security and compliance.
2.  **Activity Feed**: A user-facing notification system that alerts users to relevant events (e.g., new shares, group changes).

### Async Architecture
To minimize impact on request latency, the audit system uses an asynchronous design:
-   **Buffered Channel**: Handlers send audit events to a global buffered channel.
-   **Background Worker**: A dedicated goroutine listens to the channel and persists events to the database.
-   **Graceful Shutdown**: The system ensures the channel is drained before the application exits.

### Activity Generation
Not all audit events trigger user-facing activities. The `AuditService` determines activity creation based on the event type:
-   **Self-Activities**: Users see their own actions (e.g., "You uploaded file.txt") in their personal feed.
-   **Notifications**: Relevant actions trigger activities for other users (e.g., "User A shared a file with you").
-   **Group Activities**: Actions within a group (e.g., "User B added you to Group X") notify all relevant members.

### S3 Export
For long-term retention and external analysis, audit logs are periodically exported to S3:
-   **Format**: NDJSON (Newline Delimited JSON) for easy parsing.
-   **Path**: `audit-logs/YYYY/MM/DD/HH-mm-ss.ndjson`.
-   **Interval**: Configurable via `AUDIT_EXPORT_INTERVAL` (default: 1h).
-   **Cursor-based**: The `AuditExportCursor` ensures no logs are missed or duplicated between export cycles.

### User Access
Users can view and download their own audit logs via the **Account Settings > Audit Log** tab, providing transparency into how their data is accessed and modified.

## Security Considerations

> For comprehensive security policy, deployment best practices, and vulnerability reporting, see [SECURITY.md](./SECURITY.md).

### Password Security

- **Hashing**: bcrypt with cost factor 10
- **No plaintext**: Passwords never stored unencrypted
- **Field exclusion**: `PasswordHash` excluded from JSON serialization (`json:"-"`)

### JWT Security

- **Secret rotation**: Change `JWT_SECRET` regularly in production
- **Expiration**: Tokens expire after configurable period
- **Signature verification**: All tokens verified on each request
- **User validation**: User existence checked on each authenticated request

### Authorization

- **Recursive permission checks**: Folder shares apply to descendants
- **Expiration support**: Shares can have expiration dates
- **Permission levels**: Granular control (view, download, edit)
- **Owner bypass**: Owners always have full access

### API Token Security

- **Hashed Storage**: Tokens are stored as SHA-256 hashes; raw tokens are never saved.
- **Prefix Display**: Only the `dsh_` prefix and first few characters are stored in plaintext for user identification.
- **Token Limits**: Users are limited to 25 active tokens to prevent bloat and reduce attack surface.
- **Expiration**: Support for fixed-term tokens (30d, 90d, 365d) or permanent tokens.

### Device Flow Security

- **Hashed Codes**: Device codes are SHA-256 hashed in the database.
- **Short Expiry**: Codes expire after 15 minutes.
- **Single Use**: Device codes are hard-deleted immediately after a JWT is issued.
- **User Code Entropy**: Uses a consonant-only alphabet to avoid ambiguous characters (e.g., 0/O, 1/I) and prevent accidental word formation.

### CORS Configuration

Current configuration (development):
```go
AllowOrigins: "http://localhost:3001,http://127.0.0.1:3001"
```

**Production**: Update to actual domain(s)

### File Upload Security

- **Size limit**: 100MB by default (configurable)
- **MIME type validation**: Checked on upload
- **Path sanitization**: UUID-based paths prevent traversal
- **Virus scanning**: Not implemented (consider adding ClamAV)

### SQL Injection Prevention

- **ORM usage**: GORM parameterizes all queries
- **No raw SQL**: Direct queries avoided
- **Input validation**: All inputs validated before use

### XSS Prevention

- **React escaping**: React automatically escapes rendered content
- **Content-Type headers**: Proper headers prevent MIME sniffing
- **CSP headers**: Consider adding Content Security Policy

## Design Decisions

### Why Go for Backend?

**Pros**:
- High performance, low memory footprint
- Strong concurrency support (goroutines)
- Fast compilation, single binary deployment
- Excellent standard library
- Static typing with good DX

**Alternatives considered**:
- Node.js: Less type-safe, higher memory usage
- Python: Slower runtime, GIL limitations
- Rust: Steeper learning curve, longer compile times

### Why Fiber over net/http?

**Pros**:
- Express-like API (familiar to many developers)
- Built-in middleware ecosystem
- Better performance than many alternatives
- Excellent documentation

**Alternatives considered**:
- Chi: More idiomatic Go, but less feature-rich
- Echo: Similar to Fiber, slightly different API
- Gin: Older, less active development

### Why Next.js over Create React App?

**Pros**:
- Server-side rendering (better SEO, faster initial load)
- Built-in routing (App Router)
- API routes (could host backend too)
- Image optimization
- Production-ready defaults

**Alternatives considered**:
- Vite + React Router: More configuration needed
- Remix: Newer, smaller ecosystem
- SvelteKit: Different framework, less mature

### Why AWS S3 over Filesystem?

**Pros**:
- Managed service (no infrastructure management)
- Distributed storage (scalability)
- Built-in redundancy
- Presigned URLs (offload traffic from backend)
- Separate scaling from compute

**Alternatives considered**:
- Local filesystem: Not scalable, single point of failure
- MinIO: Self-hosted, requires infrastructure management
- Azure Blob: Similar to S3

### Why PostgreSQL over MySQL?

**Pros**:
- Better JSON support (for future flexibility)
- More advanced features (CTEs, window functions)
- True UUID type
- Better full-text search

**Alternatives considered**:
- MySQL: Simpler, but fewer features
- MongoDB: NoSQL, but relational data fits SQL better
- SQLite: Not suitable for multi-user production

### Single Table for Files and Folders?

**Decision**: Store both in `files` table with `is_directory` flag

**Pros**:
- Unified permission system
- Simpler recursive operations
- Single API endpoint for both types

**Cons**:
- Some NULL fields for directories
- Slightly more complex queries

**Alternative**: Separate `folders` table
- More normalized
- But complicates permission inheritance

### Recursive Permission Checking?

**Decision**: Check permissions up the folder tree on each access

**Pros**:
- Always accurate (no stale cache)
- Simpler implementation
- Share updates take effect immediately

**Cons**:
- Slower for deeply nested files
- Multiple DB queries per access check

**Alternative**: Denormalized permission cache
- Faster reads
- But complex invalidation logic

### Preview Generation: Sync or Async?

**Decision**: Asynchronous (background job queue) ✅ Implemented

The preview generation now uses a background job queue with the following characteristics:

- **Job Queue**: In-memory channel with DB-backed job status tracking
- **Worker Pattern**: Similar to the audit service - buffered channel + goroutine worker
- **Retry Logic**: Exponential backoff (30s, 2m, 10m) up to 3 attempts
- **Startup Recovery**: Pending/failed jobs are re-queued on service restart

**API Endpoints**:
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/files/:id/convert-preview` | POST | Enqueue preview generation (returns 202) |
| `/api/files/:id/preview-status` | GET | Get job status |
| `/api/files/:id/retry-preview` | POST | Retry failed job |

**Future**: Could move to Redis/external queue for durability across restarts

### Token Storage: localStorage or Cookies?

**Decision**: localStorage

**Pros**:
- Simple implementation
- Works with CORS
- Easy to access from JS

**Cons**:
- Vulnerable to XSS
- Manual token management

**Alternative**: HttpOnly cookies
- More secure (XSS-proof)
- But requires CSRF protection
- More complex with CORS

## Performance Considerations

### Database Indexes

Key indexes for performance:

```sql
-- Files
CREATE INDEX idx_files_owner_id ON files(owner_id);
CREATE INDEX idx_files_parent_id ON files(parent_id);
CREATE INDEX idx_files_is_directory ON files(is_directory);

-- Shares
CREATE INDEX idx_shares_file_id ON shares(file_id);
CREATE INDEX idx_shares_shared_by_id ON shares(shared_by_id);
CREATE INDEX idx_shares_shared_with_user_id ON shares(shared_with_user_id);
CREATE INDEX idx_shares_shared_with_group_id ON shares(shared_with_group_id);

-- Group Memberships
CREATE INDEX idx_group_memberships_user_id ON group_memberships(user_id);
CREATE INDEX idx_group_memberships_group_id ON group_memberships(group_id);

-- Users
CREATE UNIQUE INDEX idx_users_email ON users(email);
```

### Connection Pooling

- **Database**: GORM manages connection pool automatically
- **S3**: HTTP client with keep-alive
- **Gotenberg**: HTTP client with connection reuse

### Caching Opportunities

**Not currently implemented, but recommended for production**:

1. **User data cache** (Redis): Reduce DB lookups on auth
2. **Permission cache** (Redis): Cache expensive recursive checks
3. **Preview cache** (S3): Already stored, served via presigned URLs
4. **Metadata cache** (Redis): Cache frequently accessed file metadata

### Scalability Strategy

**Horizontal Scaling**:
- Backend: Stateless, can run multiple instances behind load balancer
- Frontend: Static build, serve from CDN
- Database: PostgreSQL read replicas for read-heavy workloads
- Storage: S3 scales automatically

**Vertical Scaling**:
- Increase backend server resources for concurrent connections
- Increase PostgreSQL resources for complex queries
- S3 handles storage scaling automatically

## Future Improvements

### Short Term
1. ~~**Background preview generation**: Move to job queue~~ ✅ Completed
2. **Pagination**: Add to all list endpoints
3. **Search**: Full-text search for file names
4. **Rate limiting**: Prevent abuse

### Medium Term
1. **Virus scanning**: Integrate ClamAV or similar
2. **File versioning**: Keep history of file changes
3. **Trash/recovery**: Soft delete with restore

### Long Term
1. **Real-time collaboration**: WebRTC or WebSocket for live editing
2. **Mobile apps**: React Native or native iOS/Android
3. **Advanced permissions**: Custom permission combinations
4. **Multi-tenant**: Support multiple organizations
