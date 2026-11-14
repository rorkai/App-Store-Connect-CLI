package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

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

			const members = `{"data":[{"type":"gameCenterLeaderboards","id":"lb-1"},{"type":"gameCenterLeaderboards","id":"lb-2"}]}`
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.prefix+"/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
			reg.Register(asctest.POST(test.prefix+"/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards").JSONBody(members),
				asctest.JSON(http.StatusNoContent, ""))
			reg.Register(asctest.PATCH(test.prefix+"/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards").JSONBody(members),
				asctest.JSON(http.StatusNoContent, ""))

			stdout, stderr, err := runCommand(t, append(test.command,
				"--set-id", "set-1",
				"--leaderboard-ids", "lb-1,lb-1,lb-2",
				"--confirm",
			))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"setId":"set-1"`) || !strings.Contains(stdout, `"memberCount":2`) || !strings.Contains(stdout, `"memberIds":["lb-1","lb-2"]`) || !strings.Contains(stdout, `"updated":true`) {
				t.Fatalf("expected update result JSON in stdout, got %q", stdout)
			}
			if callCount := len(reg.Requests()); callCount != 3 {
				t.Fatalf("expected 3 API calls, got %d", callCount)
			}
		})
	}
}
