package reviews

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDeriveOutcome(t *testing.T) {
	tests := []struct {
		name            string
		submissionState string
		itemStates      []string
		want            string
	}{
		{
			name:            "all items approved",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED"},
			want:            "approved",
		},
		{
			name:            "any item rejected",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "REJECTED"},
			want:            "rejected",
		},
		{
			name:            "unresolved issues no rejected items",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"ACCEPTED"},
			want:            "rejected",
		},
		{
			name:            "rejected item takes priority over unresolved",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"REJECTED"},
			want:            "rejected",
		},
		{
			name:            "mixed non-rejected states falls through to submission state",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "ACCEPTED"},
			want:            "complete",
		},
		{
			name:            "no items uses submission state",
			submissionState: "WAITING_FOR_REVIEW",
			itemStates:      nil,
			want:            "waiting_for_review",
		},
		{
			name:            "in review state",
			submissionState: "IN_REVIEW",
			itemStates:      []string{"READY_FOR_REVIEW"},
			want:            "in_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveOutcome(tt.submissionState, tt.itemStates)
			if got != tt.want {
				t.Errorf("deriveOutcome(%q, %v) = %q, want %q", tt.submissionState, tt.itemStates, got, tt.want)
			}
		})
	}
}

func TestReviewHistoryCommand_MissingApp(t *testing.T) {
	cmd := ReviewHistoryCommand()
	if cmd.Name != "history" {
		t.Fatalf("unexpected command name: %s", cmd.Name)
	}

	// Unset any env that could provide app ID
	t.Setenv("ASC_APP_ID", "")

	err := cmd.ParseAndRun(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for missing --app, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got: %v", err)
	}
}

func TestReviewHistoryCommand_InvalidLimit(t *testing.T) {
	cmd := ReviewHistoryCommand()
	t.Setenv("ASC_APP_ID", "test-app")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	err := cmd.ParseAndRun(context.Background(), []string{"--limit", "999"})
	if err == nil {
		t.Fatal("expected error for invalid limit, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid --limit, got: %v", err)
	}
}

func TestReviewHistoryCommand_InvalidPlatform(t *testing.T) {
	cmd := ReviewHistoryCommand()
	t.Setenv("ASC_APP_ID", "test-app")

	err := cmd.ParseAndRun(context.Background(), []string{"--platform", "NOT_A_REAL_PLATFORM"})
	if err == nil {
		t.Fatal("expected usage error for invalid --platform, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid --platform, got: %v", err)
	}
}

func TestReviewHistoryCommand_InvalidState(t *testing.T) {
	cmd := ReviewHistoryCommand()
	t.Setenv("ASC_APP_ID", "test-app")

	err := cmd.ParseAndRun(context.Background(), []string{"--state", "NOT_A_REAL_STATE"})
	if err == nil {
		t.Fatal("expected usage error for invalid --state, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid --state, got: %v", err)
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestHistoryClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("write key error: %v", err)
	}

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient error: %v", err)
	}
	return client
}

func makeSubmissions(entries ...struct {
	id, platform, state, date string
},
) []asc.ReviewSubmissionResource {
	var subs []asc.ReviewSubmissionResource
	for _, e := range entries {
		subs = append(subs, asc.ReviewSubmissionResource{
			ID: e.id,
			Attributes: asc.ReviewSubmissionAttributes{
				Platform:        asc.Platform(e.platform),
				SubmissionState: asc.ReviewSubmissionState(e.state),
				SubmittedDate:   e.date,
			},
		})
	}
	return subs
}

func makeSubmissionVersionContexts(entries ...struct {
	id, version, platform string
},
) map[string]submissionVersionContext {
	contexts := make(map[string]submissionVersionContext, len(entries))
	for _, e := range entries {
		contexts[e.id] = submissionVersionContext{
			VersionString: e.version,
			Platform:      e.platform,
		}
	}
	return contexts
}

func TestEnrichSubmissions_HappyPath(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-2",
					"attributes": {"state": "REJECTED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "TV_OS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "TV_OS", "UNRESOLVED_ISSUES", "2026-02-15T10:00:00Z"},
	)

	client := newTestHistoryClient(t, transport)
	versionContexts := makeSubmissionVersionContexts(
		struct{ id, version, platform string }{"sub-1", "3.1.1", "TV_OS"},
		struct{ id, version, platform string }{"sub-2", "3.0.0", "TV_OS"},
	)
	entries, err := enrichSubmissions(context.Background(), client, subs, versionContexts, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Sorted by submittedDate descending
	if entries[0].VersionString != "3.1.1" {
		t.Errorf("first entry version = %q, want %q", entries[0].VersionString, "3.1.1")
	}
	if entries[0].Outcome != "approved" {
		t.Errorf("first entry outcome = %q, want %q", entries[0].Outcome, "approved")
	}
	if entries[1].VersionString != "3.0.0" {
		t.Errorf("second entry version = %q, want %q", entries[1].VersionString, "3.0.0")
	}
	if entries[1].Outcome != "rejected" {
		t.Errorf("second entry outcome = %q, want %q", entries[1].Outcome, "rejected")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("first entry items count = %d, want 1", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_EmptySubmissions(t *testing.T) {
	client := newTestHistoryClient(t, testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no API calls expected for empty submissions")
		return nil, nil
	}))
	entries, err := enrichSubmissions(context.Background(), client, nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestSubmissionVersionContexts_FromIncluded(t *testing.T) {
	resp := &asc.ReviewSubmissionsResponse{
		Data: []asc.ReviewSubmissionResource{
			{
				ID: "sub-1",
				Relationships: &asc.ReviewSubmissionRelationships{
					AppStoreVersionForReview: &asc.Relationship{
						Data: asc.ResourceData{
							Type: asc.ResourceTypeAppStoreVersions,
							ID:   "ver-1",
						},
					},
				},
			},
		},
		Included: json.RawMessage(`[
			{
				"type": "appStoreVersions",
				"id": "ver-1",
				"attributes": {
					"versionString": "2.0.0",
					"platform": "IOS"
				}
			}
		]`),
	}

	got, err := submissionVersionContexts(resp)
	if err != nil {
		t.Fatalf("submissionVersionContexts() error: %v", err)
	}
	ctx, ok := got["sub-1"]
	if !ok {
		t.Fatal("expected version context for submission sub-1")
	}
	if ctx.VersionString != "2.0.0" {
		t.Fatalf("version = %q, want %q", ctx.VersionString, "2.0.0")
	}
	if ctx.Platform != "IOS" {
		t.Fatalf("platform = %q, want %q", ctx.Platform, "IOS")
	}
}

func TestFetchReviewSubmissions_PaginateDetectsRepeatedNextURL(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
		if req.URL.Query().Get("cursor") == "same" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "sub-2",
					"attributes": {
						"state": "COMPLETE",
						"platform": "IOS",
						"submittedDate": "2026-03-01T13:00:00Z"
					}
				}],
				"links": {
					"self": "/v1/apps/app-1/reviewSubmissions?cursor=same",
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=same"
				}
			}`), nil
		}
		return testJSONResponse(200, `{
			"data": [{
				"type": "reviewSubmissions",
				"id": "sub-1",
				"attributes": {
					"state": "COMPLETE",
					"platform": "IOS",
					"submittedDate": "2026-03-01T12:00:00Z"
				}
			}],
			"links": {
				"self": "/v1/apps/app-1/reviewSubmissions?limit=200",
				"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=same"
			}
		}`), nil
	})

	client := newTestHistoryClient(t, transport)
	_, _, err := fetchReviewSubmissions(
		context.Background(),
		client,
		"app-1",
		[]asc.ReviewSubmissionsOption{
			asc.WithReviewSubmissionsLimit(200),
			asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		},
		true,
	)
	if err == nil {
		t.Fatal("expected repeated pagination URL error, got nil")
	}
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got: %v", err)
	}
}

func TestFetchReviewSubmissions_PaginateAnnotatesIncludedParseErrorWithPage(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
		if req.URL.Query().Get("cursor") == "2" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "sub-2",
					"attributes": {
						"state": "COMPLETE",
						"platform": "IOS",
						"submittedDate": "2026-03-01T13:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "ver-2"}
						}
					}
				}],
				"included": {"type": "appStoreVersions", "id": "ver-2"},
				"links": {"self": "/v1/apps/app-1/reviewSubmissions?cursor=2"}
			}`), nil
		}
		return testJSONResponse(200, `{
			"data": [{
				"type": "reviewSubmissions",
				"id": "sub-1",
				"attributes": {
					"state": "COMPLETE",
					"platform": "IOS",
					"submittedDate": "2026-03-01T12:00:00Z"
				}
			}],
			"links": {
				"self": "/v1/apps/app-1/reviewSubmissions?limit=200",
				"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=2"
			}
		}`), nil
	})

	client := newTestHistoryClient(t, transport)
	_, _, err := fetchReviewSubmissions(
		context.Background(),
		client,
		"app-1",
		[]asc.ReviewSubmissionsOption{
			asc.WithReviewSubmissionsLimit(200),
			asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		},
		true,
	)
	if err == nil {
		t.Fatal("expected included parse error, got nil")
	}
	if !strings.Contains(err.Error(), "page 2:") {
		t.Fatalf("expected page context in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to parse included review submission versions") {
		t.Fatalf("expected included parsing error, got: %v", err)
	}
}

func TestEnrichSubmissions_VersionFilter(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-1", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-2", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "IOS", "COMPLETE", "2026-02-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	versionContexts := makeSubmissionVersionContexts(
		struct{ id, version, platform string }{"sub-1", "2.0.0", "IOS"},
		struct{ id, version, platform string }{"sub-2", "1.0.0", "IOS"},
	)
	entries, err := enrichSubmissions(context.Background(), client, subs, versionContexts, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after version filter, got %d", len(entries))
	}
	if entries[0].VersionString != "2.0.0" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "2.0.0")
	}
}

func TestEnrichSubmissions_NoItems(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if entries[0].Outcome != "complete" {
		t.Errorf("outcome = %q, want %q", entries[0].Outcome, "complete")
	}
	if len(entries[0].Items) != 0 {
		t.Errorf("items count = %d, want 0", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_ItemWithoutVersionRelationship(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("items count = %d, want 1", len(entries[0].Items))
	}
	if entries[0].Items[0].ResourceID != "" {
		t.Errorf("item resourceId = %q, want empty", entries[0].Items[0].ResourceID)
	}
}

func TestPrintHistoryTable_NoError(t *testing.T) {
	entries := []SubmissionHistoryEntry{
		{
			SubmissionID:  "sub-1",
			VersionString: "3.1.1",
			Platform:      "TV_OS",
			State:         "COMPLETE",
			SubmittedDate: "2026-03-01T12:00:00Z",
			Outcome:       "approved",
			Items:         []SubmissionHistoryItem{{ID: "i1", State: "APPROVED", Type: "appStoreVersion", ResourceID: "v1"}},
		},
	}
	err := printHistoryTable(entries)
	if err != nil {
		t.Fatalf("printHistoryTable error: %v", err)
	}
}

func TestEnrichSubmissions_PaginatesItemsBeforeOutcome(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
		if req.URL.Query().Get("cursor") == "next-page" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-2",
					"attributes": {"state": "REJECTED"}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items?cursor=next-page"}
			}`), nil
		}
		return testJSONResponse(200, `{
			"data": [{
				"type": "reviewSubmissionItems",
				"id": "item-1",
				"attributes": {"state": "APPROVED"}
			}],
			"links": {
				"self": "/v1/reviewSubmissions/sub-1/items?limit=200",
				"next": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items?cursor=next-page"
			}
		}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Outcome != "rejected" {
		t.Fatalf("outcome = %q, want %q", entries[0].Outcome, "rejected")
	}
	if len(entries[0].Items) != 2 {
		t.Fatalf("items count = %d, want 2", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_RequestsAndPopulatesItemRelationships(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
		if got := req.URL.Query().Get("include"); !strings.Contains(got, "appCustomProductPageVersion") || !strings.Contains(got, "backgroundAssetVersion") {
			t.Fatalf("expected include to request non-conflicting item relationships, got %q", got)
		}
		if got := req.URL.Query().Get("include"); strings.Contains(got, "appStoreVersionExperimentV2") {
			t.Fatalf("expected include to avoid appStoreVersionExperimentV2 conflict, got %q", got)
		}
		if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); !strings.Contains(got, "appCustomProductPageVersion") || !strings.Contains(got, "backgroundAssetVersion") {
			t.Fatalf("expected fields[reviewSubmissionItems] to request relationship fields, got %q", got)
		}
		return testJSONResponse(200, `{
			"data": [
				{
					"type": "reviewSubmissionItems",
					"id": "item-cpp",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appCustomProductPageVersion": {
							"data": {"type": "appCustomProductPageVersions", "id": "cppv-1"}
						}
					}
				},
				{
					"type": "reviewSubmissionItems",
					"id": "item-bg",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"backgroundAssetVersion": {
							"data": {"type": "backgroundAssetVersions", "id": "bgv-1"}
						}
					}
				}
			],
			"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
		}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if len(entries[0].Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(entries[0].Items))
	}
	if entries[0].Items[0].Type != "appCustomProductPageVersion" || entries[0].Items[0].ResourceID != "cppv-1" {
		t.Fatalf("first item relationship = (%q, %q), want (%q, %q)", entries[0].Items[0].Type, entries[0].Items[0].ResourceID, "appCustomProductPageVersion", "cppv-1")
	}
	if entries[0].Items[1].Type != "backgroundAssetVersion" || entries[0].Items[1].ResourceID != "bgv-1" {
		t.Fatalf("second item relationship = (%q, %q), want (%q, %q)", entries[0].Items[1].Type, entries[0].Items[1].ResourceID, "backgroundAssetVersion", "bgv-1")
	}
}

func TestFormatItemsSummary(t *testing.T) {
	tests := []struct {
		name  string
		items []SubmissionHistoryItem
		want  string
	}{
		{"no items", nil, "0 items"},
		{"single approved", []SubmissionHistoryItem{{State: "APPROVED"}}, "1 approved"},
		{"mixed", []SubmissionHistoryItem{{State: "APPROVED"}, {State: "REJECTED"}}, "1 approved, 1 rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatItemsSummary(tt.items)
			if got != tt.want {
				t.Errorf("formatItemsSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPopulateSubmissionHistoryItem_SupportsAdditionalRelationshipTypes(t *testing.T) {
	tests := []struct {
		name     string
		item     asc.ReviewSubmissionItemResource
		wantType string
		wantID   string
	}{
		{
			name: "app custom product page version",
			item: asc.ReviewSubmissionItemResource{
				Relationships: &asc.ReviewSubmissionItemRelationships{
					AppCustomProductPageVersion: &asc.Relationship{
						Data: asc.ResourceData{ID: "cppv-1"},
					},
				},
			},
			wantType: "appCustomProductPageVersion",
			wantID:   "cppv-1",
		},
		{
			name: "app store version experiment v2",
			item: asc.ReviewSubmissionItemResource{
				Relationships: &asc.ReviewSubmissionItemRelationships{
					AppStoreVersionExperimentV2: &asc.Relationship{
						Data: asc.ResourceData{ID: "exp-v2-1"},
					},
				},
			},
			wantType: "appStoreVersionExperimentV2",
			wantID:   "exp-v2-1",
		},
		{
			name: "leaderboard version",
			item: asc.ReviewSubmissionItemResource{
				Relationships: &asc.ReviewSubmissionItemRelationships{
					GameCenterLeaderboardVersion: &asc.Relationship{
						Data: asc.ResourceData{ID: "gclv-1"},
					},
				},
			},
			wantType: "gameCenterLeaderboardVersion",
			wantID:   "gclv-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histItem := SubmissionHistoryItem{}
			populateSubmissionHistoryItem(&histItem, tt.item)
			if histItem.Type != tt.wantType || histItem.ResourceID != tt.wantID {
				t.Fatalf("populateSubmissionHistoryItem() = (%q, %q), want (%q, %q)", histItem.Type, histItem.ResourceID, tt.wantType, tt.wantID)
			}
		})
	}
}

func TestEnrichSubmissions_SortsByParsedTimestamp(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		return testJSONResponse(200, `{
			"data": [],
			"links": {"self": "/v1/reviewSubmissions/items"}
		}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-02-20T01:00:00+01:00"},
		struct{ id, platform, state, date string }{"sub-2", "IOS", "COMPLETE", "2026-02-20T00:30:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].SubmissionID != "sub-2" {
		t.Fatalf("first submission = %q, want %q", entries[0].SubmissionID, "sub-2")
	}
}

func TestEnrichSubmissions_SkipsEmptySubmittedDate(t *testing.T) {
	calls := 0
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-draft", "IOS", "READY_FOR_REVIEW", ""},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (draft skipped), got %d", len(entries))
	}
	if calls != 0 {
		t.Errorf("expected 0 API calls for draft submissions, got %d", calls)
	}
}

func TestEnrichSubmissions_EmptyResultsMarshalAsArray(t *testing.T) {
	client := newTestHistoryClient(t, testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no API calls expected for draft-only results")
		return nil, nil
	}))
	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-draft", "IOS", "READY_FOR_REVIEW", ""},
	)

	entries, err := enrichSubmissions(context.Background(), client, subs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries == nil {
		t.Fatal("expected non-nil empty slice")
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != "[]" {
		t.Fatalf("json = %s, want []", data)
	}
}
