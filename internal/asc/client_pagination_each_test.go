package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPaginateEach_MultiPage(t *testing.T) {
	const totalPages = 3
	const perPage = 2

	firstPage := makeAppsPage(1, perPage, totalPages)
	var ids []string

	err := PaginateEach(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makeAppsPage(page, perPage, totalPages), nil
	}, func(page PaginatedResponse) error {
		apps, ok := page.(*AppsResponse)
		if !ok {
			return fmt.Errorf("unexpected page type %T", page)
		}
		for _, app := range apps.Data {
			ids = append(ids, app.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("PaginateEach() error: %v", err)
	}

	expected := totalPages * perPage
	if len(ids) != expected {
		t.Fatalf("expected %d ids, got %d", expected, len(ids))
	}
}

func TestPaginateEach_ConsumerErrorIncludesPage(t *testing.T) {
	firstPage := makeAppsPage(1, 1, 2)
	consumerErr := errors.New("consumer failed")

	err := PaginateEach(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return makeAppsPage(2, 1, 2), nil
	}, func(page PaginatedResponse) error {
		apps := page.(*AppsResponse)
		if len(apps.Data) > 0 && apps.Data[0].ID == "app-2-0" {
			return consumerErr
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, consumerErr) {
		t.Fatalf("expected wrapped consumer error, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "page 2") {
		t.Fatalf("expected page 2 context in error, got %q", got)
	}
}

func TestPaginateEach_NonPointerResponse(t *testing.T) {
	firstPage := valuePaginatedResponse{links: Links{Next: "next"}, data: []string{"first"}}
	consumed := 0

	err := PaginateEach(context.Background(), firstPage, func(context.Context, string) (PaginatedResponse, error) {
		return valuePaginatedResponse{data: []string{"second"}}, nil
	}, func(PaginatedResponse) error {
		consumed++
		return nil
	})
	if err != nil {
		t.Fatalf("PaginateEach() error: %v", err)
	}
	if consumed != 2 {
		t.Fatalf("consumed %d pages, want 2", consumed)
	}
}

func TestPaginateEach_NilFetcherWithNextLink(t *testing.T) {
	firstPage := makeAppsPage(1, 1, 2)
	consumed := 0

	err := PaginateEach(context.Background(), firstPage, nil, func(PaginatedResponse) error {
		consumed++
		return nil
	})
	if !errors.Is(err, ErrMissingPaginationFetcher) {
		t.Fatalf("expected ErrMissingPaginationFetcher, got %v", err)
	}
	if consumed != 0 {
		t.Fatalf("consumed %d pages before rejecting missing fetcher, want 0", consumed)
	}
}

func TestPaginateEach_TypedNilNextPage(t *testing.T) {
	firstPage := makeAppsPage(1, 1, 2)

	err := PaginateEach(context.Background(), firstPage, func(context.Context, string) (PaginatedResponse, error) {
		var nextPage *AppsResponse
		return nextPage, nil
	}, func(PaginatedResponse) error {
		return nil
	})
	if !errors.Is(err, ErrNilPaginationPage) {
		t.Fatalf("expected ErrNilPaginationPage, got %v", err)
	}
}

func TestPaginateEachWithMaxPagesStopsBeforeFetchingBeyondLimit(t *testing.T) {
	firstPage := makeAppsPage(1, 1, 2)
	fetchCalls := 0
	consumedPages := 0

	err := PaginateEachWithMaxPages(context.Background(), firstPage, func(_ context.Context, nextURL string) (PaginatedResponse, error) {
		fetchCalls++
		if nextURL != "page=2" {
			t.Fatalf("nextURL = %q, want page=2", nextURL)
		}
		return &AppsResponse{
			Data:  makeAppsPage(2, 1, 2).Data,
			Links: Links{Next: "page=3"},
		}, nil
	}, func(_ PaginatedResponse) error {
		consumedPages++
		return nil
	}, 2)

	if err == nil || !strings.Contains(err.Error(), "page 3") || !strings.Contains(err.Error(), "2-page safety limit") {
		t.Fatalf("expected page-limit error for page 3, got %v", err)
	}
	if fetchCalls != 1 {
		t.Fatalf("fetchNext calls = %d, want 1", fetchCalls)
	}
	if consumedPages != 2 {
		t.Fatalf("consumed pages = %d, want 2 before limit rejection", consumedPages)
	}
}
