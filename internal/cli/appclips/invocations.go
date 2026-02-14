package appclips

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

// AppClipInvocationsCommand returns the invocations command group.
func AppClipInvocationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invocations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "invocations",
		ShortUsage: "asc app-clips invocations <subcommand> [flags]",
		ShortHelp:  "Manage beta App Clip invocations.",
		LongHelp: `Manage beta App Clip invocations.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipInvocationsListCommand(),
			AppClipInvocationsGetCommand(),
			AppClipInvocationsCreateCommand(),
			AppClipInvocationsUpdateCommand(),
			AppClipInvocationsDeleteCommand(),
			AppClipInvocationLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipInvocationsListCommand lists beta App Clip invocations for a build bundle.
func AppClipInvocationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips invocations list --build-bundle-id \"BUILD_BUNDLE_ID\" [flags]",
		ShortHelp:  "List beta App Clip invocations for a build bundle.",
		LongHelp: `List beta App Clip invocations for a build bundle.

Examples:
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips invocations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips invocations list: %w", err)
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-bundle-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppClipInvocationsOption{
				asc.WithBetaAppClipInvocationsLimit(*limit),
				asc.WithBetaAppClipInvocationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaAppClipInvocationsLimit(200))
				firstPage, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						fmt.Fprintln(os.Stderr, "No invocations found.")
						empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleBetaAppClipInvocations(ctx, buildBundleValue, asc.WithBetaAppClipInvocationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips invocations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					fmt.Fprintln(os.Stderr, "No invocations found.")
					empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips invocations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsGetCommand gets a beta App Clip invocation by ID.
func AppClipInvocationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	invocationID := fs.String("invocation-id", "", "Invocation ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips invocations get --invocation-id \"INVOCATION_ID\"",
		ShortHelp:  "Get a beta App Clip invocation by ID.",
		LongHelp: `Get a beta App Clip invocation by ID.

Examples:
  asc app-clips invocations get --invocation-id "INVOCATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --invocation-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppClipInvocation(requestCtx, invocationValue)
			if err != nil {
				return fmt.Errorf("app-clips invocations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsCreateCommand creates a beta App Clip invocation.
func AppClipInvocationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	url := fs.String("url", "", "Invocation URL")
	localizationIDs := fs.String("localization-id", "", "Localization ID(s), comma-separated")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips invocations create --build-bundle-id \"BUILD_BUNDLE_ID\" --url \"https://example.com/clip\" [flags]",
		ShortHelp:  "Create a beta App Clip invocation.",
		LongHelp: `Create a beta App Clip invocation.

Examples:
  asc app-clips invocations create --build-bundle-id "BUILD_BUNDLE_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-bundle-id is required")
				return flag.ErrHelp
			}

			urlValue := strings.TrimSpace(*url)
			if urlValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --url is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaAppClipInvocationCreateAttributes{URL: urlValue}
			resp, err := client.CreateBetaAppClipInvocation(requestCtx, buildBundleValue, attrs, shared.SplitCSV(*localizationIDs))
			if err != nil {
				return fmt.Errorf("app-clips invocations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsUpdateCommand updates a beta App Clip invocation.
func AppClipInvocationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	invocationID := fs.String("invocation-id", "", "Invocation ID")
	url := fs.String("url", "", "Invocation URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips invocations update --invocation-id \"INVOCATION_ID\" [flags]",
		ShortHelp:  "Update a beta App Clip invocation.",
		LongHelp: `Update a beta App Clip invocation.

Examples:
  asc app-clips invocations update --invocation-id "INVOCATION_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --invocation-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["url"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			urlValue := strings.TrimSpace(*url)
			attrs := &asc.BetaAppClipInvocationUpdateAttributes{URL: &urlValue}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaAppClipInvocation(requestCtx, invocationValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips invocations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipInvocationsDeleteCommand deletes a beta App Clip invocation.
func AppClipInvocationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	invocationID := fs.String("invocation-id", "", "Invocation ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips invocations delete --invocation-id \"INVOCATION_ID\" --confirm",
		ShortHelp:  "Delete a beta App Clip invocation.",
		LongHelp: `Delete a beta App Clip invocation.

Examples:
  asc app-clips invocations delete --invocation-id "INVOCATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --invocation-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaAppClipInvocation(requestCtx, invocationValue); err != nil {
				return fmt.Errorf("app-clips invocations delete: failed to delete: %w", err)
			}

			result := &asc.BetaAppClipInvocationDeleteResult{
				ID:      invocationValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
