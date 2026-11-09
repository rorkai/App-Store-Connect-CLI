#!/bin/sh
# Recompute vendorHash in package.nix from go.mod and go.sum.
set -eu

root=$(git rev-parse --show-toplevel)
file="$root/package.nix"
line='^  vendorHash = "sha256-[A-Za-z0-9+/=]*";$'
grep -q "$line" "$file" || { echo "package.nix: vendorHash line not found" >&2; exit 1; }

set_hash() {
  sed -i.bak "s|^  vendorHash = \".*\";\$|  vendorHash = \"$1\";|" "$file"
  rm -f "$file.bak"
}

old=$(sed -n 's/^  vendorHash = "\(.*\)";$/\1/p' "$file")
log=$(mktemp)
trap 'rm -f "$log"' EXIT

# A fake hash forces a fresh fetch: with the old hash, Nix would reuse a
# stale module store path and report success.
set_hash "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
if nix build --no-link "$root#default.goModules" 2>"$log"; then
  set_hash "$old"
  echo "Nix accepted the fake vendorHash; refusing to guess the real one" >&2
  exit 1
fi

new=$(sed -n 's/^[[:space:]]*got:[[:space:]]*\(sha256-[A-Za-z0-9+/=]*\)$/\1/p' "$log" | tail -n 1)
if [ -z "$new" ]; then
  set_hash "$old"
  cat "$log" >&2
  exit 1
fi

set_hash "$new"
if [ "$new" = "$old" ]; then
  echo "vendorHash is up to date: $new"
else
  echo "vendorHash updated: $old -> $new"
fi
