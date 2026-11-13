package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestAppsListStateFiltersSendExactQuery covers the app-state triage filters
// documented for GET /v1/apps: filter[appStoreVersions.appVersionState] and
// filter[reviewSubmissions.state].
func TestAppsListStateFiltersSendExactQuery(t *testing.T) {
	setupAuth(t)

	tests := []struct {
		name      string
		args      []string
		wantQuery map[string]string
	}{
		{
			name: "version state",
			args: []string{"apps", "list", "--version-state", "IN_REVIEW,WAITING_FOR_REVIEW", "--output", "json"},
			wantQuery: map[string]string{
				"filter[appStoreVersions.appVersionState]": "IN_REVIEW,WAITING_FOR_REVIEW",
			},
		},
		{
			name: "review submission state lowercase input",
			args: []string{"apps", "list", "--review-submission-state", "in_review", "--output", "json"},
			wantQuery: map[string]string{
				"filter[reviewSubmissions.state]": "IN_REVIEW",
			},
		},
		{
			name: "both filters on the apps group",
			args: []string{"apps", "--version-state", "IN_REVIEW", "--review-submission-state", "IN_REVIEW,UNRESOLVED_ISSUES", "--output", "json"},
			wantQuery: map[string]string{
				"filter[appStoreVersions.appVersionState]": "IN_REVIEW",
				"filter[reviewSubmissions.state]":          "IN_REVIEW,UNRESOLVED_ISSUES",
			},
		},
		{
			name: "combined with existing filters and sort",
			args: []string{"apps", "list", "--sku", "SKU1", "--sort", "sku", "--version-state", "PENDING_DEVELOPER_RELEASE", "--output", "json"},
			wantQuery: map[string]string{
				"filter[sku]": "SKU1",
				"sort":        "sku",
				"filter[appStoreVersions.appVersionState]": "PENDING_DEVELOPER_RELEASE",
			},
		},
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
					t.Fatalf("request = %s %s, want GET /v1/apps", req.Method, req.URL.Path)
				}
				query := req.URL.Query()
				for key, want := range test.wantQuery {
					got, ok := query[key]
					if !ok {
						t.Errorf("query is missing %s", key)
						continue
					}
					if len(got) != 1 || got[0] != want {
						t.Errorf("query %s = %q, want exactly [%q]", key, got, want)
					}
				}
				if len(query) != len(test.wantQuery) {
					t.Errorf("query = %v, want exactly %v", query, test.wantQuery)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}

// TestAppsListStateFiltersRejectUnknownValues verifies validation runs before
// authentication and reports the documented enum members.
func TestAppsListStateFiltersRejectUnknownValues(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown version state",
			args:    []string{"apps", "list", "--version-state", "READY_FOR_SALE"},
			wantErr: "--version-state must be one of: ACCEPTED, DEVELOPER_REJECTED, IN_REVIEW, INVALID_BINARY, METADATA_REJECTED, PENDING_APPLE_RELEASE, PENDING_DEVELOPER_RELEASE, PREPARE_FOR_SUBMISSION, PROCESSING_FOR_DISTRIBUTION, READY_FOR_DISTRIBUTION, READY_FOR_REVIEW, REJECTED, REPLACED_WITH_NEW_VERSION, WAITING_FOR_EXPORT_COMPLIANCE, WAITING_FOR_REVIEW",
		},
		{
			name:    "unknown version state among valid ones",
			args:    []string{"apps", "list", "--version-state", "IN_REVIEW,NOPE"},
			wantErr: "--version-state must be one of: ACCEPTED,",
		},
		{
			name:    "unknown review submission state",
			args:    []string{"apps", "list", "--review-submission-state", "PREPARE_FOR_SUBMISSION"},
			wantErr: "--review-submission-state must be one of: READY_FOR_REVIEW, WAITING_FOR_REVIEW, IN_REVIEW, UNRESOLVED_ISSUES, CANCELING, COMPLETING, COMPLETE",
		},
		{
			name:    "unknown review submission state on the apps group",
			args:    []string{"apps", "--review-submission-state", "NOPE"},
			wantErr: "--review-submission-state must be one of: READY_FOR_REVIEW,",
		},
		{
			name:    "explicit empty version state",
			args:    []string{"apps", "list", "--version-state", ""},
			wantErr: "--version-state must not be empty",
		},
		{
			name:    "whitespace review submission state on the apps group",
			args:    []string{"apps", "--review-submission-state", "   "},
			wantErr: "--review-submission-state must not be empty",
		},
		{
			name:    "leading empty version state",
			args:    []string{"apps", "list", "--version-state", ",IN_REVIEW"},
			wantErr: "--version-state must not contain empty values",
		},
		{
			name:    "repeated comma version state",
			args:    []string{"apps", "list", "--version-state", "IN_REVIEW,,WAITING_FOR_REVIEW"},
			wantErr: "--version-state must not contain empty values",
		},
		{
			name:    "trailing empty review submission state",
			args:    []string{"apps", "list", "--review-submission-state", "IN_REVIEW,"},
			wantErr: "--review-submission-state must not contain empty values",
		},
	}

	runValidationTests(t, tests)
}

// TestAppsListStateFiltersRejectNextCombination proves the filters are never
// accepted and silently dropped: a links.next URL already carries its own
// query, so combining it with a state filter is a usage error.
func TestAppsListStateFiltersRejectNextCombination(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	next := "https://api.appstoreconnect.apple.com/v1/apps?cursor=next"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "version state",
			args:    []string{"apps", "list", "--next", next, "--version-state", "IN_REVIEW"},
			wantErr: "--next cannot be combined with --version-state",
		},
		{
			name:    "review submission state",
			args:    []string{"apps", "list", "--next", next, "--review-submission-state", "IN_REVIEW"},
			wantErr: "--next cannot be combined with --review-submission-state",
		},
		{
			name:    "sort",
			args:    []string{"apps", "list", "--next", next, "--sort", "sku"},
			wantErr: "--next cannot be combined with --sort",
		},
		{
			name:    "explicit empty sku",
			args:    []string{"apps", "list", "--next", next, "--sku", ""},
			wantErr: "--next cannot be combined with --sku",
		},
		{
			name:    "explicit zero limit",
			args:    []string{"apps", "--next", next, "--limit", "0"},
			wantErr: "--next cannot be combined with --limit",
		},
	}

	runValidationTests(t, tests)
}
