# DocShare

A modern, self-hosted document sharing platform. Upload, share, and manage files with granular permission controls and S3-compatible object storage.

## Features

- **File sharing** -- Share files with individuals or groups with view, download, or edit permissions
- **Group management** -- Organise users into teams with owner/admin/member roles
- **Document previews** -- Preview Office documents, PDFs, and images in the browser
- **S3 storage** -- Store files in AWS S3 or any S3-compatible backend (MinIO, Wasabi, Backblaze B2)
- **SSO** -- Google, GitHub, Keycloak (OIDC), LDAP, and SAML authentication
- **Audit logging** -- Full trail of all actions with periodic S3 export
- **CLI** -- Upload, download, share, and manage files from the terminal
- **API tokens & device flow** -- Programmatic access via long-lived tokens or OAuth2 device authorization

## Quick Start

The fastest way to evaluate DocShare locally with MinIO (S3-compatible storage):

```bash
git clone https://github.com/hayward-solutions/docshare.git
cd docshare
docker-compose -f docker-compose.dev.yml up -d
```

Open http://localhost:3001 and register your first account (automatically assigned admin).

See the [Deployment Guide](./docs/DEPLOYMENT.md) for production setup, AWS S3 configuration, and all deployment options.

## CLI

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

## Documentation

- [Deployment Guide](./docs/DEPLOYMENT.md) -- Getting started, quick evaluation, and production setup
- [Architecture](./docs/ARCHITECTURE.md) -- System design and technical details
- [API Reference](./docs/API.md) -- REST API documentation
- [CLI](./docs/CLI.md) -- CLI installation and command reference
- [Helm Chart](./docs/HELM.md) -- Kubernetes deployment with Helm
- [SSO](./docs/SSO.md) -- Single sign-on configuration
- [Security](./docs/SECURITY.md) -- Security policy and best practices
- [CloudFormation](./docs/CLOUDFORMATION.md) -- AWS ECS Fargate deployment
- [Contributing](./CONTRIBUTING.md) -- Development setup and guidelines
- [Roadmap](./docs/ROADMAP.md) -- Feature priorities and future plans

## Deployment Examples

The [examples/](./examples/) directory contains ready-to-use configurations:

- **Docker Compose** -- Minimal, S3-compatible (MinIO), external database, full with SSO, and reverse proxy variants (Nginx, Caddy, Traefik, Tailscale)
- **Helm** -- Minimal, production, external database, high availability, SSO, and ingress controller variants
- **CloudFormation** -- AWS ECS Fargate with RDS and S3
