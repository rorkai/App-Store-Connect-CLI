package agreements

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewAgreementsCommand returns the agreements command group built on d.
func NewAgreementsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("agreements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "agreements",
		ShortUsage: "asc agreements <subcommand> [flags]",
		ShortHelp:  "Manage agreements in App Store Connect.",
		LongHelp: `Manage agreements in App Store Connect.

Examples:
  asc agreements territories list --id "EULA_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAgreementsTerritoriesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAgreementsTerritoriesCommand returns the agreements territories command
// group built on d.
func newAgreementsTerritoriesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("territories", flag.ExitOnError)

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
			newAgreementsTerritoriesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAgreementsTerritoriesListCommand returns the agreements territories list
// subcommand built on d.
func newAgreementsTerritoriesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "endUserLicenseAgreements", "EULA ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("agreements territories list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("agreements territories list: %v", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractEULATerritoryIDFromNextURL(*next)
				if err != nil {
					return d.UsageErrorf("agreements territories list: %v", err)
				}
				idValue = derivedID
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("agreements territories list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.EndUserLicenseAgreementTerritoriesOption{
				asc.Limit[asc.EndUserLicenseAgreementTerritoriesOption](*limit),
				asc.NextURL[asc.EndUserLicenseAgreementTerritoriesOption](*next),
			}

			if *paginate {
				if idValue == "" {
					return d.RequiredFlag("--id")
				}
				paginateOpts := append(opts, asc.Limit[asc.EndUserLicenseAgreementTerritoriesOption](200))
				firstPage, err := client.GetEndUserLicenseAgreementTerritories(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("agreements territories list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetEndUserLicenseAgreementTerritories(ctx, idValue, asc.NextURL[asc.EndUserLicenseAgreementTerritoriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("agreements territories list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetEndUserLicenseAgreementTerritories(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("agreements territories list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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
