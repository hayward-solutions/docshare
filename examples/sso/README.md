# SSO Examples

This directory contains example configurations for deploying DocShare with various SSO providers.

## Available Examples

| Example | Description |
|---------|-------------|
| [google](google/) | Google OAuth2 authentication |
| [github](github/) | GitHub OAuth2 authentication |
| [keycloak](keycloak/) | Keycloak OIDC authentication |
| [ldap](ldap/) | OpenLDAP / Active Directory authentication |

## Quick Start

Choose an SSO provider and follow its README:

```bash
# Google OAuth2
cd google
docker-compose up -d

# GitHub OAuth2
cd github
docker-compose up -d

# Keycloak OIDC
cd keycloak
docker-compose up -d

# LDAP
cd ldap
docker-compose up -d
```

## Combining Multiple Providers

You can enable multiple SSO providers by combining environment variables:

```bash
# Enable both Google and GitHub
export GOOGLE_CLIENT_ID=xxx
export GOOGLE_CLIENT_SECRET=xxx
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
docker-compose -f ../docker-compose.yml -f google/docker-compose.yml -f github/docker-compose.yml up -d
```

## Documentation

For detailed setup instructions, configuration options, and API documentation, see [../../docs/SSO.md](../../docs/SSO.md).
