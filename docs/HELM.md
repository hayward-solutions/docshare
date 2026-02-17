# Helm Chart

Deploy DocShare on Kubernetes using the official Helm chart.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.12+
- AWS S3 bucket (or S3-compatible storage)

## Installation

### From OCI Registry (GHCR)

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  --create-namespace
```

### From Source

```bash
git clone https://github.com/hayward-solutions/docshare.git
cd docshare

helm dependency update charts/docshare
helm install docshare charts/docshare \
  --namespace docshare \
  --create-namespace
```

## Quick Start

Install with default settings (bundled PostgreSQL and Gotenberg, AWS S3 for storage):

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  --create-namespace \
  --set backend.env.jwtSecret=my-secret-change-me \
  --set s3.bucket=your-s3-bucket-name
```

Access the application:

```bash
# Port-forward the frontend
kubectl port-forward svc/docshare-frontend 3001:3000 -n docshare

# Port-forward the backend API
kubectl port-forward svc/docshare-backend 8080:8080 -n docshare
```

Then open http://localhost:3001 and create your first account.

## Configuration

### Minimal Production Example

```yaml
# production-values.yaml
backend:
  replicaCount: 2
  env:
    jwtSecret: "generate-with-openssl-rand-hex-32"

frontend:
  replicaCount: 2

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
  hosts:
    - host: docshare.example.com
      paths:
        - path: /
          pathType: Prefix
          service: frontend
        - path: /api
          pathType: Prefix
          service: backend
  tls:
    - secretName: docshare-tls
      hosts:
        - docshare.example.com

postgresql:
  auth:
    password: "generate-a-strong-password"

s3:
  region: "us-east-1"
  bucket: "docshare-prod"
```

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  --create-namespace \
  -f production-values.yaml
```

### Using External PostgreSQL

Disable the bundled PostgreSQL and point to your existing database:

```yaml
postgresql:
  enabled: false

externalDatabase:
  host: postgres.example.com
  port: 5432
  user: docshare
  password: "your-password"
  database: docshare
  sslmode: require
```

Or reference an existing Kubernetes secret:

```yaml
postgresql:
  enabled: false

externalDatabase:
  host: postgres.example.com
  port: 5432
  user: docshare
  database: docshare
  sslmode: require
  existingSecret: my-db-secret
  existingSecretPasswordKey: password
```

### S3 Configuration

#### Using IAM Roles (Recommended for EKS)

For EKS with IRSA (IAM Roles for Service Accounts):

```yaml
backend:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/docshare-s3-role

s3:
  region: "us-east-1"
  bucket: docshare-prod
```

No access keys needed - IAM role handles authentication automatically.

#### Using Static Credentials

```yaml
s3:
  region: "us-west-2"
  accessKey: "your-access-key"
  secretKey: "your-secret-key"
  bucket: docshare-prod
```

#### Custom S3-Compatible Storage

For MinIO, Wasabi, or other S3-compatible storage:

```yaml
s3:
  endpoint: "minio.example.com:9000"
  publicEndpoint: "s3.example.com"
  region: "us-east-1"
  accessKey: "your-access-key"
  secretKey: "your-secret-key"
  bucket: docshare
  useSsl: "true"
```

### Disabling Gotenberg

If you don't need document preview/conversion:

```yaml
gotenberg:
  enabled: false
```

## Values Reference

### Backend

| Key                              | Type   | Default                                      | Description                                          |
|----------------------------------|--------|----------------------------------------------|------------------------------------------------------|
| `backend.replicaCount`           | int    | `1`                                          | Number of backend replicas                           |
| `backend.image.repository`       | string | `ghcr.io/hayward-solutions/docshare/backend` | Backend image repository                             |
| `backend.image.tag`              | string | `latest`                                     | Backend image tag                                    |
| `backend.image.pullPolicy`       | string | `IfNotPresent`                               | Image pull policy                                    |
| `backend.service.type`           | string | `ClusterIP`                                  | Service type                                         |
| `backend.service.port`           | int    | `8080`                                       | Service port                                         |
| `backend.resources`              | object | `{}`                                         | CPU/memory resource requests/limits                  |
| `backend.env.dbSslmode`          | string | `disable`                                    | PostgreSQL SSL mode                                  |
| `backend.env.s3Bucket`           | string | `docshare`                                   | S3 bucket name                                       |
| `backend.env.s3UseSsl`           | string | `"true"`                                     | Use SSL for S3 connection                            |
| `backend.env.jwtSecret`          | string | `""`                                         | JWT signing secret (auto-generated if empty)         |
| `backend.env.jwtExpirationHours` | string | `"24"`                                       | JWT token lifetime                                   |
| `backend.env.serverPort`         | string | `"8080"`                                     | Backend server port                                  |
| `backend.env.gotenbergUrl`       | string | `""`                                         | Gotenberg URL (auto-configured if gotenberg.enabled) |
| `backend.env.frontendUrl`         | string | `""`                                         | Frontend URL (auto-configured from ingress if empty) |
| `backend.env.backendUrl`          | string | `""`                                         | Backend URL (auto-configured from ingress if empty)  |
| `backend.existingSecret`         | string | `""`                                         | Use existing secret for sensitive values             |

### Frontend

| Key                             | Type   | Default                                       | Description                                                     |
|---------------------------------|--------|-----------------------------------------------|-----------------------------------------------------------------|
| `frontend.replicaCount`         | int    | `1`                                           | Number of frontend replicas                                     |
| `frontend.image.repository`     | string | `ghcr.io/hayward-solutions/docshare/frontend` | Frontend image repository                                       |
| `frontend.image.tag`            | string | `latest`                                      | Frontend image tag                                              |
| `frontend.image.pullPolicy`     | string | `IfNotPresent`                                | Image pull policy                                               |
| `frontend.service.type`         | string | `ClusterIP`                                   | Service type                                                    |
| `frontend.service.port`         | int    | `3000`                                        | Service port                                                    |
| `frontend.resources`            | object | `{}`                                          | CPU/memory resource requests/limits                             |

**Note:** The frontend receives `BACKEND_URL` and `FRONTEND_URL` from the Helm chart (auto-derived from ingress config or `backend.env` values), which are passed as environment variables at runtime.

### Gotenberg

| Key                          | Type   | Default               | Description                          |
|------------------------------|--------|-----------------------|--------------------------------------|
| `gotenberg.enabled`          | bool   | `true`                | Enable Gotenberg document conversion |
| `gotenberg.replicaCount`     | int    | `1`                   | Number of replicas                   |
| `gotenberg.image.repository` | string | `gotenberg/gotenberg` | Image repository                     |
| `gotenberg.image.tag`        | string | `"8"`                 | Image tag                            |
| `gotenberg.service.port`     | int    | `3000`                | Service port                         |
| `gotenberg.resources`        | object | `{}`                  | CPU/memory resource requests/limits  |

### Ingress

| Key                   | Type   | Default         | Description                |
|-----------------------|--------|-----------------|----------------------------|
| `ingress.enabled`     | bool   | `false`         | Enable ingress             |
| `ingress.className`   | string | `""`            | Ingress class name         |
| `ingress.annotations` | object | `{}`            | Ingress annotations        |
| `ingress.hosts`       | list   | See values.yaml | Ingress host configuration |
| `ingress.tls`         | list   | `[]`            | TLS configuration          |

### PostgreSQL (Bundled)

Uses the [Bitnami PostgreSQL chart](https://github.com/bitnami/charts/tree/main/bitnami/postgresql). See the upstream chart for all available values.

| Key                        | Type   | Default           | Description               |
|----------------------------|--------|-------------------|---------------------------|
| `postgresql.enabled`       | bool   | `true`            | Deploy bundled PostgreSQL |
| `postgresql.auth.username` | string | `docshare`        | Database username         |
| `postgresql.auth.password` | string | `docshare_secret` | Database password         |
| `postgresql.auth.database` | string | `docshare`        | Database name             |

### S3 Configuration

| Key                                | Type   | Default      | Description                                 |
|------------------------------------|--------|--------------|---------------------------------------------|
| `s3.region`                        | string | `us-east-1`  | AWS region                                  |
| `s3.endpoint`                      | string | `""`         | Custom endpoint (auto: s3.$region.amazonaws.com) |
| `s3.publicEndpoint`                | string | `""`         | Public endpoint for presigned URLs          |
| `s3.accessKey`                     | string | `""`         | Access key (empty = IAM role)               |
| `s3.secretKey`                     | string | `""`         | Secret key (empty = IAM role)               |
| `s3.bucket`                        | string | `docshare`   | Bucket name                                 |
| `s3.useSsl`                        | string | `"true"`     | Use SSL                                     |
| `s3.existingSecret`                | string | `""`         | Existing secret name                        |
| `s3.existingSecretAccessKeyKey`    | string | `access-key` | Key for access key in secret                |
| `s3.existingSecretSecretKeyKey`    | string | `secret-key` | Key for secret key in secret                |

### External Database

| Key                                          | Type   | Default    | Description              |
|----------------------------------------------|--------|------------|--------------------------|
| `externalDatabase.host`                      | string | `""`       | External PostgreSQL host |
| `externalDatabase.port`                      | int    | `5432`     | External PostgreSQL port |
| `externalDatabase.user`                      | string | `docshare` | Database user            |
| `externalDatabase.password`                  | string | `""`       | Database password        |
| `externalDatabase.database`                  | string | `docshare` | Database name            |
| `externalDatabase.sslmode`                   | string | `disable`  | SSL mode                 |
| `externalDatabase.existingSecret`            | string | `""`       | Existing secret name     |
| `externalDatabase.existingSecretPasswordKey` | string | `password` | Key in existing secret   |

## Upgrading

```bash
helm upgrade docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  -f production-values.yaml
```

## Uninstalling

```bash
helm uninstall docshare --namespace docshare
```

Note: The auto-generated secrets are annotated with `helm.sh/resource-policy: keep` and will not be deleted on uninstall. Delete them manually if needed:

```bash
kubectl delete secret docshare-secret -n docshare
```

PersistentVolumeClaims created by PostgreSQL are also retained. Delete them manually:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=docshare -n docshare
```