# Nginx Reverse Proxy Example

Nginx is a battle-tested reverse proxy with extensive configuration options. This example provides a simple HTTP setup for development, with HTTPS options for production.

## Features

- Flexible routing configuration
- Fine-grained control over proxy settings
- Widely documented and supported
- Production-ready with HTTPS

## Quick Start

```bash
# Set required environment variables
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket

# Start services
docker-compose up -d
```

Access at http://localhost

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AWS_ACCESS_KEY_ID` | Yes | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS secret key |
| `S3_BUCKET` | Yes | S3 bucket name |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `JWT_SECRET` | No | JWT secret (auto-generated if not set) |

## HTTPS Configuration

For production, generate certificates and uncomment the HTTPS section in `nginx.conf`:

### Self-Signed Certificate (Testing)

```bash
mkdir certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem -out certs/cert.pem \
  -subj "/CN=localhost"
```

### Let's Encrypt Certificate (Production)

```bash
# Install certbot
sudo apt install certbot

# Generate certificate
sudo certbot certonly --standalone -d your-domain.com

# Copy certificates
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem certs/cert.pem
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem certs/key.pem
```

## Customization

### File Upload Size

Default is 100MB. Change in `nginx.conf`:

```nginx
client_max_body_size 200M;
```

### Rate Limiting

Add to `http` block:

```nginx
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;

# In location /api block:
limit_req zone=api burst=20 nodelay;
```

### Custom Headers

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
```