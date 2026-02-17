# Deployment Examples

Ready-to-use configurations for various deployment scenarios.

## Docker Compose

Docker Compose examples for different deployment scenarios:

| Example | Description | Use Case |
|---------|-------------|----------|
| [minimal](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/minimal) | Basic deployment with AWS S3 | Quick evaluation, simple setups |
| [external-db](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/external-db) | External PostgreSQL database | Production with managed database |
| [s3-compatible](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/s3-compatible) | S3-compatible storage (MinIO, Wasabi, etc.) | Self-hosted or alternative storage |
| [full](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/full) | Full stack with SSO | Enterprise deployments |

See [Docker Compose Examples](docker-compose.md) for detailed configuration.

## Helm Charts

Kubernetes deployment examples using Helm:

| Example | Description | Use Case |
|---------|-------------|----------|
| [minimal.yaml](https://github.com/hayward-solutions/docshare/blob/main/examples/helm/minimal.yaml) | Basic deployment with bundled PostgreSQL | Development, evaluation |
| [production.yaml](https://github.com/hayward-solutions/docshare/blob/main/examples/helm/production.yaml) | Production-ready with TLS and replicas | Production deployments |
| [external-db.yaml](https://github.com/hayward-solutions/docshare/blob/main/examples/helm/external-db.yaml) | External PostgreSQL database | Managed database services |
| [ha.yaml](https://github.com/hayward-solutions/docshare/blob/main/examples/helm/ha.yaml) | High availability with multiple replicas | Large-scale deployments |
| [ingress/external.yaml](https://github.com/hayward-solutions/docshare/blob/main/examples/helm/ingress/external.yaml) | External ingress controller | AWS ALB, Cloudflare, GCP CLB, etc. |

See [Helm Examples](helm.md) for detailed configuration.

## Single Sign-On (SSO)

SSO provider configurations:

| Example | Description |
|---------|-------------|
| [Google](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/sso/google) | Google OAuth2 authentication |
| [GitHub](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/sso/github) | GitHub OAuth2 authentication |
| [Keycloak](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/sso/keycloak) | Keycloak OIDC authentication |
| [LDAP](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/sso/ldap) | OpenLDAP / Active Directory authentication |

See [SSO Examples](sso.md) for detailed configuration.

## Ingress / Reverse Proxy

Reverse proxy configurations for exposing DocShare:

| Example | Description | TLS | Best For |
|---------|-------------|-----|----------|
| [Caddy](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/ingress/caddy) | Automatic HTTPS with Let's Encrypt | Automatic | Production, ease of use |
| [Nginx](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/ingress/nginx) | Traditional reverse proxy | Self-signed (demo) | Custom configurations, familiarity |
| [Traefik](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/ingress/traefik) | Modern proxy with auto-discovery | Let's Encrypt (optional) | Microservices, dynamic config |
| [Tailscale](https://github.com/hayward-solutions/docshare/tree/main/examples/docker-compose/ingress/tailscale) | Private access via tailnet | MagicDNS (built-in) | Internal tools, zero-trust access |

See [Ingress Examples](ingress.md) for detailed configuration.

## Choosing a Deployment Method

| Need | Recommended |
|------|-------------|
| Quick evaluation | Docker Compose minimal |
| Production with TLS | Docker Compose + Caddy or Helm with cert-manager |
| Kubernetes | Helm chart |
| Private access only | Tailscale ingress |
| Enterprise SSO | Keycloak or LDAP configuration |
| High availability | Helm HA configuration or external database |