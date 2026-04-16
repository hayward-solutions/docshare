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
  --set api.env.jwtSecret=my-secret-change-me \
  --set s3.bucket=your-s3-bucket-name
```

Access the application:

```bash
# Port-forward the web
kubectl port-forward svc/docshare-web 3001:3000 -n docshare

# Port-forward the API
kubectl port-forward svc/docshare-api 8080:8080 -n docshare
```

Then open http://localhost:3001 and create your first account.

## Configuration

### Minimal Production Example

```yaml
# production-values.yaml
api:
  replicaCount: 2
  env:
    jwtSecret: "generate-with-openssl-rand-hex-32"

web:
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
          service: web
        - path: /api
          pathType: Prefix
          service: api
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
api:
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

### Single Sign-On (SSO)

The chart can configure OIDC, Google OAuth, GitHub OAuth, SAML, and LDAP
via the `sso.*` values. For OIDC, only `issuerUrl` is required — endpoints are
auto-discovered from `{issuerUrl}/.well-known/openid-configuration` and id_tokens
are verified against the IdP's JWKS.

```yaml
sso:
  oidc:
    enabled: true
    clientId: docshare
    clientSecret: ""          # prefer --set or sso.existingSecret
    issuerUrl: https://keycloak.example.com/realms/docshare
    scopes: "openid,profile,email"
```

Register this callback URL with your IdP (the chart derives it from
`globals.apiUrl` or the ingress host):

```
{API_URL}/auth/sso/oauth/oidc/callback
```

See [docs/SSO.md](SSO.md) and [examples/helm/sso.yaml](../examples/helm/sso.yaml)
for full provider coverage.

## Values Reference

### Backend

| Key                              | Type   | Default                                      | Description                                          |
|----------------------------------|--------|----------------------------------------------|------------------------------------------------------|
| `api.replicaCount`           | int    | `1`                                          | Number of api replicas                           |
| `api.image.repository`       | string | `ghcr.io/hayward-solutions/docshare/api` | Backend image repository                             |
| `api.image.tag`              | string | `latest`                                     | Backend image tag                                    |
| `api.image.pullPolicy`       | string | `IfNotPresent`                               | Image pull policy                                    |
| `api.service.type`           | string | `ClusterIP`                                  | Service type                                         |
| `api.service.port`           | int    | `8080`                                       | Service port                                         |
| `api.resources`              | object | `{}`                                         | CPU/memory resource requests/limits                  |
| `api.env.dbSslmode`          | string | `disable`                                    | PostgreSQL SSL mode                                  |
| `api.env.s3Bucket`           | string | `docshare`                                   | S3 bucket name                                       |
| `api.env.s3UseSsl`           | string | `"true"`                                     | Use SSL for S3 connection                            |
| `api.env.jwtSecret`          | string | `""`                                         | JWT signing secret (auto-generated if empty)         |
| `api.env.jwtExpirationHours` | string | `"24"`                                       | JWT token lifetime                                   |
| `api.env.serverPort`         | string | `"8080"`                                     | Backend server port                                  |
| `api.env.gotenbergUrl`       | string | `""`                                         | Gotenberg URL (auto-configured if gotenberg.enabled) |
| `api.env.webUrl`         | string | `""`                                         | Frontend URL (deprecated: use `globals.webUrl`)                 |
| `api.env.apiUrl`          | string | `""`                                         | Backend URL (deprecated: use `globals.apiUrl`)                  |
| `api.existingSecret`         | string | `""`                                         | Use existing secret for sensitive values             |

### Global

| Key                             | Type   | Default                                       | Description                                                     |
|---------------------------------|--------|-----------------------------------------------|-----------------------------------------------------------------|
| `globals.webUrl`           | string | `""`                                         | Public frontend URL (e.g., https://docshare.example.com). Auto-configured from ingress if enabled. |
| `globals.apiUrl`           | string | `""`                                         | Public API URL (e.g., https://docshare.example.com/api). Auto-configured from ingress if enabled. |

**Note:** Use `globals.webUrl` and `globals.apiUrl` when using an external ingress. These values are automatically passed to both the API and web containers. See [external ingress example](../../examples/helm/ingress/external.yaml) for a complete configuration.

### Frontend

| Key                             | Type   | Default                                       | Description                                                     |
|---------------------------------|--------|-----------------------------------------------|-----------------------------------------------------------------|
| `web.replicaCount`         | int    | `1`                                           | Number of web replicas                                     |
| `web.image.repository`     | string | `ghcr.io/hayward-solutions/docshare/web` | Frontend image repository                                       |
| `web.image.tag`            | string | `latest`                                      | Frontend image tag                                              |
| `web.image.pullPolicy`     | string | `IfNotPresent`                                | Image pull policy                                               |
| `web.service.type`         | string | `ClusterIP`                                   | Service type                                                    |
| `web.service.port`         | int    | `3000`                                        | Service port                                                    |
| `web.resources`            | object | `{}`                                          | CPU/memory resource requests/limits                             |

**Note:** The web receives `API_URL` and `WEB_URL` from the Helm chart (auto-derived from ingress config, `globals`, or `api.env` values), which are passed as environment variables at runtime.

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

### SSO

All providers are disabled by default. Redirect URLs default to
`{API_URL}/auth/sso/oauth/{provider}/callback`.

| Key                                    | Type    | Default                  | Description                                                           |
|----------------------------------------|---------|--------------------------|-----------------------------------------------------------------------|
| `sso.autoRegister`                     | bool    | `true`                   | Auto-create users on first SSO login                                  |
| `sso.defaultRole`                      | string  | `"user"`                 | Role assigned to auto-created users                                   |
| `sso.existingSecret`                   | string  | `""`                     | Existing Secret with SSO credentials (overrides chart-managed Secret) |
| `sso.google.enabled`                   | bool    | `false`                  | Enable Google OAuth                                                   |
| `sso.google.clientId`                  | string  | `""`                     | Google OAuth client ID                                                |
| `sso.google.clientSecret`              | string  | `""`                     | Google OAuth client secret                                            |
| `sso.google.redirectUrl`               | string  | `""`                     | Override redirect URL (defaults to derived)                           |
| `sso.google.scopes`                    | string  | `"openid,email,profile"` | Requested scopes                                                      |
| `sso.github.enabled`                   | bool    | `false`                  | Enable GitHub OAuth                                                   |
| `sso.github.clientId`                  | string  | `""`                     | GitHub OAuth client ID                                                |
| `sso.github.clientSecret`              | string  | `""`                     | GitHub OAuth client secret                                            |
| `sso.github.redirectUrl`               | string  | `""`                     | Override redirect URL                                                 |
| `sso.github.scopes`                    | string  | `"read:user,user:email"` | Requested scopes                                                      |
| `sso.oidc.enabled`                     | bool    | `false`                  | Enable generic OIDC                                                   |
| `sso.oidc.clientId`                    | string  | `""`                     | OIDC client ID                                                        |
| `sso.oidc.clientSecret`                | string  | `""`                     | OIDC client secret                                                    |
| `sso.oidc.issuerUrl`                   | string  | `""`                     | **Required when enabled.** IdP issuer URL — endpoints auto-discovered |
| `sso.oidc.redirectUrl`                 | string  | `""`                     | Override redirect URL                                                 |
| `sso.oidc.scopes`                      | string  | `"openid,profile,email"` | Requested scopes                                                      |
| `sso.oidc.skipIssuerVerification`      | bool    | `false`                  | Skip `iss` claim verification (only for non-compliant IdPs)           |
| `sso.saml.enabled`                     | bool    | `false`                  | Enable SAML                                                           |
| `sso.saml.idpMetadataUrl`              | string  | `""`                     | IdP metadata XML URL                                                  |
| `sso.saml.spEntityId`                  | string  | `""`                     | SP entity ID (defaults to API URL)                                    |
| `sso.saml.spAcsUrl`                    | string  | `""`                     | SP ACS URL (defaults to derived)                                      |
| `sso.ldap.enabled`                     | bool    | `false`                  | Enable LDAP                                                           |
| `sso.ldap.url`                         | string  | `ldap://localhost:389`   | LDAP server URL                                                       |
| `sso.ldap.bindDn`                      | string  | `""`                     | LDAP bind DN                                                          |
| `sso.ldap.bindPassword`                | string  | `""`                     | LDAP bind password                                                    |
| `sso.ldap.searchBase`                  | string  | `""`                     | LDAP search base                                                      |
| `sso.ldap.userFilter`                  | string  | `"(uid=%s)"`             | User search filter                                                    |
| `sso.ldap.emailField`                  | string  | `"mail"`                 | Email attribute                                                       |
| `sso.ldap.nameFields`                  | string  | `"givenName,sn"`         | Name attributes                                                       |

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