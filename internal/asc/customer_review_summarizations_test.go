package asc

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetCustomerReviewSummarizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/customerReviewSummarizations" {
			t.Fatalf("expected path /v1/apps/app-1/customerReviewSummarizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if got := values.Get("filter[platform]"); got != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", got)
		}
		if got := values.Get("filter[territory]"); got != "US" {
			t.Fatalf("expected filter[territory]=US, got %q", got)
		}
		if got := values.Get("fields[customerReviewSummarizations]"); got != "locale,text" {
			t.Fatalf("expected fields[customerReviewSummarizations]=locale,text, got %q", got)
		}
		if got := values.Get("fields[territories]"); got != "currency" {
			t.Fatalf("expected fields[territories]=currency, got %q", got)
		}
		if got := values.Get("include"); got != "territory" {
			t.Fatalf("expected include=territory, got %q", got)
		}
		if got := values.Get("limit"); got != "25" {
			t.Fatalf("expected limit=25, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetCustomerReviewSummarizations(
		context.Background(), "app-1",
		WithCustomerReviewSummarizationsPlatforms([]string{"IOS"}),
		WithCustomerReviewSummarizationsTerritories([]string{"US"}),
		WithCustomerReviewSummarizationsFields([]string{"locale", "text"}),
		WithCustomerReviewSummarizationsTerritoryFields([]string{"currency"}),
		WithCustomerReviewSummarizationsInclude([]string{"territory"}),
		Limit[CustomerReviewSummarizationsOption](25),
	)
	if err != nil {
		t.Fatalf("GetCustomerReviewSummarizations() error: %v", err)
	}
}

func TestGetCustomerReviewSummarizations_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCustomerReviewSummarizations(context.Background(), "", NextURL[CustomerReviewSummarizationsOption](next)); err != nil {
		t.Fatalf("GetCustomerReviewSummarizations() error: %v", err)
	}
}

func TestGetCustomerReviewSummarizations_RejectsNextURLWithSelectorsOrQueryOptions(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=abc"
	tests := []struct {
		name  string
		appID string
		opts  []CustomerReviewSummarizationsOption
	}{
		{name: "app ID", appID: "app-1"},
		{name: "platform", opts: []CustomerReviewSummarizationsOption{WithCustomerReviewSummarizationsPlatforms([]string{"IOS"})}},
		{name: "territory", opts: []CustomerReviewSummarizationsOption{WithCustomerReviewSummarizationsTerritories([]string{"USA"})}},
		{name: "fields", opts: []CustomerReviewSummarizationsOption{WithCustomerReviewSummarizationsFields([]string{"text"})}},
		{name: "territory fields", opts: []CustomerReviewSummarizationsOption{WithCustomerReviewSummarizationsTerritoryFields([]string{"currency"})}},
		{name: "include", opts: []CustomerReviewSummarizationsOption{WithCustomerReviewSummarizationsInclude([]string{"territory"})}},
		{name: "limit", opts: []CustomerReviewSummarizationsOption{Limit[CustomerReviewSummarizationsOption](25)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newTestClient(t, func(*http.Request) { requests++ }, jsonResponse(http.StatusOK, `{"data":[]}`))
			opts := append([]CustomerReviewSummarizationsOption{NextURL[CustomerReviewSummarizationsOption](next)}, test.opts...)

			_, err := client.GetCustomerReviewSummarizations(context.Background(), test.appID, opts...)
			if err == nil || !strings.Contains(err.Error(), "next URL cannot be combined with") {
				t.Fatalf("error = %v, want next URL conflict", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
		})
	}
}
