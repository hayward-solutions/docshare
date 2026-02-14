# GitHub OAuth2 SSO

This example configures DocShare with GitHub OAuth2 for authentication.

## Prerequisites

- A GitHub OAuth App
- Docker and Docker Compose installed

## Setup

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Create a new **OAuth App**
3. Configure:
   - Homepage URL: `http://localhost:3001`
   - Authorization callback URL:
     ```
     http://localhost:8080/api/auth/sso/oauth/github/callback
     ```
4. Copy the Client ID and generate a Client Secret

## Usage

```bash
# Set environment variables
export GITHUB_CLIENT_ID="your-client-id"
export GITHUB_CLIENT_SECRET="your-client-secret"

# Start DocShare with GitHub SSO
docker-compose up -d
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_CLIENT_ID` | GitHub OAuth App Client ID |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Client Secret |

## Testing

1. Access the DocShare login page at http://localhost:3001
2. Click "Sign in with GitHub"
3. Complete the GitHub authentication flow
4. You'll be redirected back to DocShare with an authenticated session
