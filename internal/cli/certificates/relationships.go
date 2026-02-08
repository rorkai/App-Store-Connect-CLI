package certificates

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// CertificatesRelationshipsCommand returns the relationships command group.
func CertificatesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc certificates relationships <subcommand> [flags]",
		ShortHelp:  "View certificate relationship linkages.",
		LongHelp: `View certificate relationship linkages.

Examples:
  asc certificates relationships pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CertificatesRelationshipsPassTypeIDCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CertificatesRelationshipsPassTypeIDCommand returns the pass-type-id relationship command.
func CertificatesRelationshipsPassTypeIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pass-type-id", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "pass-type-id",
		ShortUsage: "asc certificates relationships pass-type-id --id \"CERT_ID\"",
		ShortHelp:  "Get pass type ID relationship for a certificate.",
		LongHelp: `Get pass type ID relationship for a certificate.

Examples:
  asc certificates relationships pass-type-id --id "CERT_ID"`,
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
				return fmt.Errorf("certificates relationships pass-type-id: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetCertificatePassTypeIDRelationship(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("certificates relationships pass-type-id: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
