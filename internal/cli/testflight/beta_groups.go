package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const buildGroupMembershipTimeout = 5 * time.Minute

// betaGroupSortValues lists the sort values GET /v1/betaGroups documents.
var betaGroupSortValues = []string{
	"name",
	"-name",
	"createdDate",
	"-createdDate",
	"publicLinkEnabled",
	"-publicLinkEnabled",
	"publicLinkLimit",
	"-publicLinkLimit",
}

var betaGroupIncludeValues = []string{
	"app",
	"builds",
	"betaTesters",
	"betaRecruitmentCriteria",
}

// These sparse-field values mirror the exact GET /v1/betaGroups OpenAPI
// enums. Keep relationship fields separate because Apple validates each
// resource type independently.
var betaGroupFieldsValues = []string{
	"name",
	"createdDate",
	"isInternalGroup",
	"hasAccessToAllBuilds",
	"publicLinkEnabled",
	"publicLinkId",
	"publicLinkLimitEnabled",
	"publicLinkLimit",
	"publicLink",
	"feedbackEnabled",
	"iosBuildsAvailableForAppleSiliconMac",
	"iosBuildsAvailableForAppleVision",
	"app",
	"builds",
	"betaTesters",
	"betaRecruitmentCriteria",
	"betaRecruitmentCriterionCompatibleBuildCheck",
}

var betaGroupAppFieldsValues = []string{
	"accessibilityUrl",
	"name",
	"bundleId",
	"sku",
	"primaryLocale",
	"isOrEverWasMadeForKids",
	"subscriptionStatusUrl",
	"subscriptionStatusUrlVersion",
	"subscriptionStatusUrlForSandbox",
	"subscriptionStatusUrlVersionForSandbox",
	"contentRightsDeclaration",
	"streamlinedPurchasingEnabled",
	"accessibilityDeclarations",
	"appEncryptionDeclarations",
	"appStoreIcon",
	"ciProduct",
	"betaTesters",
	"betaGroups",
	"appStoreVersions",
	"appTags",
	"preReleaseVersions",
	"betaAppLocalizations",
	"builds",
	"betaLicenseAgreement",
	"betaAppReviewDetail",
	"appInfos",
	"appClips",
	"appPricePoints",
	"endUserLicenseAgreement",
	"appPriceSchedule",
	"appAvailabilityV2",
	"inAppPurchases",
	"subscriptionGroups",
	"gameCenterEnabledVersions",
	"perfPowerMetrics",
	"appCustomProductPages",
	"inAppPurchasesV2",
	"promotedPurchases",
	"appEvents",
	"reviewSubmissions",
	"subscriptionGracePeriod",
	"customerReviews",
	"customerReviewSummarizations",
	"gameCenterDetail",
	"appStoreVersionExperimentsV2",
	"alternativeDistributionKey",
	"analyticsReportRequests",
	"marketplaceSearchDetail",
	"buildUploads",
	"backgroundAssets",
	"betaFeedbackScreenshotSubmissions",
	"betaFeedbackCrashSubmissions",
	"searchKeywords",
	"webhooks",
	"androidToIosAppMappingDetails",
}

var betaGroupBuildFieldsValues = []string{
	"version",
	"uploadedDate",
	"expirationDate",
	"expired",
	"minOsVersion",
	"lsMinimumSystemVersion",
	"computedMinMacOsVersion",
	"computedMinVisionOsVersion",
	"iconAssetToken",
	"processingState",
	"buildAudienceType",
	"usesNonExemptEncryption",
	"preReleaseVersion",
	"individualTesters",
	"betaGroups",
	"betaBuildLocalizations",
	"appEncryptionDeclaration",
	"betaAppReviewSubmission",
	"app",
	"buildBetaDetail",
	"appStoreVersion",
	"icons",
	"buildBundles",
	"buildUpload",
	"perfPowerMetrics",
	"diagnosticSignatures",
}

var betaGroupTesterFieldsValues = []string{
	"firstName",
	"lastName",
	"email",
	"inviteType",
	"state",
	"appDevices",
	"apps",
	"betaGroups",
	"builds",
}

var betaGroupRecruitmentCriteriaFieldsValues = []string{
	"lastModifiedDate",
	"deviceFamilyOsVersionFilters",
}

// Flag sets for `testflight groups list --build-id` conflicts. Help, validation,
// and tests share these slices so a newly rejected flag cannot land undocumented.
var (
	betaGroupsListMembershipPageControlFlags = []string{"global", "limit", "next", "paginate"}
	betaGroupsListNameSortFlags              = []string{"name", "sort"}
	betaGroupsListQuerySurfaceFlags          = []string{
		"id",
		"public-link-enabled",
		"public-link-limit-enabled",
		"public-link",
		"fields",
		"app-fields",
		"build-fields",
		"tester-fields",
		"recruitment-criteria-fields",
		"include",
		"testers-limit",
		"builds-limit",
	}
	betaGroupsListBuildIDCompatibleFlags = []string{"app", "internal", "external"}
)

func joinDashedFlagNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	flags := make([]string, len(names))
	for i, name := range names {
		flags[i] = "--" + name
	}
	switch len(flags) {
	case 1:
		return flags[0]
	case 2:
		return flags[0] + " and " + flags[1]
	default:
		return strings.Join(flags[:len(flags)-1], ", ") + ", and " + flags[len(flags)-1]
	}
}

// BetaGroupsCommand returns the beta groups command with subcommands.
func BetaGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-groups",
		ShortUsage: "asc testflight beta-groups <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta groups.",
		LongHelp: `Manage TestFlight beta groups.

Examples:
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-groups list --build-id "BUILD_ID"
  asc testflight beta-groups list --app "APP_ID" --internal
  asc testflight beta-groups list --global --internal
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
  asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal
  asc testflight beta-groups app view --group-id "GROUP_ID"
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsListCommand(),
			BetaGroupsCreateCommand(),
			BetaGroupsGetCommand(),
			BetaGroupsAppCommand(),
			BetaGroupsRecruitmentCriteriaCommand(),
			BetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand(),
			BetaGroupsUpdateCommand(),
			BetaGroupsAddTestersCommand(),
			BetaGroupsRemoveTestersCommand(),
			BetaGroupsRelationshipsCommand(),
			BetaGroupsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsListCommand returns the beta groups list subcommand.
func BetaGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	buildID := fs.String("build-id", "", "List groups that contain this build ID")
	global := fs.Bool("global", false, "List beta groups across all apps (top-level endpoint)")
	internal := fs.Bool("internal", false, "Filter to internal groups only")
	external := fs.Bool("external", false, "Filter to external groups only")
	name := fs.String("name", "", "Filter to beta groups with this exact name")
	sort := fs.String("sort", "", "Sort order ("+strings.Join(betaGroupSortValues, ", ")+")")
	id := fs.String("id", "", "Filter by beta group ID(s), comma-separated")
	publicLinkEnabled := fs.String("public-link-enabled", "", "Filter by public link enabled state (true or false)")
	publicLinkLimitEnabled := fs.String("public-link-limit-enabled", "", "Filter by public link limit enabled state (true or false)")
	publicLink := fs.String("public-link", "", "Filter by public link value")
	fields := fs.String("fields", "", "Fields to include for beta groups, comma-separated")
	appFields := fs.String("app-fields", "", "Fields to include for related apps, comma-separated")
	buildFields := fs.String("build-fields", "", "Fields to include for related builds, comma-separated")
	testerFields := fs.String("tester-fields", "", "Fields to include for related beta testers, comma-separated")
	recruitmentCriteriaFields := fs.String("recruitment-criteria-fields", "", "Fields to include for related beta recruitment criteria, comma-separated")
	include := fs.String("include", "", "Include related resources: "+strings.Join(betaGroupIncludeValues, ", "))
	testersLimit := fs.Int("testers-limit", 0, "Maximum included beta testers (1-50)")
	buildsLimit := fs.Int("builds-limit", 0, "Maximum included builds (1-1000)")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-groups list [flags]",
		ShortHelp:  "List TestFlight beta groups for an app or globally.",
		LongHelp: `List TestFlight beta groups for an app or globally.

The --build-id lookup resolves the build's app and
automatically paginates the app's groups, returning both explicit build
relationships and groups with all-build access.
App Store Connect exposes no GET /v1/builds/{id}/relationships/betaGroups. It
does document include=betaGroups on GET /v1/builds/{id}, but that read caps
included groups at limit[betaGroups]=50, has no documented build-side endpoint
for paging past the cap, and reports the same explicit linkage Apple can omit
for all-build groups. So the command prefers the documented betaGroups build
filter. If that filter is rejected, the command falls back to the inverse
group-to-build relationship.
All-build groups omitted by the filter are also checked through that inverse
linkage because Apple can omit their explicit relationships. These checks scan
linkage IDs only; cost scales with the checked groups and their build page count.

A complete lookup with no memberships prints an empty groups array and exits 0.
If an inverse relationship cannot be read, available matches and failures are
printed with complete=false and the command exits nonzero.

GET /v1/apps/{id}/betaGroups accepts only limit and fields[betaGroups], so
--fields alone stays on that app-scoped endpoint. --internal, --external,
--name, --sort, and the other query flags are served by GET /v1/betaGroups
with filter[app]. Those filters are applied by App Store Connect. For ordinary
one-page filtered and global listings, --limit is the page size. The stable
app-scoped --internal/--external aggregate fetches with the maximum page size
of 200 before applying --limit as the final cap. --name matches the exact group name.
The top-level endpoint also supports --id, --public-link-enabled,
--public-link-limit-enabled, and --public-link filters. Use --include with
--fields, --app-fields, --build-fields, --tester-fields, or
--recruitment-criteria-fields to shape related resources. --testers-limit and
--builds-limit cap included beta testers and builds respectively.
--build-id membership lookup is compatible only with --app, --internal, and --external
among query flags. Output flags still apply. It rejects page controls
(--global, --limit, --next, --paginate), --name, --sort,
and query-surface flags (--id, --public-link-enabled, --public-link-limit-enabled,
--public-link, --fields, --app-fields, --build-fields, --tester-fields,
--recruitment-criteria-fields, --include, --testers-limit, and --builds-limit).

App-scoped --internal and --external continue to collect every matching page
automatically for compatibility when --name and --sort are absent. Combined
filters return one page unless --paginate is set. Global listings also keep
their standard one-page default.

Explicit --paginate uses a page size of 200 instead of --limit. For the stable
app-scoped --internal/--external behavior, --limit still caps the final
aggregate after every page is fetched.

A links.next URL already carries the query it came from, so --next cannot be
combined with query-shaping flags such as --internal, --external, --name,
--sort, --id, --public-link-enabled, --public-link-limit-enabled,
--public-link, --fields, --app-fields, --build-fields, --tester-fields,
--recruitment-criteria-fields, --include, --testers-limit, or --builds-limit.

Examples:
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-groups list --build-id "BUILD_ID"
  asc testflight beta-groups list --build-id "BUILD_ID" --internal
  asc testflight beta-groups list --app "APP_ID" --internal
  asc testflight beta-groups list --app "APP_ID" --external
  asc testflight beta-groups list --app "APP_ID" --name "Beta Testers"
  asc testflight beta-groups list --app "APP_ID" --sort "-createdDate"
  asc testflight beta-groups list --global --public-link-enabled true
  asc testflight beta-groups list --global --include app,builds --builds-limit 100
  asc testflight beta-groups list --app "APP_ID" --limit 10
  asc testflight beta-groups list --app "APP_ID" --paginate
  asc testflight beta-groups list --global
  asc testflight beta-groups list --global --limit 50
  asc testflight beta-groups list --global --internal
  asc testflight beta-groups list --global --sort "name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.UsageErrorCtx(ctx, "beta-groups list: --limit must be between 1 and 200"),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "beta-groups list: %v", err)
			}

			appIDSet := false
			buildIDSet := false
			nameSet := false
			sortSet := false
			idSet := false
			publicLinkEnabledSet := false
			publicLinkLimitEnabledSet := false
			publicLinkSet := false
			fieldsSet := false
			appFieldsSet := false
			buildFieldsSet := false
			testerFieldsSet := false
			recruitmentCriteriaFieldsSet := false
			includeSet := false
			testersLimitSet := false
			buildsLimitSet := false
			membershipPageControlSet := false
			nameSortSet := false
			querySurfaceSet := false
			fs.Visit(func(value *flag.Flag) {
				switch value.Name {
				case "app":
					appIDSet = true
				case "build-id":
					buildIDSet = true
				case "name":
					nameSet = true
				case "sort":
					sortSet = true
				case "id":
					idSet = true
				case "public-link-enabled":
					publicLinkEnabledSet = true
				case "public-link-limit-enabled":
					publicLinkLimitEnabledSet = true
				case "public-link":
					publicLinkSet = true
				case "fields":
					fieldsSet = true
				case "app-fields":
					appFieldsSet = true
				case "build-fields":
					buildFieldsSet = true
				case "tester-fields":
					testerFieldsSet = true
				case "recruitment-criteria-fields":
					recruitmentCriteriaFieldsSet = true
				case "include":
					includeSet = true
				case "testers-limit":
					testersLimitSet = true
				case "builds-limit":
					buildsLimitSet = true
				}
				if slices.Contains(betaGroupsListMembershipPageControlFlags, value.Name) {
					membershipPageControlSet = true
				}
				if slices.Contains(betaGroupsListNameSortFlags, value.Name) {
					nameSortSet = true
				}
				if slices.Contains(betaGroupsListQuerySurfaceFlags, value.Name) {
					querySurfaceSet = true
				}
			})

			sortValue := strings.TrimSpace(*sort)
			if sortSet && sortValue == "" {
				return shared.UsageError("beta-groups list: --sort cannot be empty")
			}
			if err := shared.ValidateSort(sortValue, betaGroupSortValues...); err != nil {
				return shared.UsageError(err.Error())
			}
			nameValue := strings.TrimSpace(*name)
			if nameSet && nameValue == "" {
				return shared.UsageError("beta-groups list: --name cannot be empty")
			}

			// Both beta group reads follow a links.next URL verbatim, so any
			// query-shaping flag passed alongside --next would be dropped
			// without a trace. Reject the combination instead: the cursor URL
			// already carries the filters and sort of the query it came from.
			if strings.TrimSpace(*next) != "" {
				if err := shared.RejectNextFlagConflicts(
					fs,
					*next,
					"beta-groups list",
					"id",
					"public-link-enabled",
					"public-link-limit-enabled",
					"public-link",
					"fields",
					"app-fields",
					"build-fields",
					"tester-fields",
					"recruitment-criteria-fields",
					"include",
					"testers-limit",
					"builds-limit",
				); err != nil {
					return err
				}
				for _, conflict := range []struct {
					set  bool
					name string
				}{
					{*internal, "--internal"},
					{*external, "--external"},
					{nameSet, "--name"},
					{sortSet, "--sort"},
				} {
					if conflict.set {
						return shared.UsageError("beta-groups list: --next cannot be combined with " + conflict.name)
					}
				}
			}

			publicLinkEnabledValue, err := parseBetaGroupsListBool("--public-link-enabled", *publicLinkEnabled, publicLinkEnabledSet)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			publicLinkLimitEnabledValue, err := parseBetaGroupsListBool("--public-link-limit-enabled", *publicLinkLimitEnabled, publicLinkLimitEnabledSet)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			idValues, err := parseBetaGroupsListCSV("--id", *id, idSet)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			publicLinkValue := strings.TrimSpace(*publicLink)
			if publicLinkSet && publicLinkValue == "" {
				return shared.UsageError("beta-groups list: --public-link cannot be empty")
			}
			fieldsValue, err := parseBetaGroupsListFields("--fields", *fields, fieldsSet, betaGroupFieldsValues)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			appFieldsValue, err := parseBetaGroupsListFields("--app-fields", *appFields, appFieldsSet, betaGroupAppFieldsValues)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			buildFieldsValue, err := parseBetaGroupsListFields("--build-fields", *buildFields, buildFieldsSet, betaGroupBuildFieldsValues)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			testerFieldsValue, err := parseBetaGroupsListFields("--tester-fields", *testerFields, testerFieldsSet, betaGroupTesterFieldsValues)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			recruitmentCriteriaFieldsValue, err := parseBetaGroupsListFields("--recruitment-criteria-fields", *recruitmentCriteriaFields, recruitmentCriteriaFieldsSet, betaGroupRecruitmentCriteriaFieldsValues)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			includeValue, err := shared.NormalizeSelection(*include, betaGroupIncludeValues, "--include")
			if err != nil {
				return shared.UsageError("beta-groups list: " + err.Error())
			}
			if includeSet && len(includeValue) == 0 {
				return shared.UsageError("beta-groups list: --include cannot be empty")
			}
			if (len(appFieldsValue) > 0 || appFieldsSet) && !slices.Contains(includeValue, "app") {
				return shared.UsageError("beta-groups list: --app-fields requires --include app")
			}
			if (len(buildFieldsValue) > 0 || buildFieldsSet) && !slices.Contains(includeValue, "builds") {
				return shared.UsageError("beta-groups list: --build-fields requires --include builds")
			}
			if (len(testerFieldsValue) > 0 || testerFieldsSet) && !slices.Contains(includeValue, "betaTesters") {
				return shared.UsageError("beta-groups list: --tester-fields requires --include betaTesters")
			}
			if (len(recruitmentCriteriaFieldsValue) > 0 || recruitmentCriteriaFieldsSet) && !slices.Contains(includeValue, "betaRecruitmentCriteria") {
				return shared.UsageError("beta-groups list: --recruitment-criteria-fields requires --include betaRecruitmentCriteria")
			}
			if testersLimitSet && (*testersLimit < 1 || *testersLimit > 50) {
				return shared.UsageError("beta-groups list: --testers-limit must be between 1 and 50")
			}
			if buildsLimitSet && (*buildsLimit < 1 || *buildsLimit > 1000) {
				return shared.UsageError("beta-groups list: --builds-limit must be between 1 and 1000")
			}
			if testersLimitSet && !slices.Contains(includeValue, "betaTesters") {
				return shared.UsageError("beta-groups list: --testers-limit requires --include betaTesters")
			}
			if buildsLimitSet && !slices.Contains(includeValue, "builds") {
				return shared.UsageError("beta-groups list: --builds-limit requires --include builds")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			resolvedBuildID := strings.TrimSpace(*buildID)
			queryFilterSet := idSet || publicLinkEnabledSet || publicLinkLimitEnabledSet || publicLinkSet

			if *internal && *external {
				fmt.Fprintln(os.Stderr, "Error: --internal and --external are mutually exclusive")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if buildIDSet && resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id cannot be empty")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--build-id")
			}
			if resolvedBuildID != "" && appIDSet && strings.TrimSpace(*appID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be empty when used with --build-id")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--app")
			}
			if resolvedBuildID != "" && membershipPageControlSet {
				fmt.Fprintf(os.Stderr, "Error: %s cannot be used with --build-id; membership lookup always fetches all required pages\n", joinDashedFlagNames(betaGroupsListMembershipPageControlFlags))
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if resolvedBuildID != "" && nameSortSet {
				fmt.Fprintf(os.Stderr, "Error: %s cannot be used with --build-id; membership lookup queries the build's app relationships directly\n", joinDashedFlagNames(betaGroupsListNameSortFlags))
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if resolvedBuildID != "" && querySurfaceSet {
				fmt.Fprintln(os.Stderr, "Error: testflight groups query filters, sparse fields, includes, and relationship limits cannot be used with --build-id; membership lookup queries the build's app relationships directly")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			if resolvedBuildID != "" {
				expectedAppID := ""
				if appIDSet {
					expectedAppID = strings.TrimSpace(*appID)
				}
				var internalFilter *bool
				if *internal {
					value := true
					internalFilter = &value
				} else if *external {
					value := false
					internalFilter = &value
				}

				return runBuildGroupMembershipList(
					ctx,
					resolvedBuildID,
					expectedAppID,
					internalFilter,
					*output.Output,
					*output.Pretty,
					"beta-groups list",
				)
			}

			// Reject --global + --app combination (check explicit flag, not resolved value)
			if *global && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --global and --app are mutually exclusive")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			// Require one of --app or --global (unless --next is provided)
			if !*global && resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app or --global is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var internalFilter *bool
			if *internal {
				v := true
				internalFilter = &v
			} else if *external {
				v := false
				internalFilter = &v
			}

			opts := []asc.BetaGroupsOption{
				asc.WithBetaGroupsLimit(*limit),
				asc.WithBetaGroupsNextURL(*next),
			}
			if internalFilter != nil {
				opts = append(opts, asc.WithBetaGroupsIsInternal(*internalFilter))
			}
			if nameValue != "" {
				opts = append(opts, asc.WithBetaGroupsName(nameValue))
			}
			if sortValue != "" {
				opts = append(opts, asc.WithBetaGroupsSort(sortValue))
			}
			if publicLinkEnabledValue != nil {
				opts = append(opts, asc.WithBetaGroupsPublicLinkEnabled(*publicLinkEnabledValue))
			}
			if publicLinkLimitEnabledValue != nil {
				opts = append(opts, asc.WithBetaGroupsPublicLinkLimitEnabled(*publicLinkLimitEnabledValue))
			}
			if len(idValues) > 0 {
				opts = append(opts, asc.WithBetaGroupsIDs(idValues))
			}
			if publicLinkValue != "" {
				opts = append(opts, asc.WithBetaGroupsPublicLink(publicLinkValue))
			}
			opts = append(
				opts,
				asc.WithBetaGroupsFields(fieldsValue),
				asc.WithBetaGroupsAppFields(appFieldsValue),
				asc.WithBetaGroupsBuildFields(buildFieldsValue),
				asc.WithBetaGroupsBetaTesterFields(testerFieldsValue),
				asc.WithBetaGroupsBetaRecruitmentCriteriaFields(recruitmentCriteriaFieldsValue),
				asc.WithBetaGroupsInclude(includeValue),
				asc.WithBetaGroupsBetaTestersLimit(*testersLimit),
				asc.WithBetaGroupsBuildsLimit(*buildsLimit),
			)

			// GET /v1/apps/{id}/betaGroups accepts only limit and
			// fields[betaGroups]. GET /v1/betaGroups accepts filter[app]
			// alongside filter[isInternalGroup], filter[name], and sort, so any
			// request that needs one of those is routed there instead of being
			// narrowed client-side after walking every page.
			useTopLevelEndpoint := *global || internalFilter != nil || nameValue != "" || sortValue != "" ||
				len(idValues) > 0 || publicLinkEnabledValue != nil || publicLinkLimitEnabledValue != nil || publicLinkValue != "" ||
				len(appFieldsValue) > 0 || len(buildFieldsValue) > 0 || len(testerFieldsValue) > 0 ||
				len(recruitmentCriteriaFieldsValue) > 0 || len(includeValue) > 0 || *testersLimit > 0 || *buildsLimit > 0
			if useTopLevelEndpoint && !*global && resolvedAppID != "" {
				opts = append(opts, asc.WithBetaGroupsApps([]string{resolvedAppID}))
			}

			listPage := func(ctx context.Context, pageOpts ...asc.BetaGroupsOption) (asc.PaginatedResponse, error) {
				if useTopLevelEndpoint {
					return client.ListBetaGroups(ctx, pageOpts...)
				}
				return client.GetBetaGroups(ctx, resolvedAppID, pageOpts...)
			}

			// App-scoped internal/external filtering historically returned every
			// matching page without requiring --paginate. Keep that stable behavior
			// while moving the filtering itself to the top-level endpoint. The new
			// name/sort flags retain the normal one-page default.
			stableAppScopedFilter := !*global && resolvedAppID != "" && internalFilter != nil && nameValue == "" && sortValue == "" && !queryFilterSet
			if stableAppScopedFilter && !*paginate {
				// Fetch with Apple's maximum page size before applying the
				// stable client-side cap. Passing a small --limit here would
				// make a large filtered set require one request per page.
				firstPageOpts := append(slices.Clone(opts), asc.WithBetaGroupsLimit(200))
				firstPage, err := listPage(requestCtx, firstPageOpts...)
				if err != nil {
					return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
				}
				groups, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return listPage(ctx, asc.WithBetaGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-groups list: %w", err)
				}
				if err := preserveFilteredBetaGroupsLimit(groups, *limit); err != nil {
					return fmt.Errorf("beta-groups list: %w", err)
				}

				return shared.PrintOutput(groups, *output.Output, *output.Pretty)
			}

			if *paginate {
				paginateOpts := append(slices.Clone(opts), asc.WithBetaGroupsLimit(200))
				groups, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return listPage(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return listPage(ctx, asc.WithBetaGroupsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("beta-groups list: %w", err)
				}
				if stableAppScopedFilter {
					if err := preserveFilteredBetaGroupsLimit(groups, *limit); err != nil {
						return fmt.Errorf("beta-groups list: %w", err)
					}
				}

				return shared.PrintOutput(groups, *output.Output, *output.Pretty)
			}

			groups, err := listPage(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(groups, *output.Output, *output.Pretty)
		},
	}
}

func preserveFilteredBetaGroupsLimit(groups asc.PaginatedResponse, limit int) error {
	if limit <= 0 {
		return nil
	}

	response, ok := groups.(*asc.BetaGroupsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type %T", groups)
	}
	if len(response.Data) <= limit {
		return nil
	}

	total := len(response.Data)
	response.Data = response.Data[:limit]
	fmt.Fprintf(os.Stderr, "Warning: showing %d of %d filtered groups (--limit %d); rerun without --limit for all\n", limit, total, limit)
	return nil
}

func parseBetaGroupsListBool(flagName, value string, set bool) (*bool, error) {
	if !set {
		return nil, nil
	}
	parsed, err := shared.ParseOptionalBoolFlag(flagName, value)
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, fmt.Errorf("%s must be true or false", flagName)
	}
	return parsed, nil
}

func parseBetaGroupsListCSV(flagName, value string, set bool) ([]string, error) {
	values := shared.SplitCSV(value)
	if set && len(values) == 0 {
		return nil, fmt.Errorf("beta-groups list: %s cannot be empty", flagName)
	}
	return values, nil
}

func parseBetaGroupsListFields(flagName, value string, set bool, allowed []string) ([]string, error) {
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, err
	}
	if set && len(values) == 0 {
		return nil, fmt.Errorf("beta-groups list: %s cannot be empty", flagName)
	}
	return values, nil
}

// BuildGroupsListCommandConfig configures the build-centric beta-group lookup
// surface while keeping the membership implementation in one place.
type BuildGroupsListCommandConfig struct {
	ShortUsage  string
	ShortHelp   string
	LongHelp    string
	ErrorPrefix string
}

// BuildGroupsListCommand returns a narrow list command for beta groups that
// contain a required build ID.
func BuildGroupsListCommand(config BuildGroupsListCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build-id", "", "Build ID whose TestFlight groups should be listed")
	output := shared.BindOutputFlags(fs)

	errorPrefix := strings.TrimSpace(config.ErrorPrefix)
	if errorPrefix == "" {
		errorPrefix = "builds groups list"
	}

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBuildID := strings.TrimSpace(*buildID)
			buildIDSet := false
			fs.Visit(func(value *flag.Flag) {
				if value.Name == "build-id" {
					buildIDSet = true
				}
			})
			if buildIDSet && resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id cannot be empty")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--build-id")
			}
			if resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
			}

			return runBuildGroupMembershipList(
				ctx,
				resolvedBuildID,
				"",
				nil,
				*output.Output,
				*output.Pretty,
				errorPrefix,
			)
		},
	}
}

func runBuildGroupMembershipList(
	ctx context.Context,
	buildID string,
	expectedAppID string,
	internalFilter *bool,
	output string,
	pretty bool,
	errorPrefix string,
) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	requestCtx, cancel := contextWithBuildGroupMembershipTimeout(ctx)
	defer cancel()

	result, usedFallback, lookupErr := lookupBuildGroupMembership(
		requestCtx,
		client,
		buildID,
		expectedAppID,
		internalFilter,
	)
	if usedFallback {
		fmt.Fprintln(os.Stderr, "Apple rejected the documented betaGroups build filter; falling back to inverse group build relationships (cost scales with groups and build pages)")
	}
	if result == nil {
		return fmt.Errorf("%s: %w", errorPrefix, lookupErr)
	}
	if err := shared.PrintOutput(result, output, pretty); err != nil {
		return err
	}
	if lookupErr != nil {
		fmt.Fprintf(os.Stderr, "%d group relationship lookup failed; membership result is incomplete\n", len(result.Failures))
		return shared.NewReportedError(lookupErr)
	}
	return nil
}

func contextWithBuildGroupMembershipTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedTimeout(ctx, buildGroupMembershipTimeout)
}

// BetaGroupsCreateCommand returns the beta groups create subcommand.
func BetaGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	name := fs.String("name", "", "Beta group name")
	var internal shared.OptionalBool
	internal.EnableBoolFlag()
	fs.Var(&internal, "internal", "Create as internal group")
	var accessAllBuilds shared.OptionalBool
	accessAllBuilds.EnableBoolFlag()
	fs.Var(&accessAllBuilds, "access-all-builds", "Give the group access to all builds")
	var publicLinkEnabled shared.OptionalBool
	publicLinkEnabled.EnableBoolFlag()
	fs.Var(&publicLinkEnabled, "public-link-enabled", "Enable the public link")
	var publicLinkLimitEnabled shared.OptionalBool
	publicLinkLimitEnabled.EnableBoolFlag()
	fs.Var(&publicLinkLimitEnabled, "public-link-limit-enabled", "Enable the public link tester limit")
	publicLinkLimit := fs.Int("public-link-limit", 0, "Public link tester limit (1-10000)")
	var feedbackEnabled shared.OptionalBool
	feedbackEnabled.EnableBoolFlag()
	fs.Var(&feedbackEnabled, "feedback-enabled", "Enable tester feedback")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc testflight beta-groups create [flags]",
		ShortHelp:  "Create a TestFlight beta group.",
		LongHelp: `Create a TestFlight beta group.

Examples:
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
  asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal --access-all-builds
  asc testflight beta-groups create --app "APP_ID" --name "Public Beta" --public-link-enabled --public-link-limit-enabled --public-link-limit 250 --feedback-enabled`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if internal.Value() && (publicLinkEnabled.IsSet() || publicLinkLimitEnabled.IsSet() || visited["public-link-limit"]) {
				fmt.Fprintln(os.Stderr, "Error: --internal cannot be combined with public link controls")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--internal")
			}
			if visited["public-link-limit"] && (*publicLinkLimit < 1 || *publicLinkLimit > 10000) {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit must be between 1 and 10000")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--public-link-limit")
			}
			if publicLinkLimitEnabled.IsSet() && publicLinkLimitEnabled.Value() && !visited["public-link-limit"] {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit is required when enabling public link limit")
				return shared.MissingRequiredUsageError("--public-link-limit")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var publicLinkLimitAttr *int
			if visited["public-link-limit"] {
				publicLinkLimitAttr = publicLinkLimit
			}
			attrs := asc.BetaGroupCreateAttributes{
				Name:                   strings.TrimSpace(*name),
				IsInternalGroup:        optionalBetaGroupCreateBool(internal),
				HasAccessToAllBuilds:   optionalBetaGroupCreateBool(accessAllBuilds),
				PublicLinkEnabled:      optionalBetaGroupCreateBool(publicLinkEnabled),
				PublicLinkLimitEnabled: optionalBetaGroupCreateBool(publicLinkLimitEnabled),
				PublicLinkLimit:        publicLinkLimitAttr,
				FeedbackEnabled:        optionalBetaGroupCreateBool(feedbackEnabled),
			}

			group, err := client.CreateBetaGroupWithAttributes(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("beta-groups create: failed to create: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
		},
	}
}

func optionalBetaGroupCreateBool(value shared.OptionalBool) *bool {
	if !value.IsSet() {
		return nil
	}
	result := value.Value()
	return &result
}

// BetaGroupsGetCommand returns the beta groups get subcommand.
func BetaGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups view [flags]",
		ShortHelp:  "View a TestFlight beta group by ID.",
		LongHelp: `View a TestFlight beta group by ID.

Examples:
  asc testflight beta-groups view --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			group, err := client.GetBetaGroup(requestCtx, strings.TrimSpace(*id))
			if err != nil {
				return fmt.Errorf("beta-groups view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
		},
	}
}

// BetaGroupsUpdateCommand returns the beta groups update subcommand.
func BetaGroupsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID")
	name := fs.String("name", "", "Beta group name")
	publicLinkEnabled := fs.Bool("public-link-enabled", false, "Enable public link")
	publicLinkLimitEnabled := fs.Bool("public-link-limit-enabled", false, "Enable public link limit")
	publicLinkLimit := fs.Int("public-link-limit", 0, "Public link limit (1-10000)")
	feedbackEnabled := fs.Bool("feedback-enabled", false, "Enable feedback")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-groups update [flags]",
		ShortHelp:  "Update a TestFlight beta group.",
		LongHelp: `Update a TestFlight beta group.

Examples:
  asc testflight beta-groups update --id "GROUP_ID" --name "New Name"
  asc testflight beta-groups update --id "GROUP_ID" --public-link-enabled --public-link-limit 100
  asc testflight beta-groups update --id "GROUP_ID" --feedback-enabled`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if visited["public-link-limit"] && (*publicLinkLimit < 1 || *publicLinkLimit > 10000) {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit must be between 1 and 10000")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--public-link-limit")
			}

			hasUpdates := strings.TrimSpace(*name) != "" ||
				visited["public-link-enabled"] ||
				visited["public-link-limit-enabled"] ||
				visited["public-link-limit"] ||
				visited["feedback-enabled"]
			if !hasUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			if visited["public-link-limit-enabled"] && *publicLinkLimitEnabled && !visited["public-link-limit"] {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit is required when enabling public link limit")
				return shared.MissingRequiredUsageError("--public-link-limit")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var publicLinkEnabledAttr *bool
			var publicLinkLimitEnabledAttr *bool
			var feedbackEnabledAttr *bool

			if visited["public-link-enabled"] {
				publicLinkEnabledAttr = publicLinkEnabled
			}
			if visited["public-link-limit-enabled"] {
				publicLinkLimitEnabledAttr = publicLinkLimitEnabled
			}
			if visited["feedback-enabled"] {
				feedbackEnabledAttr = feedbackEnabled
			}

			req := asc.BetaGroupUpdateRequest{
				Data: asc.BetaGroupUpdateData{
					Type: asc.ResourceTypeBetaGroups,
					ID:   trimmedID,
					Attributes: &asc.BetaGroupUpdateAttributes{
						Name:                   strings.TrimSpace(*name),
						PublicLinkEnabled:      publicLinkEnabledAttr,
						PublicLinkLimitEnabled: publicLinkLimitEnabledAttr,
						PublicLinkLimit:        *publicLinkLimit,
						FeedbackEnabled:        feedbackEnabledAttr,
					},
				},
			}

			group, err := client.UpdateBetaGroup(requestCtx, trimmedID, req)
			if err != nil {
				return fmt.Errorf("beta-groups update: failed to update: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
		},
	}
}

// BetaGroupsDeleteCommand returns the beta groups delete subcommand.
func BetaGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight beta-groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a TestFlight beta group.",
		LongHelp: `Delete a TestFlight beta group.

Examples:
  asc testflight beta-groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaGroup(requestCtx, strings.TrimSpace(*id)); err != nil {
				return fmt.Errorf("beta-groups delete: failed to delete: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully deleted group %s\n", strings.TrimSpace(*id))
			return nil
		},
	}
}

// BetaGroupsAddTestersCommand returns the beta groups add-testers subcommand.
func BetaGroupsAddTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-testers", flag.ExitOnError)

	group := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group ID")
	tester := shared.BindOnceCSVFlag(fs, "tester", "Beta tester ID(s), comma-separated")
	email := shared.BindOnceCSVFlag(fs, "email", "Beta tester email(s), comma-separated")

	return &ffcli.Command{
		Name:       "add-testers",
		ShortUsage: "asc testflight beta-groups add-testers --group \"GROUP_ID\" [--tester \"TESTER_ID[,TESTER_ID...]\" | --email \"EMAIL[,EMAIL...]\"]",
		ShortHelp:  "Add beta testers to a beta group.",
		LongHelp: `Add beta testers to a beta group.

Examples:
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2"
  asc testflight beta-groups add-testers --group "GROUP_ID" --email "tester@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			testerIDs := shared.SplitCSV(tester.String())
			testerEmails := shared.SplitCSV(email.String())
			if len(testerIDs) == 0 && len(testerEmails) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester or --email is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups add-testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if len(testerEmails) > 0 {
				groupApp, err := client.GetBetaGroupApp(requestCtx, groupID)
				if err != nil {
					return fmt.Errorf("beta-groups add-testers: failed to resolve app for group: %w", err)
				}
				appID := strings.TrimSpace(groupApp.Data.ID)
				if appID == "" {
					return fmt.Errorf("beta-groups add-testers: group %q has empty app ID", groupID)
				}

				for _, testerEmail := range testerEmails {
					resp, err := client.GetBetaTesters(
						requestCtx,
						appID,
						asc.WithBetaTestersEmail(testerEmail),
						asc.WithBetaTestersLimit(2),
					)
					if err != nil {
						return fmt.Errorf("beta-groups add-testers: failed to resolve tester email %q: %w", testerEmail, err)
					}
					if len(resp.Data) == 0 {
						return fmt.Errorf("beta-groups add-testers: tester email %q not found for app %q", testerEmail, appID)
					}
					if len(resp.Data) > 1 {
						return fmt.Errorf("beta-groups add-testers: multiple testers found for email %q; use --tester ID", testerEmail)
					}
					testerIDs = append(testerIDs, resp.Data[0].ID)
				}
			}

			if len(testerIDs) == 0 {
				return fmt.Errorf("beta-groups add-testers: no tester IDs resolved")
			}
			seen := make(map[string]struct{}, len(testerIDs))
			deduped := make([]string, 0, len(testerIDs))
			for _, testerID := range testerIDs {
				trimmed := strings.TrimSpace(testerID)
				if trimmed == "" {
					continue
				}
				if _, ok := seen[trimmed]; ok {
					continue
				}
				seen[trimmed] = struct{}{}
				deduped = append(deduped, trimmed)
			}
			testerIDs = deduped

			if err := client.AddBetaTestersToGroup(requestCtx, groupID, testerIDs); err != nil {
				return fmt.Errorf("beta-groups add-testers: failed to add testers: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully added %d tester(s) to group %s\n", len(testerIDs), groupID)
			return nil
		},
	}
}

// BetaGroupsRemoveTestersCommand returns the beta groups remove-testers subcommand.
func BetaGroupsRemoveTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-testers", flag.ExitOnError)

	group := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group ID")
	tester := shared.BindOnceCSVFlag(fs, "tester", "Beta tester ID(s), comma-separated")
	confirm := fs.Bool("confirm", false, "Confirm removal")

	return &ffcli.Command{
		Name:       "remove-testers",
		ShortUsage: "asc testflight beta-groups remove-testers --group \"GROUP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\" --confirm",
		ShortHelp:  "Remove beta testers from a beta group.",
		LongHelp: `Remove beta testers from a beta group.

Examples:
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID" --confirm
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			testerIDs := shared.SplitCSV(tester.String())
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return shared.MissingRequiredUsageError("--tester")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups remove-testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTestersFromGroup(requestCtx, groupID, testerIDs); err != nil {
				return fmt.Errorf("beta-groups remove-testers: failed to remove testers: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully removed %d tester(s) from group %s\n", len(testerIDs), groupID)
			return nil
		},
	}
}
