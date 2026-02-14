package apps

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

// AppEncryptionDeclarationsCommand returns the app-encryption-declarations command group.
func AppEncryptionDeclarationsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "app-encryption-declarations",
		ShortUsage: "asc apps app-encryption-declarations <subcommand> [flags]",
		ShortHelp:  "List app encryption declarations for an app.",
		LongHelp: `List app encryption declarations for an app.

Examples:
  asc apps app-encryption-declarations list --id "APP_ID"
  asc apps app-encryption-declarations list --id "APP_ID" --include appEncryptionDeclarationDocument`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEncryptionDeclarationsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEncryptionDeclarationsListCommand returns the list subcommand.
func AppEncryptionDeclarationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps app-encryption-declarations list", flag.ExitOnError)

	appID := fs.String("id", "", "App Store Connect app ID (or ASC_APP_ID)")
	builds := fs.String("build", "", "Filter by build IDs (comma-separated)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(appEncryptionDeclarationFieldsList(), ", "))
	documentFields := fs.String("document-fields", "", "Document fields to include: "+strings.Join(appEncryptionDeclarationDocumentFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(appEncryptionDeclarationIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	buildLimit := fs.Int("build-limit", 0, "Maximum included builds per declaration (1-50)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps app-encryption-declarations list --id \"APP_ID\" [flags]",
		ShortHelp:  "List encryption declarations for an app.",
		LongHelp: `List encryption declarations for an app.

Examples:
  asc apps app-encryption-declarations list --id "APP_ID"
  asc apps app-encryption-declarations list --id "APP_ID" --include appEncryptionDeclarationDocument --document-fields "fileName,fileSize"
  asc apps app-encryption-declarations list --id "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("apps app-encryption-declarations list: --limit must be between 1 and 200")
			}
			if *buildLimit != 0 && (*buildLimit < 1 || *buildLimit > 50) {
				return fmt.Errorf("apps app-encryption-declarations list: --build-limit must be between 1 and 50")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: %w", err)
			}

			fieldsValue, err := normalizeAppEncryptionDeclarationFields(*fields)
			if err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: %w", err)
			}
			documentFieldsValue, err := normalizeAppEncryptionDeclarationDocumentFields(*documentFields)
			if err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: %w", err)
			}
			includeValue, err := normalizeAppEncryptionDeclarationInclude(*include)
			if err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			buildIDs := shared.SplitCSV(*builds)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEncryptionDeclarationsOption{
				asc.WithAppEncryptionDeclarationsBuildIDs(buildIDs),
				asc.WithAppEncryptionDeclarationsFields(fieldsValue),
				asc.WithAppEncryptionDeclarationsDocumentFields(documentFieldsValue),
				asc.WithAppEncryptionDeclarationsInclude(includeValue),
				asc.WithAppEncryptionDeclarationsLimit(*limit),
				asc.WithAppEncryptionDeclarationsBuildLimit(*buildLimit),
				asc.WithAppEncryptionDeclarationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEncryptionDeclarationsLimit(200))
				firstPage, err := client.GetAppEncryptionDeclarations(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("apps app-encryption-declarations list: failed to fetch: %w", err)
				}
				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEncryptionDeclarations(ctx, resolvedAppID, asc.WithAppEncryptionDeclarationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("apps app-encryption-declarations list: %w", err)
				}
				return shared.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEncryptionDeclarations(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("apps app-encryption-declarations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
