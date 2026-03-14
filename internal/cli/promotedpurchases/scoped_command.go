package promotedpurchases

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ScopedPromotedPurchasesCommandConfig customizes a promoted-purchases command tree
// to a single product family while preserving the shared generic implementation.
type ScopedPromotedPurchasesCommandConfig struct {
	PathPrefix      string
	ProductType     promotedPurchaseProductType
	ProductSingular string
	ProductPlural   string
	RootShortHelp   string
	RootLongHelp    string
}

type promotedPurchaseScope struct {
	productType promotedPurchaseProductType
	productID   string
}

type promotedPurchaseRelationships struct {
	InAppPurchaseV2 *asc.Relationship `json:"inAppPurchaseV2"`
	Subscription    *asc.Relationship `json:"subscription"`
}

// ConfigureScopedPromotedPurchasesCommand constrains a promoted-purchases command
// tree to one product family and updates help text accordingly.
func ConfigureScopedPromotedPurchasesCommand(cmd *ffcli.Command, cfg ScopedPromotedPurchasesCommandConfig) {
	if cmd == nil {
		return
	}
	if strings.TrimSpace(cfg.RootShortHelp) != "" {
		cmd.ShortHelp = cfg.RootShortHelp
	}
	if strings.TrimSpace(cfg.RootLongHelp) != "" {
		cmd.LongHelp = cfg.RootLongHelp
	}

	if listCmd := findDirectSubcommand(cmd, "list"); listCmd != nil {
		configureScopedPromotedPurchasesListCommand(listCmd, cfg)
	}
	if getCmd := findDirectSubcommand(cmd, "get"); getCmd != nil {
		wrapScopedPromotedPurchaseDetailCommand(getCmd, cfg)
	}
	if updateCmd := findDirectSubcommand(cmd, "update"); updateCmd != nil {
		wrapScopedPromotedPurchaseDetailCommand(updateCmd, cfg)
	}
	if deleteCmd := findDirectSubcommand(cmd, "delete"); deleteCmd != nil {
		wrapScopedPromotedPurchaseDetailCommand(deleteCmd, cfg)
	}
	if linkCmd := findDirectSubcommand(cmd, "link"); linkCmd != nil {
		configureScopedPromotedPurchasesLinkCommand(linkCmd, cfg)
	}
}

func configureScopedPromotedPurchasesListCommand(cmd *ffcli.Command, cfg ScopedPromotedPurchasesCommandConfig) {
	if cmd == nil || cmd.FlagSet == nil {
		return
	}

	cmd.ShortHelp = fmt.Sprintf("List promoted purchases for %s in an app.", cfg.ProductPlural)
	cmd.LongHelp = fmt.Sprintf(`List promoted purchases for %s in an app.

Examples:
  %s list --app "APP_ID"
  %s list --app "APP_ID" --limit 10
  %s list --app "APP_ID" --paginate`, cfg.ProductPlural, cfg.PathPrefix, cfg.PathPrefix, cfg.PathPrefix)

	cmd.Exec = func(ctx context.Context, args []string) error {
		limit := intFlagValue(cmd.FlagSet, "limit")
		next := stringFlagValue(cmd.FlagSet, "next")
		paginate := boolFlagValue(cmd.FlagSet, "paginate")
		output := stringFlagValue(cmd.FlagSet, "output")
		pretty := boolFlagValue(cmd.FlagSet, "pretty")
		appID := shared.ResolveAppID(stringFlagValue(cmd.FlagSet, "app"))
		errorPrefix := promotedPurchasesCommandErrorPrefix(cfg, "list")

		if limit != 0 && (limit < 1 || limit > 200) {
			fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
			return flag.ErrHelp
		}
		if err := shared.ValidateNextURL(next); err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
		if appID == "" && strings.TrimSpace(next) == "" {
			fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
			return flag.ErrHelp
		}

		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}

		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()

		opts := []asc.PromotedPurchasesOption{
			asc.WithPromotedPurchasesLimit(limit),
			asc.WithPromotedPurchasesNextURL(next),
		}

		if paginate {
			paginateOpts := append(opts, asc.WithPromotedPurchasesLimit(200))
			firstPage, err := client.GetAppPromotedPurchases(requestCtx, appID, paginateOpts...)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", errorPrefix, err)
			}

			paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetAppPromotedPurchases(ctx, appID, asc.WithPromotedPurchasesNextURL(nextURL))
			})
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			resp, ok := paginated.(*asc.PromotedPurchasesResponse)
			if !ok {
				return fmt.Errorf("%s: unexpected response type %T", errorPrefix, paginated)
			}
			if err := filterPromotedPurchasesByProductType(requestCtx, client, resp, cfg.ProductType); err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}
			return shared.PrintOutput(resp, output, pretty)
		}

		resp, err := client.GetAppPromotedPurchases(requestCtx, appID, opts...)
		if err != nil {
			return fmt.Errorf("%s: failed to fetch: %w", errorPrefix, err)
		}
		if err := filterPromotedPurchasesByProductType(requestCtx, client, resp, cfg.ProductType); err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}
}

func wrapScopedPromotedPurchaseDetailCommand(cmd *ffcli.Command, cfg ScopedPromotedPurchasesCommandConfig) {
	if cmd == nil || cmd.Exec == nil || cmd.FlagSet == nil {
		return
	}

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		promotedPurchaseID := strings.TrimSpace(stringFlagValue(cmd.FlagSet, "promoted-purchase-id"))
		if promotedPurchaseID != "" {
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", promotedPurchasesCommandErrorPrefix(cfg, cmd.Name), err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := validatePromotedPurchaseScope(requestCtx, client, promotedPurchaseID, cfg, cmd.Name); err != nil {
				return err
			}
		}

		return originalExec(ctx, args)
	}
}

func configureScopedPromotedPurchasesLinkCommand(cmd *ffcli.Command, cfg ScopedPromotedPurchasesCommandConfig) {
	if cmd == nil || cmd.FlagSet == nil {
		return
	}

	cmd.ShortHelp = fmt.Sprintf("Link or clear promoted purchases for %s while preserving %s.", cfg.ProductPlural, otherProductPlural(cfg.ProductType))
	cmd.LongHelp = fmt.Sprintf(`Link or clear promoted purchases for %s on an app.

Only promoted purchases attached to %s are modified. Existing promoted purchases
for %s are preserved.

Examples:
  %s link --app "APP_ID" --promoted-purchase-id "PROMO_ID"
  %s link --app "APP_ID" --promoted-purchase-id "PROMO_1,PROMO_2"
  %s link --app "APP_ID" --clear --confirm`, cfg.ProductPlural, cfg.ProductPlural, otherProductPlural(cfg.ProductType), cfg.PathPrefix, cfg.PathPrefix, cfg.PathPrefix)

	cmd.Exec = func(ctx context.Context, args []string) error {
		appID := shared.ResolveAppID(stringFlagValue(cmd.FlagSet, "app"))
		promotedIDs := stringFlagValue(cmd.FlagSet, "promoted-purchase-id")
		clear := boolFlagValue(cmd.FlagSet, "clear")
		confirm := boolFlagValue(cmd.FlagSet, "confirm")
		output := stringFlagValue(cmd.FlagSet, "output")
		pretty := boolFlagValue(cmd.FlagSet, "pretty")
		errorPrefix := promotedPurchasesCommandErrorPrefix(cfg, "link")

		if appID == "" {
			fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
			return flag.ErrHelp
		}

		var scopedIDs []string
		if clear {
			if strings.TrimSpace(promotedIDs) != "" {
				fmt.Fprintln(os.Stderr, "Error: --clear cannot be used with --promoted-purchase-id")
				return flag.ErrHelp
			}
			if !confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --clear")
				return flag.ErrHelp
			}
		} else {
			scopedIDs = shared.SplitCSV(promotedIDs)
			if len(scopedIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --promoted-purchase-id is required")
				return flag.ErrHelp
			}
		}

		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}

		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()

		preservedIDs, err := collectPreservedPromotedPurchaseIDs(requestCtx, client, appID, cfg.ProductType)
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}

		for _, id := range scopedIDs {
			if err := validatePromotedPurchaseScope(requestCtx, client, id, cfg, "link"); err != nil {
				return err
			}
		}

		finalIDs := preservedIDs
		if !clear {
			finalIDs = mergePromotedPurchaseIDs(preservedIDs, scopedIDs)
		}

		if err := client.SetAppPromotedPurchases(requestCtx, appID, finalIDs); err != nil {
			return fmt.Errorf("%s: failed to link: %w", errorPrefix, err)
		}

		action := "linked"
		if clear {
			action = "cleared"
		}
		result := &asc.AppPromotedPurchasesLinkResult{
			AppID:               appID,
			PromotedPurchaseIDs: finalIDs,
			Action:              action,
		}

		return shared.PrintOutput(result, output, pretty)
	}
}

func filterPromotedPurchasesByProductType(ctx context.Context, client *asc.Client, resp *asc.PromotedPurchasesResponse, productType promotedPurchaseProductType) error {
	if resp == nil {
		return nil
	}

	filtered := resp.Data[:0]
	for _, item := range resp.Data {
		scope, err := promotedPurchaseScopeForResource(ctx, client, item)
		if err != nil {
			return err
		}
		if scope.productType == productType {
			filtered = append(filtered, item)
		}
	}
	resp.Data = filtered
	return nil
}

func collectPreservedPromotedPurchaseIDs(ctx context.Context, client *asc.Client, appID string, scopedType promotedPurchaseProductType) ([]string, error) {
	firstPage, err := client.GetAppPromotedPurchases(ctx, appID, asc.WithPromotedPurchasesLimit(200))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current promoted purchases: %w", err)
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppPromotedPurchases(ctx, appID, asc.WithPromotedPurchasesNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate current promoted purchases: %w", err)
	}

	resp, ok := paginated.(*asc.PromotedPurchasesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", paginated)
	}

	preserved := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		scope, err := promotedPurchaseScopeForResource(ctx, client, item)
		if err != nil {
			return nil, err
		}
		if scope.productType != scopedType {
			preserved = append(preserved, strings.TrimSpace(item.ID))
		}
	}

	return preserved, nil
}

func validatePromotedPurchaseScope(ctx context.Context, client *asc.Client, promotedPurchaseID string, cfg ScopedPromotedPurchasesCommandConfig, action string) error {
	promotedPurchaseID = strings.TrimSpace(promotedPurchaseID)
	if promotedPurchaseID == "" {
		return nil
	}

	scope, err := promotedPurchaseScopeByID(ctx, client, promotedPurchaseID)
	if err != nil {
		return fmt.Errorf("%s: %w", promotedPurchasesCommandErrorPrefix(cfg, action), err)
	}
	if scope.productType != cfg.ProductType {
		return fmt.Errorf("%s: promoted purchase %q belongs to %s %q, not %s", promotedPurchasesCommandErrorPrefix(cfg, action), promotedPurchaseID, promotedPurchaseLabel(scope.productType), scope.productID, cfg.ProductSingular)
	}
	return nil
}

func promotedPurchaseScopeForResource(ctx context.Context, client *asc.Client, item asc.Resource[asc.PromotedPurchaseAttributes]) (promotedPurchaseScope, error) {
	if scope, ok, err := promotedPurchaseScopeFromRelationships(item.Relationships); ok || err != nil {
		return scope, err
	}
	return promotedPurchaseScopeByID(ctx, client, item.ID)
}

func promotedPurchaseScopeByID(ctx context.Context, client *asc.Client, promotedPurchaseID string) (promotedPurchaseScope, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetPromotedPurchase(requestCtx, strings.TrimSpace(promotedPurchaseID))
	if err != nil {
		return promotedPurchaseScope{}, fmt.Errorf("failed to fetch promoted purchase %q: %w", promotedPurchaseID, err)
	}
	scope, ok, err := promotedPurchaseScopeFromRelationships(resp.Data.Relationships)
	if err != nil {
		return promotedPurchaseScope{}, err
	}
	if !ok {
		return promotedPurchaseScope{}, fmt.Errorf("promoted purchase %q is missing product relationships", promotedPurchaseID)
	}
	return scope, nil
}

func promotedPurchaseScopeFromRelationships(raw json.RawMessage) (promotedPurchaseScope, bool, error) {
	if len(raw) == 0 {
		return promotedPurchaseScope{}, false, nil
	}

	var relationships promotedPurchaseRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return promotedPurchaseScope{}, false, fmt.Errorf("parse promoted purchase relationships: %w", err)
	}

	if relationships.InAppPurchaseV2 != nil {
		id := strings.TrimSpace(relationships.InAppPurchaseV2.Data.ID)
		if id != "" {
			return promotedPurchaseScope{productType: promotedPurchaseProductTypeInAppPurchase, productID: id}, true, nil
		}
	}
	if relationships.Subscription != nil {
		id := strings.TrimSpace(relationships.Subscription.Data.ID)
		if id != "" {
			return promotedPurchaseScope{productType: promotedPurchaseProductTypeSubscription, productID: id}, true, nil
		}
	}

	return promotedPurchaseScope{}, false, nil
}

func mergePromotedPurchaseIDs(preservedIDs, scopedIDs []string) []string {
	seen := make(map[string]struct{}, len(preservedIDs)+len(scopedIDs))
	merged := make([]string, 0, len(preservedIDs)+len(scopedIDs))
	for _, id := range append(append([]string{}, preservedIDs...), scopedIDs...) {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		merged = append(merged, trimmed)
	}
	return merged
}

func promotedPurchasesCommandErrorPrefix(cfg ScopedPromotedPurchasesCommandConfig, subcommand string) string {
	prefix := strings.TrimSpace(strings.TrimPrefix(cfg.PathPrefix, "asc "))
	if prefix == "" {
		return subcommand
	}
	if strings.TrimSpace(subcommand) == "" {
		return prefix
	}
	return prefix + " " + subcommand
}

func promotedPurchaseLabel(productType promotedPurchaseProductType) string {
	switch productType {
	case promotedPurchaseProductTypeInAppPurchase:
		return "in-app purchase"
	case promotedPurchaseProductTypeSubscription:
		return "subscription"
	default:
		return "unknown product"
	}
}

func otherProductPlural(productType promotedPurchaseProductType) string {
	switch productType {
	case promotedPurchaseProductTypeInAppPurchase:
		return "subscriptions"
	case promotedPurchaseProductTypeSubscription:
		return "in-app purchases"
	default:
		return "other products"
	}
}

func stringFlagValue(fs *flag.FlagSet, name string) string {
	if fs == nil {
		return ""
	}
	if f := fs.Lookup(name); f != nil {
		return strings.TrimSpace(f.Value.String())
	}
	return ""
}

func intFlagValue(fs *flag.FlagSet, name string) int {
	value := stringFlagValue(fs, name)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func boolFlagValue(fs *flag.FlagSet, name string) bool {
	value := stringFlagValue(fs, name)
	if value == "" {
		return false
	}
	parsed, _ := strconv.ParseBool(value)
	return parsed
}
