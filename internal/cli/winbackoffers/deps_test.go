package winbackoffers

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestWinBackOffersUsageErrorsWriteToDepsStderr(t *testing.T) {
	tests := []struct {
		name       string
		build      func(*shared.Deps) *ffcli.Command
		args       []string
		wantStderr string
	}{
		{
			name:       "list price fields without prices include",
			build:      newWinBackOffersListCommand,
			args:       []string{"--subscription-id", "SUB", "--price-fields", "territory"},
			wantStderr: "Error: --price-fields requires --include prices\n",
		},
		{
			name:       "prices territory fields without territory include",
			build:      newWinBackOffersPricesCommand,
			args:       []string{"--id", "OFFER", "--territory-fields", "currency"},
			wantStderr: "Error: --territory-fields requires --include territory\n",
		},
		{
			name:       "links without subscription ID",
			build:      newWinBackOffersRelationshipsCommand,
			wantStderr: "Error: --subscription-id is required\n",
		},
		{
			name:       "update without ID",
			build:      newWinBackOffersUpdateCommand,
			wantStderr: "Error: --id is required\n",
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
