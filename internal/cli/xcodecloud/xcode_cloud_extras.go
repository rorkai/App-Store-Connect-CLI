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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud products get --id \"PRODUCT_ID\"",
		ShortHelp:  "Get details for a product.",
		LongHelp: `Get details for a product.

Examples:
  asc xcode-cloud products get --id "PRODUCT_ID"
  asc xcode-cloud products get --id "PRODUCT_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiProduct(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud products get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc xcode-cloud products app --id \"PRODUCT_ID\"",
		ShortHelp:  "Get the app for a product.",
		LongHelp: `Get the app for a product.

Examples:
  asc xcode-cloud products app --id "PRODUCT_ID"
  asc xcode-cloud products app --id "PRODUCT_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiProductApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsBuildRunsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-runs", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build-runs",
		ShortUsage: "asc xcode-cloud products build-runs [flags]",
		ShortHelp:  "List build runs for a product.",
		LongHelp: `List build runs for a product.

Examples:
  asc xcode-cloud products build-runs --id "PRODUCT_ID"
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud products build-runs: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud products build-runs: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products build-runs: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiBuildRunsOption{
				asc.WithCiBuildRunsLimit(*limit),
				asc.WithCiBuildRunsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiBuildRunsLimit(200))
				firstPage, err := client.GetCiProductBuildRuns(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud products build-runs: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiProductBuildRuns(ctx, idValue, asc.WithCiBuildRunsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud products build-runs: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiProductBuildRuns(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud products build-runs: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsWorkflowsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflows", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "workflows",
		ShortUsage: "asc xcode-cloud products workflows [flags]",
		ShortHelp:  "List workflows for a product.",
		LongHelp: `List workflows for a product.

Examples:
  asc xcode-cloud products workflows --id "PRODUCT_ID"
  asc xcode-cloud products workflows --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products workflows --id "PRODUCT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud products workflows: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud products workflows: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products workflows: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiWorkflowsOption{
				asc.WithCiWorkflowsLimit(*limit),
				asc.WithCiWorkflowsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiWorkflowsLimit(200))
				firstPage, err := client.GetCiWorkflows(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud products workflows: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiWorkflows(ctx, idValue, asc.WithCiWorkflowsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud products workflows: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiWorkflows(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud products workflows: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsPrimaryRepositoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("primary-repositories", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "primary-repositories",
		ShortUsage: "asc xcode-cloud products primary-repositories [flags]",
		ShortHelp:  "List primary repositories for a product.",
		LongHelp: `List primary repositories for a product.

Examples:
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID"
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud products primary-repositories: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud products primary-repositories: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products primary-repositories: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(*limit),
				asc.WithCiProductRepositoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiProductRepositoriesLimit(200))
				firstPage, err := client.GetCiProductPrimaryRepositories(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud products primary-repositories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiProductPrimaryRepositories(ctx, idValue, asc.WithCiProductRepositoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud products primary-repositories: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiProductPrimaryRepositories(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud products primary-repositories: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsAdditionalRepositoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("additional-repositories", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "additional-repositories",
		ShortUsage: "asc xcode-cloud products additional-repositories [flags]",
		ShortHelp:  "List additional repositories for a product.",
		LongHelp: `List additional repositories for a product.

Examples:
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID"
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud products additional-repositories: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud products additional-repositories: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products additional-repositories: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(*limit),
				asc.WithCiProductRepositoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiProductRepositoriesLimit(200))
				firstPage, err := client.GetCiProductAdditionalRepositories(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud products additional-repositories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiProductAdditionalRepositories(ctx, idValue, asc.WithCiProductRepositoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud products additional-repositories: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiProductAdditionalRepositories(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud products additional-repositories: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Product ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc xcode-cloud products delete --id \"PRODUCT_ID\" --confirm",
		ShortHelp:  "Delete a product.",
		LongHelp: `Delete a product.

Examples:
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products delete: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			if err := client.DeleteCiProduct(requestCtx, idValue); err != nil {
				return fmt.Errorf("xcode-cloud products delete: failed to delete: %w", err)
			}

			result := &asc.CiProductDeleteResult{ID: idValue, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
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
		firstPage, err := client.GetCiProducts(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud products: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiProducts(ctx, asc.WithCiProductsNextURL(nextURL))
		})
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "macOS version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud macos-versions get --id \"MACOS_VERSION_ID\"",
		ShortHelp:  "Get details for a macOS version.",
		LongHelp: `Get details for a macOS version.

Examples:
  asc xcode-cloud macos-versions get --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions get --id "MACOS_VERSION_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud macos-versions get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiMacOsVersion(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud macos-versions get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudMacOSVersionsXcodeVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode-versions", flag.ExitOnError)

	id := fs.String("id", "", "macOS version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "xcode-versions",
		ShortUsage: "asc xcode-cloud macos-versions xcode-versions [flags]",
		ShortHelp:  "List Xcode versions for a macOS version.",
		LongHelp: `List Xcode versions for a macOS version.

Examples:
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --limit 50
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud macos-versions xcode-versions: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud macos-versions xcode-versions: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud macos-versions xcode-versions: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiXcodeVersionsOption{
				asc.WithCiXcodeVersionsLimit(*limit),
				asc.WithCiXcodeVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiXcodeVersionsLimit(200))
				firstPage, err := client.GetCiMacOsVersionXcodeVersions(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud macos-versions xcode-versions: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiMacOsVersionXcodeVersions(ctx, idValue, asc.WithCiXcodeVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud macos-versions xcode-versions: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiMacOsVersionXcodeVersions(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud macos-versions xcode-versions: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func xcodeCloudMacOSVersionsList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud macos-versions: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("xcode-cloud macos-versions: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud macos-versions: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.CiMacOsVersionsOption{
		asc.WithCiMacOsVersionsLimit(limit),
		asc.WithCiMacOsVersionsNextURL(next),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithCiMacOsVersionsLimit(200))
		firstPage, err := client.GetCiMacOsVersions(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud macos-versions: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(ctx, asc.WithCiMacOsVersionsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud macos-versions: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiMacOsVersions(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud macos-versions: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Xcode version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud xcode-versions get --id \"XCODE_VERSION_ID\"",
		ShortHelp:  "Get details for an Xcode version.",
		LongHelp: `Get details for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions get --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions get --id "XCODE_VERSION_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud xcode-versions get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiXcodeVersion(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud xcode-versions get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudXcodeVersionsMacOSVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("macos-versions", flag.ExitOnError)

	id := fs.String("id", "", "Xcode version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "macos-versions",
		ShortUsage: "asc xcode-cloud xcode-versions macos-versions [flags]",
		ShortHelp:  "List macOS versions for an Xcode version.",
		LongHelp: `List macOS versions for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\"
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\" --limit 50
  asc xcode-cloud xcode-versions macos-versions --id \"XCODE_VERSION_ID\" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud xcode-versions macos-versions: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud xcode-versions macos-versions: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud xcode-versions macos-versions: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiMacOsVersionsOption{
				asc.WithCiMacOsVersionsLimit(*limit),
				asc.WithCiMacOsVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiMacOsVersionsLimit(200))
				firstPage, err := client.GetCiXcodeVersionMacOsVersions(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud xcode-versions macos-versions: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiXcodeVersionMacOsVersions(ctx, idValue, asc.WithCiMacOsVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud xcode-versions macos-versions: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiXcodeVersionMacOsVersions(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud xcode-versions macos-versions: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
		firstPage, err := client.GetCiXcodeVersions(requestCtx, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud xcode-versions: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiXcodeVersions(ctx, asc.WithCiXcodeVersionsNextURL(nextURL))
		})
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
	defer file.Close()

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
