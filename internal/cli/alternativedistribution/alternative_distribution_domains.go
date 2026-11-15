package alternativedistribution

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAlternativeDistributionDomainsCommand returns the domains command group.
func newAlternativeDistributionDomainsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("domains", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "domains",
		ShortUsage: "asc alternative-distribution domains <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution domains.",
		LongHelp: `Manage alternative distribution domains.

Examples:
  asc alternative-distribution domains list
  asc alternative-distribution domains view --domain-id "DOMAIN_ID"
  asc alternative-distribution domains create --domain "example.com" --reference-name "Example"
  asc alternative-distribution domains delete --domain-id "DOMAIN_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAlternativeDistributionDomainsListCommand(d),
			newAlternativeDistributionDomainsGetCommand(d),
			newAlternativeDistributionDomainsCreateCommand(d),
			newAlternativeDistributionDomainsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAlternativeDistributionDomainsListCommand returns the domains list subcommand.
func newAlternativeDistributionDomainsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:       d,
		Name:       "list",
		ShortUsage: "asc alternative-distribution domains list [flags]",
		ShortHelp:  "List alternative distribution domains.",
		LongHelp: `List alternative distribution domains.

Examples:
  asc alternative-distribution domains list
  asc alternative-distribution domains list --limit 50
  asc alternative-distribution domains list --paginate`,
		TopLevel:             true,
		LimitMax:             alternativeDistributionMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAlternativeDistributionDomains(ctx,
				asc.Limit[asc.AlternativeDistributionDomainsOption](limit),
				asc.NextURL[asc.AlternativeDistributionDomainsOption](next),
			)
		},
	})
}

// newAlternativeDistributionDomainsGetCommand returns the domains get subcommand.
func newAlternativeDistributionDomainsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc alternative-distribution domains view --domain-id \"DOMAIN_ID\"",
		ShortHelp:  "View an alternative distribution domain.",
		LongHelp: `View an alternative distribution domain.

Examples:
  asc alternative-distribution domains view --domain-id "DOMAIN_ID"`,
		IDFlag:             "domain-id",
		IDUsage:            "Alternative distribution domain ID",
		IDType:             "alternativeDistributionDomains",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAlternativeDistributionDomain(ctx, id)
		},
	})
}

// newAlternativeDistributionDomainsCreateCommand returns the domains create subcommand.
func newAlternativeDistributionDomainsCreateCommand(d *shared.Deps) *ffcli.Command {
	var domain, referenceName string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc alternative-distribution domains create --domain \"example.com\" --reference-name \"Example\"",
		ShortHelp:  "Create an alternative distribution domain.",
		LongHelp: `Create an alternative distribution domain.

Examples:
  asc alternative-distribution domains create --domain "example.com" --reference-name "Example"`,
		TopLevel: true,
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&domain, "domain", "Domain name"),
			shared.RequiredStringFlag(&referenceName, "reference-name", "Reference name for the domain"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, _ string) (any, error) {
			return client.CreateAlternativeDistributionDomain(ctx, domain, referenceName)
		},
	})
}

// newAlternativeDistributionDomainsDeleteCommand returns the domains delete subcommand.
func newAlternativeDistributionDomainsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc alternative-distribution domains delete --domain-id \"DOMAIN_ID\" --confirm",
		ShortHelp:  "Delete an alternative distribution domain.",
		LongHelp: `Delete an alternative distribution domain.

Examples:
  asc alternative-distribution domains delete --domain-id "DOMAIN_ID" --confirm`,
		IDFlag:             "domain-id",
		IDUsage:            "Alternative distribution domain ID",
		IDType:             "alternativeDistributionDomains",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAlternativeDistributionDomain(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
