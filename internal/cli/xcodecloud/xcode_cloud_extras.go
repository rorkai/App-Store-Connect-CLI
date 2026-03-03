package xcodecloud

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudProductsListFlags(fs *flag.FlagSet) (appID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	return xcodeCloudWorkflowsListFlags(fs)
}

// XcodeCloudProductsCommand returns the xcode-cloud products command with subcommands.
func XcodeCloudProductsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("products", flag.ExitOnError)

	appID, limit, next, paginate, output, pretty := xcodeCloudProductsListFlags(fs)

	return &ffcli.Command{
		Name:       "products",
		ShortUsage: "asc xcode-cloud products [flags]",
		ShortHelp:  "Manage Xcode Cloud products.",
		LongHelp: `Manage Xcode Cloud products.

Examples:
  asc xcode-cloud products --app "APP_ID"
  asc xcode-cloud products list --app "APP_ID"
  asc xcode-cloud products get --id "PRODUCT_ID"
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudProductsListCommand(),
			XcodeCloudProductsGetCommand(),
			XcodeCloudProductsAppCommand(),
			XcodeCloudProductsBuildRunsCommand(),
			XcodeCloudProductsWorkflowsCommand(),
			XcodeCloudProductsPrimaryRepositoriesCommand(),
			XcodeCloudProductsAdditionalRepositoriesCommand(),
			XcodeCloudProductsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudProductsList(ctx, *appID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudProductsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID, limit, next, paginate, output, pretty := xcodeCloudProductsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud products list [flags]",
		ShortHelp:  "List Xcode Cloud products.",
		LongHelp: `List Xcode Cloud products.

Examples:
  asc xcode-cloud products list
  asc xcode-cloud products list --app "APP_ID"
  asc xcode-cloud products list --limit 50
  asc xcode-cloud products list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudProductsList(ctx, *appID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudProductsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud products get --id \"PRODUCT_ID\"",
		ShortHelp:   "Get details for a product.",
		LongHelp: `Get details for a product.

Examples:
  asc xcode-cloud products get --id "PRODUCT_ID"
  asc xcode-cloud products get --id "PRODUCT_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Product ID",
		ErrorPrefix: "xcode-cloud products get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiProduct(ctx, id)
		},
	})
}

func XcodeCloudProductsAppCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "app",
		Name:        "app",
		ShortUsage:  "asc xcode-cloud products app --id \"PRODUCT_ID\"",
		ShortHelp:   "Get the app for a product.",
		LongHelp: `Get the app for a product.

Examples:
  asc xcode-cloud products app --id "PRODUCT_ID"
  asc xcode-cloud products app --id "PRODUCT_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Product ID",
		ErrorPrefix: "xcode-cloud products app",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiProductApp(ctx, id)
		},
	})
}

func XcodeCloudProductsBuildRunsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "build-runs",
		Name:        "build-runs",
		ShortUsage:  "asc xcode-cloud products build-runs [flags]",
		ShortHelp:   "List build runs for a product.",
		LongHelp: `List build runs for a product.

Examples:
  asc xcode-cloud products build-runs --id "PRODUCT_ID"
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products build-runs",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiBuildRunsOption{
				asc.WithCiBuildRunsLimit(limit),
				asc.WithCiBuildRunsNextURL(next),
			}
			return client.GetCiProductBuildRuns(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsWorkflowsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "workflows",
		Name:        "workflows",
		ShortUsage:  "asc xcode-cloud products workflows [flags]",
		ShortHelp:   "List workflows for a product.",
		LongHelp: `List workflows for a product.

Examples:
  asc xcode-cloud products workflows --id "PRODUCT_ID"
  asc xcode-cloud products workflows --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products workflows --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products workflows",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiWorkflowsOption{
				asc.WithCiWorkflowsLimit(limit),
				asc.WithCiWorkflowsNextURL(next),
			}
			return client.GetCiWorkflows(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsPrimaryRepositoriesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "primary-repositories",
		Name:        "primary-repositories",
		ShortUsage:  "asc xcode-cloud products primary-repositories [flags]",
		ShortHelp:   "List primary repositories for a product.",
		LongHelp: `List primary repositories for a product.

Examples:
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID"
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products primary-repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(limit),
				asc.WithCiProductRepositoriesNextURL(next),
			}
			return client.GetCiProductPrimaryRepositories(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsAdditionalRepositoriesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "additional-repositories",
		Name:        "additional-repositories",
		ShortUsage:  "asc xcode-cloud products additional-repositories [flags]",
		ShortHelp:   "List additional repositories for a product.",
		LongHelp: `List additional repositories for a product.

Examples:
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID"
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products additional-repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(limit),
				asc.WithCiProductRepositoriesNextURL(next),
			}
			return client.GetCiProductAdditionalRepositories(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc xcode-cloud products delete --id \"PRODUCT_ID\" --confirm",
		ShortHelp:   "Delete a product.",
		LongHelp: `Delete a product.

Examples:
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		IDFlag:      "id",
		IDUsage:     "Product ID",
		ErrorPrefix: "xcode-cloud products delete",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			if err := client.DeleteCiProduct(ctx, id); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			return nil
		},
		Result: func(id string) any {
			return &asc.CiProductDeleteResult{ID: id, Deleted: true}
		},
	})
}

func xcodeCloudProductsList(ctx context.Context, appID string, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud products: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}

	resolvedAppID := shared.ResolveAppID(appID)
	opts := []asc.CiProductsOption{
		asc.WithCiProductsLimit(limit),
		asc.WithCiProductsNextURL(next),
	}
	if strings.TrimSpace(next) == "" && resolvedAppID != "" {
		opts = append(opts, asc.WithCiProductsAppID(resolvedAppID))
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	if paginate {
		paginateOpts := append(opts, asc.WithCiProductsLimit(200))
		resp, err := shared.PaginateWithSpinner(requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, asc.WithCiProductsNextURL(nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("xcode-cloud products: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiProducts(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}

func xcodeCloudVersionListFlags(fs *flag.FlagSet) (limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudMacOSVersionsCommand returns the xcode-cloud macos-versions command.
func XcodeCloudMacOSVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("macos-versions", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "macos-versions",
		ShortUsage: "asc xcode-cloud macos-versions [flags]",
		ShortHelp:  "Manage Xcode Cloud macOS versions.",
		LongHelp: `Manage Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions get --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudMacOSVersionsListCommand(),
			XcodeCloudMacOSVersionsGetCommand(),
			XcodeCloudMacOSVersionsXcodeVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudMacOSVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudMacOSVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud macos-versions list [flags]",
		ShortHelp:  "List Xcode Cloud macOS versions.",
		LongHelp: `List Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions list --limit 50
  asc xcode-cloud macos-versions list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudMacOSVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudMacOSVersionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud macos-versions get --id \"MACOS_VERSION_ID\"",
		ShortHelp:   "Get details for a macOS version.",
		LongHelp: `Get details for a macOS version.

Examples:
  asc xcode-cloud macos-versions get --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions get --id "MACOS_VERSION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "macOS version ID",
		ErrorPrefix: "xcode-cloud macos-versions get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiMacOsVersion(ctx, id)
		},
	})
}

func XcodeCloudMacOSVersionsXcodeVersionsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "xcode-versions",
		Name:        "xcode-versions",
		ShortUsage:  "asc xcode-cloud macos-versions xcode-versions [flags]",
		ShortHelp:   "List Xcode versions for a macOS version.",
		LongHelp: `List Xcode versions for a macOS version.

Examples:
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --limit 50
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "macOS version ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud macos-versions xcode-versions",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, macOSVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiXcodeVersionsOption{
				asc.WithCiXcodeVersionsLimit(limit),
				asc.WithCiXcodeVersionsNextURL(next),
			}
			return client.GetCiMacOsVersionXcodeVersions(ctx, macOSVersionID, opts...)
		},
	})
}

func xcodeCloudMacOSVersionsList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud macos-versions",
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(
				ctx,
				asc.WithCiMacOsVersionsLimit(limit),
				asc.WithCiMacOsVersionsNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(ctx, asc.WithCiMacOsVersionsNextURL(next))
		},
	)
}

// XcodeCloudXcodeVersionsCommand returns the xcode-cloud xcode-versions command.
func XcodeCloudXcodeVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode-versions", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "xcode-versions",
		ShortUsage: "asc xcode-cloud xcode-versions [flags]",
		ShortHelp:  "Manage Xcode Cloud Xcode versions.",
		LongHelp: `Manage Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions get --id \"XCODE_VERSION_ID\"
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudXcodeVersionsListCommand(),
			XcodeCloudXcodeVersionsGetCommand(),
			XcodeCloudXcodeVersionsMacOSVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudXcodeVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudXcodeVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud xcode-versions list [flags]",
		ShortHelp:  "List Xcode Cloud Xcode versions.",
		LongHelp: `List Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions list --limit 50
  asc xcode-cloud xcode-versions list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudXcodeVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudXcodeVersionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud xcode-versions get --id \"XCODE_VERSION_ID\"",
		ShortHelp:   "Get details for an Xcode version.",
		LongHelp: `Get details for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions get --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions get --id "XCODE_VERSION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Xcode version ID",
		ErrorPrefix: "xcode-cloud xcode-versions get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiXcodeVersion(ctx, id)
		},
	})
}

func XcodeCloudXcodeVersionsMacOSVersionsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "macos-versions",
		Name:        "macos-versions",
		ShortUsage:  "asc xcode-cloud xcode-versions macos-versions [flags]",
		ShortHelp:   "List macOS versions for an Xcode version.",
		LongHelp: `List macOS versions for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\"
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\" --limit 50
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Xcode version ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud xcode-versions macos-versions",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, xcodeVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiMacOsVersionsOption{
				asc.WithCiMacOsVersionsLimit(limit),
				asc.WithCiMacOsVersionsNextURL(next),
			}
			return client.GetCiXcodeVersionMacOsVersions(ctx, xcodeVersionID, opts...)
		},
	})
}

func xcodeCloudXcodeVersionsList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud xcode-versions: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("xcode-cloud xcode-versions: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud xcode-versions: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.CiXcodeVersionsOption{
		asc.WithCiXcodeVersionsLimit(limit),
		asc.WithCiXcodeVersionsNextURL(next),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithCiXcodeVersionsLimit(200))
		resp, err := shared.PaginateWithSpinner(requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetCiXcodeVersions(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiXcodeVersions(ctx, asc.WithCiXcodeVersionsNextURL(nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("xcode-cloud xcode-versions: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiXcodeVersions(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud xcode-versions: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}

func readJSONFilePayload(path string) (json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("payload path must be a file")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("payload file is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return json.RawMessage(data), nil
}
