package release

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	routingcoveragecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReleaseStageCommand prepares an App Store version without submitting it for review.
func ReleaseStageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release stage", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string (required)")
	buildID := fs.String("build", "", "Build ID to attach (required)")
	metadataDir := fs.String("metadata-dir", "", "Metadata directory to apply")
	routingCoverageFile := fs.String("routing-coverage-file", "", "[experimental] Routing app coverage GeoJSON file to reconcile before readiness")
	copyMetadataFrom := fs.String("copy-metadata-from", "", "Copy localization metadata from this source version string")
	copyFields := fs.String("copy-fields", "", "Comma-separated metadata fields to copy: description, keywords, marketingUrl, promotionalText, supportUrl, whatsNew")
	excludeFields := fs.String("exclude-fields", "", "Comma-separated metadata fields to exclude from copy")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	timeout := fs.Duration("timeout", releaseRunTimeout, "Maximum time to run the staging pipeline")
	dryRun := fs.Bool("dry-run", false, "Preview deterministic plan without mutations")
	confirm := fs.Bool("confirm", false, "Confirm staging mutations (required unless --dry-run)")
	strictValidate := fs.Bool("strict-validate", false, "Treat readiness warnings as blocking")
	checkpointFile := fs.String("checkpoint-file", "", "Checkpoint path for resumable runs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "stage",
		ShortUsage: "asc release stage --app \"APP_ID\" --version \"2.4.0\" --build \"BUILD_ID\" (--metadata-dir \"./metadata/version/2.4.0\" | --copy-metadata-from \"2.3.2\") [--routing-coverage-file \"./coverage.geojson\"] [flags]",
		ShortHelp:  "Run version + metadata + attach + validate.",
		LongHelp: `Run a deterministic pre-submit App Store staging pipeline:
1. Ensure/create version
2. Apply metadata/localizations or copy metadata from another version
3. Reconcile routing app coverage when --routing-coverage-file is set
4. Attach selected build
5. Run readiness checks

Stops before creating a review submission.
Supports dry-run planning, step-level structured output, and checkpointed resume.

Examples:
  asc release stage --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --copy-metadata-from "2.3.2" --dry-run
  asc release stage --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --copy-metadata-from "2.3.2" --confirm
  asc release stage --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --metadata-dir "./metadata/version/2.4.0" --confirm
  asc release stage --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --copy-metadata-from "2.3.2" --routing-coverage-file "./coverage.geojson" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("release stage does not accept positional arguments")
			}
			if !*dryRun && !*confirm {
				return shared.UsageError("--confirm is required unless --dry-run is set")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			trimmedVersion := strings.TrimSpace(*version)
			if trimmedVersion == "" {
				return shared.UsageError("--version is required")
			}
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return shared.UsageError("--build is required")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *timeout <= 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			copyFieldsValue, err := shared.NormalizeVersionMetadataCopyFields(*copyFields, "--copy-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			excludeFieldsValue, err := shared.NormalizeVersionMetadataCopyFields(*excludeFields, "--exclude-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			trimmedMetadataDir := strings.TrimSpace(*metadataDir)
			trimmedCopyMetadataFrom := strings.TrimSpace(*copyMetadataFrom)
			if trimmedCopyMetadataFrom == "" && (len(copyFieldsValue) > 0 || len(excludeFieldsValue) > 0) {
				return shared.UsageError("--copy-metadata-from is required when using --copy-fields or --exclude-fields")
			}
			if (trimmedMetadataDir == "" && trimmedCopyMetadataFrom == "") || (trimmedMetadataDir != "" && trimmedCopyMetadataFrom != "") {
				return shared.UsageError("exactly one of --metadata-dir or --copy-metadata-from is required")
			}

			selectedCopyFields := []string(nil)
			if trimmedCopyMetadataFrom != "" {
				selectedCopyFields, err = shared.ResolveVersionMetadataCopyFields(copyFieldsValue, excludeFieldsValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			trimmedRoutingCoverageFile := strings.TrimSpace(*routingCoverageFile)
			var preparedRoutingCoverageFile *routingcoveragecli.PreparedRoutingCoverageFile
			if trimmedRoutingCoverageFile != "" {
				prepared, prepareErr := routingcoveragecli.PrepareRoutingCoverageFile(trimmedRoutingCoverageFile)
				if prepareErr != nil {
					return shared.UsageError(fmt.Sprintf("--routing-coverage-file is not usable: %v", prepareErr))
				}
				trimmedRoutingCoverageFile = prepared.Path
				preparedRoutingCoverageFile = &prepared
			}

			checkpointPath := strings.TrimSpace(*checkpointFile)
			if checkpointPath == "" {
				checkpointPath = defaultStageCheckpointPath(resolvedAppID, trimmedVersion, trimmedBuildID, normalizedPlatform)
			}
			absCheckpointPath, err := filepath.Abs(checkpointPath)
			if err != nil {
				return fmt.Errorf("release stage: resolve checkpoint path: %w", err)
			}

			result, runErr := executeStage(ctx, runOptions{
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
				StrictValidate:              *strictValidate,
				CheckpointFile:              absCheckpointPath,
			})
			if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				return shared.NewReportedError(runErr)
			}
			return nil
		},
	}
}
