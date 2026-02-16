# Caddy Reverse Proxy Example

Caddy provides automatic HTTPS with Let's Encrypt, making it the easiest option for production deployments.

## Features

- Automatic HTTPS (Let's Encrypt)
- HTTP/3 support
- Simple Caddyfile configuration
- Automatic HTTP to HTTPS redirect
- No manual certificate management

## Quick Start

```bash
# Set required environment variables
export DOMAIN=docshare.example.com
export EMAIL=admin@example.com
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket

# Start services
docker-compose up -d
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DOMAIN` | Yes | Your domain name (e.g., `docshare.example.com`) |
| `EMAIL` | Yes | Email for Let's Encrypt registration |
| `AWS_ACCESS_KEY_ID` | Yes | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS secret key |
| `S3_BUCKET` | Yes | S3 bucket name |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `JWT_SECRET` | No | JWT secret (auto-generated if not set) |

## DNS Configuration

Ensure your domain points to the server running Docker:

```
docshare.example.com.  IN  A  your-server-ip
```

## Production Notes

1. **Ports**: Caddy listens on 80 (HTTP) and 443 (HTTPS/HTTP3)
2. **Certificates**: Stored in the `caddy_data` volume
3. **Rate Limits**: Let's Encrypt has rate limits; use staging for testing:
   ```bash
   # Add to Caddyfile for staging
   email {$EMAIL}
   acme_ca https://acme-staging-v02.api.letsencrypt.org/directory
   ```

## Customization

### File Upload Size

The default timeout is 120 seconds. For larger files, increase in `Caddyfile`:

```
transport http {
    read_timeout 300s
    write_timeout 300s
}
```

### Basic Auth

Add authentication to protect your instance:

```
basicauth {
    admin $2a$14$hashed_password
}
```

Generate password hash:
```bash
docker exec -it caddy caddy hash-password --plaintext 'your-password'
```