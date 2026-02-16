# Tailscale Sidecar Example

Expose DocShare privately via Tailscale. This setup uses the `network_mode: service:` pattern to share the Tailscale container's network with all services.

## Features

- Private access only via Tailscale tailnet
- No public internet exposure
- Automatic HTTPS via MagicDNS
- Zero-trust networking

## Prerequisites

1. **Tailscale Account**: Sign up at https://tailscale.com
2. **Auth Key**: Generate at https://login.tailscale.com/admin/settings/keys
3. **HTTPS Enabled**: Enable at https://login.tailscale.com/admin/dns (required for MagicDNS)

## Quick Start

```bash
# Set required environment variables
export TS_AUTHKEY=tskey-auth-xxxxx
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
export S3_BUCKET=your-bucket

# Start services
docker-compose up -d
```

Access at `https://docshare.tailXXXXX.ts.net` (the exact URL appears in Tailscale admin console).

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TS_AUTHKEY` | Yes | Tailscale auth key (generate in admin console) |
| `AWS_ACCESS_KEY_ID` | Yes | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Yes | AWS secret key |
| `S3_BUCKET` | Yes | S3 bucket name |
| `AWS_REGION` | No | AWS region (default: `us-east-1`) |
| `JWT_SECRET` | No | JWT secret |

## Architecture

```
┌─────────────────────────────────────────┐
│           Tailscale Container           │
│  (hostname: docshare)                   │
│  - Handles tailnet connection           │
│  - MagicDNS: docshare.tailxxx.ts.net    │
│  - Automatic HTTPS                      │
└───────────────────┬─────────────────────┘
                    │ (network_mode: service:tailscale)
    ┌───────────────┼───────────────┐
    │               │               │
    ▼               ▼               ▼
┌────────┐   ┌─────────┐   ┌──────────┐
│Frontend│   │ Backend │   │ Postgres │
│ :3000  │   │  :8080  │   │  :5432   │
└────────┘   └─────────┘   └──────────┘
```

All services share the Tailscale container's network, so:
- Services communicate via `127.0.0.1`
- External access only via Tailscale
- No ports exposed to host

## serve.json Configuration

The `serve.json` file configures Tailscale Serve:

```json
{
  "Web": {
    "${TS_CERT_DOMAIN}:443": {
      "Handlers": {
        "/api": { "Proxy": "http://127.0.0.1:8080" },
        "/": { "Proxy": "http://127.0.0.1:3000" }
      }
    }
  }
}
```

The `${TS_CERT_DOMAIN}` variable is automatically replaced with the MagicDNS domain.

## Access Control

Configure who can access DocShare via Tailscale ACLs:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["group:team"],
      "dst": ["tag:docshare:*"]
    }
  ],
  "tagOwners": {
    "tag:docshare": ["autogroup:admin"]
  }
}
```

## Production Notes

1. **Auth Key**: Use a reusable auth key for production
2. **ACL Tags**: Use tags to control access in Tailscale admin
3. **Hostname**: Change `hostname: docshare` to your preferred name
4. **Tailnet Lock**: Enable for additional security

## Troubleshooting

### Container won't start

Check Tailscale logs:
```bash
docker-compose logs tailscale
```

Common issues:
- Invalid auth key
- Auth key already used (use reusable key)
- `/dev/net/tun` not available (check Docker permissions)

### Can't access from tailnet

1. Verify device is connected to tailnet
2. Check Tailscale admin console for the device
3. Verify HTTPS is enabled for the tailnet

### DNS not resolving

Enable MagicDNS in Tailscale admin:
https://login.tailscale.com/admin/dns