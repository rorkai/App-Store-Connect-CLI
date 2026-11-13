package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/iosbuild"
)

// BuildsCompileCommand compiles a prepared package using installed xtool.
func BuildsCompileCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds compile", flag.ExitOnError)
	packagePath := fs.String("package-path", "", "Prepared xtool Swift package directory (required)")
	product := fs.String("product", "", "Exact app product name under xtool/ (required)")
	appPath := fs.String("app-path", "", "New .app output path (required; never overwritten)")
	platform := fs.String("platform", "device", "Target: device or simulator")
	configuration := fs.String("configuration", "debug", "Configuration: debug or release")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "compile", ShortUsage: "asc builds compile --package-path PATH --product NAME --app-path PATH [flags]", ShortHelp: "Compile a prepared xtool package for an iOS device or simulator.", LongHelp: `Compile a prepared Swift package with the installed xtool toolchain.

Both child output streams go to stderr; the artifact receipt goes to stdout.
Device signing is controlled by the toolchain and reported as unknown;
xtool can apply an ad hoc signature for configured entitlements. Simulator
metadata is corrected and signed ad hoc with rcodesign, but running the
simulator still requires a Mac runtime host.
xtool may rebuild an outdated installed SDK before compiling. Pin a compatible
xtool/SDK pair to avoid automatic SDK updates.
Existing output directories are rejected. A failed output publication can leave
an incomplete destination; inspect it before retrying.

Example:
  asc builds compile --package-path ./AppPackage --product App --app-path ./artifacts/App.app --platform device --configuration release`, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		if *platform != "device" && *platform != "simulator" {
			return shared.UsageError("--platform must be device or simulator")
		}
		if *configuration != "debug" && *configuration != "release" {
			return shared.UsageError("--configuration must be debug or release")
		}
		if strings.TrimSpace(*packagePath) == "" || strings.TrimSpace(*product) == "" || strings.TrimSpace(*appPath) == "" {
			return shared.UsageError("--package-path, --product, and --app-path are required")
		}
		if filepath.Base(*product) != *product || *product == "." || strings.ContainsAny(*product, `/\`) {
			return shared.UsageError("--product must be an app product name, not a path")
		}
		if filepath.Ext(*appPath) != ".app" {
			return shared.UsageError("--app-path must end with .app")
		}
		if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
			return shared.UsageError(err.Error())
		}
		result, err := iosbuild.Build(ctx, iosbuild.BuildOptions{PackagePath: *packagePath, Product: *product, AppPath: *appPath, Platform: *platform, Configuration: *configuration, LogWriter: os.Stderr})
		if err != nil {
			return fmt.Errorf("builds compile: %w", err)
		}
		return shared.PrintOutput(result, *output.Output, *output.Pretty)
	}}
}

// BuildsPackageCommand signs and packages a device app using installed rcodesign.
func BuildsPackageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds package", flag.ExitOnError)
	appPath := fs.String("app-path", "", "Fresh device .app input path (required)")
	ipaPath := fs.String("ipa-path", "", "New .ipa output path (required; never overwritten)")
	adHoc := fs.Bool("ad-hoc", false, "Local test signature without Apple identity; Apple will not accept this IPA")
	identity := fs.String("identity", "", "Local PKCS#12 identity; requires --provisioning-profile")
	password := fs.String("identity-password-file", "", "Private file containing the PKCS#12 password")
	profile := fs.String("provisioning-profile", "", "Apple-signed iOS provisioning profile matching the app and identity")
	entitlements := fs.String("entitlements", "", "Requested signing entitlements plist; omitted means no optional capabilities")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "package", ShortUsage: "asc builds package --app-path PATH --ipa-path PATH (--ad-hoc | --identity PATH --provisioning-profile PATH) [flags]", ShortHelp: "Sign one device app with rcodesign and package an IPA locally.", LongHelp: `Package one freshly built thin arm64 iOS app, using explicit local signing inputs.

Real signing validates Apple CMS profile trust, expiry, the exact bundle ID,
identity/profile certificate binding, and requested entitlement authorization.
Nested executable code, frameworks, extensions, watch apps, App Clips, dylibs,
and app symlinks are currently unsupported. This is not an existing-IPA resign
workflow; optional capabilities must be supplied explicitly with --entitlements.

--ad-hoc selects test signing without a profile or Apple credentials. Such an
IPA is not accepted by Apple and is not installable on an ordinary iPhone.
rcodesign completion is not complete Apple code-signature verification; the
receipt reports signatureVerified=false and appleAcceptance=notVerified.
No Apple requests, credential discovery, provisioning, or upload are performed.
After real signing, asc builds upload --wait is a separate Apple verification step.

Examples:
  asc builds package --app-path ./artifacts/App.app --ipa-path ./artifacts/App.ipa --ad-hoc
  asc builds package --app-path ./artifacts/App.app --ipa-path ./artifacts/App.ipa --identity ./signing/App.p12 --identity-password-file ./secrets/password --provisioning-profile ./signing/App.mobileprovision --entitlements ./signing/entitlements.plist`, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		if *adHoc && (*identity != "" || *profile != "" || *password != "") {
			return shared.UsageError("--ad-hoc cannot be combined with identity, profile, or password")
		}
		if !*adHoc && (strings.TrimSpace(*identity) == "" || strings.TrimSpace(*profile) == "") {
			return shared.UsageError("select --ad-hoc or both --identity and --provisioning-profile")
		}
		if strings.TrimSpace(*appPath) == "" || strings.TrimSpace(*ipaPath) == "" {
			return shared.UsageError("--app-path and --ipa-path are required")
		}
		if filepath.Ext(*appPath) != ".app" || filepath.Ext(*ipaPath) != ".ipa" {
			return shared.UsageError("--app-path must end with .app and --ipa-path must end with .ipa")
		}
		if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
			return shared.UsageError(err.Error())
		}
		result, err := signing.PackageIOSApp(ctx, signing.IOSPackageOptions{AppPath: *appPath, IPAPath: *ipaPath, AdHoc: *adHoc, IdentityPath: *identity, PasswordPath: *password, ProfilePath: *profile, EntitlementsPath: *entitlements, LogWriter: os.Stderr})
		if err != nil {
			return fmt.Errorf("builds package: %w", err)
		}
		return shared.PrintOutput(result, *output.Output, *output.Pretty)
	}}
}
