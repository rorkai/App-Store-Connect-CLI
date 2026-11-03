package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGameCenterDetailsCreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name         string
		args         []string
		stderrSubstr string
	}{
		{
			name:         "missing app",
			args:         []string{"game-center", "details", "create"},
			stderrSubstr: "Error: --app is required (or set ASC_APP_ID)",
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
			if !strings.Contains(stderr, test.stderrSubstr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.stderrSubstr, stderr)
			}
		})
	}
}

func TestGameCenterDetailsUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		stderrSubstr string
	}{
		{
			name:         "missing id",
			args:         []string{"game-center", "details", "update", "--game-center-group-id", "GROUP_ID"},
			stderrSubstr: "Error: --id is required",
		},
		{
			name:         "no update flags",
			args:         []string{"game-center", "details", "update", "--id", "DETAIL_ID"},
			stderrSubstr: "Error: at least one update flag is required (--game-center-group-id, --default-leaderboard-id, --default-leaderboard-v2-id, --default-group-leaderboard-id, --default-group-leaderboard-v2-id)",
		},
		{
			// Live: Apple returned 200 for both and kept only the V2 value.
			name:         "default leaderboard v1 with v2",
			args:         []string{"game-center", "details", "update", "--id", "DETAIL_ID", "--default-leaderboard-id", "LB_V1", "--default-leaderboard-v2-id", "LB_V2"},
			stderrSubstr: "Error: --default-leaderboard-id and --default-leaderboard-v2-id set the same default; pass one",
		},
		{
			name:         "default group leaderboard v1 with v2",
			args:         []string{"game-center", "details", "update", "--id", "DETAIL_ID", "--default-group-leaderboard-id", "GROUP_LB_V1", "--default-group-leaderboard-v2-id", "GROUP_LB_V2"},
			stderrSubstr: "Error: --default-group-leaderboard-id and --default-group-leaderboard-v2-id set the same default; pass one",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}))

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
			if !strings.Contains(stderr, test.stderrSubstr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.stderrSubstr, stderr)
			}
		})
	}
}

func TestGameCenterDetailsUpdateSendsOnlyRequestedRelationships(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "group with v2 default and v1 group default",
			args: []string{
				"--game-center-group-id", "GROUP_ID",
				"--default-leaderboard-v2-id", "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/LB_V2",
				"--default-group-leaderboard-id", "GROUP_LB_V1",
			},
			want: `{
				"gameCenterGroup": {"data": {"type": "gameCenterGroups", "id": "GROUP_ID"}},
				"defaultLeaderboardV2": {"data": {"type": "gameCenterLeaderboards", "id": "LB_V2"}},
				"defaultGroupLeaderboard": {"data": {"type": "gameCenterLeaderboards", "id": "GROUP_LB_V1"}}
			}`,
		},
		{
			name: "one v2 default leaves the others out",
			args: []string{"--default-group-leaderboard-v2-id", "GROUP_LB_V2"},
			want: `{"defaultGroupLeaderboardV2": {"data": {"type": "gameCenterLeaderboards", "id": "GROUP_LB_V2"}}}`,
		},
		{
			name: "existing flag request is unchanged",
			args: []string{"--default-leaderboard-id", "LB_V1"},
			want: `{"defaultLeaderboard": {"data": {"type": "gameCenterLeaderboards", "id": "LB_V1"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			var body map[string]any
			requests := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/gameCenterDetails/DETAIL_ID" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"DETAIL_ID"}}`), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"game-center", "details", "update", "--id", "DETAIL_ID", "--output", "json"}, test.args...)

			captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if requests != 1 {
				t.Fatalf("expected 1 request, got %d", requests)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(`{"data":{"type":"gameCenterDetails","id":"DETAIL_ID","relationships":`+test.want+`}}`), &want); err != nil {
				t.Fatalf("decode expected body: %v", err)
			}
			if !reflect.DeepEqual(body, want) {
				t.Fatalf("request body = %v, want %v", body, want)
			}
		})
	}
}
