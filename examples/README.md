# Examples

This directory contains example configurations for deploying DocShare in various scenarios.

## Directory Structure

```
examples/
├── docker-compose/     # Docker Compose deployment examples
│   ├── minimal/        # Basic deployment with AWS S3
│   ├── external-db/    # External PostgreSQL
│   ├── s3-compatible/  # MinIO or other S3-compatible storage
│   └── full/           # Full stack with SSO
├── helm/               # Kubernetes Helm values examples
│   ├── minimal.yaml    # Development/evaluation
│   ├── production.yaml # Production-ready
│   ├── external-db.yaml # External database
│   └── ha.yaml         # High availability
└── sso/                # SSO provider examples
    ├── google/         # Google OAuth2
    ├── github/         # GitHub OAuth2
    ├── keycloak/       # Keycloak OIDC
    └── ldap/           # LDAP/Active Directory
```

## Quick Links

| Scenario | Docker Compose | Helm |
|----------|---------------|------|
| Minimal | [docker-compose/minimal](docker-compose/minimal/) | [helm/minimal.yaml](helm/minimal.yaml) |
| Production | - | [helm/production.yaml](helm/production.yaml) |
| External DB | [docker-compose/external-db](docker-compose/external-db/) | [helm/external-db.yaml](helm/external-db.yaml) |
| S3-Compatible | [docker-compose/s3-compatible](docker-compose/s3-compatible/) | - |
| High Availability | - | [helm/ha.yaml](helm/ha.yaml) |
| SSO | [docker-compose/full](docker-compose/full/) | [sso/](sso/) |

## Documentation

- [Deployment Guide](../docs/DEPLOYMENT.md)
- [Helm Chart Reference](../docs/HELM.md)
- [SSO Configuration](../docs/SSO.md)