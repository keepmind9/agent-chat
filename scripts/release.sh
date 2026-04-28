#!/bin/bash
set -euo pipefail

VERSION="${1:?Usage: $0 <version> (e.g. v0.1.0)}"
VERSION_NUM="${VERSION#v}"

DIST_DIR="dist"
PKG_NAME="agent-chat-${VERSION_NUM}"
CMD="."
BINARY="agent-chat"

LDFLAGS="-s -w -X github.com/keepmind9/agent-chat/cmd.version=${VERSION_NUM} \
         -X github.com/keepmind9/agent-chat/cmd.gitCommit=$(git rev-parse --short HEAD) \
         -X github.com/keepmind9/agent-chat/cmd.buildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for target in "${TARGETS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$target"

    echo "Building ${PKG_NAME}-${GOOS}-${GOARCH}..."

    ASSET_DIR="${DIST_DIR}/${PKG_NAME}-${GOOS}-${GOARCH}"
    mkdir -p "${ASSET_DIR}"

    OUTPUT="${BINARY}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT="${BINARY}.exe"
    fi

    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -trimpath -ldflags "${LDFLAGS}" \
        -o "${ASSET_DIR}/${OUTPUT}" ${CMD}

    cp README.md LICENSE "${ASSET_DIR}/"

    if [ "$GOOS" = "windows" ]; then
        (cd "${DIST_DIR}" && zip -r "${PKG_NAME}-${GOOS}-${GOARCH}.zip" "$(basename "${ASSET_DIR}")")
    else
        tar -czf "${DIST_DIR}/${PKG_NAME}-${GOOS}-${GOARCH}.tar.gz" -C "${DIST_DIR}" "$(basename "${ASSET_DIR}")"
    fi

    rm -rf "${ASSET_DIR}"
done

echo "Generating checksums..."
(cd "${DIST_DIR}" && sha256sum *.tar.gz *.zip > checksums-sha256.txt)

echo "Release artifacts:"
ls -lh "${DIST_DIR}/"
