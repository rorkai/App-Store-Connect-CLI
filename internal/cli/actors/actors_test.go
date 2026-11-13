package actors

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

func TestActorsValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
	}{
		{name: "list missing id", newCommand: newActorsListCommand},
		{name: "get missing id", newCommand: newActorsViewCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			d := &shared.Deps{
				Client: func() (*asc.Client, error) {
					return nil, errors.New("unexpected client request")
				},
				IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
			}

			cmd := tt.newCommand(d)
			if err := cmd.FlagSet.Parse([]string{}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
			if got, want := stderr.String(), "Error: --id is required\n"; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestActorsHelpWritesToDepsStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
	}{
		{name: "actors", newCommand: NewActorsCommand},
		{name: "list", newCommand: newActorsListCommand},
		{name: "view", newCommand: newActorsViewCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			cmd := tt.newCommand(&shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}})
			cmd.FlagSet.Init(cmd.FlagSet.Name(), flag.ContinueOnError)
			if err := cmd.Parse([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
			if got, want := stderr.String(), cmd.UsageFunc(cmd)+"\n"; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestNormalizeActorFields(t *testing.T) {
	got, err := normalizeActorFields("actorType,userEmail")
	if err != nil {
		t.Fatalf("expected valid fields, got err=%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got))
	}

	if _, err := normalizeActorFields("badField"); err == nil {
		t.Fatal("expected error for invalid field")
	}
}
