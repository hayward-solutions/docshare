# Helm Chart

Deploy DocShare on Kubernetes using the official Helm chart.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.12+

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

Install with default settings (bundled PostgreSQL, MinIO, and Gotenberg):

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  --create-namespace \
  --set backend.env.jwtSecret=my-secret-change-me
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
    nextPublicApiUrl: "https://docshare.example.com"

frontend:
  replicaCount: 2
  env:
    nextPublicApiUrl: "https://docshare.example.com"
    apiUrl: "http://docshare-backend:8080"

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

minio:
  auth:
    rootPassword: "generate-a-strong-password"
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

### Using External S3/MinIO

Disable the bundled MinIO and point to your existing S3-compatible storage:

```yaml
minio:
  enabled: false

externalMinio:
  endpoint: s3.amazonaws.com
  publicEndpoint: s3.amazonaws.com
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
| `backend.env.minioBucket`        | string | `docshare`                                   | MinIO/S3 bucket name                                 |
| `backend.env.minioUseSsl`        | string | `"false"`                                    | Use SSL for MinIO connection                         |
| `backend.env.jwtSecret`          | string | `""`                                         | JWT signing secret (auto-generated if empty)         |
| `backend.env.jwtExpirationHours` | string | `"24"`                                       | JWT token lifetime                                   |
| `backend.env.serverPort`         | string | `"8080"`                                     | Backend server port                                  |
| `backend.env.gotenbergUrl`       | string | `""`                                         | Gotenberg URL (auto-configured if gotenberg.enabled) |
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
| `frontend.env.nextPublicApiUrl` | string | `"http://localhost:8080"`                     | Backend API URL (used by browser)                               |
| `frontend.env.apiUrl`           | string | `""`                                          | Backend API URL (server-side, defaults to internal service URL) |

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

### MinIO (Bundled)

Uses the [Bitnami MinIO chart](https://github.com/bitnami/charts/tree/main/bitnami/minio). See the upstream chart for all available values.

| Key                       | Type   | Default           | Description                  |
|---------------------------|--------|-------------------|------------------------------|
| `minio.enabled`           | bool   | `true`            | Deploy bundled MinIO         |
| `minio.auth.rootUser`     | string | `docshare`        | MinIO root user              |
| `minio.auth.rootPassword` | string | `docshare_secret` | MinIO root password          |
| `minio.defaultBuckets`    | string | `docshare`        | Buckets to create on startup |

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

### External MinIO/S3

| Key                                        | Type   | Default      | Description                        |
|--------------------------------------------|--------|--------------|------------------------------------|
| `externalMinio.endpoint`                   | string | `""`         | S3/MinIO endpoint                  |
| `externalMinio.publicEndpoint`             | string | `""`         | Public endpoint for presigned URLs |
| `externalMinio.accessKey`                  | string | `""`         | Access key                         |
| `externalMinio.secretKey`                  | string | `""`         | Secret key                         |
| `externalMinio.bucket`                     | string | `docshare`   | Bucket name                        |
| `externalMinio.useSsl`                     | string | `"true"`     | Use SSL                            |
| `externalMinio.existingSecret`             | string | `""`         | Existing secret name               |
| `externalMinio.existingSecretAccessKeyKey` | string | `access-key` | Key for access key in secret       |
| `externalMinio.existingSecretSecretKeyKey` | string | `secret-key` | Key for secret key in secret       |

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

PersistentVolumeClaims created by PostgreSQL and MinIO are also retained. Delete them manually:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=docshare -n docshare
```
