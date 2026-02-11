# Preview Auth Fix - Learnings

## Problem 1: File Preview Authentication

**Issue**: When loading preview URLs in `<iframe>`, browser doesn't send Authorization headers, causing:
```json
{"error":"missing authorization header","success":false}
```

**Root Cause**:
- `FileViewer` component loads `/api/files/:id/preview`
- This returns a proxy URL: `http://localhost:8080/api/files/:id/proxy`
- Browser loads proxy URL in `<iframe>` without Authorization header
- `ProxyPreview` handler requires Bearer authentication

## Solution: Preview Token System

Created one-time-use signed tokens for preview access.

### Backend Implementation

**File structure**:
- `backend/pkg/previewtoken/previewtoken.go` - Token generation/validation
- `backend/internal/handlers/files.go` - Modified PreviewURL/ProxyPreview handlers
- `backend/cmd/server/main.go` - Public route for proxy endpoint

**Token format**: `base64(json_data).signature`
- JSON contains: fileID, userID, expiresAt
- Signed with HMAC-SHA256 using JWT secret
- Valid for 15 minutes
- Single-use (marked as used after first successful access)

**PreviewURL handler changes**:
- Generates token via `previewtoken.Generate(fileID, userID)`
- Returns URL: `http://localhost:8080/api/files/:id/proxy?token=<token>`

**ProxyPreview handler changes**:
```go
previewToken := c.Query("token")
if previewToken != "" {
    if previewtoken.IsUsed(previewToken) {
        return Error(unauthorized, "token already used")
    }
    tokenFileID, tokenUserID, err := previewtoken.GetMetadata(previewToken)
    if err == nil && tokenFileID == fileID.String() {
        // Lookup user and validate access
        currentUser = &user
    }
}
// Fall back to Authorization header if no token
```

**Route setup**:
```go
// Order matters - public route must come BEFORE fileRoutes group
api.Get("/files/:id/proxy", filesHandler.ProxyPreview)
fileRoutes := api.Group("/files", authMiddleware.RequireAuth)
```

## Problem 2: Download Popup

**Issue**: Downloads opened in new window via `window.open(res.data.url, '_blank')`

**Root Cause**:
- Three locations using `window.open()`:
  - `frontend/src/components/file-viewer.tsx:60`
  - `frontend/src/app/(dashboard)/files/page.tsx:93`
  - `frontend/src/app/(dashboard)/files/[id]/page.tsx:121`

**Solution**: Blob download utility

### Frontend Implementation

**New file**: `frontend/src/lib/download.ts`

```typescript
export async function downloadFile({
  url,
  filename,
  token
}: DownloadOptions): Promise<void> {
  const response = await fetch(url, { 
    headers: { 'Accept': '*/*', 'Authorization': `Bearer ${token}` }
  });
  const blob = await response.blob();
  const objectUrl = window.URL.createObjectURL(blob);
  
  const link = document.createElement('a');
  link.href = objectUrl;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  
  window.URL.revokeObjectURL(objectUrl);
  document.body.removeChild(link);
}
```

**Updated all handleDownload functions** to use utility:
- Remove `window.open()` calls
- Pass file ID and filename to utility
- Handles errors with toast notifications
