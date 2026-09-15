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

var iapQueryClientFactory = shared.GetASCClient

var iapListStates = []string{
	"MISSING_METADATA",
	"WAITING_FOR_UPLOAD",
	"PROCESSING_CONTENT",
	"READY_TO_SUBMIT",
	"WAITING_FOR_REVIEW",
	"IN_REVIEW",
	"DEVELOPER_ACTION_NEEDED",
	"PENDING_BINARY_APPROVAL",
	"APPROVED",
	"DEVELOPER_REMOVED_FROM_SALE",
	"REMOVED_FROM_SALE",
	"REJECTED",
}

var iapListSortValues = []string{"name", "-name", "inAppPurchaseType", "-inAppPurchaseType"}

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
  asc iap pricing summary --app "APP_ID"
  asc iap view --id "IAP_ID"
  asc iap create --app "APP_ID" --type CONSUMABLE --ref-name "Pro" --product-id "com.example.pro"
  asc iap setup --app "APP_ID" --type NON_CONSUMABLE --reference-name "Pro Lifetime" --product-id "com.example.lifetime" --price "3.99" --base-territory "United States"
  asc iap update --id "IAP_ID" --ref-name "New Name"
  asc iap delete --id "IAP_ID" --confirm
  asc iap versions list --iap-id "IAP_ID"
  asc iap versions localizations list --version-id "IAP_VERSION_ID"
  asc iap versions images create --version-id "IAP_VERSION_ID" --file "./image.png"
  asc iap pricing availability set --iap-id "IAP_ID" --territories "US,Canada"
  asc iap offer-codes create --iap-id "IAP_ID" --name "SPRING" --prices "USA:PRICE_POINT_ID"
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPListCommand(),
			IAPVersionsCommand(),
			IAPPricingCommand(),
			IAPGetCommand(),
			IAPCreateCommand(),
			IAPSetupCommand(),
			IAPUpdateCommand(),
			IAPDeleteCommand(),
			IAPReviewScreenshotsCommand(),
			IAPPromotedPurchasesCommand(),
			IAPContentCommand(),
			IAPOfferCodesCommand(),
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
	productID := fs.String("product-id", "", "Filter by product ID(s), comma-separated")
	name := fs.String("name", "", "Filter by name(s), comma-separated")
	state := fs.String("state", "", "Filter by state(s), comma-separated: "+strings.Join(iapListStates, ", "))
	iapType := fs.String("type", "", "Filter by type(s), comma-separated: "+strings.Join(asc.ValidIAPTypes, ", "))
	sort := fs.String("sort", "", "Sort by (comma-separated): "+strings.Join(iapListSortValues, ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	legacy := fs.Bool("legacy", false, "Use legacy v1 in-app purchases endpoint")
	includeVersions := fs.Bool("include-versions", false, "Include related in-app purchase versions (v2 only)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50, v2 only)")
	fields := fs.String("fields", "", "fields[inAppPurchases] (comma-separated, v2 only)")
	versionFields := fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated, v2 only)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap list [flags]",
		ShortHelp:  "List in-app purchases for an app.",
		LongHelp: `List in-app purchases for an app.

Examples:
  asc iap list --app "APP_ID"
  asc iap list --app "APP_ID" --product-id "com.example.pro"
  asc iap list --app "APP_ID" --state READY_TO_SUBMIT --type CONSUMABLE --sort name
  asc iap list --app "APP_ID" --limit 50
  asc iap list --app "APP_ID" --paginate
  asc iap list --app "APP_ID" --include-versions --versions-limit 10
  asc iap list --app "APP_ID" --fields name,versions
  asc iap list --app "APP_ID" --legacy`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("iap list: %v", err)
			}
			if err := rejectIAPVersionNextFlagConflicts(fs, *next, "iap list", "app", "product-id", "name", "state", "type", "sort", "limit", "include-versions", "versions-limit", "fields", "version-fields"); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap list: --limit must be between 1 and 200")
			}
			if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
				return shared.UsageError("iap list: --versions-limit must be between 1 and 50")
			}
			if *legacy && strings.TrimSpace(*fields) != "" {
				return shared.UsageError("iap list: --fields requires the v2 endpoint")
			}
			if *legacy && (*includeVersions || *versionsLimit != 0 || strings.TrimSpace(*versionFields) != "") {
				return shared.UsageError("iap list: --include-versions, --versions-limit, and --version-fields require the v2 endpoint")
			}
			for _, name := range []string{"product-id", "name", "state", "type", "sort"} {
				if *legacy && flagSet(fs, name) {
					return shared.UsageErrorf("iap list: --%s requires the v2 endpoint", name)
				}
			}
			for _, selector := range []struct {
				name  string
				value string
			}{
				{"product-id", *productID},
				{"name", *name},
				{"state", *state},
				{"type", *iapType},
				{"sort", *sort},
			} {
				if flagSet(fs, selector.name) && strings.TrimSpace(selector.value) == "" {
					return shared.UsageErrorf("iap list: --%s must not be empty", selector.name)
				}
			}

			productIDValues, err := normalizeIAPListCSV(*productID, "--product-id")
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			nameValues, err := normalizeIAPListCSV(*name, "--name")
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			stateValues, err := normalizeIAPListEnum(*state, "--state", iapListStates)
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			typeValues, err := normalizeIAPListEnum(*iapType, "--type", asc.ValidIAPTypes)
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			sortValues, err := normalizeIAPListSort(*sort)
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			fieldValues, err := shared.NormalizeSelection(*fields, iapVersionIAPFields, "--fields")
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, iapVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("iap list: " + err.Error())
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			client, err := iapQueryClientFactory()
			if err != nil {
				return fmt.Errorf("iap list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOption{
				asc.WithIAPLimit(*limit),
				asc.WithIAPNextURL(*next),
				asc.WithIAPProductIDs(productIDValues),
				asc.WithIAPNames(nameValues),
				asc.WithIAPStates(stateValues),
				asc.WithIAPTypes(typeValues),
				asc.WithIAPSort(sortValues),
				asc.WithIAPFields(fieldValues),
				asc.WithIAPNestedVersionsLimit(*versionsLimit),
				asc.WithIAPVersionFields(versionFieldValues),
			}
			if *includeVersions {
				opts = append(opts, asc.WithIAPInclude([]string{"versions"}))
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

// IAPGetCommand returns the iap view subcommand.
func IAPGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	iapID := shared.BindResourceIDFlag(fs, "id", "inAppPurchases", "In-app purchase ID")
	legacy := fs.Bool("legacy", false, "Use legacy v1 in-app purchase endpoint")
	includeVersions := fs.Bool("include-versions", false, "Include related in-app purchase versions (v2 only)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50, v2 only)")
	fields := fs.String("fields", "", "fields[inAppPurchases] (comma-separated, v2 only)")
	versionFields := fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated, v2 only)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap view --id \"IAP_ID\"",
		ShortHelp:  "View an in-app purchase by ID.",
		LongHelp: `View an in-app purchase by ID.

Examples:
  asc iap view --id "IAP_ID"
  asc iap view --id "IAP_ID" --include-versions --versions-limit 10
  asc iap view --id "IAP_ID" --fields name,versions
  asc iap view --id "IAP_ID" --legacy`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
				return shared.UsageError("iap view: --versions-limit must be between 1 and 50")
			}
			if *legacy && strings.TrimSpace(*fields) != "" {
				return shared.UsageError("iap view: --fields requires the v2 endpoint")
			}
			if *legacy && (*includeVersions || *versionsLimit != 0 || strings.TrimSpace(*versionFields) != "") {
				return shared.UsageError("iap view: --include-versions, --versions-limit, and --version-fields require the v2 endpoint")
			}
			fieldValues, err := shared.NormalizeSelection(*fields, iapVersionIAPFields, "--fields")
			if err != nil {
				return shared.UsageError("iap view: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, iapVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("iap view: " + err.Error())
			}

			client, err := iapQueryClientFactory()
			if err != nil {
				return fmt.Errorf("iap view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *legacy {
				resp, err := client.GetInAppPurchase(requestCtx, id)
				if err != nil {
					return fmt.Errorf("iap view: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.IAPGetOption{
				asc.WithIAPGetFields(fieldValues),
				asc.WithIAPGetNestedVersionsLimit(*versionsLimit),
				asc.WithIAPGetVersionFields(versionFieldValues),
			}
			if *includeVersions {
				opts = append(opts, asc.WithIAPGetInclude([]string{"versions"}))
			}
			resp, err := client.GetInAppPurchaseV2(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap view: failed to fetch: %w", err)
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
				return shared.MissingRequiredUsageError("--app")
			}

			normalizedType, err := normalizeIAPType(*iapType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*refName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --ref-name is required")
				return shared.MissingRequiredUsageError("--ref-name")
			}

			product := strings.TrimSpace(*productID)
			if product == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError("--product-id")
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

	iapID := shared.BindResourceIDFlag(fs, "id", "inAppPurchases", "In-app purchase ID")
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
				return shared.MissingRequiredUsageError("--id")
			}

			name := strings.TrimSpace(*refName)
			if name == "" && !*familySharable {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
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

	iapID := shared.BindResourceIDFlag(fs, "id", "inAppPurchases", "In-app purchase ID")
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
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
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

func normalizeIAPListCSV(value, flagName string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("%s must not contain empty values", flagName)
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values, nil
}

func normalizeIAPListEnum(value, flagName string, allowed []string) ([]string, error) {
	values, err := normalizeIAPListCSV(value, flagName)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(value)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		if !slices.Contains(allowed, value) {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeIAPListSort(value string) ([]string, error) {
	values, err := normalizeIAPListCSV(value, "--sort")
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if !slices.Contains(iapListSortValues, value) {
			return nil, fmt.Errorf("--sort must be one of: %s", strings.Join(iapListSortValues, ", "))
		}
	}
	return values, nil
}
