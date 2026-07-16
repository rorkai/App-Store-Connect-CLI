package passtypeids

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

// PassTypeIDCertificatesCommand returns the certificates subcommand group.
func PassTypeIDCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc pass-type-ids certificates <subcommand> [flags]",
		ShortHelp:  "List pass type ID certificates.",
		LongHelp: `List pass type ID certificates.

Examples:
  asc pass-type-ids certificates list --pass-type-id "PASS_ID"
  asc pass-type-ids certificates view --pass-type-id "PASS_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PassTypeIDCertificatesListCommand(),
			PassTypeIDCertificatesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PassTypeIDCertificatesListCommand returns the certificates list subcommand.
func PassTypeIDCertificatesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	displayName := fs.String("display-name", "", "Filter by display name(s), comma-separated")
	certificateType := fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
	serialNumber := fs.String("serial-number", "", "Filter by serial number(s), comma-separated")
	ids := fs.String("id", "", "Filter by certificate ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(passTypeIDCertificatesSortList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(certificateFieldsList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pass-type-ids certificates list --pass-type-id \"PASS_ID\" [flags]",
		ShortHelp:  "List certificates for a pass type ID.",
		LongHelp: `List certificates for a pass type ID.

Examples:
  asc pass-type-ids certificates list --pass-type-id "PASS_ID"
  asc pass-type-ids certificates list --pass-type-id "PASS_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return shared.MissingRequiredUsageError()
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids certificates list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}
			if err := shared.ValidateSort(*sort, passTypeIDCertificatesSortList()...); err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}

			fieldsValue, err := normalizeCertificateFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PassTypeIDCertificatesOption{
				asc.WithPassTypeIDCertificatesLimit(*limit),
				asc.WithPassTypeIDCertificatesNextURL(*next),
			}
			displayNameValues := shared.SplitCSV(*displayName)
			if len(displayNameValues) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesFilterDisplayNames(displayNameValues))
			}
			certificateTypes := shared.SplitCSVUpper(*certificateType)
			if len(certificateTypes) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesFilterCertificateTypes(certificateTypes))
			}
			serialNumbers := shared.SplitCSV(*serialNumber)
			if len(serialNumbers) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesFilterSerialNumbers(serialNumbers))
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesFilterIDs(idsValue))
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithPassTypeIDCertificatesSort(*sort))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithPassTypeIDCertificatesLimit(200))
				firstPage, err := client.GetPassTypeIDCertificates(requestCtx, passTypeIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPassTypeIDCertificates(ctx, passTypeIDValue, asc.WithPassTypeIDCertificatesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetPassTypeIDCertificates(requestCtx, passTypeIDValue, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PassTypeIDCertificatesGetCommand returns the certificates view subcommand.
func PassTypeIDCertificatesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pass-type-ids certificates view --pass-type-id \"PASS_ID\" [flags]",
		ShortHelp:  "View certificate relationships for a pass type ID.",
		LongHelp: `View certificate relationships for a pass type ID.

Examples:
  asc pass-type-ids certificates view --pass-type-id "PASS_ID"
  asc pass-type-ids certificates view --pass-type-id "PASS_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return shared.MissingRequiredUsageError()
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids certificates view: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids certificates view: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetPassTypeIDCertificatesRelationships(requestCtx, passTypeIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates view: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPassTypeIDCertificatesRelationships(ctx, passTypeIDValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates view: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetPassTypeIDCertificatesRelationships(requestCtx, passTypeIDValue, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
