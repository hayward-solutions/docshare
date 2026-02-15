# Helm Deployment Examples

This directory contains example values files for deploying DocShare on Kubernetes with Helm.

## Available Examples

| Example | Description | Use Case |
|---------|-------------|----------|
| [minimal.yaml](minimal.yaml) | Basic deployment with bundled PostgreSQL | Development, evaluation |
| [production.yaml](production.yaml) | Production-ready with TLS and replicas | Production deployments |
| [external-db.yaml](external-db.yaml) | External PostgreSQL database | Managed database services |
| [ha.yaml](ha.yaml) | High availability with multiple replicas | Large-scale deployments |

## Quick Reference

### Minimal

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/minimal.yaml
```

### Production

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/production.yaml
```

### With External Database

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare --create-namespace \
  -f examples/helm/external-db.yaml
```

## Prerequisites

- Kubernetes 1.26+
- Helm 3.12+
- AWS S3 bucket (or S3-compatible storage)
- (Optional) cert-manager for TLS

## AWS S3 Configuration

### Option 1: IAM Roles (EKS - Recommended)

```yaml
backend:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/docshare-s3-role

s3:
  region: us-east-1
  bucket: docshare-prod
```

### Option 2: Static Credentials

```yaml
s3:
  region: us-east-1
  accessKey: your-access-key
  secretKey: your-secret-key
  bucket: docshare-prod
```

### Option 3: Existing Secret

```yaml
s3:
  region: us-east-1
  bucket: docshare-prod
  existingSecret: my-s3-secret
```

For detailed configuration, see the [Helm documentation](../../docs/HELM.md).