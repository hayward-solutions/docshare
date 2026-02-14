# Google OAuth2 SSO

This example configures DocShare with Google OAuth2 for authentication.

## Prerequisites

- A Google Cloud project with OAuth 2.0 credentials
- Docker and Docker Compose installed

## Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project
3. Navigate to **APIs & Services > Credentials**
4. Create **OAuth 2.0 Client ID** (Web application)
5. Add authorized redirect URI:
   ```
   http://localhost:8080/api/auth/sso/oauth/google/callback
   ```
6. Add authorized JavaScript origins:
   ```
   http://localhost:8080
   ```
7. Copy the Client ID and Client Secret

## Usage

```bash
# Set environment variables
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"

# Start DocShare with Google SSO
docker-compose up -d
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth2 Client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth2 Client Secret |

## Testing

1. Access the DocShare login page at http://localhost:3001
2. Click "Sign in with Google"
3. Complete the Google authentication flow
4. You'll be redirected back to DocShare with an authenticated session
