package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsCIProductCommand returns the ci-product command group.
func AppsCIProductCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ci-product", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "ci-product",
		ShortUsage: "asc apps ci-product <subcommand> [flags]",
		ShortHelp:  "View the CI product for an app.",
		LongHelp: `View the CI product for an app.

Examples:
  asc apps ci-product get --id "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsCIProductGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsCIProductGetCommand returns the ci-product get subcommand.
func AppsCIProductGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "App Store Connect app ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc apps ci-product get --id \"APP_ID\"",
		ShortHelp:  "Get the CI product for an app.",
		LongHelp: `Get the CI product for an app.

Examples:
  asc apps ci-product get --id "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps ci-product get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCiProduct(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("apps ci-product get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
