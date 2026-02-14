package merchantids

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

// MerchantIDsCertificatesCommand returns the merchant ID certificates command with subcommands.
func MerchantIDsCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc merchant-ids certificates <subcommand> [flags]",
		ShortHelp:  "List merchant ID certificates.",
		LongHelp: `List merchant ID certificates.

Examples:
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates get --merchant-id "MERCHANT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MerchantIDsCertificatesListCommand(),
			MerchantIDsCertificatesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MerchantIDsCertificatesListCommand returns the certificates list subcommand.
func MerchantIDsCertificatesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates list", flag.ExitOnError)

	merchantID := fs.String("merchant-id", "", "Merchant ID")
	displayName := fs.String("display-name", "", "Filter by certificate display name(s), comma-separated")
	certificateType := fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
	serialNumber := fs.String("serial-number", "", "Filter by certificate serial number(s), comma-separated")
	certificateID := fs.String("certificate-id", "", "Filter by certificate ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(certificateSortValues, ", "))
	fields := fs.String("fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	passTypeFields := fs.String("pass-type-fields", "", "Pass type fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(certificateIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc merchant-ids certificates list --merchant-id \"MERCHANT_ID\" [flags]",
		ShortHelp:  "List certificates for a merchant ID.",
		LongHelp: `List certificates for a merchant ID.

Examples:
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if merchantIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --merchant-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("merchant-ids certificates list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}
			if err := shared.ValidateSort(*sort, certificateSortValues...); err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}

			fieldsValue, err := normalizeCertificateFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}
			passTypeFieldsValue, err := normalizePassTypeIDFields(*passTypeFields, "--pass-type-fields")
			if err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}
			includeValue, err := normalizeCertificateInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}
			if len(passTypeFieldsValue) > 0 && !shared.HasInclude(includeValue, "passTypeId") {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-fields requires --include passTypeId")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids certificates list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.MerchantIDCertificatesOption{
				asc.WithMerchantIDCertificatesLimit(*limit),
				asc.WithMerchantIDCertificatesNextURL(*next),
				asc.WithMerchantIDCertificatesFilterDisplayName(*displayName),
				asc.WithMerchantIDCertificatesFilterCertificateTypes(*certificateType),
				asc.WithMerchantIDCertificatesFilterSerialNumbers(*serialNumber),
				asc.WithMerchantIDCertificatesFilterIDs(*certificateID),
				asc.WithMerchantIDCertificatesSort(*sort),
				asc.WithMerchantIDCertificatesFields(fieldsValue),
				asc.WithMerchantIDCertificatesPassTypeFields(passTypeFieldsValue),
				asc.WithMerchantIDCertificatesInclude(includeValue),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithMerchantIDCertificatesLimit(200))
				firstPage, err := client.GetMerchantIDCertificates(requestCtx, merchantIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("merchant-ids certificates list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetMerchantIDCertificates(ctx, merchantIDValue, asc.WithMerchantIDCertificatesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("merchant-ids certificates list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetMerchantIDCertificates(requestCtx, merchantIDValue, opts...)
			if err != nil {
				return fmt.Errorf("merchant-ids certificates list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// MerchantIDsCertificatesGetCommand returns the certificates relationships get subcommand.
func MerchantIDsCertificatesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates get", flag.ExitOnError)

	merchantID := fs.String("merchant-id", "", "Merchant ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc merchant-ids certificates get --merchant-id \"MERCHANT_ID\" [flags]",
		ShortHelp:  "Get certificate relationships for a merchant ID.",
		LongHelp: `Get certificate relationships for a merchant ID.

Examples:
  asc merchant-ids certificates get --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates get --merchant-id "MERCHANT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if merchantIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --merchant-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("merchant-ids certificates get: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("merchant-ids certificates get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids certificates get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetMerchantIDCertificatesRelationships(requestCtx, merchantIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("merchant-ids certificates get: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetMerchantIDCertificatesRelationships(ctx, merchantIDValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("merchant-ids certificates get: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetMerchantIDCertificatesRelationships(requestCtx, merchantIDValue, opts...)
			if err != nil {
				return fmt.Errorf("merchant-ids certificates get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

var certificateSortValues = []string{
	"displayName",
	"-displayName",
	"certificateType",
	"-certificateType",
	"serialNumber",
	"-serialNumber",
	"id",
	"-id",
}
