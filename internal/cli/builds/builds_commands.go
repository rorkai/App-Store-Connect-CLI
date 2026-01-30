package builds

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

const buildWaitDefaultTimeout = 30 * time.Minute

// BuildsUploadCommand returns a command to upload a build
func BuildsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	ipaPath := fs.String("ipa", "", "Path to .ipa file (required)")
	version := fs.String("version", "", "CFBundleShortVersionString (e.g., 1.0.0, auto-extracted from IPA if not provided)")
	buildNumber := fs.String("build-number", "", "CFBundleVersion (e.g., 123, auto-extracted from IPA if not provided)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	dryRun := fs.Bool("dry-run", false, "Reserve upload operations without uploading the file")
	concurrency := fs.Int("concurrency", 1, "Upload concurrency (default 1)")
	verifyChecksum := fs.Bool("checksum", false, "Verify upload checksums if provided by API")
	testNotes := fs.String("test-notes", "", "What to Test notes (requires build processing)")
	locale := fs.String("locale", "", "Locale for --test-notes (e.g., en-US)")
	wait := fs.Bool("wait", false, "Wait for build processing to complete")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait and --test-notes")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc builds upload [flags]",
		ShortHelp:  "Upload a build to App Store Connect.",
		LongHelp: `Upload a build to App Store Connect.

By default, this command uploads the IPA to the presigned URLs and commits
the file. Use --dry-run to only reserve the upload operations.

Examples:
  asc builds upload --app "123456789" --ipa "path/to/app.ipa"
  asc builds upload --ipa "app.ipa" --version "1.0.0" --build-number "123"
  asc builds upload --app "123456789" --ipa "app.ipa" --dry-run
  asc builds upload --app "123456789" --ipa "app.ipa" --test-notes "Test flow" --locale "en-US" --wait`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Validate required flags
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if *ipaPath == "" {
				fmt.Fprintf(os.Stderr, "Error: --ipa is required\n\n")
				return flag.ErrHelp
			}

			// Validate IPA file exists
			fileInfo, err := os.Stat(*ipaPath)
			if err != nil {
				return fmt.Errorf("builds upload: failed to stat IPA: %w", err)
			}
			if fileInfo.IsDir() {
				return fmt.Errorf("builds upload: --ipa must be a file")
			}

			// Validate platform
			platformValue := asc.Platform(strings.ToUpper(*platform))
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
				info, err := shared.ExtractBundleInfoFromIPA(*ipaPath)
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
				return fmt.Errorf("builds upload: Info.plist missing %s; provide %s", strings.Join(missingFields, " and "), strings.Join(missingFlags, " and "))
			}

			client, err := getASCClient()
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
				uploadCtx, uploadCancel := contextWithUploadTimeout(ctx)
				err = asc.ExecuteUploadOperations(uploadCtx, *ipaPath, fileResp.Data.Attributes.UploadOperations, uploadOpts...)
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
						checksums, err := asc.VerifySourceFileChecksums(*ipaPath, src)
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

				commitCtx, commitCancel := contextWithUploadTimeout(ctx)
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
				result.ChecksumVerified = checksumVerified
				result.SourceFileChecksums = verifiedChecksums
				result.Operations = nil

				if *wait || testNotesValue != "" {
					buildResp, err := shared.WaitForBuildByNumber(requestCtx, client, resolvedAppID, versionValue, buildNumberValue, string(platformValue), *pollInterval)
					if err != nil {
						return fmt.Errorf("builds upload: %w", err)
					}
					if buildResp == nil {
						return fmt.Errorf("builds upload: failed to resolve build for version %q build %q", versionValue, buildNumberValue)
					}

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

			format := *output

			return printOutput(result, format, *pretty)
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
  asc builds latest --app "123456789"
  asc builds info --build "BUILD_ID"
  asc builds expire --build "BUILD_ID"
  asc builds expire-all --app "123456789" --older-than 90d --dry-run
  asc builds upload --app "123456789" --ipa "app.ipa"
  asc builds uploads list --app "123456789"
  asc builds test-notes list --build "BUILD_ID"
  asc builds individual-testers list --build "BUILD_ID"
  asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds relationships get --build "BUILD_ID" --type "app"
  asc builds metrics beta-usages --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			listCmd,
			BuildsLatestCommand(),
			BuildsInfoCommand(),
			BuildsExpireCommand(),
			BuildsExpireAllCommand(),
			BuildsUploadCommand(),
			BuildsUploadsCommand(),
			BuildsTestNotesCommand(),
			BuildsAddGroupsCommand(),
			BuildsRemoveGroupsCommand(),
			BuildsIndividualTestersCommand(),
			BuildsRelationshipsCommand(),
			BuildsMetricsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsListCommand returns the builds list subcommand
func BuildsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	sort := fs.String("sort", "", "Sort by uploadedDate or -uploadedDate")
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
  asc builds list --app "123456789" --limit 10
  asc builds list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds: %w", err)
			}
			if err := validateSort(*sort, "uploadedDate", "-uploadedDate"); err != nil {
				return fmt.Errorf("builds: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildsOption{
				asc.WithBuildsLimit(*limit),
				asc.WithBuildsNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithBuildsSort(*sort))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithBuildsLimit(200))
				firstPage, err := client.GetBuilds(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("builds: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				builds, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuilds(ctx, resolvedAppID, asc.WithBuildsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("builds: %w", err)
				}

				format := *output
				return printOutput(builds, format, *pretty)
			}

			builds, err := client.GetBuilds(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("builds: failed to fetch: %w", err)
			}

			format := *output

			return printOutput(builds, format, *pretty)
		},
	}
}

// BuildsInfoCommand returns a build info subcommand.
func BuildsInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds info", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc builds info --build BUILD_ID",
		ShortHelp:  "Show details for a specific build.",
		LongHelp: `Show details for a specific build.

Examples:
  asc builds info --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds info: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			build, err := client.GetBuild(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				return fmt.Errorf("builds info: failed to fetch: %w", err)
			}

			format := *output

			return printOutput(build, format, *pretty)
		},
	}
}

// BuildsExpireCommand returns a build expiration subcommand.
func BuildsExpireCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds expire", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "expire",
		ShortUsage: "asc builds expire [flags]",
		ShortHelp:  "Expire a build for TestFlight.",
		LongHelp: `Expire a build for TestFlight.

This action is irreversible for the specified build.

Examples:
  asc builds expire --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds expire: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			build, err := client.ExpireBuild(requestCtx, strings.TrimSpace(*buildID))
			if err != nil {
				return fmt.Errorf("builds expire: failed to expire: %w", err)
			}

			format := *output

			return printOutput(build, format, *pretty)
		},
	}
}
