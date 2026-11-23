package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestReviewSubmissionsCreateReportsPartialCreateID(t *testing.T) {
	requests := 0
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("unexpected request %d: %s %s", requests, req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-partial"},"errors":[]}`)
	}))
	stdout, stderr, runErr := runRootWithDeps(t, func() (*asc.Client, error) {
		return client, nil
	}, "review", "submissions-create", "--app", "app-1")

	// The partial submission is an empty READY_FOR_REVIEW draft, which App
	// Store Connect refuses to cancel, so the guidance must finish it instead.
	if runErr == nil || !strings.Contains(runErr.Error(), "sub-partial") ||
		!strings.Contains(runErr.Error(), "asc review items add --submission sub-partial") ||
		!strings.Contains(runErr.Error(), "asc review submissions-submit --id sub-partial --confirm") ||
		strings.Contains(runErr.Error(), "submit cancel") {
		t.Fatalf("run error = %v, want partial submission ID and draft guidance without a cancel", runErr)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one create request", requests)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want no command output", stdout, stderr)
	}
}

func TestReviewSubmissionsCreateCleanResponseStillSucceeds(t *testing.T) {
	requests := 0
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("unexpected request %d: %s %s", requests, req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-clean"}}`)
	}))
	stdout, stderr, runErr := runRootWithDeps(t, func() (*asc.Client, error) {
		return client, nil
	}, "review", "submissions-create", "--app", "app-1")
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	if requests != 1 || !strings.Contains(stdout, `"id":"sub-clean"`) || stderr != "" {
		t.Fatalf("requests = %d, stdout = %q, stderr = %q, want successful create output", requests, stdout, stderr)
	}
}

func TestReviewCommandSubmissionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "review submissions-list missing app or global",
			args:    []string{"review", "submissions-list"},
			wantErr: "Error: --app or --global is required",
		},
		{
			name:    "review submissions-list global missing app",
			args:    []string{"review", "submissions-list", "--global"},
			wantErr: "Error: --app is required with --global",
		},
		{
			name:    "review submissions-get missing id",
			args:    []string{"review", "submissions-get"},
			wantErr: "--id is required",
		},
		{
			name:    "review submissions-create missing app",
			args:    []string{"review", "submissions-create"},
			wantErr: "--app is required",
		},
		{
			name:    "review submissions-create invalid platform",
			args:    []string{"review", "submissions-create", "--app", "app-1", "--platform", "NOPE"},
			wantErr: "--platform must be one of",
		},
		{
			name:    "review submissions-submit missing id",
			args:    []string{"review", "submissions-submit", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "review submissions-submit missing confirm",
			args:    []string{"review", "submissions-submit", "--id", "SUBMISSION_123"},
			wantErr: "--confirm is required\n",
		},
		{
			name:    "review submissions-update missing id",
			args:    []string{"review", "submissions-update", "--canceled=true"},
			wantErr: "--id is required",
		},
		{
			name:    "review submissions-update missing canceled",
			args:    []string{"review", "submissions-update", "--id", "SUBMISSION_123"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "review submissions-items-ids missing id",
			args:    []string{"review", "submissions-items-ids"},
			wantErr: "--id is required",
		},
		{
			name:    "review submissions-items-ids invalid limit",
			args:    []string{"review", "submissions-items-ids", "--id", "sub-1", "--limit", "201"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "review history rejects positional arguments",
			args:    []string{"review", "history", "unexpected", "--app", "app-1"},
			wantErr: `unexpected argument "unexpected"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestReviewCommandItemsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "review items-list missing submission",
			args:    []string{"review", "items-list"},
			wantErr: "--submission is required",
		},
		{
			name:    "review items-add missing submission",
			args:    []string{"review", "items-add", "--item-type", "appStoreVersions", "--item-id", "VERSION_ID"},
			wantErr: "--submission is required",
		},
		{
			name:    "review items-add missing item-type",
			args:    []string{"review", "items-add", "--submission", "SUBMISSION_ID", "--item-id", "VERSION_ID"},
			wantErr: "--item-type is required",
		},
		{
			name:    "review items-add missing item-id",
			args:    []string{"review", "items-add", "--submission", "SUBMISSION_ID", "--item-type", "appStoreVersions"},
			wantErr: "--item-id is required",
		},
		{
			name:    "review items-update missing id",
			args:    []string{"review", "items-update", "--resolved", "true"},
			wantErr: "--id is required",
		},
		{
			name:    "review items-update missing update",
			args:    []string{"review", "items-update", "--id", "ITEM_ID"},
			wantErr: "at least one update flag is required (--resolved, --removed, --clear-resolved, --clear-removed)",
		},
		{
			name:    "review items-remove missing id",
			args:    []string{"review", "items-remove", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "review items-remove missing confirm",
			args:    []string{"review", "items-remove", "--id", "ITEM_ID"},
			wantErr: "--confirm is required\n",
		},
		{
			name:    "nested review items list missing submission",
			args:    []string{"review", "items", "list"},
			wantErr: "--submission is required",
		},
		{
			name:    "nested review items add missing item-id",
			args:    []string{"review", "items", "add", "--submission", "SUBMISSION_ID", "--item-type", "appStoreVersions"},
			wantErr: "--item-id is required",
		},
		{
			name:    "nested review items update missing update",
			args:    []string{"review", "items", "update", "--id", "ITEM_ID"},
			wantErr: "at least one update flag is required (--resolved, --removed, --clear-resolved, --clear-removed)",
		},
		{
			name:    "nested review items remove missing confirm",
			args:    []string{"review", "items", "remove", "--id", "ITEM_ID"},
			wantErr: "--confirm is required\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestReviewCommandItemsInvalidItemType(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr, code := runWithDeps(t, nil,
		"review", "items-add",
		"--submission", "SUBMISSION_ID",
		"--item-type", "nope",
		"--item-id", "ITEM_ID",
	)
	if code != cmd.ExitUsage {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--item-type must be one of:") {
		t.Fatalf("expected invalid item type error, got %q", stderr)
	}
	wantSupportedTypes := []string{
		"backgroundAssetVersions",
		"inAppPurchaseVersions",
		"gameCenterAchievementVersions",
		"gameCenterActivityVersions",
		"gameCenterChallengeVersions",
		"gameCenterLeaderboardSetVersions",
		"gameCenterLeaderboardVersions",
	}
	for _, supportedType := range wantSupportedTypes {
		if !strings.Contains(stderr, supportedType) {
			t.Fatalf("expected stderr to list %s, got %q", supportedType, stderr)
		}
	}
	if strings.Contains(stderr, "gameCenterLeaderboardReleases") {
		t.Fatalf("did not expect undocumented leaderboard release type in stderr, got %q", stderr)
	}
}
