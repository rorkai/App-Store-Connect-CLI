package subscriptions

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsTestDeps returns deps whose streams are buffers, and the
// stderr buffer.
func newSubscriptionsTestDeps() (*shared.Deps, *bytes.Buffer) {
	var stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &stderr}}, &stderr
}

func TestSubscriptionsBuilderCommandsWriteUsageErrorsToDepsStderr(t *testing.T) {
	tests := []struct {
		args       []string
		wantStderr string
	}{
		{[]string{"offers", "retention", "links"}, "Error: --subscription-id is required\n"},
		{[]string{"offers", "retention", "presented", "links"}, "Error: --subscription-id is required\n"},
		{[]string{"offers", "retention", "prices", "links"}, "Error: --id is required\n"},
		{[]string{"offers", "retention", "delete"}, "Error: --id is required\n"},
		{[]string{"review", "screenshots", "delete"}, "Error: --screenshot-id is required\n"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			d, stderr := newSubscriptionsTestDeps()
			root := NewSubscriptionsCommand(d)
			if err := root.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			if got := stderr.String(); !strings.HasPrefix(got, test.wantStderr) {
				t.Fatalf("deps stderr = %q, want it to start with %q", got, test.wantStderr)
			}
		})
	}
}
