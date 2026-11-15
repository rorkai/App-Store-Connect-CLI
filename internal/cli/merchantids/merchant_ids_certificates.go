package merchantids

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MerchantIDsCertificatesCommand returns the merchant ID certificates command with subcommands.
func MerchantIDsCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc merchant-ids certificates <subcommand> [flags]",
		ShortHelp:  "Manage merchant ID certificates.",
		LongHelp: `Manage merchant ID certificates.

Examples:
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates view --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates create --merchant-id "MERCHANT_ID" --certificate-type APPLE_PAY_MERCHANT_IDENTITY --csr "./merchant.csr"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MerchantIDsCertificatesListCommand(),
			MerchantIDsCertificatesGetCommand(),
			MerchantIDsCertificatesCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MerchantIDsCertificatesListCommand returns the certificates list subcommand.
func MerchantIDsCertificatesListCommand() *ffcli.Command {
	var (
		displayName         *string
		certificateType     *string
		serialNumber        *string
		certificateID       *string
		sort                *string
		fieldsValue         []string
		passTypeFieldsValue []string
		includeValue        []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "certificates list",
		Name:        "list",
		ShortUsage:  "asc merchant-ids certificates list --merchant-id \"MERCHANT_ID\" [flags]",
		ShortHelp:   "List certificates for a merchant ID.",
		LongHelp: `List certificates for a merchant ID.

Examples:
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID" --paginate`,
		ParentFlag:           "merchant-id",
		ParentUsage:          "Merchant ID",
		ParentType:           "merchantIds",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"display-name", "certificate-type", "serial-number", "certificate-id", "sort", "fields", "pass-type-fields", "include"},
		Flags: func(fs *flag.FlagSet) func() error {
			displayName = fs.String("display-name", "", "Filter by certificate display name(s), comma-separated")
			certificateType = fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
			serialNumber = fs.String("serial-number", "", "Filter by certificate serial number(s), comma-separated")
			certificateID = fs.String("certificate-id", "", "Filter by certificate ID(s), comma-separated")
			sort = fs.String("sort", "", "Sort by: "+strings.Join(certificateSortValues, ", "))
			fields := fs.String("fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
			passTypeFields := fs.String("pass-type-fields", "", "Pass type fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
			include := fs.String("include", "", "Include related resources: "+strings.Join(certificateIncludeList(), ", "))
			return func() error {
				if err := shared.ValidateSort(*sort, certificateSortValues...); err != nil {
					return shared.UsageErrorf("merchant-ids certificates list: %v", err)
				}

				var err error
				fieldsValue, err = normalizeCertificateFields(*fields, "--fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids certificates list: %v", err)
				}
				passTypeFieldsValue, err = normalizePassTypeIDFields(*passTypeFields, "--pass-type-fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids certificates list: %v", err)
				}
				includeValue, err = normalizeCertificateInclude(*include, "--include")
				if err != nil {
					return shared.UsageErrorf("merchant-ids certificates list: %v", err)
				}
				if len(passTypeFieldsValue) > 0 && !shared.HasInclude(includeValue, "passTypeId") {
					fmt.Fprintln(os.Stderr, "Error: --pass-type-fields requires --include passTypeId")
					return flag.ErrHelp
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, merchantID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetMerchantIDCertificates(ctx, merchantID,
				asc.Limit[asc.MerchantIDCertificatesOption](limit),
				asc.NextURL[asc.MerchantIDCertificatesOption](next),
				asc.WithMerchantIDCertificatesFilterDisplayName(*displayName),
				asc.WithMerchantIDCertificatesFilterCertificateTypes(*certificateType),
				asc.WithMerchantIDCertificatesFilterSerialNumbers(*serialNumber),
				asc.WithMerchantIDCertificatesFilterIDs(*certificateID),
				asc.WithMerchantIDCertificatesSort(*sort),
				asc.WithMerchantIDCertificatesFields(fieldsValue),
				asc.WithMerchantIDCertificatesPassTypeFields(passTypeFieldsValue),
				asc.WithMerchantIDCertificatesInclude(includeValue),
			)
		},
	})
}

// MerchantIDsCertificatesCreateCommand returns the certificates create
// subcommand. The implementation lives in the certificates package so both
// create paths share one CSR and relationship implementation.
func MerchantIDsCertificatesCreateCommand() *ffcli.Command {
	return certificates.MerchantIDCertificatesCreateCommand()
}

// MerchantIDsCertificatesGetCommand returns the certificates relationships get subcommand.
func MerchantIDsCertificatesGetCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "certificates view",
		Name:        "view",
		ShortUsage:  "asc merchant-ids certificates view --merchant-id \"MERCHANT_ID\" [flags]",
		ShortHelp:   "View certificate relationships for a merchant ID.",
		LongHelp: `View certificate relationships for a merchant ID.

Examples:
  asc merchant-ids certificates view --merchant-id "MERCHANT_ID"
  asc merchant-ids certificates view --merchant-id "MERCHANT_ID" --paginate`,
		ParentFlag:           "merchant-id",
		ParentUsage:          "Merchant ID",
		ParentType:           "merchantIds",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, merchantID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetMerchantIDCertificatesRelationships(ctx, merchantID, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
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
