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

func TestGameCenterScoreModerations(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			setupAuth(t)
			calls := 0
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/v2/gameCenterLeaderboards/lb-45/gameCenterScoreModerations" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				q := r.URL.Query()
				if calls == 1 {
					if q.Get("exists[blocked]") != "false" || q.Get("include") != "player" || q.Get("limit") != "1" || q.Get("fields[gameCenterScoreModerations]") != "rank,score,blocked,player" || q.Get("fields[gameCenterDetailPlayers]") != "nickname,blocked" {
						t.Errorf("query=%v", q)
					}
				} else if q.Get("cursor") != "page2" {
					t.Errorf("next query=%v", q)
				}
				next := "null"
				if calls == 1 {
					next = `"https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/lb-45/gameCenterScoreModerations?cursor=page2"`
				}
				body := fmt.Sprintf(`{"data":[{"type":"gameCenterScoreModerations","id":"score-%d","attributes":{"rank":"1","score":"9007199254740993","blocked":false,"preReleased":false,"context":"18446744073709551615","challengeIds":[]},"relationships":{"player":{"data":{"type":"gameCenterDetailPlayers","id":"player-45"}}}}],"links":{"next":%s},"included":[{"type":"gameCenterDetailPlayers","id":"player-45","attributes":{"nickname":"Runner","blocked":false}}],"meta":{"paging":{"total":2}}}`, calls, next)
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			root := RootCommand("test")
			var runErr error
			out, errout := captureOutput(t, func() {
				runErr = root.ParseAndRun(context.Background(), []string{"game-center", "leaderboards", "v2", "score-moderations", "list", "--leaderboard-id", "lb-45", "--exists-blocked", "false", "--include", "player", "--fields", "rank,score,blocked", "--player-fields", "nickname,blocked", "--limit", "1", "--paginate", "--output", format})
			})
			if runErr != nil || errout != "" {
				t.Fatalf("run=%v stderr=%s", runErr, errout)
			}
			if calls != 2 || !strings.Contains(out, "score-2") || !strings.Contains(out, "9007199254740993") {
				t.Fatalf("calls=%d output=%s", calls, out)
			}
			if format == "json" {
				var response struct {
					Data []struct {
						Attributes    map[string]any
						Relationships json.RawMessage
					}
					Included []json.RawMessage
					Meta     json.RawMessage
				}
				if err := json.Unmarshal([]byte(out), &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Data) != 2 || response.Data[0].Attributes["blocked"] != false || response.Data[0].Attributes["preReleased"] != false || response.Data[0].Attributes["context"] != "18446744073709551615" || len(response.Included) != 1 || !strings.Contains(string(response.Data[0].Relationships), "player-45") {
					t.Fatalf("envelope lost: %s", out)
				}
			}
		})
	}
}

func TestGameCenterScoreModerationUpdate(t *testing.T) {
	for _, blocked := range []string{"true", "false"} {
		t.Run(blocked, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPatch || r.URL.Path != "/v1/gameCenterScoreModerations/score-45" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				body, _ := io.ReadAll(r.Body)
				var got map[string]any
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				want := fmt.Sprintf(`{"data":{"type":"gameCenterScoreModerations","id":"score-45","attributes":{"blocked":%s}}}`, blocked)
				compact, _ := json.Marshal(got)
				var expected map[string]any
				_ = json.Unmarshal([]byte(want), &expected)
				normalized, _ := json.Marshal(expected)
				if string(compact) != string(normalized) {
					t.Errorf("body=%s want=%s", body, want)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(want))}, nil
			}))
			var runErr error
			out, stderr := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"game-center", "leaderboards", "v2", "score-moderations", "update", "--id", "score-45", "--blocked", blocked, "--confirm", "--output", "json"})
			})
			if runErr != nil || stderr != "" || !strings.Contains(out, `"blocked":`+blocked) {
				t.Fatalf("run=%v stdout=%s stderr=%s", runErr, out, stderr)
			}
		})
	}
}

func TestGameCenterScoreModerationUsage(t *testing.T) {
	prefix := []string{"game-center", "leaderboards", "v2", "score-moderations"}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"missing parent", []string{"list"}, "--leaderboard-id is required"},
		{"zero limit", []string{"list", "--leaderboard-id", "lb", "--limit", "0"}, "--limit must be between 1 and 200"},
		{"next conflict", []string{"list", "--next", "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/lb/gameCenterScoreModerations?cursor=x", "--leaderboard-id", "lb"}, "--next cannot be combined with --leaderboard-id"},
		{"bad field", []string{"list", "--leaderboard-id", "lb", "--fields", "bogus"}, "--fields"},
		{"empty field", []string{"list", "--leaderboard-id", "lb", "--fields", " "}, "--fields must not be empty"},
		{"player without include", []string{"list", "--leaderboard-id", "lb", "--player-fields", "nickname"}, "--player-fields requires --include player"},
		{"missing blocked", []string{"update", "--id", "s", "--confirm"}, "--blocked is required"},
		{"missing confirm", []string{"update", "--id", "s", "--blocked", "false"}, "--confirm is required"},
		{"false confirm", []string{"update", "--id", "s", "--blocked", "true", "--confirm=false"}, "--confirm must be true"},
	} {
		t.Run(tt.name, func(t *testing.T) { assertUsageExit(t, append(append([]string{}, prefix...), tt.args...), tt.want) })
	}
}

func TestGameCenterScoreModerationNextAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		status           int
	}{
		{"next only", `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/scores"},"meta":{"paging":{"total":0}}}`, `"total":0`, 200},
		{"bad JSON", `not json`, "failed to parse score moderations", 200},
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
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"game-center", "leaderboards", "v2", "score-moderations", "list", "--next", "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/lb/gameCenterScoreModerations?cursor=next", "--output", "json"})
			})
			if tt.name == "next only" {
				if runErr != nil || stderr != "" || !strings.Contains(out, tt.want) {
					t.Fatalf("run=%v out=%s stderr=%s", runErr, out, stderr)
				}
			} else if runErr == nil || !strings.Contains(runErr.Error(), tt.want) || out != "" {
				t.Fatalf("run=%v out=%s", runErr, out)
			}
		})
	}
}
