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
)

const (
	publishDefaultTimeout = 30 * time.Minute
)

// PublishCommand returns the publish command with subcommands.
func PublishCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "publish",
		ShortUsage: "asc publish <subcommand> [flags]",
		ShortHelp:  "End-to-end publish workflows for TestFlight and App Store.",
		LongHelp: `End-to-end publish workflows.

Combines upload, distribution, and submission into single commands.

Examples:
  asc publish testflight --app APP_ID --ipa app.ipa --group GROUP_ID
  asc publish appstore --app APP_ID --ipa app.ipa --version 1.2.3 --submit --confirm`,
		UsageFunc: shared.DefaultUsageFunc,
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

Examples:
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID"
  asc publish testflight --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3 --group "GROUP_ID"
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers"
  asc publish testflight --app "123" --ipa app.ipa --group "G1,G2" --wait --notify
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

			client, err := getPublishASCClientFn()
			if err != nil {
				return fmt.Errorf("publish testflight: %w", err)
			}

			timeoutValue := resolvePublishTimeout(*timeout)
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

			var notified *bool
			if *notify {
				value := addResult.NotificationAction == asc.BuildBetaGroupsNotificationActionManual
				notified = &value
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
				Mode:               mode,
				BuildID:            buildResp.Data.ID,
				BuildVersion:       resolvedVersionValue,
				BuildNumber:        resolvedBuildNumberValue,
				GroupIDs:           resolvedPublishBetaGroupIDs(resolvedGroups),
				Uploaded:           uploaded,
				ProcessingState:    buildResp.Data.Attributes.ProcessingState,
				Notified:           notified,
				NotificationAction: addResult.NotificationAction,
			}
			if localBuildResult != nil {
				result.Archive = localBuildResult.Archive
				result.Export = localBuildResult.Export
				result.Publish = &asc.TestFlightPublishStageResult{
					BuildID:            result.BuildID,
					BuildVersion:       result.BuildVersion,
					BuildNumber:        result.BuildNumber,
					GroupIDs:           append([]string(nil), result.GroupIDs...),
					Uploaded:           result.Uploaded,
					ProcessingState:    result.ProcessingState,
					Notified:           result.Notified,
					NotificationAction: result.NotificationAction,
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// PublishAppStoreCommand uploads an IPA and submits it for App Store review.
func PublishAppStoreCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish appstore", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (required unless local-build mode is used)")
	version := fs.String("version", "", "App Store version string (defaults to IPA version)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (auto-extracted from IPA if not provided)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	submit := fs.Bool("submit", false, "Submit for review after attaching build")
	confirm := fs.Bool("confirm", false, "Confirm submission (required with --submit)")
	wait := fs.Bool("wait", false, "Wait for build processing")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and build discovery")
	timeout := fs.Duration("timeout", 0, "Override upload + processing timeout (e.g., 30m)")
	localBuild := bindPublishLocalBuildFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "appstore",
		ShortUsage: "asc publish appstore [flags]",
		ShortHelp:  "Upload and submit to App Store.",
		LongHelp: `Upload or local-build a binary, attach it to a version, and optionally submit for review.

Steps:
1. Build locally with Xcode or upload an IPA
2. Wait for processing (if --wait)
3. Find or create App Store version
4. Attach build to version
5. Submit for review (if --submit --confirm)

Examples:
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3
  asc publish appstore --app "123" --workspace App.xcworkspace --scheme App --version 1.2.3
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *submit && !*confirm {
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

			client, err := getPublishASCClientFn()
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			timeoutValue := resolvePublishTimeout(*timeout)
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
				return fmt.Errorf("publish appstore: resolve app: %w", err)
			}

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			var buildResp *asc.BuildResponse
			uploaded := false
			var mode asc.PublishMode
			var localBuildResult *publishLocalBuildExecutionResult

			if localBuildMode {
				buildNumberValue, err = resolvePublishBuildNumber(preflightCtx, client, resolvedPublishAppID, versionValue, normalizedPlatform, localBuild, buildNumberValue)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				localBuildConfig, err := resolveLocalBuildConfig(localBuild, normalizedPlatform, versionValue, buildNumberValue)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				localBuildResult, err = runPublishLocalBuild(ctx, client, resolvedPublishAppID, normalizedPlatform, versionValue, buildNumberValue, *pollInterval, timeoutValue, timeoutOverride, localBuildConfig)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				requestCtx, cancel = newPublishRequestCtx()
				defer cancel()
				buildResp = localBuildResult.Build
				versionValue = localBuildResult.Version
				uploaded = localBuildResult.Uploaded
				mode = asc.PublishModeLocalBuild
			} else {
				uploadResult, err := uploadBuildAndWaitForIDFn(requestCtx, client, resolvedPublishAppID, ipaValue, fileInfo, versionValue, buildNumberValue, platformValue, *pollInterval, timeoutValue, timeoutOverride)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
				buildResp = uploadResult.Build
				versionValue = uploadResult.Version
				uploaded = true
				mode = asc.PublishModeIPAUpload
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

			if err := client.AttachBuildToVersion(requestCtx, versionResp.Data.ID, buildResp.Data.ID); err != nil {
				return fmt.Errorf("publish appstore: failed to attach build: %w", err)
			}

			result := &asc.AppStorePublishResult{
				Mode:      mode,
				BuildID:   buildResp.Data.ID,
				VersionID: versionResp.Data.ID,
				Uploaded:  uploaded,
				Attached:  true,
				Submitted: false,
			}

			if *submit {
				submitReq := asc.AppStoreVersionSubmissionCreateRequest{
					Data: asc.AppStoreVersionSubmissionCreateData{
						Type: asc.ResourceTypeAppStoreVersionSubmissions,
						Relationships: &asc.AppStoreVersionSubmissionRelationships{
							AppStoreVersion: &asc.Relationship{
								Data: asc.ResourceData{Type: asc.ResourceTypeAppStoreVersions, ID: versionResp.Data.ID},
							},
						},
					},
				}
				submitResp, err := client.CreateAppStoreVersionSubmission(requestCtx, submitReq)
				if err != nil {
					return fmt.Errorf("publish appstore: failed to submit: %w", err)
				}
				result.SubmissionID = submitResp.Data.ID
				result.Submitted = true
			}
			if localBuildResult != nil {
				result.Archive = localBuildResult.Archive
				result.Export = localBuildResult.Export
				result.Publish = &asc.AppStorePublishStageResult{
					BuildID:      result.BuildID,
					VersionID:    result.VersionID,
					SubmissionID: result.SubmissionID,
					Uploaded:     result.Uploaded,
					Attached:     result.Attached,
					Submitted:    result.Submitted,
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
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

func validateIPAPath(ipaPath string) (os.FileInfo, error) {
	return shared.ValidateIPAPath(ipaPath)
}

func wrapPublishTestFlightAddGroupsError(err error) error {
	var partialErr *asc.BuildBetaGroupsPartialError
	if errors.As(err, &partialErr) {
		return fmt.Errorf("publish testflight: %w", err)
	}
	return fmt.Errorf("publish testflight: failed to add groups: %w", err)
}
