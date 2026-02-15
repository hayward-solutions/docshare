# LDAP Authentication

This example configures DocShare with OpenLDAP for corporate directory authentication.

## Prerequisites

- Docker and Docker Compose installed

## Setup

1. Start the services:
   ```bash
   docker-compose up -d
   ```

2. Access phpLDAPadmin at http://localhost:8081
   - Login DN: `cn=admin,dc=docshare,dc=local`
   - Password: `admin`

3. Create users in the LDAP directory

## Usage

Users can authenticate using their LDAP credentials via the API:

```bash
curl -X POST http://localhost:8080/api/auth/sso/ldap/login \
  -H "Content-Type: application/json" \
  -d '{"username": "john", "password": "password"}'
```

## Services

| Service | URL | Description |
|---------|-----|-------------|
| OpenLDAP | ldap://localhost:389 | LDAP Server |
| phpLDAPadmin | http://localhost:8081 | LDAP Management UI |
| DocShare Backend | http://localhost:8080 | API Server |
| DocShare Frontend | http://localhost:3001 | Web Application |

## Configuration

The following environment variables configure LDAP authentication:

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_URL` | `ldap://ldap:389` | LDAP server URL |
| `LDAP_BIND_DN` | `cn=admin,dc=docshare,dc=local` | Bind DN for search |
| `LDAP_BIND_PASSWORD` | `admin` | Bind password |
| `LDAP_SEARCH_BASE` | `dc=docshare,dc=local` | Base DN for user search |
| `LDAP_USER_FILTER` | `(uid=%s)` | User search filter |
| `LDAP_EMAIL_FIELD` | `mail` | Email attribute |
| `LDAP_NAME_FIELDS` | `cn` | Name attributes (comma-separated) |

## Active Directory

For Microsoft Active Directory, use these settings:

```yaml
environment:
  LDAP_URL: ldap://your-ad-server:389
  LDAP_BIND_DN: cn=docshare,cn=Users,dc=example,dc=com
  LDAP_SEARCH_BASE: dc=example,dc=com
  LDAP_USER_FILTER: (sAMAccountName=%s)
  LDAP_EMAIL_FIELD: userPrincipalName
  LDAP_NAME_FIELDS: givenName,sn
```

## Security Notes

- Change the default LDAP admin passwords in production
- Use LDAPS (LDAP over SSL/TLS) in production
- Restrict access to the LDAP and phpLDAPadmin ports
