package apps

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsCIProductCommand returns the ci-product command group.
func newAppsCIProductCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("ci-product", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "ci-product",
		ShortUsage: "asc apps ci-product <subcommand> [flags]",
		ShortHelp:  "View the CI product for an app.",
		LongHelp: `View the CI product for an app.

Examples:
  asc apps ci-product view --id "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppsCIProductGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppsCIProductGetCommand returns the ci-product get subcommand.
func newAppsCIProductGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc apps ci-product view --id \"APP_ID\"",
		ShortHelp:  "View the CI product for an app.",
		LongHelp: `View the CI product for an app.

Examples:
  asc apps ci-product view --id "APP_ID"`,
		IDUsage:            "App Store Connect app ID",
		IDType:             "apps",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppCiProduct(ctx, id)
		},
	})
}
