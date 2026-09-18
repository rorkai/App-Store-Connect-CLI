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

// XcodeSigningCommand returns the local Xcode signing-settings
// plan/apply command group. It edits only project build settings; credentials,
// profiles, and certificates remain owned by the asc signing commands.
func XcodeSigningCommand() *ffcli.Command {
	fs := flag.NewFlagSet("signing", flag.ExitOnError)
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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			xcodeSigningPlanCommand(),
			xcodeSigningApplyCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func xcodeSigningPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
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
	overwrite := fs.Bool("overwrite", false, "Replace an existing plan artifact")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc xcode signing plan --project PATH (--settings-file PATH | --profile PATH) [flags]",
		ShortHelp:  "Resolve signing settings and write a plan.",
		LongHelp: `Resolve signing settings and write a plan.

The settings file must contain schemaVersion 1 and explicit target,
configuration, and allowlisted signing-setting values. Pass one or more
--profile files to infer CODE_SIGN_STYLE, DEVELOPMENT_TEAM, CODE_SIGN_IDENTITY,
and PROVISIONING_PROFILE_SPECIFIER from each target's bundle identifier.
--settings-file overrides inferred values. A representable blocked plan is
written with ready=false and explains the blocker without changing the
project. Any unauthorized external xcconfig prevents artifact publication
because its contents cannot be safely inventoried; pass
--allow-external-xcconfig to authorize reading it.

Examples:
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json
  asc xcode signing plan --project ./App.xcodeproj --profile ./signing/App.mobileprovision --configuration Release
  asc xcode signing plan --project ./App.xcodeproj --settings-file .asc/xcode-signing.json --overwrite --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if strings.TrimSpace(*project) == "" {
				fmt.Fprintln(os.Stderr, "Error: --project is required")
				return shared.MissingRequiredUsageError("--project")
			}
			if strings.TrimSpace(*settingsFile) == "" && len(profiles) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --settings-file is required")
				return shared.MissingRequiredUsageError("--settings-file")
			}
			method, err := normalizeSigningExportMethod(*exportMethod)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			// An explicitly empty value must not fall back to the flag default;
			// silently relocating the plan and receipt would hide where the
			// artifacts were written.
			if strings.TrimSpace(*stateDir) == "" {
				return shared.UsageErrorf("--state-dir must not be empty")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			plan, err := runBuildSigningPlan(localxcode.SigningPlanOptions{
				ProjectPath:           strings.TrimSpace(*project),
				SettingsFilePath:      strings.TrimSpace(*settingsFile),
				ProfilePaths:          []string(profiles),
				Configuration:         strings.TrimSpace(*configuration),
				ExportMethod:          method,
				SkipTargets:           []string(skipTargets),
				StateDir:              strings.TrimSpace(*stateDir),
				AllowExternalXCConfig: *allowExternalXCConfig,
			})
			if err != nil {
				if localxcode.IsSigningInputError(err) {
					return xcodeSigningInputUsageError("xcode signing plan", err)
				}
				return fmt.Errorf("xcode signing plan: %w", err)
			}
			if err := writeSigningPlanArtifact(plan, *overwrite); err != nil {
				return fmt.Errorf("xcode signing plan: %w", err)
			}
			if strings.TrimSpace(*exportOptionsOut) != "" {
				if err := localxcode.WriteSigningExportOptions(strings.TrimSpace(*exportOptionsOut), plan.ExportOptions); err != nil {
					return fmt.Errorf("xcode signing plan: %w", err)
				}
			}
			for _, warning := range plan.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
			return shared.PrintOutput(newXcodeSigningPlanOutput(plan), *output.Output, *output.Pretty)
		},
	}
}

func xcodeSigningApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	planPath := fs.String("plan", "", "Path to a previously generated signing plan (required)")
	allowExternalXCConfig := fs.Bool("allow-external-xcconfig", false, "Authorize the external xcconfig setting recorded in the plan")
	confirm := fs.Bool("confirm", false, "Confirm local project mutation (required)")
	output := shared.BindOutputFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if strings.TrimSpace(*planPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --plan is required")
				return shared.MissingRequiredUsageError("--plan")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			result, err := runApplySigningPlan(localxcode.SigningApplyOptions{
				PlanPath:              strings.TrimSpace(*planPath),
				AllowExternalXCConfig: *allowExternalXCConfig,
			})
			if err != nil {
				if localxcode.IsSigningInputError(err) {
					return xcodeSigningInputUsageError("xcode signing apply", err)
				}
				return fmt.Errorf("xcode signing apply: %w", err)
			}
			return shared.PrintOutput(newXcodeSigningApplyOutput(result), *output.Output, *output.Pretty)
		},
	}
}

func xcodeSigningInputUsageError(command string, err error) error {
	message := fmt.Sprintf("%s: %v", command, err)
	fmt.Fprintln(os.Stderr, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		"",
	)
}
