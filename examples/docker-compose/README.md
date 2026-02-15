# Docker Compose Deployment Examples

This directory contains example Docker Compose configurations for different deployment scenarios.

## Available Examples

| Example | Description | Use Case |
|---------|-------------|----------|
| [minimal](minimal/) | Basic deployment with AWS S3 | Quick evaluation, simple setups |
| [external-db](external-db/) | External PostgreSQL database | Production with managed database |
| [s3-compatible](s3-compatible/) | S3-compatible storage (MinIO, Wasabi, etc.) | Self-hosted or alternative storage |
| [full](full/) | Full stack with SSO | Enterprise deployments |

## Quick Reference

### Minimal (AWS S3)

```bash
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket
docker-compose -f minimal/docker-compose.yml up -d
```

### S3-Compatible (MinIO)

```bash
docker-compose -f s3-compatible/docker-compose.yml up -d
```

### External Database

```bash
export DB_HOST=your-db-host
export DB_PASSWORD=your-db-password
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket
docker-compose -f external-db/docker-compose.yml up -d
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AWS_ACCESS_KEY_ID` | Yes* | AWS access key (required for AWS S3) |
| `AWS_SECRET_ACCESS_KEY` | Yes* | AWS secret key (required for AWS S3) |
| `S3_BUCKET` | Yes | S3 bucket name |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `DB_HOST` | No | Database host (default: `postgres`) |
| `DB_PASSWORD` | No | Database password (default: `docshare_secret`) |
| `JWT_SECRET` | No | JWT signing secret (generate with `openssl rand -hex 32`) |

\* Required for AWS S3; not needed when using IAM roles or S3-compatible storage with different auth.

## Production Checklist

Before deploying to production:

- [ ] Generate strong JWT secret (`openssl rand -hex 32`)
- [ ] Use strong database password
- [ ] Configure proper CORS origins
- [ ] Set up TLS/SSL with reverse proxy
- [ ] Configure S3 bucket permissions
- [ ] Enable S3 bucket versioning (optional)

For detailed instructions, see the [Deployment Guide](../../docs/DEPLOYMENT.md).