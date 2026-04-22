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
	publishPlanStepAttachBuild            = "attach_build"
	publishPlanStepSubmitReview           = "submit_review"
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

// PublishTestFlightCommand uploads an IPA and distributes it to TestFlight groups.
func PublishTestFlightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish testflight", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (required unless --build/--build-number is provided)")
	buildID := fs.String("build", "", "Existing build ID to distribute (skip upload)")
	version := fs.String("version", "", "CFBundleShortVersionString (auto-extracted from IPA if not provided)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (used for upload metadata with --ipa, or build lookup when --ipa is omitted)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	groupIDs := fs.String("group", "", "Beta group ID(s) or name(s), comma-separated")
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
		LongHelp: `Upload or local-build a binary and distribute it to TestFlight beta groups.

Steps:
1. Build locally with Xcode or upload an IPA (unless --build/--build-number is provided)
2. Wait for processing (if --wait)
3. Add build to specified beta groups
4. Optionally notify testers
5. Optionally submit for beta app review with --submit --confirm

Examples:
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID"
  asc publish testflight --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3 --group "GROUP_ID"
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers"
  asc publish testflight --app "123" --ipa app.ipa --group "G1,G2" --wait --notify
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers" --submit --confirm
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID" --test-notes "Test instructions" --locale "en-US" --wait
  asc publish testflight --app "123" --build "BUILD_ID" --group "GROUP_ID" --wait
  asc publish testflight --app "123" --build-number "42" --group "GROUP_ID" --wait`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppInput := shared.ResolveAppID(*appID)
			if resolvedAppInput == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			setFlags := collectSetFlags(fs)
			ipaValue := strings.TrimSpace(*ipaPath)
			buildIDValue := strings.TrimSpace(*buildID)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			versionValue := strings.TrimSpace(*version)
			localBuildMode := localBuild.localBuildMode()
			if err := validateLocalBuildFlagUsage(localBuildMode, setFlags); err != nil {
				return err
			}

			uploadMode := ipaValue != ""
			switch {
			case localBuildMode:
				if err := validateLocalBuildSelectors(localBuild); err != nil {
					return err
				}
				if uploadMode {
					return shared.UsageError("--ipa cannot be combined with --workspace or --project")
				}
				if buildIDValue != "" {
					return shared.UsageError("--build cannot be combined with --workspace or --project")
				}
				if versionValue == "" {
					return shared.UsageError("--version is required")
				}
			case uploadMode:
				if buildIDValue != "" {
					return shared.UsageError("--ipa and --build are mutually exclusive")
				}
			default:
				if buildIDValue == "" && buildNumberValue == "" {
					return shared.UsageError("--ipa is required unless --build or --build-number is provided")
				}
				if buildIDValue != "" && buildNumberValue != "" {
					return shared.UsageError("--build and --build-number are mutually exclusive when --ipa is not provided")
				}
				if versionValue != "" {
					return shared.UsageError("--version is only supported when --ipa is provided")
				}
			}

			parsedGroupIDs := shared.SplitCSV(*groupIDs)
			if len(parsedGroupIDs) == 0 {
				fmt.Fprintf(os.Stderr, "Error: --group is required\n\n")
				return flag.ErrHelp
			}
			if *submit && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --submit")
				return flag.ErrHelp
			}
			if *confirm && !*submit {
				fmt.Fprintln(os.Stderr, "Error: --confirm requires --submit")
				return flag.ErrHelp
			}

			testNotesValue := strings.TrimSpace(*testNotes)
			localeValue := strings.TrimSpace(*locale)
			if testNotesValue != "" && localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required with --test-notes")
				return flag.ErrHelp
			}
			if testNotesValue == "" && localeValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --test-notes is required with --locale")
				return flag.ErrHelp
			}
			if testNotesValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return shared.UsageError(err.Error())
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

			var uploadFileInfo os.FileInfo
			uploadVersionValue := ""
			uploadBuildNumberValue := ""
			if uploadMode {
				uploadFileInfo, err = validatePublishIPAPathFn(ipaValue)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}

				uploadVersionValue, uploadBuildNumberValue, err = shared.ResolveBundleInfoForIPA(ipaValue, *version, *buildNumber)
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

			groupLookupCtx := preflightCtx
			resolvedGroups, err := resolvePublishBetaGroups(groupLookupCtx, client, resolvedPublishAppID, parsedGroupIDs)
			if err != nil {
				return fmt.Errorf("publish testflight: %w", err)
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
				uploadResult, err := uploadBuildAndWaitForIDFn(
					requestCtx,
					client,
					resolvedPublishAppID,
					ipaValue,
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
				mode = asc.PublishModeIPAUpload
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

			if *wait || testNotesValue != "" {
				buildResp, err = waitForPublishBuildProcessingFn(requestCtx, client, buildResp.Data.ID, *pollInterval)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			if testNotesValue != "" {
				if _, err := shared.UpsertBetaBuildLocalization(requestCtx, client, buildResp.Data.ID, localeValue, testNotesValue); err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			addResult, err := shared.AddBuildBetaGroups(requestCtx, client, buildResp.Data.ID, resolvedGroups, shared.AddBuildBetaGroupsOptions{
				// Apple requires Xcode Cloud builds to be added to internal groups manually,
				// so only skip redundant internal-group adds for builds uploaded by this command.
				SkipInternalWithAllBuilds: uploaded,
				Notify:                    *notify,
			})
			if err != nil {
				return wrapPublishTestFlightAddGroupsError(err)
			}

			submissionResult, err := shared.SubmitBuildBetaReviewIfNeeded(requestCtx, client, buildResp.Data.ID, resolvedGroups, addResult.AddedGroupIDs, *submit, "publish testflight")
			if err != nil {
				return err
			}
			if submissionResult.Message != "" {
				fmt.Fprintln(os.Stderr, submissionResult.Message)
			}

			var notified *bool
			if *notify {
				value := addResult.NotificationAction == asc.BuildBetaGroupsNotificationActionManual
				notified = &value
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

			result := &asc.TestFlightPublishResult{
				Mode:                   mode,
				BuildID:                buildResp.Data.ID,
				BuildVersion:           resolvedVersionValue,
				BuildNumber:            resolvedBuildNumberValue,
				GroupIDs:               resolvedPublishBetaGroupIDs(resolvedGroups),
				Uploaded:               uploaded,
				ProcessingState:        buildResp.Data.Attributes.ProcessingState,
				Notified:               notified,
				NotificationAction:     addResult.NotificationAction,
				BetaReviewSubmitted:    betaReviewSubmitted,
				BetaReviewSubmissionID: submissionResult.SubmissionID,
			}
			if localBuildResult != nil {
				result.Archive = localBuildResult.Archive
				result.Export = localBuildResult.Export
				result.Publish = &asc.TestFlightPublishStageResult{
					BuildID:                result.BuildID,
					BuildVersion:           result.BuildVersion,
					BuildNumber:            result.BuildNumber,
					GroupIDs:               append([]string(nil), result.GroupIDs...),
					Uploaded:               result.Uploaded,
					ProcessingState:        result.ProcessingState,
					Notified:               result.Notified,
					NotificationAction:     result.NotificationAction,
					BetaReviewSubmitted:    result.BetaReviewSubmitted,
					BetaReviewSubmissionID: result.BetaReviewSubmissionID,
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// PublishAppStoreCommand uploads an IPA, attaches it to an App Store version, and optionally submits it.
func PublishAppStoreCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish appstore", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (required unless local-build mode is used)")
	version := fs.String("version", "", "App Store version string (defaults to IPA version)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (auto-extracted from IPA if not provided)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	submit := fs.Bool("submit", false, "Submit for review after attaching build")
	confirm := fs.Bool("confirm", false, "Confirm submission (required with --submit)")
	dryRun := fs.Bool("dry-run", false, "Preview high-level publish plan without uploading or submitting")
	wait := fs.Bool("wait", false, "Wait for build processing")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and build discovery")
	timeout := fs.Duration("timeout", 0, "Override upload + processing timeout (e.g., 30m)")
	localBuild := bindPublishLocalBuildFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "appstore",
		ShortUsage: "asc publish appstore [flags]",
		ShortHelp:  "Canonical App Store upload + submit workflow.",
		LongHelp: `Use this as the canonical high-level App Store publish command.

Workflow:
1. Build locally with Xcode or upload an IPA
2. Wait for build processing (if --wait)
3. Find or create the App Store version
4. Attach the build to the version
5. Optionally submit for review with --submit --confirm

Use ` + "`asc release stage`" + ` when you want metadata-driven preparation without
submission. Use ` + "`asc validate`" + ` to run readiness checks before you add
` + "`--submit`" + `. Use ` + "`--dry-run`" + ` to preview the planned mutation
sequence without uploading or submitting.

Examples:
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --dry-run
  asc publish appstore --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *submit && !*confirm && !*dryRun {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --submit")
				return flag.ErrHelp
			}

			resolvedAppInput := shared.ResolveAppID(*appID)
			if resolvedAppInput == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			setFlags := collectSetFlags(fs)
			ipaValue := strings.TrimSpace(*ipaPath)
			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			localBuildMode := localBuild.localBuildMode()
			if err := validateLocalBuildFlagUsage(localBuildMode, setFlags); err != nil {
				return err
			}
			switch {
			case localBuildMode:
				if err := validateLocalBuildSelectors(localBuild); err != nil {
					return err
				}
				if ipaValue != "" {
					return shared.UsageError("--ipa cannot be combined with --workspace or --project")
				}
				if versionValue == "" {
					return shared.UsageError("--version is required")
				}
			case ipaValue == "":
				fmt.Fprintf(os.Stderr, "Error: --ipa is required\n\n")
				return flag.ErrHelp
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

			var fileInfo os.FileInfo
			if ipaValue != "" {
				fileInfo, err = validatePublishIPAPathFn(ipaValue)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}

				versionValue, buildNumberValue, err = shared.ResolveBundleInfoForIPA(ipaValue, *version, *buildNumber)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
			}

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			mode := asc.PublishModeIPAUpload
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
				result := plannedAppStorePublishResult(mode, versionValue, buildNumberValue, *wait, *submit, localBuildMode, localBuildConfig)
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
				requestCtx, cancel = newPublishRequestCtx()
				defer cancel()
				buildResp = localBuildResult.Build
				versionValue = localBuildResult.Version
				buildNumberValue = localBuildResult.BuildNumber
				uploaded = localBuildResult.Uploaded
			} else {
				uploadResult, err := uploadBuildAndWaitForIDFn(requestCtx, client, resolvedPublishAppID, ipaValue, fileInfo, versionValue, buildNumberValue, platformValue, *pollInterval, timeoutValue, timeoutOverride)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				buildResp = uploadResult.Build
				versionValue = uploadResult.Version
				buildNumberValue = uploadResult.BuildNumber
				uploaded = true
			}

			if *wait {
				buildResp, err = waitForPublishBuildProcessingFn(requestCtx, client, buildResp.Data.ID, *pollInterval)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
			}

			versionResp, err := client.FindOrCreateAppStoreVersion(requestCtx, resolvedPublishAppID, versionValue, platformValue)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
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

			attachResult, err := submitcli.EnsureBuildAttached(requestCtx, client, versionResp.Data.ID, buildResp.Data.ID, false)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}
			result.Attached = attachResult.Attached || attachResult.AlreadyAttached

			if *submit {

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

func plannedAppStorePublishResult(mode asc.PublishMode, version, buildNumber string, wait, submit, localBuildMode bool, localBuildConfig publishLocalBuildConfig) *asc.AppStorePublishResult {
	result := &asc.AppStorePublishResult{
		Mode:         mode,
		DryRun:       true,
		BuildVersion: version,
		BuildNumber:  buildNumber,
		Uploaded:     false,
		Attached:     false,
		Submitted:    false,
		Plan:         plannedAppStorePublishSteps(localBuildMode, wait, submit),
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
		Version:           version,
		BuildNumber:       buildNumber,
		ExportOptionsPath: localBuildConfig.ExportOptionsPath,
		DirectUpload:      false,
	}

	return result
}

func plannedAppStorePublishSteps(localBuildMode, wait, submit bool) []asc.PublishPlanStep {
	steps := make([]asc.PublishPlanStep, 0, 5)
	if localBuildMode {
		steps = append(steps,
			newPublishPlanStep(publishPlanStepArchiveLocalBuild, "Archive the selected Xcode workspace or project to a local .xcarchive."),
			newPublishPlanStep(publishPlanStepExportLocalBuild, "Export the archive to a local App Store IPA artifact."),
		)
	}

	steps = append(steps, newPublishPlanStep(publishPlanStepUploadBuild, "Upload the IPA to App Store Connect and wait for the build record to appear."))
	if wait {
		steps = append(steps, newPublishPlanStep(publishPlanStepWaitForBuildProcessing, "Wait for App Store Connect build processing to reach a terminal state."))
	}
	steps = append(steps,
		newPublishPlanStep(publishPlanStepEnsureVersion, "Find the requested App Store version or create it if missing."),
		newPublishPlanStep(publishPlanStepAttachBuild, "Attach the resolved build to the App Store version."),
	)
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
	uploadResp, fileResp, err := shared.PrepareBuildUpload(ctx, client, appID, fileInfo, version, buildNumber, platform, asc.UTIIPA)
	if err != nil {
		return nil, err
	}

	if len(fileResp.Data.Attributes.UploadOperations) == 0 {
		return nil, fmt.Errorf("no upload operations returned")
	}

	fmt.Fprintf(os.Stderr, "Uploading %s (%d bytes) to App Store Connect...\n", fileInfo.Name(), fileInfo.Size())
	uploadCtx, uploadCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	err = asc.ExecuteUploadOperations(uploadCtx, ipaPath, fileResp.Data.Attributes.UploadOperations)
	uploadCancel()
	if err != nil {
		return nil, err
	}

	commitCtx, commitCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	_, err = shared.CommitBuildUploadFile(commitCtx, client, fileResp.Data.ID, nil)
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

func resolvePublishTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return asc.ResolveTimeoutWithDefault(publishDefaultTimeout)
}

func contextWithPublishUploadTimeout(ctx context.Context, timeout time.Duration, override bool) (context.Context, context.CancelFunc) {
	if override {
		if ctx == nil {
			ctx = context.Background()
		}
		return context.WithTimeout(ctx, timeout)
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
