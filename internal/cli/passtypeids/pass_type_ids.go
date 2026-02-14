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

// PassTypeIDsCommand returns the pass type IDs command with subcommands.
func PassTypeIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pass-type-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pass-type-ids",
		ShortUsage: "asc pass-type-ids <subcommand> [flags]",
		ShortHelp:  "Manage pass type IDs.",
		LongHelp: `Manage pass type IDs.

Examples:
  asc pass-type-ids list
  asc pass-type-ids get --pass-type-id "PASS_ID"
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"
  asc pass-type-ids update --pass-type-id "PASS_ID" --name "New Name"
  asc pass-type-ids delete --pass-type-id "PASS_ID" --confirm
  asc pass-type-ids certificates list --pass-type-id "PASS_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PassTypeIDsListCommand(),
			PassTypeIDsGetCommand(),
			PassTypeIDsCreateCommand(),
			PassTypeIDsUpdateCommand(),
			PassTypeIDsDeleteCommand(),
			PassTypeIDCertificatesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PassTypeIDsListCommand returns the pass type IDs list subcommand.
func PassTypeIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	ids := fs.String("id", "", "Filter by pass type ID(s), comma-separated")
	identifier := fs.String("identifier", "", "Filter by identifier (comma-separated)")
	name := fs.String("name", "", "Filter by name (comma-separated)")
	sort := fs.String("sort", "", "Sort by: "+strings.Join(passTypeIDSortList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(passTypeIDIncludeList(), ", "))
	certificatesLimit := fs.Int("limit-certificates", 0, "Maximum included certificates (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pass-type-ids list [flags]",
		ShortHelp:  "List pass type IDs.",
		LongHelp: `List pass type IDs.

Examples:
  asc pass-type-ids list
  asc pass-type-ids list --id "PASS_ID"
  asc pass-type-ids list --identifier "pass.com.example"
  asc pass-type-ids list --name "Example"
  asc pass-type-ids list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pass-type-ids list: --limit must be between 1 and 200")
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("pass-type-ids list: --limit-certificates must be between 1 and 50")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}
			if err := shared.ValidateSort(*sort, passTypeIDSortList()...); err != nil {
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
			includeValue, err := normalizePassTypeIDInclude(*include)
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PassTypeIDsOption{
				asc.WithPassTypeIDsLimit(*limit),
				asc.WithPassTypeIDsNextURL(*next),
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDsFilterIDs(idsValue))
			}
			if strings.TrimSpace(*identifier) != "" {
				opts = append(opts, asc.WithPassTypeIDsFilterIdentifier(*identifier))
			}
			if strings.TrimSpace(*name) != "" {
				opts = append(opts, asc.WithPassTypeIDsFilterName(*name))
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithPassTypeIDsSort(*sort))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDsFields(fieldsValue))
			}
			if len(certificateFieldsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDsCertificateFields(certificateFieldsValue))
			}
			if len(includeValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDsInclude(includeValue))
			}
			if *certificatesLimit > 0 {
				opts = append(opts, asc.WithPassTypeIDsCertificatesLimit(*certificatesLimit))
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

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetPassTypeIDs(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PassTypeIDsGetCommand returns the pass type IDs get subcommand.
func PassTypeIDsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(passTypeIDIncludeList(), ", "))
	certificatesLimit := fs.Int("limit-certificates", 0, "Maximum included certificates (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pass-type-ids get --pass-type-id \"PASS_ID\"",
		ShortHelp:  "Get a pass type ID by ID.",
		LongHelp: `Get a pass type ID by ID.

Examples:
  asc pass-type-ids get --pass-type-id "PASS_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return fmt.Errorf("pass-type-ids get: --limit-certificates must be between 1 and 50")
			}

			fieldsValue, err := normalizePassTypeIDFields(*fields, "--fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}
			includeValue, err := normalizePassTypeIDInclude(*include)
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PassTypeIDOption{}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDFields(fieldsValue))
			}
			if len(certificateFieldsValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificateFields(certificateFieldsValue))
			}
			if len(includeValue) > 0 {
				opts = append(opts, asc.WithPassTypeIDInclude(includeValue))
			}
			if *certificatesLimit > 0 {
				opts = append(opts, asc.WithPassTypeIDCertificatesIncludeLimit(*certificatesLimit))
			}

			resp, err := client.GetPassTypeID(requestCtx, passTypeIDValue, opts...)
			if err != nil {
				return fmt.Errorf("pass-type-ids get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PassTypeIDsCreateCommand returns the pass type IDs create subcommand.
func PassTypeIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Pass type identifier (e.g., pass.com.example)")
	name := fs.String("name", "", "Pass type name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc pass-type-ids create --identifier \"pass.com.example\" --name \"Example\"",
		ShortHelp:  "Create a pass type ID.",
		LongHelp: `Create a pass type ID.

Examples:
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"`,
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
				return fmt.Errorf("pass-type-ids create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.PassTypeIDCreateAttributes{
				Name:       nameValue,
				Identifier: identifierValue,
			}
			resp, err := client.CreatePassTypeID(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("pass-type-ids create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PassTypeIDsUpdateCommand returns the pass type IDs update subcommand.
func PassTypeIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	name := fs.String("name", "", "Pass type name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc pass-type-ids update --pass-type-id \"PASS_ID\" --name \"New Name\"",
		ShortHelp:  "Update a pass type ID.",
		LongHelp: `Update a pass type ID.

Examples:
  asc pass-type-ids update --pass-type-id "PASS_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --pass-type-id is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.PassTypeIDUpdateAttributes{
				Name: &asc.NullableString{Value: &nameValue},
			}
			resp, err := client.UpdatePassTypeID(requestCtx, passTypeIDValue, attrs)
			if err != nil {
				return fmt.Errorf("pass-type-ids update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PassTypeIDsDeleteCommand returns the pass type IDs delete subcommand.
func PassTypeIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	passTypeID := fs.String("pass-type-id", "", "Pass type ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc pass-type-ids delete --pass-type-id \"PASS_ID\" --confirm",
		ShortHelp:  "Delete a pass type ID.",
		LongHelp: `Delete a pass type ID.

Examples:
  asc pass-type-ids delete --pass-type-id "PASS_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pass-type-ids delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeletePassTypeID(requestCtx, passTypeIDValue); err != nil {
				return fmt.Errorf("pass-type-ids delete: failed to delete: %w", err)
			}

			result := &asc.PassTypeIDDeleteResult{
				ID:      passTypeIDValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizePassTypeIDFields(value, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range passTypeIDFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(passTypeIDFieldsList(), ", "))
		}
	}
	return fields, nil
}

func normalizeCertificateFields(value, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range certificateFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(certificateFieldsList(), ", "))
		}
	}
	return fields, nil
}

func normalizePassTypeIDInclude(value string) ([]string, error) {
	return shared.NormalizeSelection(value, passTypeIDIncludeList(), "--include")
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

func passTypeIDIncludeList() []string {
	return []string{"certificates"}
}

func passTypeIDSortList() []string {
	return []string{"name", "-name", "identifier", "-identifier", "id", "-id"}
}

func passTypeIDCertificatesSortList() []string {
	return []string{
		"displayName",
		"-displayName",
		"certificateType",
		"-certificateType",
		"serialNumber",
		"-serialNumber",
		"id",
		"-id",
	}
}
