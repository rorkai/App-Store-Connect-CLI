{
  perSystem =
    { pkgs, lib, ... }:
    {
      apps.update-vendor-hash = {
        type = "app";
        program = lib.getExe (
          pkgs.writeShellApplication {
            name = "update-vendor-hash";
            runtimeInputs = [
              pkgs.git
              pkgs.gnugrep
              pkgs.gnused
            ];
            text = ''
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
              committed=false
              cleanup() {
                status=$?
                trap - EXIT HUP INT TERM
                if [ "$committed" != true ]; then
                  set_hash "$old"
                fi
                rm -f "$log"
                exit "$status"
              }
              trap cleanup EXIT
              trap 'exit 129' HUP
              trap 'exit 130' INT
              trap 'exit 143' TERM

              # A fake hash forces a fresh fetch: with the old hash, Nix would reuse a
              # stale module store path and report success.
              set_hash "${lib.fakeHash}"
              if nix build --no-link "$root#default.goModules" 2>"$log"; then
                echo "Nix accepted the fake vendorHash; refusing to guess the real one" >&2
                exit 1
              fi

              new=$(sed -n 's/^[[:space:]]*got:[[:space:]]*\(sha256-[A-Za-z0-9+/=]*\)$/\1/p' "$log" | tail -n 1)
              if [ -z "$new" ]; then
                cat "$log" >&2
                exit 1
              fi

              set_hash "$new"
              committed=true
              if [ "$new" = "$old" ]; then
                echo "vendorHash is up to date: $new"
              else
                echo "vendorHash updated: $old -> $new"
              fi
            '';
          }
        );
        meta.description = "Refresh vendorHash in package.nix from go.mod and go.sum";
      };
    };
}
