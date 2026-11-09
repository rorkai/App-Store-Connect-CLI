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
            commit = self.rev or "unknown";
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
