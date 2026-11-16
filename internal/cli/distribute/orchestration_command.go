package distribute

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var (
	planHashPattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
	distributionRunIDPattern = regexp.MustCompile(`^drun_[0-9a-f]{32}$`)
)

func distributionPlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("distribute plan", flag.ExitOnError)
	archivePath := fs.String("archive-path", "", "Existing Xcode archive to distribute (required)")
	configPath := fs.String("config", "", "Owner-private distribution configuration JSON (required)")
	planPath := fs.String("plan", "", "Create-only distribution plan path (required)")
	stateDir := fs.String("state-dir", ".asc/distribution/runs", "Owner-private distribution run-state directory")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc distribute plan --archive-path PATH --config PATH --plan PATH [flags]",
		ShortHelp:  "Build a read-only, hash-bound distribution plan.",
		LongHelp: `Inspect an archive, the requested devices, signing identity,
current Apple signing state, and a private S3-compatible destination without mutation.

The resulting owner-private plan lists the exact additive account mutations and
local/storage effects. Apply authorizes only its exact planHash.

Example:
  asc distribute plan --archive-path .asc/artifacts/App.xcarchive --config .asc/distribution/config.json --plan .asc/distribution/plan.json --output json`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*archivePath) == "" {
				return d.UsageError("--archive-path is required")
			}
			if strings.TrimSpace(*configPath) == "" {
				return d.UsageError("--config is required")
			}
			if strings.TrimSpace(*planPath) == "" {
				return d.UsageError("--plan is required")
			}
			if strings.TrimSpace(*stateDir) == "" {
				return d.UsageError("--state-dir is required")
			}
			if err := validateDistributionOutputFormat(d, *output.Output, *output.Pretty); err != nil {
				return err
			}
			plan, err := executeDistributionPlan(ctx, d, distributionPlanRequest{
				ArchivePath: d.OSPath(*archivePath), ConfigPath: d.OSPath(*configPath), PlanPath: d.OSPath(*planPath), StateDir: d.OSPath(*stateDir),
			})
			if err != nil {
				return fmt.Errorf("distribute plan: %w", err)
			}
			return printDistributionValue(d, plan, *output.Output, *output.Pretty)
		},
	}
}

func distributionApplyCommand(d *shared.Deps) *ffcli.Command {
	return distributionApplyCommandWithExecutor(d, executeDistributionApply)
}

type distributionApplyExecutor func(context.Context, *shared.Deps, distributionApplyRequest) (*distributionRunState, error)

func distributionApplyCommandWithExecutor(d *shared.Deps, execute distributionApplyExecutor) *ffcli.Command {
	fs := d.NewFlagSet("distribute apply", flag.ExitOnError)
	planPath := fs.String("plan", "", "Exact distribution plan path (required)")
	confirm := fs.String("confirm", "", "Exact 64-character planHash to authorize")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc distribute apply --plan PATH --confirm PLAN_HASH [flags]",
		ShortHelp:  "Apply one exact distribution plan.",
		LongHelp: `Apply the exact additive effects authorized by a distribution plan.

Confirmation is the plan's complete SHA-256 planHash, not a boolean. Changed
inputs or remote state that require different effects stop and require a new plan.

Example:
  asc distribute apply --plan .asc/distribution/plan.json --confirm PLAN_HASH --output json`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			confirmation := strings.TrimSpace(*confirm)
			if confirmation == "" {
				return d.UsageError("--confirm PLAN_HASH is required")
			}
			if !planHashPattern.MatchString(confirmation) {
				return d.UsageError("--confirm must be a 64-character lowercase SHA-256 plan hash")
			}
			if strings.TrimSpace(*planPath) == "" {
				return d.UsageError("--plan is required")
			}
			if err := validateDistributionOutputFormat(d, *output.Output, *output.Pretty); err != nil {
				return err
			}
			result, err := execute(ctx, d, distributionApplyRequest{PlanPath: d.OSPath(*planPath), Confirmation: confirmation})
			if result != nil {
				if printErr := printDistributionValue(d, result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("distribute apply: %w", err)
			}
			return nil
		},
	}
}

func distributionResumeCommand(d *shared.Deps) *ffcli.Command {
	return distributionRunCommand(d, "resume", "Continue a previously confirmed recoverable distribution run.", executeDistributionResume)
}

func distributionStatusCommand(d *shared.Deps) *ffcli.Command {
	return distributionRunCommand(d, "status", "Read a distribution run without network or keychain access.", executeDistributionStatus)
}

func distributionVerifyCommand(d *shared.Deps) *ffcli.Command {
	return distributionVerifyCommandWithExecutor(d, executeDistributionVerify)
}

type distributionVerifyExecutor func(context.Context, *shared.Deps, distributionVerifyRequest) (*distributionVerificationResult, error)

func distributionVerifyCommandWithExecutor(d *shared.Deps, execute distributionVerifyExecutor) *ffcli.Command {
	fs := d.NewFlagSet("distribute verify", flag.ExitOnError)
	runID := fs.String("run", "", "Distribution run ID (required)")
	stateDir := fs.String("state-dir", ".asc/distribution/runs", "Distribution run-state directory")
	device := fs.String("device", "", "Optional connected-device selector for installed-app observation")
	timeout := fs.Duration("timeout", 30*time.Second, "Timeout for live fetch and optional device checks")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "verify", ShortUsage: "asc distribute verify --run RUN_ID [flags]",
		ShortHelp: "Reverify published artifacts and optionally observe a device install.",
		LongHelp: `Reverify the immutable receipt, local IPA, and every remotely
published object without mutation. --device observes the matching bundle, version,
and build on a connected device; it does not claim byte identity with the IPA.`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDistributionRunFlags(d, *runID, *stateDir, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if *timeout <= 0 {
				return d.UsageError("--timeout must be positive")
			}
			deviceSelector := strings.TrimSpace(*device)
			if deviceSelector != "" && (*timeout < deviceObservationMinimumTimeout || *timeout > deviceObservationMaximumTimeout) {
				return d.UsageError(fmt.Sprintf(
					"--timeout with --device must be between %s and %s",
					deviceObservationMinimumTimeout,
					deviceObservationMaximumTimeout,
				))
			}
			result, err := execute(ctx, d, distributionVerifyRequest{
				RunID: *runID, StateDir: d.OSPath(*stateDir), Device: deviceSelector, Timeout: *timeout,
			})
			if result != nil {
				if printErr := printDistributionValue(d, result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("distribute verify: %w", err)
			}
			return nil
		},
	}
}

type distributionRunExecutor func(context.Context, *shared.Deps, distributionRunRequest) (*distributionRunState, error)

func distributionRunCommand(d *shared.Deps, name, help string, execute distributionRunExecutor) *ffcli.Command {
	fs := d.NewFlagSet("distribute "+name, flag.ExitOnError)
	runID := fs.String("run", "", "Distribution run ID (required)")
	stateDir := fs.String("state-dir", ".asc/distribution/runs", "Distribution run-state directory")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortUsage: "asc distribute " + name + " --run RUN_ID [flags]",
		ShortHelp: "" + help,
		FlagSet:   fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDistributionRunFlags(d, *runID, *stateDir, *output.Output, *output.Pretty); err != nil {
				return err
			}
			result, err := execute(ctx, d, distributionRunRequest{RunID: *runID, StateDir: d.OSPath(*stateDir)})
			if result != nil {
				if printErr := printDistributionValue(d, result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("distribute %s: %w", name, err)
			}
			return nil
		},
	}
}

func validateDistributionRunFlags(d *shared.Deps, runID, stateDir, format string, pretty bool) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return d.UsageError("--run is required")
	}
	if !distributionRunIDPattern.MatchString(runID) {
		return d.UsageError("--run must be a drun_ identifier returned by distribute apply")
	}
	if strings.TrimSpace(stateDir) == "" {
		return d.UsageError("--state-dir is required")
	}
	if err := validateDistributionOutputFormat(d, format, pretty); err != nil {
		return err
	}
	return nil
}

func printDistributionValue(d *shared.Deps, value any, format string, pretty bool) error {
	switch typed := value.(type) {
	case *distributionPlan:
		return printDistributionPlan(d, typed, format, pretty)
	case *distributionRunState:
		return printDistributionRunState(d, typed, format, pretty)
	case *distributionVerificationResult:
		return printDistributionVerification(d, typed, format, pretty)
	default:
		return fmt.Errorf("unsupported distribution output type %T", value)
	}
}

func printDistributionPlan(d *shared.Deps, plan *distributionPlan, format string, pretty bool) error {
	if plan == nil {
		return fmt.Errorf("distribution plan output is required")
	}
	return printDistributionRows(d, plan, format, pretty, distributionPlanRows(plan))
}

func distributionPlanRows(plan *distributionPlan) [][]string {
	rows := [][]string{
		{"Schema Version", strconv.Itoa(plan.SchemaVersion)},
		{"Plan ID", plan.PlanID},
		{"Plan Hash", plan.PlanHash},
		{"Ready", strconv.FormatBool(plan.Ready)},
		{"Created At", plan.CreatedAt},
		{"Config Path", plan.ConfigPath},
		{"Config SHA-256", plan.ConfigSHA256},
		{"Archive Path", plan.Archive.Path},
		{"Bundle ID", plan.Archive.BundleID},
		{"Archived App Title", plan.Archive.Title},
		{"Published App Title", plan.Archive.PublishedTitle},
		{"App Version", plan.Archive.Version},
		{"App Build Number", plan.Archive.BuildNumber},
		{"Minimum OS Version", plan.Archive.MinimumOSVersion},
		{"Team ID", plan.Archive.TeamID},
		{"Archive Tree SHA-256", plan.Archive.TreeSHA256},
		{"Archive Size", strconv.FormatInt(plan.Archive.SizeBytes, 10)},
		{"Archive Files", strconv.Itoa(plan.Archive.FileCount)},
		{"Archive Targets", strconv.Itoa(plan.Archive.TargetCount)},
		{"Devices", strconv.Itoa(plan.DeviceSet.Count)},
		{"Device Set SHA-256", plan.DeviceSet.SHA256},
		{"Devices File SHA-256", plan.DeviceSet.FileSHA256},
		{"Certificate SHA-256", plan.Identity.CertificateSHA256},
		{"Certificate Resource ID", plan.Identity.CertificateResourceID},
		{"Identity Team ID", plan.Identity.TeamID},
		{"Certificate Expires At", plan.Identity.ExpirationDate},
		{"Minimum Valid Until", plan.Identity.MinimumValidUntil},
		{"Reconcile Plan Path", plan.Reconcile.PlanPath},
		{"Reconcile Plan SHA-256", plan.Reconcile.PlanHash},
		{"Reconcile Receipt Path", plan.Reconcile.ReceiptPath},
		{"Effective Minimum Validity Days", strconv.Itoa(plan.Reconcile.MinimumValidityDays)},
		{"Account Mutations", strconv.Itoa(plan.Reconcile.MutationCount)},
		{"Maximum Account Mutations", strconv.Itoa(plan.Reconcile.MaxMutations)},
		{"Storage Endpoint", plan.Publication.Endpoint},
		{"Download Endpoint", plan.Publication.DownloadEndpoint},
		{"Storage Region", plan.Publication.Region},
		{"Storage Bucket", plan.Publication.Bucket},
		{"Storage Prefix", plan.Publication.Prefix},
		{"Addressing Style", plan.Publication.AddressingStyle},
		{"URL TTL", plan.Publication.URLTTL},
		{"Download Grace", plan.Publication.DownloadGrace},
		{"Verify Timeout", plan.Publication.VerifyTimeout},
		{"Effects", strconv.Itoa(len(plan.Effects))},
		{"Blockers", strconv.Itoa(len(plan.Blockers))},
		{"Run State Directory", plan.Paths.StateDir},
	}
	for index, effect := range plan.Effects {
		value := fmt.Sprintf("stage=%s kind=%s count=%d", effect.Stage, effect.Kind, effect.Count)
		if effect.BundleID != "" {
			value += " bundleId=" + effect.BundleID
		}
		rows = append(rows, []string{fmt.Sprintf("Effect %d", index+1), value})
	}
	for index, blocker := range plan.Blockers {
		rows = append(rows, []string{
			fmt.Sprintf("Blocker %d", index+1),
			fmt.Sprintf("stage=%s code=%s: %s", blocker.Stage, blocker.Code, blocker.Message),
		})
	}
	return rows
}

func printDistributionRunState(d *shared.Deps, state *distributionRunState, format string, pretty bool) error {
	if state == nil {
		return fmt.Errorf("distribution run output is required")
	}
	return printDistributionRows(d, state, format, pretty, distributionRunRows(state))
}

func distributionRunRows(state *distributionRunState) [][]string {
	rows := [][]string{
		{"Schema Version", strconv.Itoa(state.SchemaVersion)},
		{"Run ID", state.RunID},
		{"Plan ID", state.PlanID},
		{"Plan Hash", state.PlanHash},
		{"Status", state.Status},
		{"Stage", state.Stage},
		{"Attempt", strconv.Itoa(state.Attempt)},
		{"Recoverable", strconv.FormatBool(state.Recoverable)},
		{"Updated At", state.UpdatedAt},
	}
	if state.LastFailureCode != "" {
		rows = append(rows, []string{"Last Failure Code", state.LastFailureCode})
	}
	if artifact := state.Artifacts.ArchiveSnapshot; artifact != nil {
		rows = append(rows, []string{"Archive Snapshot SHA-256", artifact.TreeSHA256})
	}
	if artifact := state.Artifacts.Profile; artifact != nil {
		rows = append(rows, []string{"Profile UUID", artifact.UUID}, []string{"Profile SHA-256", artifact.SHA256})
	}
	if artifact := state.Artifacts.IPA; artifact != nil {
		rows = append(rows, []string{"IPA SHA-256", artifact.SHA256}, []string{"IPA Size", strconv.FormatInt(artifact.SizeBytes, 10)})
	}
	if artifact := state.Artifacts.Bundle; artifact != nil {
		rows = append(rows, []string{"Bundle Descriptor SHA-256", artifact.DescriptorSHA256})
	}
	if artifact := state.Artifacts.Publication; artifact != nil {
		rows = append(
			rows,
			[]string{"Publication Receipt Path", artifact.ReceiptPath},
			[]string{"Publication Receipt SHA-256", artifact.ReceiptSHA256},
			[]string{"Private Link Artifact", artifact.LinkPath},
			[]string{"Install URL (Redacted)", artifact.InstallURLRedacted},
		)
	}
	return rows
}

func printDistributionVerification(d *shared.Deps, result *distributionVerificationResult, format string, pretty bool) error {
	if result == nil {
		return fmt.Errorf("distribution verification output is required")
	}
	return printDistributionRows(d, result, format, pretty, distributionVerificationRows(result))
}

func distributionVerificationRows(result *distributionVerificationResult) [][]string {
	rows := [][]string{
		{"Schema Version", strconv.Itoa(result.SchemaVersion)},
		{"Run ID", result.RunID},
		{"Plan Hash", result.PlanHash},
		{"Publication Verified", strconv.FormatBool(result.PublicationVerified)},
		{"Verified At", result.VerifiedAt},
		{"IPA SHA-256", result.ArtifactSHA256},
		{"Bundle ID", result.AppBundleID},
		{"Version", result.AppVersion},
		{"Build", result.AppBuildNumber},
	}
	if observation := result.DeviceObservation; observation != nil {
		rows = append(
			rows,
			[]string{"Device Observation Requested", strconv.FormatBool(observation.Requested)},
			[]string{"Device Found", strconv.FormatBool(observation.DeviceFound)},
			[]string{"App Installed", strconv.FormatBool(observation.AppInstalled)},
			[]string{"Observed Bundle ID", observation.BundleID},
			[]string{"Observed Version", observation.Version},
			[]string{"Observed Build", observation.Build},
		)
	}
	return rows
}

func printDistributionRows(d *shared.Deps, value any, format string, pretty bool, rows [][]string) error {
	return d.PrintOutputWithRenderers(
		value, format, pretty,
		func() error {
			asc.RenderTableTo(d.IO.Out, []string{"Field", "Value"}, rows)
			return nil
		},
		func() error {
			asc.RenderMarkdownTo(d.IO.Out, []string{"Field", "Value"}, rows)
			return nil
		},
	)
}

func validateDistributionOutputFormat(d *shared.Deps, format string, pretty bool) error {
	_, err := shared.ValidateOutputFormatAllowed(format, pretty, "json", "table", "markdown")
	if err != nil {
		return d.UsageError(err.Error())
	}
	return nil
}
