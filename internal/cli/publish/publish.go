package publish

import (
	"context"
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "testflight",
		ShortUsage: "asc publish testflight [flags]",
		ShortHelp:  "Upload and distribute to TestFlight.",
		LongHelp: `Upload IPA and distribute to TestFlight beta groups.

Steps:
1. Upload IPA to App Store Connect (unless --build/--build-number is provided)
2. Wait for processing (if --wait)
3. Add build to specified beta groups
4. Optionally notify testers

Examples:
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID"
  asc publish testflight --app "123" --ipa app.ipa --group "External Testers"
  asc publish testflight --app "123" --ipa app.ipa --group "G1,G2" --wait --notify
  asc publish testflight --app "123" --ipa app.ipa --group "GROUP_ID" --test-notes "Test instructions" --locale "en-US" --wait
  asc publish testflight --app "123" --build "BUILD_ID" --group "GROUP_ID" --wait
  asc publish testflight --app "123" --build-number "42" --group "GROUP_ID" --wait`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			ipaValue := strings.TrimSpace(*ipaPath)
			buildIDValue := strings.TrimSpace(*buildID)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			versionValue := strings.TrimSpace(*version)

			uploadMode := ipaValue != ""
			if uploadMode {
				if buildIDValue != "" {
					return shared.UsageError("--ipa and --build are mutually exclusive")
				}
			} else {
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
				uploadFileInfo, err = validateIPAPath(ipaValue)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}

				uploadVersionValue, uploadBuildNumberValue, err = resolveBundleInfoForIPA(ipaValue, *version, *buildNumber)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("publish testflight: %w", err)
			}

			timeoutValue := resolvePublishTimeout(*timeout)
			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, timeoutValue)
			defer cancel()

			resolvedGroupIDs, err := resolvePublishBetaGroupIDs(requestCtx, client, resolvedAppID, parsedGroupIDs)
			if err != nil {
				return fmt.Errorf("publish testflight: %w", err)
			}

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			uploaded := false
			resolvedVersionValue := ""
			resolvedBuildNumberValue := ""

			var buildResp *asc.BuildResponse
			if uploadMode {
				uploadResult, err := uploadBuildAndWaitForID(
					requestCtx,
					client,
					resolvedAppID,
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
			} else if buildIDValue != "" {
				buildResp, err = client.GetBuild(requestCtx, buildIDValue)
				if err != nil {
					return fmt.Errorf("publish testflight: failed to fetch build: %w", err)
				}
				resolvedBuildNumberValue = strings.TrimSpace(buildResp.Data.Attributes.Version)
			} else {
				buildResp, err = findPublishBuildByNumber(requestCtx, client, resolvedAppID, buildNumberValue, normalizedPlatform)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
				resolvedBuildNumberValue = strings.TrimSpace(buildResp.Data.Attributes.Version)
			}

			if *wait || testNotesValue != "" {
				buildResp, err = client.WaitForBuildProcessing(requestCtx, buildResp.Data.ID, *pollInterval)
				if err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			if testNotesValue != "" {
				if _, err := shared.UpsertBetaBuildLocalization(requestCtx, client, buildResp.Data.ID, localeValue, testNotesValue); err != nil {
					return fmt.Errorf("publish testflight: %w", err)
				}
			}

			if err := client.AddBetaGroupsToBuildWithNotify(requestCtx, buildResp.Data.ID, resolvedGroupIDs, *notify); err != nil {
				return fmt.Errorf("publish testflight: failed to add groups: %w", err)
			}

			result := &asc.TestFlightPublishResult{
				BuildID:         buildResp.Data.ID,
				BuildVersion:    resolvedVersionValue,
				BuildNumber:     resolvedBuildNumberValue,
				GroupIDs:        resolvedGroupIDs,
				Uploaded:        uploaded,
				ProcessingState: buildResp.Data.Attributes.ProcessingState,
				Notified:        *notify,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// PublishAppStoreCommand uploads an IPA and submits it for App Store review.
func PublishAppStoreCommand() *ffcli.Command {
	fs := flag.NewFlagSet("publish appstore", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (required)")
	version := fs.String("version", "", "App Store version string (defaults to IPA version)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (auto-extracted from IPA if not provided)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	submit := fs.Bool("submit", false, "Submit for review after attaching build")
	confirm := fs.Bool("confirm", false, "Confirm submission (required with --submit)")
	wait := fs.Bool("wait", false, "Wait for build processing")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and build discovery")
	timeout := fs.Duration("timeout", 0, "Override upload + processing timeout (e.g., 30m)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "appstore",
		ShortUsage: "asc publish appstore [flags]",
		ShortHelp:  "Upload and submit to App Store.",
		LongHelp: `Upload IPA, attach to version, and optionally submit for review.

Steps:
1. Upload IPA to App Store Connect
2. Wait for processing (if --wait)
3. Find or create App Store version
4. Attach build to version
5. Submit for review (if --submit --confirm)

Examples:
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3
  asc publish appstore --app "123" --ipa app.ipa --version 1.2.3 --submit --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *submit && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --submit")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*ipaPath) == "" {
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

			fileInfo, err := validateIPAPath(*ipaPath)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			versionValue, buildNumberValue, err := resolveBundleInfoForIPA(*ipaPath, *version, *buildNumber)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			timeoutValue := resolvePublishTimeout(*timeout)
			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, timeoutValue)
			defer cancel()

			platformValue := asc.Platform(normalizedPlatform)
			timeoutOverride := *timeout > 0
			uploadResult, err := uploadBuildAndWaitForID(requestCtx, client, resolvedAppID, *ipaPath, fileInfo, versionValue, buildNumberValue, platformValue, *pollInterval, timeoutValue, timeoutOverride)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			buildResp := uploadResult.Build
			if *wait {
				buildResp, err = client.WaitForBuildProcessing(requestCtx, buildResp.Data.ID, *pollInterval)
				if err != nil {
					return fmt.Errorf("publish appstore: %w", err)
				}
			}

			versionResp, err := client.FindOrCreateAppStoreVersion(requestCtx, resolvedAppID, uploadResult.Version, platformValue)
			if err != nil {
				return fmt.Errorf("publish appstore: %w", err)
			}

			if err := client.AttachBuildToVersion(requestCtx, versionResp.Data.ID, buildResp.Data.ID); err != nil {
				return fmt.Errorf("publish appstore: failed to attach build: %w", err)
			}

			result := &asc.AppStorePublishResult{
				BuildID:   buildResp.Data.ID,
				VersionID: versionResp.Data.ID,
				Uploaded:  true,
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
	_, fileResp, err := prepareBuildUpload(ctx, client, appID, fileInfo, version, buildNumber, platform)
	if err != nil {
		return nil, err
	}

	if len(fileResp.Data.Attributes.UploadOperations) == 0 {
		return nil, fmt.Errorf("no upload operations returned")
	}

	uploadCtx, uploadCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	err = asc.ExecuteUploadOperations(uploadCtx, ipaPath, fileResp.Data.Attributes.UploadOperations)
	uploadCancel()
	if err != nil {
		return nil, err
	}

	commitCtx, commitCancel := contextWithPublishUploadTimeout(ctx, uploadTimeout, overrideUploadTimeout)
	err = commitBuildUploadFile(commitCtx, client, fileResp.Data.ID, nil)
	commitCancel()
	if err != nil {
		return nil, err
	}

	buildResp, err := shared.WaitForBuildByNumber(ctx, client, appID, version, buildNumber, string(platform), pollInterval)
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
	fileInfo, err := os.Lstat(ipaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat IPA: %w", err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", ipaPath)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("--ipa must be a file")
	}
	return fileInfo, nil
}

func resolveBundleInfoForIPA(ipaPath, version, buildNumber string) (string, string, error) {
	versionValue := strings.TrimSpace(version)
	buildNumberValue := strings.TrimSpace(buildNumber)
	if versionValue == "" || buildNumberValue == "" {
		info, err := shared.ExtractBundleInfoFromIPA(ipaPath)
		if err != nil {
			missingFlags := make([]string, 0, 2)
			if versionValue == "" {
				missingFlags = append(missingFlags, "--version")
			}
			if buildNumberValue == "" {
				missingFlags = append(missingFlags, "--build-number")
			}
			return "", "", fmt.Errorf("%s required (failed to extract from IPA: %w)", strings.Join(missingFlags, " and "), err)
		}
		if versionValue == "" {
			versionValue = info.Version
		}
		if buildNumberValue == "" {
			buildNumberValue = info.BuildNumber
		}
	}
	if versionValue == "" || buildNumberValue == "" {
		missingFields := make([]string, 0, 2)
		missingFlags := make([]string, 0, 2)
		if versionValue == "" {
			missingFields = append(missingFields, "CFBundleShortVersionString")
			missingFlags = append(missingFlags, "--version")
		}
		if buildNumberValue == "" {
			missingFields = append(missingFields, "CFBundleVersion")
			missingFlags = append(missingFlags, "--build-number")
		}
		return "", "", fmt.Errorf("missing Info.plist keys %s; provide %s", strings.Join(missingFields, " and "), strings.Join(missingFlags, " and "))
	}
	return versionValue, buildNumberValue, nil
}

func prepareBuildUpload(ctx context.Context, client *asc.Client, appID string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform) (*asc.BuildUploadResponse, *asc.BuildUploadFileResponse, error) {
	uploadReq := asc.BuildUploadCreateRequest{
		Data: asc.BuildUploadCreateData{
			Type: asc.ResourceTypeBuildUploads,
			Attributes: asc.BuildUploadAttributes{
				CFBundleShortVersionString: version,
				CFBundleVersion:            buildNumber,
				Platform:                   platform,
			},
			Relationships: &asc.BuildUploadRelationships{
				App: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeApps, ID: appID},
				},
			},
		},
	}

	uploadResp, err := client.CreateBuildUpload(ctx, uploadReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upload record: %w", err)
	}

	fileReq := asc.BuildUploadFileCreateRequest{
		Data: asc.BuildUploadFileCreateData{
			Type: asc.ResourceTypeBuildUploadFiles,
			Attributes: asc.BuildUploadFileAttributes{
				FileName:  fileInfo.Name(),
				FileSize:  fileInfo.Size(),
				UTI:       asc.UTIIPA,
				AssetType: asc.AssetTypeAsset,
			},
			Relationships: &asc.BuildUploadFileRelationships{
				BuildUpload: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeBuildUploads, ID: uploadResp.Data.ID},
				},
			},
		},
	}

	fileResp, err := client.CreateBuildUploadFile(ctx, fileReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file reservation: %w", err)
	}

	return uploadResp, fileResp, nil
}

func commitBuildUploadFile(ctx context.Context, client *asc.Client, fileID string, checksums *asc.Checksums) error {
	uploaded := true
	attrs := &asc.BuildUploadFileUpdateAttributes{
		Uploaded:            &uploaded,
		SourceFileChecksums: checksums,
	}
	req := asc.BuildUploadFileUpdateRequest{
		Data: asc.BuildUploadFileUpdateData{
			Type:       asc.ResourceTypeBuildUploadFiles,
			ID:         fileID,
			Attributes: attrs,
		},
	}

	if _, err := client.UpdateBuildUploadFile(ctx, fileID, req); err != nil {
		return fmt.Errorf("commit upload file: %w", err)
	}
	return nil
}
