package cmdtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestGameCenterLeaderboardSetMemberLocalizationsViewCompositeOutput(t *testing.T) {
	requestCount := 0
	client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboard" {
				t.Fatalf("request 1 path = %q", req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`), nil
		case 2:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboardSet" {
				t.Fatalf("request 2 path = %q", req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{}}}`), nil
		case 3:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations" {
				t.Fatalf("request 3 path = %q", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[gameCenterLeaderboard]") != "lb-1" || query.Get("filter[gameCenterLeaderboardSet]") != "set-1" {
				t.Fatalf("request 3 filters = %q", req.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-1","attributes":{"name":"Top Score","locale":"en-US"}}],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, runErr := runRootWithDeps(t, client, "game-center", "leaderboard-sets", "member-localizations", "view", "--id", "loc-1", "--output", "json")

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	var output struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Name   string `json:"name"`
				Locale string `json:"locale"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	if output.Data.ID != "loc-1" || output.Data.Attributes.Name != "Top Score" || output.Data.Attributes.Locale != "en-US" {
		t.Fatalf("unexpected output: %+v", output.Data)
	}
}

func TestGameCenterLeaderboardSetMemberLocalizationsViewNotFoundExitCode(t *testing.T) {
	requestCount := 0
	client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{}}}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL.String())
			return nil, nil
		}
	}))

	stdout, _, runErr := runRootWithDeps(t, client, "game-center", "leaderboard-sets", "member-localizations", "view", "--id", "loc-missing", "--output", "json")

	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("run error = %v, want ErrNotFound", runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitNotFound)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}
