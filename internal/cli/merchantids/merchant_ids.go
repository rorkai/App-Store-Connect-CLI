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
  asc merchant-ids get --merchant-id "MERCHANT_ID"
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Filter by merchant ID identifier(s), comma-separated")
	name := fs.String("name", "", "Filter by merchant ID name(s), comma-separated")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(merchantIDSortValues, ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(merchantIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(merchantIDIncludeList(), ", "))
	certificatesLimit := fs.Int("certificates-limit", 0, "Maximum included certificates per merchant ID (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc merchant-ids list [flags]",
		ShortHelp:  "List merchant IDs.",
		LongHelp: `List merchant IDs.

Examples:
  asc merchant-ids list
  asc merchant-ids list --identifier "merchant.com.example"
  asc merchant-ids list --name "Example"
  asc merchant-ids list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("merchant-ids list: --limit must be between 1 and 200")
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("merchant-ids list: --certificates-limit must be between 1 and 50")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}
			if err := shared.ValidateSort(*sort, merchantIDSortValues...); err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}

			fieldsValue, err := normalizeMerchantIDFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}
			includeValue, err := normalizeMerchantIDInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}
			if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.MerchantIDsOption{
				asc.WithMerchantIDsLimit(*limit),
				asc.WithMerchantIDsNextURL(*next),
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

			if *paginate {
				paginateOpts := append(opts, asc.WithMerchantIDsLimit(200))
				firstPage, err := client.GetMerchantIDs(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("merchant-ids list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetMerchantIDs(ctx, asc.WithMerchantIDsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("merchant-ids list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetMerchantIDs(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("merchant-ids list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// MerchantIDsGetCommand returns the merchant IDs get subcommand.
func MerchantIDsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	merchantID := fs.String("merchant-id", "", "Merchant ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(merchantIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(merchantIDIncludeList(), ", "))
	certificatesLimit := fs.Int("certificates-limit", 0, "Maximum included certificates per merchant ID (1-50)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc merchant-ids get --merchant-id \"MERCHANT_ID\"",
		ShortHelp:  "Get a merchant ID by ID.",
		LongHelp: `Get a merchant ID by ID.

Examples:
  asc merchant-ids get --merchant-id "MERCHANT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if merchantIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --merchant-id is required")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("merchant-ids get: --certificates-limit must be between 1 and 50")
			}

			fieldsValue, err := normalizeMerchantIDFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("merchant-ids get: %w", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return fmt.Errorf("merchant-ids get: %w", err)
			}
			includeValue, err := normalizeMerchantIDInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("merchant-ids get: %w", err)
			}
			if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetMerchantID(
				requestCtx,
				merchantIDValue,
				asc.WithMerchantIDsFields(fieldsValue),
				asc.WithMerchantIDsCertificateFields(certificateFieldsValue),
				asc.WithMerchantIDsInclude(includeValue),
				asc.WithMerchantIDsCertificatesLimit(*certificatesLimit),
			)
			if err != nil {
				return fmt.Errorf("merchant-ids get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// MerchantIDsCreateCommand returns the merchant IDs create subcommand.
func MerchantIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Merchant ID identifier (e.g., merchant.com.example)")
	name := fs.String("name", "", "Merchant ID name")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc merchant-ids create --identifier \"merchant.com.example\" --name \"Example\"",
		ShortHelp:  "Create a merchant ID.",
		LongHelp: `Create a merchant ID.

Examples:
  asc merchant-ids create --identifier "merchant.com.example" --name "Example"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			identifierValue := strings.TrimSpace(*identifier)
			if identifierValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --identifier is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.MerchantIDCreateAttributes{
				Name:       nameValue,
				Identifier: identifierValue,
			}
			resp, err := client.CreateMerchantID(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("merchant-ids create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// MerchantIDsUpdateCommand returns the merchant IDs update subcommand.
func MerchantIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	merchantID := fs.String("merchant-id", "", "Merchant ID")
	name := fs.String("name", "", "Merchant ID name")
	clearName := fs.Bool("clear-name", false, "Clear the merchant ID name")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				fmt.Fprintln(os.Stderr, "Error: --merchant-id is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" && !*clearName {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
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

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// MerchantIDsDeleteCommand returns the merchant IDs delete subcommand.
func MerchantIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	merchantID := fs.String("merchant-id", "", "Merchant ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc merchant-ids delete --merchant-id \"MERCHANT_ID\" --confirm",
		ShortHelp:  "Delete a merchant ID.",
		LongHelp: `Delete a merchant ID.

Examples:
  asc merchant-ids delete --merchant-id "MERCHANT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if merchantIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --merchant-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("merchant-ids delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteMerchantID(requestCtx, merchantIDValue); err != nil {
				return fmt.Errorf("merchant-ids delete: failed to delete: %w", err)
			}

			result := &asc.MerchantIDDeleteResult{
				ID:      merchantIDValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

var merchantIDSortValues = []string{"name", "-name", "identifier", "-identifier"}

func normalizeMerchantIDFields(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, merchantIDFieldsList())
}

func normalizeMerchantIDInclude(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, merchantIDIncludeList())
}

func normalizeCertificateFields(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, certificateFieldsList())
}

func normalizeCertificateInclude(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, certificateIncludeList())
}

func normalizePassTypeIDFields(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, passTypeIDFieldsList())
}

func normalizeSelection(value, flagName string, allowed []string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	allowedSet := map[string]struct{}{}
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	for _, item := range values {
		if _, ok := allowedSet[item]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
	}

	return values, nil
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
