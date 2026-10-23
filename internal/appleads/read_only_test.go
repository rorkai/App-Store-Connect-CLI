package appleads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeAllowsSelectorQueriesAndRefusesWrites(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	client, err := NewClient(Credentials{AccessToken: "ACCESS", AdAccountID: "123456"}, WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sent.Add(1)
			return jsonResponse(200, `{"result":[]}`), nil
		}),
	}))
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	find, ok := PlatformEndpointByCommandPath("campaigns", "find")
	if !ok {
		t.Fatal("missing campaigns find endpoint")
	}
	if _, err := client.Do(context.Background(), find, nil, nil, json.RawMessage(`{"pagination":{"limit":1}}`)); err != nil {
		t.Fatalf("campaigns find Do() error = %v, want nil (POST query is a read)", err)
	}
	reports, ok := PlatformEndpointByCommandPath("reports", "apps", "campaigns")
	if !ok {
		t.Fatal("missing reports apps campaigns endpoint")
	}
	if _, err := client.Do(context.Background(), reports, nil, nil, json.RawMessage(`{"startTime":"2026-01-01","endTime":"2026-01-02"}`)); err != nil {
		t.Fatalf("reports apps campaigns Do() error = %v, want nil (POST report is a read)", err)
	}
	if got := sent.Load(); got != 2 {
		t.Fatalf("server received %d requests, want 2", got)
	}

	create, ok := PlatformEndpointByCommandPath("campaigns", "create")
	if !ok {
		t.Fatal("missing campaigns create endpoint")
	}
	_, err = client.Do(context.Background(), create, nil, nil, json.RawMessage(`{"name":"x"}`))
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("campaigns create Do() error = %v, want readonly.ErrRefused", err)
	}
	if _, err := client.RequestForVersion(context.Background(), APIVersionPlatformV1, http.MethodPost, "v1/reports/apps/campaigns/query", nil, json.RawMessage(`{}`), ContextAdAccount); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("raw RequestForVersion(POST) error = %v, want readonly.ErrRefused (passthrough is method-based)", err)
	}
	if got := sent.Load(); got != 2 {
		t.Fatalf("server received %d requests after refused writes, want 2", got)
	}
}

func TestEndpointSpecReadOnlyRequestClassification(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want bool
	}{
		{"GET view", []string{"campaigns", "view"}, true},
		{"POST query", []string{"campaigns", "find"}, true},
		{"POST report", []string{"reports", "apps", "campaigns"}, true},
		// Apple serves geolocation resolution as a POST search.
		{"POST geo resolve", []string{"geo", "resolve"}, true},
		{"POST create", []string{"campaigns", "create"}, false},
		{"POST bulk create", []string{"targeting-keywords", "create-bulk"}, false},
		{"DELETE", []string{"campaigns", "delete"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec, ok := PlatformEndpointByCommandPath(test.path...)
			if !ok {
				t.Fatalf("missing endpoint %v", test.path)
			}
			if got := spec.ReadOnlyRequest(); got != test.want {
				t.Fatalf("%s %s ReadOnlyRequest() = %v, want %v", spec.Method, spec.Path, got, test.want)
			}
		})
	}
	platformApply := EndpointSpec{Method: "POST", Path: "v1/recommendations/daily-budgets/apply", Version: APIVersionPlatformV1}
	if platformApply.ReadOnlyRequest() {
		t.Fatal("platform /apply POST must not be classified as a read")
	}
}
