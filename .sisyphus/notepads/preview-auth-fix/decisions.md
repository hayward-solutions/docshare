# Preview Auth Fix - Architectural Decisions

## Decision 1: Preview Token vs Cookie-Based Auth

**Chosen**: Preview tokens (signed HMAC tokens in query parameters)

**Rationale**:
- Cookies require CORS complexity and HttpOnly handling
- Tokens are stateless and simpler to implement
- Tokens work without CORS preflight issues in iframes
- Single-use tokens prevent token leakage risk
- 15-minute expiration limits security exposure

**Rejected alternatives**:
- Cookie-based auth: Requires CORS configuration, HttpOnly cookies, more complex session management
- Permanent tokens: Higher security risk if leaked
- No auth at all: Defeats permission system

## Decision 2: Route Ordering for Public Proxy Endpoint

**Chosen**: Define `api.Get("/files/:id/proxy", ...)` BEFORE `fileRoutes` group

**Fiber route matching order**: Routes are matched in registration order. Earlier routes take precedence.

**Code structure**:
```go
// Public proxy endpoint FIRST
api.Get("/files/:id/proxy", filesHandler.ProxyPreview)

// Then file routes all require auth
fileRoutes := api.Group("/files", authMiddleware.RequireAuth)
```

**Rejected alternatives**:
- Separate middleware for proxy endpoint: More complex, duplication
- Middleware in ProxyPreview handler itself: Cleaner but harder to maintain

## Decision 3: Single-Use Tokens

**Chosen**: Mark tokens as used after first successful access

**Rationale**:
- Prevents token replay attacks
- Tokens in browser history or network logs are useless after first use
- No need for complex revocation mechanisms

**Implementation**:
- In-memory store: `map[string]time.Time` of used tokens
- `previewtoken.MarkUsed()` when access succeeds
- Check `previewtoken.IsUsed()` before allowing access
- Cleanup of old tokens periodically (handled by `Cleanup()`)

## Decision 4: HMAC-SHA256 vs JWT for Preview Tokens

**Chosen**: HMAC-SHA256 with custom format

**Rationale**:
- Lighter than full JWT for simple token use case
- Can validate signature without database lookup
- Format: `base64(json_data).signature`
- Simple to parse and validate

**Rejected alternatives**:
- Full JWT: Overkill for one-time tokens, requires additional package
- Unencrypted tokens: Easy to tamper with, insecure

## Decision 5: Blob Download vs Presigned URLs for Downloads

**Chosen**: Blob download with temporary link element

**Rationale**:
- Works with existing auth system (no separate token system needed for download)
- Allows filename specification
- No popup windows
- Uses existing Authorization header mechanism

**Implementation**:
- `fetch()` with Authorization header
- `response.blob()` for binary data
- `URL.createObjectURL()` + `<a download>` for direct download
- Cleanup with `URL.revokeObjectURL()`

**Rejected alternatives**:
- `window.open(url, '_blank')`: Opens popup window (the problem being fixed)
- Stream download via iframe: Can't specify filename reliably
- Presigned MinIO URLs: Had signature issues, requires hostname replacement

## Technical Notes

### HMAC Secret Placeholder
The `previewtoken.go` currently uses a hardcoded secret key:
```go
key := []byte("docshare-preview-token-secret")
```
**TODO**: Use JWT secret from config when available (`config.JWT.Secret`).

### Token Cleanup Period
The preview token store doesn't automatically clean up. Consider starting a goroutine in main.go to run `previewtoken.Cleanup()` periodically.

### CORS Configuration
Current CORS config allows `Authorization` header:
```go
AllowHeaders: "Origin, Content-Type, Accept, Authorization"
```
This is required for both token and header-based auth to work.