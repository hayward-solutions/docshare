# DocShare Deployment Guide

Complete guide for deploying DocShare in development and production environments.

> **Ready-to-use examples**: See [examples/](../examples/) for Docker Compose and Helm configurations for common deployment scenarios.

## Table of Contents

1. [Quick Start (Development)](#quick-start-development)
2. [Development Setup](#development-setup)
3. [Production Deployment](#production-deployment)
4. [AWS S3 Setup](#aws-s3-setup)
5. [Environment Variables](#environment-variables)
6. [Database Management](#database-management)
7. [Backup & Recovery](#backup--recovery)
8. [Monitoring](#monitoring)
9. [Security Hardening](#security-hardening)
10. [Scaling](#scaling)
11. [Troubleshooting](#troubleshooting)

---

## Quick Start (Development)

The fastest way to get DocShare running locally:

```bash
# Clone repository
git clone <repository-url>
cd docshare
```

### Choosing the Right Configuration

| File | Use Case | When to Use |
|------|----------|-------------|
| `docker-compose.dev.yml` | **Development** | Building/contributing to DocShare |
| `docker-compose.yml` | **Production** | Running pre-built release images |

### Development (Recommended for contributors)

```bash
# Start all services (builds from local source)
docker-compose -f docker-compose.dev.yml up -d

# Access the application
# Frontend: http://localhost:3001
# Backend: http://localhost:8080
```

### Production

```bash
# Start all services (uses pre-built images)
docker-compose up -d

# Access the application
# Frontend: http://localhost:3001
# Backend: http://localhost:8080
```

That's it! The application is now running with:
- PostgreSQL database
- AWS S3 object storage
- Gotenberg document converter
- Backend API server
- Frontend web application

**Note:** You must have an AWS S3 bucket configured before starting the application. See the [AWS S3 Setup](#aws-s3-setup) section for details.

### First-Time Setup

1. **Register the first user**
   - Navigate to http://localhost:3001/register
   - Create an account
   - First user is automatically assigned admin role

---

## Development Setup

### Prerequisites

**Required:**
- Docker 20.10+ and Docker Compose 2.0+
- Git

**Optional (for local development without Docker):**
- Go 1.24+
- Node.js 22+
- PostgreSQL 16+
- AWS account with S3 bucket

### Option 1: Full Docker Development

**Best for:** Quick start, consistent environment, building from source

```bash
# Start all services (builds from local source)
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.dev.yml logs -f backend
docker-compose -f docker-compose.dev.yml logs -f frontend

# Stop all services
docker-compose -f docker-compose.dev.yml down

# Stop and remove volumes (fresh start)
docker-compose -f docker-compose.dev.yml down -v

# Rebuild after code changes
docker-compose -f docker-compose.dev.yml up -d --build
```

### Option 2: Hybrid Development

**Best for:** Backend or frontend development with live reload

#### Backend Development (Local)

```bash
# Start dependencies only
docker-compose up -d postgres gotenberg

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=docshare
export DB_PASSWORD=docshare_secret
export DB_NAME=docshare
export DB_SSLMODE=disable
export S3_REGION=us-east-1
export S3_BUCKET=docshare
export S3_ACCESS_KEY=<your-aws-access-key>
export S3_SECRET_KEY=<your-aws-secret-key>
export JWT_SECRET=dev-secret-change-in-production
export GOTENBERG_URL=http://localhost:3000
export SERVER_PORT=8080

# Run backend
cd backend
go run cmd/server/main.go
```

#### Frontend Development (Local)

```bash
# Start backend services
docker-compose up -d postgres backend gotenberg

# Set environment variables (optional, defaults to http://localhost:8080)
export NEXT_PUBLIC_API_URL=http://localhost:8080

# Install dependencies
cd frontend
npm install

# Run development server
npm run dev

# Access at http://localhost:3000 (not 3001!)
```

### Option 3: Fully Local Development

**Best for:** Offline development, custom configurations

#### 1. Install Dependencies

**PostgreSQL:**
```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Ubuntu
sudo apt install postgresql-16
sudo systemctl start postgresql

# Create database
createdb docshare
```

**Gotenberg:**
```bash
# Use Docker for Gotenberg (recommended)
docker run -d -p 3000:3000 gotenberg/gotenberg:8
```

#### 2. Configure & Run

Follow the same environment variable setup as Option 2, then run backend and frontend as described above.

---

## Production Deployment

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     Internet                            │
└──────────────────────┬──────────────────────────────────┘
                        │
                        │ HTTPS (443)
                        │
┌──────────────────────▼──────────────────────────────────┐
│              Reverse Proxy (Nginx/Traefik)              │
│  - TLS Termination                                      │
│  - Rate Limiting                                        │
│  - Static Asset Caching                                 │
└────────┬──────────────────────────┬─────────────────────┘
          │                          │
          │ HTTP (3001)              │ HTTP (8080)
          │                          │
┌────────▼────────┐       ┌─────────▼────────┐
│   Frontend      │       │     Backend      │
│   Container     │       │    Container     │
│   (Next.js)     │       │      (Go)        │
└─────────────────┘       └─────────┬────────┘
                                    │
                    ┌────────────────┼──────────────┐
                    │                │              │
            ┌───────▼────────┐ ┌─────▼─────┐ ┌──────▼──────┐
            │   PostgreSQL   │ │  AWS S3   │ │  Gotenberg  │
            │  (Primary DB)  │ │ (Storage) │ │  (Convert)  │
            └────────────────┘ └───────────┘ └─────────────┘
```

### Deployment Options

#### Option 1: Docker Compose (Small to Medium Scale)

**Best for:** Single server deployments, < 1000 users

**Steps:**

1. **Prepare server**
   ```bash
   # Update system
   sudo apt update && sudo apt upgrade -y
   
   # Install Docker
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   
   # Install Docker Compose
   sudo apt install docker-compose-plugin
   ```

2. **Clone and configure**
   ```bash
   git clone <repository-url>
   cd docshare
   
   # Create production environment file
   cp .env.example .env
   nano .env  # Edit configuration (see Environment Variables section)
   ```

3. **Update docker-compose.yml for production**
    ```yaml
    # docker-compose.prod.yml
    services:
      postgres:
        restart: always
        environment:
          POSTGRES_PASSWORD: ${DB_PASSWORD}  # Use strong password
    
      backend:
        restart: always
        environment:
          JWT_SECRET: ${JWT_SECRET}  # Use long random string (32+ chars)
          DB_PASSWORD: ${DB_PASSWORD}
          S3_ACCESS_KEY: ${S3_ACCESS_KEY}
          S3_SECRET_KEY: ${S3_SECRET_KEY}
    
      frontend:
        restart: always
        environment:
          API_URL: http://backend:8080
    ```

4. **Setup reverse proxy (Nginx)**
   ```nginx
   # /etc/nginx/sites-available/docshare
   server {
       listen 80;
       server_name your-domain.com;
       return 301 https://$server_name$request_uri;
   }
   
   server {
       listen 443 ssl http2;
       server_name your-domain.com;
   
       ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
       ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
   
       # Frontend
       location / {
           proxy_pass http://localhost:3001;
           proxy_http_version 1.1;
           proxy_set_header Upgrade $http_upgrade;
           proxy_set_header Connection 'upgrade';
           proxy_set_header Host $host;
           proxy_cache_bypass $http_upgrade;
       }
   
       # Backend API
       location /api {
           proxy_pass http://localhost:8080;
           proxy_http_version 1.1;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
           
           # File upload size limit
           client_max_body_size 100M;
       }
   
       # Rate limiting
       limit_req_zone $binary_remote_addr zone=api:10m rate=100r/m;
       location /api/auth {
           limit_req zone=api burst=5;
           proxy_pass http://localhost:8080;
       }
   }
   ```

5. **Setup SSL with Let's Encrypt**
   ```bash
   sudo apt install certbot python3-certbot-nginx
   sudo certbot --nginx -d your-domain.com
   ```

6. **Deploy**
   ```bash
   docker-compose up -d
   ```

   Or with explicit file:
   ```bash
   docker-compose -f docker-compose.yml up -d
   ```

#### Option 2: Kubernetes with Helm (Large Scale)

**Best for:** Multi-server deployments, high availability, > 10,000 users

DocShare provides an official Helm chart for Kubernetes deployments. Install from the OCI registry:

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  --create-namespace \
  -f production-values.yaml
```

The chart includes:
- Deployments for backend, frontend, and Gotenberg
- Bundled PostgreSQL via Bitnami subchart (or bring your own)
- Ingress with TLS support
- Configurable replicas, resources, and environment
- Secret management (auto-generated or existing secrets)

See the [Helm Chart documentation](HELM.md) for the full configuration reference, production examples, and external database/storage setup.

#### Option 3: Managed Services (Hybrid Cloud)

**Best for:** Minimal infrastructure management

**Architecture:**
- **Frontend**: Vercel, Netlify, or Cloudflare Pages
- **Backend**: AWS ECS, Google Cloud Run, or DigitalOcean App Platform
- **Database**: AWS RDS, Google Cloud SQL, or DigitalOcean Managed Database
- **Storage**: AWS S3, Google Cloud Storage, or DigitalOcean Spaces
- **Document Conversion**: Container on Cloud Run or ECS

---

## AWS S3 Setup

DocShare uses AWS S3 for object storage. This section covers the required setup.

### Bucket Creation

1. **Create an S3 bucket:**
   ```bash
   aws s3 mb s3://docshare-prod --region us-east-1
   ```

2. **Configure bucket for security:**
   ```bash
   # Block public access (recommended)
   aws s3api put-public-access-block \
     --bucket docshare-prod \
     --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
   
   # Enable versioning (optional, for recovery)
   aws s3api put-bucket-versioning \
     --bucket docshare-prod \
     --versioning-configuration Status=Enabled
   
   # Enable encryption
   aws s3api put-bucket-encryption \
     --bucket docshare-prod \
     --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
   ```

3. **Configure lifecycle policy (optional):**
   ```json
   {
     "Rules": [
       {
         "ID": "DeleteOldVersions",
         "Status": "Enabled",
         "NoncurrentVersionExpiration": {
           "NoncurrentDays": 30
         }
       }
     ]
   }
   ```
   ```bash
   aws s3api put-bucket-lifecycle-configuration \
     --bucket docshare-prod \
     --lifecycle-configuration file://lifecycle.json
   ```

### IAM User Authentication (Development/Standalone)

For non-EKS deployments, use IAM user credentials:

1. **Create IAM user:**
   ```bash
   aws iam create-user --user-name docshare-app
   ```

2. **Create access key:**
   ```bash
   aws iam create-access-key --user-name docshare-app
   ```

3. **Attach policy:**
   ```bash
   aws iam attach-user-policy \
     --user-name docshare-app \
     --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess
   ```

### IAM Role Authentication (EKS/Production)

For EKS deployments, use IAM roles for service accounts (IRSA):

1. **Create IAM OIDC provider (if not exists):**
   ```bash
   eksctl utils associate-iam-oidc-provider --cluster your-cluster --approve
   ```

2. **Create IAM policy:**
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Sid": "DocShareS3Access",
         "Effect": "Allow",
         "Action": [
           "s3:PutObject",
           "s3:GetObject",
           "s3:DeleteObject",
           "s3:ListBucket",
           "s3:GetBucketLocation"
         ],
         "Resource": [
           "arn:aws:s3:::docshare-prod",
           "arn:aws:s3:::docshare-prod/*"
         ]
       }
     ]
   }
   ```
   ```bash
   aws iam create-policy \
     --policy-name DocShareS3Access \
     --policy-document file://s3-policy.json
   ```

3. **Create IAM role for service account:**
   ```bash
   eksctl create iamserviceaccount \
     --name docshare-backend \
     --namespace docshare \
     --cluster your-cluster \
     --attach-policy-arn arn:aws:iam::<account-id>:policy/DocShareS3Access \
     --approve \
     --override-existing-serviceaccounts
   ```

4. **Configure environment (no access keys needed):**
   When using IAM roles, leave `S3_ACCESS_KEY` and `S3_SECRET_KEY` empty or unset. The SDK will automatically use the IAM role.

---

## Environment Variables

### Backend Environment Variables

| Variable                | Required | Default                   | Description                                                                          |
|-------------------------|----------|---------------------------|--------------------------------------------------------------------------------------|
| `DB_HOST`               | Yes      | `localhost`               | PostgreSQL host                                                                      |
| `DB_PORT`               | Yes      | `5432`                    | PostgreSQL port                                                                      |
| `DB_USER`               | Yes      | `docshare`                | PostgreSQL username                                                                  |
| `DB_PASSWORD`           | Yes      | `docshare_secret`         | PostgreSQL password                                                                  |
| `DB_NAME`               | Yes      | `docshare`                | PostgreSQL database name                                                             |
| `DB_SSLMODE`            | Yes      | `disable`                 | PostgreSQL SSL mode (`disable`, `require`, `verify-full`)                            |
| `S3_REGION`             | Yes      | `us-east-1`               | AWS region for S3 bucket                                                             |
| `S3_ENDPOINT`           | No       | Auto-derived from region  | S3 endpoint (internal), defaults to s3.$REGION.amazonaws.com                        |
| `S3_PUBLIC_ENDPOINT`    | No       | Same as S3_ENDPOINT       | S3 endpoint (public, for presigned URLs)                                             |
| `S3_ACCESS_KEY`         | No       | (empty)                   | AWS access key (empty = use IAM role)                                                |
| `S3_SECRET_KEY`         | No       | (empty)                   | AWS secret key (empty = use IAM role)                                                |
| `S3_BUCKET`             | Yes      | `docshare`                | S3 bucket name                                                                       |
| `S3_USE_SSL`            | Yes      | `true`                    | Use SSL for S3 connection                                                            |
| `JWT_SECRET`            | Yes      | `change-me-in-production` | JWT signing secret (32+ characters)                                                  |
| `JWT_EXPIRATION_HOURS`  | No       | `24`                      | JWT token lifetime in hours                                                          |
| `GOTENBERG_URL`         | Yes      | `http://localhost:3000`   | Gotenberg service URL                                                                |
| `SERVER_PORT`           | No       | `8080`                    | Backend server port                                                                  |
| `AUDIT_EXPORT_INTERVAL` | No       | `1h`                      | Interval for exporting audit logs to S3 (Go duration format, e.g. `30m`, `2h`)       |

### Frontend Environment Variables

| Variable  | Required | Default                 | Description                                          |
|-----------|----------|-------------------------|------------------------------------------------------|
| `API_URL` | No       | (empty)                 | Backend API URL (server-side, for SSR if needed)     |

**Note:** `NEXT_PUBLIC_API_URL` is embedded at build time, not runtime. For production, it's set to empty string, making API calls relative (`/api/...`). The reverse proxy routes `/api` to the backend. For local development, set it via shell: `NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev`.

### Production Environment Variable Recommendations

```bash
# Backend (.env.backend.prod)
DB_HOST=postgres-prod.example.com
DB_PORT=5432
DB_USER=docshare_prod
DB_PASSWORD=<generate-strong-password>  # Use password manager
DB_NAME=docshare_production
DB_SSLMODE=require

S3_REGION=us-east-1
S3_BUCKET=docshare-prod
S3_ACCESS_KEY=<aws-access-key>  # Or leave empty for IAM role
S3_SECRET_KEY=<aws-secret-key>  # Or leave empty for IAM role

JWT_SECRET=<generate-random-64-char-string>  # openssl rand -hex 32
JWT_EXPIRATION_HOURS=24

GOTENBERG_URL=http://gotenberg:3000
SERVER_PORT=8080
AUDIT_EXPORT_INTERVAL=1h
```

```bash
# Frontend (.env.frontend.prod)
# NEXT_PUBLIC_API_URL is set to empty at build time for relative API paths
# No environment variables needed - reverse proxy handles routing
```

### Generating Secrets

```bash
# JWT Secret (64 characters)
openssl rand -hex 32

# Database Password (32 characters)
openssl rand -base64 32

# AWS credentials - use IAM roles in production
# For development, create access keys in AWS IAM console
```

---

## Database Management

### Migrations

DocShare uses GORM's AutoMigrate feature. Database schema is automatically created/updated on application startup.

**Models migrated:**
- Users
- Files
- Groups
- GroupMemberships
- Shares
- AuditLogs
- AuditExportCursors
- Activities

### Manual Migration (If needed)

If you need to run migrations manually or create custom migrations:

```bash
# Connect to PostgreSQL
psql -h localhost -U docshare -d docshare

# Example: Add index
CREATE INDEX idx_files_created_at ON files(created_at);

# Example: Add column
ALTER TABLE users ADD COLUMN phone_number VARCHAR(20);
```

### Database Backup

**Automated daily backups:**

```bash
#!/bin/bash
# /usr/local/bin/backup-docshare.sh

BACKUP_DIR="/backups/docshare"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/docshare_$DATE.sql.gz"

# PostgreSQL backup
docker exec docshare-postgres pg_dump -U docshare docshare | gzip > $BACKUP_FILE

# Retain only last 7 days
find $BACKUP_DIR -name "*.sql.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_FILE"
```

**Setup cron job:**
```bash
# Run daily at 2 AM
crontab -e
0 2 * * * /usr/local/bin/backup-docshare.sh >> /var/log/docshare-backup.log 2>&1
```

### Database Restore

```bash
# Stop application
docker-compose down backend

# Restore database
gunzip -c /backups/docshare/docshare_20240211_020000.sql.gz | \
  docker exec -i docshare-postgres psql -U docshare docshare

# Restart application
docker-compose up -d backend
```

### Database Maintenance

**Regular maintenance tasks:**

```sql
-- Vacuum (reclaim storage)
VACUUM ANALYZE;

-- Reindex (rebuild indexes)
REINDEX DATABASE docshare;

-- Check database size
SELECT pg_size_pretty(pg_database_size('docshare'));

-- Check table sizes
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Clean up expired shares
DELETE FROM shares WHERE expires_at < NOW();
```

**Automate with cron:**
```bash
# Weekly maintenance on Sundays at 3 AM
0 3 * * 0 docker exec docshare-postgres psql -U docshare -d docshare -c "VACUUM ANALYZE;" >> /var/log/docshare-maintenance.log 2>&1
```

---

## Backup & Recovery

### Full Backup Strategy

**What to backup:**
1. PostgreSQL database (metadata)
2. S3 bucket versioning (enabled for file recovery)
3. Environment configuration files
4. SSL certificates

**Note:** S3 handles file storage backup through versioning and cross-region replication. No manual backup needed for file content.

**Backup script:**

```bash
#!/bin/bash
# /usr/local/bin/full-backup-docshare.sh

BACKUP_ROOT="/backups/docshare"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$BACKUP_ROOT/$DATE"

mkdir -p $BACKUP_DIR

echo "Starting full backup: $DATE"

# 1. PostgreSQL
echo "Backing up database..."
docker exec docshare-postgres pg_dump -U docshare docshare | \
  gzip > $BACKUP_DIR/database.sql.gz

# 2. Configuration
echo "Backing up configuration..."
cp .env $BACKUP_DIR/
cp docker-compose.yml $BACKUP_DIR/

# 3. SSL certificates
echo "Backing up SSL certificates..."
if [ -d "/etc/letsencrypt" ]; then
  sudo tar czf $BACKUP_DIR/ssl-certs.tar.gz /etc/letsencrypt
fi

# Create manifest
cat > $BACKUP_DIR/manifest.txt << EOF
Backup Date: $DATE
Database: database.sql.gz
Configuration: .env, docker-compose.yml
SSL Certificates: ssl-certs.tar.gz
EOF

# Compress entire backup
cd $BACKUP_ROOT
tar czf "docshare_full_$DATE.tar.gz" $DATE
rm -rf $DATE

echo "Backup completed: docshare_full_$DATE.tar.gz"

# Optional: Upload to remote storage
# aws s3 cp "docshare_full_$DATE.tar.gz" s3://your-backup-bucket/
```

### Disaster Recovery Procedure

**Scenario: Complete server failure**

1. **Provision new server**
   ```bash
   # Install Docker & Docker Compose
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   ```

2. **Restore backup**
   ```bash
   # Download backup (if stored remotely)
   aws s3 cp s3://your-backup-bucket/docshare_full_20240211_020000.tar.gz .
   
   # Extract backup
   tar xzf docshare_full_20240211_020000.tar.gz
   cd 20240211_020000
   
   # Restore configuration
   cp .env ../
   cp docker-compose.yml ../
   ```

3. **Start infrastructure services**
    ```bash
    cd ..
    docker-compose up -d postgres
    
    # Wait for services to be ready
    sleep 10
    ```

4. **Restore database**
   ```bash
gunzip -c 20240211_020000/database.sql.gz | \
      docker exec -i docshare-postgres psql -U docshare docshare
    ```

5. **Restore SSL certificates**
   ```bash
sudo tar xzf 20240211_020000/ssl-certs.tar.gz -C /
    ```

6. **Start application**
   ```bash
docker-compose up -d
    ```

7. **Verify**
   ```bash
   # Check all services are running
   docker-compose ps
   
   # Check logs
   docker-compose logs -f backend
   
   # Test login
   curl https://your-domain.com/api/health
   ```

**Expected recovery time:** 30-60 minutes

---

## Monitoring

### Health Checks

**Backend health endpoint:**
```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

**Container health:**
```bash
docker-compose ps
# All containers should show "Up" and "healthy"
```

### Logging

**View logs:**
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f backend
docker-compose logs -f frontend

# Last 100 lines
docker-compose logs --tail=100 backend

# With timestamps
docker-compose logs -f -t backend
```

**Centralized logging (production):**

Use a log aggregation solution:
- **ELK Stack** (Elasticsearch, Logstash, Kibana)
- **Loki + Grafana**
- **Datadog**
- **CloudWatch Logs** (AWS)

**Example: Ship logs to Loki**
```yaml
# docker-compose.yml
services:
  backend:
    logging:
      driver: loki
      options:
        loki-url: "http://loki:3100/loki/api/v1/push"
```

### Metrics

**Key metrics to monitor:**

1. **Application Metrics:**
   - Request rate (requests/sec)
   - Response time (p50, p95, p99)
   - Error rate (4xx, 5xx)
   - Active users

2. **System Metrics:**
   - CPU usage
   - Memory usage
   - Disk usage
   - Network I/O

3. **Database Metrics:**
   - Connection count
   - Query performance
   - Slow queries
   - Database size

4. **Storage Metrics:**
   - Object count
   - Storage usage
   - Upload/download rate

**Prometheus + Grafana setup:**

```yaml
# docker-compose.monitoring.yml
services:
  prometheus:
    image: prom/prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9090:9090"
  
  grafana:
    image: grafana/grafana
    volumes:
      - grafana_data:/var/lib/grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

volumes:
  prometheus_data:
  grafana_data:
```

### Alerts

**Critical alerts to configure:**

1. Service down (any container stopped)
2. High error rate (>5% 5xx responses)
3. High response time (p95 > 2 seconds)
4. Database connection failures
5. Disk usage > 80%
6. Memory usage > 90%

**Example: Alertmanager configuration**
```yaml
# alertmanager.yml
route:
  receiver: 'email'
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h

receivers:
  - name: 'email'
    email_configs:
      - to: 'ops@example.com'
        from: 'alerts@example.com'
        smarthost: 'smtp.gmail.com:587'
        auth_username: 'alerts@example.com'
        auth_password: 'password'
```

---

## Security Hardening

### 1. Change Default Credentials

**CRITICAL: Before going to production**

- [ ] Change PostgreSQL password
- [ ] Configure AWS IAM roles or secure access keys for S3
- [ ] Generate strong JWT secret (32+ characters)
- [ ] Update admin user password

### 2. Network Security

**Firewall rules:**
```bash
# Allow only necessary ports
sudo ufw allow 22    # SSH
sudo ufw allow 80    # HTTP
sudo ufw allow 443   # HTTPS
sudo ufw enable

# Block direct access to internal services
sudo ufw deny 5432   # PostgreSQL
sudo ufw deny 8080   # Backend (use reverse proxy)
```

**Docker network isolation:**
```yaml
# docker-compose.yml
services:
  backend:
    networks:
      - frontend
      - backend
  
  postgres:
    networks:
      - backend  # Not accessible from frontend network
  
  frontend:
    networks:
      - frontend

networks:
  frontend:
  backend:
    internal: true  # No external access
```

### 3. SSL/TLS Configuration

**Strong SSL configuration (Nginx):**
```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
ssl_prefer_server_ciphers off;
ssl_session_timeout 1d;
ssl_session_cache shared:SSL:50m;
ssl_stapling on;
ssl_stapling_verify on;
add_header Strict-Transport-Security "max-age=63072000" always;
```

### 4. Database Security

**PostgreSQL hardening:**
```sql
-- Revoke public schema access
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Create read-only user for backups
CREATE USER docshare_backup WITH PASSWORD 'strong-password';
GRANT CONNECT ON DATABASE docshare TO docshare_backup;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO docshare_backup;

-- Limit connections
ALTER DATABASE docshare CONNECTION LIMIT 100;
```

### 5. Application Security

**Content Security Policy (CSP):**
```nginx
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https://api.your-domain.com";
```

**Additional headers:**
```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
```

### 6. Rate Limiting

**Nginx rate limiting:**
```nginx
# Define rate limit zones
limit_req_zone $binary_remote_addr zone=auth:10m rate=5r/m;
limit_req_zone $binary_remote_addr zone=api:10m rate=100r/m;
limit_req_zone $binary_remote_addr zone=upload:10m rate=10r/m;

server {
    # Auth endpoints (strict)
    location /api/auth {
        limit_req zone=auth burst=5 nodelay;
        proxy_pass http://backend;
    }
    
    # Upload endpoints
    location /api/files/upload {
        limit_req zone=upload burst=3 nodelay;
        proxy_pass http://backend;
    }
    
    # General API
    location /api {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://backend;
    }
}
```

### 7. Regular Updates

**Automated security updates (Ubuntu):**
```bash
sudo apt install unattended-upgrades
sudo dpkg-reconfigure --priority=low unattended-upgrades
```

**Update containers regularly:**
```bash
# Pull latest images
docker-compose pull

# Recreate containers
docker-compose up -d

# Remove old images
docker image prune -a
```

### 8. Distroless Container Images

DocShare uses Google's distroless container images for production deployments, providing:

- **Reduced attack surface** — No shell, package manager, or unnecessary utilities
- **Smaller image sizes** — ~50-70% smaller than Alpine-based images
- **Fewer CVEs** — Minimal packages mean fewer vulnerabilities to patch

**Runtime images used:**

| Service | Distroless Image |
|---------|-----------------|
| Backend (Go) | `gcr.io/distroless/static-debian12` |
| Frontend (Node.js) | `gcr.io/distroless/nodejs22-debian12` |

**Debugging distroless containers:**

For debugging, use the `:debug` variants:
```bash
# Development docker-compose.yml
services:
  backend:
    image: gcr.io/distroless/static-debian12:debug

# Then you can exec into the container
docker exec -it docshare-backend sh
```

**Note:** Distroless images run as non-root by default. If you need root access for debugging, use the debug variants.

---

## Scaling

### Vertical Scaling

**Increase resources for existing containers:**

```yaml
# docker-compose.yml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
  
  postgres:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
```

### Horizontal Scaling

#### 1. Load Balanced Backend

**Nginx upstream configuration:**
```nginx
upstream backend {
    least_conn;
    server backend1:8080;
    server backend2:8080;
    server backend3:8080;
}

server {
    location /api {
        proxy_pass http://backend;
    }
}
```

**Docker Compose with replicas:**
```yaml
services:
  backend:
    image: docshare-backend
    deploy:
      replicas: 3
    environment:
      # ... same environment variables
```

#### 2. Database Read Replicas

**PostgreSQL replication:**
```yaml
services:
  postgres-primary:
    image: postgres:16-alpine
    environment:
      POSTGRES_REPLICATION_MODE: master
      POSTGRES_REPLICATION_USER: replicator
      POSTGRES_REPLICATION_PASSWORD: replication_pass
  
  postgres-replica:
    image: postgres:16-alpine
    environment:
      POSTGRES_REPLICATION_MODE: slave
      POSTGRES_MASTER_HOST: postgres-primary
      POSTGRES_REPLICATION_USER: replicator
      POSTGRES_REPLICATION_PASSWORD: replication_pass
```

**Configure backend to use read replicas:**
```go
// Separate read/write connections
dbWrite := ConnectPostgres(primaryHost)
dbRead := ConnectPostgres(replicaHost)

// Use read connection for queries
files, err := dbRead.Find(&files)

// Use write connection for mutations
err := dbWrite.Create(&file)
```

#### 3. Distributed Storage

**AWS S3 provides built-in durability and availability:**
- 99.999999999% (11 9s) durability
- Cross-region replication for disaster recovery
- S3 Transfer Acceleration for faster uploads
- No additional infrastructure needed

### Caching

**Add Redis for caching:**
```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
```

**Cache strategies:**
1. **User data cache** - Cache user objects after login
2. **Permission cache** - Cache expensive permission checks
3. **File metadata cache** - Cache frequently accessed file details
4. **Session cache** - Store JWT tokens with expiration

---

## Troubleshooting

### Common Issues

#### 1. Container won't start

**Symptom:** `docker-compose ps` shows container as "Restarting" or "Exit 1"

**Solution:**
```bash
# Check logs
docker-compose logs backend

# Common causes:
# - Environment variable missing
# - Database connection failed
# - Port already in use

# Check port conflicts
sudo lsof -i :8080
```

#### 2. Cannot connect to database

**Symptom:** Backend logs show "connection refused" or "password authentication failed"

**Solution:**
```bash
# Verify PostgreSQL is running
docker-compose ps postgres

# Check PostgreSQL logs
docker-compose logs postgres

# Test connection manually
docker exec -it docshare-postgres psql -U docshare -d docshare

# Verify environment variables match
docker-compose exec backend env | grep DB_
```

#### 3. File upload fails

**Symptom:** 400 or 500 error on upload, or upload hangs

**Solution:**
```bash
# Verify S3 bucket exists
aws s3 ls s3://docshare-prod

# Check IAM permissions
aws sts get-caller-identity

# Check backend can reach S3
docker-compose exec backend curl https://s3.amazonaws.com

# Check file size limit (backend)
# Default: 100MB in cmd/server/main.go

# Check Nginx file size limit
# client_max_body_size 100M;
```

#### 4. Preview generation fails

**Symptom:** Office documents don't show preview

**Solution:**
```bash
# Check Gotenberg is running
docker-compose ps gotenberg

# Test Gotenberg manually
curl -X POST http://localhost:3000/forms/libreoffice/convert \
  -F file=@test.docx \
  -o test.pdf

# Check backend can reach Gotenberg
docker-compose exec backend curl http://gotenberg:3000/health
```

#### 5. High memory usage

**Symptom:** System running slow, OOM errors

**Solution:**
```bash
# Check container memory usage
docker stats

# Identify memory hog
docker stats --no-stream --format "table {{.Container}}\t{{.MemUsage}}"

# Common causes:
# - Too many concurrent uploads
# - Large preview generations
# - Database query without limit

# Add memory limits
# See docker-compose.yml deploy.resources section
```

#### 6. Slow queries

**Symptom:** API responses are slow, database CPU high

**Solution:**
```sql
-- Find slow queries
SELECT 
  pid,
  now() - query_start AS duration,
  query
FROM pg_stat_activity
WHERE state = 'active'
ORDER BY duration DESC;

-- Check missing indexes
SELECT 
  schemaname,
  tablename,
  attname,
  n_distinct,
  correlation
FROM pg_stats
WHERE schemaname = 'public'
ORDER BY abs(correlation) ASC;

-- Add missing indexes
CREATE INDEX idx_shares_file_id_permission ON shares(file_id, permission);
```

#### 7. SSL certificate issues

**Symptom:** "Certificate has expired" or "Certificate not trusted"

**Solution:**
```bash
# Check certificate expiration
sudo certbot certificates

# Renew certificate
sudo certbot renew

# Test auto-renewal
sudo certbot renew --dry-run

# Force renewal
sudo certbot renew --force-renewal

# Reload Nginx
sudo systemctl reload nginx
```

### Debug Mode

**Enable verbose logging:**

```bash
# Backend (Go)
# Set log level to debug in pkg/logger/logger.go

# Frontend (Next.js)
export DEBUG=*
npm run dev

# PostgreSQL
# Edit postgresql.conf
log_statement = 'all'
log_duration = on
```

### Performance Profiling

**Backend profiling:**
```bash
# Add pprof to main.go
import _ "net/http/pprof"

# Start profiling server
go tool pprof http://localhost:6060/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap
```

**Database profiling:**
```sql
-- Enable query timing
\timing

-- Analyze query plan
EXPLAIN ANALYZE SELECT * FROM files WHERE owner_id = 'uuid';

-- Check index usage
SELECT * FROM pg_stat_user_indexes;
```

### Getting Help

1. **Check logs first** - Most issues are visible in logs
2. **Search issues** - Check GitHub issues for similar problems
3. **Ask community** - Join Discord/Slack for help
4. **Create issue** - Provide logs, configuration, and reproduction steps

---

## Additional Resources

- [README.md](../README.md) - Project overview
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture details
- [API.md](API.md) - API documentation
- Docker Documentation: https://docs.docker.com/
- PostgreSQL Documentation: https://www.postgresql.org/docs/
- AWS S3 Documentation: https://docs.aws.amazon.com/s3/
- Nginx Documentation: https://nginx.org/en/docs/

---

**Last Updated:** February 2026
