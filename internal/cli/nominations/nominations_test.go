package nominations

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestNominationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
		args       []string
		wantStderr string
	}{
		{name: "list missing status", newCommand: newNominationsListCommand, wantStderr: "Error: --status is required\n"},
		{name: "get missing id", newCommand: newNominationsGetCommand, wantStderr: "Error: --id is required\n"},
		{
			name:       "create missing submitted flag",
			newCommand: newNominationsCreateCommand,
			args: []string{
				"--app", "APP_ID",
				"--name", "Launch",
				"--type", "APP_LAUNCH",
				"--description", "Desc",
				"--publish-start-date", "2026-02-01T08:00:00Z",
			},
			wantStderr: "Error: --submitted is required\n",
		},
		{name: "update missing submitted and archived", newCommand: newNominationsUpdateCommand, args: []string{"--id", "NOM_ID", "--notes", "updated"}, wantStderr: "Error: --submitted or --archived is required by Apple's live API\n"},
		{name: "delete missing confirm", newCommand: newNominationsDeleteCommand, args: []string{"--id", "NOM_ID"}, wantStderr: "Error: --confirm is required to delete\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := test.newCommand(&shared.Deps{
				Client: func() (*asc.Client, error) {
					return nil, errors.New("unexpected client request")
				},
				IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
			})
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestNominationHelpers(t *testing.T) {
	if _, err := normalizeNominationType("bad"); err == nil {
		t.Fatal("expected invalid type error")
	}
	if got, err := normalizeNominationType("app_launch"); err != nil || got != "APP_LAUNCH" {
		t.Fatalf("expected APP_LAUNCH, got %q err=%v", got, err)
	}

	if _, err := normalizeNominationPublishDate("--publish-start-date", "bad", true); err == nil {
		t.Fatal("expected date parsing error")
	}

	ids := []string{"app-1", "app-2"}
	rel := buildNominationRelationshipList(asc.ResourceTypeApps, ids)
	if rel == nil || len(rel.Data) != 2 {
		t.Fatalf("expected relationship list with 2 items, got %#v", rel)
	}
}
