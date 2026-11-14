package asc

import (
	"context"
	"fmt"
	"net/url"
)

// Asset collection options are intentionally separate from relationship options:
// these endpoints return full resources and accept their own links.next URLs.
type appScreenshotSetsQuery struct {
	listQuery
	limitSet       bool
	includeMedia   bool
	requestContext RequestContextFunc
}

type appScreenshotsQuery struct {
	listQuery
	limitSet       bool
	requestContext RequestContextFunc
}

const appScreenshotCollectionLimitMax = 200

// AppScreenshotSetsOption configures screenshot-set collection requests.
type AppScreenshotSetsOption func(*appScreenshotSetsQuery)

// AppScreenshotsOption configures screenshot collection requests.
type AppScreenshotsOption func(*appScreenshotsQuery)

func buildAppScreenshotSetsQuery(query *appScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.includeMedia {
		addIncludedSetMedia(values, "appScreenshots")
	}
	return values.Encode()
}

func buildAppScreenshotsQuery(query *appScreenshotsQuery) string {
	return buildListQuery(&query.listQuery)
}

// WithAppScreenshotSetsLimit sets the maximum number of screenshot sets to return.
func WithAppScreenshotSetsLimit(limit int) AppScreenshotSetsOption {
	return func(query *appScreenshotSetsQuery) {
		query.limit = limit
		query.limitSet = true
	}
}

// WithAppScreenshotSetsIncludeScreenshots includes each set's screenshots.
func WithAppScreenshotSetsIncludeScreenshots() AppScreenshotSetsOption {
	return func(query *appScreenshotSetsQuery) {
		query.includeMedia = true
	}
}

// WithAppScreenshotSetsRequestContext creates a fresh context for each page
// fetched by GetAllAppScreenshotSets.
func WithAppScreenshotSetsRequestContext(factory RequestContextFunc) AppScreenshotSetsOption {
	return func(query *appScreenshotSetsQuery) {
		query.requestContext = factory
	}
}

// WithAppScreenshotsLimit sets the maximum number of screenshots to return.
func WithAppScreenshotsLimit(limit int) AppScreenshotsOption {
	return func(query *appScreenshotsQuery) {
		query.limit = limit
		query.limitSet = true
	}
}

// WithAppScreenshotsRequestContext creates a fresh context for each page
// fetched by GetAllAppScreenshots.
func WithAppScreenshotsRequestContext(factory RequestContextFunc) AppScreenshotsOption {
	return func(query *appScreenshotsQuery) {
		query.requestContext = factory
	}
}

// WithAppStoreVersionLocalizationScreenshotSetsRequestContext creates a fresh
// context for each page fetched by
// GetAllAppStoreVersionLocalizationScreenshotSets.
func WithAppStoreVersionLocalizationScreenshotSetsRequestContext(factory RequestContextFunc) AppStoreVersionLocalizationScreenshotSetsOption {
	return func(query *appStoreVersionLocalizationScreenshotSetsQuery) {
		query.requestContext = factory
	}
}

// WithAppCustomProductPageLocalizationScreenshotSetsRequestContext creates a
// fresh context for each page fetched by
// GetAllAppCustomProductPageLocalizationScreenshotSets.
func WithAppCustomProductPageLocalizationScreenshotSetsRequestContext(factory RequestContextFunc) AppCustomProductPageLocalizationScreenshotSetsOption {
	return func(query *appCustomProductPageLocalizationScreenshotSetsQuery) {
		query.requestContext = factory
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRequestContext creates a fresh
// context for each page fetched by
// GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets.
func WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRequestContext(factory RequestContextFunc) AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption {
	return func(query *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) {
		query.requestContext = factory
	}
}

// GetAllAppScreenshotSets retrieves every screenshot set using automatic pagination.
func (c *Client) GetAllAppScreenshotSets(ctx context.Context, localizationID string, opts ...AppScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	requestCtx, cancel := requestContextFor(ctx, query.requestContext)
	firstPage, err := c.GetAppScreenshotSets(requestCtx, localizationID, opts...)
	cancel()
	if err != nil {
		return nil, err
	}

	result, err := paginateAssetResponse(ctx, firstPage, func(parentCtx context.Context, nextURL string) (PaginatedResponse, error) {
		nextCtx, nextCancel := requestContextFor(parentCtx, query.requestContext)
		defer nextCancel()
		return c.GetAppScreenshotSets(nextCtx, "", NextURL[AppScreenshotSetsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllAppScreenshots retrieves every screenshot for a set using automatic pagination.
func (c *Client) GetAllAppScreenshots(ctx context.Context, setID string, opts ...AppScreenshotsOption) (*AppScreenshotsResponse, error) {
	query := &appScreenshotsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	requestCtx, cancel := requestContextFor(ctx, query.requestContext)
	firstPage, err := c.GetAppScreenshots(requestCtx, setID, opts...)
	cancel()
	if err != nil {
		return nil, err
	}

	result, err := paginateAssetResponse(ctx, firstPage, func(parentCtx context.Context, nextURL string) (PaginatedResponse, error) {
		nextCtx, nextCancel := requestContextFor(parentCtx, query.requestContext)
		defer nextCancel()
		return c.GetAppScreenshots(nextCtx, "", NextURL[AppScreenshotsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// paginateAssetResponse keeps collection wrappers typed while sharing the
// standard repeated-next protection and envelope aggregation.
func paginateAssetResponse[T any](ctx context.Context, firstPage *Response[T], fetchNext PaginateFunc) (*Response[T], error) {
	result, err := PaginateAll(ctx, firstPage, fetchNext)
	if err != nil {
		return nil, err
	}
	response, ok := result.(*Response[T])
	if !ok {
		return nil, fmt.Errorf("unexpected paginated response type %T", result)
	}
	return response, nil
}

// GetAllAppStoreVersionLocalizationScreenshotSets retrieves every screenshot
// set for an App Store version localization.
func (c *Client) GetAllAppStoreVersionLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appStoreVersionLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	requestCtx, cancel := requestContextFor(ctx, query.requestContext)
	firstPage, err := c.GetAppStoreVersionLocalizationScreenshotSets(requestCtx, localizationID, opts...)
	cancel()
	if err != nil {
		return nil, err
	}

	result, err := paginateAssetResponse(ctx, firstPage, func(parentCtx context.Context, nextURL string) (PaginatedResponse, error) {
		nextCtx, nextCancel := requestContextFor(parentCtx, query.requestContext)
		defer nextCancel()
		return c.GetAppStoreVersionLocalizationScreenshotSets(nextCtx, "", WithAppStoreVersionLocalizationScreenshotSetsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllAppCustomProductPageLocalizationScreenshotSets retrieves every
// screenshot set for a custom product page localization.
func (c *Client) GetAllAppCustomProductPageLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppCustomProductPageLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appCustomProductPageLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	requestCtx, cancel := requestContextFor(ctx, query.requestContext)
	firstPage, err := c.GetAppCustomProductPageLocalizationScreenshotSets(requestCtx, localizationID, opts...)
	cancel()
	if err != nil {
		return nil, err
	}

	result, err := paginateAssetResponse(ctx, firstPage, func(parentCtx context.Context, nextURL string) (PaginatedResponse, error) {
		nextCtx, nextCancel := requestContextFor(parentCtx, query.requestContext)
		defer nextCancel()
		return c.GetAppCustomProductPageLocalizationScreenshotSets(nextCtx, "", WithAppCustomProductPageLocalizationScreenshotSetsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets retrieves
// every screenshot set for an experiment treatment localization.
func (c *Client) GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	requestCtx, cancel := requestContextFor(ctx, query.requestContext)
	firstPage, err := c.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(requestCtx, localizationID, opts...)
	cancel()
	if err != nil {
		return nil, err
	}

	result, err := paginateAssetResponse(ctx, firstPage, func(parentCtx context.Context, nextURL string) (PaginatedResponse, error) {
		nextCtx, nextCancel := requestContextFor(parentCtx, query.requestContext)
		defer nextCancel()
		return c.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(nextCtx, "", WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
