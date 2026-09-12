package xcode

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var (
	runGetVersion                    = localxcode.GetVersionScoped
	runGetConsistentMarketingVersion = localxcode.GetConsistentMarketingVersion
	runValidateSetVersion            = localxcode.ValidateSetVersion
	runValidateBumpVersion           = localxcode.ValidateBumpVersion
	runSetVersion                    = localxcode.SetVersion
	runBumpVersion                   = localxcode.BumpVersion
	runResolveXcodeNextBuildNumber   = resolveXcodeNextBuildNumber
)

type xcodeRemoteBuildNumberOptions struct {
	AppID              string
	Version            string
	Platform           string
	ProcessingState    string
	ExcludeExpired     bool
	InitialBuildNumber int
}

type xcodeRemoteBuildNumberFlags struct {
	next               *bool
	appID              *string
	platform           *string
	processingState    *string
	excludeExpired     *bool
	initialBuildNumber *int
}

func bindXcodeRemoteBuildNumberFlags(fs *flag.FlagSet) xcodeRemoteBuildNumberFlags {
	return xcodeRemoteBuildNumberFlags{
		next:               fs.Bool("next-build-number", false, "Set the remote-safe next build number from App Store Connect"),
		appID:              fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID with --next-build-number)"),
		platform:           fs.String("platform", "", "Optional remote build platform filter: IOS, MAC_OS, TV_OS, VISION_OS"),
		processingState:    fs.String("processing-state", "", "Optional remote processing-state filter"),
		excludeExpired:     fs.Bool("exclude-expired", false, "Exclude expired builds from remote selection"),
		initialBuildNumber: fs.Int("initial-build-number", 1, "Initial build number when no remote build exists"),
	}
}

func (flags xcodeRemoteBuildNumberFlags) options(version string) xcodeRemoteBuildNumberOptions {
	return xcodeRemoteBuildNumberOptions{
		AppID:              strings.TrimSpace(*flags.appID),
		Version:            strings.TrimSpace(version),
		Platform:           strings.TrimSpace(*flags.platform),
		ProcessingState:    strings.TrimSpace(*flags.processingState),
		ExcludeExpired:     *flags.excludeExpired,
		InitialBuildNumber: *flags.initialBuildNumber,
	}
}

func (flags xcodeRemoteBuildNumberFlags) hasRemoteOnlyValues() bool {
	return strings.TrimSpace(*flags.appID) != "" || strings.TrimSpace(*flags.platform) != "" ||
		strings.TrimSpace(*flags.processingState) != "" || *flags.excludeExpired || *flags.initialBuildNumber != 1
}

// XcodeVersionCommand returns the xcode version command group.
func XcodeVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("version", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "asc xcode version <subcommand> [flags]",
		ShortHelp:  "Read and modify Xcode project version numbers.",
		LongHelp: `Read and modify Xcode project version and build numbers.

Modern project and xcconfig build settings are edited structurally without
requiring Xcode. Legacy Info.plist-only projects use agvtool on macOS.

Examples:
  asc xcode version view
  asc xcode version view --project-dir ./MyApp
  asc xcode version view --project ./MyApp/App.xcodeproj
  asc xcode version view --target MyApp --configuration Release
  asc xcode version edit --version "1.3.0"
  asc xcode version edit --build-number "42"
  asc xcode version edit --version "1.3.0" --build-number "42"
  asc xcode version edit --target Widget --configuration Release --build-number "42"
  asc xcode version edit --next-build-number --app "com.example.app"
  asc xcode version bump --type patch
  asc xcode version bump --type minor
  asc xcode version bump --type major
  asc xcode version bump --type build
  asc xcode version bump --type build --next-build-number --app "com.example.app"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			xcodeVersionViewCommand(),
			xcodeVersionEditCommand(),
			xcodeVersionBumpCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func selectedProjectInput(projectDir, project string) string {
	if project != "" {
		return project
	}
	if projectDir == "" {
		return "."
	}
	return projectDir
}

func parseXcodebuildSettingsLookup(value string) (localxcode.BuildSettingsLookupPolicy, error) {
	if strings.TrimSpace(value) == "" {
		return "", shared.UsageError("--xcodebuild-settings-lookup must be one of: auto, never")
	}
	policy, err := localxcode.ParseBuildSettingsLookupPolicy(value)
	if err != nil {
		return "", shared.UsageError(err.Error())
	}
	return policy, nil
}

func bindXcodebuildSettingsLookup(fs *flag.FlagSet) *string {
	return fs.String(
		"xcodebuild-settings-lookup",
		"auto",
		"xcodebuild -showBuildSettings fallback policy: auto or never",
	)
}

func xcodeVersionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	target := fs.String("target", "", "Xcode target name (for multi-target projects)")
	configuration := fs.String("configuration", "", "Xcode build configuration name")
	xcodebuildSettingsLookup := bindXcodebuildSettingsLookup(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc xcode version view [--project XCODEPROJ] [--project-dir DIR] [--target NAME] [--configuration NAME] [--xcodebuild-settings-lookup auto|never]",
		ShortHelp:  "View current version and build number.",
		LongHelp: `View the current marketing version and build number.

Structured project and xcconfig settings are read without launching Xcode. If
they cannot resolve MARKETING_VERSION and CURRENT_PROJECT_VERSION, the default
auto policy warns on stderr before running xcodebuild -showBuildSettings. Use
--xcodebuild-settings-lookup never to fail without launching xcodebuild.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			lookupPolicy, err := parseXcodebuildSettingsLookup(*xcodebuildSettingsLookup)
			if err != nil {
				return err
			}
			lookupSession := localxcode.NewBuildSettingsLookupSession(lookupPolicy, os.Stderr)

			result, err := runGetVersion(ctx, localxcode.GetVersionOptions{
				ProjectDir:              selectedProjectInput(*projectDir, *project),
				Target:                  strings.TrimSpace(*target),
				Configuration:           strings.TrimSpace(*configuration),
				BuildSettingsLookup:     lookupPolicy,
				BuildSettingsDiagnostic: os.Stderr,
				BuildSettingsSession:    lookupSession,
			})
			if err != nil {
				return fmt.Errorf("xcode version view: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					fmt.Printf("Version: %s\n", result.Version)
					fmt.Printf("Build:   %s\n", result.BuildNumber)
					printVersionScopeAndSources(result, "")
					return nil
				},
				func() error {
					fmt.Printf("**Version:** %s\n\n**Build:** %s\n", result.Version, result.BuildNumber)
					printVersionScopeAndSources(result, "**")
					return nil
				},
			)
		},
	}
}

func printVersionScopeAndSources(result *localxcode.VersionInfo, markdownMarker string) {
	printValue := func(label, value string) {
		if value == "" {
			return
		}
		if markdownMarker != "" {
			fmt.Printf("\n**%s:** %s\n", label, value)
			return
		}
		fmt.Printf("%-15s %s\n", label+":", value)
	}
	printValue("Target", result.Target)
	printValue("Configuration", result.Configuration)
	printValue("Version source", result.VersionSource)
	printValue("Build source", result.BuildNumberSource)
}

func xcodeVersionEditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	target := fs.String("target", "", "Xcode target name to edit")
	configuration := fs.String("configuration", "", "Xcode build configuration name to edit")
	version := fs.String("version", "", "Marketing version (CFBundleShortVersionString)")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion)")
	allowExternalXCConfig := fs.Bool("allow-external-xcconfig", false, "Allow rewriting xcconfig files referenced outside the project directory")
	xcodebuildSettingsLookup := bindXcodebuildSettingsLookup(fs)
	remote := bindXcodeRemoteBuildNumberFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc xcode version edit [--version VER] [--build-number NUM | --next-build-number --app APP] [--target NAME] [--configuration NAME] [--xcodebuild-settings-lookup auto|never]",
		ShortHelp:  "Edit version and/or build number.",
		LongHelp: `Edit the marketing version and/or build number in an Xcode project.

Modern project and xcconfig build settings are edited structurally.

By default only xcconfig files inside the project directory are rewritten.
Pass the --allow-external-xcconfig flag to authorize rewriting an
xcconfig the project references outside that directory.

The --xcodebuild-settings-lookup flag controls the hidden
xcodebuild fallback when the command must read unresolved version settings.
The default auto policy warns on stderr before running xcodebuild. Use never to
fail without launching xcodebuild.

Examples:
  asc xcode version edit --version "1.3.0"
  asc xcode version edit --build-number "42"
  asc xcode version edit --target Widget --configuration Release --build-number "42"
  asc xcode version edit --next-build-number --app "com.example.app"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			lookupPolicy, err := parseXcodebuildSettingsLookup(*xcodebuildSettingsLookup)
			if err != nil {
				return err
			}
			lookupSession := localxcode.NewBuildSettingsLookupSession(lookupPolicy, os.Stderr)

			v := strings.TrimSpace(*version)
			b := strings.TrimSpace(*buildNumber)
			if b != "" && *remote.next {
				return shared.UsageError("--build-number and --next-build-number are mutually exclusive")
			}
			if !*remote.next && remote.hasRemoteOnlyValues() {
				return shared.UsageError("remote build-number flags require --next-build-number")
			}
			if v == "" && b == "" && !*remote.next {
				return shared.UsageError("--version or --build-number is required")
			}

			projectInput := selectedProjectInput(*projectDir, *project)
			if *allowExternalXCConfig {
				warnExternalXCConfigAuthorized()
			}
			if *remote.next {
				if err := validateXcodeRemoteBuildNumberOptions(remote.options(v)); err != nil {
					return err
				}
				if err := runValidateSetVersion(ctx, localxcode.SetVersionOptions{
					ProjectDir:            projectInput,
					Target:                strings.TrimSpace(*target),
					Configuration:         strings.TrimSpace(*configuration),
					Version:               v,
					BuildNumber:           "1",
					AllowExternalXCConfig: *allowExternalXCConfig,
				}); err != nil {
					return fmt.Errorf("xcode version edit: validate local mutation: %w", err)
				}
				versionFilter := v
				if versionFilter == "" {
					resolvedVersion, err := runGetConsistentMarketingVersion(ctx, localxcode.GetVersionOptions{
						ProjectDir:              projectInput,
						Target:                  strings.TrimSpace(*target),
						Configuration:           strings.TrimSpace(*configuration),
						BuildSettingsLookup:     lookupPolicy,
						BuildSettingsDiagnostic: os.Stderr,
						BuildSettingsSession:    lookupSession,
					})
					if err != nil {
						return fmt.Errorf("xcode version edit: resolve local version for remote build selection: %w", err)
					}
					versionFilter = resolvedVersion
				}
				var err error
				b, err = runResolveXcodeNextBuildNumber(ctx, remote.options(versionFilter))
				if err != nil {
					return fmt.Errorf("xcode version edit: resolve next build number: %w", err)
				}
			}

			result, err := runSetVersion(ctx, localxcode.SetVersionOptions{
				ProjectDir:            projectInput,
				Target:                strings.TrimSpace(*target),
				Configuration:         strings.TrimSpace(*configuration),
				Version:               v,
				BuildNumber:           b,
				AllowExternalXCConfig: *allowExternalXCConfig,
			})
			if err != nil {
				return fmt.Errorf("xcode version edit: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func xcodeVersionBumpCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bump", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	target := fs.String("target", "", "Xcode target name to bump")
	configuration := fs.String("configuration", "", "Xcode build configuration name to bump")
	bumpType := fs.String("type", "", "Bump type: major, minor, patch, or build (required)")
	allowExternalXCConfig := fs.Bool("allow-external-xcconfig", false, "Allow rewriting xcconfig files referenced outside the project directory")
	xcodebuildSettingsLookup := bindXcodebuildSettingsLookup(fs)
	remote := bindXcodeRemoteBuildNumberFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "bump",
		ShortUsage: "asc xcode version bump --type TYPE [--target NAME] [--configuration NAME] [--next-build-number --app APP] [--xcodebuild-settings-lookup auto|never]",
		ShortHelp:  "Increment version or build number.",
		LongHelp: `Increment the version or build number in an Xcode project.

Bump types:
  major   1.2.3 → 2.0.0
  minor   1.2.3 → 1.3.0
  patch   1.2.3 → 1.2.4
  build   Increment CFBundleVersion (build number)

Note:
  --project selects a specific .xcodeproj when the containing directory has
  multiple sibling projects.
  --target and --configuration scope both the read and write.
  --next-build-number is accepted with --type build and uses App Store Connect.
  Only xcconfig files inside the project directory are rewritten unless the
  --allow-external-xcconfig flag is passed.
  The --xcodebuild-settings-lookup flag controls the hidden
  xcodebuild fallback for unresolved version settings. The default auto policy
  warns on stderr before running xcodebuild; never fails without launching it.

Examples:
  asc xcode version bump --type patch
  asc xcode version bump --type patch --project ./MyApp/App.xcodeproj
  asc xcode version bump --type patch --target Extension
  asc xcode version bump --type minor --project-dir ./MyApp
  asc xcode version bump --type build`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			lookupPolicy, err := parseXcodebuildSettingsLookup(*xcodebuildSettingsLookup)
			if err != nil {
				return err
			}
			lookupSession := localxcode.NewBuildSettingsLookupSession(lookupPolicy, os.Stderr)

			parsed, err := localxcode.ParseBumpType(*bumpType)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			if *remote.next && parsed != localxcode.BumpBuild {
				return shared.UsageError("--next-build-number requires --type build")
			}
			if !*remote.next && remote.hasRemoteOnlyValues() {
				return shared.UsageError("remote build-number flags require --next-build-number")
			}
			projectInput := selectedProjectInput(*projectDir, *project)
			if *allowExternalXCConfig {
				warnExternalXCConfigAuthorized()
			}
			remoteBuildNumber := ""
			if *remote.next {
				if err := validateXcodeRemoteBuildNumberOptions(remote.options("")); err != nil {
					return err
				}
				if err := runValidateBumpVersion(ctx, localxcode.BumpVersionOptions{
					ProjectDir:              projectInput,
					Target:                  strings.TrimSpace(*target),
					Configuration:           strings.TrimSpace(*configuration),
					BumpType:                localxcode.BumpBuild,
					BuildNumber:             "1",
					BuildSettingsLookup:     lookupPolicy,
					BuildSettingsDiagnostic: os.Stderr,
					BuildSettingsSession:    lookupSession,
					AllowExternalXCConfig:   *allowExternalXCConfig,
				}); err != nil {
					return fmt.Errorf("xcode version bump: validate local mutation: %w", err)
				}
				versionFilter, err := runGetConsistentMarketingVersion(ctx, localxcode.GetVersionOptions{
					ProjectDir:              projectInput,
					Target:                  strings.TrimSpace(*target),
					Configuration:           strings.TrimSpace(*configuration),
					BuildSettingsLookup:     lookupPolicy,
					BuildSettingsDiagnostic: os.Stderr,
					BuildSettingsSession:    lookupSession,
				})
				if err != nil {
					return fmt.Errorf("xcode version bump: resolve local version for remote build selection: %w", err)
				}
				remoteBuildNumber, err = runResolveXcodeNextBuildNumber(ctx, remote.options(versionFilter))
				if err != nil {
					return fmt.Errorf("xcode version bump: resolve next build number: %w", err)
				}
			}

			result, err := runBumpVersion(ctx, localxcode.BumpVersionOptions{
				ProjectDir:              projectInput,
				Target:                  strings.TrimSpace(*target),
				Configuration:           strings.TrimSpace(*configuration),
				BumpType:                parsed,
				BuildNumber:             remoteBuildNumber,
				BuildSettingsLookup:     lookupPolicy,
				BuildSettingsDiagnostic: os.Stderr,
				BuildSettingsSession:    lookupSession,
				AllowExternalXCConfig:   *allowExternalXCConfig,
			})
			if err != nil {
				return fmt.Errorf("xcode version bump: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func warnExternalXCConfigAuthorized() {
	fmt.Fprintln(os.Stderr, "Warning: --allow-external-xcconfig permits rewriting xcconfig files outside the project directory.")
}

func resolveXcodeNextBuildNumber(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
	if err := validateXcodeRemoteBuildNumberOptions(opts); err != nil {
		return "", err
	}
	selection, err := shared.NormalizeLatestBuildSelectionOptions(
		opts.AppID,
		opts.Version,
		opts.Platform,
		opts.ProcessingState,
		opts.ExcludeExpired,
	)
	if err != nil {
		return "", err
	}
	client, err := shared.GetASCClient()
	if err != nil {
		return "", err
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	result, err := shared.ResolveNextBuildNumber(requestCtx, client, shared.NextBuildNumberOptions{
		LatestBuildSelectionOptions: selection,
		InitialBuildNumber:          opts.InitialBuildNumber,
	})
	if err != nil {
		return "", err
	}
	return result.NextBuildNumber, nil
}

func validateXcodeRemoteBuildNumberOptions(opts xcodeRemoteBuildNumberOptions) error {
	if _, err := shared.NormalizeLatestBuildSelectionOptions(
		opts.AppID,
		opts.Version,
		opts.Platform,
		opts.ProcessingState,
		opts.ExcludeExpired,
	); err != nil {
		return err
	}
	if opts.InitialBuildNumber < 1 {
		return shared.UsageError("--initial-build-number must be >= 1")
	}
	return nil
}
