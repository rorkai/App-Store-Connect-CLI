package asc

import (
	"context"
	"encoding/json"
	"fmt"
)

// BuildBundleType represents the type of build bundle.
type BuildBundleType string

const (
	BuildBundleTypeApp     BuildBundleType = "APP"
	BuildBundleTypeAppClip BuildBundleType = "APP_CLIP"
)

// BuildBundleAttributes describes a build bundle resource.
type BuildBundleAttributes struct {
	BundleID                        *string                      `json:"bundleId,omitempty"`
	BundleType                      *BuildBundleType             `json:"bundleType,omitempty"`
	SDKBuild                        *string                      `json:"sdkBuild,omitempty"`
	PlatformBuild                   *string                      `json:"platformBuild,omitempty"`
	FileName                        *string                      `json:"fileName,omitempty"`
	HasSiriKit                      *bool                        `json:"hasSirikit,omitempty"`
	HasOnDemandResources            *bool                        `json:"hasOnDemandResources,omitempty"`
	HasPrerenderedIcon              *bool                        `json:"hasPrerenderedIcon,omitempty"`
	UsesLocationServices            *bool                        `json:"usesLocationServices,omitempty"`
	IsIOSBuildMacAppStoreCompatible *bool                        `json:"isIosBuildMacAppStoreCompatible,omitempty"`
	IncludesSymbols                 *bool                        `json:"includesSymbols,omitempty"`
	DSYMURL                         *string                      `json:"dSYMUrl,omitempty"`
	SupportedArchitectures          []string                     `json:"supportedArchitectures,omitempty"`
	RequiredCapabilities            []string                     `json:"requiredCapabilities,omitempty"`
	DeviceProtocols                 []string                     `json:"deviceProtocols,omitempty"`
	Locales                         []string                     `json:"locales,omitempty"`
	Entitlements                    map[string]map[string]string `json:"entitlements,omitempty"`
	BADownloadAllowance             *int64                       `json:"baDownloadAllowance,omitempty"`
	BAMaxInstallSize                *int64                       `json:"baMaxInstallSize,omitempty"`
	MinimumOSVersion                *string                      `json:"minimumOsVersion,omitempty"`
}

// BuildBundleFileSizeAttributes describes a build bundle file size resource.
type BuildBundleFileSizeAttributes struct {
	DeviceModel   *string `json:"deviceModel,omitempty"`
	OSVersion     *string `json:"osVersion,omitempty"`
	DownloadBytes *int64  `json:"downloadBytes,omitempty"`
	InstallBytes  *int64  `json:"installBytes,omitempty"`
}

// AppClipDomainStatusDomain describes a single App Clip domain entry.
type AppClipDomainStatusDomain struct {
	Domain          *string `json:"domain,omitempty"`
	IsValid         *bool   `json:"isValid,omitempty"`
	LastUpdatedDate *string `json:"lastUpdatedDate,omitempty"`
	ErrorCode       *string `json:"errorCode,omitempty"`
}

// AppClipDomainStatusAttributes describes App Clip domain status details.
type AppClipDomainStatusAttributes struct {
	Domains         []AppClipDomainStatusDomain `json:"domains,omitempty"`
	LastUpdatedDate *string                     `json:"lastUpdatedDate,omitempty"`
}

// BetaAppClipInvocationAttributes describes a beta app clip invocation resource.
type BetaAppClipInvocationAttributes struct {
	URL *string `json:"url,omitempty"`
}

// BuildBundlesResponse is the response from build bundle include list.
type BuildBundlesResponse = Response[BuildBundleAttributes]

// BuildBundleFileSizesResponse is the response from build bundle file sizes endpoint.
type BuildBundleFileSizesResponse = Response[BuildBundleFileSizeAttributes]

// AppClipDomainStatusResponse is the response for app clip domain status endpoints.
type AppClipDomainStatusResponse = SingleResponse[AppClipDomainStatusAttributes]

// BetaAppClipInvocationsResponse is the response from beta app clip invocations endpoint.
type BetaAppClipInvocationsResponse = Response[BetaAppClipInvocationAttributes]

// AppClipDomainStatusResult represents CLI output for App Clip domain status.
type AppClipDomainStatusResult struct {
	BuildBundleID   string                      `json:"buildBundleId"`
	Available       bool                        `json:"available"`
	StatusID        string                      `json:"statusId,omitempty"`
	LastUpdatedDate *string                     `json:"lastUpdatedDate,omitempty"`
	Domains         []AppClipDomainStatusDomain `json:"domains,omitempty"`
}

// NewAppClipDomainStatusResult builds a CLI-friendly App Clip domain status result.
func NewAppClipDomainStatusResult(buildBundleID string, resp *AppClipDomainStatusResponse) *AppClipDomainStatusResult {
	result := &AppClipDomainStatusResult{
		BuildBundleID: buildBundleID,
	}
	if resp == nil {
		return result
	}
	result.Available = true
	result.StatusID = resp.Data.ID
	result.LastUpdatedDate = resp.Data.Attributes.LastUpdatedDate
	if len(resp.Data.Attributes.Domains) > 0 {
		result.Domains = resp.Data.Attributes.Domains
	}
	return result
}

// GetBuildBundlesForBuild retrieves build bundles for a build via include.
func (c *Client) GetBuildBundlesForBuild(ctx context.Context, buildID string, opts ...BuildBundlesOption) (*BuildBundlesResponse, error) {
	query := applyOptions(opts)
	resp, err := getJSON[BuildResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/builds/%s", buildID), Query: buildBuildBundlesQuery(query)})
	if err != nil {
		return nil, err
	}

	bundles, err := extractBuildBundles(resp.Included)
	if err != nil {
		return nil, err
	}

	return &BuildBundlesResponse{Data: bundles}, nil
}

// GetBuildBundleFileSizes retrieves build bundle file sizes by build bundle ID.
func (c *Client) GetBuildBundleFileSizes(ctx context.Context, buildBundleID string, opts ...BuildBundleFileSizesOption) (*BuildBundleFileSizesResponse, error) {
	query := applyOptions(opts)
	return getJSON[BuildBundleFileSizesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/buildBundles/%s/buildBundleFileSizes", buildBundleID),
		Query:       buildBuildBundleFileSizesQuery(query),
		Next:        query.nextURL,
		NextContext: "build bundle file sizes",
	})
}

// GetBuildBundleAppClipDomainCacheStatus retrieves App Clip domain cache status for a build bundle.
func (c *Client) GetBuildBundleAppClipDomainCacheStatus(ctx context.Context, buildBundleID string) (*AppClipDomainStatusResponse, error) {
	return getJSON[AppClipDomainStatusResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/buildBundles/%s/appClipDomainCacheStatus", buildBundleID)})
}

// GetBuildBundleAppClipDomainDebugStatus retrieves App Clip domain debug status for a build bundle.
func (c *Client) GetBuildBundleAppClipDomainDebugStatus(ctx context.Context, buildBundleID string) (*AppClipDomainStatusResponse, error) {
	return getJSON[AppClipDomainStatusResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/buildBundles/%s/appClipDomainDebugStatus", buildBundleID)})
}

// GetBuildBundleBetaAppClipInvocations retrieves beta app clip invocations for a build bundle.
func (c *Client) GetBuildBundleBetaAppClipInvocations(ctx context.Context, buildBundleID string, opts ...BetaAppClipInvocationsOption) (*BetaAppClipInvocationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BetaAppClipInvocationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/buildBundles/%s/betaAppClipInvocations", buildBundleID),
		Query:       buildBetaAppClipInvocationsQuery(query),
		Next:        query.nextURL,
		NextContext: "build bundle app clip invocations",
	})
}

func extractBuildBundles(included json.RawMessage) ([]Resource[BuildBundleAttributes], error) {
	if len(included) == 0 {
		return []Resource[BuildBundleAttributes]{}, nil
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(included, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to parse included: %w", err)
	}

	bundles := make([]Resource[BuildBundleAttributes], 0, len(rawItems))
	for _, raw := range rawItems {
		var probe struct {
			Type ResourceType `json:"type"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, fmt.Errorf("failed to parse included type: %w", err)
		}
		if probe.Type != ResourceTypeBuildBundles {
			continue
		}

		var bundle Resource[BuildBundleAttributes]
		if err := json.Unmarshal(raw, &bundle); err != nil {
			return nil, fmt.Errorf("failed to parse build bundle: %w", err)
		}
		bundles = append(bundles, bundle)
	}

	return bundles, nil
}
