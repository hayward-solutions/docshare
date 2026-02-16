---
hide:
  - navigation
  - toc
---

# DocShare

A modern, secure document sharing platform. Upload, share, and manage your files with granular permission controls.

## Why DocShare?

DocShare was created to solve a simple problem: a private server to host documents. It supports object storage, without bloat.

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

See the [CLI documentation](CLI.md) for the full command reference.

## Quick Start

DocShare provides two Docker Compose configurations:

| File | Use Case | Storage | Builds From |
|------|----------|---------|-------------|
| `docker-compose.yml` | **Production** | AWS S3 | Pre-built GHCR images |
| `docker-compose.dev.yml` | **Development** | MinIO (local) | Local source code |

### Development

```bash
git clone https://github.com/hayward-solutions/docshare.git
cd docshare
docker-compose -f docker-compose.dev.yml up -d
```

Access the application at http://localhost:3001

### Production

```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export S3_BUCKET=your-s3-bucket
docker-compose up -d
```

## Documentation

<div class="grid cards" markdown>

-   :material-rocket-launch: **Getting Started**

    ---

    [Deployment Guide](DEPLOYMENT.md) — Getting started and production setup

-   :material-application-brackets: **Architecture**

    ---

    [Architecture Overview](ARCHITECTURE.md) — System design and technical details

-   :material-api: **API Reference**

    ---

    [REST API Documentation](API.md) — Complete endpoint reference

-   :material-console: **CLI Reference**

    ---

    [CLI Documentation](CLI.md) — Installation and command reference

-   :material-kubernetes: **Kubernetes**

    ---

    [Helm Chart](HELM.md) — Kubernetes deployment with Helm

-   :material-shield-lock: **Security**

    ---

    [Security Policy](SECURITY.md) — Security policy and best practices

-   :material-account-key: **SSO**

    ---

    [Single Sign-On](SSO.md) — SSO configuration for providers

-   :material-road-variant: **Roadmap**

    ---

    [Feature Roadmap](ROADMAP.md) — Feature priorities and future plans

</div>

## Examples

Ready-to-use configurations for various deployment scenarios:

- **Docker Compose**: Minimal, external database, S3-compatible storage, full with SSO
- **Helm**: Minimal, production, external database, high availability
- **SSO**: Google, GitHub, Keycloak, LDAP

See [Examples Overview](examples/index.md) for all available configurations.

## Contributing

See [Contributing Guide](contributing.md) for development setup and guidelines.