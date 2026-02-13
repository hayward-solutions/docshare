# DocShare CLI

A fast, single-binary command-line interface for managing files on your DocShare server.

## Table of Contents

1. [Installation](#installation)
2. [Quick Start](#quick-start)
3. [Authentication](#authentication)
4. [Commands](#commands)
   - [Files](#files)
   - [Upload & Download](#upload--download)
   - [Sharing](#sharing)
5. [Path Resolution](#path-resolution)
6. [Global Flags](#global-flags)
7. [Configuration](#configuration)
8. [Shell Completion](#shell-completion)
9. [Examples](#examples)
10. [Building from Source](#building-from-source)

---

## Installation

### One-line install (recommended)

```bash
curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh
```

This will:
1. Download the pre-built binary for your OS and architecture from GitHub Releases
2. Fall back to building from source if no binary is available (requires Go 1.24+)
3. Install to `/usr/local/bin/docshare`

**Install to a custom directory:**

```bash
curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh -s -- --dir ~/.local/bin
```

**Install a specific version:**

```bash
curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh -s -- --version v1.0.0
```

### Homebrew (macOS/Linux)

*Coming soon.*

### Manual download

Download a pre-built binary from [GitHub Releases](https://github.com/hayward-solutions/docshare/releases), extract it, and place it somewhere on your `PATH`:

```bash
tar -xzf docshare_*_linux_amd64.tar.gz
sudo mv docshare /usr/local/bin/
```

### Build from source

Requires Go 1.24+:

```bash
git clone https://github.com/hayward-solutions/docshare.git
cd docshare/cli
go build -o docshare .
sudo mv docshare /usr/local/bin/
```

---

## Quick Start

```bash
# 1. Point to your server (skip if using localhost:8080)
docshare login --server https://docshare.example.com

# 2. Authenticate
docshare login

# 3. Start using it
docshare ls
docshare upload report.pdf
docshare download /Documents/report.pdf
```

---

## Authentication

DocShare CLI supports two authentication methods.

### Device flow (default)

The device flow opens your browser to approve the CLI session. This is the recommended method for interactive use.

```bash
docshare login
```

The CLI will:
1. Request a device code from the server
2. Open your browser to the approval page
3. Display a user code (e.g. `BCDF-GHJK`) in case the browser doesn't open
4. Poll until you approve, then save the token

### API token

For scripting or headless environments, use an API token. Generate one in the DocShare web UI under **Settings > API Tokens**, then:

```bash
docshare login --token dsh_7f8e9d0c1b2a3948576d5e4f...
```

The token is validated against the server and saved to your local config.

### Verify authentication

```bash
docshare whoami
```

### Log out

```bash
docshare logout
```

This removes the stored token from your machine.

---

## Commands

### Files

#### `ls` — List files and directories

```bash
docshare ls                          # List root directory
docshare ls /Documents               # List folder by path
docshare ls /Documents --sort name   # Sort by name, createdAt, or size
docshare ls /Documents --order desc  # Sort descending
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--sort` | Sort field: `name`, `createdAt`, `size` |
| `--order` | Sort order: `asc`, `desc` |

#### `info` — File details

```bash
docshare info /Documents/report.pdf
```

Displays name, ID, type, size, owner, share count, and timestamps.

#### `search` — Search by name

```bash
docshare search "quarterly report"
```

#### `mkdir` — Create a directory

```bash
docshare mkdir "My Documents"                  # Create in root
docshare mkdir Reports /Documents              # Create inside a folder
docshare mkdir /Documents/Reports/Q1           # Create with full path
docshare mkdir Reports --parent <folder-id>    # Create by parent ID
```

#### `rm` — Delete a file or directory

```bash
docshare rm /Documents/old-report.pdf          # Prompts for confirmation
docshare rm /Temp --force                      # Skip confirmation
```

Deleting a directory removes all contents recursively. This cannot be undone.

#### `mv` — Move or rename

```bash
docshare mv /Documents/report.pdf /Archive     # Move to different folder
docshare mv /Documents/old.pdf new-name.pdf    # Rename
```

If the destination resolves to an existing directory, the file is moved into it. Otherwise, the file is renamed.

---

### Upload & Download

#### `upload` — Upload files and directories

**Single file:**
```bash
docshare upload report.pdf                     # Upload to root
docshare upload report.pdf /Documents          # Upload to a folder
```

**Directory (recursive):**
```bash
docshare upload ./project/ /Documents          # Uploads all files and recreates directory structure
docshare upload ./project/ /Documents -w 8     # Use 8 concurrent upload workers
```

Directory uploads create the remote folder structure automatically and upload files in parallel using a worker pool (default: 4 workers).

**Flags:**
| Flag | Description |
|------|-------------|
| `--parent` | Parent folder ID (alternative to positional path argument) |
| `-w`, `--workers` | Number of concurrent upload workers for directories (default: `4`) |

#### `download` — Download files and directories

**Single file:**
```bash
docshare download /Documents/report.pdf            # Download to current directory
docshare download /Documents/report.pdf ./out       # Download to specific directory
docshare download /Documents/report.pdf -o my.pdf   # Download with custom filename
```

**Directory (recursive):**
```bash
docshare download /Projects                         # Download entire folder tree
docshare download /Projects ./backup                # Download to specific directory
```

Downloads use presigned URLs to stream directly from object storage, bypassing the backend for maximum throughput.

**Flags:**
| Flag | Description |
|------|-------------|
| `-o`, `--output` | Output file path (overrides default naming) |

---

### Sharing

#### `share` — Share a file with a user

```bash
docshare share /Documents/report.pdf alice@example.com
docshare share /Documents/report.pdf alice@example.com --permission edit
```

The CLI looks up the user by email via the API. Permission levels:
- `view` — Can see metadata and preview
- `download` — Can download the file (default)
- `edit` — Can modify and reshare

**Flags:**
| Flag | Description |
|------|-------------|
| `--permission` | Permission level: `view`, `download`, `edit` (default: `download`) |

#### `unshare` — Revoke a share

```bash
docshare unshare <share-id>
```

Use `docshare info <file>` or the web UI to find share IDs.

#### `shared` — List files shared with you

```bash
docshare shared
```

---

## Path Resolution

Most commands accept remote paths in two formats:

**Human-readable paths** resolve folder names by walking the directory tree:
```bash
docshare ls /Documents/Projects/2025
docshare download /Documents/Projects/2025/report.pdf
```

Path matching is case-insensitive.

**UUIDs** are passed through directly (useful for scripting):
```bash
docshare ls 550e8400-e29b-41d4-a716-446655440000
docshare download 770e8400-e29b-41d4-a716-446655440003
```

---

## Global Flags

These flags are available on every command:

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (useful for scripting and piping) |
| `--server` | Override server URL for this command |
| `-h`, `--help` | Show help for any command |

**JSON output example:**

```bash
docshare ls --json | jq '.[].name'
docshare info /Documents/report.pdf --json | jq '.size'
```

---

## Configuration

The CLI stores its configuration in `~/.config/docshare/config.json`:

```json
{
  "server_url": "http://localhost:8080",
  "token": "dsh_7f8e9d..."
}
```

| Field | Description |
|-------|-------------|
| `server_url` | Base URL of your DocShare server |
| `token` | Authentication token (API token or JWT from device flow) |

The config file is created automatically on `docshare login`. You can also edit it directly.

**Override the server URL per-command:**

```bash
docshare --server https://other-server.com ls
```

---

## Shell Completion

Generate shell completions for tab-completion of commands, subcommands, and flags.

### Zsh

```bash
# If using Oh My Zsh:
docshare completion zsh > ~/.oh-my-zsh/completions/_docshare

# Otherwise, use a directory in your fpath:
docshare completion zsh > /usr/local/share/zsh/site-functions/_docshare
```

Then restart your shell or run `source ~/.zshrc`.

### Bash

```bash
# macOS (Homebrew bash-completion):
docshare completion bash > /usr/local/etc/bash_completion.d/docshare

# Linux:
docshare completion bash > /etc/bash_completion.d/docshare
```

Then restart your shell or run `source ~/.bashrc`.

### Fish

```bash
docshare completion fish > ~/.config/fish/completions/docshare.fish
```

Completions are picked up automatically on the next shell session.

### PowerShell

```powershell
docshare completion powershell > docshare.ps1
. ./docshare.ps1
```

To load completions on every session, add the output to your PowerShell profile (`$PROFILE`).

---

## Examples

### Back up a remote directory locally

```bash
docshare download /Documents ./backup-$(date +%Y%m%d)
```

### Upload an entire project

```bash
docshare mkdir "Project Alpha"
docshare upload ./src /Project\ Alpha -w 8
```

### List all files as JSON and filter with jq

```bash
# Find all PDFs
docshare ls --json | jq '[.[] | select(.mimeType == "application/pdf")]'

# Total size of all files
docshare ls --json | jq '[.[].size] | add' 
```

### Share a folder with a colleague

```bash
docshare share /Documents/Shared alice@example.com --permission download
```

### Scripting with API tokens

```bash
export DOCSHARE_TOKEN="dsh_..."
docshare login --token "$DOCSHARE_TOKEN"
docshare upload ./reports/*.pdf /Monthly\ Reports
```

### Use with a remote server

```bash
docshare --server https://files.example.com login
docshare --server https://files.example.com ls
```

Or set it once:

```bash
docshare login --server https://files.example.com
# Server URL is now saved — no need to repeat it
docshare ls
```

---

## Building from Source

```bash
# Clone
git clone https://github.com/hayward-solutions/docshare.git
cd docshare/cli

# Build
go build -o docshare .

# Build with optimizations (smaller binary)
CGO_ENABLED=0 go build -ldflags="-s -w" -o docshare .

# Run tests
go vet ./...

# Install globally
sudo mv docshare /usr/local/bin/
```

**Cross-compile for other platforms:**

```bash
GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o docshare-linux-amd64 .
GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o docshare-linux-arm64 .
GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o docshare-darwin-amd64 .
GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o docshare-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o docshare-windows-amd64.exe .
```
