package alternativedistribution

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

// AlternativeDistributionKeysCommand returns the keys command group.
func AlternativeDistributionKeysCommand() *ffcli.Command {
	fs := flag.NewFlagSet("keys", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "keys",
		ShortUsage: "asc alternative-distribution keys <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution keys.",
		LongHelp: `Manage alternative distribution keys.

Examples:
  asc alternative-distribution keys list
  asc alternative-distribution keys get --key-id "KEY_ID"
  asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"
  asc alternative-distribution keys delete --key-id "KEY_ID" --confirm
  asc alternative-distribution keys app --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AlternativeDistributionKeysListCommand(),
			AlternativeDistributionKeysGetCommand(),
			AlternativeDistributionKeysCreateCommand(),
			AlternativeDistributionKeysDeleteCommand(),
			AlternativeDistributionKeysAppCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AlternativeDistributionKeysListCommand returns the keys list subcommand.
func AlternativeDistributionKeysListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc alternative-distribution keys list [flags]",
		ShortHelp:  "List alternative distribution keys.",
		LongHelp: `List alternative distribution keys.

Examples:
  asc alternative-distribution keys list
  asc alternative-distribution keys list --limit 50
  asc alternative-distribution keys list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > alternativeDistributionMaxLimit) {
				return fmt.Errorf("alternative-distribution keys list: --limit must be between 1 and %d", alternativeDistributionMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("alternative-distribution keys list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AlternativeDistributionKeysOption{
				asc.WithAlternativeDistributionKeysLimit(*limit),
				asc.WithAlternativeDistributionKeysNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAlternativeDistributionKeysLimit(alternativeDistributionMaxLimit))
				firstPage, err := client.GetAlternativeDistributionKeys(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("alternative-distribution keys list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAlternativeDistributionKeys(ctx, asc.WithAlternativeDistributionKeysNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("alternative-distribution keys list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAlternativeDistributionKeys(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionKeysGetCommand returns the keys get subcommand.
func AlternativeDistributionKeysGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	keyID := fs.String("key-id", "", "Alternative distribution key ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc alternative-distribution keys get --key-id \"KEY_ID\"",
		ShortHelp:  "Get an alternative distribution key.",
		LongHelp: `Get an alternative distribution key.

Examples:
  asc alternative-distribution keys get --key-id "KEY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*keyID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAlternativeDistributionKey(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionKeysCreateCommand returns the keys create subcommand.
func AlternativeDistributionKeysCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	publicKey := fs.String("public-key", "", "Public key content")
	publicKeyPath := fs.String("public-key-path", "", "Path to public key file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc alternative-distribution keys create --app \"APP_ID\" --public-key-path \"./key.pem\"",
		ShortHelp:  "Create an alternative distribution key.",
		LongHelp: `Create an alternative distribution key.

Examples:
  asc alternative-distribution keys create --app "APP_ID" --public-key "KEY_DATA"
  asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			keyValue := strings.TrimSpace(*publicKey)
			keyPath := strings.TrimSpace(*publicKeyPath)
			if keyValue != "" && keyPath != "" {
				return fmt.Errorf("alternative-distribution keys create: only one of --public-key or --public-key-path is allowed")
			}
			if keyValue == "" && keyPath == "" {
				fmt.Fprintln(os.Stderr, "Error: --public-key or --public-key-path is required")
				return flag.ErrHelp
			}
			if keyValue == "" && keyPath != "" {
				var err error
				keyValue, err = readPublicKey(keyPath)
				if err != nil {
					return fmt.Errorf("alternative-distribution keys create: %w", err)
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAlternativeDistributionKey(requestCtx, resolvedAppID, keyValue)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionKeysDeleteCommand returns the keys delete subcommand.
func AlternativeDistributionKeysDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	keyID := fs.String("key-id", "", "Alternative distribution key ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc alternative-distribution keys delete --key-id \"KEY_ID\" --confirm",
		ShortHelp:  "Delete an alternative distribution key.",
		LongHelp: `Delete an alternative distribution key.

Examples:
  asc alternative-distribution keys delete --key-id "KEY_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*keyID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAlternativeDistributionKey(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("alternative-distribution keys delete: failed to delete: %w", err)
			}

			result := &asc.AlternativeDistributionKeyDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AlternativeDistributionKeysAppCommand returns the app key subcommand.
func AlternativeDistributionKeysAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc alternative-distribution keys app --app \"APP_ID\"",
		ShortHelp:  "Get an app's alternative distribution key.",
		LongHelp: `Get an app's alternative distribution key.

Examples:
  asc alternative-distribution keys app --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys app: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppAlternativeDistributionKey(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys app: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
