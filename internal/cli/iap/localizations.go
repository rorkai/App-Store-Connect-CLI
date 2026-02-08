package iap

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

// IAPLocalizationsCreateCommand returns the localizations create subcommand.
func IAPLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	name := fs.String("name", "", "Localization name")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	description := fs.String("description", "", "Description")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap localizations create --iap-id \"IAP_ID\" --name \"Name\" --locale \"en-US\"",
		ShortHelp:  "Create an in-app purchase localization.",
		LongHelp: `Create an in-app purchase localization.

Examples:
  asc iap localizations create --iap-id "IAP_ID" --name "Title" --locale "en-US"
  asc iap localizations create --iap-id "IAP_ID" --name "Titre" --locale "fr-FR" --description "Detail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseLocalization(requestCtx, iapValue, asc.InAppPurchaseLocalizationCreateAttributes{
				Name:        nameValue,
				Locale:      localeValue,
				Description: strings.TrimSpace(*description),
			})
			if err != nil {
				return fmt.Errorf("iap localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// IAPLocalizationsUpdateCommand returns the localizations update subcommand.
func IAPLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	name := fs.String("name", "", "Localization name")
	description := fs.String("description", "", "Description")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update an in-app purchase localization.",
		LongHelp: `Update an in-app purchase localization.

Examples:
  asc iap localizations update --localization-id "LOC_ID" --name "New Name"
  asc iap localizations update --localization-id "LOC_ID" --description "New Description"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			descriptionValue := strings.TrimSpace(*description)
			if nameValue == "" && descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.InAppPurchaseLocalizationUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if descriptionValue != "" {
				attrs.Description = &descriptionValue
			}

			resp, err := client.UpdateInAppPurchaseLocalization(requestCtx, locValue, attrs)
			if err != nil {
				return fmt.Errorf("iap localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// IAPLocalizationsDeleteCommand returns the localizations delete subcommand.
func IAPLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc iap localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete an in-app purchase localization.",
		LongHelp: `Delete an in-app purchase localization.

Examples:
  asc iap localizations delete --localization-id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteInAppPurchaseLocalization(requestCtx, locValue); err != nil {
				return fmt.Errorf("iap localizations delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{
				ID:      locValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
