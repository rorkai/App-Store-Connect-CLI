package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestStatusWatchJSONEmitsChangedSnapshots(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"WAITING_FOR_REVIEW",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"READY_FOR_SALE",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"status",
		"--app", "123456789",
		"--include", "appstore,submission,review",
		"--watch",
		"--poll-interval", "1ms",
		"--max-polls", "2",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON snapshots, got %d\nstdout=%s", len(lines), stdout)
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first snapshot: %v\n%s", err, lines[0])
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal second snapshot: %v\n%s", err, lines[1])
	}

	firstSummary := first["summary"].(map[string]any)
	if firstSummary["health"] != "yellow" {
		t.Fatalf("expected first snapshot health=yellow, got %v", firstSummary["health"])
	}
	if firstSummary["nextAction"] != "Wait for App Store review outcome." {
		t.Fatalf("expected first nextAction to wait for review, got %v", firstSummary["nextAction"])
	}

	secondSummary := second["summary"].(map[string]any)
	if secondSummary["health"] != "green" {
		t.Fatalf("expected second snapshot health=green, got %v", secondSummary["health"])
	}
	if secondSummary["nextAction"] != "No action needed." {
		t.Fatalf("expected second nextAction to be no action needed, got %v", secondSummary["nextAction"])
	}
}

func TestStatusWatchSubmissionAndReviewUseBoundedRecentPage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-complete",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-20T00:00:00Z"}
			}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/123456789/reviewSubmissions?cursor=page-2"}
	}`))

	stdout, stderr, err := runCommand(t, []string{
		"status",
		"--app", "123456789",
		"--include", "submission,review",
		"--watch",
		"--poll-interval", "1ms",
		"--max-polls", "1",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls := countStatusRequests(reg, "/v1/apps/123456789/reviewSubmissions"); reviewCalls != 1 {
		t.Fatalf("expected 1 review submissions request, got %d", reviewCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("unmarshal watch output: %v\nstdout=%s", err, stdout)
	}
	if _, ok := payload["submission"]; !ok {
		t.Fatalf("expected submission section, got %v", payload)
	}
	if _, ok := payload["review"]; !ok {
		t.Fatalf("expected review section, got %v", payload)
	}

	review := payload["review"].(map[string]any)
	if review["state"] != "COMPLETE" {
		t.Fatalf("expected review state COMPLETE from bounded recent page, got %v", review["state"])
	}
	if review["latestSubmissionId"] != "review-sub-complete" {
		t.Fatalf("expected latest review submission id review-sub-complete, got %v", review["latestSubmissionId"])
	}

	submission := payload["submission"].(map[string]any)
	if submission["inFlight"] != false {
		t.Fatalf("expected bounded recent page to report no in-flight submissions, got %v", submission["inFlight"])
	}
}

func TestStatusWatchContextEndDuringSnapshotFetchExitsCleanly(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{
			name: "cancellation",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
		},
		{
			name: "deadline",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, req.Context().Err()
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			ctx, cancel := test.ctx()
			defer cancel()

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"status",
					"--app", "123456789",
					"--watch",
					"--poll-interval", "1ms",
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(ctx); err != nil {
					t.Fatalf("expected clean exit on %s, got %v", test.name, err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestStatusWatchDeadlineWhileWaitingForNextPollExitsCleanly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"status",
			"--app", "123456789",
			"--include", "links",
			"--watch",
			"--poll-interval", "1h",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(ctx); err != nil {
			t.Fatalf("expected clean exit on deadline, got %v", err)
		}
	})

	if !strings.Contains(stdout, `"links"`) {
		t.Fatalf("expected first snapshot before deadline exit, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestStatusWatchRequestTimeoutDuringSnapshotFetchReturnsError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "1ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"status",
			"--app", "123456789",
			"--include", "app",
			"--watch",
			"--poll-interval", "1ms",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected request timeout error, got nil")
	}
	if !strings.Contains(runErr.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline exceeded error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on request timeout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
