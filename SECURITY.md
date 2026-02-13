# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| Latest  | ✅                 |
| 1.x     | ✅                 |
| < 1.0   | ❌                 |

## Reporting a Vulnerability

We take the security of DocShare seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

Please [open an issue](https://github.com/docshare/docshare/issues/new) on GitHub to report security vulnerabilities.

When reporting a vulnerability, please include:

- **Description**: Clear description of the vulnerability
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Impact**: Potential impact of the vulnerability
- **Environment**: Version and environment details
- **Proof of Concept**: If available, a minimal proof of concept

### Response Timeline

- **Initial Response**: We will acknowledge receipt within 48 hours
- **Detailed Response**: We will provide a detailed response within 7 days
- **Patch Timeline**: We aim to release a patch within 30 days of disclosure

### Security Coordinators

Report security issues via [GitHub Issues](https://github.com/docshare/docshare/issues/new).

## Security Best Practices

### For Deployments

1. **Change Default Credentials**
   - Update database credentials in production
   - Change MinIO access keys
   - Use a strong JWT secret (minimum 32 characters)

2. **Network Security**
   - Use HTTPS in production
   - Configure firewall rules
   - Limit database access to application servers only

3. **Environment Variables**
   - Never commit secrets to version control
   - Use environment-specific configurations
   - Regularly rotate secrets

4. **Container Security**
   - Use official Docker images
   - Regularly update base images
   - Scan images for vulnerabilities

### For Development

1. **Local Development**
   - Use different credentials than production
   - Keep development and production data separate
   - Use HTTPS locally when possible

2. **Code Security**
   - Review code for security issues
   - Use security scanning tools
   - Follow secure coding practices

## Security Features

DocShare includes several security features:

- **Authentication**: JWT-based authentication with configurable expiration
- **Authorization**: Role-based access control (RBAC)
- **Password Security**: bcrypt hashing for password storage
- **File Security**: Presigned URLs for secure file access
- **Input Validation**: Server-side validation for all inputs
- **CORS Protection**: Configurable CORS settings
- **File Upload Limits**: Configurable file size restrictions
- **Audit Logging**: Comprehensive audit trail tracking all user actions (uploads, downloads, shares, logins, admin operations) with IP address and request correlation, automatically exported to S3/MinIO
- **API Tokens**: SHA-256 hashed at rest, raw token shown once, prefix stored for display
- **Device Flow**: Codes SHA-256 hashed, 15-minute expiry, single-use (hard deleted after token issuance)

## Known Security Considerations

### Current Limitations

1. **File Type Validation**: Currently relies on MIME type detection
2. **Rate Limiting**: Not implemented in current version

### Future Improvements

- Rate limiting for API endpoints
- File content scanning for malware
- Multi-factor authentication support

## Security Updates

We will announce security updates through:

- GitHub Security Advisories
- Release notes
- Project documentation

Users are encouraged to:

- Monitor for security updates
- Update to patched versions promptly
- Review security advisories for impact assessment

## Responsible Disclosure Policy

We follow a responsible disclosure policy:

1. **Private Reporting**: Security issues should be reported privately
2. **Coordination**: We will coordinate with reporters on disclosure timing
3. **Credit**: We will credit reporters who follow this policy
4. **Legal Protection**: We will not pursue legal action against researchers who follow this policy

## Security Resources

### Tools and Libraries

- **Go Security**: https://gosec.github.io/
- **Node.js Security**: https://nodejs.org/en/security
- **OWASP**: https://owasp.org/
- **Docker Security**: https://docs.docker.com/security/

### Further Reading

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Security Checklist](https://github.com/Checkmarx/Go-SCP)
- [Node.js Security Best Practices](https://expressjs.com/en/advanced/security-best-practices.html)

## Questions

If you have questions about this security policy or need to report a security issue, please [open an issue](https://github.com/docshare/docshare/issues/new) on GitHub.

Thank you for helping keep DocShare and its users safe! 🛡️