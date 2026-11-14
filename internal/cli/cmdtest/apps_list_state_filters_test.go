package cmdtest

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps", queryPairs(test.wantQuery)...), asctest.JSON(http.StatusOK, `{"data":[]}`))

			_, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if calls := len(reg.Requests()); calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}

// queryPairs turns an exact query map into asctest matcher arguments.
func queryPairs(query map[string]string) []string {
	pairs := make([]string, 0, len(query))
	for key, value := range query {
		pairs = append(pairs, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	return pairs
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
