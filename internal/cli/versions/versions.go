package versions

import (
	"bytes"
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
	webcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
)

func VersionsCommand() *ffcli.Command {
	viewCmd := VersionsViewCommand()

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc versions <subcommand> [flags]",
		ShortHelp:  "Manage App Store versions.",
		LongHelp: `Manage App Store versions.

Examples:
  asc versions list --app "123456789"
  asc versions list --app "123456789" --platform IOS --state READY_FOR_REVIEW
  asc versions view --version-id "VERSION_ID" --include-build --include-submission
  asc versions create --app "123456789" --version "2.0.0" --platform IOS
  asc versions create --app "123456789" --version "2.4.0" --copy-metadata-from "2.3.2"
  asc versions update --version-id "VERSION_ID" --release-type MANUAL
  asc versions attach-build --version-id "VERSION_ID" --build-id "BUILD_ID"
  asc versions release --version-id "VERSION_ID" --confirm
  asc versions phased-release view --version-id "VERSION_ID"
  asc versions rating-reset view --version-id "VERSION_ID"`,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsListCommand(),
			viewCmd,
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
			webcli.VersionRatingResetCommand(),
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
	include := shared.BindOnceCSVFlag(fs, "include", "[experimental] Include related resources: "+strings.Join(appStoreVersionsIncludeList(), ", "))
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	latest := fs.Bool("latest", false, "[experimental] Keep only the newest version per platform by createdDate (fetches all pages)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc versions list [flags]",
		ShortHelp:  "List app store versions for an app.",
		LongHelp: `List app store versions for an app.

The App Store Connect API can report every historical version of an app as
READY_FOR_SALE, so a state filter alone cannot identify the live version.
--latest fetches every page and keeps only the newest version per platform by
createdDate; combine it with --state READY_FOR_SALE to get the version that
is actually live on each platform.

Use --include to return related resources in the same response instead of
issuing a follow-up request per version. Included review-detail passwords are
redacted by default; use --include-sensitive to print them explicitly.

Examples:
  asc versions list --app "123456789"
  asc versions list --app "123456789" --version "1.0.0"
  asc versions list --app "123456789" --platform IOS --state READY_FOR_REVIEW
  asc versions list --app "123456789" --state READY_FOR_SALE --latest
  asc versions list --app "123456789" --include "build,appStoreVersionSubmission"
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
			if *latest && strings.TrimSpace(*next) != "" {
				return shared.UsageError("versions list: --latest fetches all pages itself and cannot be combined with --next")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}
			states, err := shared.NormalizeAppStoreVersionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}
			if err := shared.ValidateAppStoreVersionStateFilterCombination(states); err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			includeValues, err := normalizeAppStoreVersionsInclude(include.String())
			if err != nil {
				return shared.UsageErrorf("versions list: %v", err)
			}
			if len(includeValues) > 0 && strings.TrimSpace(*next) != "" {
				return shared.UsageError("versions list: --next cannot be combined with --include")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
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
				asc.WithAppStoreVersionsInclude(includeValues),
				asc.WithAppStoreVersionsNextURL(*next),
			}

			if *paginate || *latest {
				// Use the caller's page size when provided; otherwise request the
				// largest page to keep automatic pagination efficient.
				paginateOpts := opts
				if *limit == 0 {
					paginateOpts = append(paginateOpts, asc.WithAppStoreVersionsLimit(200))
				}
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
				versionsResponse, ok := versions.(*asc.AppStoreVersionsResponse)
				if !ok {
					return fmt.Errorf("versions list: unexpected paginated response type %T", versions)
				}
				if *latest {
					selected, err := latestVersionsPerPlatform(versionsResponse.Data)
					if err != nil {
						return fmt.Errorf("versions list: %w", err)
					}
					included, err := latestVersionsIncluded(selected, versionsResponse.Included)
					if err != nil {
						return fmt.Errorf("versions list: %w", err)
					}
					selectedResponse := &asc.AppStoreVersionsResponse{Data: selected, Included: included}
					if *includeSensitive {
						shared.WarnIncludeSensitive(os.Stderr, true)
					} else {
						selectedResponse, err = asc.RedactAppStoreReviewDetailIncludesInListResponse(selectedResponse)
						if err != nil {
							return fmt.Errorf("versions list: %w", err)
						}
					}
					result := &asc.AppStoreVersionsLatestResult{
						Items:      selected,
						Included:   selectedResponse.Included,
						TotalCount: len(selected),
						HasMore:    false,
					}
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return printAppStoreVersionsList(versionsResponse, *includeSensitive, *output.Output, *output.Pretty)
			}

			versions, err := client.GetAppStoreVersions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("versions list: %w", err)
			}

			return printAppStoreVersionsList(versions, *includeSensitive, *output.Output, *output.Pretty)
		},
	}
}

// latestVersionsPerPlatform keeps the newest version per platform by
// createdDate; the API reports historical versions with live-looking states,
// so recency has to be derived rather than filtered.
func latestVersionsPerPlatform(data []asc.Resource[asc.AppStoreVersionAttributes]) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	type selectedVersion struct {
		index       int
		createdDate time.Time
	}

	newest := map[string]selectedVersion{}
	order := []string{}
	for index, version := range data {
		createdDate, err := time.Parse(time.RFC3339, strings.TrimSpace(version.Attributes.CreatedDate))
		if err != nil {
			return nil, fmt.Errorf("version %q has invalid createdDate %q: %w", version.ID, version.Attributes.CreatedDate, err)
		}
		platform := string(version.Attributes.Platform)
		current, seen := newest[platform]
		if !seen {
			newest[platform] = selectedVersion{index: index, createdDate: createdDate}
			order = append(order, platform)
			continue
		}
		if createdDate.After(current.createdDate) {
			newest[platform] = selectedVersion{index: index, createdDate: createdDate}
		}
	}
	result := make([]asc.Resource[asc.AppStoreVersionAttributes], 0, len(order))
	for _, platform := range order {
		result = append(result, data[newest[platform].index])
	}
	return result, nil
}

func latestVersionsIncluded(selected []asc.Resource[asc.AppStoreVersionAttributes], included json.RawMessage) (json.RawMessage, error) {
	if len(included) == 0 {
		return nil, nil
	}

	type linkage struct {
		Type asc.ResourceType `json:"type"`
		ID   string           `json:"id"`
	}
	linked := map[linkage]struct{}{}
	for _, version := range selected {
		if len(version.Relationships) == 0 {
			continue
		}
		var relationships map[string]struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(version.Relationships, &relationships); err != nil {
			return nil, fmt.Errorf("decode selected version %q relationships: %w", version.ID, err)
		}
		for _, relationship := range relationships {
			raw := bytes.TrimSpace(relationship.Data)
			if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
				continue
			}
			if raw[0] == '[' {
				var items []linkage
				if err := json.Unmarshal(raw, &items); err != nil {
					return nil, fmt.Errorf("decode selected version %q relationship linkage: %w", version.ID, err)
				}
				for _, item := range items {
					linked[item] = struct{}{}
				}
				continue
			}
			var item linkage
			if err := json.Unmarshal(raw, &item); err != nil {
				return nil, fmt.Errorf("decode selected version %q relationship linkage: %w", version.ID, err)
			}
			linked[item] = struct{}{}
		}
	}

	var resources []json.RawMessage
	if err := json.Unmarshal(included, &resources); err != nil {
		return nil, fmt.Errorf("decode included resources: %w", err)
	}
	filtered := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		var item linkage
		if err := json.Unmarshal(resource, &item); err != nil {
			return nil, fmt.Errorf("decode included resource: %w", err)
		}
		if _, ok := linked[item]; ok {
			filtered = append(filtered, resource)
		}
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("encode selected included resources: %w", err)
	}
	return encoded, nil
}

func printAppStoreVersionsList(versions *asc.AppStoreVersionsResponse, includeSensitive bool, output string, pretty bool) error {
	if includeSensitive {
		shared.WarnIncludeSensitive(os.Stderr, true)
		return shared.PrintOutput(versions, output, pretty)
	}
	safe, err := asc.RedactAppStoreReviewDetailIncludesInListResponse(versions)
	if err != nil {
		return fmt.Errorf("versions list: %w", err)
	}
	return shared.PrintOutput(safe, output, pretty)
}

func VersionsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions view", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID")
	legacyID := shared.BindDeprecatedStringFlagAlias(fs, "id", "version-id")
	appID := fs.String("app", "", "[experimental] App Store Connect app ID (or ASC_APP_ID)")
	versionString := fs.String("version", "", "[experimental] Version string used with --app")
	platform := fs.String("platform", "IOS", "[experimental] Platform used with --app and --version: IOS, MAC_OS, TV_OS, VISION_OS")
	includeBuild := fs.Bool("include-build", false, "Include attached build information")
	includeSubmission := fs.Bool("include-submission", false, "Include submission information")
	include := fs.String("include", "", "Include related resources: "+strings.Join(appStoreVersionIncludeList(), ", "))
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc versions view (--version-id \"VERSION_ID\" | --app \"APP_ID\" --version \"1.2.3\") [flags]",
		ShortHelp:  "View details for an app store version.",
		LongHelp: `View details for an app store version.

Examples:
  asc versions view --version-id "VERSION_ID"
  asc versions view --app "123456789" --version "1.2.3"
  asc versions view --app "123456789" --version "1.2.3" --platform MAC_OS
  asc versions view --version-id "VERSION_ID" --include-build --include-submission
  asc versions view --version-id "VERSION_ID" --include "ageRatingDeclaration,appStoreReviewDetail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyID.Apply(versionID); err != nil {
				return err
			}
			trimmedID := strings.TrimSpace(*versionID)
			directIDRequested := false
			lookupRequested := false
			fs.Visit(func(parsed *flag.Flag) {
				switch parsed.Name {
				case "id", "version-id":
					directIDRequested = true
				case "app", "version", "platform":
					lookupRequested = true
				}
			})
			if directIDRequested && lookupRequested {
				return shared.UsageError("--version-id cannot be combined with --app, --version, or --platform")
			}
			if trimmedID == "" && !lookupRequested {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}

			resolvedAppID := ""
			resolvedPlatform := ""
			trimmedVersion := strings.TrimSpace(*versionString)
			if trimmedID == "" {
				if trimmedVersion == "" {
					fmt.Fprintln(os.Stderr, "Error: --version is required when resolving by app")
					return shared.MissingRequiredUsageError("--version")
				}
				resolvedAppID = strings.TrimSpace(shared.ResolveAppID(*appID))
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("--app")
				}
				var err error
				resolvedPlatform, err = shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return shared.UsageErrorf("versions view: %v", err)
				}
			}

			includeValues, err := normalizeAppStoreVersionInclude(*include)
			if err != nil {
				return shared.UsageErrorf("versions view: %v", err)
			}
			if len(includeValues) > 0 && (*includeBuild || *includeSubmission) {
				return shared.UsageError("--include cannot be used with --include-build or --include-submission")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if trimmedID == "" {
				trimmedID, err = shared.ResolveAppStoreVersionID(requestCtx, client, resolvedAppID, trimmedVersion, resolvedPlatform)
				if err != nil {
					return fmt.Errorf("versions view: resolve version: %w", err)
				}
			}

			if len(includeValues) > 0 {
				apiIncludes, includeAgeRating := splitCompatAppStoreVersionIncludes(includeValues)
				var versionResp *asc.AppStoreVersionResponse
				if len(apiIncludes) > 0 {
					versionResp, err = client.GetAppStoreVersion(requestCtx, trimmedID, asc.WithAppStoreVersionInclude(apiIncludes))
				} else {
					versionResp, err = client.GetAppStoreVersion(requestCtx, trimmedID)
				}
				if err != nil {
					return fmt.Errorf("versions view: %w", err)
				}

				if includeAgeRating {
					ageRatingResp, err := client.GetAgeRatingDeclarationForAppStoreVersion(requestCtx, trimmedID)
					if err != nil {
						return fmt.Errorf("versions view: %w", err)
					}
					if err := appendAgeRatingDeclarationInclude(versionResp, ageRatingResp); err != nil {
						return fmt.Errorf("versions view: %w", err)
					}
				}

				if *includeSensitive {
					shared.WarnIncludeSensitive(os.Stderr, true)
					return shared.PrintOutput(versionResp, *output.Output, *output.Pretty)
				}
				safe, err := asc.RedactAppStoreReviewDetailIncludesInSingleResponse(versionResp)
				if err != nil {
					return fmt.Errorf("versions view: %w", err)
				}
				return shared.PrintOutput(safe, *output.Output, *output.Pretty)
			}

			versionResp, err := client.GetAppStoreVersion(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("versions view: %w", err)
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
					return fmt.Errorf("versions view: %w", err)
				}
				if buildResp != nil {
					result.BuildID = buildResp.Data.ID
					result.BuildVersion = buildResp.Data.Attributes.Version
				}
			}

			if *includeSubmission {
				submissionResp, err := fetchOptionalSubmission(requestCtx, trimmedID, client.GetAppStoreVersionSubmissionForVersion)
				if err != nil {
					return fmt.Errorf("versions view: %w", err)
				}
				if submissionResp != nil {
					result.SubmissionID = submissionResp.Data.ID
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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
	copyMetadataFrom := fs.String("copy-metadata-from", "", "Copy localization metadata from this source version string")
	copyFields := shared.BindOnceCSVFlag(fs, "copy-fields", "Comma-separated metadata fields to copy: description, keywords, marketingUrl, promotionalText, supportUrl, whatsNew")
	excludeFields := shared.BindOnceCSVFlag(fs, "exclude-fields", "Comma-separated metadata fields to exclude from copy")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions create [flags]",
		ShortHelp:  "Create a new app store version.",
		LongHelp: `Create a new app store version.

Examples:
  asc versions create --app "123456789" --version "2.0.0"
  asc versions create --app "123456789" --version "2.0.0" --platform IOS
  asc versions create --app "123456789" --version "2.0.0" --copyright "2026 My Company" --release-type MANUAL
  asc versions create --app "123456789" --version "2.4.0" --platform IOS --copy-metadata-from "2.3.2"
  asc versions create --app "123456789" --version "2.4.0" --copy-metadata-from "2.3.2" --copy-fields "description,keywords,supportUrl" --exclude-fields "whatsNew"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionString) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return shared.MissingRequiredUsageError("--version")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return fmt.Errorf("versions create: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			copyMetadataFromValue := strings.TrimSpace(*copyMetadataFrom)
			copyFieldsValue, err := normalizeVersionMetadataCopyFields(copyFields.String(), "--copy-fields")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return flag.ErrHelp
			}
			excludeFieldsValue, err := normalizeVersionMetadataCopyFields(excludeFields.String(), "--exclude-fields")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return flag.ErrHelp
			}
			if copyMetadataFromValue == "" && (len(copyFieldsValue) > 0 || len(excludeFieldsValue) > 0) {
				fmt.Fprintln(os.Stderr, "Error: --copy-metadata-from is required when using --copy-fields or --exclude-fields")
				return shared.MissingRequiredUsageError("--copy-metadata-from")
			}

			selectedCopyFields := []string(nil)
			if copyMetadataFromValue != "" {
				selectedCopyFields, err = resolveVersionMetadataCopyFields(copyFieldsValue, excludeFieldsValue)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return flag.ErrHelp
				}
			}

			normalizedReleaseType, err := normalizeAppStoreVersionReleaseType(*releaseType)
			if err != nil {
				return shared.UsageErrorf("versions create: %v", err)
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
			if normalizedReleaseType != "" {
				attrs.ReleaseType = normalizedReleaseType
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
			if copyMetadataFromValue != "" {
				copySummary, err := copyVersionMetadataFromSource(
					requestCtx,
					client,
					resolvedAppID,
					normalizedPlatform,
					copyMetadataFromValue,
					resp.Data.ID,
					selectedCopyFields,
				)
				if err != nil {
					return fmt.Errorf("versions create: %w", err)
				}
				if len(copySummary.SkippedLocales) > 0 {
					fmt.Fprintf(os.Stderr, "Warning: skipped source locales not enabled on destination: %s\n", strings.Join(copySummary.SkippedLocales, ", "))
				}
				result.MetadataCopy = copySummary
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func VersionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions update", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	legacyID := shared.BindDeprecatedStringFlagAlias(fs, "id", "version-id")
	copyright := fs.String("copyright", "", "Copyright text (e.g., '2026 My Company')")
	releaseType := fs.String("release-type", "", "Release type: MANUAL, AFTER_APPROVAL, SCHEDULED")
	earliestReleaseDate := fs.String("earliest-release-date", "", "Earliest release date (ISO 8601, e.g., 2026-02-01T08:00:00+00:00)")
	versionString := fs.String("version", "", "Version string (e.g., 1.0.1)")
	output := shared.BindOutputFlags(fs)

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
			if err := legacyID.Apply(versionID); err != nil {
				return err
			}
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}

			normalizedReleaseType, err := normalizeAppStoreVersionReleaseType(*releaseType)
			if err != nil {
				return shared.UsageErrorf("versions update: %v", err)
			}

			// Check that at least one update field is provided
			if *copyright == "" && normalizedReleaseType == "" && *earliestReleaseDate == "" && *versionString == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one of --copyright, --release-type, --earliest-release-date, or --version is required")
				return shared.MissingRequiredUsageError("")
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
			if normalizedReleaseType != "" {
				attrs.ReleaseType = &normalizedReleaseType
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func VersionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions delete", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--version-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete a version")
				return shared.MissingRequiredUsageError("--confirm")
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

			result := map[string]any{
				"versionId": strings.TrimSpace(*versionID),
				"deleted":   true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func VersionsAttachBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions attach-build", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	buildID := fs.String("build-id", "", "Build ID to attach (required)")
	legacyBuildID := shared.BindDeprecatedStringFlagAlias(fs, "build", "build-id")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach-build",
		ShortUsage: "asc versions attach-build [flags]",
		ShortHelp:  "Attach a build to an app store version.",
		LongHelp: `Attach a build to an app store version.

To find the version and build IDs, list each resource for the app and use its
returned id field:
  asc versions list --app "APP_ID" --paginate --output json
  asc builds list --app "APP_ID" --paginate --output json

Examples:
  asc versions attach-build --version-id "VERSION_ID" --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyBuildID.Apply(buildID); err != nil {
				return err
			}
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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

func normalizeAppStoreVersionReleaseType(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if value != "" && trimmed == "" {
		return "", fmt.Errorf("--release-type must be one of: MANUAL, AFTER_APPROVAL, SCHEDULED")
	}

	normalized := strings.ToUpper(trimmed)
	switch normalized {
	case "", "MANUAL", "AFTER_APPROVAL", "SCHEDULED":
		return normalized, nil
	default:
		return "", fmt.Errorf("--release-type must be one of: MANUAL, AFTER_APPROVAL, SCHEDULED")
	}
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

func splitCompatAppStoreVersionIncludes(values []string) ([]string, bool) {
	apiIncludes := make([]string, 0, len(values))
	includeAgeRating := false
	for _, value := range values {
		if value == "ageRatingDeclaration" {
			includeAgeRating = true
			continue
		}
		apiIncludes = append(apiIncludes, value)
	}
	return apiIncludes, includeAgeRating
}

func appendAgeRatingDeclarationInclude(versionResp *asc.AppStoreVersionResponse, ageRatingResp *asc.AgeRatingDeclarationResponse) error {
	if versionResp == nil || ageRatingResp == nil {
		return nil
	}

	relationship := struct {
		Data asc.ResourceData `json:"data"`
	}{
		Data: asc.ResourceData{
			Type: asc.ResourceTypeAgeRatingDeclarations,
			ID:   ageRatingResp.Data.ID,
		},
	}

	relationships := map[string]json.RawMessage{}
	if len(versionResp.Data.Relationships) > 0 {
		if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
			return fmt.Errorf("failed to parse app store version relationships: %w", err)
		}
	}

	if _, exists := relationships["ageRatingDeclaration"]; !exists {
		rawRelationship, err := json.Marshal(relationship)
		if err != nil {
			return fmt.Errorf("failed to encode age rating relationship: %w", err)
		}
		relationships["ageRatingDeclaration"] = rawRelationship
		rawRelationships, err := json.Marshal(relationships)
		if err != nil {
			return fmt.Errorf("failed to encode app store version relationships: %w", err)
		}
		versionResp.Data.Relationships = rawRelationships
	}

	included := make([]json.RawMessage, 0, 1)
	if len(versionResp.Included) > 0 {
		if err := json.Unmarshal(versionResp.Included, &included); err != nil {
			return fmt.Errorf("failed to parse included resources: %w", err)
		}
	}

	for _, item := range included {
		var resource asc.Resource[map[string]any]
		if err := json.Unmarshal(item, &resource); err != nil {
			return fmt.Errorf("failed to inspect included resource: %w", err)
		}
		if resource.Type == asc.ResourceTypeAgeRatingDeclarations && resource.ID == ageRatingResp.Data.ID {
			return nil
		}
	}

	rawAgeRating, err := json.Marshal(ageRatingResp.Data)
	if err != nil {
		return fmt.Errorf("failed to encode age rating include: %w", err)
	}
	included = append(included, rawAgeRating)

	rawIncluded, err := json.Marshal(included)
	if err != nil {
		return fmt.Errorf("failed to encode included resources: %w", err)
	}
	versionResp.Included = rawIncluded
	return nil
}
