package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestGameCenterBlockedPlayersList(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/gameCenterDetails/detail-45/blockedPlayers", "fields[gameCenterDetailPlayers]=nickname,blocked,bundleId", "limit=1"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterDetailPlayers","id":"player-1","attributes":{"nickname":"Runner","blocked":true,"bundleId":"com.example.game"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-45/blockedPlayers?cursor=page2"}}`))
			reg.Register(asctest.GET("/v1/gameCenterDetails/detail-45/blockedPlayers", "cursor=page2"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterDetailPlayers","id":"player-2","attributes":{"nickname":"Runner","blocked":true,"bundleId":"com.example.game"}}],"links":{"next":null}}`))

			out, stderr, runErr := runCommand(t, []string{"game-center", "details", "blocked-players", "list", "--detail-id", "detail-45", "--fields", "nickname,blocked,bundleId", "--limit", "1", "--paginate", "--output", format})
			calls := len(reg.Requests())
			if runErr != nil || stderr != "" || calls != 2 || !strings.Contains(out, "player-2") || !strings.Contains(out, "com.example.game") {
				t.Fatalf("run=%v calls=%d stdout=%s stderr=%s", runErr, calls, out, stderr)
			}
		})
	}
}

func TestGameCenterBlockedPlayersUpdate(t *testing.T) {
	for _, tt := range []struct{ name, blocked, bundle, wantAttrs string }{
		{"block", "true", "com.example.game", `{"blocked":true,"bundleId":"com.example.game"}`}, {"unblock", "false", "", `{"blocked":false}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH("/v1/gameCenterDetailPlayers/player-45").JSONBody(`{"data":{"type":"gameCenterDetailPlayers","id":"player-45","attributes":`+tt.wantAttrs+`}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterDetailPlayers","id":"player-45","attributes":{"nickname":"","blocked":`+tt.blocked+`,"bundleId":"com.example.game"}}}`))
			args := []string{"game-center", "details", "blocked-players", "update", "--id", "player-45", "--blocked", tt.blocked, "--confirm", "--output", "json"}
			if tt.bundle != "" {
				args = append(args, "--bundle-id", tt.bundle)
			}
			out, stderr, runErr := runCommand(t, args)
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/gameCenterDetails/d/blockedPlayers", "cursor=next"), asctest.JSON(tt.status, tt.body))
			out, stderr, runErr := runCommand(t, []string{"game-center", "details", "blocked-players", "list", "--next", "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/d/blockedPlayers?cursor=next", "--output", "json"})
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
