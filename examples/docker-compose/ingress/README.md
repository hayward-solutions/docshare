# Ingress Examples

This directory contains Docker Compose configurations with different reverse proxy solutions for exposing DocShare.

## Available Examples

| Example | Description | TLS | Best For |
|---------|-------------|-----|----------|
| [Caddy](caddy/) | Automatic HTTPS with Let's Encrypt | Automatic | Production, ease of use |
| [Nginx](nginx/) | Traditional reverse proxy | Self-signed (demo) | Custom configurations, familiarity |
| [Traefik](traefik/) | Modern proxy with auto-discovery | Let's Encrypt (optional) | Microservices, dynamic config |
| [Tailscale](tailscale/) | Private access via tailnet | MagicDNS (built-in) | Internal tools, zero-trust access |

## Architecture

All examples follow the same pattern:

```
                    ┌─────────────────┐
                    │  Reverse Proxy  │
                    │ (Caddy/Nginx/   │
                    │  Traefik/TS)    │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │ Frontend │  │ Backend  │  │ Gotenberg│
        │  :3000   │  │  :8080   │  │  :3000   │
        └──────────┘  └──────────┘  └──────────┘
```

The reverse proxy routes:
- `/` → Frontend (port 3000)
- `/api` → Backend (port 8080)

## Quick Start

### Caddy (Recommended for Production)

```bash
cd caddy
export DOMAIN=docshare.example.com
export EMAIL=admin@example.com
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket
docker-compose up -d
```

### Tailscale (Private Access)

```bash
cd tailscale
export TS_AUTHKEY=tskey-auth-xxxxx
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket
docker-compose up -d
```

## Prerequisites

- Docker and Docker Compose
- AWS S3 bucket (or S3-compatible storage)
- Domain name (for Caddy/Traefik with Let's Encrypt)
- Tailscale auth key (for Tailscale example)

## Choosing an Ingress

| Need | Recommended |
|------|-------------|
| Automatic HTTPS | Caddy or Traefik |
| Private access only | Tailscale |
| Custom routing rules | Nginx |
| Kubernetes-like experience | Traefik |
| Simplest setup | Caddy |