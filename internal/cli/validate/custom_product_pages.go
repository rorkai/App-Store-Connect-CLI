package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	customProductPageVersionReplaced = "REPLACED_WITH_NEW_VERSION"
	// includedSearchKeywordsLimit is the API maximum for limit[searchKeywords].
	includedSearchKeywordsLimit = 50
)

// fetchCustomProductPageKeywords lists the search keywords of every
// localization on the app's current custom product page versions.
func fetchCustomProductPageKeywords(ctx context.Context, client *asc.Client, appID string) ([]validation.CustomProductPageLocalization, error) {
	appID = strings.TrimSpace(appID)
	pages, err := fetchAllReadinessPages(ctx, func(requestCtx context.Context, nextURL string) (*asc.AppCustomProductPagesResponse, error) {
		return client.GetAppCustomProductPages(requestCtx, appID, asc.Limit[asc.AppCustomProductPagesOption](200), asc.NextURL[asc.AppCustomProductPagesOption](nextURL))
	})
	if err != nil || len(pages) == 0 {
		return nil, err
	}

	var mu sync.Mutex
	var localizations []validation.CustomProductPageLocalization
	tasks := make([]readinessTask, 0, len(pages))
	for _, page := range pages {
		tasks = append(tasks, func(taskCtx context.Context) error {
			versions, err := fetchAllReadinessPages(taskCtx, func(requestCtx context.Context, nextURL string) (*asc.AppCustomProductPageVersionsResponse, error) {
				return client.GetAppCustomProductPageVersions(requestCtx, page.ID, asc.Limit[asc.AppCustomProductPageVersionsOption](200), asc.NextURL[asc.AppCustomProductPageVersionsOption](nextURL))
			})
			if err != nil {
				return err
			}
			for _, version := range versions {
				if strings.EqualFold(strings.TrimSpace(version.Attributes.State), customProductPageVersionReplaced) {
					continue
				}
				locs, err := fetchAllReadinessPages(taskCtx, func(requestCtx context.Context, nextURL string) (*asc.AppCustomProductPageLocalizationsResponse, error) {
					return client.GetAppCustomProductPageLocalizations(
						requestCtx,
						version.ID,
						asc.Limit[asc.AppCustomProductPageLocalizationsOption](200),
						asc.WithAppCustomProductPageLocalizationsSearchKeywords(includedSearchKeywordsLimit),
						asc.NextURL[asc.AppCustomProductPageLocalizationsOption](nextURL),
					)
				})
				if err != nil {
					return err
				}
				for _, loc := range locs {
					entry := validation.CustomProductPageLocalization{
						PageName:       page.Attributes.Name,
						LocalizationID: loc.ID,
						Locale:         loc.Attributes.Locale,
					}
					keywords, err := includedSearchKeywords(loc.Relationships)
					if err != nil {
						return fmt.Errorf("custom product page localization %q: %w", loc.ID, err)
					}
					entry.SearchKeywords = keywords
					mu.Lock()
					localizations = append(localizations, entry)
					mu.Unlock()
				}
			}
			return nil
		})
	}
	if err := runReadinessTasks(ctx, tasks...); err != nil {
		return nil, err
	}

	sort.Slice(localizations, func(i, j int) bool {
		if localizations[i].PageName != localizations[j].PageName {
			return localizations[i].PageName < localizations[j].PageName
		}
		if localizations[i].Locale != localizations[j].Locale {
			return localizations[i].Locale < localizations[j].Locale
		}
		return localizations[i].LocalizationID < localizations[j].LocalizationID
	})
	return localizations, nil
}

// includedSearchKeywords reads the keywords included in a localization's
// searchKeywords relationship. Apple caps each locale's keywords at 100 bytes
// of comma-separated terms longer than two characters, so a page cannot hold
// more keywords than limit[searchKeywords] returns; an incomplete list is
// reported as an error instead of fetched again.
func includedSearchKeywords(relationships json.RawMessage) ([]string, error) {
	var parsed struct {
		SearchKeywords *struct {
			Data []asc.RelationshipData `json:"data"`
			Meta struct {
				Paging struct {
					Total *int `json:"total"`
				} `json:"paging"`
			} `json:"meta"`
		} `json:"searchKeywords"`
	}
	if len(relationships) == 0 || json.Unmarshal(relationships, &parsed) != nil || parsed.SearchKeywords == nil {
		return nil, fmt.Errorf("search keywords were not included in the response")
	}
	related := parsed.SearchKeywords
	if total := related.Meta.Paging.Total; total != nil && *total > len(related.Data) {
		return nil, fmt.Errorf("the response reports %d search keywords but includes %d", *total, len(related.Data))
	}
	keywords := make([]string, 0, len(related.Data))
	for _, keyword := range related.Data {
		keywords = append(keywords, keyword.ID)
	}
	return keywords, nil
}

// fetchAllReadinessPages fetches every page of a list endpoint through the
// readiness request gate. fetch receives an empty nextURL for the first page.
func fetchAllReadinessPages[T any](ctx context.Context, fetch func(context.Context, string) (*asc.Response[T], error)) ([]asc.Resource[T], error) {
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.Response[T], error) {
		return fetch(requestCtx, "")
	})
	if err != nil {
		return nil, err
	}
	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return fetch(requestCtx, nextURL)
		})
	})
	if err != nil {
		return nil, err
	}
	resp, ok := paginated.(*asc.Response[T])
	if !ok {
		return nil, fmt.Errorf("unexpected pagination response type: %T", paginated)
	}
	return resp.Data, nil
}
