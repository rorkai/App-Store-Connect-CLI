package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGameCenterBlockedPlayersList(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			setupAuth(t)
			calls := 0
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/v1/gameCenterDetails/detail-45/blockedPlayers" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				if calls == 1 && (r.URL.Query().Get("fields[gameCenterDetailPlayers]") != "nickname,blocked,bundleId" || r.URL.Query().Get("limit") != "1") {
					t.Errorf("query=%s", r.URL.RawQuery)
				}
				if calls == 2 && r.URL.Query().Get("cursor") != "page2" {
					t.Errorf("query=%s", r.URL.RawQuery)
				}
				next := "null"
				if calls == 1 {
					next = `"https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-45/blockedPlayers?cursor=page2"`
				}
				body := fmt.Sprintf(`{"data":[{"type":"gameCenterDetailPlayers","id":"player-%d","attributes":{"nickname":"Runner","blocked":true,"bundleId":"com.example.game"}}],"links":{"next":%s}}`, calls, next)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			var runErr error
			out, stderr := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"game-center", "details", "blocked-players", "list", "--detail-id", "detail-45", "--fields", "nickname,blocked,bundleId", "--limit", "1", "--paginate", "--output", format})
			})
			if runErr != nil || stderr != "" || calls != 2 || !strings.Contains(out, "player-2") || !strings.Contains(out, "com.example.game") {
				t.Fatalf("run=%v calls=%d stdout=%s stderr=%s", runErr, calls, out, stderr)
			}
		})
	}
}

func TestGameCenterBlockedPlayersUpdate(t *testing.T) {
	for _, tt := range []struct{ name, blocked, bundle string }{
		{"block", "true", "com.example.game"}, {"unblock", "false", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPatch || r.URL.Path != "/v1/gameCenterDetailPlayers/player-45" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				var got struct {
					Data struct {
						Type, ID   string
						Attributes map[string]any
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if got.Data.Type != "gameCenterDetailPlayers" || got.Data.ID != "player-45" || got.Data.Attributes["blocked"] != (tt.blocked == "true") {
					t.Errorf("payload=%+v", got)
				}
				if tt.bundle == "" {
					if len(got.Data.Attributes) != 1 {
						t.Errorf("unexpected attributes=%v", got.Data.Attributes)
					}
				} else if got.Data.Attributes["bundleId"] != tt.bundle || len(got.Data.Attributes) != 2 {
					t.Errorf("attributes=%v", got.Data.Attributes)
				}
				body := fmt.Sprintf(`{"data":{"type":"gameCenterDetailPlayers","id":"player-45","attributes":{"nickname":"","blocked":%s,"bundleId":"com.example.game"}}}`, tt.blocked)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			args := []string{"game-center", "details", "blocked-players", "update", "--id", "player-45", "--blocked", tt.blocked, "--confirm", "--output", "json"}
			if tt.bundle != "" {
				args = append(args, "--bundle-id", tt.bundle)
			}
			var runErr error
			out, stderr := captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			if runErr != nil || stderr != "" || !strings.Contains(out, `"blocked":`+tt.blocked) || !strings.Contains(out, `"nickname":""`) {
				t.Fatalf("run=%v out=%s stderr=%s", runErr, out, stderr)
			}
		})
	}
}

func TestGameCenterBlockedPlayersUsage(t *testing.T) {
	prefix := []string{"game-center", "details", "blocked-players"}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing detail", []string{"list"}, "--detail-id is required"},
		{"zero limit", []string{"list", "--detail-id", "d", "--limit", "0"}, "--limit must be between 1 and 200"},
		{"bad fields", []string{"list", "--detail-id", "d", "--fields", "score"}, "--fields"},
		{"empty fields", []string{"list", "--detail-id", "d", "--fields", " "}, "--fields must not be empty"},
		{"next conflict", []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/d/blockedPlayers?cursor=x", "--detail-id", "d"}, "--next cannot be combined with --detail-id"},
		{"missing blocked", []string{"update", "--id", "p", "--confirm"}, "--blocked is required"},
		{"missing confirm", []string{"update", "--id", "p", "--blocked", "false"}, "--confirm is required"},
		{"empty bundle", []string{"update", "--id", "p", "--blocked", "true", "--bundle-id", " ", "--confirm"}, "--bundle-id must not be empty"},
	} {
		t.Run(tt.name, func(t *testing.T) { assertUsageExit(t, append(append([]string{}, prefix...), tt.args...), tt.want) })
	}
}

func TestGameCenterBlockedPlayersNextAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		status           int
	}{
		{"next only", `{"data":[],"links":{},"meta":{"paging":{"total":0}}}`, `"total":0`, 200},
		{"bad JSON", `invalid`, "failed to parse blocked players", 200},
		{"Apple error", `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Not allowed"}]}`, "Not allowed", 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("cursor") != "next" {
					t.Errorf("query=%s", r.URL.RawQuery)
				}
				return &http.Response{StatusCode: tt.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			}))
			var runErr error
			out, stderr := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"game-center", "details", "blocked-players", "list", "--next", "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/d/blockedPlayers?cursor=next", "--output", "json"})
			})
			if tt.name == "next only" {
				if runErr != nil || stderr != "" || !strings.Contains(out, tt.want) {
					t.Fatalf("run=%v stdout=%s stderr=%s", runErr, out, stderr)
				}
			} else if runErr == nil || !strings.Contains(runErr.Error(), tt.want) || out != "" {
				t.Fatalf("run=%v stdout=%s", runErr, out)
			}
		})
	}
}
