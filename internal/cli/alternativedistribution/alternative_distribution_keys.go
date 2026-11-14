package alternativedistribution

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAlternativeDistributionKeysCommand returns the keys command group.
func newAlternativeDistributionKeysCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("keys", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "keys",
		ShortUsage: "asc alternative-distribution keys <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution keys.",
		LongHelp: `Manage alternative distribution keys.

Examples:
  asc alternative-distribution keys list
  asc alternative-distribution keys view --key-id "KEY_ID"
  asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"
  asc alternative-distribution keys delete --key-id "KEY_ID" --confirm
  asc alternative-distribution keys app --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAlternativeDistributionKeysListCommand(d),
			newAlternativeDistributionKeysGetCommand(d),
			newAlternativeDistributionKeysCreateCommand(d),
			newAlternativeDistributionKeysDeleteCommand(d),
			newAlternativeDistributionKeysAppCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAlternativeDistributionKeysListCommand returns the keys list subcommand.
func newAlternativeDistributionKeysListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc alternative-distribution keys list [flags]",
		ShortHelp:   "List alternative distribution keys.",
		LongHelp: `List alternative distribution keys.

Examples:
  asc alternative-distribution keys list
  asc alternative-distribution keys list --limit 50
  asc alternative-distribution keys list --paginate`,
		TopLevel:             true,
		LimitMax:             alternativeDistributionMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "alternative-distribution keys list",
		FetchErrorPrefix:     "alternative-distribution keys list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAlternativeDistributionKeys(ctx,
				asc.Limit[asc.AlternativeDistributionKeysOption](limit),
				asc.NextURL[asc.AlternativeDistributionKeysOption](next),
			)
		},
	})
}

// newAlternativeDistributionKeysGetCommand returns the keys get subcommand.
func newAlternativeDistributionKeysGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc alternative-distribution keys view --key-id \"KEY_ID\"",
		ShortHelp:   "View an alternative distribution key.",
		LongHelp: `View an alternative distribution key.

Examples:
  asc alternative-distribution keys view --key-id "KEY_ID"`,
		IDFlag:             "key-id",
		IDUsage:            "Alternative distribution key ID",
		IDType:             "alternativeDistributionKeys",
		RequiredDiagnostic: true,
		ErrorPrefix:        "alternative-distribution keys view",
		FetchErrorPrefix:   "alternative-distribution keys view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAlternativeDistributionKey(ctx, id)
		},
	})
}

// newAlternativeDistributionKeysCreateCommand returns the keys create subcommand.
func newAlternativeDistributionKeysCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	publicKey := fs.String("public-key", "", "Public key content")
	publicKeyPath := fs.String("public-key-path", "", "Path to public key file")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			keyValue := strings.TrimSpace(*publicKey)
			keyPath := strings.TrimSpace(*publicKeyPath)
			if keyValue != "" && keyPath != "" {
				return d.UsageError("alternative-distribution keys create: only one of --public-key or --public-key-path is allowed")
			}
			if keyValue == "" && keyPath == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --public-key or --public-key-path is required")
				return shared.MissingRequiredUsageError("")
			}
			if keyValue == "" && keyPath != "" {
				var err error
				keyValue, err = readPublicKey(keyPath)
				if err != nil {
					return fmt.Errorf("alternative-distribution keys create: %w", err)
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAlternativeDistributionKey(requestCtx, resolvedAppID, keyValue)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAlternativeDistributionKeysDeleteCommand returns the keys delete subcommand.
func newAlternativeDistributionKeysDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc alternative-distribution keys delete --key-id \"KEY_ID\" --confirm",
		ShortHelp:   "Delete an alternative distribution key.",
		LongHelp: `Delete an alternative distribution key.

Examples:
  asc alternative-distribution keys delete --key-id "KEY_ID" --confirm`,
		IDFlag:             "key-id",
		IDUsage:            "Alternative distribution key ID",
		IDType:             "alternativeDistributionKeys",
		RequiredDiagnostic: true,
		ErrorPrefix:        "alternative-distribution keys delete",
		DeleteErrorPrefix:  "alternative-distribution keys delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAlternativeDistributionKey(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AlternativeDistributionKeyDeleteResult{ID: id, Deleted: true}
		},
	})
}

// newAlternativeDistributionKeysAppCommand returns the app key subcommand.
func newAlternativeDistributionKeysAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("alternative-distribution keys app: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppAlternativeDistributionKey(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("alternative-distribution keys app: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
