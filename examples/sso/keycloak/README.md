# Keycloak OIDC SSO

This example configures DocShare with Keycloak for enterprise SSO using OpenID Connect (OIDC).

## Prerequisites

- Docker and Docker Compose installed
- (Optional) Existing Keycloak deployment

## Setup

1. Start the services:
   ```bash
   docker-compose up -d
   ```

2. Access Keycloak admin console at http://localhost:8180
   - Username: `admin`
   - Password: `admin`

3. Create a new Realm named `docshare`

4. Create an OIDC Client:
   - Client ID: `docshare`
   - Client Protocol: `openid-connect`
   - Access Type: `confidential`
   - Valid Redirect URIs: `http://localhost:8080/*`
   - Web Origins: `http://localhost:8080`

5. Get the Client Secret from the **Credentials** tab

6. Note the Issuer URL: `http://localhost:8180/realms/docshare`

## Usage

The docker-compose file automatically configures the backend with the Keycloak OIDC settings. No additional environment variables needed.

## Services

| Service | URL | Description |
|---------|-----|-------------|
| Keycloak | http://localhost:8180 | Identity Provider |
| DocShare Backend | http://localhost:8080 | API Server |
| DocShare Frontend | http://localhost:3001 | Web Application |

## Testing

1. Access the DocShare login page at http://localhost:3001
2. Click "Sign in with OpenID Connect" (or configure frontend to show Keycloak option)
3. Complete the Keycloak authentication flow
4. You'll be redirected back to DocShare with an authenticated session

## Production Notes

For production deployment:
- Enable HTTPS for Keycloak
- Use a proper domain name
- Update redirect URIs to match your production URL
- Secure the admin credentials
