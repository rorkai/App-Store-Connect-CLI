package asc

import (
	"context"
	"errors"
	"testing"
)

func TestPaginateAllRejectsEquivalentRepeatedNextBeforeRefetch(t *testing.T) {
	tests := []struct {
		name      string
		firstNext string
		pageNext  string
	}{
		{
			name:      "whitespace",
			firstNext: "/v1/apps?cursor=abc",
			pageNext:  "  /v1/apps?cursor=abc  ",
		},
		{
			name:      "relative and same-host absolute",
			firstNext: "/v1/apps?cursor=abc",
			pageNext:  "https://api.appstoreconnect.apple.com/v1/apps?cursor=abc",
		},
		{
			name:      "reordered query parameters",
			firstNext: "/v1/apps?cursor=abc&limit=200",
			pageNext:  "/v1/apps?limit=200&cursor=abc",
		},
		{
			name:      "reordered query parameters across relative and same-host absolute",
			firstNext: "/v1/apps?cursor=abc&limit=200",
			pageNext:  "https://api.appstoreconnect.apple.com/v1/apps?limit=200&cursor=abc",
		},
		{
			name:      "empty query marker",
			firstNext: "/v1/apps",
			pageNext:  "/v1/apps?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firstPage := makeAppsPage(1, 1, 1)
			firstPage.Links.Next = tt.firstNext
			fetchCalls := 0
			var fetchURLs []string

			result, err := PaginateAll(context.Background(), firstPage, func(_ context.Context, nextURL string) (PaginatedResponse, error) {
				fetchCalls++
				fetchURLs = append(fetchURLs, nextURL)
				page := makeAppsPage(2, 1, 1)
				page.Links.Next = tt.pageNext
				return page, nil
			})

			if !errors.Is(err, ErrRepeatedPaginationURL) {
				t.Fatalf("PaginateAll() error = %v, want ErrRepeatedPaginationURL", err)
			}
			if fetchCalls != 1 {
				t.Fatalf("fetchNext calls = %d, want 1", fetchCalls)
			}
			if len(fetchURLs) != 1 || fetchURLs[0] != tt.firstNext {
				t.Fatalf("fetchNext URLs = %q, want the original next URL %q", fetchURLs, tt.firstNext)
			}
			apps, ok := result.(*AppsResponse)
			if !ok {
				t.Fatalf("PaginateAll() result type = %T, want *AppsResponse", result)
			}
			if len(apps.Data) != 2 || apps.Data[0].ID != "app-1-0" || apps.Data[1].ID != "app-2-0" {
				t.Fatalf("aggregated data = %#v, want both fetched pages", apps.Data)
			}
		})
	}
}

func TestPaginateEachRejectsEquivalentRepeatedNextBeforeRefetch(t *testing.T) {
	tests := []struct {
		name      string
		firstNext string
		pageNext  string
	}{
		{
			name:      "whitespace",
			firstNext: "/v1/apps?cursor=abc",
			pageNext:  "  /v1/apps?cursor=abc  ",
		},
		{
			name:      "relative and same-host absolute",
			firstNext: "/v1/apps?cursor=abc",
			pageNext:  "https://api.appstoreconnect.apple.com/v1/apps?cursor=abc",
		},
		{
			name:      "reordered query parameters",
			firstNext: "/v1/apps?cursor=abc&limit=200",
			pageNext:  "/v1/apps?limit=200&cursor=abc",
		},
		{
			name:      "reordered query parameters across relative and same-host absolute",
			firstNext: "/v1/apps?cursor=abc&limit=200",
			pageNext:  "https://api.appstoreconnect.apple.com/v1/apps?limit=200&cursor=abc",
		},
		{
			name:      "empty query marker",
			firstNext: "/v1/apps",
			pageNext:  "/v1/apps?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firstPage := makeAppsPage(1, 1, 1)
			firstPage.Links.Next = tt.firstNext
			fetchCalls := 0
			var fetchURLs []string
			var consumed []string

			err := PaginateEach(context.Background(), firstPage, func(_ context.Context, nextURL string) (PaginatedResponse, error) {
				fetchCalls++
				fetchURLs = append(fetchURLs, nextURL)
				page := makeAppsPage(2, 1, 1)
				page.Links.Next = tt.pageNext
				return page, nil
			}, func(page PaginatedResponse) error {
				apps := page.(*AppsResponse)
				for _, app := range apps.Data {
					consumed = append(consumed, app.ID)
				}
				return nil
			})

			if !errors.Is(err, ErrRepeatedPaginationURL) {
				t.Fatalf("PaginateEach() error = %v, want ErrRepeatedPaginationURL", err)
			}
			if fetchCalls != 1 {
				t.Fatalf("fetchNext calls = %d, want 1", fetchCalls)
			}
			if len(fetchURLs) != 1 || fetchURLs[0] != tt.firstNext {
				t.Fatalf("fetchNext URLs = %q, want the original next URL %q", fetchURLs, tt.firstNext)
			}
			if len(consumed) != 2 || consumed[0] != "app-1-0" || consumed[1] != "app-2-0" {
				t.Fatalf("consumed data = %v, want both fetched pages", consumed)
			}
		})
	}
}
