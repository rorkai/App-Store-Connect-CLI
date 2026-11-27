package release

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	routingcoveragecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// newReleaseStageCommand prepares an App Store version without submitting it for review.
func newReleaseStageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("release stage", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string (required)")
	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to attach (required)")
	shared.BindFlagAlias(fs, "build", "build-id")
	metadataDir := fs.String("metadata-dir", "", "Metadata directory to apply")
	allowDeletes := fs.Bool("allow-deletes", false, "Allow destructive delete operations when applying --metadata-dir (disables default locale fallback for missing locales)")
	routingCoverageFile := fs.String("routing-coverage-file", "", "Routing app coverage GeoJSON file to reconcile before readiness")
	copyMetadataFrom := fs.String("copy-metadata-from", "", "Copy localization metadata from this source version string")
	copyFields := shared.BindOnceCSVFlag(fs, "copy-fields", "Comma-separated metadata fields to copy: description, keywords, marketingUrl, promotionalText, supportUrl, whatsNew")
	excludeFields := shared.BindOnceCSVFlag(fs, "exclude-fields", "Comma-separated metadata fields to exclude from copy")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	timeout := fs.Duration("timeout", releaseRunTimeout, "Maximum time to run the staging pipeline")
	dryRun := fs.Bool("dry-run", false, "Preview deterministic plan without mutations")
	confirm := fs.Bool("confirm", false, "Confirm staging mutations (required unless --dry-run)")
	strictValidate := fs.Bool("strict-validate", false, "Treat readiness warnings as blocking")
	checkpointFile := fs.String("checkpoint-file", "", "Checkpoint path for resumable runs")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "stage",
		ShortUsage: "asc release stage --app \"APP_ID\" --version \"2.4.0\" --build-id \"BUILD_ID\" (--metadata-dir \"./metadata\" | --copy-metadata-from \"2.3.2\") [--routing-coverage-file \"./coverage.geojson\"] [flags]",
		ShortHelp:  "Run version + metadata + attach + validate.",
		LongHelp: `Run a deterministic pre-submit App Store staging pipeline:
1. Verify --build-id exists, belongs to --app, and matches --platform
2. Ensure/create version
3. Apply metadata/localizations or copy metadata from another version
4. Reconcile routing app coverage when --routing-coverage-file is set
5. Attach selected build
6. Run readiness checks

Stops before creating a review submission.
Supports dry-run planning, step-level structured output, and checkpointed resume.

After staging succeeds, submit the existing version and build with:
  asc review submit --app "APP_ID" --version-id "VERSION_ID" --build-id "BUILD_ID" --platform "IOS" --confirm
Use the version ID, build ID, and platform from the staging receipt.

--copy-metadata-from copies version localization text only (description,
keywords, promotional text, what's new, marketing and support URLs) into
locales that already exist on the target version. Neither metadata source sets
the age rating, screenshots, review details, category, or pricing, so readiness
still blocks until those are complete and the attached build is ready. When it
does, the earlier steps stay applied, the blockers are printed to stderr, and
the command exits 1; fix them and rerun the same command to resume from the
checkpoint. List the remaining blockers with:
  asc validate --app "APP_ID" --version "2.4.0"

Examples:
  asc release stage --app "APP_ID" --version "2.4.0" --build-id "BUILD_ID" --copy-metadata-from "2.3.2" --dry-run
  asc release stage --app "APP_ID" --version "2.4.0" --build-id "BUILD_ID" --copy-metadata-from "2.3.2" --confirm
  asc release stage --app "APP_ID" --version "2.4.0" --build-id "BUILD_ID" --metadata-dir "./metadata" --confirm
  asc release stage --app "APP_ID" --version "2.4.0" --build-id "BUILD_ID" --copy-metadata-from "2.3.2" --routing-coverage-file "./coverage.geojson" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			trimmedVersion := strings.TrimSpace(*version)
			if trimmedVersion == "" {
				return d.UsageError("--version is required")
			}
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return d.UsageError("--build-id is required")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if *timeout <= 0 {
				return d.UsageError("--timeout must be greater than 0")
			}

			copyFieldsValue, err := shared.NormalizeVersionMetadataCopyFields(copyFields.String(), "--copy-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			excludeFieldsValue, err := shared.NormalizeVersionMetadataCopyFields(excludeFields.String(), "--exclude-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}

			trimmedMetadataDir := strings.TrimSpace(*metadataDir)
			trimmedCopyMetadataFrom := strings.TrimSpace(*copyMetadataFrom)
			if trimmedCopyMetadataFrom == "" && (len(copyFieldsValue) > 0 || len(excludeFieldsValue) > 0) {
				return d.UsageError("--copy-metadata-from is required when using --copy-fields or --exclude-fields")
			}
			if (trimmedMetadataDir == "" && trimmedCopyMetadataFrom == "") || (trimmedMetadataDir != "" && trimmedCopyMetadataFrom != "") {
				return d.UsageError("exactly one of --metadata-dir or --copy-metadata-from is required")
			}
			if err := d.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}
			if *allowDeletes && trimmedMetadataDir == "" {
				return d.UsageError("--allow-deletes requires --metadata-dir")
			}

			selectedCopyFields := []string(nil)
			if trimmedCopyMetadataFrom != "" {
				selectedCopyFields, err = shared.ResolveVersionMetadataCopyFields(copyFieldsValue, excludeFieldsValue)
				if err != nil {
					return d.UsageError(err.Error())
				}
			}

			trimmedRoutingCoverageFile := strings.TrimSpace(*routingCoverageFile)
			var preparedRoutingCoverageFile *routingcoveragecli.PreparedRoutingCoverageFile
			if trimmedRoutingCoverageFile != "" {
				prepared, prepareErr := routingcoveragecli.PrepareRoutingCoverageFile(d, trimmedRoutingCoverageFile)
				if prepareErr != nil {
					return d.UsageError(fmt.Sprintf("--routing-coverage-file is not usable: %v", prepareErr))
				}
				trimmedRoutingCoverageFile = prepared.Path
				preparedRoutingCoverageFile = &prepared
			}

			checkpointPath := strings.TrimSpace(*checkpointFile)
			if checkpointPath == "" {
				checkpointPath = defaultStageCheckpointPath(resolvedAppID, trimmedVersion, trimmedBuildID, normalizedPlatform)
			}
			absCheckpointPath, err := d.Abs(checkpointPath)
			if err != nil {
				return fmt.Errorf("release stage: resolve checkpoint path: %w", err)
			}

			result, runErr := executeStage(ctx, d, runOptions{
				AppID:                       resolvedAppID,
				Version:                     trimmedVersion,
				BuildID:                     trimmedBuildID,
				MetadataDir:                 trimmedMetadataDir,
				CopyMetadataFrom:            trimmedCopyMetadataFrom,
				SelectedCopyFields:          selectedCopyFields,
				RoutingCoverageFile:         trimmedRoutingCoverageFile,
				PreparedRoutingCoverageFile: preparedRoutingCoverageFile,
				Platform:                    normalizedPlatform,
				Timeout:                     *timeout,
				DryRun:                      *dryRun,
				Confirm:                     *confirm,
				AllowDeletes:                *allowDeletes,
				StrictValidate:              *strictValidate,
				CheckpointFile:              absCheckpointPath,
			})
			if printErr := printStageResult(d, result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				if blocked, ok := errors.AsType[readinessBlockedError](runErr); ok {
					printReadinessBlockers(d.IO.ErrOut, blocked.report, resolvedAppID, trimmedVersion, normalizedPlatform)
					return shared.NewStderrReportedError(runErr)
				}
				return shared.NewReportedError(runErr)
			}
			if result.DryRun || result.Status != "ok" {
				return nil
			}
			rootFlags, ok := shared.RootFlagsForNextCommand()
			if !ok {
				return nil
			}
			command := append([]string{"asc"}, rootFlags...)
			command = append(command, "review", "submit")
			for _, selector := range []struct{ flag, value string }{
				{"--app", result.AppID},
				{"--version-id", result.VersionID},
				{"--build-id", result.BuildID},
				{"--platform", result.Platform},
			} {
				quoted, ok := shared.ShellQuote(selector.value)
				if !ok || strings.TrimSpace(selector.value) == "" {
					return nil
				}
				command = append(command, selector.flag, quoted)
			}
			command = append(command, "--confirm")
			fmt.Fprintf(d.IO.ErrOut, "To submit this staged version and build for review:\n  %s\n", strings.Join(command, " "))
			return nil
		},
	}
}

const maxListedReadinessBlockers = 5

func printReadinessBlockers(w io.Writer, report validation.Report, appID, version, platform string) {
	fmt.Fprintf(w, "Error: release stage: %s: %d blocking issue(s)\n", stepValidateReadiness, report.Summary.Blocking)
	listed := 0
	for _, step := range report.Remediation.Steps {
		if !step.Blocking {
			continue
		}
		if listed == maxListedReadinessBlockers {
			break
		}
		fmt.Fprintf(w, "  - %s: %s\n", shared.SanitizeTerminal(step.Message), shared.SanitizeTerminal(step.Remediation))
		listed++
	}
	if remaining := report.Summary.Blocking - listed; remaining > 0 && listed > 0 {
		fmt.Fprintf(w, "  ... and %d more\n", remaining)
	}
	command := fmt.Sprintf("asc validate --app %q --version %q --platform %q", appID, version, platform)
	if report.Strict {
		command += " --strict"
	}
	fmt.Fprintf(w, "Run: %s\n", shared.SanitizeTerminal(command))
}
