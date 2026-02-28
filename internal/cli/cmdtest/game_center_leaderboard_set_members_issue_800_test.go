package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestIssue800_LeaderboardSetMembersSetFallsBackToAddWhenSetIsEmpty(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.String() != "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards" {
				t.Fatalf("unexpected PATCH URL: %s", req.URL.String())
			}
			body := `{"errors":[{"code":"CONFLICT","title":"Conflict","detail":"replace rejected for empty set"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards?limit=1" {
				t.Fatalf("unexpected GET URL: %s", req.URL.String())
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.String() != "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards" {
				t.Fatalf("unexpected POST URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count: %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"game-center", "leaderboard-sets", "members", "set",
			"--set-id", "set-1",
			"--leaderboard-ids", "lb-1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"setId":"set-1"`) || !strings.Contains(stdout, `"memberCount":1`) {
		t.Fatalf("expected update output, got %q", stdout)
	}
}

func TestIssue800_LeaderboardSetMembersSetFallbackFailureReturnsConflictExitCode(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			body := `{"errors":[{"code":"CONFLICT","title":"Conflict","detail":"replace rejected for empty set"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			body := `{"errors":[{"code":"CONFLICT","title":"Conflict","detail":"add failed"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count: %d", requestCount)
			return nil, nil
		}
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"game-center", "leaderboard-sets", "members", "set",
			"--set-id", "set-1",
			"--leaderboard-ids", "lb-1",
		}, "1.2.3")
		if code != cmd.ExitConflict {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitConflict, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "set is empty") || !strings.Contains(stderr, "add fallback failed") {
		t.Fatalf("expected fallback error message, got %q", stderr)
	}
}
