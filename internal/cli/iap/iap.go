package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPCommand returns the in-app purchases command group.
func IAPCommand() *ffcli.Command {
	fs := flag.NewFlagSet("iap", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "iap",
		ShortUsage: "asc iap <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchases in App Store Connect.",
		LongHelp: `Manage in-app purchases in App Store Connect.

Examples:
  asc iap list --app "APP_ID"
  asc iap prices --app "APP_ID"
  asc iap get --id "IAP_ID"
  asc iap create --app "APP_ID" --type CONSUMABLE --ref-name "Pro" --product-id "com.example.pro"
  asc iap update --id "IAP_ID" --ref-name "New Name"
  asc iap delete --id "IAP_ID" --confirm
  asc iap localizations list --iap-id "IAP_ID"
  asc iap images create --iap-id "IAP_ID" --file "./image.png"
  asc iap availability set --iap-id "IAP_ID" --territories "USA,CAN"
  asc iap offer-codes create --iap-id "IAP_ID" --name "SPRING" --prices "USA:PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPListCommand(),
			IAPPricesCommand(),
			IAPGetCommand(),
			IAPCreateCommand(),
			IAPUpdateCommand(),
			IAPDeleteCommand(),
			IAPLocalizationsCommand(),
			IAPImagesCommand(),
			IAPReviewScreenshotsCommand(),
			IAPAvailabilityCommand(),
			IAPAvailabilitiesCommand(),
			IAPPromotedPurchaseCommand(),
			IAPContentCommand(),
			IAPPricePointsCommand(),
			IAPPriceSchedulesCommand(),
			IAPOfferCodesCommand(),
			IAPSubmitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPListCommand returns the iap list subcommand.
func IAPListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	legacy := fs.Bool("legacy", false, "Use legacy v1 in-app purchases endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap list [flags]",
		ShortHelp:  "List in-app purchases for an app.",
		LongHelp: `List in-app purchases for an app.

Examples:
  asc iap list --app "APP_ID"
  asc iap list --app "APP_ID" --limit 50
  asc iap list --app "APP_ID" --paginate
  asc iap list --app "APP_ID" --legacy`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOption{
				asc.WithIAPLimit(*limit),
				asc.WithIAPNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPLimit(200))
				if *legacy {
					firstPage, err := client.GetInAppPurchases(requestCtx, resolvedAppID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("iap list: failed to fetch: %w", err)
					}

					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetInAppPurchases(ctx, resolvedAppID, asc.WithIAPNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("iap list: %w", err)
					}

					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				firstPage, err := client.GetInAppPurchasesV2(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasesV2(ctx, resolvedAppID, asc.WithIAPNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			if *legacy {
				resp, err := client.GetInAppPurchases(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("iap list: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasesV2(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("iap list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPGetCommand returns the iap get subcommand.
func IAPGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	iapID := fs.String("id", "", "In-app purchase ID")
	legacy := fs.Bool("legacy", false, "Use legacy v1 in-app purchase endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap get --id \"IAP_ID\"",
		ShortHelp:  "Get an in-app purchase by ID.",
		LongHelp: `Get an in-app purchase by ID.

Examples:
  asc iap get --id "IAP_ID"
  asc iap get --id "IAP_ID" --legacy`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *legacy {
				resp, err := client.GetInAppPurchase(requestCtx, id)
				if err != nil {
					return fmt.Errorf("iap get: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPCreateCommand returns the iap create subcommand.
func IAPCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	iapType := fs.String("type", "", "IAP type: CONSUMABLE, NON_CONSUMABLE, NON_RENEWING_SUBSCRIPTION")
	refName := fs.String("ref-name", "", "Reference name")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.product)")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap create [flags]",
		ShortHelp:  "Create a new in-app purchase.",
		LongHelp: `Create a new in-app purchase.

Examples:
  asc iap create --app "APP_ID" --type CONSUMABLE --ref-name "Pro" --product-id "com.example.pro"
  asc iap create --app "APP_ID" --type NON_CONSUMABLE --ref-name "Lifetime" --product-id "com.example.lifetime" --family-sharable`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			normalizedType, err := normalizeIAPType(*iapType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*refName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --ref-name is required")
				return flag.ErrHelp
			}

			product := strings.TrimSpace(*productID)
			if product == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.InAppPurchaseV2CreateAttributes{
				Name:              name,
				ProductID:         product,
				InAppPurchaseType: normalizedType,
			}
			if *familySharable {
				attrs.FamilySharable = true
			}

			resp, err := client.CreateInAppPurchaseV2(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("iap create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPUpdateCommand returns the iap update subcommand.
func IAPUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	iapID := fs.String("id", "", "In-app purchase ID")
	refName := fs.String("ref-name", "", "Reference name")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap update [flags]",
		ShortHelp:  "Update an in-app purchase.",
		LongHelp: `Update an in-app purchase.

Examples:
  asc iap update --id "IAP_ID" --ref-name "New Name"
  asc iap update --id "IAP_ID" --family-sharable`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*refName)
			if name == "" && !*familySharable {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.InAppPurchaseV2UpdateAttributes{}
			if name != "" {
				attrs.Name = &name
			}
			if *familySharable {
				val := true
				attrs.FamilySharable = &val
			}

			resp, err := client.UpdateInAppPurchaseV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("iap update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPDeleteCommand returns the iap delete subcommand.
func IAPDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	iapID := fs.String("id", "", "In-app purchase ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc iap delete --id \"IAP_ID\" --confirm",
		ShortHelp:  "Delete an in-app purchase.",
		LongHelp: `Delete an in-app purchase.

Examples:
  asc iap delete --id "IAP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteInAppPurchaseV2(requestCtx, id); err != nil {
				return fmt.Errorf("iap delete: failed to delete: %w", err)
			}

			result := &asc.InAppPurchaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// IAPLocalizationsCommand returns the iap localizations command group.
func IAPLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc iap localizations <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase localizations.",
		LongHelp: `Manage in-app purchase localizations.

Examples:
  asc iap localizations list --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPLocalizationsListCommand(),
			IAPLocalizationsCreateCommand(),
			IAPLocalizationsUpdateCommand(),
			IAPLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPLocalizationsListCommand returns the localizations list subcommand.
func IAPLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations list", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	legacyID := fs.String("id", "", "In-app purchase ID (deprecated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap localizations list [flags]",
		ShortHelp:  "List in-app purchase localizations.",
		LongHelp: `List in-app purchase localizations.

Examples:
  asc iap localizations list --iap-id "IAP_ID"
  asc iap localizations list --iap-id "IAP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedID := strings.TrimSpace(*iapID)
			if resolvedID == "" {
				resolvedID = strings.TrimSpace(*legacyID)
			}
			if resolvedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap localizations list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPLocalizationsOption{
				asc.WithIAPLocalizationsLimit(*limit),
				asc.WithIAPLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPLocalizationsLimit(200))
				firstPage, err := client.GetInAppPurchaseLocalizations(requestCtx, resolvedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseLocalizations(ctx, resolvedID, asc.WithIAPLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseLocalizations(requestCtx, resolvedID, opts...)
			if err != nil {
				return fmt.Errorf("iap localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeIAPType(value string) (string, error) {
	normalized := strings.TrimSpace(strings.ToUpper(value))
	if normalized == "" {
		return "", fmt.Errorf("--type is required")
	}
	if slices.Contains(asc.ValidIAPTypes, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--type must be one of: %s", strings.Join(asc.ValidIAPTypes, ", "))
}
