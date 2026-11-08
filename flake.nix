{
  description = "App Store Connect CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs }: {
    packages = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = "dev-${self.shortRev or "dirty"}";
      in {
        default = pkgs.buildGo127Module {
          pname = "asc";
          inherit version;
          src = self;
          vendorHash = "sha256-vrdu2W2gmiUyMZUj21k53goaanmKFIc0Qbr0biohH6E=";
          subPackages = [ "." ];
          env.CGO_ENABLED = "0";
          ldflags = [
            "-X main.version=${version}"
            "-X main.commit=${self.rev or "unknown"}"
          ];
          # The repository runs the Go suite separately from packaging.
          doCheck = false;
          postInstall = ''
            mv "$out/bin/App-Store-Connect-CLI" "$out/bin/asc"
          '';
          meta.mainProgram = "asc";
        };
      });
  };
}
