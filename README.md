# DocShare

A modern, secure document sharing platform built with Go and Next.js. DocShare enables users to store, organize, and share files with granular permission controls at both individual and group levels.

## Overview

DocShare is a full-stack document management system featuring:

- 🔐 **Secure Authentication** - JWT-based authentication with role-based access control (RBAC)
- 📁 **File Management** - Hierarchical folder structure with upload, download, and preview capabilities
- 👥 **Group Management** - Create groups with multiple permission levels (owner, admin, member)
- 🔗 **Flexible Sharing** - Share files with individual users or groups with customizable permissions (view, download, edit)
- 📄 **Document Preview** - Automatic preview generation for various document types including Office documents
- 🗄️ **S3-Compatible Storage** - Uses MinIO for scalable object storage
- 📋 **Activity Feed** - Real-time activity notifications for file shares, uploads, and group changes
- 📊 **Audit Log** - Comprehensive audit trail tracking all user actions, exportable to CSV/JSON and periodically archived to S3/MinIO
- 🚀 **Production Ready** - Dockerized deployment with health checks and graceful shutdown

## Architecture

The application follows a clean three-tier architecture:

```
┌─────────────────┐
│   Frontend      │  Next.js 16 (React 19)
│   (Port 3001)   │  TypeScript, TailwindCSS, shadcn/ui
└────────┬────────┘
         │
         │ REST API
         │
┌────────▼────────┐
│   Backend       │  Go (Fiber Framework)
│   (Port 8080)   │  JWT Auth, GORM ORM
└────┬───┬────┬───┘
     │   │    │
     │   │    └──────────────┐
     │   │                   │
┌────▼───▼────┐   ┌──────────▼─────────┐
│  PostgreSQL │   │  MinIO (S3 Storage)│
│  (Port 5432)│   │  (Port 9000/9001)  │
└─────────────┘   └────────────────────┘
                           │
                  ┌────────▼──────────┐
                  │   Gotenberg       │
                  │ (Document Convert)│
                  │   (Port 3000)     │
                  └───────────────────┘
```

### Core Services

- **Backend (Go)**: RESTful API server handling authentication, authorization, and business logic
- **Frontend (Next.js)**: Server-side rendered React application with modern UI components
- **PostgreSQL**: Primary database for metadata and relationships
- **MinIO**: S3-compatible object storage for file content
- **Gotenberg**: LibreOffice-based document conversion service for preview generation

## Technology Stack

### Backend
- **Language**: Go 1.24+
- **Web Framework**: Fiber v2
- **ORM**: GORM
- **Database**: PostgreSQL 16
- **Storage**: MinIO (S3-compatible)
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

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Git

### Development Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd docshare
   ```

2. **Start all services**
   ```bash
   docker-compose up -d
   ```

3. **Access the application**
   - Frontend: http://localhost:3001
   - Backend API: http://localhost:8080
   - MinIO Console: http://localhost:9001 (credentials: docshare/docshare_secret)

4. **Create your first user**
   - Navigate to http://localhost:3001/register
   - Register an account
   - First registered user is automatically assigned admin role

### Development Without Docker

#### Backend Development

```bash
cd backend

# Install dependencies
go mod download

# Set environment variables (see DEPLOYMENT.md for full list)
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=docshare
export DB_PASSWORD=docshare_secret
export DB_NAME=docshare
export MINIO_ENDPOINT=localhost:9000
export MINIO_ACCESS_KEY=docshare
export MINIO_SECRET_KEY=docshare_secret
export JWT_SECRET=your-secret-key-here
export GOTENBERG_URL=http://localhost:3000

# Run the server
go run cmd/server/main.go
```

#### Frontend Development

```bash
cd frontend

# Install dependencies
npm install

# Set environment variables
export NEXT_PUBLIC_API_URL=http://localhost:8080

# Run the development server
npm run dev
```

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
│   │   └── storage/         # Storage abstraction (MinIO)
│   ├── pkg/
│   │   ├── logger/          # Structured logging utilities
│   │   ├── previewtoken/    # Preview token generation
│   │   └── utils/           # Shared utilities (JWT, validation)
│   ├── Dockerfile
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
├── README.md                # This file
├── ARCHITECTURE.md          # Detailed architecture documentation
├── API.md                   # API reference documentation
└── DEPLOYMENT.md            # Deployment guide
```

## Key Features

### Authentication & Authorization
- JWT-based authentication with configurable expiration
- Role-based access control (admin, user)
- Secure password hashing with bcrypt
- Protected routes with automatic token refresh

### File Management
- Hierarchical folder structure (unlimited depth)
- Drag-and-drop file upload with progress tracking
- Bulk file operations (move, delete)
- File preview for supported formats
- Download with presigned URLs
- Automatic MIME type detection

### Sharing & Permissions
- Share files/folders with users or groups
- Three permission levels:
  - **View**: Can view file metadata and previews
  - **Download**: Can download files
  - **Edit**: Can modify files and manage shares
- Expiration dates for time-limited access
- Permission inheritance from parent folders
- Revocable shares

### Group Management
- Create groups with multiple members
- Three group roles:
  - **Owner**: Full control including deletion
  - **Admin**: Can manage members
  - **Member**: Standard group membership
- Share files with entire groups

### Activity Feed & Audit Log
- Real-time activity feed showing file uploads, downloads, shares, group changes, and more
- Unread notification count with badge indicator in sidebar
- Mark individual or all activities as read
- Comprehensive server-side audit log tracking all user actions
- Users can download their own audit log as CSV or JSON from Account Settings
- Server-wide audit log automatically exported to S3/MinIO as NDJSON on a configurable interval

### Document Preview
- Automatic preview generation for Office documents (DOCX, XLSX, PPTX)
- PDF preview support
- Image preview support
- Secure preview tokens with expiration
- Proxy endpoint for external previews (Gotenberg)

## Security Considerations

- **JWT Secret**: Change `JWT_SECRET` in production to a long random string (minimum 32 characters)
- **Database Credentials**: Update default credentials in `docker-compose.yml`
- **MinIO Credentials**: Change MinIO access keys in production
- **CORS Configuration**: Update allowed origins in `backend/internal/middleware/auth.go`
- **HTTPS**: Use a reverse proxy (Nginx, Traefik, Caddy) to handle TLS termination
- **File Size Limits**: Default is 100MB (configurable in `cmd/server/main.go`)
- **Password Requirements**: Implemented in frontend validation

## Documentation

- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: Detailed architecture decisions, design patterns, and system components
- **[API.md](./API.md)**: Complete REST API reference with request/response examples
- **[DEPLOYMENT.md](./DEPLOYMENT.md)**: Production deployment guide with configuration options

## Development

### Building

```bash
# Build backend
cd backend
go build -o server ./cmd/server

# Build frontend
cd frontend
npm run build
```

### Testing

```bash
# Backend tests
cd backend
go test ./...

# Frontend tests
cd frontend
npm test
```

### Linting

```bash
# Backend
cd backend
go vet ./...
golangci-lint run

# Frontend
cd frontend
npm run lint
```

## Environment Variables

See [DEPLOYMENT.md](./DEPLOYMENT.md) for a complete list of environment variables.

## License

[Add your license here]

## Contributing

[Add contributing guidelines here]

## Support

[Add support information here]
