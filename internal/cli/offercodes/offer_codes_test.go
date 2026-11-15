package offercodes

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestOfferCodesUsageErrorsWriteToDepsStderr(t *testing.T) {
	tests := []struct {
		name       string
		build      func(*shared.Deps) *ffcli.Command
		args       []string
		wantStderr string
	}{
		{
			name:       "values without batch ID",
			build:      NewOfferCodesValuesCommand,
			wantStderr: "Error: --batch-id is required\n",
		},
		{
			name:       "values with an unknown file format",
			build:      NewOfferCodesValuesCommand,
			args:       []string{"--batch-id", "BATCH", "--format", "xml"},
			wantStderr: "Error: --format must be text or csv\n",
		},
		{
			name:       "generate without offer code ID",
			build:      NewOfferCodesGenerateCommand,
			wantStderr: "Error: --offer-code-id is required\n\n",
		},
		{
			name:       "custom code create with a bad expiration date",
			build:      newOfferCodeCustomCodesCreateCommand,
			args:       []string{"--offer-code-id", "OFFER", "--code", "SPRING", "--quantity", "1", "--expiration-date", "soon"},
			wantStderr: "Error: --expiration-date must be in YYYY-MM-DD format\n",
		},
		{
			name: "active update without ID",
			build: func(d *shared.Deps) *ffcli.Command {
				return NewActiveUpdateCommand(ActiveUpdateCommandConfig{FlagSetName: "update", Name: "update", IDFlag: "custom-code-id", Deps: d})
			},
			wantStderr: "Error: --custom-code-id is required\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := test.build(&shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}})
			cmd.FlagSet.Init(cmd.Name, flag.ContinueOnError)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if stderr.String() != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantStderr)
			}
		})
	}
}
