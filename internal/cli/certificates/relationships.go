package certificates

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCertificatesRelationshipsCommand returns the links command group.
func newCertificatesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc certificates links <subcommand> [flags]",
		ShortHelp:  "View certificate relationship linkages.",
		LongHelp: `View certificate relationship linkages.

Examples:
  asc certificates links pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCertificatesRelationshipsPassTypeIDCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCertificatesRelationshipsPassTypeIDCommand returns the pass-type-id links command.
func newCertificatesRelationshipsPassTypeIDCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pass-type-id", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "certificates", "Certificate ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pass-type-id",
		ShortUsage: "asc certificates links pass-type-id --id \"CERT_ID\"",
		ShortHelp:  "Get pass type ID relationship for a certificate.",
		LongHelp: `Get pass type ID relationship for a certificate.

Examples:
  asc certificates links pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("certificates links pass-type-id: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetCertificatePassTypeIDRelationship(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("certificates links pass-type-id: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
