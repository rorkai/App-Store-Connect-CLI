package marketplace

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestMarketplaceSearchDetailsGetCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := MarketplaceSearchDetailsGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsCreateCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := MarketplaceSearchDetailsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--catalog-url", "https://example.com"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsCreateCommand_MissingCatalogURL(t *testing.T) {
	cmd := MarketplaceSearchDetailsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --catalog-url is missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsUpdateCommand_MissingID(t *testing.T) {
	cmd := MarketplaceSearchDetailsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--catalog-url", "https://example.com"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --search-detail-id is missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsUpdateCommand_MissingUpdates(t *testing.T) {
	cmd := MarketplaceSearchDetailsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--search-detail-id", "DETAIL_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when update flags are missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsDeleteCommand_MissingID(t *testing.T) {
	cmd := MarketplaceSearchDetailsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --search-detail-id is missing, got %v", err)
	}
}

func TestMarketplaceSearchDetailsDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := MarketplaceSearchDetailsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{"--search-detail-id", "DETAIL_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestMarketplaceWebhooksCommandIncludesView(t *testing.T) {
	command := MarketplaceWebhooksCommand()
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
	cmd := MarketplaceWebhooksCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--secret", "secret"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --url is missing, got %v", err)
	}
}

func TestMarketplaceWebhooksCreateCommand_MissingSecret(t *testing.T) {
	cmd := MarketplaceWebhooksCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--url", "https://example.com"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --secret is missing, got %v", err)
	}
}

func TestMarketplaceWebhooksUpdateCommand_MissingID(t *testing.T) {
	cmd := MarketplaceWebhooksUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--url", "https://example.com"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --webhook-id is missing, got %v", err)
	}
}

func TestMarketplaceWebhooksUpdateCommand_MissingUpdates(t *testing.T) {
	cmd := MarketplaceWebhooksUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--webhook-id", "WEBHOOK_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when update flags are missing, got %v", err)
	}
}

func TestMarketplaceWebhooksDeleteCommand_MissingID(t *testing.T) {
	cmd := MarketplaceWebhooksDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --webhook-id is missing, got %v", err)
	}
}

func TestMarketplaceWebhooksDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := MarketplaceWebhooksDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{"--webhook-id", "WEBHOOK_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}
