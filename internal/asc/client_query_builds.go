package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type buildsQuery struct {
	listQuery
	sort                 string
	version              string
	preReleaseVersion    string
	processingStates     []string
	preReleasePlatforms  []string
	preReleaseVersionIDs []string
	expired              *bool
	include              []string
}

type buildUploadsQuery struct {
	listQuery
	cfBundleShortVersions []string
	cfBundleVersions      []string
	platforms             []string
	states                []string
	sort                  string
}

type buildBundlesQuery struct {
	limit int
}

type buildBundleFileSizesQuery struct {
	listQuery
}

type buildUploadFilesQuery struct {
	listQuery
}

type buildIndividualTestersQuery struct {
	listQuery
}

type preReleaseVersionsQuery struct {
	listQuery
	platform string
	version  string
}

func buildBuildBundlesQuery(query *buildBundlesQuery) string {
	values := url.Values{}
	values.Set("include", "buildBundles")
	if query.limit > 0 {
		values.Set("limit[buildBundles]", strconv.Itoa(query.limit))
	}
	return values.Encode()
}

func buildBuildBundleFileSizesQuery(query *buildBundleFileSizesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildUploadsQuery(query *buildUploadsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[cfBundleShortVersionString]", query.cfBundleShortVersions)
	addCSV(values, "filter[cfBundleVersion]", query.cfBundleVersions)
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildUploadFilesQuery(query *buildUploadFilesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildIndividualTestersQuery(query *buildIndividualTestersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPreReleaseVersionsQuery(appID string, query *preReleaseVersionsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(appID))
	}
	if strings.TrimSpace(query.platform) != "" {
		values.Set("filter[platform]", strings.TrimSpace(query.platform))
	}
	if strings.TrimSpace(query.version) != "" {
		values.Set("filter[version]", strings.TrimSpace(query.version))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

// BuildsOption is a functional option for GetBuilds.
type BuildsOption func(*buildsQuery)

// BuildBundlesOption is a functional option for GetBuildBundlesForBuild.
type BuildBundlesOption func(*buildBundlesQuery)

// BuildBundleFileSizesOption is a functional option for GetBuildBundleFileSizes.
type BuildBundleFileSizesOption func(*buildBundleFileSizesQuery)

// BuildUploadsOption is a functional option for GetBuildUploads.
type BuildUploadsOption func(*buildUploadsQuery)

// BuildUploadFilesOption is a functional option for GetBuildUploadFiles.
type BuildUploadFilesOption func(*buildUploadFilesQuery)

// BuildIndividualTestersOption is a functional option for GetBuildIndividualTesters.
type BuildIndividualTestersOption func(*buildIndividualTestersQuery)

// BuildIconsOption is a functional option for GetBuildIcons.
type BuildIconsOption func(*listQuery)

// PreReleaseVersionsOption is a functional option for GetPreReleaseVersions.
type PreReleaseVersionsOption func(*preReleaseVersionsQuery)

// AppPreReleaseVersionsOption is a functional option for GetAppPreReleaseVersions.
type AppPreReleaseVersionsOption func(*listQuery)

// PreReleaseVersionBuildsOption is a functional option for GetPreReleaseVersionBuilds.
type PreReleaseVersionBuildsOption func(*listQuery)

// WithBuildsLimit sets the max number of builds to return.
func WithBuildsLimit(limit int) BuildsOption {
	return func(q *buildsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildsNextURL uses a next page URL directly.
func WithBuildsNextURL(next string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildsSort sets the sort order for builds.
func WithBuildsSort(sort string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBuildsVersion filters builds by build number (CFBundleVersion) via
// filter[version].
func WithBuildsVersion(version string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(version) != "" {
			q.version = strings.TrimSpace(version)
		}
	}
}

// WithBuildsPreReleaseVersionVersion filters builds by marketing version
// (CFBundleShortVersionString) via filter[preReleaseVersion.version].
func WithBuildsPreReleaseVersionVersion(version string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(version) != "" {
			q.preReleaseVersion = strings.TrimSpace(version)
		}
	}
}

// WithBuildsBuildNumber filters builds by build number.
// App Store Connect models build number as build version, so this maps to filter[version].
func WithBuildsBuildNumber(buildNumber string) BuildsOption {
	return WithBuildsVersion(buildNumber)
}

// WithBuildsProcessingStates filters builds by processing state.
func WithBuildsProcessingStates(states []string) BuildsOption {
	return func(q *buildsQuery) {
		normalized := normalizeUpperList(states)
		if len(normalized) > 0 {
			q.processingStates = normalized
		}
	}
}

// WithBuildsPreReleaseVersionPlatforms filters builds by the related pre-release platform.
func WithBuildsPreReleaseVersionPlatforms(platforms []string) BuildsOption {
	return func(q *buildsQuery) {
		normalized := normalizeUpperList(platforms)
		if len(normalized) > 0 {
			q.preReleasePlatforms = normalized
		}
	}
}

// WithBuildsPreReleaseVersion filters builds by a single pre-release version ID.
func WithBuildsPreReleaseVersion(preReleaseVersionID string) BuildsOption {
	return WithBuildsPreReleaseVersions([]string{preReleaseVersionID})
}

// WithBuildsPreReleaseVersions filters builds by one or more pre-release version IDs.
func WithBuildsPreReleaseVersions(preReleaseVersionIDs []string) BuildsOption {
	return func(q *buildsQuery) {
		normalized := normalizeList(preReleaseVersionIDs)
		if len(normalized) > 0 {
			q.preReleaseVersionIDs = normalized
		}
	}
}

// WithBuildsExpired filters builds by expired state.
func WithBuildsExpired(expired bool) BuildsOption {
	return func(q *buildsQuery) {
		value := expired
		q.expired = &value
	}
}

// WithBuildsInclude specifies related resources to include in the response
// (e.g., "preReleaseVersion" to get the marketing version string).
func WithBuildsInclude(include []string) BuildsOption {
	return func(q *buildsQuery) {
		normalized := normalizeList(include)
		if len(normalized) > 0 {
			q.include = normalized
		}
	}
}

// WithBuildBundlesLimit sets the max number of included build bundles to return.
func WithBuildBundlesLimit(limit int) BuildBundlesOption {
	return func(q *buildBundlesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBundleFileSizesLimit sets the max number of file size items to return.
func WithBuildBundleFileSizesLimit(limit int) BuildBundleFileSizesOption {
	return func(q *buildBundleFileSizesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBundleFileSizesNextURL uses a next page URL directly.
func WithBuildBundleFileSizesNextURL(next string) BuildBundleFileSizesOption {
	return func(q *buildBundleFileSizesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildUploadsLimit sets the max number of build uploads to return.
func WithBuildUploadsLimit(limit int) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildUploadsNextURL uses a next page URL directly.
func WithBuildUploadsNextURL(next string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildUploadsCFBundleShortVersionStrings filters build uploads by CFBundleShortVersionString.
func WithBuildUploadsCFBundleShortVersionStrings(values []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.cfBundleShortVersions = normalizeList(values)
	}
}

// WithBuildUploadsCFBundleVersions filters build uploads by CFBundleVersion.
func WithBuildUploadsCFBundleVersions(values []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.cfBundleVersions = normalizeList(values)
	}
}

// WithBuildUploadsPlatforms filters build uploads by platform(s).
func WithBuildUploadsPlatforms(platforms []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithBuildUploadsStates filters build uploads by upload state(s).
func WithBuildUploadsStates(states []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithBuildUploadsSort sets the sort order for build uploads.
func WithBuildUploadsSort(sort string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBuildUploadFilesLimit sets the max number of build upload files to return.
func WithBuildUploadFilesLimit(limit int) BuildUploadFilesOption {
	return func(q *buildUploadFilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildUploadFilesNextURL uses a next page URL directly.
func WithBuildUploadFilesNextURL(next string) BuildUploadFilesOption {
	return func(q *buildUploadFilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildIndividualTestersLimit sets the max number of build individual testers to return.
func WithBuildIndividualTestersLimit(limit int) BuildIndividualTestersOption {
	return func(q *buildIndividualTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildIndividualTestersNextURL uses a next page URL directly.
func WithBuildIndividualTestersNextURL(next string) BuildIndividualTestersOption {
	return func(q *buildIndividualTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPreReleaseVersionsPlatform filters pre-release versions by platform.
func WithPreReleaseVersionsPlatform(platform string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		normalized := normalizeUpperCSVString(platform)
		if normalized != "" {
			q.platform = normalized
		}
	}
}

// WithPreReleaseVersionsVersion filters pre-release versions by version string.
func WithPreReleaseVersionsVersion(version string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		normalized := normalizeCSVString(version)
		if normalized != "" {
			q.version = normalized
		}
	}
}

// WithPreReleaseVersionsLimit sets the max number of pre-release versions to return.
func WithPreReleaseVersionsLimit(limit int) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPreReleaseVersionsNextURL uses a next page URL directly.
func WithPreReleaseVersionsNextURL(next string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildIconsLimit sets the max number of build icons to return.
func WithBuildIconsLimit(limit int) BuildIconsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildIconsNextURL uses a next page URL directly.
func WithBuildIconsNextURL(next string) BuildIconsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppPreReleaseVersionsLimit sets the max number of pre-release versions to return.
func WithAppPreReleaseVersionsLimit(limit int) AppPreReleaseVersionsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppPreReleaseVersionsNextURL uses a next page URL directly.
func WithAppPreReleaseVersionsNextURL(next string) AppPreReleaseVersionsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPreReleaseVersionBuildsLimit sets the max number of builds to return.
func WithPreReleaseVersionBuildsLimit(limit int) PreReleaseVersionBuildsOption {
	return func(q *listQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPreReleaseVersionBuildsNextURL uses a next page URL directly.
func WithPreReleaseVersionBuildsNextURL(next string) PreReleaseVersionBuildsOption {
	return func(q *listQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}
