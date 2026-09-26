package asc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// makeBetaGroupsPage creates a BetaGroupsResponse page for testing pagination.
func makeBetaGroupsPage(page, perPage, totalPages int) *BetaGroupsResponse {
	data := make([]Resource[BetaGroupAttributes], 0, perPage)
	for i := range perPage {
		data = append(data, Resource[BetaGroupAttributes]{
			Type:       ResourceTypeBetaGroups,
			ID:         fmt.Sprintf("group-%d-%d", page, i),
			Attributes: BetaGroupAttributes{Name: fmt.Sprintf("Group %d-%d", page, i)},
		})
	}
	links := Links{}
	if page < totalPages {
		links.Next = fmt.Sprintf("page=%d", page+1)
	}
	return &BetaGroupsResponse{Data: data, Links: links}
}

// makeAppsPage creates an AppsResponse page for testing pagination.
func makeAppsPage(page, perPage, totalPages int) *AppsResponse {
	data := make([]Resource[AppAttributes], 0, perPage)
	for i := range perPage {
		data = append(data, Resource[AppAttributes]{
			Type: ResourceTypeApps,
			ID:   fmt.Sprintf("app-%d-%d", page, i),
		})
	}
	links := Links{}
	if page < totalPages {
		links.Next = fmt.Sprintf("page=%d", page+1)
	}
	return &AppsResponse{Data: data, Links: links}
}

// parseMockPageNum extracts the page number from a mock nextURL like "page=3".
func parseMockPageNum(nextURL string) (int, error) {
	pageStr := strings.TrimPrefix(nextURL, "page=")
	return strconv.Atoi(pageStr)
}

func TestPaginateAll_SinglePage(t *testing.T) {
	firstPage := makeBetaGroupsPage(1, 3, 1) // 1 page total, no next link

	fetchCalls := 0
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		fetchCalls++
		return nil, fmt.Errorf("should not be called")
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}
	if fetchCalls != 0 {
		t.Fatalf("expected 0 fetchNext calls for single page, got %d", fetchCalls)
	}

	groups, ok := result.(*BetaGroupsResponse)
	if !ok {
		t.Fatalf("expected *BetaGroupsResponse, got %T", result)
	}
	if len(groups.Data) != 3 {
		t.Fatalf("expected 3 items, got %d", len(groups.Data))
	}
}

func TestPaginateAll_MultiPage(t *testing.T) {
	const totalPages = 3
	const perPage = 2

	firstPage := makeBetaGroupsPage(1, perPage, totalPages)
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q: %w", nextURL, err)
		}
		return makeBetaGroupsPage(page, perPage, totalPages), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	groups, ok := result.(*BetaGroupsResponse)
	if !ok {
		t.Fatalf("expected *BetaGroupsResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(groups.Data) != expected {
		t.Fatalf("expected %d items, got %d", expected, len(groups.Data))
	}
	// Verify items from all pages are present
	if groups.Data[0].ID != "group-1-0" {
		t.Fatalf("expected first item from page 1, got %q", groups.Data[0].ID)
	}
	if groups.Data[expected-1].ID != fmt.Sprintf("group-%d-%d", totalPages, perPage-1) {
		t.Fatalf("expected last item from page %d, got %q", totalPages, groups.Data[expected-1].ID)
	}
}

func TestPaginateAll_APIErrorOnPageN(t *testing.T) {
	const totalPages = 5
	const perPage = 2
	const failOnPage = 3

	firstPage := makeAppsPage(1, perPage, totalPages)
	apiErr := fmt.Errorf("server error on page %d", failOnPage)

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, parseErr := parseMockPageNum(nextURL)
		if parseErr != nil {
			return nil, parseErr
		}
		if page == failOnPage {
			return nil, apiErr
		}
		return makeAppsPage(page, perPage, totalPages), nil
	})

	// Should return an error
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should wrap the page number
	if !strings.Contains(err.Error(), fmt.Sprintf("page %d", failOnPage)) {
		t.Fatalf("expected error to mention page %d, got: %v", failOnPage, err)
	}

	// Should return partial results collected before the error
	if result == nil {
		t.Fatal("expected partial results, got nil")
	}
	apps, ok := result.(*AppsResponse)
	if !ok {
		t.Fatalf("expected *AppsResponse, got %T", result)
	}
	// Pages 1 and 2 should have been aggregated before page 3 failed
	expectedItems := (failOnPage - 1) * perPage
	if len(apps.Data) != expectedItems {
		t.Fatalf("expected %d partial items (pages 1-%d), got %d", expectedItems, failOnPage-1, len(apps.Data))
	}
}

func TestPaginateAll_RepeatedURL_Sentinel(t *testing.T) {
	firstPage := &BetaGroupsResponse{
		Data: []Resource[BetaGroupAttributes]{
			{Type: ResourceTypeBetaGroups, ID: "group-1"},
		},
		Links: Links{Next: "page=1"},
	}

	_, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return &BetaGroupsResponse{
			Data: []Resource[BetaGroupAttributes]{
				{Type: ResourceTypeBetaGroups, ID: "group-2"},
			},
			Links: Links{Next: "page=1"}, // Same URL → repeated
		}, nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got: %v", err)
	}
}

func TestPaginateAll_TypeMismatch(t *testing.T) {
	firstPage := &AppsResponse{
		Data: []Resource[AppAttributes]{
			{Type: ResourceTypeApps, ID: "app-1"},
		},
		Links: Links{Next: "page=2"},
	}

	_, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		// Return a different type on page 2
		return &BetaGroupsResponse{
			Data: []Resource[BetaGroupAttributes]{
				{Type: ResourceTypeBetaGroups, ID: "group-1"},
			},
		}, nil
	})

	if err == nil {
		t.Fatal("expected error for type mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected response type") {
		t.Fatalf("expected type mismatch error, got: %v", err)
	}
}

func TestPaginateAll_NilFirstPage(t *testing.T) {
	result, err := PaginateAll(context.Background(), nil, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return nil, fmt.Errorf("should not be called")
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got: %v", result)
	}
}

func TestPaginateAll_TypedNilFirstPage(t *testing.T) {
	// A typed nil (non-nil interface containing a nil pointer) should not panic.
	// This tests the edge case where someone accidentally passes a typed nil.
	var typedNil *BetaGroupsResponse = nil
	var firstPage PaginatedResponse = typedNil // interface is non-nil, but contains nil pointer

	// The function should handle this gracefully without panicking
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return nil, fmt.Errorf("should not be called")
	})
	// Should succeed and return an empty result (no data, no links to follow)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for typed nil input")
	}
	groups, ok := result.(*BetaGroupsResponse)
	if !ok {
		t.Fatalf("expected *BetaGroupsResponse, got %T", result)
	}
	if len(groups.Data) != 0 {
		t.Fatalf("expected 0 items, got %d", len(groups.Data))
	}
}

func TestPaginateAll_NonPointerResponseReturnsError(t *testing.T) {
	firstPage := valuePaginatedResponse{data: []string{"first"}}

	_, err := PaginateAll(context.Background(), firstPage, nil)
	if err == nil {
		t.Fatal("expected an unsupported response error, got nil")
	}
	if !strings.Contains(err.Error(), "expected pointer") {
		t.Fatalf("expected pointer error, got %v", err)
	}
}

func TestPaginateAll_NilFetcherWithNextLink(t *testing.T) {
	firstPage := makeBetaGroupsPage(1, 1, 2)

	_, err := PaginateAll(context.Background(), firstPage, nil)
	if !errors.Is(err, ErrMissingPaginationFetcher) {
		t.Fatalf("expected ErrMissingPaginationFetcher, got %v", err)
	}
}

func TestPaginateAll_TypedNilNextPage(t *testing.T) {
	firstPage := makeBetaGroupsPage(1, 1, 2)

	_, err := PaginateAll(context.Background(), firstPage, func(context.Context, string) (PaginatedResponse, error) {
		var nextPage *BetaGroupsResponse
		return nextPage, nil
	})
	if !errors.Is(err, ErrNilPaginationPage) {
		t.Fatalf("expected ErrNilPaginationPage, got %v", err)
	}
}

func TestPaginateAll_PointerToNonStructResponseReturnsError(t *testing.T) {
	page := pointerToNonStructPaginatedResponse{"first"}

	_, err := PaginateAll(context.Background(), &page, nil)
	if err == nil {
		t.Fatal("expected an unsupported response error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported response type") {
		t.Fatalf("expected unsupported response error, got %v", err)
	}
}

func TestPaginateAll_TypedNilPointerToNonStructResponseReturnsError(t *testing.T) {
	var page *pointerToNonStructPaginatedResponse

	_, err := PaginateAll(context.Background(), page, nil)
	if err == nil {
		t.Fatal("expected an unsupported response error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported response type") {
		t.Fatalf("expected unsupported response error, got %v", err)
	}
}

// valuePaginatedResponse verifies that paginator input validation does not
// call reflect.Value.IsNil on a non-nilable concrete implementation.
type valuePaginatedResponse struct {
	links Links
	data  []string
}

func (r valuePaginatedResponse) GetLinks() *Links { return &r.links }
func (r valuePaginatedResponse) GetData() any     { return r.data }

type pointerToNonStructPaginatedResponse []string

func (r *pointerToNonStructPaginatedResponse) GetLinks() *Links { return nil }
func (r *pointerToNonStructPaginatedResponse) GetData() any     { return []string(*r) }

func TestPaginateAll_EmptyData(t *testing.T) {
	firstPage := &BetaTestersResponse{
		Data:  []Resource[BetaTesterAttributes]{},
		Links: Links{}, // No next link
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return nil, fmt.Errorf("should not be called")
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	testers, ok := result.(*BetaTestersResponse)
	if !ok {
		t.Fatalf("expected *BetaTestersResponse, got %T", result)
	}
	if len(testers.Data) != 0 {
		t.Fatalf("expected 0 items, got %d", len(testers.Data))
	}
}

func TestPaginateAll_UnsupportedType(t *testing.T) {
	// Create a type that implements PaginatedResponse but lacks a Data field.
	// With reflection-based pagination, types without a Data slice field
	// are rejected during aggregation rather than via a type switch.
	unsupported := &unsupportedPaginatedResponse{
		links: Links{},
		data:  []string{"a", "b"},
	}

	_, err := PaginateAll(context.Background(), unsupported, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return nil, fmt.Errorf("should not be called")
	})

	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	// The error should mention the Data field issue (reflection-based check)
	if !strings.Contains(err.Error(), "Data field") && !strings.Contains(err.Error(), "unsupported response type") {
		t.Fatalf("expected Data field or unsupported type error, got: %v", err)
	}
}

// unsupportedPaginatedResponse is a test-only type that implements PaginatedResponse
// but is not registered in the PaginateAll type switch.
type unsupportedPaginatedResponse struct {
	links Links
	data  []string
}

func (r *unsupportedPaginatedResponse) GetLinks() *Links { return &r.links }
func (r *unsupportedPaginatedResponse) GetData() any     { return r.data }

// rawDataPaginatedResponse is a test-only type whose GetData returns raw JSON
// bytes instead of an item slice.
type rawDataPaginatedResponse struct {
	links Links
	data  json.RawMessage
}

func (r *rawDataPaginatedResponse) GetLinks() *Links { return &r.links }
func (r *rawDataPaginatedResponse) GetData() any     { return r.data }

// nonSliceDataPaginatedResponse is a test-only type whose GetData does not
// return a slice at all.
type nonSliceDataPaginatedResponse struct {
	links Links
}

func (r *nonSliceDataPaginatedResponse) GetLinks() *Links { return &r.links }
func (r *nonSliceDataPaginatedResponse) GetData() any     { return "not a slice" }

func TestPageDataLen(t *testing.T) {
	t.Run("counts resource slice", func(t *testing.T) {
		page := makeBetaGroupsPage(1, 3, 2)
		count, ok := PageDataLen(page)
		if !ok || count != 3 {
			t.Fatalf("PageDataLen() = (%d, %t), want (3, true)", count, ok)
		}
	})

	t.Run("counts empty resource slice", func(t *testing.T) {
		page := &BetaGroupsResponse{Data: []Resource[BetaGroupAttributes]{}}
		count, ok := PageDataLen(page)
		if !ok || count != 0 {
			t.Fatalf("PageDataLen() = (%d, %t), want (0, true)", count, ok)
		}
	})

	t.Run("counts non-resource item slice", func(t *testing.T) {
		page := &unsupportedPaginatedResponse{data: []string{"a", "b"}}
		count, ok := PageDataLen(page)
		if !ok || count != 2 {
			t.Fatalf("PageDataLen() = (%d, %t), want (2, true)", count, ok)
		}
	})

	t.Run("nil interface not counted", func(t *testing.T) {
		count, ok := PageDataLen(nil)
		if ok || count != 0 {
			t.Fatalf("PageDataLen(nil) = (%d, %t), want (0, false)", count, ok)
		}
	})

	t.Run("typed nil not counted", func(t *testing.T) {
		var typedNil *BetaGroupsResponse
		var page PaginatedResponse = typedNil
		count, ok := PageDataLen(page)
		if ok || count != 0 {
			t.Fatalf("PageDataLen(typed nil) = (%d, %t), want (0, false)", count, ok)
		}
	})

	t.Run("raw JSON data not counted as bytes", func(t *testing.T) {
		page := &rawDataPaginatedResponse{data: json.RawMessage(`[{"id":"a"},{"id":"b"}]`)}
		count, ok := PageDataLen(page)
		if ok || count != 0 {
			t.Fatalf("PageDataLen(raw JSON data) = (%d, %t), want (0, false)", count, ok)
		}
	})

	t.Run("non-slice data not counted", func(t *testing.T) {
		page := &nonSliceDataPaginatedResponse{}
		count, ok := PageDataLen(page)
		if ok || count != 0 {
			t.Fatalf("PageDataLen(non-slice data) = (%d, %t), want (0, false)", count, ok)
		}
	})
}

func TestPaginateAll_ContextCancelled(t *testing.T) {
	firstPage := &AppsResponse{
		Data: []Resource[AppAttributes]{
			{Type: ResourceTypeApps, ID: "app-1"},
		},
		Links: Links{Next: "page=2"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		// The caller should pass ctx through to network calls; simulate that check
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return makeAppsPage(2, 1, 2), nil
	})

	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestPaginateAll_LinkagesResponse(t *testing.T) {
	const totalPages = 2
	const perPage = 3

	firstPage := &LinkagesResponse{
		Data:  make([]ResourceData, perPage),
		Links: Links{Next: "page=2"},
	}
	for i := range perPage {
		firstPage.Data[i] = ResourceData{Type: ResourceTypeBetaGroups, ID: fmt.Sprintf("linkage-1-%d", i)}
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page2 := &LinkagesResponse{
			Data:  make([]ResourceData, perPage),
			Links: Links{},
		}
		for i := range perPage {
			page2.Data[i] = ResourceData{Type: ResourceTypeBetaGroups, ID: fmt.Sprintf("linkage-2-%d", i)}
		}
		return page2, nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	linkages, ok := result.(*LinkagesResponse)
	if !ok {
		t.Fatalf("expected *LinkagesResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(linkages.Data) != expected {
		t.Fatalf("expected %d linkages, got %d", expected, len(linkages.Data))
	}
}

func TestPaginateAll_PreReleaseVersionsResponse(t *testing.T) {
	const totalPages = 2
	const perPage = 2

	firstPage := &PreReleaseVersionsResponse{
		Data: []PreReleaseVersion{
			{Type: ResourceTypePreReleaseVersions, ID: "prv-1-0"},
			{Type: ResourceTypePreReleaseVersions, ID: "prv-1-1"},
		},
		Links:    Links{Self: "page=1", First: "page=1", Next: "page=2"},
		Included: json.RawMessage(`[{"type":"apps","id":"app-1"}]`),
		Meta:     json.RawMessage(`{"paging":{"total":4,"limit":2}}`),
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return &PreReleaseVersionsResponse{
			Data: []PreReleaseVersion{
				{Type: ResourceTypePreReleaseVersions, ID: "prv-2-0"},
				{Type: ResourceTypePreReleaseVersions, ID: "prv-2-1"},
			},
			Links:    Links{},
			Included: json.RawMessage(`[{"type":"builds","id":"build-1"}]`),
		}, nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	versions, ok := result.(*PreReleaseVersionsResponse)
	if !ok {
		t.Fatalf("expected *PreReleaseVersionsResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(versions.Data) != expected {
		t.Fatalf("expected %d versions, got %d", expected, len(versions.Data))
	}
	if versions.Links != (Links{}) {
		t.Fatalf("expected page-local links to be cleared, got %#v", versions.Links)
	}
	if len(versions.Meta) != 0 {
		t.Fatalf("expected page-local meta to be cleared, got %s", versions.Meta)
	}
	var included []Resource[json.RawMessage]
	if err := json.Unmarshal(versions.Included, &included); err != nil {
		t.Fatalf("decode included resources: %v", err)
	}
	if len(included) != 2 {
		t.Fatalf("expected 2 included resources, got %d", len(included))
	}
	gotIDs := make(map[string]bool, len(included))
	for _, resource := range included {
		gotIDs[resource.ID] = true
	}
	for _, id := range []string{"app-1", "build-1"} {
		if !gotIDs[id] {
			t.Fatalf("missing included resource %q: %#v", id, included)
		}
	}
}

func TestPaginateAll_PreReleaseVersionsEmptyDataIsArray(t *testing.T) {
	result, err := PaginateAll(context.Background(), &PreReleaseVersionsResponse{
		Data:  []PreReleaseVersion{},
		Links: Links{Self: "page=1"},
		Meta:  json.RawMessage(`{"paging":{"total":0,"limit":50}}`),
	}, func(context.Context, string) (PaginatedResponse, error) {
		t.Fatal("unexpected next-page fetch")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal paginated response: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("unmarshal paginated response: %v", err)
	}
	if string(envelope["data"]) != "[]" {
		t.Fatalf("expected empty data array, got %s", envelope["data"])
	}
	if result.GetLinks().Self != "page=1" {
		t.Fatalf("expected document links to be preserved, got %#v", result.GetLinks())
	}
	versions, ok := result.(*PreReleaseVersionsResponse)
	if !ok {
		t.Fatalf("expected *PreReleaseVersionsResponse, got %T", result)
	}
	if string(versions.Meta) != `{"paging":{"total":0,"limit":50}}` {
		t.Fatalf("expected document meta to be preserved, got %s", versions.Meta)
	}
}

func TestPaginateAll_ManyPages_BetaTesters(t *testing.T) {
	const totalPages = 10
	const perPage = 5

	makePage := func(page int) *BetaTestersResponse {
		data := make([]Resource[BetaTesterAttributes], 0, perPage)
		for i := range perPage {
			data = append(data, Resource[BetaTesterAttributes]{
				Type: ResourceTypeBetaTesters,
				ID:   fmt.Sprintf("tester-%d-%d", page, i),
				Attributes: BetaTesterAttributes{
					Email: fmt.Sprintf("tester-%d-%d@example.com", page, i),
				},
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &BetaTestersResponse{Data: data, Links: links}
	}

	firstPage := makePage(1)
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	testers, ok := result.(*BetaTestersResponse)
	if !ok {
		t.Fatalf("expected *BetaTestersResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(testers.Data) != expected {
		t.Fatalf("expected %d testers, got %d", expected, len(testers.Data))
	}
	// Verify last item
	last := testers.Data[expected-1]
	if last.ID != fmt.Sprintf("tester-%d-%d", totalPages, perPage-1) {
		t.Fatalf("expected last tester ID tester-%d-%d, got %q", totalPages, perPage-1, last.ID)
	}
}

func TestPaginateAll_Builds(t *testing.T) {
	const totalPages = 3
	const perPage = 4

	makePage := func(page int) *BuildsResponse {
		data := make([]Resource[BuildAttributes], 0, perPage)
		for i := range perPage {
			data = append(data, Resource[BuildAttributes]{
				Type: ResourceTypeBuilds,
				ID:   fmt.Sprintf("build-%d-%d", page, i),
				Attributes: BuildAttributes{
					Version: fmt.Sprintf("%d.%d", page, i),
				},
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &BuildsResponse{Data: data, Links: links}
	}

	firstPage := makePage(1)
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected *BuildsResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(builds.Data) != expected {
		t.Fatalf("expected %d builds, got %d", expected, len(builds.Data))
	}
}

func TestPaginateAll_BuildsPreservesIncluded(t *testing.T) {
	firstPage := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type: ResourceTypeBuilds,
				ID:   "build-1",
			},
		},
		Included: json.RawMessage(`[
			{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"TV_OS"}}
		]`),
		Links: Links{Next: "page=2"},
	}

	secondPage := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type: ResourceTypeBuilds,
				ID:   "build-2",
			},
		},
		Included: json.RawMessage(`[
			{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"2.0.0","platform":"IOS"}}
		]`),
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		if nextURL != "page=2" {
			t.Fatalf("expected next URL page=2, got %q", nextURL)
		}
		return secondPage, nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected *BuildsResponse, got %T", result)
	}
	if len(builds.Data) != 2 {
		t.Fatalf("expected 2 builds, got %d", len(builds.Data))
	}

	var included []Resource[PreReleaseVersionAttributes]
	if err := json.Unmarshal(builds.Included, &included); err != nil {
		t.Fatalf("expected merged included payload to be valid JSON, got %v", err)
	}
	if len(included) != 2 {
		t.Fatalf("expected 2 included pre-release versions, got %d", len(included))
	}
	if included[0].ID != "prv-1" || included[1].ID != "prv-2" {
		t.Fatalf("unexpected included IDs: %+v", included)
	}
}

func TestRawJSONArrayAccumulatorDeduplicatesJSONAPIResourcesByIdentity(t *testing.T) {
	acc := &rawJSONArrayAccumulator{}
	for i, payload := range []json.RawMessage{
		json.RawMessage(`[
			{"type":"betaGroups","id":"group-a","attributes":{"name":"Alpha"}},
			{"type":"apps","id":"shared-id"}
		]`),
		json.RawMessage(`[
			{"id":"group-a","type":"betaGroups","attributes":{"name":"Alpha updated"}},
			{"type":"betaGroups","id":"group-b","attributes":{"name":"Bravo"}},
			{"type":"builds","id":"shared-id"}
		]`),
	} {
		if err := acc.add(payload); err != nil {
			t.Fatalf("add payload %d: %v", i+1, err)
		}
	}
	merged, err := acc.merged()
	if err != nil {
		t.Fatalf("acc.merged() error: %v", err)
	}

	var included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(merged, &included); err != nil {
		t.Fatalf("decode merged resources: %v", err)
	}
	if len(included) != 4 {
		t.Fatalf("expected four distinct type-and-ID resources, got %+v", included)
	}
	if included[0].Type != "betaGroups" || included[0].ID != "group-a" || included[0].Attributes.Name != "Alpha" {
		t.Fatalf("expected the first group-a representation to win, got %+v", included[0])
	}
	if included[1].Type != "apps" || included[1].ID != "shared-id" ||
		included[2].Type != "betaGroups" || included[2].ID != "group-b" ||
		included[3].Type != "builds" || included[3].ID != "shared-id" {
		t.Fatalf("expected stable order with type-scoped identities, got %+v", included)
	}
}

func TestPaginateAll_MergesOverlappingIncludedAcrossPages(t *testing.T) {
	const totalPages = 3

	makePage := func(page int) *BuildsResponse {
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &BuildsResponse{
			Data: []Resource[BuildAttributes]{
				{Type: ResourceTypeBuilds, ID: fmt.Sprintf("build-%d", page)},
			},
			Included: json.RawMessage(fmt.Sprintf(`[
				{"type":"apps","id":"app-1","attributes":{"name":"App from page %d"}},
				{"type":"preReleaseVersions","id":"prv-%d","attributes":{"version":"1.0.%d"}}
			]`, page, page, page)),
			Links: links,
		}
	}

	result, err := PaginateAll(context.Background(), makePage(1), func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected *BuildsResponse, got %T", result)
	}
	if len(builds.Data) != totalPages {
		t.Fatalf("data length = %d, want %d", len(builds.Data), totalPages)
	}
	var included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(builds.Included, &included); err != nil {
		t.Fatalf("decode merged included payload: %v", err)
	}

	wantIdentities := []string{"apps/app-1", "preReleaseVersions/prv-1", "preReleaseVersions/prv-2", "preReleaseVersions/prv-3"}
	gotIdentities := make([]string, 0, len(included))
	for _, resource := range included {
		gotIdentities = append(gotIdentities, resource.Type+"/"+resource.ID)
	}
	if !slices.Equal(gotIdentities, wantIdentities) {
		t.Fatalf("included identities = %v, want %v", gotIdentities, wantIdentities)
	}
	if included[0].Attributes.Name != "App from page 1" {
		t.Fatalf("expected first page's app representation to win, got %q", included[0].Attributes.Name)
	}
}

func TestPaginateAll_SinglePageIncludedRetainedVerbatim(t *testing.T) {
	payload := json.RawMessage("[ { \"type\": \"apps\", \"id\": \"app-1\" } ]")
	firstPage := &BuildsResponse{
		Data:     []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-1"}},
		Included: payload,
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		t.Fatalf("unexpected fetch of %q for a single page", nextURL)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}
	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected *BuildsResponse, got %T", result)
	}
	if string(builds.Included) != string(payload) {
		t.Fatalf("included = %q, want byte-preserved %q", builds.Included, payload)
	}
}

func TestPaginateAll_PaginationErrorPreservesPartialIncluded(t *testing.T) {
	firstPage := &BuildsResponse{
		Data:     []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-1"}},
		Included: json.RawMessage(`[{"type":"apps","id":"app-1"}]`),
		Links:    Links{Next: "page=2"},
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return nil, errors.New("page unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "page 2") {
		t.Fatalf("expected page 2 error, got %v", err)
	}
	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected partial *BuildsResponse, got %T", result)
	}
	if len(builds.Data) != 1 {
		t.Fatalf("partial data length = %d, want 1", len(builds.Data))
	}
	if got := string(builds.Included); got != string(firstPage.Included) {
		t.Fatalf("partial included = %q, want %q", got, firstPage.Included)
	}
}

func TestPaginateAll_PaginationErrorMergesPartialIncluded(t *testing.T) {
	firstPage := &BuildsResponse{
		Data:     []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-1"}},
		Included: json.RawMessage(`[{"type":"apps","id":"app-1","attributes":{"name":"first"}}]`),
		Links:    Links{Next: "page=2"},
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		switch nextURL {
		case "page=2":
			return &BuildsResponse{
				Data: []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-2"}},
				Included: json.RawMessage(`[
					{"type":"apps","id":"app-1","attributes":{"name":"later"}},
					{"type":"apps","id":"app-2"}
				]`),
				Links: Links{Next: "page=3"},
			}, nil
		case "page=3":
			return nil, errors.New("page unavailable")
		default:
			t.Fatalf("unexpected next URL %q", nextURL)
			return nil, nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "page 3") {
		t.Fatalf("expected page 3 error, got %v", err)
	}

	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected partial *BuildsResponse, got %T", result)
	}
	if len(builds.Data) != 2 {
		t.Fatalf("partial data length = %d, want 2", len(builds.Data))
	}
	var included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(builds.Included, &included); err != nil {
		t.Fatalf("decode partial included: %v", err)
	}
	if len(included) != 2 || included[0].ID != "app-1" || included[0].Attributes.Name != "first" || included[1].ID != "app-2" {
		t.Fatalf("partial included = %+v, want first app-1 representation and app-2", included)
	}
}

func TestPaginateAll_RepeatedURLPreservesPartialIncluded(t *testing.T) {
	firstPage := &BuildsResponse{
		Data:     []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-1"}},
		Included: json.RawMessage(`[{"type":"apps","id":"app-1"}]`),
		Links:    Links{Next: "page=2"},
	}

	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return &BuildsResponse{
			Data:     []Resource[BuildAttributes]{{Type: ResourceTypeBuilds, ID: "build-2"}},
			Included: json.RawMessage(`[{"type":"apps","id":"app-2"}]`),
			Links:    Links{Next: "page=2"},
		}, nil
	})
	if err == nil || !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("expected repeated URL error, got %v", err)
	}
	builds, ok := result.(*BuildsResponse)
	if !ok {
		t.Fatalf("expected partial *BuildsResponse, got %T", result)
	}
	if len(builds.Data) != 2 {
		t.Fatalf("partial data length = %d, want 2", len(builds.Data))
	}
	var included []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(builds.Included, &included); err != nil {
		t.Fatalf("decode partial included: %v", err)
	}
	if len(included) != 2 || included[0].ID != "app-1" || included[1].ID != "app-2" {
		t.Fatalf("partial included = %+v, want app-1 and app-2", included)
	}
}

var benchmarkIncludedSink int

func BenchmarkPaginateAllIncludedAggregation(b *testing.B) {
	const (
		pageCount        = 96
		resourcesPerPage = 24
	)
	payloads := make([]json.RawMessage, pageCount)
	for page := range pageCount {
		resources := make([]map[string]any, 0, resourcesPerPage+1)
		resources = append(resources, map[string]any{
			"type": "apps",
			"id":   "shared-app",
		})
		for resource := range resourcesPerPage {
			resources = append(resources, map[string]any{
				"type": "preReleaseVersions",
				"id":   fmt.Sprintf("page-%d-resource-%d", page, resource),
			})
		}
		encoded, err := json.Marshal(resources)
		if err != nil {
			b.Fatalf("marshal benchmark payload: %v", err)
		}
		payloads[page] = encoded
	}

	b.Run("legacy-remerge", func(b *testing.B) {
		for range b.N {
			var merged json.RawMessage
			for _, payload := range payloads {
				var err error
				merged, err = mergeRawJSONArrayBaseline(merged, payload)
				if err != nil {
					b.Fatal(err)
				}
			}
			benchmarkIncludedSink = len(merged)
		}
	})

	b.Run("accumulator", func(b *testing.B) {
		for range b.N {
			acc := &rawJSONArrayAccumulator{}
			for _, payload := range payloads {
				if err := acc.add(payload); err != nil {
					b.Fatal(err)
				}
			}
			merged, err := acc.merged()
			if err != nil {
				b.Fatal(err)
			}
			benchmarkIncludedSink = len(merged)
		}
	})
}

// mergeRawJSONArrayBaseline mirrors the pre-accumulator implementation for
// the benchmark comparison. It intentionally reparses and remarshals the
// growing aggregate on every page.
func mergeRawJSONArrayBaseline(dst, src json.RawMessage) (json.RawMessage, error) {
	switch {
	case len(src) == 0:
		return dst, nil
	case len(dst) == 0:
		return append(json.RawMessage(nil), src...), nil
	}

	var dstItems []json.RawMessage
	if err := json.Unmarshal(dst, &dstItems); err != nil {
		return nil, fmt.Errorf("parse existing array: %w", err)
	}
	var srcItems []json.RawMessage
	if err := json.Unmarshal(src, &srcItems); err != nil {
		return nil, fmt.Errorf("parse incoming array: %w", err)
	}

	merged := make([]json.RawMessage, 0, len(dstItems)+len(srcItems))
	seen := make(map[string]struct{}, len(dstItems)+len(srcItems))
	appendUnique := func(items []json.RawMessage) {
		for _, item := range items {
			key := rawJSONArrayItemKey(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
	}
	appendUnique(dstItems)
	appendUnique(srcItems)

	result, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal merged array: %w", err)
	}
	return result, nil
}

func TestPaginateAll_GameCenterEnabledVersions(t *testing.T) {
	const totalPages = 2
	const perPage = 3

	makePage := func(page int) *GameCenterEnabledVersionsResponse {
		data := make([]Resource[GameCenterEnabledVersionAttributes], 0, perPage)
		for i := range perPage {
			data = append(data, Resource[GameCenterEnabledVersionAttributes]{
				Type: ResourceTypeGameCenterEnabledVersions,
				ID:   fmt.Sprintf("gcev-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &GameCenterEnabledVersionsResponse{Data: data, Links: links}
	}

	firstPage := makePage(1)
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	versions, ok := result.(*GameCenterEnabledVersionsResponse)
	if !ok {
		t.Fatalf("expected *GameCenterEnabledVersionsResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(versions.Data) != expected {
		t.Fatalf("expected %d versions, got %d", expected, len(versions.Data))
	}
}

func TestPaginateAll_SubscriptionGroups(t *testing.T) {
	const totalPages = 2
	const perPage = 2

	makePage := func(page int) *SubscriptionGroupsResponse {
		data := make([]Resource[SubscriptionGroupAttributes], 0, perPage)
		for i := range perPage {
			data = append(data, Resource[SubscriptionGroupAttributes]{
				Type: ResourceTypeSubscriptionGroups,
				ID:   fmt.Sprintf("subgrp-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &SubscriptionGroupsResponse{Data: data, Links: links}
	}

	firstPage := makePage(1)
	result, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	groups, ok := result.(*SubscriptionGroupsResponse)
	if !ok {
		t.Fatalf("expected *SubscriptionGroupsResponse, got %T", result)
	}
	expected := totalPages * perPage
	if len(groups.Data) != expected {
		t.Fatalf("expected %d subscription groups, got %d", expected, len(groups.Data))
	}
}
