# AGENTS.md - backend/internal/handlers

## OVERVIEW
Core HTTP request handlers for the DocShare API, implementing the REST interface using the Go Fiber framework.

## WHERE TO LOOK
| File | Purpose |
|------|---------|
| `auth.go` | User registration, login, and session management. |
| `files.go` | File CRUD, uploads, downloads, and metadata management. |
| `users.go` | User profile management and administrative actions. |
| `groups.go` | Group creation, membership, and role-based access control. |
| `shares.go` | Public and private file sharing logic and permissions. |
| `transfers.go` | Temporary file transfer codes and ownership logic. |
| `device_auth.go` | OAuth2 device flow (RFC 8628) for CLI authentication. |
| `api_tokens.go` | Personal access token (PAT) lifecycle management. |
| `audit.go` | Audit log retrieval and filtering. |
| `activities.go` | User activity feed and event tracking. |
| `testutil_test.go` | Shared test harness for handler integration tests. |

## CONVENTIONS
- **Struct-based Handlers**: Handlers are methods on structs (e.g., `AuthHandler`) to allow dependency injection (DB, Services).
- **Response Helpers**: Use `utils.Error(c, status, message)` and `utils.Success(c, status, data)` for consistent API responses.
- **Request Parsing**: Define local `*Request` structs for `c.BodyParser` to ensure type safety and validation.
- **Audit Logging**: Every state-changing action MUST be logged asynchronously via `h.Audit.LogAsync`.
- **Request ID**: Use `getRequestID(c)` to correlate audit logs with the specific HTTP request.
- **UUID Parsing**: Use `parseUUID(value)` helper from `helpers.go` for consistent ID handling.

## ANTI-PATTERNS
- **Direct DB Logic**: Avoid complex business logic in handlers; delegate to `internal/services` where possible.
- **Hardcoded Status**: Use `fiber.Status...` constants instead of raw integers (e.g., `200`, `404`).
- **Missing Validation**: Never trust client input; validate all fields in the `*Request` struct before processing.
- **Synchronous Audit**: Do not block the request for audit logging; use the async service methods.
- **Manual Error Responses**: Avoid using `c.Status(x).JSON(...)` directly; use `utils.Error` to ensure uniform error formats.
