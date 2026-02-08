package alternativedistribution

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AlternativeDistributionDomainsCommand returns the domains command group.
func AlternativeDistributionDomainsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("domains", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "domains",
		ShortUsage: "asc alternative-distribution domains <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution domains.",
		LongHelp: `Manage alternative distribution domains.

Examples:
  asc alternative-distribution domains list
  asc alternative-distribution domains get --domain-id "DOMAIN_ID"
  asc alternative-distribution domains create --domain "example.com" --reference-name "Example"
  asc alternative-distribution domains delete --domain-id "DOMAIN_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AlternativeDistributionDomainsListCommand(),
			AlternativeDistributionDomainsGetCommand(),
			AlternativeDistributionDomainsCreateCommand(),
			AlternativeDistributionDomainsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AlternativeDistributionDomainsListCommand returns the domains list subcommand.
func AlternativeDistributionDomainsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc alternative-distribution domains list [flags]",
		ShortHelp:  "List alternative distribution domains.",
		LongHelp: `List alternative distribution domains.

Examples:
  asc alternative-distribution domains list
  asc alternative-distribution domains list --limit 50
  asc alternative-distribution domains list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return fmt.Errorf("alternative-distribution domains list: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("alternative-distribution domains list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution domains list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionDomainsOption{
				asc.WithAlternativeDistributionDomainsLimit(*limit),
				asc.WithAlternativeDistributionDomainsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAlternativeDistributionDomainsLimit(alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionDomains(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution domains list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionDomains(ctx, asc.WithAlternativeDistributionDomainsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution domains list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAlternativeDistributionDomains(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution domains list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AlternativeDistributionDomainsGetCommand returns the domains get subcommand.
func AlternativeDistributionDomainsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	domainID := fs.String("domain-id", "", "Alternative distribution domain ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc alternative-distribution domains get --domain-id \"DOMAIN_ID\"",
		ShortHelp:  "Get an alternative distribution domain.",
		LongHelp: `Get an alternative distribution domain.

Examples:
  asc alternative-distribution domains get --domain-id "DOMAIN_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*domainID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --domain-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution domains get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionDomain(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution domains get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AlternativeDistributionDomainsCreateCommand returns the domains create subcommand.
func AlternativeDistributionDomainsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	domain := fs.String("domain", "", "Domain name")
	referenceName := fs.String("reference-name", "", "Reference name for the domain")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc alternative-distribution domains create --domain \"example.com\" --reference-name \"Example\"",
		ShortHelp:  "Create an alternative distribution domain.",
		LongHelp: `Create an alternative distribution domain.

Examples:
  asc alternative-distribution domains create --domain "example.com" --reference-name "Example"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			domainValue := strings.TrimSpace(*domain)
			if domainValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --domain is required")
				return flag.ErrHelp
			}

			referenceValue := strings.TrimSpace(*referenceName)
			if referenceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution domains create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAlternativeDistributionDomain(requestCtx, domainValue, referenceValue)
			if err != nil {
				return fmt.Errorf("alternative-distribution domains create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AlternativeDistributionDomainsDeleteCommand returns the domains delete subcommand.
func AlternativeDistributionDomainsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	domainID := fs.String("domain-id", "", "Alternative distribution domain ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc alternative-distribution domains delete --domain-id \"DOMAIN_ID\" --confirm",
		ShortHelp:  "Delete an alternative distribution domain.",
		LongHelp: `Delete an alternative distribution domain.

Examples:
  asc alternative-distribution domains delete --domain-id "DOMAIN_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*domainID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --domain-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution domains delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAlternativeDistributionDomain(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("alternative-distribution domains delete: failed to delete: %w", err)
			}

			result := &asc.AlternativeDistributionDomainDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
