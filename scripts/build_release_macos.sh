#!/bin/sh
# Build and qualify self-contained macOS releases before signing/notarization.
set -eu
: "${VERSION:?VERSION is required}"
[ "$(uname -s)" = Darwin ] || { echo 'macOS release builds require a macOS host' >&2; exit 1; }
release_dir=${RELEASE_DIR:-release}
mkdir -p "$release_dir"
if [ -z "${FIDO2_DEPS_DIR:-}" ]; then
  FIDO2_DEPS_DIR=$(mktemp -d "${TMPDIR:-/tmp}/asc-release-fido2.XXXXXX")
  trap 'rm -rf -- "$FIDO2_DEPS_DIR"' EXIT
fi
jobs=${GOMAXPROCS:-3}
COMMIT=$(git rev-parse --short HEAD)
DATE=$(git show -s --format=%cI HEAD)
export MACOSX_DEPLOYMENT_TARGET=13.0 PKG_CONFIG_PATH=""
for arch in amd64 arm64; do
  prefix="$FIDO2_DEPS_DIR/$arch"
  python3 scripts/build_fido2_deps.py --arch "$arch" --prefix "$prefix" --jobs "$jobs"
  PKG_CONFIG_LIBDIR="$prefix/lib/pkgconfig" \
    CGO_CFLAGS="-O2 -g -mmacosx-version-min=13.0 -I$prefix/include" \
    CGO_LDFLAGS="-O2 -g -mmacosx-version-min=13.0 -L$prefix/lib" \
    CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" \
    go build -p=1 -tags fido2 -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
      -o "$release_dir/asc_${VERSION}_macOS_${arch}" .
  binary="$release_dir/asc_${VERSION}_macOS_${arch}"
  libraries=$(otool -L "$binary")
  printf '%s\n' "$libraries" | awk 'NR > 1 && $1 !~ /^\/usr\/lib\/|^\/System\/Library\// { print "Unexpected native library: " $1; bad=1 } END { exit bad }'
  deployment=$(vtool -show-build "$binary")
  printf '%s\n' "$deployment" | awk '$1 == "minos" { seen=1; if ($2 != "13.0" && $2 != "13.0.0") bad=1 } END { if (!seen || bad) { print "Expected macOS deployment target 13.0"; exit 1 } }'
  go version -m "$binary" | awk '$1 == "build" && $2 ~ /(^-tags=|,)fido2(,|$)/ { fido=1 } $1 == "build" && $2 == "CGO_ENABLED=1" { cgo=1 } END { if (!fido || !cgo) { print "Missing CGO FIDO2 backend"; exit 1 } }'
done
case "$(uname -m)" in arm64) arch=arm64 ;; *) arch=amd64 ;; esac
prefix="$FIDO2_DEPS_DIR/$arch"
PKG_CONFIG_LIBDIR="$prefix/lib/pkgconfig" \
  CGO_CFLAGS="-O2 -g -mmacosx-version-min=13.0 -I$prefix/include" \
  CGO_LDFLAGS="-O2 -g -mmacosx-version-min=13.0 -L$prefix/lib" \
  CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" ASC_BYPASS_KEYCHAIN=1 \
  go test -tags fido2 -p=1 -parallel="$jobs" ./internal/securitykey -count=1
ASC_BYPASS_KEYCHAIN=1 "$release_dir/asc_${VERSION}_macOS_${arch}" --version
ASC_BYPASS_KEYCHAIN=1 "$release_dir/asc_${VERSION}_macOS_${arch}" docs show third-party-notices >/dev/null
