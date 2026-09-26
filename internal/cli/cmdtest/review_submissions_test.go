package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	reviewcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
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
	restore := reviewcli.SetReviewSubmissionsClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-create", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "sub-partial") ||
		!strings.Contains(runErr.Error(), "asc submit cancel --id sub-partial --confirm") {
		t.Fatalf("run error = %v, want partial submission ID and cancellation guidance", runErr)
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
	restore := reviewcli.SetReviewSubmissionsClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-create", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
			wantErr: "--app or --global is required",
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
			wantErr: "--confirm is required to submit",
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
			wantErr: "unexpected positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
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
			wantErr: "at least one of --resolved, --removed, --clear-resolved, or --clear-removed is required",
		},
		{
			name:    "review items-remove missing id",
			args:    []string{"review", "items-remove", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "review items-remove missing confirm",
			args:    []string{"review", "items-remove", "--id", "ITEM_ID"},
			wantErr: "--confirm is required to remove",
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
			wantErr: "at least one of --resolved, --removed, --clear-resolved, or --clear-removed is required",
		},
		{
			name:    "nested review items remove missing confirm",
			args:    []string{"review", "items", "remove", "--id", "ITEM_ID"},
			wantErr: "--confirm is required to remove",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestReviewCommandItemsInvalidItemType(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"review", "items-add",
			"--submission", "SUBMISSION_ID",
			"--item-type", "nope",
			"--item-id", "ITEM_ID",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

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
