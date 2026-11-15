package passtypeids

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewPassTypeIDsCommand returns the pass type IDs command with subcommands.
func NewPassTypeIDsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pass-type-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pass-type-ids",
		ShortUsage: "asc pass-type-ids <subcommand> [flags]",
		ShortHelp:  "Manage pass type IDs.",
		LongHelp: `Manage pass type IDs.

Examples:
  asc pass-type-ids list
  asc pass-type-ids view --pass-type-id "PASS_ID"
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"
  asc pass-type-ids update --pass-type-id "PASS_ID" --name "New Name"
  asc pass-type-ids delete --pass-type-id "PASS_ID" --confirm
  asc pass-type-ids certificates list --pass-type-id "PASS_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPassTypeIDsListCommand(d),
			newPassTypeIDsGetCommand(d),
			newPassTypeIDsCreateCommand(d),
			newPassTypeIDsUpdateCommand(d),
			newPassTypeIDsDeleteCommand(d),
			newPassTypeIDCertificatesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPassTypeIDsListCommand returns the pass type IDs list subcommand.
func newPassTypeIDsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		ids                    *string
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
		ShortUsage: "asc pass-type-ids list [flags]",
		ShortHelp:  "List pass type IDs.",
		LongHelp: `List pass type IDs.

Examples:
  asc pass-type-ids list
  asc pass-type-ids list --id "PASS_ID"
  asc pass-type-ids list --identifier "pass.com.example"
  asc pass-type-ids list --name "Example"
  asc pass-type-ids list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"id", "identifier", "name", "sort", "fields", "certificate-fields", "include", "limit-certificates", "limit"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			ids = fs.String("id", "", "Filter by pass type ID(s), comma-separated")
			identifier = fs.String("identifier", "", "Filter by identifier (comma-separated)")
			name = fs.String("name", "", "Filter by name (comma-separated)")
			sort = fs.String("sort", "", "Sort by: "+strings.Join(passTypeIDSortList(), ", "))
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
			certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
			include := fs.String("include", "", "Include relationships: "+strings.Join(passTypeIDIncludeList(), ", "))
			certificatesLimit = fs.Int("limit-certificates", 0, "Maximum included certificates (1-50)")
			return func() error {
				if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
					return d.UsageError("pass-type-ids list: --limit-certificates must be between 1 and 50")
				}
				if err := shared.ValidateSort(*sort, passTypeIDSortList()...); err != nil {
					return d.UsageErrorf("pass-type-ids list: %v", err)
				}

				var err error
				fieldsValue, err = normalizePassTypeIDFields(*fields, "--fields")
				if err != nil {
					return d.UsageErrorf("pass-type-ids list: %v", err)
				}
				certificateFieldsValue, err = normalizeCertificateFields(*certificateFields, "--certificate-fields")
				if err != nil {
					return d.UsageErrorf("pass-type-ids list: %v", err)
				}
				includeValue, err = normalizePassTypeIDInclude(*include)
				if err != nil {
					return d.UsageErrorf("pass-type-ids list: %v", err)
				}
				if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(d.IO.ErrOut, "Error: --certificate-fields requires --include certificates")
					return flag.ErrHelp
				}
				if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
					fmt.Fprintln(d.IO.ErrOut, "Error: --limit-certificates requires --include certificates")
					return flag.ErrHelp
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.PassTypeIDsOption{
				asc.Limit[asc.PassTypeIDsOption](limit),
				asc.NextURL[asc.PassTypeIDsOption](next),
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
			return client.GetPassTypeIDs(ctx, opts...)
		},
	})
}

// newPassTypeIDsGetCommand returns the pass type IDs get subcommand.
func newPassTypeIDsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	passTypeID := shared.BindResourceIDFlag(fs, "pass-type-id", "passTypeIds", "Pass type ID")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(passTypeIDFieldsList(), ", "))
	certificateFields := fs.String("certificate-fields", "", "Certificate fields to include: "+strings.Join(certificateFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(passTypeIDIncludeList(), ", "))
	certificatesLimit := fs.Int("limit-certificates", 0, "Maximum included certificates (1-50)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pass-type-ids view --pass-type-id \"PASS_ID\"",
		ShortHelp:  "View a pass type ID by ID.",
		LongHelp: `View a pass type ID by ID.

Examples:
  asc pass-type-ids view --pass-type-id "PASS_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			if passTypeIDValue == "" {
				return d.RequiredFlag("--pass-type-id")
			}
			if *certificatesLimit != 0 && (*certificatesLimit < 1 || *certificatesLimit > 50) {
				return d.UsageError("pass-type-ids view: --limit-certificates must be between 1 and 50")
			}

			fieldsValue, err := normalizePassTypeIDFields(*fields, "--fields")
			if err != nil {
				return d.UsageErrorf("pass-type-ids view: %v", err)
			}
			certificateFieldsValue, err := normalizeCertificateFields(*certificateFields, "--certificate-fields")
			if err != nil {
				return d.UsageErrorf("pass-type-ids view: %v", err)
			}
			includeValue, err := normalizePassTypeIDInclude(*include)
			if err != nil {
				return d.UsageErrorf("pass-type-ids view: %v", err)
			}
			if len(certificateFieldsValue) > 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --certificate-fields requires --include certificates")
				return flag.ErrHelp
			}
			if *certificatesLimit != 0 && !shared.HasInclude(includeValue, "certificates") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --limit-certificates requires --include certificates")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pass-type-ids view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
				return fmt.Errorf("pass-type-ids view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPassTypeIDsCreateCommand returns the pass type IDs create subcommand.
func newPassTypeIDsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.PassTypeIDCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc pass-type-ids create --identifier \"pass.com.example\" --name \"Example\"",
		ShortHelp:  "Create a pass type ID.",
		LongHelp: `Create a pass type ID.

Examples:
  asc pass-type-ids create --identifier "pass.com.example" --name "Example"`,
		TopLevel: true,
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Identifier, "identifier", "Pass type identifier (e.g., pass.com.example)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Pass type name"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, _ string) (any, error) {
			return client.CreatePassTypeID(ctx, attrs)
		},
	})
}

// newPassTypeIDsUpdateCommand returns the pass type IDs update subcommand.
func newPassTypeIDsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	passTypeID := shared.BindResourceIDFlag(fs, "pass-type-id", "passTypeIds", "Pass type ID")
	name := fs.String("name", "", "Pass type name")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--pass-type-id")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pass-type-ids update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.PassTypeIDUpdateAttributes{
				Name: &asc.NullableString{Value: &nameValue},
			}
			resp, err := client.UpdatePassTypeID(requestCtx, passTypeIDValue, attrs)
			if err != nil {
				return fmt.Errorf("pass-type-ids update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPassTypeIDsDeleteCommand returns the pass type IDs delete subcommand.
func newPassTypeIDsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc pass-type-ids delete --pass-type-id \"PASS_ID\" --confirm",
		ShortHelp:  "Delete a pass type ID.",
		LongHelp: `Delete a pass type ID.

Examples:
  asc pass-type-ids delete --pass-type-id "PASS_ID" --confirm`,
		IDFlag:             "pass-type-id",
		IDUsage:            "Pass type ID",
		IDType:             "passTypeIds",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeletePassTypeID(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
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
