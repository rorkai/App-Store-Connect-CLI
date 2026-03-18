package builds

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const buildWaitDefaultTimeout = 30 * time.Minute

// BuildsUploadCommand returns a command to upload a build
func BuildsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (for iOS, tvOS, visionOS apps)")
	pkgPath := fs.String("pkg", "", "Path to .pkg file (for macOS apps)")
	version := fs.String("version", "", "CFBundleShortVersionString (e.g., 1.0.0, auto-extracted from IPA if not provided)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (e.g., 123, auto-extracted from IPA if not provided)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (auto-detected for --pkg)")
	dryRun := fs.Bool("dry-run", false, "Reserve upload operations without uploading the file")
	concurrency := fs.Int("concurrency", 1, "Upload concurrency (default 1)")
	verifyChecksum := fs.Bool("checksum", false, "Verify upload checksums if provided by API")
	testNotes := fs.String("test-notes", "", "What to Test notes (requires build processing)")
	locale := fs.String("locale", "", "Locale for --test-notes (e.g., en-US)")
	wait := fs.Bool("wait", false, "Wait for build processing to complete")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and --test-notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc builds upload [flags]",
		ShortHelp:  "Upload a build to App Store Connect.",
		LongHelp: `Upload a build to App Store Connect.

By default, this command uploads the IPA/PKG to the presigned URLs and commits
the file. Use --dry-run to only reserve the upload operations.

Use --ipa for iOS, tvOS, and visionOS apps. Use --pkg for macOS apps.
When using --pkg, the platform is automatically set to MAC_OS.

Examples:
  asc builds upload --app "123456789" --ipa "path/to/app.ipa"
  asc builds upload --ipa "app.ipa" --version "1.0.0" --build-number "123"
  asc builds upload --app "123456789" --ipa "app.ipa" --dry-run
  asc builds upload --app "123456789" --ipa "app.ipa" --test-notes "Test flow" --locale "en-US" --wait
  asc builds upload --app "123456789" --pkg "path/to/app.pkg" --version "1.0.0" --build-number "123"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Validate required flags
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			// Validate that exactly one of --ipa or --pkg is provided
			hasIPA := *ipaPath != ""
			hasPKG := *pkgPath != ""
			if !hasIPA && !hasPKG {
				fmt.Fprintf(os.Stderr, "Error: --ipa or --pkg is required\n\n")
				return flag.ErrHelp
			}
			if hasIPA && hasPKG {
				fmt.Fprintf(os.Stderr, "Error: --ipa and --pkg are mutually exclusive\n\n")
				return flag.ErrHelp
			}

			// Determine file path and UTI based on provided flag
			var filePath string
			var fileUTI asc.UTI
			if hasIPA {
				filePath = *ipaPath
				fileUTI = asc.UTIIPA
			} else {
				filePath = *pkgPath
				fileUTI = asc.UTIPKG
			}

			// Validate file exists
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				if hasIPA {
					return fmt.Errorf("builds upload: failed to stat IPA: %w", err)
				}
				return fmt.Errorf("builds upload: failed to stat PKG: %w", err)
			}
			if fileInfo.IsDir() {
				if hasIPA {
					return fmt.Errorf("builds upload: --ipa must be a file")
				}
				return fmt.Errorf("builds upload: --pkg must be a file")
			}

			// Determine platform
			var platformValue asc.Platform
			if hasPKG {
				// For PKG files, platform must be MAC_OS
				if *platform != "" && strings.ToUpper(*platform) != "MAC_OS" {
					return fmt.Errorf("builds upload: --pkg requires --platform MAC_OS (or omit --platform)")
				}
				platformValue = asc.PlatformMacOS
			} else {
				// For IPA files, default to IOS if not specified
				platformStr := strings.ToUpper(*platform)
				if platformStr == "" {
					platformStr = "IOS"
				}
				platformValue = asc.Platform(platformStr)
			}

			// Validate platform
			switch platformValue {
			case asc.PlatformIOS, asc.PlatformMacOS, asc.PlatformTVOS, asc.PlatformVisionOS:
			default:
				return fmt.Errorf("builds upload: --platform must be IOS, MAC_OS, TV_OS, or VISION_OS")
			}
			if *dryRun {
				if *concurrency != 1 {
					return fmt.Errorf("builds upload: --concurrency is not supported with --dry-run")
				}
				if *verifyChecksum {
					return fmt.Errorf("builds upload: --checksum is not supported with --dry-run")
				}
				if *wait {
					return fmt.Errorf("builds upload: --wait is not supported with --dry-run")
				}
			} else if *concurrency < 1 {
				return fmt.Errorf("builds upload: --concurrency must be at least 1")
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
				if *dryRun {
					return fmt.Errorf("builds upload: --test-notes is not supported with --dry-run")
				}
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return fmt.Errorf("builds upload: %w", err)
				}
			}
			if (*wait || testNotesValue != "") && *pollInterval <= 0 {
				return fmt.Errorf("builds upload: --poll-interval must be greater than 0")
			}

			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			if versionValue == "" || buildNumberValue == "" {
				// Auto-extraction only works for IPA files
				if hasIPA {
					info, err := shared.ExtractBundleInfoFromIPA(filePath)
					if err != nil {
						missingFlags := make([]string, 0, 2)
						if versionValue == "" {
							missingFlags = append(missingFlags, "--version")
						}
						if buildNumberValue == "" {
							missingFlags = append(missingFlags, "--build-number")
						}
						return fmt.Errorf("builds upload: %s required (failed to extract from IPA: %w)", strings.Join(missingFlags, " and "), err)
					}
					if versionValue == "" {
						versionValue = info.Version
					}
					if buildNumberValue == "" {
						buildNumberValue = info.BuildNumber
					}
				} else {
					// PKG files require explicit version and build number
					missingFlags := make([]string, 0, 2)
					if versionValue == "" {
						missingFlags = append(missingFlags, "--version")
					}
					if buildNumberValue == "" {
						missingFlags = append(missingFlags, "--build-number")
					}
					return fmt.Errorf("builds upload: %s required for PKG uploads", strings.Join(missingFlags, " and "))
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
				return fmt.Errorf("builds upload: missing Info.plist keys %s; provide %s", strings.Join(missingFields, " and "), strings.Join(missingFlags, " and "))
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds upload: %w", err)
			}

			timeoutValue := asc.ResolveTimeout()
			if *wait || testNotesValue != "" {
				timeoutValue = asc.ResolveTimeoutWithDefault(buildWaitDefaultTimeout)
			}
			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, timeoutValue)
			defer cancel()

			// Step 1: Create build upload record
			uploadReq := asc.BuildUploadCreateRequest{
				Data: asc.BuildUploadCreateData{
					Type: asc.ResourceTypeBuildUploads,
					Attributes: asc.BuildUploadAttributes{
						CFBundleShortVersionString: versionValue,
						CFBundleVersion:            buildNumberValue,
						Platform:                   platformValue,
					},
					Relationships: &asc.BuildUploadRelationships{
						App: &asc.Relationship{
							Data: asc.ResourceData{Type: asc.ResourceTypeApps, ID: resolvedAppID},
						},
					},
				},
			}

			uploadResp, err := client.CreateBuildUpload(requestCtx, uploadReq)
			if err != nil {
				return fmt.Errorf("builds upload: failed to create upload record: %w", err)
			}

			// Step 2: Create build upload file reservation
			fileReq := asc.BuildUploadFileCreateRequest{
				Data: asc.BuildUploadFileCreateData{
					Type: asc.ResourceTypeBuildUploadFiles,
					Attributes: asc.BuildUploadFileAttributes{
						FileName:  fileInfo.Name(),
						FileSize:  fileInfo.Size(),
						UTI:       fileUTI,
						AssetType: asc.AssetTypeAsset,
					},
					Relationships: &asc.BuildUploadFileRelationships{
						BuildUpload: &asc.Relationship{
							Data: asc.ResourceData{Type: asc.ResourceTypeBuildUploads, ID: uploadResp.Data.ID},
						},
					},
				},
			}

			fileResp, err := client.CreateBuildUploadFile(requestCtx, fileReq)
			if err != nil {
				return fmt.Errorf("builds upload: failed to create file reservation: %w", err)
			}

			// Return upload info including presigned URL operations
			result := &asc.BuildUploadResult{
				UploadID:   uploadResp.Data.ID,
				FileID:     fileResp.Data.ID,
				FileName:   fileResp.Data.Attributes.FileName,
				FileSize:   fileResp.Data.Attributes.FileSize,
				Operations: fileResp.Data.Attributes.UploadOperations,
			}

			if !*dryRun {
				if len(fileResp.Data.Attributes.UploadOperations) == 0 {
					return fmt.Errorf("builds upload: no upload operations returned")
				}

				uploadOpts := []asc.UploadOption{
					asc.WithUploadConcurrency(*concurrency),
				}
				fmt.Fprintf(os.Stderr, "Uploading %s (%d bytes) to App Store Connect...\n", fileInfo.Name(), fileInfo.Size())
				uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
				err = asc.ExecuteUploadOperations(uploadCtx, filePath, fileResp.Data.Attributes.UploadOperations, uploadOpts...)
				uploadCancel()
				if err != nil {
					return fmt.Errorf("builds upload: upload failed: %w", err)
				}

				var verifiedChecksums *asc.Checksums
				var checksumVerified *bool
				if *verifyChecksum {
					src := fileResp.Data.Attributes.SourceFileChecksums
					if src == nil || (src.File == nil && src.Composite == nil) {
						fmt.Fprintln(os.Stderr, "Warning: --checksum requested but API provided no checksums to verify; skipping")
					} else {
						checksums, err := asc.VerifySourceFileChecksums(filePath, src)
						if err != nil {
							return fmt.Errorf("builds upload: checksum verification failed: %w", err)
						}
						verifiedChecksums = checksums
						verified := true
						checksumVerified = &verified
					}
				}

				uploaded := true
				updateReq := asc.BuildUploadFileUpdateRequest{
					Data: asc.BuildUploadFileUpdateData{
						Type: asc.ResourceTypeBuildUploadFiles,
						ID:   fileResp.Data.ID,
						Attributes: &asc.BuildUploadFileUpdateAttributes{
							Uploaded:            &uploaded,
							SourceFileChecksums: verifiedChecksums,
						},
					},
				}

				commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
				commitResp, err := client.UpdateBuildUploadFile(commitCtx, fileResp.Data.ID, updateReq)
				commitCancel()
				if err != nil {
					return fmt.Errorf("builds upload: failed to commit upload: %w", err)
				}

				if commitResp != nil && commitResp.Data.Attributes.Uploaded != nil {
					result.Uploaded = commitResp.Data.Attributes.Uploaded
				} else {
					result.Uploaded = &uploaded
				}
				fmt.Fprintln(os.Stderr, "Upload committed in App Store Connect.")
				result.ChecksumVerified = checksumVerified
				result.SourceFileChecksums = verifiedChecksums
				result.Operations = nil

				if *wait || testNotesValue != "" {
					fmt.Fprintf(os.Stderr, "Waiting for build %s (%s) to appear in App Store Connect...\n", buildNumberValue, versionValue)
					buildResp, err := shared.WaitForBuildByNumberOrUploadFailure(requestCtx, client, resolvedAppID, uploadResp.Data.ID, versionValue, buildNumberValue, string(platformValue), *pollInterval)
					if err != nil {
						return fmt.Errorf("builds upload: %w", err)
					}
					if buildResp == nil {
						return fmt.Errorf("builds upload: failed to resolve build for version %q build %q", versionValue, buildNumberValue)
					}

					fmt.Fprintf(os.Stderr, "Build %s discovered; waiting for processing...\n", buildResp.Data.ID)
					buildResp, err = client.WaitForBuildProcessing(requestCtx, buildResp.Data.ID, *pollInterval)
					if err != nil {
						return fmt.Errorf("builds upload: %w", err)
					}

					if testNotesValue != "" {
						if _, err := shared.UpsertBetaBuildLocalization(requestCtx, client, buildResp.Data.ID, localeValue, testNotesValue); err != nil {
							return fmt.Errorf("builds upload: %w", err)
						}
					}
				}
			}

			format := *output.Output

			return shared.PrintOutput(result, format, *output.Pretty)
		},
	}
}

// BuildsCommand returns the builds command with subcommands
func BuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds", flag.ExitOnError)

	// Parent command has no flags - subcommands define their own
	listCmd := BuildsListCommand()

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc builds <subcommand> [flags]",
		ShortHelp:  "Manage builds in App Store Connect.",
		LongHelp: `Manage builds in App Store Connect.

Examples:
  asc builds list --app "123456789"
  asc builds count --app "123456789"
  asc builds latest --app "123456789"
  asc builds find --app "123456789" --build-number "42"
  asc builds wait --build "BUILD_ID"
  asc builds wait --app "123456789" --newest
  asc builds info --build "BUILD_ID"
  asc builds expire --build "BUILD_ID"
  asc builds expire-all --app "123456789" --older-than 90d --dry-run
  asc builds upload --app "123456789" --ipa "app.ipa"
  asc builds upload --app "123456789" --pkg "app.pkg" --version "1.0.0" --build-number "1"
  asc builds uploads list --app "123456789"
  asc builds test-notes list --build "BUILD_ID"
  asc builds individual-testers list --build "BUILD_ID"
  asc builds update --build "BUILD_ID" --uses-non-exempt-encryption=false
  asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds app get --build "BUILD_ID"
  asc builds pre-release-version get --build "BUILD_ID"
  asc builds icons list --build "BUILD_ID"
  asc builds beta-app-review-submission get --build "BUILD_ID"
  asc builds build-beta-detail get --build "BUILD_ID"
  asc builds links view --build "BUILD_ID" --type "app"
  asc builds metrics beta-usages --build "BUILD_ID"
  asc builds dsyms --build "BUILD_ID" --output-dir "./dsyms"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			listCmd,
			BuildsCountCommand(),
			BuildsLatestCommand(),
			BuildsFindCommand(),
			BuildsWaitCommand(),
			BuildsInfoCommand(),
			BuildsExpireCommand(),
			BuildsExpireAllCommand(),
			BuildsUploadCommand(),
			BuildsUploadsCommand(),
			BuildsTestNotesCommand(),
			BuildsAppEncryptionDeclarationCommand(),
			BuildsUpdateCommand(),
			BuildsAddGroupsCommand(),
			BuildsRemoveGroupsCommand(),
			BuildsIndividualTestersCommand(),
			BuildsAppCommand(),
			BuildsPreReleaseVersionCommand(),
			BuildsIconsCommand(),
			BuildsBetaAppReviewSubmissionCommand(),
			BuildsBuildBetaDetailCommand(),
			BuildsRelationshipsCommand(),
			deprecatedBuildsRelationshipsAliasCommand(),
			BuildsMetricsCommand(),
			BuildsDsymsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsListCommand returns the builds list subcommand
func BuildsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)
	sort := fs.String("sort", "", "Sort by uploadedDate or -uploadedDate")
	version := fs.String("version", "", "Filter by marketing version string (CFBundleShortVersionString)")
	buildNumber := fs.String("build-number", "", "Filter by build number (CFBundleVersion)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Filter by processing state: VALID, PROCESSING, FAILED, INVALID, or all")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds list [flags]",
		ShortHelp:  "List builds for an app in App Store Connect.",
		LongHelp: `List builds for an app in App Store Connect.

This command fetches builds uploaded to App Store Connect,
including processing status and expiration dates.

Examples:
  asc builds list --app "123456789"
  asc builds list --app "123456789" --version "1.2.3"
  asc builds list --app "123456789" --build-number "123"
  asc builds list --app "123456789" --platform TV_OS
  asc builds list --app "123456789" --platform IOS --version "1.2.3"
  asc builds list --app "123456789" --processing-state "PROCESSING"
  asc builds list --app "123456789" --processing-state "all"
  asc builds list --app "123456789" --version "1.2.3" --build-number "123"
  asc builds list --app "123456789" --limit 10
  asc builds list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds: --limit must be between 1 and 200")
			}
			nextValue := strings.TrimSpace(*next)
			if err := shared.ValidateNextURL(nextValue); err != nil {
				return fmt.Errorf("builds: %w", err)
			}
			if err := shared.ValidateSort(*sort, "uploadedDate", "-uploadedDate"); err != nil {
				return fmt.Errorf("builds: %w", err)
			}

			platformValue := ""
			if strings.TrimSpace(*platform) != "" {
				normalizedPlatform, err := shared.NormalizePlatform(*platform)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				platformValue = string(normalizedPlatform)
			}

			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			processingStateValues, err := normalizeBuildProcessingStateFilter(*processingState)
			if err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && nextValue == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if resolvedAppID != "" && nextValue == "" {
				resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("builds: %w", err)
				}
			}

			preReleaseVersionIDs := []string{}
			if versionValue != "" && nextValue == "" {
				preReleaseVersionIDs, err = findPreReleaseVersionIDsForBuildsList(requestCtx, client, resolvedAppID, versionValue)
				if err != nil {
					return fmt.Errorf("builds: %w", err)
				}
				if len(preReleaseVersionIDs) == 0 {
					return shared.PrintOutput(&asc.BuildsResponse{Data: []asc.Resource[asc.BuildAttributes]{}}, *output.Output, *output.Pretty)
				}
			}

			opts := []asc.BuildsOption{
				asc.WithBuildsLimit(*limit),
				asc.WithBuildsNextURL(nextValue),
				asc.WithBuildsInclude([]string{"preReleaseVersion"}),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithBuildsSort(*sort))
			}
			if buildNumberValue != "" {
				opts = append(opts, asc.WithBuildsBuildNumber(buildNumberValue))
			}
			if platformValue != "" {
				opts = append(opts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platformValue}))
			}
			if len(processingStateValues) > 0 {
				opts = append(opts, asc.WithBuildsProcessingStates(processingStateValues))
			}
			if len(preReleaseVersionIDs) > 0 {
				opts = append(opts, asc.WithBuildsPreReleaseVersions(preReleaseVersionIDs))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBuildsLimit(200))
				builds, err := shared.PaginateWithSpinner(requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBuilds(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBuilds(ctx, resolvedAppID, asc.WithBuildsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds: %w", err)
				}

				format := *output.Output
				return shared.PrintOutput(builds, format, *output.Pretty)
			}

			builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("builds: failed to fetch: %w", err)
			}

			format := *output.Output

			return shared.PrintOutput(builds, format, *output.Pretty)
		},
	}
}

func normalizeBuildProcessingStateFilter(raw string) ([]string, error) {
	return shared.NormalizeBuildProcessingStateFilter(raw, shared.BuildProcessingStateFilterOptions{
		FlagName:          "--processing-state",
		AllowedValuesHelp: "VALID, PROCESSING, FAILED, INVALID, or all",
	})
}

func findPreReleaseVersionIDsForBuildsList(
	ctx context.Context,
	client *asc.Client,
	appID string,
	version string,
) ([]string, error) {
	version = strings.TrimSpace(version)

	firstPage, err := client.GetPreReleaseVersions(
		ctx,
		appID,
		asc.WithPreReleaseVersionsVersion(version),
		asc.WithPreReleaseVersionsLimit(200),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup pre-release versions for marketing version %q: %w", version, err)
	}

	ids := make([]string, 0, len(firstPage.Data))
	seen := make(map[string]struct{}, len(firstPage.Data))
	appendIDs := func(page *asc.PreReleaseVersionsResponse) {
		for _, preReleaseVersion := range page.Data {
			// ASC's version filter can return dotted-version near-matches like
			// 1.1 and 1.1.0 together, so enforce exact matching client-side
			// when the response includes attributes.version. If ASC omits the
			// attribute entirely, trust the server-side filter instead.
			versionAttr := strings.TrimSpace(preReleaseVersion.Attributes.Version)
			if versionAttr != "" && versionAttr != version {
				continue
			}
			id := strings.TrimSpace(preReleaseVersion.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetPreReleaseVersions(ctx, appID, asc.WithPreReleaseVersionsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.PreReleaseVersionsResponse)
		if !ok {
			return fmt.Errorf("unexpected pre-release versions page type %T", page)
		}
		appendIDs(resp)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to paginate pre-release versions for marketing version %q: %w", version, err)
	}

	return ids, nil
}

func attachBuildInfoPreReleaseVersion(
	ctx context.Context,
	client *asc.Client,
	build *asc.BuildResponse,
) error {
	if client == nil || build == nil {
		return nil
	}
	if strings.TrimSpace(build.Data.ID) == "" {
		return nil
	}

	preReleaseVersion, err := client.GetBuildPreReleaseVersion(ctx, build.Data.ID)
	if err != nil {
		return nil
	}

	included, err := json.Marshal([]asc.PreReleaseVersion{preReleaseVersion.Data})
	if err != nil {
		return fmt.Errorf("failed to encode pre-release version include: %w", err)
	}
	build.Included = included

	relationships, err := mergeBuildRelationship(build.Data.Relationships, "preReleaseVersion", map[string]any{
		"preReleaseVersion": map[string]any{
			"data": map[string]string{
				"type": "preReleaseVersions",
				"id":   preReleaseVersion.Data.ID,
			},
		},
	})
	if err != nil {
		return err
	}
	build.Data.Relationships = relationships
	return nil
}

func mergeBuildRelationship(relationships json.RawMessage, key string, value map[string]any) (json.RawMessage, error) {
	merged := make(map[string]json.RawMessage)
	if len(relationships) > 0 {
		if err := json.Unmarshal(relationships, &merged); err != nil {
			return nil, fmt.Errorf("failed to decode existing build relationships: %w", err)
		}
	}

	entry, ok := value[key]
	if !ok {
		return nil, fmt.Errorf("missing %s relationship payload", key)
	}
	encodedEntry, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s relationship: %w", key, err)
	}
	merged[key] = encodedEntry

	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to encode merged build relationships: %w", err)
	}
	return raw, nil
}

// BuildsInfoCommand returns a build info subcommand.
func BuildsInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds info", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc builds info --build BUILD_ID",
		ShortHelp:  "Show details for a specific build.",
		LongHelp: `Show details for a specific build.

Examples:
  asc builds info --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds info: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			build, err := client.GetBuild(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				return fmt.Errorf("builds info: failed to fetch: %w", err)
			}
			if err := attachBuildInfoPreReleaseVersion(requestCtx, client, build); err != nil {
				return fmt.Errorf("builds info: %w", err)
			}

			format := *output.Output

			return shared.PrintOutput(build, format, *output.Pretty)
		},
	}
}

// BuildsExpireCommand returns a build expiration subcommand.
func BuildsExpireCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds expire", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	confirm := fs.Bool("confirm", false, "Confirm expiration")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "expire",
		ShortUsage: "asc builds expire --build BUILD_ID --confirm [flags]",
		ShortHelp:  "Expire a build for TestFlight.",
		LongHelp: `Expire a build for TestFlight.

This action is irreversible for the specified build.

Examples:
  asc builds expire --build "BUILD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to expire build")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds expire: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			build, err := client.ExpireBuild(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				return fmt.Errorf("builds expire: failed to expire: %w", err)
			}

			format := *output.Output

			return shared.PrintOutput(build, format, *output.Pretty)
		},
	}
}
