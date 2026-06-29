#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X hookgram/internal/version.Version=${VERSION}"
GO_BIN="${GO:-go}"

rm -f web/node_modules/go.mod 2>/dev/null || true
npm --prefix web install
npm --prefix web run build
printf 'module hookgram_node_modules\n\ngo 1.26\n' > web/node_modules/go.mod
touch web/dist/.gitkeep

mkdir -p dist
CGO_ENABLED=0 "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o dist/hookgram ./cmd/server

echo "built dist/hookgram (version=${VERSION})"