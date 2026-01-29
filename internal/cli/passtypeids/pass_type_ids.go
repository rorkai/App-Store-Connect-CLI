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

// PassTypeIDsCommand returns the pass type IDs command with subcommands.
func PassTypeIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pass-type-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pass-type-ids",
		ShortUsage: "asc pass-type-ids <subcommand> [flags]",
		ShortHelp:  "Manage pass type IDs and certificates.",
		LongHelp: `Manage pass type IDs and certificates.

Examples:
  asc pass-type-ids list
  asc pass-type-ids get --pass-type-id "PASS_TYPE_ID"
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"
  asc pass-type-ids update --pass-type-id "PASS_TYPE_ID" --name "New Name"
  asc pass-type-ids delete --pass-type-id "PASS_TYPE_ID" --confirm
  asc pass-type-ids certificates list --pass-type-id "PASS_TYPE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PassTypeIDsListCommand(),
			PassTypeIDsGetCommand(),
			PassTypeIDsCreateCommand(),
			PassTypeIDsUpdateCommand(),
			PassTypeIDsDeleteCommand(),
			PassTypeIDsCertificatesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PassTypeIDsListCommand returns the pass type IDs list subcommand.
func PassTypeIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	id := fs.String("id", "", "Filter by pass type ID(s), comma-separated")
	identifier := fs.String("identifier", "", "Filter by pass type ID identifier(s), comma-separated")
	name := fs.String("name", "", "Filter by pass type ID name(s), comma-separated")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(passTypeIDSortValues, ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(passTypeIDIncludeList(), ", "))
	certificatesLimit := fs.Int("certificates-limit", 0, "Maximum included certificates per pass type ID (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pass-type-ids list [flags]",
		ShortHelp:  "List pass type IDs.",
		LongHelp: `List pass type IDs.

Examples:
  asc pass-type-ids list
  asc pass-type-ids list --identifier "pass.com.example"
  asc pass-type-ids list --name "Example"
  asc pass-type-ids list --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids list: --limit must be between 1 and 200")
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("pass-type-ids list: --certificates-limit must be between 1 and 50")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}
			if err := validateSort(*sort, passTypeIDSortValues...); err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}

			fieldsValue, err := normalizePassTypeIDFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}
			includeValue, err := normalizePassTypeIDInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}
			if len(certificateFieldsValue) > 0 && !hasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && !hasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PassTypeIDsOption{
				asc.WithPassTypeIDsLimit(*limit),
				asc.WithPassTypeIDsNextURL(*next),
				asc.WithPassTypeIDsSort(*sort),
				asc.WithPassTypeIDsFields(fieldsValue),
				asc.WithPassTypeIDsCertificateFields(certificateFieldsValue),
				asc.WithPassTypeIDsInclude(includeValue),
				asc.WithPassTypeIDsCertificatesLimit(*certificatesLimit),
			}
			if strings.TrimSpace(*id) != "" {
				opts = append(opts, asc.WithPassTypeIDsFilterIDs(*id))
			}
			if strings.TrimSpace(*identifier) != "" {
				opts = append(opts, asc.WithPassTypeIDsFilterIdentifier(*identifier))
			}
			if strings.TrimSpace(*name) != "" {
				opts = append(opts, asc.WithPassTypeIDsFilterName(*name))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithPassTypeIDsLimit(200))
				firstPage, err := client.GetPassTypeIDs(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pass-type-ids list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPassTypeIDs(ctx, asc.WithPassTypeIDsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pass-type-ids list: %w", err)
				}

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetPassTypeIDs(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PassTypeIDsGetCommand returns the pass type IDs get subcommand.
func PassTypeIDsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(passTypeIDIncludeList(), ", "))
	certificatesLimit := fs.Int("certificates-limit", 0, "Maximum included certificates per pass type ID (1-50)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pass-type-ids get --pass-type-id \"PASS_TYPE_ID\"",
		ShortHelp:  "Get a pass type ID by ID.",
		LongHelp: `Get a pass type ID by ID.

Examples:
  asc pass-type-ids get --pass-type-id "PASS_TYPE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("pass-type-ids get: --certificates-limit must be between 1 and 50")
			}

			fieldsValue, err := normalizePassTypeIDFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}
			includeValue, err := normalizePassTypeIDInclude(*include, "--include")
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}
			if len(certificateFieldsValue) > 0 && !hasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificate-fields requires --include certificates")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && !hasInclude(includeValue, "certificates") {
				fmt.Fprintln(os.Stderr, "Error: --certificates-limit requires --include certificates")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetPassTypeID(
				requestCtx,
				passTypeIDValue,
				asc.WithPassTypeIDsFields(fieldsValue),
				asc.WithPassTypeIDsCertificateFields(certificateFieldsValue),
				asc.WithPassTypeIDsInclude(includeValue),
				asc.WithPassTypeIDsCertificatesLimit(*certificatesLimit),
			)
			if err != nil {
				return fmt.Errorf("pass-type-ids get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PassTypeIDsCreateCommand returns the pass type IDs create subcommand.
func PassTypeIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Pass type ID identifier (e.g., pass.com.example)")
	name := fs.String("name", "", "Pass type ID name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc pass-type-ids create --identifier \"pass.com.example\" --name \"Example\"",
		ShortHelp:  "Create a pass type ID.",
		LongHelp: `Create a pass type ID.

Examples:
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.PassTypeIDCreateAttributes{
				Name:       nameValue,
				Identifier: identifierValue,
			}
			resp, err := client.CreatePassTypeID(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("pass-type-ids create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PassTypeIDsUpdateCommand returns the pass type IDs update subcommand.
func PassTypeIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	name := fs.String("name", "", "Pass type ID name")
	clearName := fs.Bool("clear-name", false, "Clear the pass type ID name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc pass-type-ids update --pass-type-id \"PASS_TYPE_ID\" --name \"New Name\"",
		ShortHelp:  "Update a pass type ID.",
		LongHelp: `Update a pass type ID.

Examples:
  asc pass-type-ids update --pass-type-id "PASS_TYPE_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
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

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			var name *string
			if !*clearName {
				name = &nameValue
			}
			attrs := asc.PassTypeIDUpdateAttributes{Name: name}
			resp, err := client.UpdatePassTypeID(requestCtx, passTypeIDValue, attrs)
			if err != nil {
				return fmt.Errorf("pass-type-ids update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PassTypeIDsDeleteCommand returns the pass type IDs delete subcommand.
func PassTypeIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc pass-type-ids delete --pass-type-id \"PASS_TYPE_ID\" --confirm",
		ShortHelp:  "Delete a pass type ID.",
		LongHelp: `Delete a pass type ID.

Examples:
  asc pass-type-ids delete --pass-type-id "PASS_TYPE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeletePassTypeID(requestCtx, passTypeIDValue); err != nil {
				return fmt.Errorf("pass-type-ids delete: failed to delete: %w", err)
			}

			result := &asc.PassTypeIDDeleteResult{
				ID:      passTypeIDValue,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

var passTypeIDSortValues = []string{"name", "-name", "identifier", "-identifier", "id", "-id"}

func normalizePassTypeIDFields(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, passTypeIDFieldsList())
}

func normalizePassTypeIDInclude(value, flagName string) ([]string, error) {
	return normalizeSelection(value, flagName, passTypeIDIncludeList())
}

func passTypeIDFieldsList() []string {
	return []string{"name", "identifier", "certificates"}
}

func passTypeIDIncludeList() []string {
	return []string{"certificates"}
}
