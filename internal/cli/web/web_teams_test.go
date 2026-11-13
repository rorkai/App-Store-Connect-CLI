package web

import (
	"context"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebTeamsListDependencies(t *testing.T) {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listDeveloperTeamsFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listDeveloperTeamsFn = origList
		persistWebSessionFn = origPersist
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
}

func TestWebTeamsListPrintsTeamFieldsAsJSON(t *testing.T) {
	stubWebTeamsListDependencies(t)
	listDeveloperTeamsFn = func(context.Context, *webcore.Client) (*asc.WebDeveloperTeamsResult, error) {
		return &asc.WebDeveloperTeamsResult{Teams: []asc.WebDeveloperTeam{
			{TeamID: "TEAMONE123", Name: "Example One", Status: "active"},
		}}, nil
	}

	command := WebTeamsListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	want := `{"teams":[{"teamId":"TEAMONE123","name":"Example One","status":"active"}]}`
	if got := strings.TrimSpace(stdout); got != want {
		t.Fatalf("stdout = %s, want %s", got, want)
	}
}
