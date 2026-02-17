# MODELS KNOWLEDGE BASE

## OVERVIEW
GORM data models for the DocShare backend, defining the database schema and relationships.

## WHERE TO LOOK
- `base.go`: `BaseModel` with UUID primary keys and GORM hooks.
- `user.go`: User accounts, roles (admin/user), and authentication data.
- `file.go`: File and directory metadata, hierarchical structure (Parent/Children).
- `group.go` & `group_membership.go`: Team organization and role-based access.
- `share.go`: Granular permissions (view/download/edit) and share types.
- `activity.go`: User-facing notifications for file and group events.
- `audit_log.go`: Append-only security event logging (does NOT use `BaseModel`).
- `api_token.go` & `device_code.go`: CLI authentication and personal access tokens.
- `preview_job.go`: Tracks asynchronous document preview generation states.
- `transfer.go`: Direct file transfers between users via short codes.

## CONVENTIONS
- **UUIDs**: All primary and foreign keys must use `uuid.UUID`.
- **BaseModel**: Embed `BaseModel` for standard ID/timestamps (except `AuditLog`).
- **GORM Tags**: Explicitly define types (e.g., `type:uuid`, `type:varchar(255)`).
- **JSON**: Use `camelCase` for JSON tags; use `json:"-"` for sensitive fields.
- **Pluralization**: Use `TableName()` for non-standard plurals (e.g., `activities`).
- **Indexes**: Always index foreign keys and fields used in `WHERE` clauses.

## ANTI-PATTERNS
- **No Logic**: Avoid complex business logic in models; keep them as data structures.
- **No Int IDs**: Never use `uint` or `int` for primary keys.
- **Soft Deletes**: Be careful with `DeletedAt` (from `BaseModel`); ensure queries handle it correctly.
- **Audit Logs**: Do not update or delete `AuditLog` entries; they are append-only.
