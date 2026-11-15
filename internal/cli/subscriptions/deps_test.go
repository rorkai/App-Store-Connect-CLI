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

func TestSubscriptionsUsageErrorsWriteToDepsStderr(t *testing.T) {
	tests := []struct {
		args       []string
		wantStderr string
	}{
		{[]string{"offers", "retention", "links"}, "Error: --subscription-id is required\n"},
		{[]string{"offers", "retention", "presented", "links"}, "Error: --subscription-id is required\n"},
		{[]string{"offers", "retention", "prices", "links"}, "Error: --id is required\n"},
		{[]string{"offers", "retention", "delete"}, "Error: --id is required\n"},
		{[]string{"review", "screenshots", "delete"}, "Error: --screenshot-id is required\n"},
		{[]string{"offers", "retention", "list", "--subscription-id", "S1", "--subscription-fields", "name"}, "Error: subscriptions offers retention list: --subscription-fields requires --include subscription\n"},
		{[]string{"offers", "retention", "view", "--id", "R1", "--subscription-fields", "name"}, "Error: subscriptions offers retention view: --subscription-fields requires --include subscription\n"},
		{[]string{"offers", "retention", "prices", "list", "--id", "R1", "--territory-fields", "currency"}, "Error: subscriptions offers retention prices list: --territory-fields requires --include territory\n"},
		{[]string{"retention-messages", "list", "--app", "A1", "--subscription-fields", "name"}, "Error: subscriptions retention-messages list: --subscription-fields requires --include subscriptions\n"},
		{[]string{"review", "screenshots", "view", "--screenshot-id", "X1", "--subscription-fields", "bogus"}, "Error: --subscription-fields must be one of: "},
		{[]string{"review", "app-store-screenshot", "view", "--subscription-id", "S1", "--subscription-fields", "bogus"}, "Error: --subscription-fields must be one of: "},
		{[]string{"offers", "introductory", "delete"}, "Error: --id is required\n"},
		{[]string{"offers", "promotional", "delete"}, "Error: --id is required\n"},
		{[]string{"offers", "promotional", "prices"}, "Error: --id is required\n"},
		{[]string{"offers", "offer-codes", "one-time-codes", "list"}, "Error: --offer-code-id is required\n"},
		{[]string{"offers", "offer-codes", "one-time-codes", "view"}, "Error: --batch-id is required\n"},
		{[]string{"offers", "offer-codes", "one-time-codes", "update"}, "Error: --batch-id is required\n"},
		{[]string{"offers", "offer-codes", "custom-codes", "list"}, "Error: --offer-code-id is required\n"},
		{[]string{"offers", "offer-codes", "custom-codes", "view"}, "Error: --custom-code-id is required\n"},
		{[]string{"offers", "win-back", "delete"}, "Error: --id is required\n"},
		{[]string{"offers", "win-back", "prices-links"}, "Error: --id is required\n"},
		{[]string{"pricing", "availability", "available-territories"}, "Error: --availability-id or --subscription-id is required\n"},
		{[]string{"pricing", "availability", "edit"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "availability", "view"}, "Error: --availability-id or --subscription-id is required\n"},
		{[]string{"pricing", "price-points", "adjusted-equalizations"}, "Error: --price-point-id is required\n"},
		{[]string{"pricing", "price-points", "equalizations"}, "Error: --price-point-id is required\n"},
		{[]string{"pricing", "price-points", "list"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "price-points", "view"}, "Error: --price-point-id is required\n"},
		{[]string{"pricing", "prices", "delete"}, "Error: --price-id is required\n"},
		{[]string{"pricing", "prices", "import"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "prices", "list"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "prices", "set"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "derive"}, "Error: --source-subscription-id is required\n"},
		{[]string{"pricing", "equalize"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "monthly-commitment", "disable"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "monthly-commitment", "enable"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "monthly-commitment", "list"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "plan-availability", "available-territories"}, "Error: --plan-availability-id is required\n"},
		{[]string{"pricing", "plan-availability", "set"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "plan-availability", "show"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "ppp"}, "Error: --subscription-id is required\n"},
		{[]string{"pricing", "summary"}, "Error: --app or --subscription-id is required\n"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			d, stderr := newSubscriptionsTestDeps()
			d.Client = func() (*asc.Client, error) { return nil, errors.New("no client in this test") }
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
