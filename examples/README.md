# Examples

This directory contains example configurations for deploying DocShare in various scenarios.

## Directory Structure

```
examples/
├── docker-compose/      # Docker Compose deployment examples
│   ├── minimal/         # Basic deployment with AWS S3
│   ├── external-db/     # External PostgreSQL
│   ├── s3-compatible/   # MinIO or other S3-compatible storage
│   ├── full/            # Full stack with SSO
│   ├── sso/             # SSO provider examples
│   │   ├── google/      # Google OAuth2
│   │   ├── github/      # GitHub OAuth2
│   │   ├── keycloak/    # Keycloak OIDC
│   │   └── ldap/        # LDAP/Active Directory
│   └── ingress/         # Reverse proxy / ingress examples
│       ├── caddy/       # Caddy (automatic HTTPS)
│       ├── nginx/       # Nginx (flexible config)
│       ├── traefik/     # Traefik (auto-discovery)
│       └── tailscale/   # Tailscale (private access)
├── cloudformation/      # AWS CloudFormation deployment
│   ├── docshare.yaml    # Main CloudFormation template
│   ├── deploy-policy.json # IAM policy for deployment
│   └── README.md        # Deployment guide
└── helm/                # Kubernetes Helm values examples
    ├── minimal.yaml     # Development/evaluation
    ├── production.yaml  # Production-ready
    ├── external-db.yaml # External database
    ├── ha.yaml          # High availability
    └── ingress/         # Ingress controller examples
        ├── caddy.yaml
        ├── nginx.yaml
        ├── traefik.yaml
        └── tailscale.yaml
```

## Quick Links

| Scenario | Docker Compose | Helm | CloudFormation |
|----------|---------------|------|----------------|
| Minimal | [docker-compose/minimal](docker-compose/minimal/) | [helm/minimal.yaml](helm/minimal.yaml) | - |
| Production | - | [helm/production.yaml](helm/production.yaml) | [cloudformation/](cloudformation/) |
| External DB | [docker-compose/external-db](docker-compose/external-db/) | [helm/external-db.yaml](helm/external-db.yaml) | - |
| S3-Compatible | [docker-compose/s3-compatible](docker-compose/s3-compatible/) | - | - |
| High Availability | - | [helm/ha.yaml](helm/ha.yaml) | - |
| AWS ECS Fargate | - | - | [cloudformation/](cloudformation/) |
| SSO | [docker-compose/full](docker-compose/full/) | [sso/](docker-compose/sso/) | - |
| **Ingress** | | | |
| Caddy | [docker-compose/ingress/caddy](docker-compose/ingress/caddy/) | [helm/ingress/caddy.yaml](helm/ingress/caddy.yaml) | - |
| Nginx | [docker-compose/ingress/nginx](docker-compose/ingress/nginx/) | [helm/ingress/nginx.yaml](helm/ingress/nginx.yaml) | - |
| Traefik | [docker-compose/ingress/traefik](docker-compose/ingress/traefik/) | [helm/ingress/traefik.yaml](helm/ingress/traefik.yaml) | - |
| Tailscale | [docker-compose/ingress/tailscale](docker-compose/ingress/tailscale/) | [helm/ingress/tailscale.yaml](helm/ingress/tailscale.yaml) | - |

## Documentation

- [Deployment Guide](../docs/DEPLOYMENT.md)
- [Helm Chart Reference](../docs/HELM.md)
- [AWS CloudFormation Reference](../docs/CLOUDFORMATION.md)
- [SSO Configuration](../docs/SSO.md)