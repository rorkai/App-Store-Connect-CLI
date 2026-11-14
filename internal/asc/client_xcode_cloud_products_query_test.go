package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func TestBuildCiProductsQueryIncludesOpenAPIOptions(t *testing.T) {
	query := &ciProductsQuery{}
	for _, option := range []CiProductsOption{
		WithCiProductsProductTypes([]string{" app ", "framework"}),
		WithCiProductsAppID("app-1"),
		WithCiProductsFields([]string{"name", "productType"}),
		WithCiProductsAppFields([]string{"name", "bundleId"}),
		WithCiProductsBundleIDFields([]string{"identifier", "platform"}),
		WithCiProductsScmRepositoryFields([]string{"repositoryName", "ownerName"}),
		WithCiProductsInclude([]string{"app", "bundleId", "primaryRepositories"}),
		WithCiProductsPrimaryRepositoriesLimit(25),
		Limit[CiProductsOption](10),
	} {
		option(query)
	}

	values, err := url.ParseQuery(buildCiProductsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	want := map[string]string{
		"filter[productType]":        "APP,FRAMEWORK",
		"filter[app]":                "app-1",
		"fields[ciProducts]":         "name,productType,app,bundleId,primaryRepositories",
		"fields[apps]":               "name,bundleId",
		"fields[bundleIds]":          "identifier,platform",
		"fields[scmRepositories]":    "repositoryName,ownerName",
		"include":                    "app,bundleId,primaryRepositories",
		"limit[primaryRepositories]": "25",
		"limit":                      "10",
	}
	for key, expected := range want {
		if got := values.Get(key); got != expected {
			t.Errorf("query[%q] = %q, want %q", key, got, expected)
		}
	}
}

func TestGetCiProductsWithOpenAPIQueryOptions(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciProducts","id":"product-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts" {
			t.Fatalf("expected path /v1/ciProducts, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		want := map[string]string{
			"filter[productType]":        "APP,FRAMEWORK",
			"filter[app]":                "app-1",
			"fields[ciProducts]":         "name,productType,app,bundleId,primaryRepositories",
			"fields[apps]":               "name,bundleId",
			"fields[bundleIds]":          "identifier,platform",
			"fields[scmRepositories]":    "repositoryName,ownerName",
			"include":                    "app,bundleId,primaryRepositories",
			"limit[primaryRepositories]": "25",
			"limit":                      "10",
		}
		for key, expected := range want {
			if got := values.Get(key); got != expected {
				t.Errorf("query[%q] = %q, want %q", key, got, expected)
			}
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProducts(
		context.Background(),
		WithCiProductsProductTypes([]string{"APP", "FRAMEWORK"}),
		WithCiProductsAppID("app-1"),
		WithCiProductsFields([]string{"name", "productType"}),
		WithCiProductsAppFields([]string{"name", "bundleId"}),
		WithCiProductsBundleIDFields([]string{"identifier", "platform"}),
		WithCiProductsScmRepositoryFields([]string{"repositoryName", "ownerName"}),
		WithCiProductsInclude([]string{"app", "bundleId", "primaryRepositories"}),
		WithCiProductsPrimaryRepositoriesLimit(25),
		Limit[CiProductsOption](10),
	); err != nil {
		t.Fatalf("GetCiProducts() error: %v", err)
	}
}

func TestBuildCiProductsQueryDeduplicatesIncludedRelationshipFields(t *testing.T) {
	query := &ciProductsQuery{}
	WithCiProductsFields([]string{"name", "app", "name"})(query)
	WithCiProductsInclude([]string{"app", "bundleId", "app"})(query)

	values, err := url.ParseQuery(buildCiProductsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if got := values.Get("fields[ciProducts]"); got != "name,app,bundleId" {
		t.Fatalf("fields[ciProducts] = %q, want name,app,bundleId", got)
	}
}

func TestGetCiProductsPreservesIncludedResources(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{
		"data": [{"type":"ciProducts","id":"product-1"}],
		"included": [{"type":"apps","id":"app-1","attributes":{"name":"Example"}}],
		"meta": {"paging":{"total":1}}
	}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/ciProducts" {
			t.Fatalf("path = %q, want /v1/ciProducts", req.URL.Path)
		}
	}, response)

	got, err := client.GetCiProducts(context.Background(), WithCiProductsInclude([]string{"app"}))
	if err != nil {
		t.Fatalf("GetCiProducts() error: %v", err)
	}

	var included []struct {
		Type ResourceType `json:"type"`
		ID   string       `json:"id"`
	}
	if err := json.Unmarshal(got.Included, &included); err != nil {
		t.Fatalf("decode included: %v (raw=%s)", err, got.Included)
	}
	if len(included) != 1 || included[0].Type != ResourceTypeApps || included[0].ID != "app-1" {
		t.Fatalf("included = %+v, want apps/app-1", included)
	}
	if string(got.Meta) != `{"paging":{"total":1}}` {
		t.Fatalf("meta = %s, want paging total", got.Meta)
	}
}

func TestGetCiProductsPreservesOmittedAttributesForRelationshipOnlyFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{
		"data": [{
			"type": "ciProducts",
			"id": "product-1",
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}],
		"links": {"next": ""}
	}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/ciProducts" {
			t.Fatalf("path = %q, want /v1/ciProducts", req.URL.Path)
		}
	}, response)

	got, err := client.GetCiProducts(context.Background(), WithCiProductsFields([]string{"app"}))
	if err != nil {
		t.Fatalf("GetCiProducts() error: %v", err)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var payload struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, present := payload.Data[0]["attributes"]; present {
		t.Fatalf("encoded relationship-only resource synthesized attributes: %s", encoded)
	}
}

func TestPaginateCiProductsPreservesIncludedResourcesAcrossPages(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2"
	requestCount := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requestCount++
			if requestCount == 1 && req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("first path = %q, want /v1/ciProducts", req.URL.Path)
			}
			if requestCount == 2 && req.URL.String() != nextURL {
				t.Fatalf("continuation URL = %q, want %q", req.URL.String(), nextURL)
			}
		},
		jsonResponse(http.StatusOK, `{
			"data": [{"type":"ciProducts","id":"product-1"}],
			"included": [{"type":"apps","id":"app-1","attributes":{"name":"Example"}}],
			"links": {"next":"`+nextURL+`"}
		}`),
		jsonResponse(http.StatusOK, `{
			"data": [{"type":"ciProducts","id":"product-2"}],
			"included": [
				{"type":"apps","id":"app-1","attributes":{"name":"Example"}},
				{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}
			],
			"links": {"next":""}
		}`),
	)

	firstPage, err := client.GetCiProducts(context.Background())
	if err != nil {
		t.Fatalf("GetCiProducts() error: %v", err)
	}
	aggregated, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, next string) (PaginatedResponse, error) {
		return client.GetCiProducts(ctx, NextURL[CiProductsOption](next))
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	got, ok := aggregated.(*CiProductsResponse)
	if !ok {
		t.Fatalf("aggregated response = %T, want *CiProductsResponse", aggregated)
	}
	if len(got.Data) != 2 {
		t.Fatalf("data length = %d, want 2", len(got.Data))
	}

	var included []struct {
		Type ResourceType `json:"type"`
		ID   string       `json:"id"`
	}
	if err := json.Unmarshal(got.Included, &included); err != nil {
		t.Fatalf("decode included: %v (raw=%s)", err, got.Included)
	}
	if len(included) != 2 {
		t.Fatalf("included length = %d, want 2: %+v", len(included), included)
	}
	if included[0].Type != ResourceTypeApps || included[0].ID != "app-1" {
		t.Fatalf("first included = %+v, want apps/app-1", included[0])
	}
	if included[1].Type != ResourceTypeBundleIds || included[1].ID != "bundle-1" {
		t.Fatalf("second included = %+v, want bundleIds/bundle-1", included[1])
	}
}
