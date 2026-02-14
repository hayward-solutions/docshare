# CLI COMMANDS KNOWLEDGE BASE

## OVERVIEW
Cobra command implementations for the DocShare CLI, handling user interaction and API orchestration.

## WHERE TO LOOK
| Command | File | Purpose |
|---------|------|---------|
| **Root** | `root.go` | Entry point, global flags (`--json`, `--server`), and config loading. |
| **Auth** | `login.go`, `logout.go` | OAuth2 device flow, API token auth, and session termination. |
| **Transfer** | `upload.go`, `download.go` | Recursive file/directory transfers with worker pools and concurrency. |
| **Discovery** | `ls.go`, `search.go`, `info.go` | File listing, search, and detailed metadata retrieval. |
| **Sharing** | `share.go`, `unshare.go`, `shared.go` | Management of file permissions, public links, and shared items. |
| **Filesystem** | `mkdir.go`, `mv.go`, `rm.go` | Remote file operations (create, move, delete) using path resolution. |
| **System** | `version.go`, `upgrade.go`, `whoami.go` | CLI versioning, self-update logic, and identity checks. |
| **Transfer** | `transfer.go` | Logic for transferring ownership of files or groups. |

## CONVENTIONS
- **Auth Guard**: Use `requireAuth()` at the start of `RunE` for any command requiring a token.
- **Path Resolution**: Use `pathutil.Resolve(apiClient, path)` to convert user-provided paths or IDs into UUIDs.
- **Output**: Prefer `internal/output` helpers (e.g., `output.FileTable`, `output.JSON`) to respect the `--json` flag.
- **API Client**: Use the pre-initialized `apiClient` from `root.go` instead of creating new instances.
- **Errors**: Set `SilenceUsage: true` and `SilenceErrors: true` in `rootCmd` to handle error formatting manually in `Execute()`.
- **Concurrency**: For batch operations (like `upload`), use `sync.WaitGroup` and worker channels to manage load.
- **Flags**: Use `PersistentFlags()` in `root.go` for global options and `Flags()` in `init()` for command-specific ones.
- **Validation**: Use `cobra.ExactArgs`, `cobra.MinimumNArgs`, or `cobra.RangeArgs` to validate command arguments.
- **Long Descriptions**: Use backticks for `Long` descriptions in `cobra.Command` to include usage examples.

## ANTI-PATTERNS
- **Direct Printing**: Avoid `fmt.Println` for data; it breaks machine-readable `--json` output.
- **Manual Path Parsing**: Don't manually parse `/` paths; let `pathutil` handle the API traversal.
- **Hardcoded URLs**: Never hardcode the backend URL; always use `cfg.ServerURL` from the loaded config.
- **Fat Handlers**: Keep `RunE` logic focused on argument parsing and output; move complex logic to `internal/`.
- **Global State**: Avoid adding new global variables in `root.go` unless they are truly cross-cutting flags.
