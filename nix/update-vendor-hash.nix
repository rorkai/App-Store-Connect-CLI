{
  perSystem =
    { pkgs, lib, ... }:
    {
      apps.update-vendor-hash = {
        type = "app";
        program = lib.getExe (
          pkgs.writeShellApplication {
            name = "update-vendor-hash";
            runtimeInputs = [ pkgs.nix-update ];
            text = ''
              nix-update --flake --version=skip default
            '';
          }
        );
        meta.description = "Refresh vendorHash in package.nix from go.mod and go.sum";
      };
    };
}
