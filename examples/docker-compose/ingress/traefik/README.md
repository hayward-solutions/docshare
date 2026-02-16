# Traefik Reverse Proxy Example

Traefik is a modern reverse proxy designed for microservices. It automatically discovers services via Docker labels and supports dynamic configuration.

## Features

- Auto-discovery via Docker labels
- Built-in Let's Encrypt support
- Dashboard for monitoring
- HTTP/3 support
- Middleware system for custom logic

## Quick Start

```bash
# Set required environment variables
export DOMAIN=docshare.example.com
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket

# Start services
docker-compose up -d
```

Access at https://localhost (self-signed cert) or http://localhost

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AWS_ACCESS_KEY_ID` | Yes | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS secret key |
| `S3_BUCKET` | Yes | S3 bucket name |
| `DOMAIN` | No | Your domain (default: `localhost`) |
| `EMAIL` | No | Email for Let's Encrypt |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `JWT_SECRET` | No | JWT secret |

## Let's Encrypt Configuration

For production with automatic TLS:

1. Uncomment the Let's Encrypt lines in `docker-compose.yml`
2. Set the `EMAIL` environment variable
3. Point your domain DNS to the server
4. Add the TLS resolver to routes:

```yaml
labels:
  - "traefik.http.routers.frontend.tls.certresolver=letsencrypt"
  - "traefik.http.routers.backend.tls.certresolver=letsencrypt"
```

## Dashboard

Access the Traefik dashboard at `https://traefik.<your-domain>/`

### Dashboard Authentication

Generate a password:

```bash
htpasswd -nB admin
```

Add to the dashboard labels:

```yaml
labels:
  - "traefik.http.middlewares.dashboard-auth.basicauth.users=admin:$$2y$$05$$..."
  - "traefik.http.routers.dashboard.middlewares=dashboard-auth"
```

## Routing Labels

The routing is configured via labels:

| Label | Purpose |
|-------|---------|
| `traefik.enable=true` | Enable Traefik for this service |
| `traefik.http.routers.*.rule` | Routing rule (PathPrefix, Host, etc.) |
| `traefik.http.routers.*.entrypoints` | Entrypoint (web, websecure) |
| `traefik.http.services.*.loadbalancer.server.port` | Target port |

## Customization

### Additional Middleware

```yaml
labels:
  # Rate limiting
  - "traefik.http.middlewares.rate-limit.ratelimit.average=100"
  - "traefik.http.routers.backend.middlewares=rate-limit"
  
  # Custom headers
  - "traefik.http.middlewares.security.headers.frameDeny=true"
  - "traefik.http.middlewares.security.headers.browserXssFilter=true"
```

### File Upload Size

Traefik doesn't limit body size by default. Configure via backend middleware if needed.