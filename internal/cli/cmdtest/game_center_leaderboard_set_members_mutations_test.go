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

func gcLeaderboardSetMembersJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGameCenterLeaderboardSetMembersSetAddsMembersToEmptySet(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		prefix  string
	}{
		{name: "v1", command: []string{"game-center", "leaderboard-sets", "members", "set"}, prefix: "/v1"},
		{name: "v2", command: []string{"game-center", "leaderboard-sets", "v2", "members", "set"}, prefix: "/v2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			callCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				switch callCount {
				case 1:
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if req.URL.Path != test.prefix+"/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards" {
						t.Fatalf("expected path %s/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards, got %s", test.prefix, req.URL.Path)
					}
					if req.URL.Query().Get("limit") != "200" {
						t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
					}

					return gcLeaderboardSetMembersJSONResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
				case 2:
					if req.Method != http.MethodPost {
						t.Fatalf("expected POST, got %s", req.Method)
					}
					if req.URL.Path != test.prefix+"/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards" {
						t.Fatalf("expected path %s/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards, got %s", test.prefix, req.URL.Path)
					}

					payload, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read body error: %v", err)
					}
					body := string(payload)
					if !strings.Contains(body, `"id":"lb-1"`) || !strings.Contains(body, `"id":"lb-2"`) {
						t.Fatalf("expected leaderboard ids lb-1/lb-2 in POST payload, got %s", body)
					}
					if strings.Count(body, `"id":"lb-1"`) != 1 || strings.Count(body, `"id":"lb-2"`) != 1 {
						t.Fatalf("expected deduplicated POST payload, got %s", body)
					}

					return gcLeaderboardSetMembersJSONResponse(http.StatusNoContent, ""), nil
				case 3:
					if req.Method != http.MethodPatch {
						t.Fatalf("expected PATCH, got %s", req.Method)
					}
					if req.URL.Path != test.prefix+"/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards" {
						t.Fatalf("expected path %s/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards, got %s", test.prefix, req.URL.Path)
					}

					payload, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read body error: %v", err)
					}
					body := string(payload)
					i1 := strings.Index(body, `"id":"lb-1"`)
					i2 := strings.Index(body, `"id":"lb-2"`)
					if i1 == -1 || i2 == -1 || i1 > i2 {
						t.Fatalf("expected PATCH payload order lb-1,lb-2; got %s", body)
					}
					if strings.Count(body, `"id":"lb-1"`) != 1 || strings.Count(body, `"id":"lb-2"`) != 1 {
						t.Fatalf("expected deduplicated PATCH payload, got %s", body)
					}

					return gcLeaderboardSetMembersJSONResponse(http.StatusNoContent, ""), nil
				default:
					t.Fatalf("unexpected request #%d: %s %s", callCount, req.Method, req.URL.Path)
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append(test.command,
					"--set-id", "set-1",
					"--leaderboard-ids", "lb-1,lb-1,lb-2",
					"--confirm",
				)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"setId":"set-1"`) || !strings.Contains(stdout, `"memberCount":2`) || !strings.Contains(stdout, `"memberIds":["lb-1","lb-2"]`) || !strings.Contains(stdout, `"updated":true`) {
				t.Fatalf("expected update result JSON in stdout, got %q", stdout)
			}
			if callCount != 3 {
				t.Fatalf("expected 3 API calls, got %d", callCount)
			}
		})
	}
}
