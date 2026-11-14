package productpages

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const productPagesMaxLimit = 200

// NewProductPagesCommand returns the product pages command group built on d.
func NewProductPagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("product-pages", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "product-pages",
		ShortUsage: "asc product-pages <subcommand> [flags]",
		ShortHelp:  "Manage custom product pages and product page experiments.",
		LongHelp: `Manage custom product pages and product page optimization experiments.

Examples:
  asc product-pages custom-pages list --app "APP_ID"
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"
  asc product-pages experiments list --version-id "VERSION_ID"
  asc product-pages experiments create --version-id "VERSION_ID" --name "Icon Test" --traffic-proportion 25`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCustomPagesCommand(d),
			newExperimentsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
