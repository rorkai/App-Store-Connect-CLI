package publish

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	submitcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/submit"
)

const (
	publishDefaultTimeout    = 30 * time.Minute
	publishPlanStatusPlanned = "planned"

	publishPlanStepArchiveLocalBuild      = "archive_local_build"
	publishPlanStepExportLocalBuild       = "export_local_build"
	publishPlanStepUploadBuild            = "upload_build"
	publishPlanStepWaitForBuildProcessing = "wait_for_build_processing"
	publishPlanStepEnsureVersion          = "ensure_version"
	publishPlanStepApplyMetadata          = "apply_metadata"
	publishPlanStepAttachBuild            = "attach_build"
	publishPlanStepSubmitReview           = "submit_review"

	publishPartialStatus                  = "partial"
	publishCompletedStageArchive          = "archive"
	publishCompletedStageExport           = "export"
	publishCompletedStageUpload           = "upload"
	publishCompletedStageBuildProcessing  = "build_processing"
	publishCompletedStageTestNotes        = "test_notes"
	publishCompletedStageBetaDistribution = "beta_group_distribution"
	publishFailureStageBuildProcessing    = "build_processing"
	publishFailureStageTestNotes          = "test_notes"
	publishFailureStageBetaDistribution   = "beta_group_distribution"
	publishFailureStageNotification       = "notification"
	publishFailureStageBetaReview         = "beta_review_submission"
)

// PublishCommand returns the publish command with subcommands.
func PublishCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "publish",
		ShortUsage: "asc publish <subcommand> [flags]",
		ShortHelp:  "High-level publish workflows for TestFlight and App Store.",
		LongHelp: `High-level publish workflows.

Use:
  - asc publish testflight for TestFlight distribution
  - asc publish appstore for the canonical App Store upload + submit flow
  - asc release stage to prepare an App Store version without submitting it

Examples:
  asc publish testflight --app APP_ID --ipa app.ipa --group GROUP_ID
  asc publish appstore --app APP_ID --ipa app.ipa --version 1.2.3 --submit --confirm`,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			PublishTestFlightCommand(),
			PublishAppStoreCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PublishTestFlightCommand uploads a build and optionally distributes it to TestFlight groups.
func PublishTestFlightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish testflight", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to prebuilt .ipa file")
	pkgPath := fs.String("pkg", "", "Path to prebuilt macOS .pkg file (requires --version and --build-number)")
	buildID := fs.String("build-id", "", "Existing build ID to distribute (skip upload)")
	version := fs.String("version", "", "CFBundleShortVersionString (auto-extracted from IPA if not provided; required with --pkg)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (required with --pkg; used for build lookup when no artifact is provided)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	groupIDs := fs.String("group", "", "Beta group ID(s) or name(s), comma-separated")
	uploadOnly := fs.Bool("upload-only", false, "Upload the build without adding it to beta groups or submitting beta review")
	notify := fs.Bool("notify", false, "Notify testers after adding to groups")
	submit := fs.Bool("submit", false, "Submit build for beta app review after adding external groups")
	confirm := fs.Bool("confirm", false, "Confirm beta app review submission (required with --submit)")
	wait := fs.Bool("wait", false, "Wait for build processing to complete")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and build discovery")
	timeout := fs.Duration("timeout", 0, "Override upload + processing timeout (e.g., 30m)")
	testNotes := fs.String("test-notes", "", "What to Test notes for the build")
	locale := fs.String("locale", "", "Locale for --test-notes (e.g., en-US)")
	localBuild := bindPublishLocalBuildFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "testflight",
		ShortUsage: "asc publish testflight [flags]",
		ShortHelp:  "Upload and distribute to TestFlight.",
		LongHelp: `Upload or local-build a binary, then optionally distribute it to TestFlight beta groups.

Steps:
1. Build locally with Xcode or upload an IPA or macOS PKG (unless --build-id/--build-number is provided)
2. Wait for processing when needed (--wait, --test-notes, or --submit)
3. Stop and return the build metadata with --upload-only, or add the build to specified beta groups
4. Optionally notify testers
5. Optionally submit for beta app review with --submit --confirm

Examples:
  asc publish testflight --app "123" --ipa app.ipa --upload-only --output json
  asc publish testflight --app "123" --pkg MacApp.pkg --version 1.2.3 --build-number 42 --upload-only --output json
  asc publish testflight --app "123" --ipa app.ipa --upload-only --wait --output json
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID"
  asc publish testflight --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3 --group "GROUP_ID"
  asc publish testflight --app "123" --workspace MacApp.xcworkspace --scheme MacApp --version 1.2.3 --platform MAC_OS --pkg-path .asc/artifacts/MacApp.pkg --group "GROUP_ID"
  asc publish testflight --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3 --group "GROUP_ID" --signing-style manual --team-id TEAM_ID
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers"
  asc publish testflight --app "123" --ipa app.ipa --group "G1,G2" --wait --notify
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers" --submit --confirm
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID" --test-notes "Test instructions" --locale "en-US" --wait
  asc publish testflight --app "123" --build-id "BUILD_ID" --group "GROUP_ID" --wait
  asc publish testflight --app "123" --build-number "42" --group "GROUP_ID" --wait`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppInput := shared.ResolveAppID(*appID)
			if resolvedAppInput == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			setFlags := collectSetFlags(fs)
			ipaValue := strings.TrimSpace(*ipaPath)
			pkgValue := strings.TrimSpace(*pkgPath)
			buildIDValue := strings.TrimSpace(*buildID)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			versionValue := strings.TrimSpace(*version)
			testNotesValue := strings.TrimSpace(*testNotes)
			localeValue := strings.TrimSpace(*locale)
			localBuildMode := localBuild.localBuildMode()
			if *uploadOnly {
				for _, flagName := range []string{"group", "notify", "submit", "confirm", "test-notes", "locale"} {
					if setFlags[flagName] {
						return shared.UsageErrorf("--%s cannot be used with --upload-only", flagName)
					}
				}
				if setFlags["build-id"] {
					return shared.UsageError("--build-id cannot be used with --upload-only")
				}
			}
			if err := validateLocalBuildFlagUsage(localBuildMode, setFlags); err != nil {
				return err
			}
			if localBuildMode {
				if err := validatePublishExportOptionsFlags(localBuild, setFlags); err != nil {
					return err
				}
				if err := validatePublishExportXcodebuildArgs(localBuild.exportXcodebuildArg); err != nil {
					return err
				}
			}

			if ipaValue != "" && pkgValue != "" {
				return shared.UsageError("--ipa and --pkg are mutually exclusive")
			}
			uploadMode := ipaValue != "" || pkgValue != ""
			switch {
			case localBuildMode:
				if err := validateLocalBuildSelectors(localBuild); err != nil {
					return err
				}
				if ipaValue != "" {
					return shared.UsageError("--ipa cannot be combined with --workspace or --project")
				}
				if pkgValue != "" {
					return shared.UsageError("--pkg cannot be combined with --workspace or --project")
				}
				if buildIDValue != "" {
					return shared.UsageError("--build-id cannot be combined with --workspace or --project")
				}
				if versionValue == "" {
					return shared.UsageError("--version is required")
				}
			case uploadMode:
				if buildIDValue != "" {
					if ipaValue != "" {
						return shared.UsageError("--ipa and --build-id are mutually exclusive")
					}
					return shared.UsageError("--pkg and --build-id are mutually exclusive")
				}
				if pkgValue != "" {
					if err := validatePublishPKGMetadata(versionValue, buildNumberValue); err != nil {
						return err
					}
				}
			default:
				if *uploadOnly {
					return shared.UsageError("--upload-only requires --ipa, --pkg, --workspace, or --project")
				}
				if buildIDValue == "" && buildNumberValue == "" {
					return shared.UsageError("--ipa or --pkg is required unless --build-id or --build-number is provided")
				}
				if buildIDValue != "" && buildNumberValue != "" {
					return shared.UsageError("--build-id and --build-number are mutually exclusive when --ipa is not provided")
				}
				if versionValue != "" {
					return shared.UsageError("--version is only supported when --ipa is provided")
				}
			}

			parsedGroupIDs := shared.SplitCSV(*groupIDs)
			if !*uploadOnly {
				if len(parsedGroupIDs) == 0 {
					fmt.Fprintf(os.Stderr, "Error: --group is required\n\n")
					return shared.MissingRequiredUsageError("--group")
				}
				if *submit && !*confirm {
					fmt.Fprintln(os.Stderr, "Error: --confirm is required with --submit")
					return shared.MissingRequiredUsageError("--confirm")
				}
				if *confirm && !*submit {
					fmt.Fprintln(os.Stderr, "Error: --confirm requires --submit")
					return flag.ErrHelp
				}

				if testNotesValue != "" && localeValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --locale is required with --test-notes")
					return shared.MissingRequiredUsageError("--locale")
				}
				if testNotesValue == "" && localeValue != "" {
					fmt.Fprintln(os.Stderr, "Error: --test-notes is required with --locale")
					return shared.MissingRequiredUsageError("--test-notes")
				}
				if testNotesValue != "" {
					if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
						return shared.UsageError(err.Error())
					}
				}
			}

			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedPlatform, err = validatePublishPrebuiltArtifactPlatform(ipaValue, pkgValue, normalizedPlatform, setFlags["platform"])
			if err != nil {
				return err
			}
			if localBuildMode {
				if err := validateLocalBuildArtifactFlags(localBuild, setFlags, normalizedPlatform); err != nil {
					return err
				}
			}

			var uploadFileInfo os.FileInfo
			uploadVersionValue := ""
			uploadBuildNumberValue := ""
			if uploadMode {
				if pkgValue != "" {
					uploadFileInfo, err = validatePublishPKGPathFn(pkgValue)
					uploadVersionValue, uploadBuildNumberValue = versionValue, buildNumberValue
				} else {
					uploadFileInfo, err = validatePublishIPAPathFn(ipaValue)
					if err == nil {
						uploadVersionValue, uploadBuildNumberValue, err = shared.ResolveBundleInfoForIPA(ipaValue, *version, *buildNumber)
					}
				}
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			timeoutValue := resolvePublishTimeout(*timeout)
			client, err := getPublishASCClientFn(timeoutValue)
			if err != nil {
				return fmt.Errorf("publish testflight: %w", err)
			}
			newPublishRequestCtx := func() (context.Context, context.CancelFunc) {
				return shared.ContextWithTimeoutDuration(ctx, timeoutValue)
			}
			requestCtx, cancel := newPublishRequestCtx()
			if !localBuildMode {
				defer cancel()
			}

			resolvedPublishAppID := resolvedAppInput
			preflightCtx := requestCtx
			if localBuildMode {
				cancel()
				var preflightCancel context.CancelFunc
				preflightCtx, preflightCancel = newPublishRequestCtx()
				defer preflightCancel()
			}
			resolvedPublishAppID, err = resolvePublishAppIDWithLookupFn(preflightCtx, client, resolvedPublishAppID)
			if err != nil {
				return fmt.Errorf("publish testflight: resolve app: %w", err)
			}

			var resolvedGroups []shared.ResolvedBetaGroup
			if !*uploadOnly {
				groupLookupCtx := preflightCtx
				resolvedGroups, err = resolvePublishBetaGroups(groupLookupCtx, client, resolvedPublishAppID, parsedGroupIDs)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			uploaded := false
			resolvedVersionValue := ""
			resolvedBuildNumberValue := ""
			mode := asc.PublishModeExistingBuild
			var localBuildResult *publishLocalBuildExecutionResult

			var buildResp *asc.BuildResponse
			if localBuildMode {
				buildNumberValue, err = resolvePublishBuildNumber(preflightCtx, client, resolvedPublishAppID, versionValue, normalizedPlatform, localBuild, buildNumberValue)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
				localBuildConfig, err := resolveLocalBuildConfig(localBuild, normalizedPlatform, versionValue, buildNumberValue)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
				localBuildResult, err = runPublishLocalBuild(ctx, client, resolvedPublishAppID, normalizedPlatform, versionValue, buildNumberValue, *pollInterval, timeoutValue, timeoutOverride, localBuildConfig)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
				requestCtx, cancel = newPublishRequestCtx()
				defer cancel()
				buildResp = localBuildResult.Build
				uploaded = localBuildResult.Uploaded
				resolvedVersionValue = localBuildResult.Version
				resolvedBuildNumberValue = localBuildResult.BuildNumber
				mode = asc.PublishModeLocalBuild
			} else if uploadMode {
				uploadArtifact := uploadBuildAndWaitForIDFn
				artifactPath := ipaValue
				mode = asc.PublishModeIPAUpload
				if pkgValue != "" {
					uploadArtifact = uploadPKGBuildAndWaitForIDFn
					artifactPath = pkgValue
					mode = asc.PublishModePKGUpload
				}
				uploadResult, err := uploadArtifact(
					requestCtx,
					client,
					resolvedPublishAppID,
					artifactPath,
					uploadFileInfo,
					uploadVersionValue,
					uploadBuildNumberValue,
					platformValue,
					*pollInterval,
					timeoutValue,
					timeoutOverride,
				)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}

				buildResp = uploadResult.Build
				uploaded = true
				resolvedVersionValue = uploadResult.Version
				resolvedBuildNumberValue = uploadResult.BuildNumber
			} else if buildIDValue != "" {
				buildResp, err = client.GetBuild(requestCtx, buildIDValue)
				if err != nil {
					return fmt.Errorf("publish testflight: failed to fetch build: %w", err)
				}
				resolvedBuildNumberValue = strings.TrimSpace(buildResp.Data.Attributes.Version)
			} else {
				buildResp, err = findPublishBuildByNumber(requestCtx, client, resolvedPublishAppID, buildNumberValue, normalizedPlatform)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
				resolvedBuildNumberValue = strings.TrimSpace(buildResp.Data.Attributes.Version)
			}

			result := &asc.TestFlightPublishResult{
				Mode:            mode,
				BuildID:         buildResp.Data.ID,
				BuildVersion:    resolvedVersionValue,
				BuildNumber:     resolvedBuildNumberValue,
				GroupIDs:        resolvedPublishBetaGroupIDs(resolvedGroups),
				Uploaded:        uploaded,
				UploadOnly:      *uploadOnly,
				ProcessingState: buildResp.Data.Attributes.ProcessingState,
			}
			completedStages := make([]string, 0, 6)
			if uploaded {
				if localBuildResult != nil {
					completedStages = append(completedStages, publishCompletedStageArchive, publishCompletedStageExport)
				}
				completedStages = append(completedStages, publishCompletedStageUpload)
			}
			reportPartialFailure := func(stage string, failure error) error {
				if !uploaded {
					return failure
				}
				result.Status = publishPartialStatus
				result.FailureStage = stage
				result.Failure = shared.SanitizeTerminal(failure.Error())
				result.CompletedStages = append([]string(nil), completedStages...)
				result.ProcessingState = buildResp.Data.Attributes.ProcessingState
				attachTestFlightLocalPublishResult(result, localBuildResult)
				if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
					return errors.Join(failure, fmt.Errorf("print partial publish result: %w", printErr))
				}
				return failure
			}

			if *wait || testNotesValue != "" || (*submit && !isPublishBuildProcessed(buildResp)) {
				processedBuildResp, waitErr := waitForPublishBuildProcessingFn(requestCtx, client, buildResp.Data.ID, *pollInterval)
				if processedBuildResp != nil && strings.TrimSpace(processedBuildResp.Data.ID) == strings.TrimSpace(buildResp.Data.ID) {
					buildResp = processedBuildResp
					result.ProcessingState = buildResp.Data.Attributes.ProcessingState
				}
				if waitErr != nil {
					return reportPartialFailure(publishFailureStageBuildProcessing, fmt.Errorf("publish testflight: %w", waitErr))
				}
				completedStages = append(completedStages, publishCompletedStageBuildProcessing)
			}

			if *uploadOnly {
				attachTestFlightLocalPublishResult(result, localBuildResult)
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if testNotesValue != "" {
				if _, err := shared.UpsertBetaBuildLocalization(requestCtx, client, buildResp.Data.ID, localeValue, testNotesValue); err != nil {
					recoveryErr := shared.NewTestNotesRecoveryError(buildResp.Data.ID, localeValue, testNotesValue, err)
					result.Recovery = recoveryErr.Recovery()
					return reportPartialFailure(publishFailureStageTestNotes, fmt.Errorf("publish testflight: %w", recoveryErr))
				}
				completedStages = append(completedStages, publishCompletedStageTestNotes)
			}

			addOptions := shared.AddBuildBetaGroupsOptions{
				// Apple requires Xcode Cloud builds to be added to internal groups manually,
				// so only skip redundant internal-group adds for builds uploaded by this command.
				SkipInternalWithAllBuilds: uploaded,
				Notify:                    *notify,
			}
			var addResult *shared.AddBuildBetaGroupsResult
			if uploaded {
				addResult, err = addUploadedBuildBetaGroupsFn(requestCtx, client, buildResp.Data.ID, resolvedGroups, addOptions)
			} else {
				addResult, err = shared.AddBuildBetaGroups(requestCtx, client, buildResp.Data.ID, resolvedGroups, addOptions)
			}
			if err != nil {
				failureStage := publishFailureStageBetaDistribution
				var processingFailure *postUploadBuildProcessingFailure
				if errors.As(err, &processingFailure) {
					buildResp = processingFailure.build
					result.ProcessingState = buildResp.Data.Attributes.ProcessingState
					return reportPartialFailure(publishFailureStageBuildProcessing, fmt.Errorf("publish testflight: %w", err))
				}
				var partialErr *asc.BuildBetaGroupsPartialError
				if errors.As(err, &partialErr) {
					completedStages = append(completedStages, publishCompletedStageBetaDistribution)
					failureStage = publishFailureStageNotification
				}
				return reportPartialFailure(failureStage, wrapPublishTestFlightAddGroupsError(err))
			}
			completedStages = append(completedStages, publishCompletedStageBetaDistribution)

			var notified *bool
			if *notify {
				value := addResult.NotificationAction == asc.BuildBetaGroupsNotificationActionManual
				notified = &value
			}
			result.Notified = notified
			result.NotificationAction = addResult.NotificationAction

			submissionResult, err := shared.SubmitBuildBetaReviewIfNeeded(requestCtx, client, buildResp.Data.ID, resolvedGroups, addResult.AddedGroupIDs, *submit, "publish testflight")
			if err != nil {
				return reportPartialFailure(publishFailureStageBetaReview, err)
			}
			if submissionResult.Message != "" {
				fmt.Fprintln(os.Stderr, submissionResult.Message)
			}

			var betaReviewSubmitted *bool
			if *submit {
				value := submissionResult.Submitted
				betaReviewSubmitted = &value
			}

			for _, group := range addResult.SkippedInternalAllBuildsGroups {
				fmt.Fprintf(
					os.Stderr,
					"Skipped internal group %q (%s) because it already receives all builds\n",
					group.NameForDisplay(),
					group.ID,
				)
			}
			result.BetaReviewSubmitted = betaReviewSubmitted
			result.BetaReviewSubmissionID = submissionResult.SubmissionID
			attachTestFlightLocalPublishResult(result, localBuildResult)

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// PublishAppStoreCommand uploads a prebuilt artifact or local build, attaches it to an App Store version, and optionally submits it.
func PublishAppStoreCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish appstore", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to prebuilt .ipa file")
	pkgPath := fs.String("pkg", "", "Path to prebuilt macOS .pkg file (requires --version and --build-number)")
	version := fs.String("version", "", "App Store version string (defaults to IPA version; required with --pkg)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (auto-extracted from IPA; required with --pkg)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	metadataDir := fs.String("metadata-dir", "", "Metadata directory with version/<version>/*.json files to apply after ensuring the App Store version")
	submit := fs.Bool("submit", false, "Submit for review after attaching build")
	confirm := fs.Bool("confirm", false, "Confirm submission (required with --submit)")
	dryRun := fs.Bool("dry-run", false, "Preview high-level publish plan without uploading or submitting")
	wait := fs.Bool("wait", false, "Wait for build processing")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and build discovery")
	timeout := fs.Duration("timeout", 0, "Override upload + processing timeout; also applies to submission with --submit (e.g., 30m)")
	localBuild := bindPublishLocalBuildFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "appstore",
		ShortUsage: "asc publish appstore [flags]",
		ShortHelp:  "Canonical App Store upload + submit workflow.",
		LongHelp: `Use this as the canonical high-level App Store publish command.

Workflow:
1. Build locally with Xcode or upload an IPA or macOS PKG
2. Wait for build processing (if --wait)
3. Find or create the App Store version
4. Apply version localization metadata (if --metadata-dir)
5. Attach the build to the version
6. Optionally submit for review with --submit --confirm

Use ` + "`asc release stage`" + ` when you want metadata-driven preparation without
submission. Use ` + "`asc validate`" + ` to run readiness checks before you add
` + "`--submit`" + `. Use ` + "`--dry-run`" + ` to preview the planned mutation
sequence without uploading or submitting.

Examples:
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3
  asc publish appstore --app "123" --pkg MacApp.pkg --version 1.2.3 --build-number 42
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --metadata-dir ./metadata --submit --confirm
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --dry-run
  asc publish appstore --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3
  asc publish appstore --app "123" --workspace MacApp.xcworkspace --scheme MacApp --version 1.2.3 --platform MAC_OS --pkg-path .asc/artifacts/MacApp.pkg
  asc publish appstore --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3 --signing-style manual --team-id TEAM_ID
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *submit && !*confirm && !*dryRun {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --submit")
				return shared.MissingRequiredUsageError("--confirm")
			}

			resolvedAppInput := shared.ResolveAppID(*appID)
			if resolvedAppInput == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			setFlags := collectSetFlags(fs)
			ipaValue := strings.TrimSpace(*ipaPath)
			pkgValue := strings.TrimSpace(*pkgPath)
			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			metadataDirValue := strings.TrimSpace(*metadataDir)
			localBuildMode := localBuild.localBuildMode()
			if err := validateLocalBuildFlagUsage(localBuildMode, setFlags); err != nil {
				return err
			}
			if localBuildMode {
				if err := validatePublishExportOptionsFlags(localBuild, setFlags); err != nil {
					return err
				}
				if err := validatePublishExportXcodebuildArgs(localBuild.exportXcodebuildArg); err != nil {
					return err
				}
			}
			if setFlags["metadata-dir"] && metadataDirValue == "" {
				return shared.UsageError("--metadata-dir cannot be empty")
			}
			if ipaValue != "" && pkgValue != "" {
				return shared.UsageError("--ipa and --pkg are mutually exclusive")
			}
			switch {
			case localBuildMode:
				if err := validateLocalBuildSelectors(localBuild); err != nil {
					return err
				}
				if ipaValue != "" {
					return shared.UsageError("--ipa cannot be combined with --workspace or --project")
				}
				if pkgValue != "" {
					return shared.UsageError("--pkg cannot be combined with --workspace or --project")
				}
				if versionValue == "" {
					return shared.UsageError("--version is required")
				}
			case ipaValue == "" && pkgValue == "":
				fmt.Fprintf(os.Stderr, "Error: --ipa or --pkg is required\n\n")
				return shared.MissingRequiredUsageError("")
			case pkgValue != "":
				if err := validatePublishPKGMetadata(versionValue, buildNumberValue); err != nil {
					return err
				}
			}
			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedPlatform, err = validatePublishPrebuiltArtifactPlatform(ipaValue, pkgValue, normalizedPlatform, setFlags["platform"])
			if err != nil {
				return err
			}
			if localBuildMode {
				if err := validateLocalBuildArtifactFlags(localBuild, setFlags, normalizedPlatform); err != nil {
					return err
				}
			}

			var fileInfo os.FileInfo
			if ipaValue != "" || pkgValue != "" {
				if pkgValue != "" {
					fileInfo, err = validatePublishPKGPathFn(pkgValue)
				} else {
					fileInfo, err = validatePublishIPAPathFn(ipaValue)
					if err == nil {
						versionValue, buildNumberValue, err = shared.ResolveBundleInfoForIPA(ipaValue, *version, *buildNumber)
					}
				}
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
			}

			var metadataValuesByLocale map[string]map[string]string
			if metadataDirValue != "" {
				metadataValuesByLocale, err = loadPublishVersionMetadataValues(metadataDirValue, versionValue)
				if err != nil {
					return shared.UsageErrorf("--metadata-dir %q: %v", metadataDirValue, err)
				}
				if err := shared.ValidateVersionLocalizationValueSet(metadataValuesByLocale); err != nil {
					return shared.UsageErrorf("--metadata-dir %q: %v", metadataDirValue, err)
				}
			}

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			mode := asc.PublishModeIPAUpload
			if pkgValue != "" {
				mode = asc.PublishModePKGUpload
			}
			var localBuildConfig publishLocalBuildConfig
			timeoutValue := resolvePublishTimeout(*timeout)
			newPublishRequestCtx := func() (context.Context, context.CancelFunc) {
				return shared.ContextWithTimeoutDuration(ctx, timeoutValue)
			}

			var client *asc.Client
			var requestCtx context.Context
			var cancel context.CancelFunc
			resolvedPublishAppID := resolvedAppInput

			if *dryRun && canPlanAppStorePublishWithoutASC(resolvedPublishAppID, localBuildMode, buildNumberValue) {
				if localBuildMode {
					localBuildConfig, err = resolveLocalBuildConfig(localBuild, normalizedPlatform, versionValue, buildNumberValue)
					if err != nil {
						return fmt.Errorf("publish appstore: %w", err)
					}
					mode = asc.PublishModeLocalBuild
				}
			} else {
				client, err = getPublishASCClientFn(timeoutValue)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				requestCtx, cancel = newPublishRequestCtx()
				if !localBuildMode {
					defer cancel()
				}

				preflightCtx := requestCtx
				if localBuildMode {
					cancel()
					var preflightCancel context.CancelFunc
					preflightCtx, preflightCancel = newPublishRequestCtx()
					defer preflightCancel()
				}
				resolvedPublishAppID, err = resolvePublishAppIDWithLookupFn(preflightCtx, client, resolvedPublishAppID)
				if err != nil {
					return fmt.Errorf("publish appstore: resolve app: %w", err)
				}

				if localBuildMode {
					buildNumberValue, err = resolvePublishBuildNumber(preflightCtx, client, resolvedPublishAppID, versionValue, normalizedPlatform, localBuild, buildNumberValue)
					if err != nil {
						return fmt.Errorf("publish appstore: %w", err)
					}
					localBuildConfig, err = resolveLocalBuildConfig(localBuild, normalizedPlatform, versionValue, buildNumberValue)
					if err != nil {
						return fmt.Errorf("publish appstore: %w", err)
					}
					mode = asc.PublishModeLocalBuild
				}
			}

			if *dryRun {
				result := plannedAppStorePublishResult(mode, versionValue, buildNumberValue, *wait, *submit, metadataDirValue != "", localBuildMode, localBuildConfig)
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			var buildResp *asc.BuildResponse
			uploaded := false
			var localBuildResult *publishLocalBuildExecutionResult
			if localBuildMode {
				localBuildResult, err = runPublishLocalBuild(ctx, client, resolvedPublishAppID, normalizedPlatform, versionValue, buildNumberValue, *pollInterval, timeoutValue, timeoutOverride, localBuildConfig)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				buildResp = localBuildResult.Build
				versionValue = localBuildResult.Version
				buildNumberValue = localBuildResult.BuildNumber
				uploaded = localBuildResult.Uploaded
			} else {
				uploadArtifact := uploadBuildAndWaitForIDFn
				artifactPath := ipaValue
				if pkgValue != "" {
					uploadArtifact = uploadPKGBuildAndWaitForIDFn
					artifactPath = pkgValue
				}
				uploadResult, err := uploadArtifact(requestCtx, client, resolvedPublishAppID, artifactPath, fileInfo, versionValue, buildNumberValue, platformValue, *pollInterval, timeoutValue, timeoutOverride)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				buildResp = uploadResult.Build
				versionValue = uploadResult.Version
				buildNumberValue = uploadResult.BuildNumber
				uploaded = true
			}

			if *wait {
				cancel()
				requestCtx, cancel = newPublishRequestCtx()
				defer cancel()
				buildResp, err = waitForPublishBuildProcessingFn(requestCtx, client, buildResp.Data.ID, *pollInterval)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
			}

			versionResp, err := findOrCreatePublishAppStoreVersion(ctx, client, resolvedPublishAppID, versionValue, platformValue)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			if metadataDirValue != "" {
				_, metadataErr := applyPublishVersionMetadataFn(ctx, client, publishVersionMetadataOptions{
					VersionID:      versionResp.Data.ID,
					Version:        versionValue,
					Dir:            metadataDirValue,
					ValuesByLocale: metadataValuesByLocale,
				})
				if metadataErr != nil {
					if shared.IsLocalizationInputError(metadataErr) {
						return shared.UsageErrorf("--metadata-dir %q: %v", metadataDirValue, metadataErr)
					}
					return fmt.Errorf("publish appstore: apply metadata: %w", metadataErr)
				}
			}

			resolvedBuildNumberValue := firstNonEmpty(strings.TrimSpace(buildResp.Data.Attributes.Version), buildNumberValue)

			result := &asc.AppStorePublishResult{
				Mode:         mode,
				BuildVersion: versionValue,
				BuildNumber:  resolvedBuildNumberValue,
				BuildID:      buildResp.Data.ID,
				VersionID:    versionResp.Data.ID,
				Uploaded:     uploaded,
				Attached:     false,
				Submitted:    false,
			}

			attachLocalPublishResult := func() {
				if localBuildResult == nil {
					return
				}
				result.Archive = localBuildResult.Archive
				result.Export = localBuildResult.Export
				result.Publish = &asc.AppStorePublishStageResult{
					BuildVersion: result.BuildVersion,
					BuildNumber:  result.BuildNumber,
					BuildID:      result.BuildID,
					VersionID:    result.VersionID,
					SubmissionID: result.SubmissionID,
					Uploaded:     result.Uploaded,
					Attached:     result.Attached,
					Submitted:    result.Submitted,
				}
			}

			cancel()
			requestCtx, cancel = newPublishRequestCtx()
			defer cancel()

			submitCtx := requestCtx
			submitRequestTimeout := time.Duration(0)
			if *submit && *timeout > 0 {
				submitCtx = ctx
				submitRequestTimeout = timeoutValue
			}

			if *submit {
				existingSubmissionID, err := submitcli.LookupExistingSubmissionForVersion(
					submitCtx,
					client,
					versionResp.Data.ID,
					submitRequestTimeout,
				)
				if err != nil {
					return fmt.Errorf("publish appstore: failed to lookup existing submission: %w", err)
				}
				if existingSubmissionID != "" {
					result.SubmissionID = existingSubmissionID
					result.Submitted = true
					attachLocalPublishResult()
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
			}

			attachResult, err := submitcli.EnsureBuildAttached(ctx, client, versionResp.Data.ID, buildResp.Data.ID, false)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}
			result.Attached = attachResult.Attached || attachResult.AlreadyAttached

			if *submit {
				if submitRequestTimeout == 0 {
					cancel()
					requestCtx, cancel = newPublishRequestCtx()
					defer cancel()
					submitCtx = requestCtx
				}

				localizationPreflight := func() error {
					if submitRequestTimeout > 0 {
						return submitcli.SubmissionLocalizationPreflightWithTimeout(
							submitCtx,
							client,
							resolvedPublishAppID,
							versionResp.Data.ID,
							normalizedPlatform,
							submitRequestTimeout,
							"asc publish appstore --submit",
						)
					}
					return submitcli.SubmissionLocalizationPreflight(
						submitCtx,
						client,
						resolvedPublishAppID,
						versionResp.Data.ID,
						normalizedPlatform,
						"asc publish appstore --submit",
					)
				}
				if err := localizationPreflight(); err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}

				if submitRequestTimeout > 0 {
					submitcli.SubmissionSubscriptionPreflightWithTimeout(
						submitCtx,
						client,
						resolvedPublishAppID,
						submitRequestTimeout,
						"asc publish appstore --submit",
					)
				} else {
					submitcli.SubmissionSubscriptionPreflight(submitCtx, client, resolvedPublishAppID, "asc publish appstore --submit")
				}

				submitResult, err := submitcli.SubmitResolvedVersion(submitCtx, client, submitcli.SubmitResolvedVersionOptions{
					AppID:                    resolvedPublishAppID,
					VersionID:                versionResp.Data.ID,
					BuildID:                  buildResp.Data.ID,
					Platform:                 normalizedPlatform,
					RequestTimeout:           submitRequestTimeout,
					EnsureBuildAttached:      false,
					LookupExistingSubmission: false,
					DryRun:                   false,
					Emit: func(message string) {
						fmt.Fprintln(os.Stderr, message)
					},
				})
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				result.SubmissionID = submitResult.SubmissionID
				result.Submitted = submitResult.SubmissionID != ""
			}
			attachLocalPublishResult()

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func plannedAppStorePublishResult(mode asc.PublishMode, version, buildNumber string, wait, submit, applyMetadata, localBuildMode bool, localBuildConfig publishLocalBuildConfig) *asc.AppStorePublishResult {
	result := &asc.AppStorePublishResult{
		Mode:         mode,
		DryRun:       true,
		BuildVersion: version,
		BuildNumber:  buildNumber,
		Uploaded:     false,
		Attached:     false,
		Submitted:    false,
		Plan:         plannedAppStorePublishSteps(localBuildMode, mode == asc.PublishModePKGUpload || localBuildConfig.PKGPath != "", wait, submit, applyMetadata),
	}

	if !localBuildMode {
		return result
	}

	result.Archive = &asc.PublishArchiveStageResult{
		ArchivePath:   localBuildConfig.ArchivePath,
		Version:       version,
		BuildNumber:   buildNumber,
		Scheme:        localBuildConfig.Scheme,
		Configuration: localBuildConfig.Configuration,
	}
	result.Export = &asc.PublishExportStageResult{
		ArchivePath:       localBuildConfig.ArchivePath,
		IPAPath:           localBuildConfig.IPAPath,
		PKGPath:           localBuildConfig.PKGPath,
		Version:           version,
		BuildNumber:       buildNumber,
		ExportOptionsPath: localBuildConfig.ExportOptionsPath,
		DirectUpload:      false,
	}

	return result
}

func plannedAppStorePublishSteps(localBuildMode, pkgArtifact, wait, submit, applyMetadata bool) []asc.PublishPlanStep {
	steps := make([]asc.PublishPlanStep, 0, 8)
	if localBuildMode {
		artifactName := "IPA"
		if pkgArtifact {
			artifactName = "PKG"
		}
		steps = append(
			steps,
			newPublishPlanStep(publishPlanStepArchiveLocalBuild, "Archive the selected Xcode workspace or project to a local .xcarchive."),
			newPublishPlanStep(publishPlanStepExportLocalBuild, "Export the archive to a local App Store "+artifactName+" artifact."),
		)
	}

	artifactName := "IPA"
	if pkgArtifact {
		artifactName = "PKG"
	}
	steps = append(steps, newPublishPlanStep(publishPlanStepUploadBuild, "Upload the "+artifactName+" to App Store Connect and wait for the build record to appear."))
	if wait {
		steps = append(steps, newPublishPlanStep(publishPlanStepWaitForBuildProcessing, "Wait for App Store Connect build processing to reach a terminal state."))
	}
	steps = append(steps, newPublishPlanStep(publishPlanStepEnsureVersion, "Find the requested App Store version or create it if missing."))
	if applyMetadata {
		steps = append(steps, newPublishPlanStep(publishPlanStepApplyMetadata, "Apply version localization metadata from --metadata-dir."))
	}
	steps = append(steps, newPublishPlanStep(publishPlanStepAttachBuild, "Attach the resolved build to the App Store version."))
	if submit {
		steps = append(steps, newPublishPlanStep(publishPlanStepSubmitReview, "Run submission preflight and submit the version for App Store review."))
	}

	return steps
}

func canPlanAppStorePublishWithoutASC(appID string, localBuildMode bool, buildNumber string) bool {
	if !shared.IsNumericAppID(strings.TrimSpace(appID)) {
		return false
	}
	if !localBuildMode {
		return true
	}
	return strings.TrimSpace(buildNumber) != ""
}

func isPublishBuildProcessed(buildResp *asc.BuildResponse) bool {
	if buildResp == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState), asc.BuildProcessingStateValid)
}

func newPublishPlanStep(name, message string) asc.PublishPlanStep {
	return asc.PublishPlanStep{
		Name:    name,
		Status:  publishPlanStatusPlanned,
		Message: message,
	}
}

type publishUploadResult struct {
	Build       *asc.BuildResponse
	Version     string
	BuildNumber string
}

func uploadBuildAndWaitForID(ctx context.Context, client *asc.Client, appID, ipaPath string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval time.Duration, uploadTimeout time.Duration, overrideUploadTimeout bool) (*publishUploadResult, error) {
	return uploadBuildArtifactAndWaitForID(ctx, client, appID, ipaPath, fileInfo, version, buildNumber, platform, asc.UTIIPA, pollInterval, uploadTimeout, overrideUploadTimeout)
}

func uploadPKGBuildAndWaitForID(ctx context.Context, client *asc.Client, appID, pkgPath string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval time.Duration, uploadTimeout time.Duration, overrideUploadTimeout bool) (*publishUploadResult, error) {
	return uploadBuildArtifactAndWaitForID(ctx, client, appID, pkgPath, fileInfo, version, buildNumber, platform, asc.UTIPKG, pollInterval, uploadTimeout, overrideUploadTimeout)
}

func uploadBuildArtifactAndWaitForID(ctx context.Context, client *asc.Client, appID, artifactPath string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform, fileUTI asc.UTI, pollInterval time.Duration, uploadTimeout time.Duration, overrideUploadTimeout bool) (*publishUploadResult, error) {
	var (
		artifactFile *os.File
		openedInfo   os.FileInfo
		err          error
	)
	if fileUTI == asc.UTIPKG {
		artifactFile, openedInfo, err = shared.OpenValidatedPKGPath(artifactPath)
	} else {
		artifactFile, openedInfo, err = shared.OpenValidatedIPAPath(artifactPath)
	}
	if err != nil {
		return nil, err
	}
	defer artifactFile.Close()
	if fileInfo != nil && !os.SameFile(fileInfo, openedInfo) {
		return nil, fmt.Errorf("build artifact changed before upload")
	}

	uploadResp, fileResp, err := shared.PrepareBuildUpload(ctx, client, appID, openedInfo, version, buildNumber, platform, fileUTI)
	if err != nil {
		return nil, err
	}

	if len(fileResp.Data.Attributes.UploadOperations) == 0 {
		return nil, fmt.Errorf("no upload operations returned")
	}

	fmt.Fprintf(os.Stderr, "Uploading %s (%d bytes) to App Store Connect...\n", fileInfo.Name(), fileInfo.Size())
	uploadCtx, uploadCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	err = asc.ExecuteUploadOperationsFromFile(uploadCtx, artifactFile, fileResp.Data.Attributes.UploadOperations)
	uploadCancel()
	if err != nil {
		return nil, err
	}

	commitCtx, commitCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	_, err = shared.CommitBuildUploadFile(commitCtx, client, uploadResp.Data.ID, fileResp.Data.ID, nil)
	commitCancel()
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, "Upload committed in App Store Connect.")
	fmt.Fprintf(os.Stderr, "Waiting for build %s (%s) to appear in App Store Connect...\n", buildNumber, version)
	buildResp, err := shared.WaitForBuildByNumberOrUploadFailure(ctx, client, appID, uploadResp.Data.ID, version, buildNumber, string(platform), pollInterval)
	if err != nil {
		return nil, err
	}

	return &publishUploadResult{
		Build:       buildResp,
		Version:     version,
		BuildNumber: buildNumber,
	}, nil
}

func findPublishBuildByNumber(ctx context.Context, client *asc.Client, appID, buildNumber, platform string) (*asc.BuildResponse, error) {
	buildNumber = strings.TrimSpace(buildNumber)
	if buildNumber == "" {
		return nil, fmt.Errorf("build number is required")
	}

	opts := []asc.BuildsOption{
		asc.WithBuildsBuildNumber(buildNumber),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(1),
		asc.WithBuildsProcessingStates([]string{
			asc.BuildProcessingStateProcessing,
			asc.BuildProcessingStateFailed,
			asc.BuildProcessingStateInvalid,
			asc.BuildProcessingStateValid,
		}),
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}

	buildsResp, err := client.GetBuilds(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	if len(buildsResp.Data) == 0 {
		return nil, fmt.Errorf("no build found for app %q with build number %q", appID, buildNumber)
	}

	return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
}

func validatePublishPKGMetadata(version, buildNumber string) error {
	missingFlags := make([]string, 0, 2)
	if strings.TrimSpace(version) == "" {
		missingFlags = append(missingFlags, "--version")
	}
	if strings.TrimSpace(buildNumber) == "" {
		missingFlags = append(missingFlags, "--build-number")
	}
	if len(missingFlags) > 0 {
		return shared.UsageErrorf("%s required for PKG uploads", strings.Join(missingFlags, " and "))
	}
	return nil
}

func validatePublishPrebuiltArtifactPlatform(ipaPath, pkgPath, platform string, platformWasSet bool) (string, error) {
	if strings.TrimSpace(ipaPath) != "" && platform == string(asc.PlatformMacOS) {
		fmt.Fprintln(os.Stderr, "Warning: --ipa with --platform MAC_OS is deprecated and will be removed in a future major release. Use --pkg with --version and --build-number for macOS uploads.")
		return platform, nil
	}
	if strings.TrimSpace(pkgPath) == "" {
		return platform, nil
	}
	if platformWasSet && platform != string(asc.PlatformMacOS) {
		return "", shared.UsageErrorf("--platform %s does not match PKG platform MAC_OS", platform)
	}
	return string(asc.PlatformMacOS), nil
}

func resolvePublishTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return asc.ResolveTimeoutWithDefault(publishDefaultTimeout)
}

func contextWithPublishUploadTimeout(ctx context.Context, timeout time.Duration, override bool) (context.Context, context.CancelFunc) {
	if override {
		return shared.ContextWithTimeoutDuration(ctx, timeout)
	}
	return shared.ContextWithUploadTimeout(ctx)
}

func wrapPublishTestFlightAddGroupsError(err error) error {
	var partialErr *asc.BuildBetaGroupsPartialError
	if errors.As(err, &partialErr) {
		return fmt.Errorf("publish testflight: %w", err)
	}
	return fmt.Errorf("publish testflight: failed to add groups: %w", err)
}

func attachTestFlightLocalPublishResult(result *asc.TestFlightPublishResult, localBuildResult *publishLocalBuildExecutionResult) {
	if result == nil || localBuildResult == nil {
		return
	}
	result.Archive = localBuildResult.Archive
	result.Export = localBuildResult.Export
	result.Publish = &asc.TestFlightPublishStageResult{
		BuildID:                result.BuildID,
		BuildVersion:           result.BuildVersion,
		BuildNumber:            result.BuildNumber,
		GroupIDs:               append([]string(nil), result.GroupIDs...),
		Uploaded:               result.Uploaded,
		UploadOnly:             result.UploadOnly,
		ProcessingState:        result.ProcessingState,
		Notified:               result.Notified,
		NotificationAction:     result.NotificationAction,
		BetaReviewSubmitted:    result.BetaReviewSubmitted,
		BetaReviewSubmissionID: result.BetaReviewSubmissionID,
	}
}
