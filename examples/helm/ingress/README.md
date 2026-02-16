# Helm Ingress Examples

This directory contains Helm values files for deploying DocShare with different ingress controllers.

## Available Examples

| Example | Ingress Class | TLS | Best For |
|---------|--------------|-----|----------|
| [caddy.yaml](caddy.yaml) | `caddy` | Automatic (Caddy) | Production, automatic HTTPS |
| [nginx.yaml](nginx.yaml) | `nginx` | cert-manager | Standard Kubernetes deployments |
| [traefik.yaml](traefik.yaml) | `traefik` | Traefik TLS | Modern Kubernetes, dynamic config |
| [tailscale.yaml](tailscale.yaml) | `tailscale` | MagicDNS | Private access, zero-trust |

## Quick Reference

### Nginx (with cert-manager)

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/ingress/nginx.yaml
```

### Caddy

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/ingress/caddy.yaml
```

### Traefik

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/ingress/traefik.yaml
```

### Tailscale

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/ingress/tailscale.yaml
```

## Prerequisites

### Nginx
- Nginx Ingress Controller installed
- cert-manager installed (for TLS)

### Caddy
- Caddy Ingress Controller installed

### Traefik
- Traefik installed (often via Helm)

### Tailscale
- Tailscale Kubernetes Operator installed
- HTTPS enabled on your tailnet

## Routing

All examples configure:
- `/` → Frontend service
- `/api` → Backend service