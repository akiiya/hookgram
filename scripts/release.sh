#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')}"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X hookgram/internal/version.Version=${VERSION}"
GO_BIN="${GO:-go}"
EXTRA_FILES=(README.md RELEASE.md)
[ -f LICENSE ] && EXTRA_FILES+=(LICENSE)

rm -f web/node_modules/go.mod 2>/dev/null || true
npm --prefix web install
npm --prefix web run build
printf 'module hookgram_node_modules\n\ngo 1.26\n' > web/node_modules/go.mod
touch web/dist/.gitkeep

"${GO_BIN}" vet ./...
"${GO_BIN}" test ./...

rm -rf dist
mkdir -p dist

package() {
  local goos="$1"
  local goarch="$2"
  local goarm="$3"
  local asset_arch="$4"
  local ext="$5"
  local archive="$6"
  local bin="hookgram${ext}"
  local stage="dist/stage_${goos}_${asset_arch}"
  local asset="hookgram_${VERSION}_${goos}_${asset_arch}"

  rm -rf "${stage}"
  mkdir -p "${stage}"

  if [ -n "${goarm}" ]; then
    CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" GOARM="${goarm}" \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${stage}/${bin}" ./cmd/server
  else
    CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${stage}/${bin}" ./cmd/server
  fi

  for f in "${EXTRA_FILES[@]}"; do
    if [ -f "${f}" ]; then
      mkdir -p "${stage}/$(dirname "${f}")"
      cp "${f}" "${stage}/${f}"
    fi
  done

  case "${archive}" in
    tar.gz)
      tar -C "${stage}" -czf "dist/${asset}.tar.gz" .
      ;;
    zip)
      command -v zip >/dev/null || { echo "zip command not found"; exit 1; }
      (cd "${stage}" && zip -qr "../${asset}.zip" .)
      ;;
    *)
      echo "unsupported archive: ${archive}"
      exit 1
      ;;
  esac

  rm -rf "${stage}"
}

package linux amd64 "" amd64 "" tar.gz
package linux arm64 "" arm64 "" tar.gz
package linux 386 "" 386 "" tar.gz
package linux arm 7 armv7 "" tar.gz
package windows amd64 "" amd64 .exe zip
package windows arm64 "" arm64 .exe zip

(
  cd dist
  if command -v sha256sum >/dev/null; then
    sha256sum hookgram_* > SHA256SUMS
  else
    shasum -a 256 hookgram_* > SHA256SUMS
  fi
)

echo "done ${VERSION}"
ls -1 dist