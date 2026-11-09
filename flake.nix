{
  description = "App Store Connect CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import ./nix/systems.nix;

      imports = [ ./nix/update-vendor-hash.nix ];

      perSystem =
        { config, pkgs, ... }:
        {
          packages = {
            asc = pkgs.callPackage ./package.nix {
              version = "dev-${self.shortRev or "dirty"}";
              commit = self.shortRev or self.dirtyShortRev or "unknown";
              date =
                let
                  d = self.lastModifiedDate;
                  part = start: len: builtins.substring start len d;
                in
                "${part 0 4}-${part 4 2}-${part 6 2}T${part 8 2}:${part 10 2}:${part 12 2}Z";
            };
            default = config.packages.asc;
          };

          formatter = pkgs.nixfmt-tree;

          checks.build = config.packages.asc;

          devShells.default = pkgs.mkShell {
            inputsFrom = [ config.packages.asc ];
            packages = [ pkgs.python3 ];
          };
        };
    };
}
