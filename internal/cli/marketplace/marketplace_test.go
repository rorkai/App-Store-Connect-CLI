package marketplace

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// assertMarketplaceUsageError runs the command built on test deps with args
// and checks that it fails validation with wantStderr before any request.
func assertMarketplaceUsageError(t *testing.T, newCommand func(*shared.Deps) *ffcli.Command, args []string, wantStderr string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := newCommand(&shared.Deps{
		Client: func() (*asc.Client, error) {
			return nil, errors.New("unexpected client request")
		},
		IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
	})
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestMarketplaceSearchDetailsGetCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsGetCommand, nil, "Error: --app is required (or set ASC_APP_ID)\n")
}

func TestMarketplaceSearchDetailsCreateCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsCreateCommand, []string{"--catalog-url", "https://example.com"}, "Error: --app is required (or set ASC_APP_ID)\n")
}

func TestMarketplaceSearchDetailsCreateCommand_MissingCatalogURL(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsCreateCommand, []string{"--app", "APP_ID"}, "Error: --catalog-url is required\n")
}

func TestMarketplaceSearchDetailsUpdateCommand_MissingID(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsUpdateCommand, []string{"--catalog-url", "https://example.com"}, "Error: --search-detail-id is required\n")
}

func TestMarketplaceSearchDetailsUpdateCommand_MissingUpdates(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsUpdateCommand, []string{"--search-detail-id", "DETAIL_ID"}, "Error: at least one update flag is required\n")
}

func TestMarketplaceSearchDetailsDeleteCommand_MissingID(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsDeleteCommand, []string{"--confirm"}, "Error: --search-detail-id is required\n")
}

func TestMarketplaceSearchDetailsDeleteCommand_MissingConfirm(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceSearchDetailsDeleteCommand, []string{"--search-detail-id", "DETAIL_ID"}, "Error: --confirm is required\n")
}

func TestMarketplaceWebhooksCommandIncludesView(t *testing.T) {
	command := newMarketplaceWebhooksCommand(&shared.Deps{IO: &shared.IO{Out: io.Discard, ErrOut: io.Discard}})
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "view" {
			if subcommand.ShortUsage != `asc marketplace webhooks view --webhook-id "WEBHOOK_ID" [flags]` {
				t.Fatalf("unexpected view usage: %q", subcommand.ShortUsage)
			}
			if subcommand.UsageFunc == nil {
				t.Fatal("expected view UsageFunc")
			}
			return
		}
	}
	t.Fatal("expected marketplace webhooks view subcommand")
}

func TestMarketplaceWebhooksCreateCommand_MissingURL(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksCreateCommand, []string{"--secret", "secret"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: --url is required\n")
}

func TestMarketplaceWebhooksCreateCommand_MissingSecret(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksCreateCommand, []string{"--url", "https://example.com"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: --secret is required\n")
}

func TestMarketplaceWebhooksUpdateCommand_MissingID(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksUpdateCommand, []string{"--url", "https://example.com"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: --webhook-id is required\n")
}

func TestMarketplaceWebhooksUpdateCommand_MissingUpdates(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksUpdateCommand, []string{"--webhook-id", "WEBHOOK_ID"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: at least one update flag is required\n")
}

func TestMarketplaceWebhooksDeleteCommand_MissingID(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksDeleteCommand, []string{"--confirm"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: --webhook-id is required\n")
}

func TestMarketplaceWebhooksDeleteCommand_MissingConfirm(t *testing.T) {
	assertMarketplaceUsageError(t, newMarketplaceWebhooksDeleteCommand, []string{"--webhook-id", "WEBHOOK_ID"}, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.\nError: --confirm is required\n")
}
