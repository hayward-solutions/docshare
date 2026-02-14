# SSO Authentication Setup Guide

DocShare supports multiple SSO providers including OAuth2/OIDC (Google, GitHub, Keycloak, Authentik), SAML (Okta, Azure AD), and LDAP/Active Directory.

## Table of Contents

1. [Quick Start](#quick-start)
2. [OAuth2/OIDC Providers](#oauth2oidc-providers)
   - [Google](#google-oauth2)
   - [GitHub](#github-oauth2)
   - [OIDC (Keycloak, Authentik)](#oidc-keycloak-authentik)
3. [SAML](#saml)
4. [LDAP/Active Directory](#ldapactive-directory)
5. [Environment Variables](#environment-variables)
6. [API Endpoints](#api-endpoints)

---

## Quick Start

### Docker Compose

Enable SSO by adding an override file:

```bash
# Google OAuth2
docker-compose -f docker-compose.yml -f docker-compose.sso-google.yml up -d

# GitHub OAuth2  
docker-compose -f docker-compose.yml -f docker-compose.sso-github.yml up -d

# Keycloak OIDC
docker-compose -f docker-compose.yml -f docker-compose.sso-keycloak.yml up -d

# LDAP
docker-compose -f docker-compose.yml -f docker-compose.sso-ldap.yml up -d
```

### Helm

```bash
helm install docshare oci://ghcr.io/hayward-solutions/charts/docshare \
  --namespace docshare \
  -f values-sso.yaml
```

---

## OAuth2/OIDC Providers

### Google OAuth2

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
7. Copy Client ID and Client Secret

**Docker Compose:**
```bash
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
docker-compose -f docker-compose.yml -f docker-compose.sso-google.yml up -d
```

**Environment Variables:**
```bash
OAUTH_GOOGLE_ENABLED=true
OAUTH_GOOGLE_CLIENT_ID=your-client-id
OAUTH_GOOGLE_CLIENT_SECRET=your-client-secret
OAUTH_GOOGLE_REDIRECT_URL=http://localhost:8080/api/auth/sso/oauth/google/callback
OAUTH_GOOGLE_SCOPES=openid,email,profile
```

### GitHub OAuth2

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Create a new **OAuth App**
3. Homepage URL: `http://localhost:3001`
4. Authorization callback URL:
   ```
   http://localhost:8080/api/auth/sso/oauth/github/callback
   ```
5. Copy Client ID and generate Client Secret

**Docker Compose:**
```bash
export GITHUB_CLIENT_ID="your-client-id"
export GITHUB_CLIENT_SECRET="your-client-secret"
docker-compose -f docker-compose.yml -f docker-compose.sso-github.yml up -d
```

**Environment Variables:**
```bash
OAUTH_GITHUB_ENABLED=true
OAUTH_GITHUB_CLIENT_ID=your-client-id
OAUTH_GITHUB_CLIENT_SECRET=your-client-secret
OAUTH_GITHUB_REDIRECT_URL=http://localhost:8080/api/auth/sso/oauth/github/callback
OAUTH_GITHUB_SCOPES=read:user,user:email
```

### OIDC (Keycloak, Authentik)

#### Keycloak Setup

1. Start Keycloak:
   ```bash
   docker-compose -f docker-compose.yml -f docker-compose.sso-keycloak.yml up -d
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

**Environment Variables:**
```bash
OAUTH_OIDC_ENABLED=true
OAUTH_OIDC_CLIENT_ID=docshare
OAUTH_OIDC_CLIENT_SECRET=your-client-secret
OAUTH_OIDC_ISSUER_URL=http://localhost:8180/realms/docshare
OAUTH_OIDC_REDIRECT_URL=http://localhost:8080/api/auth/sso/oauth/oidc/callback
OAUTH_OIDC_SCOPES=openid,profile,email
```

#### Authentik Setup

1. Deploy Authentik (see [Authentik documentation](https://docs.goauthentik.io))

2. Create an OAuth2/OpenID Provider:
   - Name: DocShare
   - Client ID: Generate or specify
   - Client Secret: Generate
   - Authorization flow: Default
   - Redirect URIs: `http://localhost:8080/api/auth/sso/oauth/oidc/callback`

3. Create an Application pointing to the Provider

4. Note the Issuer URL (usually `https://your-authentik.com/application/o`)

---

## SAML

### Okta SAML Setup

1. Create a new SAML Application in Okta Admin Console

2. Configure SAML Settings:
   - Single sign-on URL: `http://localhost:8080/api/auth/sso/saml/acs`
   - Audience URI (SP Entity ID): `http://localhost:8080`
   - Name ID format: `EmailAddress`
   - Attribute Statements:
     - email → user.email
     - firstName → user.firstName
     - lastName → user.lastName

3. Copy the IdP Metadata URL

**Environment Variables:**
```bash
SAML_ENABLED=true
SAML_IDP_METADATA_URL=https://your-org.okta.com/app/abc123/sso/saml
SAML_SP_ENTITY_ID=http://localhost:8080
SAML_SP_ACS_URL=http://localhost:8080/api/auth/sso/saml/acs
```

### Azure AD SAML Setup

1. Register DocShare as an Enterprise Application in Azure AD

2. Configure Single Sign-On with SAML:
   - Identifier (Entity ID): `http://localhost:8080`
   - Reply URL: `http://localhost:8080/api/auth/sso/saml/acs`
   - User attributes:
     - user.email
     - user.givenName
     - user.surname

3. Download Federation Metadata XML

**Environment Variables:**
```bash
SAML_ENABLED=true
SAML_IDP_METADATA_URL=https://login.microsoftonline.com/{tenant}/federationmetadata/2007-06/federationmetadata.xml
SAML_SP_ENTITY_ID=http://localhost:8080
SAML_SP_ACS_URL=http://localhost:8080/api/auth/sso/saml/acs
```

---

## LDAP/Active Directory

### OpenLDAP

Start with the LDAP compose file:

```bash
docker-compose -f docker-compose.yml -f docker-compose.sso-ldap.yml up -d
```

Access phpLDAPadmin at http://localhost:8081

**Environment Variables:**
```bash
LDAP_ENABLED=true
LDAP_URL=ldap://ldap:389
LDAP_BIND_DN=cn=admin,dc=docshare,dc=local
LDAP_BIND_PASSWORD=admin
LDAP_SEARCH_BASE=dc=docshare,dc=local
LDAP_USER_FILTER=(uid=%s)
LDAP_EMAIL_FIELD=mail
LDAP_NAME_FIELDS=cn
```

### Active Directory

```bash
LDAP_ENABLED=true
LDAP_URL=ldap://your-ad-server:389
LDAP_BIND_DN=cn=docshare,cn=Users,dc=example,dc=com
LDAP_BIND_PASSWORD=your-password
LDAP_SEARCH_BASE=dc=example,dc=com
LDAP_USER_FILTER=(sAMAccountName=%s)
LDAP_EMAIL_FIELD=userPrincipalName
LDAP_NAME_FIELDS=givenName,sn
```

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| **General** | | |
| `SSO_AUTO_REGISTER` | Auto-create user on first SSO login | `true` |
| `SSO_DEFAULT_ROLE` | Default role for new SSO users | `user` |
| **Google** | | |
| `OAUTH_GOOGLE_ENABLED` | Enable Google OAuth | `true` |
| `OAUTH_GOOGLE_CLIENT_ID` | Google Client ID | `xxx.apps.googleusercontent.com` |
| `OAUTH_GOOGLE_CLIENT_SECRET` | Google Client Secret | `xxx` |
| `OAUTH_GOOGLE_REDIRECT_URL` | Callback URL | `http://localhost:8080/api/auth/sso/oauth/google/callback` |
| **GitHub** | | |
| `OAUTH_GITHUB_ENABLED` | Enable GitHub OAuth | `true` |
| `OAUTH_GITHUB_CLIENT_ID` | GitHub Client ID | `xxx` |
| `OAUTH_GITHUB_CLIENT_SECRET` | GitHub Client Secret | `xxx` |
| **OIDC** | | |
| `OAUTH_OIDC_ENABLED` | Enable OIDC | `true` |
| `OAUTH_OIDC_CLIENT_ID` | OIDC Client ID | `docshare` |
| `OAUTH_OIDC_CLIENT_SECRET` | OIDC Client Secret | `xxx` |
| `OAUTH_OIDC_ISSUER_URL` | OIDC Issuer URL | `https://keycloak.example.com/realms/docshare` |
| **SAML** | | |
| `SAML_ENABLED` | Enable SAML | `true` |
| `SAML_IDP_METADATA_URL` | IdP Metadata URL | `https://idp.example.com/metadata` |
| `SAML_SP_ENTITY_ID` | SP Entity ID | `http://localhost:8080` |
| `SAML_SP_ACS_URL` | SP ACS URL | `http://localhost:8080/api/auth/sso/saml/acs` |
| **LDAP** | | |
| `LDAP_ENABLED` | Enable LDAP | `true` |
| `LDAP_URL` | LDAP Server URL | `ldap://localhost:389` |
| `LDAP_BIND_DN` | Bind DN | `cn=admin,dc=example,dc=com` |
| `LDAP_BIND_PASSWORD` | Bind Password | `xxx` |
| `LDAP_SEARCH_BASE` | Search Base | `dc=example,dc=com` |
| `LDAP_USER_FILTER` | User Search Filter | `(uid=%s)` or `(sAMAccountName=%s)` |
| `LDAP_EMAIL_FIELD` | Email Attribute | `mail` or `userPrincipalName` |
| `LDAP_NAME_FIELDS` | Name Attributes | `givenName,sn` |

---

## API Endpoints

### List Available Providers
```
GET /api/auth/sso/providers
```

Response:
```json
[
  {"name": "google", "displayName": "Google", "type": "oauth"},
  {"name": "github", "displayName": "GitHub", "type": "oauth"},
  {"name": "oidc", "displayName": "OpenID Connect", "type": "oidc"},
  {"name": "saml", "displayName": "Enterprise SSO (SAML)", "type": "saml"},
  {"name": "ldap", "displayName": "Corporate Directory (LDAP)", "type": "ldap"}
]
```

### OAuth2/OIDC Login
```
GET /api/auth/sso/oauth/{provider}
```

Redirects to provider authorization page.

### OAuth2/OIDC Callback
```
GET /api/auth/sso/oauth/{provider}/callback?code=xxx&state=xxx
```

Returns JWT token and user:
```json
{
  "success": true,
  "data": {
    "token": "eyJ...",
    "user": {...}
  }
}
```

### SAML Metadata
```
GET /api/auth/sso/saml/metadata
```

Returns SP metadata XML for IdP configuration.

### SAML ACS
```
POST /api/auth/sso/saml/acs
```

### LDAP Login
```
POST /api/auth/sso/ldap/login
```

Request:
```json
{
  "username": "john",
  "password": "xxx"
}
```

Response:
```json
{
  "success": true,
  "data": {
    "token": "eyJ...",
    "user": {...}
  }
}
```

### Linked Accounts
```
GET /api/auth/linked-accounts
DELETE /api/auth/linked-accounts/:id
POST /api/auth/linked-accounts/link
```

---

## Security Considerations

1. **Use HTTPS in production** - Never use HTTP for SSO
2. **Rotate secrets regularly** - Update client secrets periodically
3. **Validate redirect URLs** - Only allow pre-registered redirect URLs
4. **Enable audit logging** - Monitor SSO authentication events
5. **Configure session timeout** - Balance security with user experience
