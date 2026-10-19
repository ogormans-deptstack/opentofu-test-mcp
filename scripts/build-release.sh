#!/usr/bin/env bash
set -euo pipefail

VERSION=${1:?usage: bash scripts/build-release.sh VERSION}
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "VERSION must be a semantic version without a leading v" >&2
  exit 1
fi
ARCHIVES=()
cleanup() {
  rm -f "$STAGING/opentofu-test-mcp" "$STAGING/opentofu-test-mcp.exe" \
    "$STAGING/README.md" "$STAGING/LICENSE"
  rmdir "$STAGING"
}
mkdir -p dist/release
for OS in darwin linux windows; do
  for ARCH in amd64 arm64; do
    NAME="opentofu-test-mcp_${VERSION}_${OS}_${ARCH}"
    STAGING=$(mktemp -d)
    trap cleanup EXIT
    BINARY=opentofu-test-mcp
    if [ "$OS" = windows ]; then BINARY="$BINARY.exe"; fi
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" -o "$STAGING/$BINARY" .
    cp README.md LICENSE "$STAGING/"
    if [ "$OS" = windows ]; then
      (cd "$STAGING" && zip -q - "$BINARY" README.md LICENSE) > "dist/release/$NAME.zip"
      ARCHIVES+=("$NAME.zip")
    else
      tar -czf "dist/release/$NAME.tar.gz" -C "$STAGING" "$BINARY" README.md LICENSE
      ARCHIVES+=("$NAME.tar.gz")
    fi
    cleanup
    trap - EXIT
  done
done
(cd dist/release && shasum -a 256 "${ARCHIVES[@]}" > checksums.txt)
