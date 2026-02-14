# UTILS KNOWLEDGE BASE

## OVERVIEW
Shared Go utilities for authentication, pagination, and standardized API responses.

## WHERE TO LOOK
| Utility | Purpose | Key Functions |
|---------|---------|---------------|
| `jwt.go` | JWT management | `GenerateToken`, `ValidateToken` |
| `password.go` | Security | `HashPassword`, `CheckPassword` |
| `pagination.go` | API Pagination | `ParsePagination`, `ApplyPagination` |
| `response.go` | Fiber Responses | `Success`, `Error`, `Paginated` |

## CONVENTIONS
- **Stateless**: Utilities must be thread-safe and avoid internal state.
- **Fiber Integration**: Functions interacting with the web layer must accept `*fiber.Ctx` as the first parameter.
- **GORM Scopes**: Pagination helpers return `*gorm.DB` to support clean method chaining in services/handlers.
- **Error Handling**: Return errors to callers; do not log or panic within the utility package.

## ANTI-PATTERNS
- **Business Logic**: Never include domain-specific logic (e.g., "is user active?"); keep utils generic.
- **Manual JSON**: Avoid calling `c.JSON()` directly in handlers; use `response.go` wrappers for consistency.
- **Hardcoded Secrets**: Do not modify `jwtSecret` directly; use `ConfigureJWT` during server initialization.
- **Missing Tests**: Every utility must have a corresponding `*_test.go` file in the same directory.
