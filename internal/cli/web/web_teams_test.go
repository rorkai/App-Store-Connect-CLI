package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebTeamsListDependencies(t *testing.T) *int {
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
	var persistCalls int
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	listDeveloperTeamsFn = func(context.Context, *webcore.Client) (*asc.WebDeveloperTeamsResult, error) {
		return &asc.WebDeveloperTeamsResult{Teams: []asc.WebDeveloperTeam{
			{TeamID: "TEAMONE123", Name: "Example One", Status: "active"},
			{TeamID: "TEAMTWO456", Name: "Example Two", Status: "expired"},
		}}, nil
	}
	return &persistCalls
}

func TestWebTeamsListPrintsEveryTeam(t *testing.T) {
	tests := []struct {
		output string
		check  func(t *testing.T, stdout string)
	}{
		{
			output: "json",
			check: func(t *testing.T, stdout string) {
				var got struct {
					Teams []map[string]string `json:"teams"`
				}
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatalf("decode JSON output %q: %v", stdout, err)
				}
				want := []map[string]string{
					{"teamId": "TEAMONE123", "name": "Example One", "status": "active"},
					{"teamId": "TEAMTWO456", "name": "Example Two", "status": "expired"},
				}
				if len(got.Teams) != len(want) {
					t.Fatalf("teams = %+v, want %+v", got.Teams, want)
				}
				for i := range want {
					for key, value := range want[i] {
						if got.Teams[i][key] != value {
							t.Fatalf("teams[%d][%s] = %q, want %q", i, key, got.Teams[i][key], value)
						}
					}
				}
			},
		},
		{
			output: "table",
			check: func(t *testing.T, stdout string) {
				for _, want := range []string{"Team ID", "Name", "Status", "TEAMONE123", "Example One", "active", "TEAMTWO456", "Example Two", "expired"} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("table output %q does not contain %q", stdout, want)
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.output, func(t *testing.T) {
			persistCalls := stubWebTeamsListDependencies(t)

			command := WebTeamsListCommand()
			if err := command.FlagSet.Parse([]string{"--output", test.output}); err != nil {
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
			if *persistCalls != 1 {
				t.Fatalf("persist calls = %d, want 1", *persistCalls)
			}
			test.check(t, stdout)
		})
	}
}

func TestWebTeamsListRejectsPositionalArguments(t *testing.T) {
	stubWebTeamsListDependencies(t)
	listDeveloperTeamsFn = func(context.Context, *webcore.Client) (*asc.WebDeveloperTeamsResult, error) {
		t.Fatal("list must not be called")
		return nil, nil
	}

	command := WebTeamsListCommand()
	if err := command.FlagSet.Parse([]string{"extra"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), command.FlagSet.Args()); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "web teams list does not accept positional arguments") {
		t.Fatalf("stderr = %q, want positional argument error", stderr)
	}
}
