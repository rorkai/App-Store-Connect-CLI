package agreements

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AgreementsCommand returns the agreements command group.
func AgreementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("agreements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "agreements",
		ShortUsage: "asc agreements <subcommand> [flags]",
		ShortHelp:  "Manage App Store Connect agreements.",
		LongHelp: `Manage App Store Connect agreements.

Examples:
  asc agreements territories list --id "EULA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AgreementsTerritoriesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AgreementsTerritoriesCommand returns the agreements territories command group.
func AgreementsTerritoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("territories", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "territories",
		ShortUsage: "asc agreements territories <subcommand> [flags]",
		ShortHelp:  "List EULA territories.",
		LongHelp: `List EULA territories.

Examples:
  asc agreements territories list --id "EULA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AgreementsTerritoriesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AgreementsTerritoriesListCommand returns the agreements territories list subcommand.
func AgreementsTerritoriesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	id := fs.String("id", "", "EULA ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc agreements territories list --id \"EULA_ID\" [flags]",
		ShortHelp:  "List territories for an EULA.",
		LongHelp: `List territories for an EULA.

Examples:
  asc agreements territories list --id "EULA_ID"
  asc agreements territories list --id "EULA_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("agreements territories list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("agreements territories list: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractEULATerritoryIDFromNextURL(*next)
				if err != nil {
					return fmt.Errorf("agreements territories list: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("agreements territories list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.EndUserLicenseAgreementTerritoriesOption{
				asc.WithEndUserLicenseAgreementTerritoriesLimit(*limit),
				asc.WithEndUserLicenseAgreementTerritoriesNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithEndUserLicenseAgreementTerritoriesLimit(200))
				firstPage, err := client.GetEndUserLicenseAgreementTerritories(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("agreements territories list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetEndUserLicenseAgreementTerritories(ctx, idValue, asc.WithEndUserLicenseAgreementTerritoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("agreements territories list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetEndUserLicenseAgreementTerritories(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("agreements territories list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func extractEULATerritoryIDFromNextURL(nextURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid --next URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "endUserLicenseAgreements" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if parts[3] == "territories" {
		return parts[2], nil
	}
	if len(parts) >= 5 && parts[3] == "relationships" && parts[4] == "territories" {
		return parts[2], nil
	}
	return "", fmt.Errorf("invalid --next URL")
}
