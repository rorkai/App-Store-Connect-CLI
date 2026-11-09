{
  description = "App Store Connect CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;

      forAllSystems = lib.genAttrs (import ./nix/systems.nix);

      packages = forAllSystems (
        system:
        let
          asc = nixpkgs.legacyPackages.${system}.callPackage ./package.nix {
            version = "dev-${self.shortRev or "dirty"}";
            commit = self.shortRev or self.dirtyShortRev or "unknown";
            date =
              let
                d = self.lastModifiedDate;
                part = start: len: builtins.substring start len d;
              in
              "${part 0 4}-${part 4 2}-${part 6 2}T${part 8 2}:${part 10 2}:${part 12 2}Z";
          };
        in
        {
          inherit asc;
          default = asc;
        }
      );
    in
    {
      inherit packages;

      apps = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          update-vendor-hash = {
            type = "app";
            program = lib.getExe (
              pkgs.writeShellApplication {
                name = "update-vendor-hash";
                runtimeInputs = [
                  pkgs.git
                  pkgs.gnugrep
                  pkgs.gnused
                ];
                text = builtins.readFile ./nix/update-vendor-hash.sh;
              }
            );
            meta.description = "Refresh vendorHash in package.nix from go.mod and go.sum";
          };
        }
      );

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);

      checks = forAllSystems (system: {
        build = packages.${system}.asc;
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            inputsFrom = [ packages.${system}.asc ];
            packages = [ pkgs.python3 ];
          };
        }
      );
    };
}
