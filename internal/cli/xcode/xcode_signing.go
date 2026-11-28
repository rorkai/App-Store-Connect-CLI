package xcode

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var (
	runBuildSigningPlan      = localxcode.BuildSigningPlan
	writeSigningPlanArtifact = localxcode.WriteSigningPlanArtifact
	runApplySigningPlan      = localxcode.ApplySigningPlan
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("value must not be empty")
	}
	*f = append(*f, trimmed)
	return nil
}

func normalizeSigningExportMethod(value string) (string, error) {
	method := strings.ToLower(strings.TrimSpace(value))
	switch method {
	case "", "app-store", "ad-hoc", "development", "developer-id", "enterprise":
		return method, nil
	default:
		return "", fmt.Errorf("--export-method must be app-store, ad-hoc, development, developer-id, or enterprise")
	}
}

// newXcodeSigningCommand returns the local Xcode signing-settings
// plan/apply command group. It edits only project build settings; credentials,
// profiles, and certificates remain owned by the asc signing commands.
func newXcodeSigningCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("signing", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "signing",
		ShortUsage: "asc xcode signing <subcommand> [flags]",
		ShortHelp:  "Plan and apply deterministic Xcode signing settings.",
		LongHelp: `Plan and apply deterministic Xcode signing settings.

The plan command reads a strict JSON settings manifest, resolves target and
configuration precedence, and writes a mode-0600 plan artifact. The apply
command verifies that artifact and all source-file digests before making a
rooted atomic local project update. Neither command contacts App Store Connect
or imports credentials.

Examples:
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json --state-dir .asc/xcode/signing --output json
  asc xcode signing apply --plan .asc/xcode/signing/plan.json --confirm
`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			xcodeSigningPlanCommand(d),
			xcodeSigningApplyCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func xcodeSigningPlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("plan", flag.ExitOnError)
	project := fs.String("project", "", "Path to the .xcodeproj to plan (required)")
	settingsFile := fs.String("settings-file", "", "Strict JSON signing settings manifest; required unless --profile is set")
	var profiles stringListFlag
	fs.Var(&profiles, "profile", "Provisioning profile to infer signing settings from (repeatable .mobileprovision or .provisionprofile)")
	configuration := fs.String("configuration", "", "Limit inference to one build configuration")
	exportMethod := fs.String("export-method", "", "Export method: app-store, ad-hoc, development, developer-id, or enterprise")
	exportOptionsOut := fs.String("export-options-out", "", "Write ExportOptions.plist for the inferred profiles")
	var skipTargets stringListFlag
	fs.Var(&skipTargets, "skip-target", "Target that may remain unmatched without blocking the plan (repeatable)")
	stateDir := fs.String("state-dir", ".asc/xcode/signing", "Directory for plan and receipt artifacts")
	allowExternalXCConfig := fs.Bool("allow-external-xcconfig", false, "Allow updating xcconfig files outside the project directory")
	overwrite := fs.Bool("overwrite", false, "Replace an existing plan artifact and --export-options-out file")
	output := d.BindOutputFlags(fs)
	shared.DeclareCommandRuntimeRequirements(fs, shared.CommandRuntimeRequirements{Host: "portable", ToolCoverage: "none"})

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc xcode signing plan --project PATH (--settings-file PATH | --profile PATH) [flags]",
		ShortHelp:  "Resolve signing settings and write a plan.",
		LongHelp: `Resolve signing settings and write a plan.

The settings file must contain schemaVersion 1 and explicit target,
configuration, and allowlisted signing-setting values. Pass one or more
--profile files to infer CODE_SIGN_STYLE, DEVELOPMENT_TEAM, CODE_SIGN_IDENTITY,
and PROVISIONING_PROFILE_SPECIFIER from each target's bundle identifier.
Only application and extension targets are inferred, expired profiles are
never selected, and a macOS target matches only macOS profiles.
--settings-file overrides inferred values. --configuration, --export-method,
--export-options-out, and --skip-target require --profile.

A representable blocked plan is written with ready=false and explains the
blocker without changing the project. Any unauthorized external xcconfig
prevents artifact publication because its contents cannot be safely
inventoried; pass --allow-external-xcconfig to authorize reading it.

Examples:
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json
  asc xcode signing plan --project ./App.xcodeproj --profile ./signing/App.mobileprovision --configuration Release
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json --overwrite --output markdown`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*project) == "" {
				return d.RequiredFlag("--project")
			}
			if strings.TrimSpace(*settingsFile) == "" && len(profiles) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --settings-file or --profile is required")
				return shared.MissingRequiredUsageError("--settings-file")
			}
			if len(profiles) == 0 {
				// These flags only shape profile inference; accepting them
				// with a settings file alone would silently ignore them.
				for _, item := range []struct {
					name  string
					value string
				}{
					{"--configuration", *configuration},
					{"--export-method", *exportMethod},
					{"--export-options-out", *exportOptionsOut},
					{"--skip-target", strings.Join(skipTargets, ",")},
				} {
					if strings.TrimSpace(item.value) != "" {
						return d.UsageErrorf("%s requires --profile", item.name)
					}
				}
			}
			method, err := normalizeSigningExportMethod(*exportMethod)
			if err != nil {
				return d.UsageError(err.Error())
			}
			// An explicitly empty value must not fall back to the flag default;
			// silently relocating the plan and receipt would hide where the
			// artifacts were written.
			if strings.TrimSpace(*stateDir) == "" {
				return d.UsageErrorf("--state-dir must not be empty")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			exportOptionsPath := osPath(d, strings.TrimSpace(*exportOptionsOut))
			if exportOptionsPath != "" {
				// Reject an unwritable or existing destination before the plan
				// artifact is published, so a failure leaves nothing behind.
				if err := localxcode.CheckSigningExportOptionsDestination(exportOptionsPath, *overwrite); err != nil {
					return fmt.Errorf("xcode signing plan: %w", err)
				}
			}
			plan, err := runBuildSigningPlan(localxcode.SigningPlanOptions{
				ProjectPath:           osPath(d, strings.TrimSpace(*project)),
				SettingsFilePath:      osPath(d, strings.TrimSpace(*settingsFile)),
				ProfilePaths:          osPaths(d, profiles),
				Configuration:         strings.TrimSpace(*configuration),
				ExportMethod:          method,
				SkipTargets:           []string(skipTargets),
				StateDir:              osPath(d, strings.TrimSpace(*stateDir)),
				AllowExternalXCConfig: *allowExternalXCConfig,
			})
			if err != nil {
				if localxcode.IsSigningInputError(err) {
					return xcodeSigningInputUsageError(d, "xcode signing plan", err)
				}
				return fmt.Errorf("xcode signing plan: %w", err)
			}
			if exportOptionsPath != "" {
				if err := localxcode.CheckSigningExportOptionsAliases(exportOptionsPath, plan); err != nil {
					return fmt.Errorf("xcode signing plan: %w", err)
				}
			}
			if err := writeSigningPlanArtifact(plan, *overwrite); err != nil {
				return fmt.Errorf("xcode signing plan: %w", err)
			}
			if exportOptionsPath != "" {
				if !plan.Ready {
					// A blocked plan's export options may omit targets; do not
					// leave a plist that looks usable next to a plan that is not.
					fmt.Fprintf(d.IO.ErrOut, "Warning: export options were not written to %s because the plan is blocked\n", exportOptionsPath)
				} else if plan.ExportOptions == nil {
					fmt.Fprintf(d.IO.ErrOut, "Warning: export options were not written to %s because no target is manually signed with a provisioning profile\n", exportOptionsPath)
				} else if err := localxcode.WriteSigningExportOptions(exportOptionsPath, plan.ExportOptions, *overwrite); err != nil {
					return fmt.Errorf("xcode signing plan: %w", err)
				}
			}
			for _, warning := range plan.Warnings {
				fmt.Fprintf(d.IO.ErrOut, "Warning: %s\n", shared.SanitizeTerminal(warning))
			}
			return d.PrintOutput(newXcodeSigningPlanOutput(plan), *output.Output, *output.Pretty)
		},
	}
}

func xcodeSigningApplyCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apply", flag.ExitOnError)
	planPath := fs.String("plan", "", "Path to a previously generated signing plan (required)")
	allowExternalXCConfig := fs.Bool("allow-external-xcconfig", false, "Authorize the external xcconfig setting recorded in the plan")
	confirm := fs.Bool("confirm", false, "Confirm local project mutation (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc xcode signing apply --plan PATH --confirm [flags]",
		ShortHelp:  "Verify and apply a signing plan.",
		LongHelp: `Verify and apply a signing plan.

Apply re-resolves the project and strict settings manifest and refuses stale,
redirected, blocked, or tampered plans. It requires --confirm and records a
mode-0600 receipt beside the plan.

Apply requires native identity-coupled file mutation support. On Windows, it
currently fails closed before modifying project or receipt files.

Examples:
  asc xcode signing apply --plan .asc/xcode/signing/plan.json --confirm
  asc xcode signing apply --plan .asc/xcode/signing/plan.json --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*planPath) == "" {
				return d.RequiredFlag("--plan")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			result, err := runApplySigningPlan(localxcode.SigningApplyOptions{
				PlanPath:              osPath(d, strings.TrimSpace(*planPath)),
				AllowExternalXCConfig: *allowExternalXCConfig,
			})
			if err != nil {
				if localxcode.IsSigningInputError(err) {
					return xcodeSigningInputUsageError(d, "xcode signing apply", err)
				}
				return fmt.Errorf("xcode signing apply: %w", err)
			}
			return d.PrintOutput(newXcodeSigningApplyOutput(result), *output.Output, *output.Pretty)
		},
	}
}

func xcodeSigningInputUsageError(d *shared.Deps, command string, err error) error {
	message := fmt.Sprintf("%s: %v", command, err)
	fmt.Fprintln(d.IO.ErrOut, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		"",
	)
}
