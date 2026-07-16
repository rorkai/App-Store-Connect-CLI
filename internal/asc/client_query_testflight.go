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

type betaAppLocalizationsQuery struct {
	listQuery
	locales []string
	appIDs  []string
}

type betaBuildLocalizationsQuery struct {
	listQuery
	locales  []string
	buildIDs []string
}

type betaBuildUsagesQuery struct {
	listQuery
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
	isInternalGroup *bool
}

type betaGroupBuildsQuery struct {
	listQuery
}

type betaGroupTestersQuery struct {
	listQuery
}

type betaTestersQuery struct {
	listQuery
	email        string
	groupIDs     []string
	filterBuilds string
}

type betaAppReviewDetailsQuery struct {
	listQuery
}

type betaAppReviewSubmissionsQuery struct {
	listQuery
	buildIDs []string
}

type buildBetaDetailsQuery struct {
	listQuery
	buildIDs []string
}

type betaRecruitmentCriterionOptionsQuery struct {
	listQuery
	fields []string
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
			"appPlatform",
			"devicePlatform",
			"screenshots",
		}
		// A sparse fieldset omits any field not listed, including relationship
		// fields. If the caller also requested relationships via include, those
		// relationship names must be present here or ASC will not return the
		// linked resources. `build`/`tester` are valid screenshot submission
		// fields, so append whatever was included.
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
	return values.Encode()
}

func buildBetaGroupsQuery(query *betaGroupsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.isInternalGroup != nil {
		values.Set("filter[isInternalGroup]", strconv.FormatBool(*query.isInternalGroup))
	}
	return values.Encode()
}

func buildBetaGroupBuildsQuery(query *betaGroupBuildsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaGroupTestersQuery(query *betaGroupTestersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
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
	addLimit(values, query.limit)
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildBetaDetailsQuery(query *buildBetaDetailsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[build]", query.buildIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaRecruitmentCriterionOptionsQuery(query *betaRecruitmentCriterionOptionsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[betaRecruitmentCriterionOptions]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppLocalizationsQuery(query *betaAppLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addCSV(values, "filter[app]", query.appIDs)
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

func buildBetaBuildUsagesQuery(query *betaBuildUsagesQuery) string {
	values := url.Values{}
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

// BetaGroupBuildsOption is a functional option for GetBetaGroupBuilds.
type BetaGroupBuildsOption func(*betaGroupBuildsQuery)

// BetaGroupTestersOption is a functional option for GetBetaGroupTesters.
type BetaGroupTestersOption func(*betaGroupTestersQuery)

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

// BuildBetaDetailsOption is a functional option for build beta details.
type BuildBetaDetailsOption func(*buildBetaDetailsQuery)

// BetaRecruitmentCriterionOptionsOption is a functional option for recruitment options.
type BetaRecruitmentCriterionOptionsOption func(*betaRecruitmentCriterionOptionsQuery)

// BetaAppLocalizationsOption is a functional option for beta app localizations.
type BetaAppLocalizationsOption func(*betaAppLocalizationsQuery)

// AppBetaAppLocalizationsOption is a functional option for GetAppBetaAppLocalizations.
type AppBetaAppLocalizationsOption func(*listQuery)

// BetaBuildLocalizationsOption is a functional option for beta build localizations.
type BetaBuildLocalizationsOption func(*betaBuildLocalizationsQuery)

// BetaBuildUsagesOption is a functional option for beta build usage metrics.
type BetaBuildUsagesOption func(*betaBuildUsagesQuery)

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

// WithFeedbackLimit sets the max number of feedback items to return.
func WithFeedbackLimit(limit int) FeedbackOption {
	return func(q *feedbackQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithFeedbackNextURL uses a next page URL directly.
func WithFeedbackNextURL(next string) FeedbackOption {
	return func(q *feedbackQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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

// WithCrashLimit sets the max number of crash items to return.
func WithCrashLimit(limit int) CrashOption {
	return func(q *crashQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCrashNextURL uses a next page URL directly.
func WithCrashNextURL(next string) CrashOption {
	return func(q *crashQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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

// WithBetaGroupsLimit sets the max number of beta groups to return.
func WithBetaGroupsLimit(limit int) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupsNextURL uses a next page URL directly.
func WithBetaGroupsNextURL(next string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaGroupsIsInternal filters beta groups by internal/external groups.
func WithBetaGroupsIsInternal(isInternal bool) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		q.isInternalGroup = &isInternal
	}
}

// WithBetaGroupBuildsLimit sets the max number of builds to return for a group.
func WithBetaGroupBuildsLimit(limit int) BetaGroupBuildsOption {
	return func(q *betaGroupBuildsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupBuildsNextURL uses a next page URL directly.
func WithBetaGroupBuildsNextURL(next string) BetaGroupBuildsOption {
	return func(q *betaGroupBuildsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaGroupTestersLimit sets the max number of testers to return for a group.
func WithBetaGroupTestersLimit(limit int) BetaGroupTestersOption {
	return func(q *betaGroupTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupTestersNextURL uses a next page URL directly.
func WithBetaGroupTestersNextURL(next string) BetaGroupTestersOption {
	return func(q *betaGroupTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTestersLimit sets the max number of beta testers to return.
func WithBetaTestersLimit(limit int) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTestersNextURL uses a next page URL directly.
func WithBetaTestersNextURL(next string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTestersEmail filters beta testers by email.
func WithBetaTestersEmail(email string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.email = strings.TrimSpace(email)
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

// WithBetaTesterUsagesLimit sets the max number of beta tester usage records to return.
func WithBetaTesterUsagesLimit(limit int) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTesterUsagesNextURL uses a next page URL directly.
func WithBetaTesterUsagesNextURL(next string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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

// WithBetaAppReviewDetailsLimit sets the max number of review detail records to return.
func WithBetaAppReviewDetailsLimit(limit int) BetaAppReviewDetailsOption {
	return func(q *betaAppReviewDetailsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppReviewDetailsNextURL uses a next page URL directly.
func WithBetaAppReviewDetailsNextURL(next string) BetaAppReviewDetailsOption {
	return func(q *betaAppReviewDetailsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppReviewSubmissionsLimit sets the max number of submissions to return.
func WithBetaAppReviewSubmissionsLimit(limit int) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppReviewSubmissionsNextURL uses a next page URL directly.
func WithBetaAppReviewSubmissionsNextURL(next string) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppReviewSubmissionsBuildIDs filters submissions by build ID(s).
func WithBetaAppReviewSubmissionsBuildIDs(ids []string) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithBuildBetaDetailsLimit sets the max number of build beta details to return.
func WithBuildBetaDetailsLimit(limit int) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBetaDetailsNextURL uses a next page URL directly.
func WithBuildBetaDetailsNextURL(next string) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildBetaDetailsBuildIDs filters build beta details by build ID(s).
func WithBuildBetaDetailsBuildIDs(ids []string) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithBetaRecruitmentCriterionOptionsLimit sets the max number of criterion options to return.
func WithBetaRecruitmentCriterionOptionsLimit(limit int) BetaRecruitmentCriterionOptionsOption {
	return func(q *betaRecruitmentCriterionOptionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaRecruitmentCriterionOptionsFields sets fields for criterion options.
func WithBetaRecruitmentCriterionOptionsFields(fields []string) BetaRecruitmentCriterionOptionsOption {
	return func(q *betaRecruitmentCriterionOptionsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBetaRecruitmentCriterionOptionsNextURL uses a next page URL directly.
func WithBetaRecruitmentCriterionOptionsNextURL(next string) BetaRecruitmentCriterionOptionsOption {
	return func(q *betaRecruitmentCriterionOptionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppLocalizationsLimit sets the max number of beta app localizations to return.
func WithBetaAppLocalizationsLimit(limit int) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppLocalizationsNextURL uses a next page URL directly.
func WithBetaAppLocalizationsNextURL(next string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppLocalizationLocales filters beta app localizations by locale.
func WithBetaAppLocalizationLocales(locales []string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithBetaAppLocalizationAppIDs filters beta app localizations by app ID(s).
func WithBetaAppLocalizationAppIDs(ids []string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		q.appIDs = normalizeList(ids)
	}
}

// WithBetaBuildLocalizationsLimit sets the max number of beta build localizations to return.
func WithBetaBuildLocalizationsLimit(limit int) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaBuildLocalizationsNextURL uses a next page URL directly.
func WithBetaBuildLocalizationsNextURL(next string) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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

// WithBetaBuildUsagesLimit sets the max number of beta build usage records to return.
func WithBetaBuildUsagesLimit(limit int) BetaBuildUsagesOption {
	return func(q *betaBuildUsagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaBuildUsagesNextURL uses a next page URL directly.
func WithBetaBuildUsagesNextURL(next string) BetaBuildUsagesOption {
	return func(q *betaBuildUsagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppBetaAppLocalizationsLimit sets the max number of beta app localizations to return.
func WithAppBetaAppLocalizationsLimit(limit int) AppBetaAppLocalizationsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppBetaAppLocalizationsNextURL uses a next page URL directly.
func WithAppBetaAppLocalizationsNextURL(next string) AppBetaAppLocalizationsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTesterAppsLimit sets the max number of apps to return.
func WithBetaTesterAppsLimit(limit int) BetaTesterAppsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTesterAppsNextURL uses a next page URL directly.
func WithBetaTesterAppsNextURL(next string) BetaTesterAppsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTesterBetaGroupsLimit sets the max number of beta groups to return.
func WithBetaTesterBetaGroupsLimit(limit int) BetaTesterBetaGroupsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTesterBetaGroupsNextURL uses a next page URL directly.
func WithBetaTesterBetaGroupsNextURL(next string) BetaTesterBetaGroupsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTesterBuildsLimit sets the max number of builds to return.
func WithBetaTesterBuildsLimit(limit int) BetaTesterBuildsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTesterBuildsNextURL uses a next page URL directly.
func WithBetaTesterBuildsNextURL(next string) BetaTesterBuildsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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
