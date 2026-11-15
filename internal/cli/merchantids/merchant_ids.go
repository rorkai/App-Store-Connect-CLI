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

// MerchantIDsCommand returns the merchant IDs command with subcommands.
func MerchantIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("merchant-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "merchant-ids",
		ShortUsage: "asc merchant-ids <subcommand> [flags]",
		ShortHelp:  "Manage merchant IDs and certificates.",
		LongHelp: `Manage merchant IDs and certificates.

Examples:
  asc merchant-ids list
  asc merchant-ids view --merchant-id "MERCHANT_ID"
  asc merchant-ids create --identifier "merchant.com.example" --name "Example"
  asc merchant-ids update --merchant-id "MERCHANT_ID" --name "New Name"
  asc merchant-ids delete --merchant-id "MERCHANT_ID" --confirm
  asc merchant-ids certificates list --merchant-id "MERCHANT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MerchantIDsListCommand(),
			MerchantIDsGetCommand(),
			MerchantIDsCreateCommand(),
			MerchantIDsUpdateCommand(),
			MerchantIDsDeleteCommand(),
			MerchantIDsCertificatesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MerchantIDsListCommand returns the merchant IDs list subcommand.
func MerchantIDsListCommand() *ffcli.Command {
	var (
		identifier             *string
		name                   *string
		sort                   *string
		certificatesLimit      *int
		fieldsValue            []string
		certificateFieldsValue []string
		includeValue           []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc merchant-ids list [flags]",
		ShortHelp:  "List merchant IDs.",
		LongHelp: `List merchant IDs.

Examples:
  asc merchant-ids list
  asc merchant-ids list --identifier "merchant.com.example"
  asc merchant-ids list --name "Example"
  asc merchant-ids list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"identifier", "name", "sort", "fields", "certificate-fields", "include", "certificates-limit", "limit"},
		Flags: func(fs *flag.FlagSet) func() error {
			identifier = fs.String("identifier", "", "Filter by merchant ID identifier(s), comma-separated")
			name = fs.String("name", "", "Filter by merchant ID name(s), comma-separated")
			sort = fs.String("sort", "", "Sort by: "+strings.Join(merchantIDSortValues, ", "))
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(merchantIDFieldsList(), ", "))
			certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
			include := fs.String("include", "", "Include related resources: "+strings.Join(merchantIDIncludeList(), ", "))
			certificatesLimit = fs.Int("certificates-limit", 0, "Maximum included certificates per merchant ID (1-50)")
			return func() error {
				if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
					return shared.UsageError("merchant-ids list: --certificates-limit must be between 1 and 50")
				}
				if err := shared.ValidateSort(*sort, merchantIDSortValues...); err != nil {
					return shared.UsageErrorf("merchant-ids list: %v", err)
				}

				var err error
				fieldsValue, err = normalizeMerchantIDFields(*fields, "--fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids list: %v", err)
				}
				certificateFieldsValue, err = normalizeCertificateFields(*certificateFields, "--certificate-fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids list: %v", err)
				}
				includeValue, err = normalizeMerchantIDInclude(*include, "--include")
				if err != nil {
					return shared.UsageErrorf("merchant-ids list: %v", err)
				}
				if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
					return flag.ErrHelp
				}
				if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
					return flag.ErrHelp
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.MerchantIDsOption{
				asc.Limit[asc.MerchantIDsOption](limit),
				asc.NextURL[asc.MerchantIDsOption](next),
				asc.WithMerchantIDsSort(*sort),
				asc.WithMerchantIDsFields(fieldsValue),
				asc.WithMerchantIDsCertificateFields(certificateFieldsValue),
				asc.WithMerchantIDsInclude(includeValue),
				asc.WithMerchantIDsCertificatesLimit(*certificatesLimit),
			}
			if strings.TrimSpace(*identifier) != "" {
				opts = append(opts, asc.WithMerchantIDsFilterIdentifier(*identifier))
			}
			if strings.TrimSpace(*name) != "" {
				opts = append(opts, asc.WithMerchantIDsFilterName(*name))
			}
			return client.GetMerchantIDs(ctx, opts...)
		},
	})
}

// MerchantIDsGetCommand returns the merchant IDs get subcommand.
func MerchantIDsGetCommand() *ffcli.Command {
	var (
		fieldsValue, certificateFieldsValue, includeValue []string
		certificatesLimit                                 *int
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc merchant-ids view --merchant-id \"MERCHANT_ID\"",
		ShortHelp:  "View a merchant ID by ID.",
		LongHelp: `View a merchant ID by ID.

Examples:
  asc merchant-ids view --merchant-id "MERCHANT_ID"`,
		IDFlag:             "merchant-id",
		IDUsage:            "Merchant ID",
		IDType:             "merchantIds",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(merchantIDFieldsList(), ", "))
			certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
			include := fs.String("include", "", "Include related resources: "+strings.Join(merchantIDIncludeList(), ", "))
			certificatesLimit = fs.Int("certificates-limit", 0, "Maximum included certificates per merchant ID (1-50)")
			return func() (err error) {
				if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
					return shared.UsageError("merchant-ids view: --certificates-limit must be between 1 and 50")
				}
				fieldsValue, err = normalizeMerchantIDFields(*fields, "--fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids view: %v", err)
				}
				certificateFieldsValue, err = normalizeCertificateFields(*certificateFields, "--certificate-fields")
				if err != nil {
					return shared.UsageErrorf("merchant-ids view: %v", err)
				}
				includeValue, err = normalizeMerchantIDInclude(*include, "--include")
				if err != nil {
					return shared.UsageErrorf("merchant-ids view: %v", err)
				}
				if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
					return flag.ErrHelp
				}
				if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
					return flag.ErrHelp
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetMerchantID(
				ctx,
				id,
				asc.WithMerchantIDsFields(fieldsValue),
				asc.WithMerchantIDsCertificateFields(certificateFieldsValue),
				asc.WithMerchantIDsInclude(includeValue),
				asc.WithMerchantIDsCertificatesLimit(*certificatesLimit),
			)
		},
	})
}

// MerchantIDsCreateCommand returns the merchant IDs create subcommand.
func MerchantIDsCreateCommand() *ffcli.Command {
	var attrs asc.MerchantIDCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc merchant-ids create --identifier \"merchant.com.example\" --name \"Example\"",
		ShortHelp:  "Create a merchant ID.",
		LongHelp: `Create a merchant ID.

Examples:
  asc merchant-ids create --identifier "merchant.com.example" --name "Example"`,
		TopLevel: true,
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Identifier, "identifier", "Merchant ID identifier (e.g., merchant.com.example)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Merchant ID name"),
		},
		Send: func(ctx context.Context, client *asc.Client, _ string) (any, error) {
			return client.CreateMerchantID(ctx, attrs)
		},
	})
}

// MerchantIDsUpdateCommand returns the merchant IDs update subcommand.
func MerchantIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	merchantID := shared.BindResourceIDFlag(fs, "merchant-id", "merchantIds", "Merchant ID")
	name := fs.String("name", "", "Merchant ID name")
	clearName := fs.Bool("clear-name", false, "Clear the merchant ID name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc merchant-ids update --merchant-id \"MERCHANT_ID\" --name \"New Name\"",
		ShortHelp:  "Update a merchant ID.",
		LongHelp: `Update a merchant ID.

Examples:
  asc merchant-ids update --merchant-id "MERCHANT_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if merchantIDValue == "" {
				return shared.RequiredFlag("--merchant-id")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" && !*clearName {
				return shared.RequiredFlag("--name")
			}
			if nameValue != "" && *clearName {
				fmt.Fprintln(os.Stderr, "Error: --name cannot be used with --clear-name")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var name *string
			if !*clearName {
				name = &nameValue
			}
			attrs := asc.MerchantIDUpdateAttributes{Name: name}
			resp, err := client.UpdateMerchantID(requestCtx, merchantIDValue, attrs)
			if err != nil {
				return fmt.Errorf("merchant-ids update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// MerchantIDsDeleteCommand returns the merchant IDs delete subcommand.
func MerchantIDsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc merchant-ids delete --merchant-id \"MERCHANT_ID\" --confirm",
		ShortHelp:  "Delete a merchant ID.",
		LongHelp: `Delete a merchant ID.

Examples:
  asc merchant-ids delete --merchant-id "MERCHANT_ID" --confirm`,
		IDFlag:             "merchant-id",
		IDUsage:            "Merchant ID",
		IDType:             "merchantIds",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteMerchantID(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

var merchantIDSortValues = []string{"name", "-name", "identifier", "-identifier"}

func normalizeMerchantIDFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, merchantIDFieldsList(), flagName)
}

func normalizeMerchantIDInclude(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, merchantIDIncludeList(), flagName)
}

func normalizeCertificateFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, certificateFieldsList(), flagName)
}

func normalizeCertificateInclude(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, certificateIncludeList(), flagName)
}

func normalizePassTypeIDFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, passTypeIDFieldsList(), flagName)
}

func merchantIDFieldsList() []string {
	return []string{"name", "identifier", "certificates"}
}

func merchantIDIncludeList() []string {
	return []string{"certificates"}
}

func passTypeIDFieldsList() []string {
	return []string{"name", "identifier", "certificates"}
}

func certificateFieldsList() []string {
	return []string{
		"name",
		"certificateType",
		"displayName",
		"serialNumber",
		"platform",
		"expirationDate",
		"certificateContent",
		"activated",
		"passTypeId",
	}
}

func certificateIncludeList() []string {
	return []string{"passTypeId"}
}
