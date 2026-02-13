# DocShare API Documentation

Complete REST API reference for DocShare.

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Error Handling](#error-handling)
4. [Endpoints](#endpoints)
   - [Authentication](#authentication-endpoints)
   - [API Tokens](#api-token-endpoints)
   - [Device Flow](#device-flow-endpoints)
   - [Users](#user-endpoints)
   - [Files](#file-endpoints)
   - [Shares](#share-endpoints)
    - [Groups](#group-endpoints)
    - [Activities](#activity-endpoints)
    - [Audit Log](#audit-log-endpoints)

## Overview

### Base URL

```
Development: http://localhost:8080/api
Production: https://your-domain.com/api
```

### Content Type

All requests and responses use JSON unless otherwise specified.

```
Content-Type: application/json
```

### Response Format

All successful responses follow this structure:

```json
{
  "success": true,
  "data": { ... }
}
```

Error responses:

```json
{
  "success": false,
  "error": "Error message here"
}
```

Paginated responses include pagination metadata:

```json
{
  "success": true,
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "totalPages": 5
  }
}
```

## Authentication

Most endpoints require a valid token in the Authorization header. DocShare supports three types of authentication:

1. **JWT Tokens** — Used by the web frontend. Obtained via login/register.
2. **API Tokens** — Long-lived tokens for CLI and programmatic use.
3. **Device Flow** — For CLI tools that cannot open a browser directly.

For JWT and Device Flow tokens:
```
Authorization: Bearer <jwt_token>
```

For API tokens (prefixed with `dsh_`):
```
Authorization: Bearer dsh_<token_value>
```

### Obtaining a Token

- **Web Login**: Use the `/auth/register` or `/auth/login` endpoints.
- **API Tokens**: Generate tokens in Settings → API Tokens or via the `/auth/tokens` endpoints.
- **Device Flow**: Use the `/auth/device/code` and `/auth/device/token` flow.

### Token Expiration

Tokens expire after 24 hours (configurable). The frontend should handle 401 responses by redirecting to login.

## Error Handling

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing or invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |

### Error Response Examples

**Invalid Input (400)**
```json
{
  "success": false,
  "error": "email is required"
}
```

**Unauthorized (401)**
```json
{
  "success": false,
  "error": "invalid or expired token"
}
```

**Forbidden (403)**
```json
{
  "success": false,
  "error": "you don't have permission to access this file"
}
```

---

## Authentication Endpoints

### Register User

Create a new user account.

**Endpoint:** `POST /auth/register`

**Authentication:** Not required

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "securepassword123",
  "firstName": "John",
  "lastName": "Doe"
}
```

**Validation:**
- `email`: Required, valid email format, must be unique
- `password`: Required, minimum 6 characters
- `firstName`: Required, 1-100 characters
- `lastName`: Required, 1-100 characters

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "user",
      "createdAt": "2024-02-11T10:30:00Z"
    }
  }
}
```

**Error Response (400):**
```json
{
  "success": false,
  "error": "email already exists"
}
```

**Notes:**
- First registered user is automatically assigned `admin` role
- Subsequent users receive `user` role

---

### Login

Authenticate and receive a JWT token.

**Endpoint:** `POST /auth/login`

**Authentication:** Not required

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "user",
      "avatarURL": null,
      "createdAt": "2024-02-11T10:30:00Z"
    }
  }
}
```

**Error Response (401):**
```json
{
  "success": false,
  "error": "invalid credentials"
}
```

---

### Get Current User

Get authenticated user's details.

**Endpoint:** `GET /auth/me`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "firstName": "John",
    "lastName": "Doe",
    "role": "user",
    "avatarURL": "https://example.com/avatar.jpg",
    "createdAt": "2024-02-11T10:30:00Z"
  }
}
```

---

### Update Current User

Update authenticated user's profile.

**Endpoint:** `PUT /auth/me`

**Authentication:** Required

**Request Body:**
```json
{
  "firstName": "Jane",
  "lastName": "Smith",
  "avatarURL": "https://example.com/new-avatar.jpg"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "firstName": "Jane",
    "lastName": "Smith",
    "role": "user",
    "avatarURL": "https://example.com/new-avatar.jpg",
    "createdAt": "2024-02-11T10:30:00Z"
  }
}
```

**Notes:**
- Email and role cannot be changed via this endpoint
- Use admin endpoints to change user roles

---

### Change Password

Change authenticated user's password.

**Endpoint:** `PUT /auth/password`

**Authentication:** Required

**Request Body:**
```json
{
  "currentPassword": "oldpassword123",
  "newPassword": "newpassword456"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "password changed successfully"
  }
}
```

**Error Response (400):**
```json
{
  "success": false,
  "error": "current password is incorrect"
}
```

---

## API Token Endpoints

### Create API Token

Generate a new long-lived personal access token.

**Endpoint:** `POST /auth/tokens`

**Authentication:** Required

**Request Body:**
```json
{
  "name": "My CLI Token",
  "expiresInDays": 90
}
```

**Validation:**
- `name`: Required, 1-100 characters
- `expiresInDays`: Optional, one of: 30, 90, 365 (null or omitted for "never")

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "ff0e8400-e29b-41d4-a716-446655440011",
    "name": "My CLI Token",
    "token": "dsh_7f8e9d0c1b2a3948576d5e4f3c2b1a09z8y7x6w5v4u3t2s1",
    "prefix": "dsh_7f8e9d",
    "expiresAt": "2024-05-11T10:30:00Z",
    "createdAt": "2024-02-11T10:30:00Z"
  }
}
```

**Notes:**
- The full `token` is only shown once upon creation.
- Users can have a maximum of 25 active tokens.

---

### List API Tokens

List all API tokens for the authenticated user.

**Endpoint:** `GET /auth/tokens`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "ff0e8400-e29b-41d4-a716-446655440011",
      "name": "My CLI Token",
      "prefix": "dsh_7f8e9d",
      "lastUsedAt": "2024-02-12T15:45:00Z",
      "expiresAt": "2024-05-11T10:30:00Z",
      "createdAt": "2024-02-11T10:30:00Z"
    }
  ]
}
```

---

### Revoke API Token

Permanently revoke an API token.

**Endpoint:** `DELETE /auth/tokens/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "token revoked successfully"
  }
}
```

---

## Device Flow Endpoints

### Request Device Code

Start the device authorization flow. Used by CLI tools.

**Endpoint:** `POST /auth/device/code`

**Authentication:** Not required

**Content-Type:** `application/x-www-form-urlencoded`

**Request Parameters:**
- `client_id` (required): The client identifier
- `scope` (optional): Requested scopes

**Success Response (200):**
```json
{
  "device_code": "7f8e9d0c1b2a3948576d5e4f3c2b1a09z8y7x6w5v4u3t2s1",
  "user_code": "BCDF-GHJK",
  "verification_uri": "http://localhost:3001/device",
  "verification_uri_complete": "http://localhost:3001/device?code=BCDF-GHJK",
  "expires_in": 900,
  "interval": 5
}
```

**Notes:**
- This endpoint returns standard RFC 8628 JSON (no `success`/`data` wrapper).

---

### Poll for Token

Exchange a device code for an access token. Used by CLI tools.

**Endpoint:** `POST /auth/device/token`

**Authentication:** Not required

**Content-Type:** `application/x-www-form-urlencoded`

**Request Parameters:**
- `grant_type` (required): Must be `urn:ietf:params:oauth:grant-type:device_code`
- `device_code` (required): The device code from the previous step
- `client_id` (required): The client identifier

**Success Response (200):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 86400
}
```

**Error Responses (400):**
```json
{ "error": "authorization_pending", "error_description": "The user has not yet approved the request" }
{ "error": "slow_down", "error_description": "Polling too frequently" }
{ "error": "expired_token", "error_description": "The device code has expired" }
{ "error": "access_denied", "error_description": "The user denied the request" }
```

**Notes:**
- This endpoint returns standard RFC 6749 error formats (no `success`/`data` wrapper).

---

### Verify User Code

Look up the status of a user code. Used by the browser during approval.

**Endpoint:** `GET /auth/device/verify`

**Authentication:** Required

**Query Parameters:**
- `code` (required): The `user_code` (e.g., `BCDF-GHJK`)

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "userCode": "BCDF-GHJK",
    "expiresAt": "2024-02-11T10:45:00Z",
    "status": "pending"
  }
}
```

---

### Approve Device Code

Approve a pending device authorization request. Used by the browser.

**Endpoint:** `POST /auth/device/approve`

**Authentication:** Required

**Request Body:**
```json
{
  "userCode": "BCDF-GHJK"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "device approved successfully"
  }
}
```

---

## User Endpoints

### Search Users

Search for users by email or name (for sharing purposes).

**Endpoint:** `GET /users/search`

**Authentication:** Required

**Query Parameters:**
- `q` (required): Search query string

**Example Request:**
```
GET /users/search?q=john
```

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "john@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "avatarURL": null
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "email": "johnny@example.com",
      "firstName": "Johnny",
      "lastName": "Smith",
      "avatarURL": null
    }
  ]
}
```

**Notes:**
- Searches in email, firstName, and lastName fields
- Case-insensitive
- Returns max 20 results

---

### List All Users (Admin)

List all users in the system.

**Endpoint:** `GET /users`

**Authentication:** Required (Admin only)

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20)

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "user",
      "createdAt": "2024-02-11T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 45,
    "totalPages": 3
  }
}
```

---

### Get User (Admin)

Get details of a specific user.

**Endpoint:** `GET /users/:id`

**Authentication:** Required (Admin only)

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "firstName": "John",
    "lastName": "Doe",
    "role": "user",
    "avatarURL": null,
    "createdAt": "2024-02-11T10:30:00Z"
  }
}
```

---

### Update User (Admin)

Update a user's details.

**Endpoint:** `PUT /users/:id`

**Authentication:** Required (Admin only)

**Request Body:**
```json
{
  "firstName": "Jane",
  "lastName": "Smith",
  "role": "admin"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "firstName": "Jane",
    "lastName": "Smith",
    "role": "admin",
    "createdAt": "2024-02-11T10:30:00Z"
  }
}
```

---

### Delete User (Admin)

Delete a user account.

**Endpoint:** `DELETE /users/:id`

**Authentication:** Required (Admin only)

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "user deleted successfully"
  }
}
```

**Notes:**
- This is a hard delete
- All user's files and shares will be affected
- Consider implementing soft delete in production

---

## File Endpoints

### Upload File

Upload a new file.

**Endpoint:** `POST /files/upload`

**Authentication:** Required

**Content-Type:** `multipart/form-data`

**Form Fields:**
- `file` (required): The file to upload
- `parentID` (optional): UUID of parent folder

**Example Request (curl):**
```bash
curl -X POST http://localhost:8080/api/files/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@/path/to/document.pdf" \
  -F "parentID=550e8400-e29b-41d4-a716-446655440000"
```

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "770e8400-e29b-41d4-a716-446655440003",
    "name": "document.pdf",
    "mimeType": "application/pdf",
    "size": 1048576,
    "isDirectory": false,
    "parentID": "550e8400-e29b-41d4-a716-446655440000",
    "ownerID": "660e8400-e29b-41d4-a716-446655440001",
    "storagePath": "files/770e8400-e29b-41d4-a716-446655440003/document.pdf",
    "thumbnailPath": null,
    "createdAt": "2024-02-11T11:00:00Z",
    "updatedAt": "2024-02-11T11:00:00Z"
  }
}
```

**Error Response (400):**
```json
{
  "success": false,
  "error": "file is required"
}
```

**Notes:**
- Maximum file size: 100MB (configurable)
- If parentID is omitted, file is uploaded to root
- Preview generation happens synchronously for supported formats

---

### Create Directory

Create a new folder.

**Endpoint:** `POST /files/directory`

**Authentication:** Required

**Request Body:**
```json
{
  "name": "My Documents",
  "parentID": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440004",
    "name": "My Documents",
    "mimeType": "application/directory",
    "size": 0,
    "isDirectory": true,
    "parentID": "550e8400-e29b-41d4-a716-446655440000",
    "ownerID": "660e8400-e29b-41d4-a716-446655440001",
    "storagePath": "",
    "thumbnailPath": null,
    "createdAt": "2024-02-11T11:05:00Z",
    "updatedAt": "2024-02-11T11:05:00Z"
  }
}
```

---

### List Root Files

List files and folders in the root directory.

**Endpoint:** `GET /files`

**Authentication:** Required

**Query Parameters:**
- `sort` (optional): Sort field (name, createdAt, size)
- `order` (optional): Sort order (asc, desc)

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440004",
      "name": "My Documents",
      "mimeType": "application/directory",
      "size": 0,
      "isDirectory": true,
      "parentID": null,
      "ownerID": "660e8400-e29b-41d4-a716-446655440001",
      "sharedWith": 0,
      "createdAt": "2024-02-11T11:05:00Z",
      "updatedAt": "2024-02-11T11:05:00Z"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440003",
      "name": "document.pdf",
      "mimeType": "application/pdf",
      "size": 1048576,
      "isDirectory": false,
      "parentID": null,
      "ownerID": "660e8400-e29b-41d4-a716-446655440001",
      "sharedWith": 2,
      "createdAt": "2024-02-11T11:00:00Z",
      "updatedAt": "2024-02-11T11:00:00Z"
    }
  ]
}
```

**Notes:**
- Only returns files the user owns or has access to
- `sharedWith` indicates number of active shares

---

### List Folder Contents

List files and folders within a specific folder.

**Endpoint:** `GET /files/:id/children`

**Authentication:** Required

**Query Parameters:**
- `sort` (optional): Sort field (name, createdAt, size)
- `order` (optional): Sort order (asc, desc)

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440005",
      "name": "report.docx",
      "mimeType": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "size": 524288,
      "isDirectory": false,
      "parentID": "880e8400-e29b-41d4-a716-446655440004",
      "ownerID": "660e8400-e29b-41d4-a716-446655440001",
      "sharedWith": 0,
      "createdAt": "2024-02-11T11:10:00Z",
      "updatedAt": "2024-02-11T11:10:00Z"
    }
  ]
}
```

**Error Response (403):**
```json
{
  "success": false,
  "error": "you don't have permission to access this folder"
}
```

---

### Get File Details

Get metadata for a specific file or folder.

**Endpoint:** `GET /files/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "770e8400-e29b-41d4-a716-446655440003",
    "name": "document.pdf",
    "mimeType": "application/pdf",
    "size": 1048576,
    "isDirectory": false,
    "parentID": "880e8400-e29b-41d4-a716-446655440004",
    "ownerID": "660e8400-e29b-41d4-a716-446655440001",
    "storagePath": "files/770e8400-e29b-41d4-a716-446655440003/document.pdf",
    "thumbnailPath": null,
    "createdAt": "2024-02-11T11:00:00Z",
    "updatedAt": "2024-02-11T11:00:00Z",
    "owner": {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe"
    },
    "sharedWith": 2
  }
}
```

---

### Get File Path (Breadcrumbs)

Get the full path from root to a file/folder.

**Endpoint:** `GET /files/:id/path`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440004",
      "name": "My Documents"
    },
    {
      "id": "990e8400-e29b-41d4-a716-446655440005",
      "name": "Projects"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440003",
      "name": "document.pdf"
    }
  ]
}
```

**Notes:**
- Useful for breadcrumb navigation
- Returns array from root to current item

---

### Download File

Download file content directly through the backend.

**Endpoint:** `GET /files/:id/download`

**Authentication:** Required

**Success Response (200):**
- **Content-Type**: File's actual MIME type
- **Content-Disposition**: `attachment; filename="document.pdf"`
- **Body**: Binary file content

**Error Response (403):**
```json
{
  "success": false,
  "error": "you don't have download permission for this file"
}
```

**Notes:**
- Requires `download` or `edit` permission
- Streams file through backend
- For large files, consider using `/download-url` instead

---

### Get Download URL

Get a presigned URL for direct download from MinIO.

**Endpoint:** `GET /files/:id/download-url`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "url": "http://localhost:9000/docshare/files/770e8400.../document.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&...",
    "expiresIn": 3600
  }
}
```

**Notes:**
- URL is valid for 1 hour
- Client downloads directly from MinIO (no backend traffic)
- Requires `download` or `edit` permission

---

### Get Preview URL

Get a URL for file preview.

**Endpoint:** `GET /files/:id/preview`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "url": "http://localhost:8080/api/files/770e8400.../proxy?token=abc123...",
    "type": "pdf",
    "available": true
  }
}
```

**Response Fields:**
- `url`: Preview URL (may be MinIO presigned URL or proxy URL)
- `type`: Preview type (pdf, image, video, etc.)
- `available`: Whether preview is ready

**Notes:**
- For PDFs and images: Returns direct MinIO URL
- For Office documents: Returns proxy URL (if conversion complete)
- Requires `view`, `download`, or `edit` permission

---

### Convert & Preview Document

Trigger preview generation for Office documents.

**Endpoint:** `GET /files/:id/convert-preview`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "preview generated successfully",
    "previewPath": "previews/770e8400-e29b-41d4-a716-446655440003.pdf"
  }
}
```

**Error Response (400):**
```json
{
  "success": false,
  "error": "preview generation not supported for this file type"
}
```

**Notes:**
- Synchronous operation (may take several seconds)
- Converts DOCX, XLSX, PPTX to PDF via Gotenberg
- Preview is cached in MinIO

---

### Proxy Preview

Proxy endpoint for serving previews with token-based auth.

**Endpoint:** `GET /files/:id/proxy`

**Authentication:** Token in query parameter

**Query Parameters:**
- `token` (required): Preview token

**Success Response (200):**
- **Content-Type**: `application/pdf` or image MIME type
- **Body**: Preview content

**Error Response (401):**
```json
{
  "success": false,
  "error": "invalid or expired preview token"
}
```

**Notes:**
- Used to embed previews in iframe/img tags
- Token expires after configured period
- Bypasses standard JWT auth

---

### Update File/Folder

Update file or folder metadata.

**Endpoint:** `PUT /files/:id`

**Authentication:** Required

**Request Body:**
```json
{
  "name": "renamed-document.pdf",
  "parentID": "new-parent-folder-id"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "770e8400-e29b-41d4-a716-446655440003",
    "name": "renamed-document.pdf",
    "mimeType": "application/pdf",
    "size": 1048576,
    "isDirectory": false,
    "parentID": "new-parent-folder-id",
    "ownerID": "660e8400-e29b-41d4-a716-446655440001",
    "createdAt": "2024-02-11T11:00:00Z",
    "updatedAt": "2024-02-11T11:30:00Z"
  }
}
```

**Notes:**
- Requires `edit` permission
- Can rename file/folder
- Can move to different parent folder
- Moving a folder moves all descendants

---

### Delete File/Folder

Delete a file or folder.

**Endpoint:** `DELETE /files/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "file deleted successfully"
  }
}
```

**Error Response (403):**
```json
{
  "success": false,
  "error": "only the owner can delete this file"
}
```

**Notes:**
- Only owner can delete
- Deleting a folder deletes all contents recursively
- Deletes file from MinIO storage
- This is a hard delete (no recovery)

---

## Share Endpoints

### Share File

Create a new share for a file or folder.

**Endpoint:** `POST /files/:id/share`

**Authentication:** Required

**Request Body (Share with User):**
```json
{
  "sharedWithUserID": "550e8400-e29b-41d4-a716-446655440000",
  "permission": "download",
  "expiresAt": "2024-12-31T23:59:59Z"
}
```

**Request Body (Share with Group):**
```json
{
  "sharedWithGroupID": "660e8400-e29b-41d4-a716-446655440001",
  "permission": "view"
}
```

**Permission Values:**
- `view`: Can view metadata and preview
- `download`: Can download file
- `edit`: Can modify file and manage shares

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "aa0e8400-e29b-41d4-a716-446655440006",
    "fileID": "770e8400-e29b-41d4-a716-446655440003",
    "sharedByID": "660e8400-e29b-41d4-a716-446655440001",
    "sharedWithUserID": "550e8400-e29b-41d4-a716-446655440000",
    "sharedWithGroupID": null,
    "permission": "download",
    "expiresAt": "2024-12-31T23:59:59Z",
    "createdAt": "2024-02-11T12:00:00Z"
  }
}
```

**Error Response (400):**
```json
{
  "success": false,
  "error": "must specify either sharedWithUserID or sharedWithGroupID"
}
```

**Notes:**
- Requires `edit` permission on the file
- Cannot specify both user and group
- `expiresAt` is optional (null = never expires)

---

### List File Shares

Get all shares for a specific file.

**Endpoint:** `GET /files/:id/shares`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440006",
      "fileID": "770e8400-e29b-41d4-a716-446655440003",
      "sharedByID": "660e8400-e29b-41d4-a716-446655440001",
      "sharedWithUserID": "550e8400-e29b-41d4-a716-446655440000",
      "permission": "download",
      "expiresAt": "2024-12-31T23:59:59Z",
      "createdAt": "2024-02-11T12:00:00Z",
      "sharedWithUser": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "email": "collaborator@example.com",
        "firstName": "Alice",
        "lastName": "Johnson"
      }
    }
  ]
}
```

**Notes:**
- Requires `edit` permission to view shares
- Includes user/group details

---

### List Shared With Me

Get all files shared with the authenticated user.

**Endpoint:** `GET /shared`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440006",
      "fileID": "770e8400-e29b-41d4-a716-446655440003",
      "permission": "download",
      "expiresAt": null,
      "createdAt": "2024-02-11T12:00:00Z",
      "file": {
        "id": "770e8400-e29b-41d4-a716-446655440003",
        "name": "document.pdf",
        "mimeType": "application/pdf",
        "size": 1048576,
        "isDirectory": false,
        "owner": {
          "id": "660e8400-e29b-41d4-a716-446655440001",
          "email": "owner@example.com",
          "firstName": "John",
          "lastName": "Doe"
        }
      },
      "sharedBy": {
        "id": "660e8400-e29b-41d4-a716-446655440001",
        "email": "owner@example.com",
        "firstName": "John",
        "lastName": "Doe"
      }
    }
  ]
}
```

**Notes:**
- Includes shares via direct user assignment
- Includes shares via group membership
- Excludes expired shares

---

### Update Share

Update an existing share's permission or expiration.

**Endpoint:** `PUT /shares/:id`

**Authentication:** Required

**Request Body:**
```json
{
  "permission": "edit",
  "expiresAt": "2025-12-31T23:59:59Z"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "aa0e8400-e29b-41d4-a716-446655440006",
    "fileID": "770e8400-e29b-41d4-a716-446655440003",
    "permission": "edit",
    "expiresAt": "2025-12-31T23:59:59Z",
    "updatedAt": "2024-02-11T12:30:00Z"
  }
}
```

**Notes:**
- Requires `edit` permission on the file
- Can update permission level or expiration independently

---

### Delete Share

Revoke a share.

**Endpoint:** `DELETE /shares/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "share deleted successfully"
  }
}
```

**Error Response (403):**
```json
{
  "success": false,
  "error": "only the file owner or share creator can delete this share"
}
```

**Notes:**
- File owner or share creator can delete
- Share is immediately revoked

---

## Group Endpoints

### Create Group

Create a new group.

**Endpoint:** `POST /groups`

**Authentication:** Required

**Request Body:**
```json
{
  "name": "Marketing Team",
  "description": "Marketing department collaboration space"
}
```

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "bb0e8400-e29b-41d4-a716-446655440007",
    "name": "Marketing Team",
    "description": "Marketing department collaboration space",
    "createdByID": "660e8400-e29b-41d4-a716-446655440001",
    "createdAt": "2024-02-11T13:00:00Z"
  }
}
```

**Notes:**
- Creator is automatically added as owner
- Description is optional

---

### List Groups

List all groups the user is a member of.

**Endpoint:** `GET /groups`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "bb0e8400-e29b-41d4-a716-446655440007",
      "name": "Marketing Team",
      "description": "Marketing department collaboration space",
      "createdByID": "660e8400-e29b-41d4-a716-446655440001",
      "memberCount": 5,
      "createdAt": "2024-02-11T13:00:00Z"
    }
  ]
}
```

**Notes:**
- Only returns groups where user is a member
- Includes member count

---

### Get Group

Get details of a specific group including members.

**Endpoint:** `GET /groups/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "bb0e8400-e29b-41d4-a716-446655440007",
    "name": "Marketing Team",
    "description": "Marketing department collaboration space",
    "createdByID": "660e8400-e29b-41d4-a716-446655440001",
    "createdAt": "2024-02-11T13:00:00Z",
    "memberships": [
      {
        "id": "cc0e8400-e29b-41d4-a716-446655440008",
        "groupID": "bb0e8400-e29b-41d4-a716-446655440007",
        "userID": "660e8400-e29b-41d4-a716-446655440001",
        "role": "owner",
        "createdAt": "2024-02-11T13:00:00Z",
        "user": {
          "id": "660e8400-e29b-41d4-a716-446655440001",
          "email": "john@example.com",
          "firstName": "John",
          "lastName": "Doe",
          "avatarURL": null
        }
      }
    ]
  }
}
```

**Notes:**
- Requires group membership to view
- Includes full member list with user details

---

### Update Group

Update group details.

**Endpoint:** `PUT /groups/:id`

**Authentication:** Required

**Request Body:**
```json
{
  "name": "Marketing & Sales Team",
  "description": "Combined marketing and sales collaboration"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "bb0e8400-e29b-41d4-a716-446655440007",
    "name": "Marketing & Sales Team",
    "description": "Combined marketing and sales collaboration",
    "createdByID": "660e8400-e29b-41d4-a716-446655440001",
    "createdAt": "2024-02-11T13:00:00Z",
    "updatedAt": "2024-02-11T14:00:00Z"
  }
}
```

**Notes:**
- Requires `owner` or `admin` role in group

---

### Delete Group

Delete a group.

**Endpoint:** `DELETE /groups/:id`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "group deleted successfully"
  }
}
```

**Error Response (403):**
```json
{
  "success": false,
  "error": "only group owners can delete the group"
}
```

**Notes:**
- Only group owners can delete
- All memberships are removed
- Existing shares to this group are removed

---

### Add Group Member

Add a user to a group.

**Endpoint:** `POST /groups/:id/members`

**Authentication:** Required

**Request Body:**
```json
{
  "userID": "550e8400-e29b-41d4-a716-446655440000",
  "role": "member"
}
```

**Role Values:**
- `owner`: Full control
- `admin`: Can manage members
- `member`: Standard member

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "dd0e8400-e29b-41d4-a716-446655440009",
    "groupID": "bb0e8400-e29b-41d4-a716-446655440007",
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "role": "member",
    "createdAt": "2024-02-11T14:30:00Z"
  }
}
```

**Notes:**
- Requires `owner` or `admin` role in group
- User cannot already be a member

---

### Update Member Role

Update a group member's role.

**Endpoint:** `PUT /groups/:id/members/:userId`

**Authentication:** Required

**Request Body:**
```json
{
  "role": "admin"
}
```

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "dd0e8400-e29b-41d4-a716-446655440009",
    "groupID": "bb0e8400-e29b-41d4-a716-446655440007",
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "role": "admin",
    "updatedAt": "2024-02-11T15:00:00Z"
  }
}
```

**Notes:**
- Requires `owner` or `admin` role in group
- Cannot change your own role

---

### Remove Group Member

Remove a user from a group.

**Endpoint:** `DELETE /groups/:id/members/:userId`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "member removed successfully"
  }
}
```

**Error Response (403):**
```json
{
  "success": false,
  "error": "insufficient permissions to remove member"
}
```

**Notes:**
- Requires `owner` or `admin` role in group
- Members can remove themselves
- Cannot remove the last owner

---

---

## Activity Endpoints

### List Activities

Get a paginated list of activities for the authenticated user.

**Endpoint:** `GET /activities`

**Authentication:** Required

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20)

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "ee0e8400-e29b-41d4-a716-446655440010",
      "userID": "550e8400-e29b-41d4-a716-446655440000",
      "actorID": "660e8400-e29b-41d4-a716-446655440001",
      "action": "file.upload",
      "resourceType": "file",
      "resourceID": "770e8400-e29b-41d4-a716-446655440003",
      "resourceName": "document.pdf",
      "message": "John Doe uploaded document.pdf",
      "isRead": false,
      "createdAt": "2024-02-11T16:00:00Z",
      "actor": {
        "id": "660e8400-e29b-41d4-a716-446655440001",
        "email": "john@example.com",
        "firstName": "John",
        "lastName": "Doe",
        "avatarURL": null
      }
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 15,
    "totalPages": 1
  }
}
```

---

### Get Unread Count

Get the number of unread activities for the authenticated user.

**Endpoint:** `GET /activities/unread-count`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "count": 5
  }
}
```

---

### Mark All as Read

Mark all activities as read for the authenticated user.

**Endpoint:** `PUT /activities/read-all`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "all marked as read"
  }
}
```

---

### Mark Activity as Read

Mark a specific activity as read.

**Endpoint:** `PUT /activities/:id/read`

**Authentication:** Required

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "message": "marked as read"
  }
}
```

**Error Response (404):**
```json
{
  "success": false,
  "error": "activity not found"
}
```

**Notes:**
- Returns 404 if activity not found or doesn't belong to user

---

## Audit Log Endpoints

### Export My Audit Log

Export the authenticated user's audit log entries.

**Endpoint:** `GET /audit-log/export`

**Authentication:** Required

**Query Parameters:**
- `format` (optional): Export format, `csv` or `json` (default: `csv`)

**Success Response (200 - CSV):**
- **Content-Type**: `text/csv`
- **Content-Disposition**: `attachment; filename="audit_log.csv"`
- **Body**:
```csv
Timestamp,Action,Resource Type,Resource ID,IP Address,Details
2024-02-11T10:30:00Z,user.login,user,550e8400-e29b-41d4-a716-446655440000,192.168.1.1,User logged in
2024-02-11T11:00:00Z,file.upload,file,770e8400-e29b-41d4-a716-446655440003,192.168.1.1,Uploaded document.pdf
```

**Success Response (200 - JSON):**
```json
{
  "success": true,
  "data": [
    {
      "timestamp": "2024-02-11T10:30:00Z",
      "action": "user.login",
      "resourceType": "user",
      "resourceID": "550e8400-e29b-41d4-a716-446655440000",
      "ipAddress": "192.168.1.1",
      "details": "User logged in"
    }
  ]
}
```

**Notes:**
- Limited to 10,000 most recent entries
- Only returns the authenticated user's own audit log entries

---

## Rate Limiting

Currently not implemented. Consider adding rate limiting in production:

- Authentication endpoints: 5 requests per minute
- Upload endpoints: 10 requests per minute per user
- Other endpoints: 100 requests per minute per user

## Webhooks

Not currently supported. Future enhancement could include:
- File uploaded
- File shared
- Group membership changed

## API Versioning

Current API is unversioned. Future versions could be prefixed:
- `/api/v1/files`
- `/api/v2/files`

## Additional Resources

- [README.md](./README.md) - Project overview and setup
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Architecture details
- [DEPLOYMENT.md](./DEPLOYMENT.md) - Deployment guide
