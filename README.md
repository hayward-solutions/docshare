# DocShare

A modern, secure document sharing platform. Upload, share, and manage your files with granular permission controls.

## Why DocShare?

DocShare was created to solve a simple problem: I need a private server to host my documents. It needs to support object storage, and it shouldn't be bloated with features I don't need.

## Features

- **Simple sharing** — Share files with individuals or groups in just a few clicks
- **Permission controls** — Three levels: view, download, or edit
- **Group management** — Organise users into teams with owner/admin/member roles
- **Document previews** — Preview Office documents, PDFs, and images directly in the browser
- **Activity tracking** — Know what's happening with file uploads, shares, and group changes
- **Audit logging** — Complete trail of all actions for compliance and security
- **API tokens** — Generate long-lived personal access tokens for CLI and programmatic use
- **Device flow** — Authenticate CLI tools and apps via browser approval (OAuth2 RFC 8628)
- **CLI tool** — Upload, download, share, and manage files from the terminal

## Goals

See [ROADMAP.md](./docs/ROADMAP.md) for feature priorities and future plans.

## CLI

Install the CLI to manage files from your terminal:

```bash
curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh
```

```bash
docshare login                           # Authenticate via browser
docshare upload report.pdf /Documents    # Upload a file
docshare download /Documents/report.pdf  # Download a file
docshare ls /Documents                   # List files
```

See the [CLI documentation](./docs/CLI.md) for the full command reference.

## Quick Start

DocShare provides two Docker Compose configurations:

| File | Use Case | Storage | Builds From |
|------|----------|---------|-------------|
| `docker-compose.yml` | **Production** | AWS S3 | Pre-built GHCR images |
| `docker-compose.dev.yml` | **Development** | MinIO (local) | Local source code |

### Development (Recommended for contributors)

```bash
# Clone repository
git clone https://github.com/hayward-solutions/docshare.git
cd docshare

# Start all services (builds from local source, uses local MinIO storage)
docker-compose -f docker-compose.dev.yml up -d

# Access the application
# Frontend: http://localhost:3001
# Backend: http://localhost:8080
# MinIO Console: http://localhost:9001
```

### Production

```bash
# Clone repository
git clone https://github.com/hayward-solutions/docshare.git
cd docshare

# Set required environment variables
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export S3_BUCKET=your-s3-bucket

# Start all services (uses pre-built images, requires AWS S3)
docker-compose up -d

# Access the application
# Frontend: http://localhost:3001
# Backend: http://localhost:8080
```

**Note:** Production deployments require an AWS S3 bucket. See [Deployment Guide](./docs/DEPLOYMENT.md) for full setup instructions.

Then open http://localhost:3001 and create your first account.

## Documentation

- [Deployment](./docs/DEPLOYMENT.md) — Getting started and production setup
- [Architecture](./docs/ARCHITECTURE.md) — System design and technical details
- [API Reference](./docs/API.md) — REST API documentation
- [CLI](./docs/CLI.md) — CLI installation and command reference
- [Helm Chart](./docs/HELM.md) — Kubernetes deployment with Helm
- [SSO](./docs/SSO.md) — Single sign-on configuration
- [Security](./docs/SECURITY.md) — Security policy and best practices
- [Contributing](./CONTRIBUTING.md) — Development setup and guidelines
- [Roadmap](./docs/ROADMAP.md) — Feature priorities and future plans

## Deployment Examples

See [examples/](./examples/) for ready-to-use configurations:

- **Docker Compose**: Minimal, external database, S3-compatible storage, full with SSO
- **Helm**: Minimal, production, external database, high availability
- **SSO**: Google, GitHub, Keycloak, LDAP
