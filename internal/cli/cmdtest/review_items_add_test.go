package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func reviewItemsAddJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestReviewItemsAddAcceptsDocumentedAdditionalItemTypes(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
	}{
		{name: "background asset versions", itemType: "backgroundAssetVersions"},
		{name: "game center achievement versions", itemType: "gameCenterAchievementVersions"},
		{name: "game center activity versions", itemType: "gameCenterActivityVersions"},
		{name: "game center challenge versions", itemType: "gameCenterChallengeVersions"},
		{name: "game center leaderboard set versions", itemType: "gameCenterLeaderboardSetVersions"},
		{name: "game center leaderboard versions", itemType: "gameCenterLeaderboardVersions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", req.Method)
				}
				if req.URL.Path != "/v1/reviewSubmissionItems" {
					t.Fatalf("expected path /v1/reviewSubmissionItems, got %s", req.URL.Path)
				}

				return reviewItemsAddJSONResponse(http.StatusCreated, `{
					"data": {
						"type": "reviewSubmissionItems",
						"id": "item-123"
					}
				}`)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"review", "items-add",
					"--submission", "submission-123",
					"--item-type", test.itemType,
					"--item-id", "resource-123",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stdout == "" {
				t.Fatal("expected JSON output on stdout")
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}
