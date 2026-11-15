package pricing

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newPricingTiersCommand returns the tiers subcommand.
func newPricingTiersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing tiers", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "USA", "Territory input (accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "tiers",
		ShortUsage: "asc pricing tiers --app \"APP_ID\" [--territory \"US\"] [--refresh]",
		ShortHelp:  "List pricing tiers for an app in a territory.",
		LongHelp: `List pricing tiers for an app in a territory.

Tiers are dynamically computed from the App Store Connect API by sorting
price points by customer price (ascending) and assigning tier numbers starting at 1.
Free (0.00) price points are excluded.

Results are cached locally (~/.asc/cache/) for 24 hours. Use --refresh to force a refresh.

Examples:
  asc pricing tiers --app "123456789"
  asc pricing tiers --app "123456789" --territory "United States"
  asc pricing tiers --app "123456789" --territory "JPN" --refresh
  asc pricing tiers --app "123456789" --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			territoryInput := strings.TrimSpace(*territory)
			if territoryInput == "" {
				territoryInput = "USA"
			}
			territoryValue, err := ascterritory.Normalize(territoryInput)
			if err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing tiers: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			tiers, err := d.ResolveTiers(requestCtx, client, resolvedAppID, territoryValue, *refresh)
			if err != nil {
				return fmt.Errorf("pricing tiers: %w", err)
			}

			return d.PrintOutputWithRenderers(
				tiers, *output.Output, *output.Pretty,
				func() error {
					return printTiersTable(d.IO.Out, tiers)
				},
				func() error {
					return printTiersMarkdown(d.IO.Out, tiers)
				},
			)
		},
	}
}

func printTiersTable(w io.Writer, tiers []shared.TierEntry) error {
	fmt.Fprintf(w, "%-6s  %-40s  %-15s  %-10s\n", "Tier", "Price Point ID", "Customer Price", "Proceeds")
	fmt.Fprintf(w, "%-6s  %-40s  %-15s  %-10s\n", "----", "----------------------------------------", "---------------", "----------")
	for _, t := range tiers {
		fmt.Fprintf(w, "%-6d  %-40s  %-15s  %-10s\n", t.Tier, t.PricePointID, t.CustomerPrice, t.Proceeds)
	}
	return nil
}

func printTiersMarkdown(w io.Writer, tiers []shared.TierEntry) error {
	fmt.Fprintln(w, "| Tier | Price Point ID | Customer Price | Proceeds |")
	fmt.Fprintln(w, "|------|----------------|----------------|----------|")
	for _, t := range tiers {
		fmt.Fprintf(w, "| %d | %s | %s | %s |\n", t.Tier, t.PricePointID, t.CustomerPrice, t.Proceeds)
	}
	return nil
}
