#!/bin/bash
set -euo pipefail

REPO="keepmind9/agent-chat"
BINARY="agent-chat"
INSTALL_DIR="${HOME}/.local/bin"

# --- Detect OS and arch ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
    echo "Unsupported OS: $OS" >&2; exit 1
fi

# --- Check existing install ---
if command -v "$BINARY" &>/dev/null; then
    CURRENT=$("$BINARY" version 2>/dev/null | grep "^Version:" | awk '{print $2}' || echo "")
fi

# --- Fetch latest release ---
echo "Fetching latest release from ${REPO}..."
RELEASE=$(curl -sf "https://api.github.com/repos/${REPO}/releases/latest") || {
    echo "Failed to fetch release info" >&2; exit 1;
}

LATEST=$(echo "$RELEASE" | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')

if [ -n "${CURRENT:-}" ] && [ "$CURRENT" = "$LATEST" ]; then
    echo "${BINARY} is already up to date (v${CURRENT})"
    exit 0
fi

# --- Find matching asset ---
PATTERN="${BINARY}-${LATEST}-${OS}-${ARCH}"
ASSET_URL=$(echo "$RELEASE" | grep -E '"browser_download_url".*'${PATTERN}'' | head -1 | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')

if [ -z "$ASSET_URL" ]; then
    echo "No asset found for ${OS}-${ARCH}" >&2; exit 1
fi

# --- Download and extract ---
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${PATTERN}..."
ARCHIVE=$(basename "$ASSET_URL")
curl -sfL "$ASSET_URL" -o "${TMPDIR}/${ARCHIVE}"

echo "Extracting..."
if [[ "$ARCHIVE" == *.tar.gz ]]; then
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}" --strip-components=1
else
    unzip -o -q "${TMPDIR}/${ARCHIVE}" -d "${TMPDIR}/${PATTERN}"
    mv "${TMPDIR}/${PATTERN}/${BINARY}" "${TMPDIR}/${BINARY}" 2>/dev/null || true
fi

# --- Install ---
mkdir -p "$INSTALL_DIR"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} v${LATEST} to ${INSTALL_DIR}/${BINARY}"

# --- Update PATH ---
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    SHELL_RC="${HOME}/.bashrc"
    if [ -n "${ZSH_VERSION:-}" ] || [ "${SHELL##*/}" = "zsh" ]; then
        SHELL_RC="${HOME}/.zshrc"
    fi

    echo "" >> "$SHELL_RC"
    echo 'export PATH="'${INSTALL_DIR}':$PATH"' >> "$SHELL_RC"
    echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}"
    echo "Run: source ${SHELL_RC}  (or open a new terminal)"
fi

echo "Done! Run: ${BINARY} version"
