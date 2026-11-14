package asc

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type feedbackQuery struct {
	listQuery
	includedFieldsQuery
	deviceModels              []string
	osVersions                []string
	appPlatforms              []string
	devicePlatforms           []string
	buildIDs                  []string
	buildPreReleaseVersionIDs []string
	testerIDs                 []string
	sort                      string
	include                   []string
	includeScreenshots        bool
}

type crashQuery struct {
	listQuery
	includedFieldsQuery
	deviceModels              []string
	osVersions                []string
	appPlatforms              []string
	devicePlatforms           []string
	buildIDs                  []string
	buildPreReleaseVersionIDs []string
	testerIDs                 []string
	sort                      string
	include                   []string
}

type betaBuildLocalizationsQuery struct {
	listQuery
	locales  []string
	buildIDs []string
}

type betaTesterUsagesQuery struct {
	listQuery
	period            string
	appID             string
	groupBy           string
	filterBetaTesters string
}

type betaGroupsQuery struct {
	listQuery
	isInternalGroup               *bool
	publicLinkEnabled             *bool
	publicLinkLimitEnabled        *bool
	appIDs                        []string
	buildIDs                      []string
	ids                           []string
	publicLink                    string
	fields                        []string
	appFields                     []string
	buildFields                   []string
	betaTesterFields              []string
	betaRecruitmentCriteriaFields []string
	include                       []string
	betaTestersLimit              int
	buildsLimit                   int
	name                          string
	sort                          string
}

type betaTestersQuery struct {
	listQuery
	includedFieldsQuery
	email        string
	firstName    string
	lastName     string
	ids          []string
	groupIDs     []string
	filterBuilds string
	inviteTypes  []string
	sort         string
	include      []string
	fields       []string
}

type betaAppReviewDetailsQuery struct {
	listQuery
}

type betaAppReviewSubmissionsQuery struct {
	listQuery
	buildIDs     []string
	includeBuild bool
}

func buildFeedbackQuery(query *feedbackQuery) string {
	values := url.Values{}
	if query.includeScreenshots {
		fields := []string{
			"createdDate",
			"comment",
			"email",
			"deviceModel",
			"osVersion",
			"locale",
			"timeZone",
			"architecture",
			"connectionType",
			"pairedAppleWatch",
			"appUptimeInMilliseconds",
			"diskBytesAvailable",
			"diskBytesTotal",
			"batteryPercentage",
			"screenWidthInPoints",
			"screenHeightInPoints",
			"appPlatform",
			"devicePlatform",
			"deviceFamily",
			"buildBundleId",
			"screenshots",
			"build",
			"tester",
		}
		// A sparse fieldset omits any field not listed, including relationship
		// fields. Keep build and tester above even when the caller did not ask
		// for compound resources via include, so enabling screenshots does not
		// suppress the primary resources' relationship linkages. Also append any
		// included relationship defensively if this endpoint gains another one.
		for _, rel := range normalizeList(query.include) {
			if !slices.Contains(fields, rel) {
				fields = append(fields, rel)
			}
		}
		values.Set("fields[betaFeedbackScreenshotSubmissions]", strings.Join(fields, ","))
	}
	addCSV(values, "filter[deviceModel]", query.deviceModels)
	addCSV(values, "filter[osVersion]", query.osVersions)
	addCSV(values, "filter[appPlatform]", query.appPlatforms)
	addCSV(values, "filter[devicePlatform]", query.devicePlatforms)
	addCSV(values, "filter[build]", query.buildIDs)
	addCSV(values, "filter[build.preReleaseVersion]", query.buildPreReleaseVersionIDs)
	addCSV(values, "filter[tester]", query.testerIDs)
	addBetaSubmissionInclude(values, query.include)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildCrashQuery(query *crashQuery) string {
	values := url.Values{}
	addCSV(values, "filter[deviceModel]", query.deviceModels)
	addCSV(values, "filter[osVersion]", query.osVersions)
	addCSV(values, "filter[appPlatform]", query.appPlatforms)
	addCSV(values, "filter[devicePlatform]", query.devicePlatforms)
	addCSV(values, "filter[build]", query.buildIDs)
	addCSV(values, "filter[build.preReleaseVersion]", query.buildPreReleaseVersionIDs)
	addCSV(values, "filter[tester]", query.testerIDs)
	addBetaSubmissionInclude(values, query.include)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildBetaGroupsQuery(query *betaGroupsQuery) string {
	values := url.Values{}
	fields := normalizeList(query.fields)
	if len(fields) > 0 {
		// A primary sparse fieldset must retain every included relationship or
		// App Store Connect can omit the linkage and leave included resources
		// unreachable from the primary data.
		fields = normalizeUniqueList(append(fields, query.include...))
	}
	addCSV(values, "filter[app]", query.appIDs)
	addCSV(values, "filter[builds]", query.buildIDs)
	addCSV(values, "filter[id]", query.ids)
	addValue(values, "filter[name]", query.name)
	addValue(values, "filter[publicLink]", query.publicLink)
	addCSV(values, "fields[betaGroups]", fields)
	addCSV(values, "fields[apps]", query.appFields)
	addCSV(values, "fields[builds]", query.buildFields)
	addCSV(values, "fields[betaTesters]", query.betaTesterFields)
	addCSV(values, "fields[betaRecruitmentCriteria]", query.betaRecruitmentCriteriaFields)
	addCSV(values, "include", query.include)
	addValue(values, "sort", query.sort)
	addLimit(values, query.limit)
	if query.isInternalGroup != nil {
		values.Set("filter[isInternalGroup]", strconv.FormatBool(*query.isInternalGroup))
	}
	if query.publicLinkEnabled != nil {
		values.Set("filter[publicLinkEnabled]", strconv.FormatBool(*query.publicLinkEnabled))
	}
	if query.publicLinkLimitEnabled != nil {
		values.Set("filter[publicLinkLimitEnabled]", strconv.FormatBool(*query.publicLinkLimitEnabled))
	}
	if query.betaTestersLimit > 0 {
		values.Set("limit[betaTesters]", strconv.Itoa(query.betaTestersLimit))
	}
	if query.buildsLimit > 0 {
		values.Set("limit[builds]", strconv.Itoa(query.buildsLimit))
	}
	return values.Encode()
}

func buildBetaTestersQuery(appID string, query *betaTestersQuery) (string, error) {
	// The API allows only one relationship filter at a time. Reject conflicting
	// combinations up front so every call site gets a consistent error instead of
	// silently dropping a filter.
	if strings.TrimSpace(query.filterBuilds) != "" && len(query.groupIDs) > 0 {
		return "", fmt.Errorf("--group cannot be combined with --build-id (API supports only one relationship filter)")
	}

	values := url.Values{}
	if strings.TrimSpace(query.filterBuilds) != "" {
		values.Set("filter[builds]", strings.TrimSpace(query.filterBuilds))
	} else if len(query.groupIDs) > 0 {
		addCSV(values, "filter[betaGroups]", query.groupIDs)
	} else if strings.TrimSpace(appID) != "" {
		values.Set("filter[apps]", strings.TrimSpace(appID))
	}
	if strings.TrimSpace(query.email) != "" {
		values.Set("filter[email]", strings.TrimSpace(query.email))
	}
	if strings.TrimSpace(query.firstName) != "" {
		values.Set("filter[firstName]", strings.TrimSpace(query.firstName))
	}
	if strings.TrimSpace(query.lastName) != "" {
		values.Set("filter[lastName]", strings.TrimSpace(query.lastName))
	}
	addCSV(values, "filter[id]", query.ids)
	addCSV(values, "filter[inviteType]", query.inviteTypes)
	addCSV(values, "include", query.include)
	if fields := normalizeList(query.fields); len(fields) > 0 {
		// A sparse fieldset must keep every included relationship, or Apple
		// returns no linkage to join the included resources to.
		addCSV(values, "fields[betaTesters]", normalizeUniqueList(append(fields, query.include...)))
	}
	addValue(values, "sort", query.sort)
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode(), nil
}

func buildBetaAppReviewDetailsQuery(appID string, query *betaAppReviewDetailsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(appID))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppReviewSubmissionsQuery(query *betaAppReviewSubmissionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[build]", query.buildIDs)
	if query.includeBuild {
		values.Set("include", "build")
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaBuildLocalizationsQuery(query *betaBuildLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[build]", query.buildIDs)
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaTesterUsagesQuery(query *betaTesterUsagesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.period) != "" {
		values.Set("period", strings.TrimSpace(query.period))
	}
	if strings.TrimSpace(query.appID) != "" {
		values.Set("filter[apps]", strings.TrimSpace(query.appID))
	}
	if strings.TrimSpace(query.groupBy) != "" {
		values.Set("groupBy", strings.TrimSpace(query.groupBy))
	}
	if strings.TrimSpace(query.filterBetaTesters) != "" {
		values.Set("filter[betaTesters]", strings.TrimSpace(query.filterBetaTesters))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

// FeedbackOption is a functional option for GetFeedback.
type FeedbackOption func(*feedbackQuery)

// CrashOption is a functional option for GetCrashes.
type CrashOption func(*crashQuery)

// BetaGroupsOption is a functional option for GetBetaGroups.
type BetaGroupsOption func(*betaGroupsQuery)

// BetaTestersOption is a functional option for GetBetaTesters.
type BetaTestersOption func(*betaTestersQuery)

// BetaTesterAppsOption is a functional option for GetBetaTesterApps.
type BetaTesterAppsOption func(*listQuery)

// BetaTesterBetaGroupsOption is a functional option for GetBetaTesterBetaGroups.
type BetaTesterBetaGroupsOption func(*listQuery)

// BetaTesterBuildsOption is a functional option for GetBetaTesterBuilds.
type BetaTesterBuildsOption func(*listQuery)

// BetaTesterUsagesOption is a functional option for beta tester usage metrics.
type BetaTesterUsagesOption func(*betaTesterUsagesQuery)

// BetaAppReviewDetailsOption is a functional option for beta app review details.
type BetaAppReviewDetailsOption func(*betaAppReviewDetailsQuery)

// BetaAppReviewSubmissionsOption is a functional option for beta app review submissions.
type BetaAppReviewSubmissionsOption func(*betaAppReviewSubmissionsQuery)

// AppBetaAppLocalizationsOption is a functional option for GetAppBetaAppLocalizations.
type AppBetaAppLocalizationsOption func(*listQuery)

// BetaBuildLocalizationsOption is a functional option for beta build localizations.
type BetaBuildLocalizationsOption func(*betaBuildLocalizationsQuery)

// WithFeedbackDeviceModels filters feedback by device model(s).
func WithFeedbackDeviceModels(models []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.deviceModels = normalizeList(models)
	}
}

// WithFeedbackOSVersions filters feedback by OS version(s).
func WithFeedbackOSVersions(versions []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.osVersions = normalizeList(versions)
	}
}

// WithFeedbackAppPlatforms filters feedback by app platform(s).
func WithFeedbackAppPlatforms(platforms []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.appPlatforms = normalizeUpperList(platforms)
	}
}

// WithFeedbackDevicePlatforms filters feedback by device platform(s).
func WithFeedbackDevicePlatforms(platforms []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.devicePlatforms = normalizeUpperList(platforms)
	}
}

// WithFeedbackBuildIDs filters feedback by build ID(s).
func WithFeedbackBuildIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithFeedbackBuildPreReleaseVersionIDs filters feedback by pre-release version ID(s).
func WithFeedbackBuildPreReleaseVersionIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.buildPreReleaseVersionIDs = normalizeList(ids)
	}
}

// WithFeedbackTesterIDs filters feedback by tester ID(s).
func WithFeedbackTesterIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.testerIDs = normalizeList(ids)
	}
}

// WithFeedbackSort sets the sort order for feedback.
func WithFeedbackSort(sort string) FeedbackOption {
	return func(q *feedbackQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithFeedbackIncludeScreenshots includes screenshot URLs in feedback responses.
func WithFeedbackIncludeScreenshots() FeedbackOption {
	return func(q *feedbackQuery) {
		q.includeScreenshots = true
	}
}

// WithFeedbackInclude specifies related resources to include in screenshot
// feedback responses (e.g., "build" to surface the build the feedback was
// submitted against, or "tester").
func WithFeedbackInclude(include []string) FeedbackOption {
	return func(q *feedbackQuery) {
		normalized := normalizeList(include)
		if len(normalized) > 0 {
			q.include = normalized
		}
	}
}

// WithCrashDeviceModels filters crashes by device model(s).
func WithCrashDeviceModels(models []string) CrashOption {
	return func(q *crashQuery) {
		q.deviceModels = normalizeList(models)
	}
}

// WithCrashOSVersions filters crashes by OS version(s).
func WithCrashOSVersions(versions []string) CrashOption {
	return func(q *crashQuery) {
		q.osVersions = normalizeList(versions)
	}
}

// WithCrashAppPlatforms filters crashes by app platform(s).
func WithCrashAppPlatforms(platforms []string) CrashOption {
	return func(q *crashQuery) {
		q.appPlatforms = normalizeUpperList(platforms)
	}
}

// WithCrashDevicePlatforms filters crashes by device platform(s).
func WithCrashDevicePlatforms(platforms []string) CrashOption {
	return func(q *crashQuery) {
		q.devicePlatforms = normalizeUpperList(platforms)
	}
}

// WithCrashBuildIDs filters crashes by build ID(s).
func WithCrashBuildIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithCrashBuildPreReleaseVersionIDs filters crashes by pre-release version ID(s).
func WithCrashBuildPreReleaseVersionIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.buildPreReleaseVersionIDs = normalizeList(ids)
	}
}

// WithCrashTesterIDs filters crashes by tester ID(s).
func WithCrashTesterIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.testerIDs = normalizeList(ids)
	}
}

// WithCrashSort sets the sort order for crashes.
func WithCrashSort(sort string) CrashOption {
	return func(q *crashQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithCrashInclude specifies related resources to include in crash submission
// responses (e.g., "build" to surface the build the crash occurred on, or
// "tester"). Including "build" also returns the build's version and
// preReleaseVersion so the marketing version can be resolved.
func WithCrashInclude(include []string) CrashOption {
	return func(q *crashQuery) {
		normalized := normalizeList(include)
		if len(normalized) > 0 {
			q.include = normalized
		}
	}
}

// WithBetaGroupsIsInternal filters beta groups by internal/external groups.
func WithBetaGroupsIsInternal(isInternal bool) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.isInternalGroup = &isInternal
	}
}

// WithBetaGroupsPublicLinkEnabled filters beta groups by public-link state.
func WithBetaGroupsPublicLinkEnabled(enabled bool) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.publicLinkEnabled = &enabled
	}
}

// WithBetaGroupsPublicLinkLimitEnabled filters beta groups by public-link limit state.
func WithBetaGroupsPublicLinkLimitEnabled(enabled bool) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.publicLinkLimitEnabled = &enabled
	}
}

// WithBetaGroupsApps filters beta groups by related app IDs.
func WithBetaGroupsApps(appIDs []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.appIDs = normalizeList(appIDs)
	}
}

// WithBetaGroupsBuilds filters beta groups by related build IDs.
func WithBetaGroupsBuilds(buildIDs []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.buildIDs = normalizeList(buildIDs)
	}
}

// WithBetaGroupsIDs filters beta groups by ID(s).
func WithBetaGroupsIDs(ids []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithBetaGroupsName filters beta groups by group name.
func WithBetaGroupsName(name string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.name = strings.TrimSpace(name)
	}
}

// WithBetaGroupsPublicLink filters beta groups by public-link value.
func WithBetaGroupsPublicLink(publicLink string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.publicLink = strings.TrimSpace(publicLink)
	}
}

// WithBetaGroupsSort sets the beta groups sort order.
func WithBetaGroupsSort(sort string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.sort = strings.TrimSpace(sort)
	}
}

// WithBetaGroupsFields selects a sparse beta group fieldset.
func WithBetaGroupsFields(fields []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBetaGroupsAppFields sets fields[apps] for included apps.
func WithBetaGroupsAppFields(fields []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithBetaGroupsBuildFields sets fields[builds] for included builds.
func WithBetaGroupsBuildFields(fields []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.buildFields = normalizeList(fields)
	}
}

// WithBetaGroupsBetaTesterFields sets fields[betaTesters] for included testers.
func WithBetaGroupsBetaTesterFields(fields []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.betaTesterFields = normalizeList(fields)
	}
}

// WithBetaGroupsBetaRecruitmentCriteriaFields sets fields[betaRecruitmentCriteria]
// for included beta recruitment criteria.
func WithBetaGroupsBetaRecruitmentCriteriaFields(fields []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.betaRecruitmentCriteriaFields = normalizeList(fields)
	}
}

// WithBetaGroupsInclude includes related resources in beta group responses.
func WithBetaGroupsInclude(include []string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.include = normalizeList(include)
	}
}

// WithBetaGroupsBetaTestersLimit sets limit[betaTesters] for included testers.
func WithBetaGroupsBetaTestersLimit(limit int) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if limit > 0 {
			q.betaTestersLimit = limit
		}
	}
}

// WithBetaGroupsBuildsLimit sets limit[builds] for included builds.
func WithBetaGroupsBuildsLimit(limit int) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if limit > 0 {
			q.buildsLimit = limit
		}
	}
}

// WithBetaTestersEmail filters beta testers by email.
func WithBetaTestersEmail(email string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.email = strings.TrimSpace(email)
	}
}

// WithBetaTestersFirstName filters beta testers by first name.
func WithBetaTestersFirstName(firstName string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.firstName = strings.TrimSpace(firstName)
	}
}

// WithBetaTestersLastName filters beta testers by last name.
func WithBetaTestersLastName(lastName string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.lastName = strings.TrimSpace(lastName)
	}
}

// WithBetaTestersIDs filters beta testers by tester ID(s).
func WithBetaTestersIDs(ids []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithBetaTestersGroupIDs filters beta testers by beta group ID(s).
func WithBetaTestersGroupIDs(ids []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.groupIDs = normalizeList(ids)
	}
}

// WithBetaTestersBuildID filters beta testers by build ID.
func WithBetaTestersBuildID(buildID string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.filterBuilds = strings.TrimSpace(buildID)
	}
}

// WithBetaTestersInviteTypes filters beta testers by invite type
// (EMAIL, PUBLIC_LINK). Unlike the relationship filters, filter[inviteType]
// combines freely with the app, group, and build filters.
func WithBetaTestersInviteTypes(inviteTypes []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.inviteTypes = normalizeList(inviteTypes)
	}
}

// WithBetaTestersSort sets the sort order for beta testers.
func WithBetaTestersSort(sort string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBetaTestersInclude specifies related resources to include in beta tester
// responses (apps, betaGroups, builds). Including betaGroups returns each
// tester's group memberships in one call instead of a per-tester lookup.
func WithBetaTestersInclude(include []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if normalized := normalizeList(include); len(normalized) > 0 {
			q.include = normalized
		}
	}
}

// WithBetaTestersFields sets fields[betaTesters].
func WithBetaTestersFields(fields []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBetaTesterUsagesPeriod sets the reporting period for beta tester usage metrics.
func WithBetaTesterUsagesPeriod(period string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(period) != "" {
			q.period = strings.TrimSpace(period)
		}
	}
}

// WithBetaTesterUsagesAppID filters beta tester usage metrics by app ID.
func WithBetaTesterUsagesAppID(appID string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(appID) != "" {
			q.appID = strings.TrimSpace(appID)
		}
	}
}

// WithBetaTesterUsagesGroupBy sets the groupBy dimension for beta tester usage metrics.
func WithBetaTesterUsagesGroupBy(groupBy string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(groupBy) != "" {
			q.groupBy = strings.TrimSpace(groupBy)
		}
	}
}

// WithBetaTesterUsagesFilterBetaTesters filters beta tester usage metrics by beta tester ID.
func WithBetaTesterUsagesFilterBetaTesters(testerID string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(testerID) != "" {
			q.filterBetaTesters = strings.TrimSpace(testerID)
		}
	}
}

// WithBetaAppReviewSubmissionsBuildIDs filters submissions by build ID(s).
func WithBetaAppReviewSubmissionsBuildIDs(ids []string) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithBetaAppReviewSubmissionsIncludeBuild includes each submission's related build.
func WithBetaAppReviewSubmissionsIncludeBuild() BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		q.includeBuild = true
	}
}

// WithBetaBuildLocalizationLocales filters beta build localizations by locale.
func WithBetaBuildLocalizationLocales(locales []string) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithBetaBuildLocalizationBuildIDs filters beta build localizations by build ID(s).
func WithBetaBuildLocalizationBuildIDs(ids []string) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// addBetaSubmissionInclude sets the `include` parameter for beta feedback crash
// and screenshot submission queries. When the `build` relationship is requested
// it also narrows `fields[builds]` to the build version (CFBundleVersion, i.e.
// the build number) and its preReleaseVersion relationship, so callers can
// resolve the marketing version (CFBundleShortVersionString) without parsing the
// crash log. The submission endpoints only support including `build` and
// `tester`.
func addBetaSubmissionInclude(values url.Values, include []string) {
	include = normalizeList(include)
	addCSV(values, "include", include)
	if slices.Contains(include, "build") {
		addCSV(values, "fields[builds]", []string{"version", "preReleaseVersion"})
	}
}
