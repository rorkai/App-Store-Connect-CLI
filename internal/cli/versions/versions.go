package versions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func VersionsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc versions <subcommand> [flags]",
		ShortHelp:  "Manage App Store versions.",
		LongHelp:   `Manage App Store versions.`,
		UsageFunc:  shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsListCommand(),
			VersionsGetCommand(),
			VersionsRelationshipsCommand(),
			VersionsExperimentsV2Command(),
			VersionsCustomerReviewsCommand(),
			VersionsAppClipDefaultExperienceCommand(),
			VersionsCreateCommand(),
			VersionsUpdateCommand(),
			VersionsDeleteCommand(),
			VersionsAttachBuildCommand(),
			VersionsReleaseCommand(),
			PhasedReleaseCommand(),
			VersionsPromotionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func VersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "Filter by version string (comma-separated)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by state (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc versions list [flags]",
		ShortHelp:  "List app store versions for an app.",
		LongHelp: `List app store versions for an app.

Examples:
  asc versions list --app "123456789"
  asc versions list --app "123456789" --version "1.0.0"
  asc versions list --app "123456789" --platform IOS --state READY_FOR_REVIEW
  asc versions list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}
			states, err := shared.NormalizeAppStoreVersionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionsOption{
				asc.WithAppStoreVersionsLimit(*limit),
				asc.WithAppStoreVersionsPlatforms(platforms),
				asc.WithAppStoreVersionsVersionStrings(shared.SplitCSV(*version)),
				asc.WithAppStoreVersionsStates(states),
				asc.WithAppStoreVersionsNextURL(*next),
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithAppStoreVersionsLimit(200))
				firstPage, err := client.GetAppStoreVersions(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("versions list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				versions, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersions(ctx, resolvedAppID, asc.WithAppStoreVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("versions list: %w", err)
				}

				return shared.PrintOutput(versions, *output, *pretty)
			}

			versions, err := client.GetAppStoreVersions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			return shared.PrintOutput(versions, *output, *pretty)
		},
	}
}

func VersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	includeBuild := fs.Bool("include-build", false, "Include attached build information")
	includeSubmission := fs.Bool("include-submission", false, "Include submission information")
	include := fs.String("include", "", "Include related resources: "+strings.Join(appStoreVersionIncludeList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc versions get [flags]",
		ShortHelp:  "Get details for an app store version.",
		LongHelp: `Get details for an app store version.

Examples:
  asc versions get --version-id "VERSION_ID"
  asc versions get --version-id "VERSION_ID" --include-build --include-submission
  asc versions get --version-id "VERSION_ID" --include "ageRatingDeclaration,appStoreReviewDetail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			includeValues, err := normalizeAppStoreVersionInclude(*include)
			if err != nil {
				return fmt.Errorf("versions get: %w", err)
			}
			if len(includeValues) > 0 {
				if *includeBuild || *includeSubmission {
					fmt.Fprintln(os.Stderr, "Error: --include cannot be used with --include-build or --include-submission")
					return flag.ErrHelp
				}
				versionResp, err := client.GetAppStoreVersion(requestCtx, trimmedID, asc.WithAppStoreVersionInclude(includeValues))
				if err != nil {
					return fmt.Errorf("versions get: %w", err)
				}
				return shared.PrintOutput(versionResp, *output, *pretty)
			}

			versionResp, err := client.GetAppStoreVersion(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("versions get: %w", err)
			}

			result := &asc.AppStoreVersionDetailResult{
				ID:            versionResp.Data.ID,
				VersionString: versionResp.Data.Attributes.VersionString,
				Platform:      string(versionResp.Data.Attributes.Platform),
				State:         shared.ResolveAppStoreVersionState(versionResp.Data.Attributes),
			}

			if *includeBuild {
				buildResp, err := fetchOptionalBuild(requestCtx, trimmedID, client.GetAppStoreVersionBuild)
				if err != nil {
					return fmt.Errorf("versions get: %w", err)
				}
				if buildResp != nil {
					result.BuildID = buildResp.Data.ID
					result.BuildVersion = buildResp.Data.Attributes.Version
				}
			}

			if *includeSubmission {
				submissionResp, err := fetchOptionalSubmission(requestCtx, trimmedID, client.GetAppStoreVersionSubmissionForVersion)
				if err != nil {
					return fmt.Errorf("versions get: %w", err)
				}
				if submissionResp != nil {
					result.SubmissionID = submissionResp.Data.ID
				}
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func VersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	versionString := fs.String("version", "", "Version string (e.g., 1.0.0) (required)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	copyright := fs.String("copyright", "", "Copyright text (e.g., '2026 My Company')")
	releaseType := fs.String("release-type", "", "Release type: MANUAL, AFTER_APPROVAL, SCHEDULED")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions create [flags]",
		ShortHelp:  "Create a new app store version.",
		LongHelp: `Create a new app store version.

Examples:
  asc versions create --app "123456789" --version "2.0.0"
  asc versions create --app "123456789" --version "2.0.0" --platform IOS
  asc versions create --app "123456789" --version "2.0.0" --copyright "2026 My Company" --release-type MANUAL`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionString) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return fmt.Errorf("versions create: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppStoreVersionCreateAttributes{
				Platform:      asc.Platform(normalizedPlatform),
				VersionString: strings.TrimSpace(*versionString),
			}
			if *copyright != "" {
				attrs.Copyright = *copyright
			}
			if *releaseType != "" {
				attrs.ReleaseType = strings.ToUpper(*releaseType)
			}

			resp, err := client.CreateAppStoreVersion(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("versions create: %w", err)
			}

			result := &asc.AppStoreVersionDetailResult{
				ID:            resp.Data.ID,
				VersionString: resp.Data.Attributes.VersionString,
				Platform:      string(resp.Data.Attributes.Platform),
				State:         shared.ResolveAppStoreVersionState(resp.Data.Attributes),
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func VersionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions update", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	copyright := fs.String("copyright", "", "Copyright text (e.g., '2026 My Company')")
	releaseType := fs.String("release-type", "", "Release type: MANUAL, AFTER_APPROVAL, SCHEDULED")
	earliestReleaseDate := fs.String("earliest-release-date", "", "Earliest release date (ISO 8601, e.g., 2026-02-01T08:00:00+00:00)")
	versionString := fs.String("version", "", "Version string (e.g., 1.0.1)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc versions update [flags]",
		ShortHelp:  "Update an app store version.",
		LongHelp: `Update an app store version.

Examples:
  asc versions update --version-id "VERSION_ID" --copyright "2026 My Company"
  asc versions update --version-id "VERSION_ID" --release-type MANUAL
  asc versions update --version-id "VERSION_ID" --release-type SCHEDULED --earliest-release-date "2026-02-01T08:00:00+00:00"
  asc versions update --version-id "VERSION_ID" --version "1.0.1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			// Check that at least one update field is provided
			if *copyright == "" && *releaseType == "" && *earliestReleaseDate == "" && *versionString == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one of --copyright, --release-type, --earliest-release-date, or --version is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppStoreVersionUpdateAttributes{}
			if *copyright != "" {
				attrs.Copyright = copyright
			}
			if *releaseType != "" {
				rt := strings.ToUpper(*releaseType)
				attrs.ReleaseType = &rt
			}
			if *earliestReleaseDate != "" {
				attrs.EarliestReleaseDate = earliestReleaseDate
			}
			if *versionString != "" {
				attrs.VersionString = versionString
			}

			resp, err := client.UpdateAppStoreVersion(requestCtx, strings.TrimSpace(*versionID), attrs)
			if err != nil {
				return fmt.Errorf("versions update: %w", err)
			}

			result := &asc.AppStoreVersionDetailResult{
				ID:            resp.Data.ID,
				VersionString: resp.Data.Attributes.VersionString,
				Platform:      string(resp.Data.Attributes.Platform),
				State:         shared.ResolveAppStoreVersionState(resp.Data.Attributes),
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func VersionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions delete", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc versions delete [flags]",
		ShortHelp:  "Delete an app store version (only versions in PREPARE_FOR_SUBMISSION state).",
		LongHelp: `Delete an app store version.

Only versions in PREPARE_FOR_SUBMISSION state can be deleted.

Examples:
  asc versions delete --version-id "VERSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete a version")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersion(requestCtx, strings.TrimSpace(*versionID)); err != nil {
				return fmt.Errorf("versions delete: %w", err)
			}

			result := map[string]interface{}{
				"versionId": strings.TrimSpace(*versionID),
				"deleted":   true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func VersionsAttachBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions attach-build", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	buildID := fs.String("build", "", "Build ID to attach (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "attach-build",
		ShortUsage: "asc versions attach-build [flags]",
		ShortHelp:  "Attach a build to an app store version.",
		LongHelp: `Attach a build to an app store version.

Examples:
  asc versions attach-build --version-id "VERSION_ID" --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions attach-build: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AttachBuildToVersion(requestCtx, strings.TrimSpace(*versionID), strings.TrimSpace(*buildID)); err != nil {
				return fmt.Errorf("versions attach-build: %w", err)
			}

			result := &asc.AppStoreVersionAttachBuildResult{
				VersionID: strings.TrimSpace(*versionID),
				BuildID:   strings.TrimSpace(*buildID),
				Attached:  true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func fetchOptionalBuild(ctx context.Context, versionID string, fetch func(context.Context, string) (*asc.BuildResponse, error)) (*asc.BuildResponse, error) {
	resp, err := fetch(ctx, versionID)
	if err != nil {
		if asc.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}

func fetchOptionalSubmission(ctx context.Context, versionID string, fetch func(context.Context, string) (*asc.AppStoreVersionSubmissionResourceResponse, error)) (*asc.AppStoreVersionSubmissionResourceResponse, error) {
	resp, err := fetch(ctx, versionID)
	if err != nil {
		if asc.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}
