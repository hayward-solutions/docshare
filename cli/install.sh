#!/bin/sh
# DocShare CLI installer
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh -s -- --dir /usr/local/bin
#   curl -sSfL https://raw.githubusercontent.com/hayward-solutions/docshare/main/cli/install.sh | sh -s -- --version v1.2.0

set -e

REPO="hayward-solutions/docshare"
BINARY="docshare"
INSTALL_DIR="/usr/local/bin"
VERSION="latest"

# ─── Helpers ───────────────────────────────────────────────

log()   { printf '%s\n' "$1"; }
info()  { printf '\033[0;34m%s\033[0m\n' "$1"; }
ok()    { printf '\033[0;32m%s\033[0m\n' "$1"; }
warn()  { printf '\033[0;33m%s\033[0m\n' "$1" >&2; }
die()   { printf '\033[0;31mError: %s\033[0m\n' "$1" >&2; exit 1; }

need_cmd() {
    if ! command -v "$1" > /dev/null 2>&1; then
        die "Required command not found: $1"
    fi
}

# ─── Parse arguments ───────────────────────────────────────

while [ $# -gt 0 ]; do
    case "$1" in
        --dir)      INSTALL_DIR="$2"; shift 2 ;;
        --version)  VERSION="$2"; shift 2 ;;
        --help|-h)
            log "DocShare CLI installer"
            log ""
            log "Usage:"
            log "  curl -sSfL https://raw.githubusercontent.com/${REPO}/main/cli/install.sh | sh"
            log ""
            log "Options:"
            log "  --dir DIR        Install directory (default: /usr/local/bin)"
            log "  --version VER    Version tag to install (default: latest)"
            log "  --help           Show this help"
            exit 0
            ;;
        *) die "Unknown option: $1" ;;
    esac
done

# ─── Detect platform ──────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)          die "Unsupported operating system: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        armv7l)         echo "armv7" ;;
        *)              die "Unsupported architecture: $(uname -m)" ;;
    esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

info "Detected platform: ${OS}/${ARCH}"

# ─── Resolve version ──────────────────────────────────────

need_cmd curl

resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
            | grep '"tag_name"' \
            | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' ) || true
    fi

    if [ -z "$VERSION" ]; then
        return 1
    fi
    return 0
}

# ─── Install from GitHub Release ──────────────────────────

install_from_release() {
    EXT="tar.gz"
    if [ "$OS" = "windows" ]; then
        EXT="zip"
    fi

    ARCHIVE="${BINARY}_${VERSION#v}_${OS}_${ARCH}.${EXT}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${URL}..."
    if ! curl -sSfL -o "${TMPDIR}/${ARCHIVE}" "$URL" 2>/dev/null; then
        return 1
    fi

    info "Extracting..."
    if [ "$EXT" = "tar.gz" ]; then
        tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
    else
        need_cmd unzip
        unzip -q "${TMPDIR}/${ARCHIVE}" -d "$TMPDIR"
    fi

    install_binary "${TMPDIR}/${BINARY}"
    return 0
}

# ─── Install from source ─────────────────────────────────

install_from_source() {
    need_cmd go
    need_cmd git

    GO_VERSION=$(go version 2>/dev/null | sed -E 's/.*go([0-9]+\.[0-9]+).*/\1/')
    REQUIRED="1.24"
    if [ "$(printf '%s\n' "$REQUIRED" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED" ]; then
        die "Go ${REQUIRED}+ is required (found ${GO_VERSION})"
    fi

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Cloning repository..."
    git clone --depth 1 "https://github.com/${REPO}.git" "${TMPDIR}/docshare" 2>/dev/null

    info "Building from source..."
    (cd "${TMPDIR}/docshare/cli" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMPDIR}/${BINARY}" .)

    install_binary "${TMPDIR}/${BINARY}"
}

# ─── Place the binary ────────────────────────────────────

install_binary() {
    src="$1"

    if [ ! -f "$src" ]; then
        die "Binary not found at ${src}"
    fi

    chmod +x "$src"

    # Check if we can write to the install directory.
    if [ -w "$INSTALL_DIR" ]; then
        mv "$src" "${INSTALL_DIR}/${BINARY}"
    else
        info "Elevated permissions required to install to ${INSTALL_DIR}"
        sudo mv "$src" "${INSTALL_DIR}/${BINARY}"
    fi

    ok "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
}

# ─── Main ─────────────────────────────────────────────────

log ""
info "Installing DocShare CLI..."
log ""

# Try release binary first, fall back to building from source.
if resolve_version; then
    info "Version: ${VERSION}"
    if install_from_release; then
        log ""
        ok "Done! Run 'docshare --help' to get started."
        exit 0
    fi
    warn "Pre-built binary not available for ${OS}/${ARCH}, falling back to source build..."
fi

if ! command -v go > /dev/null 2>&1; then
    warn "No pre-built release found and Go is not installed."
    log ""
    log "Install options:"
    log "  1. Install Go (https://go.dev/dl/) and re-run this script"
    log "  2. Build manually:"
    log "       git clone https://github.com/${REPO}.git"
    log "       cd docshare/cli"
    log "       go build -o docshare ."
    log "       sudo mv docshare /usr/local/bin/"
    exit 1
fi

install_from_source

log ""
ok "Done! Run 'docshare --help' to get started."
