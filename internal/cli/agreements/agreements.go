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
		UsageFunc: d.DefaultUsageFunc,
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
		UsageFunc: d.DefaultUsageFunc,
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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc agreements territories list --id \"EULA_ID\" [flags]",
		ShortHelp:  "List territories for an EULA.",
		LongHelp: `List territories for an EULA.

Examples:
  asc agreements territories list --id "EULA_ID"
  asc agreements territories list --id "EULA_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "EULA ID",
		ParentType:           "endUserLicenseAgreements",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			id, next := fs.Lookup("id").Value, fs.Lookup("next").Value
			return func() error {
				if strings.TrimSpace(id.String()) != "" || strings.TrimSpace(next.String()) == "" {
					return nil
				}
				if _, err := extractEULATerritoryIDFromNextURL(next.String()); err != nil {
					return d.UsageErrorf("agreements territories list: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetEndUserLicenseAgreementTerritories(ctx, id,
				asc.Limit[asc.EndUserLicenseAgreementTerritoriesOption](limit),
				asc.NextURL[asc.EndUserLicenseAgreementTerritoriesOption](next),
			)
		},
	})
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
