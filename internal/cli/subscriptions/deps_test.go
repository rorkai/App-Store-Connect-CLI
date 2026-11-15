package subscriptions

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

func TestSubscriptionsStableSelectorWithoutAppWritesToDepsStderr(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	for _, args := range [][]string{
		{"versions", "list", "--subscription-id", "com.example.pro"},
		{"versions", "create", "--subscription-id", "com.example.pro"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			d, stderr := newSubscriptionsTestDeps()
			d.Client = func() (*asc.Client, error) { return nil, nil }
			d.Timeout = shared.ContextWithTimeout
			root := NewSubscriptionsCommand(d)
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			want := "Error: --app is required (or set ASC_APP_ID) when --subscription-id is a product ID or name\n"
			if got := stderr.String(); !strings.HasPrefix(got, want) {
				t.Fatalf("deps stderr = %q, want it to start with %q", got, want)
			}
		})
	}
}
