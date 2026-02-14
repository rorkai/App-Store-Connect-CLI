package sandbox

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SandboxListCommand returns the sandbox list subcommand.
func SandboxListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	email := fs.String("email", "", "Filter by tester email")
	territory := fs.String("territory", "", "Filter by territory (e.g., USA, JPN)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc sandbox list [flags]",
		ShortHelp:  "List sandbox testers.",
		LongHelp: `List sandbox testers for the App Store Connect team.

Examples:
  asc sandbox list
  asc sandbox list --email "tester@example.com"
  asc sandbox list --territory "USA"
  asc sandbox list --limit 50
  asc sandbox list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("sandbox list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("sandbox list: %w", err)
			}
			if strings.TrimSpace(*email) != "" {
				if err := validateSandboxEmail(*email); err != nil {
					return fmt.Errorf("sandbox list: %w", err)
				}
			}
			normalizedTerritory, err := normalizeSandboxTerritoryFilter(*territory)
			if err != nil {
				return fmt.Errorf("sandbox list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SandboxTestersOption{
				asc.WithSandboxTestersLimit(*limit),
				asc.WithSandboxTestersNextURL(*next),
			}
			if strings.TrimSpace(*email) != "" {
				opts = append(opts, asc.WithSandboxTestersEmail(*email))
			}
			if normalizedTerritory != "" {
				opts = append(opts, asc.WithSandboxTestersTerritory(normalizedTerritory))
			}

			// Sandbox testers use a different response type - need to handle separately
			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithSandboxTestersLimit(200))
				firstPage, err := client.GetSandboxTesters(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("sandbox list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSandboxTesters(ctx, asc.WithSandboxTestersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("sandbox list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSandboxTesters(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("sandbox list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
