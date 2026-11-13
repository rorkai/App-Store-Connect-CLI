package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Live App Store version state spellings. appVersionState reports a version
// that is on the App Store as READY_FOR_DISTRIBUTION; the deprecated
// appStoreState reports it as READY_FOR_SALE. Apple returns the two
// attributes inconsistently across versions and the READY_FOR_DISTRIBUTION to
// READY_FOR_SALE remapping happens only in App Store Connect's web client
// (docs/API_NOTES.md), so live-version checks must recognize both.
// appVersionState has no pre-order value; a pre-order listing is visible only
// as appStoreState PREORDER_READY_FOR_SALE.
const (
	liveAppVersionState       = "READY_FOR_DISTRIBUTION"
	liveAppStoreState         = "READY_FOR_SALE"
	preorderLiveAppStoreState = "PREORDER_READY_FOR_SALE"
)

// IsLiveAppStoreVersion reports whether a version is live on the App Store. It
// decides from appVersionState when Apple returns it and falls back to the
// deprecated appStoreState only when appVersionState is absent, matching
// ResolveAppStoreVersionState and the status and review next actions.
func IsLiveAppStoreVersion(attrs asc.AppStoreVersionAttributes) bool {
	return IsLiveAppStoreVersionState(ResolveAppStoreVersionState(attrs))
}

// IsLiveOrPreorderAppStoreVersion reports whether a version is live or listed
// for pre-order. Pre-order has no appVersionState spelling, so the legacy
// appStoreState is consulted for it even when appVersionState is present.
func IsLiveOrPreorderAppStoreVersion(attrs asc.AppStoreVersionAttributes) bool {
	if IsLiveAppStoreVersion(attrs) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(attrs.AppStoreState), preorderLiveAppStoreState)
}

// AppStoreVersionStateFilter selects App Store versions by state across both
// state attributes. App Store Connect combines filter[appStoreState] and
// filter[appVersionState] with AND, so each non-empty spelling is sent as its
// own request and the results are merged.
type AppStoreVersionStateFilter struct {
	// AppStoreStates are deprecated appStoreState values.
	AppStoreStates []string
	// AppVersionStates are appVersionState values.
	AppVersionStates []string
}

// LiveAppStoreVersionStateFilter selects versions that are live on the App
// Store under either spelling. Callers may append further states to the
// returned filter; each call returns fresh slices.
func LiveAppStoreVersionStateFilter() AppStoreVersionStateFilter {
	return AppStoreVersionStateFilter{
		AppStoreStates:   []string{liveAppStoreState},
		AppVersionStates: []string{liveAppVersionState},
	}
}

// requestContextFunc derives the context for one App Store Connect request.
type requestContextFunc func(context.Context) (context.Context, context.CancelFunc)

// callerRequestContext sends every request with the caller's context, so the
// caller's deadline bounds the whole lookup.
func callerRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, func() {}
}

// ListAppStoreVersionsInStates lists every app store version of the app,
// optionally narrowed to one platform, whose appStoreState or appVersionState
// matches the filter. Versions reported under both spellings are returned
// once, in the order first seen (appStoreState results first). Every request
// uses ctx, so the caller's deadline bounds the whole lookup.
func ListAppStoreVersionsInStates(ctx context.Context, client *asc.Client, appID, platform string, filter AppStoreVersionStateFilter) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	return listAppStoreVersionsInStates(ctx, client, appID, platform, filter, callerRequestContext)
}

// ListAppStoreVersionsInStatesWithRequestTimeouts is ListAppStoreVersionsInStates
// for callers without their own deadline: each request, including every page
// and each state spelling, gets a fresh ContextWithTimeout budget, so the
// lookup is not capped by a single request timeout.
func ListAppStoreVersionsInStatesWithRequestTimeouts(ctx context.Context, client *asc.Client, appID, platform string, filter AppStoreVersionStateFilter) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	return listAppStoreVersionsInStates(ctx, client, appID, platform, filter, ContextWithTimeout)
}

func listAppStoreVersionsInStates(ctx context.Context, client *asc.Client, appID, platform string, filter AppStoreVersionStateFilter, requestContext requestContextFunc) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	var merged []asc.Resource[asc.AppStoreVersionAttributes]
	seen := map[string]struct{}{}
	for _, stateOpt := range filter.options() {
		versions, err := listAppStoreVersionsWithState(ctx, client, appID, platform, stateOpt, requestContext)
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			if id := strings.TrimSpace(version.ID); id != "" {
				if _, duplicate := seen[id]; duplicate {
					continue
				}
				seen[id] = struct{}{}
			}
			merged = append(merged, version)
		}
	}
	return merged, nil
}

// HasAppStoreVersionInStates reports whether the app has at least one app
// store version, optionally narrowed to one platform, whose appStoreState or
// appVersionState matches the filter. It stops after the first match. Every
// request uses ctx, so the caller's deadline bounds the whole check.
func HasAppStoreVersionInStates(ctx context.Context, client *asc.Client, appID, platform string, filter AppStoreVersionStateFilter) (bool, error) {
	for _, stateOpt := range filter.options() {
		opts := []asc.AppStoreVersionsOption{stateOpt, asc.Limit[asc.AppStoreVersionsOption](1)}
		if trimmed := strings.TrimSpace(platform); trimmed != "" {
			opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{trimmed}))
		}
		versions, err := client.GetAppStoreVersions(ctx, appID, opts...)
		if err != nil {
			return false, err
		}
		if versions != nil && len(versions.Data) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (filter AppStoreVersionStateFilter) options() []asc.AppStoreVersionsOption {
	opts := make([]asc.AppStoreVersionsOption, 0, 2)
	if len(filter.AppStoreStates) > 0 {
		opts = append(opts, asc.WithAppStoreVersionsStates(filter.AppStoreStates))
	}
	if len(filter.AppVersionStates) > 0 {
		opts = append(opts, asc.WithAppStoreVersionsVersionStates(filter.AppVersionStates))
	}
	return opts
}

// listAppStoreVersionsWithState lists every page of the app's versions for one
// state option, at 200 versions per page. requestContext derives the context
// for each request.
func listAppStoreVersionsWithState(ctx context.Context, client *asc.Client, appID, platform string, stateOpt asc.AppStoreVersionsOption, requestContext requestContextFunc) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	opts := []asc.AppStoreVersionsOption{stateOpt, asc.Limit[asc.AppStoreVersionsOption](200)}
	if trimmed := strings.TrimSpace(platform); trimmed != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{trimmed}))
	}
	firstCtx, cancel := requestContext(ctx)
	firstPage, err := client.GetAppStoreVersions(firstCtx, appID, opts...)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to list app store versions: %w", err)
	}
	if firstPage == nil {
		return nil, nil
	}
	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := requestContext(ctx)
		defer pageCancel()
		return client.GetAppStoreVersions(pageCtx, appID, asc.NextURL[asc.AppStoreVersionsOption](nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list app store versions: %w", err)
	}
	typed, ok := all.(*asc.AppStoreVersionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected paginated response type %T", all)
	}
	return typed.Data, nil
}
