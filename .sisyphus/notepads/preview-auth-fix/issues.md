# Preview Auth Fix - Issues & Technical Debt

## Known Issues

### 1. Hardcoded HMAC Secret
**Location**: `backend/pkg/previewtoken/previewtoken.go` line 145

**Issue**: Preview tokens use a hardcoded secret key instead of the JWT secret from config.

**Impact**: Tokens generated from a different deployment won't validate.

**Todo**: Use `config.JWT.Secret` from config when available. Need to make config accessible to previewtoken package.

### 2. No Automatic Token Cleanup
**Location**: `backend/pkg/previewtoken/previewtoken.go`

**Issue**: Used tokens are stored in memory but never cleaned up, leading to potential memory leaks over time.

**Todo**: Add a periodic cleanup goroutine in `cmd/server/main.go`:
```go
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        previewtoken.Cleanup()
    }
}()
```

### 3. download.ts Error Handling Limited
**Location**: `frontend/src/lib/download.ts`

**Issue**: Error handling only logs error or shows toast - doesn't provide retry mechanism or fallback to window.open.

**Impact**: Users might have to refresh and try again if download fails transiently.

**Todo**: Consider adding retry logic for transient network errors.

### 4. Preview URL Uses Fixed Hostname
**Location**: `backend/internal/handlers/files.go` line 416

**Issue**: Preview URLs use hardcoded `localhost:8080` as fallback when `X-Forwarded-Host` header is missing.

**Impact**: If running behind a reverse proxy without the header, preview URLs will be incorrect.

**Todo**: Make the fallback hostname configurable via environment variable.

## Potential Improvements

### 1. Preview Token via URL Path Instead of Query Param
**Current**: `http://localhost:8080/api/files/:id/proxy?token=<token>`

**Alternative**: `http://localhost:8080/api/files/proxy/:token`

**Benefit**: Cleaner URLs, some CDNs/proxies might handle query parameters differently.

**Tradeoff**: Less descriptive URLs, requires route restructure.

### 2. Add Token Expiration Headers to Response
**Current**: Browser relies on token expiration check in backend.

**Alternative**: Set `Expires` or `Cache-Control: max-age` headers based on token expiration.

**Benefit**: Browser can cache file preview for token's lifetime.

### 3. Token-Specific CORS Headers
**Current**: Same CORS policy for all endpoints.

**Alternative**: Restrict preview proxy origin to only the frontend URL.

**Benefit**: Harder for unauthorized sites to use preview tokens.

## Security Considerations

### 1. Token Replay Protection
Single-use tokens prevent replay attacks, but ensure:
- Tokens are only generated server-side
- Tokens are validated before access is granted
- Tokens are marked as used immediately after access

### 2. Token Expiration
15-minute expiration balances security vs usability. Consider:
- Shorter for sensitive files (configurable per-file?)
- Longer for large files that take time to preview

### 3. Token Leak Vector
If browser logs, network packets, or browser history capture tokens:
- Token is only valid for 15 minutes
- Token can only be used once
- Requires user to already have been authenticated with JWT

This limits the damage from token leakage compared to permanent credentials.

## Testing Recommendations

### Manual Tests
1. Preview valid file with new token - should work
2. Replay same token - should fail (token already used)
3. Try expired token - should fail
4. Try invalid token format - should fail with error
5. Download without popup - should trigger direct download
6. Preview from iframe - should load without Authorization header
7. Access without token - should fall back to Authorization header check

### Automated Tests
Consider adding:
- Token validation tests
- Replay attack prevention tests
- Blob download fallback tests
- Preview route ordering tests