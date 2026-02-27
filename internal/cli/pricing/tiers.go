package pricing

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PricingTiersCommand returns the tiers subcommand.
func PricingTiersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing tiers", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "USA", "Territory ID (e.g., USA)")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "tiers",
		ShortUsage: "asc pricing tiers --app \"APP_ID\" [--territory \"USA\"] [--refresh]",
		ShortHelp:  "List pricing tiers for an app in a territory.",
		LongHelp: `List pricing tiers for an app in a territory.

Tiers are dynamically computed from the App Store Connect API by sorting
price points by customer price (ascending) and assigning tier numbers starting at 1.
Free (0.00) price points are excluded.

Results are cached locally (~/.asc/cache/) for 24 hours. Use --refresh to force a refresh.

Examples:
  asc pricing tiers --app "123456789"
  asc pricing tiers --app "123456789" --territory "USA"
  asc pricing tiers --app "123456789" --territory "JPN" --refresh
  asc pricing tiers --app "123456789" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			territoryValue := strings.ToUpper(strings.TrimSpace(*territory))
			if territoryValue == "" {
				territoryValue = "USA"
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing tiers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			tiers, err := shared.ResolveTiers(requestCtx, client, resolvedAppID, territoryValue, *refresh)
			if err != nil {
				return fmt.Errorf("pricing tiers: %w", err)
			}

			return shared.PrintOutputWithRenderers(tiers, *output.Output, *output.Pretty,
				func() error {
					return printTiersTable(tiers)
				},
				func() error {
					return printTiersMarkdown(tiers)
				},
			)
		},
	}
}

func printTiersTable(tiers []shared.TierEntry) error {
	fmt.Printf("%-6s  %-40s  %-15s  %-10s\n", "Tier", "Price Point ID", "Customer Price", "Proceeds")
	fmt.Printf("%-6s  %-40s  %-15s  %-10s\n", "----", "----------------------------------------", "---------------", "----------")
	for _, t := range tiers {
		fmt.Printf("%-6d  %-40s  %-15s  %-10s\n", t.Tier, t.PricePointID, t.CustomerPrice, t.Proceeds)
	}
	return nil
}

func printTiersMarkdown(tiers []shared.TierEntry) error {
	fmt.Println("| Tier | Price Point ID | Customer Price | Proceeds |")
	fmt.Println("|------|----------------|----------------|----------|")
	for _, t := range tiers {
		fmt.Printf("| %d | %s | %s | %s |\n", t.Tier, t.PricePointID, t.CustomerPrice, t.Proceeds)
	}
	return nil
}
