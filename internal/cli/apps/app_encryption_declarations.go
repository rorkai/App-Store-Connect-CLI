package apps

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEncryptionDeclarationsCommand returns the app-encryption-declarations command group.
func newAppEncryptionDeclarationsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "app-encryption-declarations",
		ShortUsage: "asc apps app-encryption-declarations <subcommand> [flags]",
		ShortHelp:  "List app encryption declarations for an app.",
		LongHelp: `List app encryption declarations for an app.

Examples:
  asc apps app-encryption-declarations list --id "APP_ID"
  asc apps app-encryption-declarations list --id "APP_ID" --include appEncryptionDeclarationDocument`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEncryptionDeclarationsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEncryptionDeclarationsListCommand returns the list subcommand.
func newAppEncryptionDeclarationsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		builds                                         *string
		buildLimit                                     *int
		fieldsValue, documentFieldsValue, includeValue []string
		included                                       asc.IncludedFields
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "apps app-encryption-declarations list",
		Name:        "list",
		ShortUsage:  "asc apps app-encryption-declarations list --id \"APP_ID\" [flags]",
		ShortHelp:   "List encryption declarations for an app.",
		LongHelp: `List encryption declarations for an app.

Examples:
  asc apps app-encryption-declarations list --id "APP_ID"
  asc apps app-encryption-declarations list --id "APP_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"
  asc apps app-encryption-declarations list --id "APP_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID)",
		ParentType:           "apps",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"build-id", "fields", "document-fields", "include", "build-limit"},
		Flags: func(fs *flag.FlagSet) func() error {
			next := fs.Lookup("next").Value
			builds = fs.String("build-id", "", "Filter by build IDs (comma-separated)")
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(appEncryptionDeclarationFieldsList(), ", "))
			documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(appEncryptionDeclarationDocumentFieldsList(), ", "))
			include := fs.String("include", "", "Include relationships: "+strings.Join(appEncryptionDeclarationIncludeList(), ", "))
			includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"app": "apps", "builds": "builds"})
			buildLimit = fs.Int("build-limit", 0, "Maximum included builds per declaration (1-50)")
			return func() error {
				if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
					return d.UsageError("apps app-encryption-declarations list: --build-limit must be between 1 and 50")
				}

				var err error
				fieldsValue, err = normalizeAppEncryptionDeclarationFields(*fields)
				if err != nil {
					return d.UsageErrorf("apps app-encryption-declarations list: %v", err)
				}
				documentFieldsValue, err = normalizeAppEncryptionDeclarationDocumentFields(*documentFields)
				if err != nil {
					return d.UsageErrorf("apps app-encryption-declarations list: %v", err)
				}
				includeValue, err = normalizeAppEncryptionDeclarationInclude(*include)
				if err != nil {
					return d.UsageErrorf("apps app-encryption-declarations list: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "apps app-encryption-declarations list", next.String(), includeValue)
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEncryptionDeclarations(ctx, appID,
				asc.WithAppEncryptionDeclarationsBuildIDs(shared.SplitCSV(*builds)),
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithIncludedFields[asc.AppEncryptionDeclarationsOption](included),
				asc.Limit[asc.AppEncryptionDeclarationsOption](limit),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
				asc.NextURL[asc.AppEncryptionDeclarationsOption](next),
			)
		},
	})
}

func normalizeAppEncryptionDeclarationFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range appEncryptionDeclarationFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(appEncryptionDeclarationFieldsList(), ", "))
		}
	}
	return fields, nil
}

func normalizeAppEncryptionDeclarationDocumentFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range appEncryptionDeclarationDocumentFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--document-fields must be one of: %s", strings.Join(appEncryptionDeclarationDocumentFieldsList(), ", "))
		}
	}
	return fields, nil
}

func normalizeAppEncryptionDeclarationInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, item := range appEncryptionDeclarationIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(appEncryptionDeclarationIncludeList(), ", "))
		}
	}
	return include, nil
}

func appEncryptionDeclarationFieldsList() []string {
	return []string{
		"appDescription",
		"createdDate",
		"usesEncryption",
		"exempt",
		"containsProprietaryCryptography",
		"containsThirdPartyCryptography",
		"availableOnFrenchStore",
		"platform",
		"uploadedDate",
		"documentUrl",
		"documentName",
		"documentType",
		"appEncryptionDeclarationState",
		"codeValue",
		"app",
		"builds",
		"appEncryptionDeclarationDocument",
	}
}

func appEncryptionDeclarationDocumentFieldsList() []string {
	return []string{
		"fileSize",
		"fileName",
		"assetToken",
		"downloadUrl",
		"sourceFileChecksum",
		"uploadOperations",
		"assetDeliveryState",
	}
}

func appEncryptionDeclarationIncludeList() []string {
	return []string{
		"app",
		"builds",
		"appEncryptionDeclarationDocument",
	}
}
