package assets

import (
	"context"
	"fmt"
	"net/url"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func validateScreenshotCollectionNextURL(client *asc.Client, next, collectionPath string) error {
	parsed, err := url.Parse(next)
	if err != nil {
		return fmt.Errorf("invalid screenshot pagination URL")
	}
	expected, err := url.PathUnescape(collectionPath)
	if err != nil || parsed.Path != expected || parsed.EscapedPath() != collectionPath ||
		parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return fmt.Errorf("screenshot pagination left the selected collection")
	}
	if parsed.Scheme == "" && parsed.Host == "" {
		return nil
	}
	origin := asc.BaseURL
	if client.IsEnterpriseProgramAPI() {
		origin = asc.EnterpriseBaseURL
	}
	if parsed.Scheme+"://"+parsed.Host != origin {
		return fmt.Errorf("screenshot pagination left the selected API origin")
	}
	return nil
}

func collectScopedScreenshotPages[T any](ctx context.Context, client *asc.Client, collectionPath string, requestContext asc.RequestContextFunc, fetch func(context.Context, string) (*asc.Response[T], error)) (*asc.Response[T], error) {
	fetchPage := func(parent context.Context, next string) (*asc.Response[T], error) {
		if next != "" {
			if err := validateScreenshotCollectionNextURL(client, next, collectionPath); err != nil {
				return nil, err
			}
		}
		factory := requestContext
		if factory == nil {
			factory = context.WithCancel
		}
		requestCtx, cancel := factory(parent)
		defer cancel()
		return fetch(requestCtx, next)
	}
	firstPage, err := fetchPage(ctx, "")
	if err != nil {
		return nil, err
	}
	result, err := asc.PaginateAll(ctx, firstPage, func(parent context.Context, next string) (asc.PaginatedResponse, error) {
		return fetchPage(parent, next)
	})
	if err != nil {
		return nil, err
	}
	response, ok := result.(*asc.Response[T])
	if !ok {
		return nil, fmt.Errorf("unexpected screenshot collection response %T", result)
	}
	return response, nil
}

func getScopedAppScreenshotSets(ctx context.Context, client *asc.Client, localizationID string, requestContext asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
	collectionPath := "/v1/appStoreVersionLocalizations/" + url.PathEscape(localizationID) + "/appScreenshotSets"
	return collectScopedScreenshotPages(ctx, client, collectionPath, requestContext, func(requestCtx context.Context, next string) (*asc.AppScreenshotSetsResponse, error) {
		return client.GetAppScreenshotSets(requestCtx, localizationID, asc.NextURL[asc.AppScreenshotSetsOption](next))
	})
}

func getScopedAppScreenshots(ctx context.Context, client *asc.Client, setID string, requestContext asc.RequestContextFunc) (*asc.AppScreenshotsResponse, error) {
	collectionPath := "/v1/appScreenshotSets/" + url.PathEscape(setID) + "/appScreenshots"
	return collectScopedScreenshotPages(ctx, client, collectionPath, requestContext, func(requestCtx context.Context, next string) (*asc.AppScreenshotsResponse, error) {
		return client.GetAppScreenshots(requestCtx, setID, asc.NextURL[asc.AppScreenshotsOption](next))
	})
}
