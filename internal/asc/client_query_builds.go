package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type buildsQuery struct {
	listQuery
	includedFieldsQuery
	sort                 string
	version              string
	processingStates     []string
	buildAudienceType    BuildAudienceType
	preReleasePlatforms  []string
	preReleaseVersionIDs []string
	betaReviewStates     []string
	expired              *bool
	include              []string
}

type buildUploadQuery struct {
	include []string
}

type buildQuery struct {
	includedFieldsQuery
	include []string
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

func buildBuildBundlesQuery(query *buildBundlesQuery) string {
	values := url.Values{}
	values.Set("include", "buildBundles")
	if query.limit > 0 {
		values.Set("limit[buildBundles]", strconv.Itoa(query.limit))
	}
	return values.Encode()
}

func buildBuildUploadQuery(query *buildUploadQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildBuildUploadsQuery(query *buildUploadsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[cfBundleShortVersionString]", query.cfBundleShortVersions)
	addCSV(values, "filter[cfBundleVersion]", query.cfBundleVersions)
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

// BuildsOption is a functional option for GetBuilds.
type BuildsOption func(*buildsQuery)

// BuildBundlesOption is a functional option for GetBuildBundlesForBuild.
type BuildBundlesOption func(*buildBundlesQuery)

// BuildOption is a functional option for GetBuild.
type BuildOption func(*buildQuery)

// BuildUploadOption is a functional option for GetBuildUpload.
type BuildUploadOption func(*buildUploadQuery)

// BuildUploadsOption is a functional option for GetBuildUploads.
type BuildUploadsOption func(*buildUploadsQuery)

// BuildIconsOption is a functional option for GetBuildIcons.
type BuildIconsOption func(*listQuery)

// AppPreReleaseVersionsOption is a functional option for GetAppPreReleaseVersions.
type AppPreReleaseVersionsOption func(*listQuery)

// PreReleaseVersionBuildsOption is a functional option for GetPreReleaseVersionBuilds.
type PreReleaseVersionBuildsOption func(*listQuery)

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

// WithBuildsBuildNumber filters builds by build number.
// App Store Connect models build number as build version, so this maps to filter[version].
func WithBuildsBuildNumber(buildNumber string) BuildsOption {
	return WithBuildsVersion(buildNumber)
}

// WithBuildsAudienceType filters builds by their distribution audience.
func WithBuildsAudienceType(audience BuildAudienceType) BuildsOption {
	return func(q *buildsQuery) { q.buildAudienceType = audience }
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

// WithBuildsBetaReviewStates filters builds by their beta app review submission state.
func WithBuildsBetaReviewStates(states []string) BuildsOption {
	return func(q *buildsQuery) {
		normalized := normalizeUpperList(states)
		if len(normalized) > 0 {
			q.betaReviewStates = normalized
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
		q.expired = &expired
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

// WithBuildInclude requests related resources for a single build.
func WithBuildInclude(include []string) BuildOption {
	return func(q *buildQuery) {
		q.include = include
	}
}

// WithBuildUploadInclude requests related resources for a single build upload.
// App Store Connect returns the build relationship linkage only when "build"
// is included.
func WithBuildUploadInclude(include []string) BuildUploadOption {
	return func(q *buildUploadQuery) {
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
