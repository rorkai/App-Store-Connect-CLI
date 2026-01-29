package passtypeids

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PassTypeIDsCertificatesCommand returns the pass type ID certificates command with subcommands.
func PassTypeIDsCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc pass-type-ids certificates <subcommand> [flags]",
		ShortHelp:  "List pass type ID certificates.",
		LongHelp: `List pass type ID certificates.

Examples:
  asc pass-type-ids certificates list --pass-type-id "PASS_TYPE_ID"
  asc pass-type-ids certificates get --pass-type-id "PASS_TYPE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PassTypeIDsCertificatesListCommand(),
			PassTypeIDsCertificatesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PassTypeIDsCertificatesListCommand returns the certificates list subcommand.
func PassTypeIDsCertificatesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates list", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	displayName := fs.String("display-name", "", "Filter by certificate display name(s), comma-separated")
	certificateType := fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
	serialNumber := fs.String("serial-number", "", "Filter by certificate serial number(s), comma-separated")
	certificateID := fs.String("certificate-id", "", "Filter by certificate ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(certificateSortValues(), ", "))
	fields := fs.String("fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	passTypeFields := fs.String("pass-type-fields", "", "Pass type fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(certificateIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pass-type-ids certificates list --pass-type-id \"PASS_TYPE_ID\" [flags]",
		ShortHelp:  "List certificates for a pass type ID.",
		LongHelp: `List certificates for a pass type ID.

Examples:
  asc pass-type-ids certificates list --pass-type-id "PASS_TYPE_ID"
  asc pass-type-ids certificates list --pass-type-id "PASS_TYPE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids certificates list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}
			if err := validateSort(*sort, certificateSortValues()...); err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}

			fieldsValue, err := normalizeCertificateFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}
			passTypeFieldsValue, err := normalizePassTypeIDFields(*passTypeFields, "--pass-type-fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}
			includeValue, err := normalizeCertificateInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}
			if len(passTypeFieldsValue) > 0 && !hasInclude(includeValue, "passTypeId") {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-fields requires --include passTypeId")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PassTypeIDCertificatesOption{
				asc.WithPassTypeIDCertificatesLimit(*limit),
				asc.WithPassTypeIDCertificatesNextURL(*next),
				asc.WithPassTypeIDCertificatesFilterDisplayName(*displayName),
				asc.WithPassTypeIDCertificatesFilterCertificateTypes(*certificateType),
				asc.WithPassTypeIDCertificatesFilterSerialNumbers(*serialNumber),
				asc.WithPassTypeIDCertificatesFilterIDs(*certificateID),
				asc.WithPassTypeIDCertificatesSort(*sort),
				asc.WithPassTypeIDCertificatesFields(fieldsValue),
				asc.WithPassTypeIDCertificatesPassTypeFields(passTypeFieldsValue),
				asc.WithPassTypeIDCertificatesInclude(includeValue),
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

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetPassTypeIDCertificates(requestCtx, passTypeIDValue, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PassTypeIDsCertificatesGetCommand returns the certificates relationships get subcommand.
func PassTypeIDsCertificatesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates get", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pass-type-ids certificates get --pass-type-id \"PASS_TYPE_ID\" [flags]",
		ShortHelp:  "Get certificate relationships for a pass type ID.",
		LongHelp: `Get certificate relationships for a pass type ID.

Examples:
  asc pass-type-ids certificates get --pass-type-id "PASS_TYPE_ID"
  asc pass-type-ids certificates get --pass-type-id "PASS_TYPE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids certificates get: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids certificates get: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetPassTypeIDCertificatesRelationships(requestCtx, passTypeIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates get: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPassTypeIDCertificatesRelationships(ctx, passTypeIDValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pass-type-ids certificates get: %w", err)
				}

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetPassTypeIDCertificatesRelationships(requestCtx, passTypeIDValue, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids certificates get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
