package offercodes

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestActiveUpdateCommandUsesDeps(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "missing id", args: nil, wantStderr: "Error: --code-id is required\n"},
		{name: "missing active", args: []string{"--code-id", "code-1"}, wantStderr: "Error: --active is required\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			clientRequested := false
			d := &shared.Deps{
				Client: func() (*asc.Client, error) {
					clientRequested = true
					return nil, errors.New("client must not be created")
				},
				IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
			}
			cmd := NewActiveUpdateCommand(ActiveUpdateCommandConfig{
				FlagSetName: "codes update",
				Name:        "update",
				IDFlag:      "code-id",
				IDType:      "subscriptionOfferCodeCustomCodes",
				ErrorPrefix: "codes update",
				Deps:        d,
			})
			if cmd.FlagSet.Output() != &stderr {
				t.Fatal("flag set output is not the deps stderr")
			}
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp", err)
			}
			if clientRequested {
				t.Fatal("client requested before validation")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
		})
	}
}
