{
  lib,
  pkgs,
  stdenv,
  buildGoModule,
  pkg-config,
  libfido2,
  versionCheckHook,
  version ? "dev",
  commit ? "unknown",
  date ? "unknown",
}:
let
  goModVersion =
    let
      lines = lib.splitString "\n" (builtins.readFile ./go.mod);
      goLine = lib.findFirst (lib.hasPrefix "go ") (throw "go.mod has no go directive") lines;
    in
    lib.removePrefix "go " goLine;
  goAttr = "go_" + lib.replaceStrings [ "." ] [ "_" ] (lib.versions.majorMinor goModVersion);
  go =
    pkgs.${goAttr}
      or (throw "nixpkgs has no ${goAttr} for go.mod's Go ${goModVersion}; update flake.lock");
in
assert lib.assertMsg (lib.versionAtLeast go.version goModVersion)
  "nixpkgs provides Go ${go.version}, but go.mod requires ${goModVersion}; update flake.lock";
(buildGoModule.override { inherit go; }) {
  pname = "asc";
  inherit version;

  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./main.go
      ./cmd
      ./docs
      ./internal
    ];
  };

  vendorHash = "sha256-M4VuRGq7mrWxJ47zok5uCfdAucvXVfRO2jpoQM+hppo=";

  subPackages = [ "." ];

  # Matches the release builds: macOS links the Keychain and libfido2 through
  # cgo, while the other platforms ship static binaries.
  env.CGO_ENABLED = if stdenv.hostPlatform.isDarwin then "1" else "0";
  tags = lib.optionals stdenv.hostPlatform.isDarwin [ "fido2" ];
  nativeBuildInputs = lib.optionals stdenv.hostPlatform.isDarwin [ pkg-config ];
  buildInputs = lib.optionals stdenv.hostPlatform.isDarwin [ libfido2 ];

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
    "-X main.commit=${commit}"
    "-X main.date=${date}"
  ];

  # With subPackages = [ "." ] only the root package's tests would run, and
  # those cover the Makefile and release workflows (needing make, python3 and
  # xattr), not the CLI. versionCheckHook checks the built binary instead.
  doCheck = false;

  postInstall = ''
    mv "$out/bin/App-Store-Connect-CLI" "$out/bin/asc"
  '';

  doInstallCheck = true;
  nativeInstallCheckInputs = [
    versionCheckHook
  ];
  versionCheckProgramArg = [ "--version" ];

  meta = {
    description = "Fast, scriptable CLI for the App Store Connect API";
    homepage = "https://github.com/rorkai/App-Store-Connect-CLI";
    license = lib.licenses.mit;
    sourceProvenance = [ lib.sourceTypes.fromSource ];
    mainProgram = "asc";
    platforms = import ./nix/systems.nix;
  };
}
