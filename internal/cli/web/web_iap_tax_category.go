package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const iapTaxCategoryInheritedLabel = "Inherited from parent app"

// WebIAPCommand returns the In-App Purchase web-session command
// group.
func WebIAPCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "iap",
		ShortUsage: "asc web iap <subcommand> [flags]",
		ShortHelp:  "Manage In-App Purchase tax categories via a web session.",
		LongHelp: `WEB SESSION WORKFLOWS

Read and change an In-App Purchase tax category through Apple's internal
web-session API. The public App Store Connect API does not expose this
resource. An In-App Purchase without an explicit selection inherits the parent
app's tax category; the CLI reports that state without guessing the inherited
category value.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebIAPTaxCategoryCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebIAPTaxCategoryCommand returns the IAP tax-category command
// group.
func WebIAPTaxCategoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap tax-category", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "tax-category",
		ShortUsage: "asc web iap tax-category <subcommand> [flags]",
		ShortHelp:  "Read or set an In-App Purchase tax category.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect or change the tax category and compatible conditions assigned to one
In-App Purchase. ` + "`list`" + ` reads the ADDON tax-category catalog, ` + "`view`" + `
reads the current explicit selection, ` + "`set`" + ` converges a complete category
and condition set, and ` + "`reset`" + ` removes the explicit selection so the
purchase inherits its parent app's tax category.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebIAPTaxCategoryListCommand(),
			WebIAPTaxCategoryViewCommand(),
			WebIAPTaxCategorySetCommand(),
			WebIAPTaxCategoryResetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebIAPTaxCategoryListCommand lists the ADDON tax-category catalog.
func WebIAPTaxCategoryListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap tax-category list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web iap tax-category list [flags]",
		ShortHelp:  "List In-App Purchase tax categories and conditions.",
		LongHelp: `WEB SESSION WORKFLOWS

List the ADDON tax categories and conditions exposed by Apple's In-App Purchase
tax picker. The opaque IDs returned here are accepted by ` + "`set`" + `.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var catalog webcore.TaxCategoryCatalog
			err = withWebSpinner("Fetching In-App Purchase tax category catalog", func() error {
				catalog, err = newWebClientFn(session).ListIAPTaxCategories(requestCtx)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web iap tax-category list")
			}
			return printWebAppTaxCategoryCatalog(catalog, *output.Output, *output.Pretty)
		},
	}
}

// WebIAPTaxCategoryViewCommand reads an IAP's explicit selection. A missing
// explicit selection is rendered as an inherited state for human output; the
// JSON path preserves the raw Apple response and does not infer a category.
func WebIAPTaxCategoryViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap tax-category view", flag.ExitOnError)
	iapID := fs.String("iap", "", "In-App Purchase ID")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web iap tax-category view --iap IAP_ID [flags]",
		ShortHelp:  "View an In-App Purchase tax category.",
		LongHelp: `WEB SESSION WORKFLOWS

Read the explicit tax category and enabled conditions for one In-App Purchase.
When the explicit resource is absent, human output labels the selection as
inherited from the parent app. JSON output preserves Apple's response and does
not add an inferred effective-category field.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedIAPID := strings.TrimSpace(*iapID)
			if resolvedIAPID == "" {
				return shared.UsageError("--iap is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var current *webcore.IAPTaxCategory
			err = withWebSpinner("Fetching In-App Purchase tax category", func() error {
				current, err = newWebClientFn(session).GetIAPTaxCategory(requestCtx, resolvedIAPID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web iap tax-category view")
			}
			if current == nil {
				return fmt.Errorf("web iap tax-category view failed: missing current selection")
			}
			if !iapTaxCategoryIdentityMatches(current, resolvedIAPID) {
				return fmt.Errorf("web iap tax-category view failed: response did not identify In-App Purchase %q", resolvedIAPID)
			}

			return shared.PrintOutputWithRenderers(
				current,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable(webIAPTaxCategoryViewHeaders(), webIAPTaxCategoryViewRows(current))
					return nil
				},
				func() error {
					asc.RenderMarkdown(webIAPTaxCategoryViewHeaders(), webIAPTaxCategoryViewRows(current))
					return nil
				},
			)
		},
	}
}

// WebIAPTaxCategorySetCommand converges a complete desired IAP category and
// condition set. It performs one preflight read, writes at most once, and
// verifies the resulting state without automatic retries.
func WebIAPTaxCategorySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap tax-category set", flag.ExitOnError)
	iapID := fs.String("iap", "", "In-App Purchase ID")
	categoryID := fs.String("category", "", "ADDON tax category resource ID from `asc web iap tax-category list`")
	var conditionIDs shared.MultiStringFlag
	fs.Var(&conditionIDs, "condition", "Compatible tax condition resource ID (repeatable; omit to clear conditions)")
	confirm := fs.Bool("confirm", false, "Confirm changing the In-App Purchase tax category and conditions")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web iap tax-category set --iap IAP_ID --category CATEGORY_ID [--condition CONDITION_ID ...] --confirm [flags]",
		ShortHelp:  "Set an In-App Purchase tax category and condition set.",
		LongHelp: `WEB SESSION WORKFLOWS

Set the In-App Purchase tax category through Apple's internal web-session API.
The selected category and every ` + "`--condition`" + ` value are validated against
the ADDON catalog before any write. The desired condition set is declarative:
omitting ` + "`--condition`" + ` sends an explicit empty relationship and clears
stale conditions. The result is re-read and verified. An ambiguous write is
reported without an automatic retry.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedIAPID := strings.TrimSpace(*iapID)
			if resolvedIAPID == "" {
				return shared.UsageError("--iap is required")
			}
			resolvedCategoryID := strings.TrimSpace(*categoryID)
			if resolvedCategoryID == "" {
				return shared.UsageError("--category is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			desiredConditions, err := normalizeWebTaxConditionIDs([]string(conditionIDs))
			if err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			var catalog webcore.TaxCategoryCatalog
			var current *webcore.IAPTaxCategory
			err = withWebSpinner("Reading In-App Purchase tax category", func() error {
				catalog, err = client.ListIAPTaxCategories(requestCtx)
				if err != nil {
					return err
				}
				current, err = client.GetIAPTaxCategory(requestCtx, resolvedIAPID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web iap tax-category set")
			}
			if current == nil {
				return fmt.Errorf("web iap tax-category set failed: missing preflight selection for In-App Purchase %q", resolvedIAPID)
			}
			if !iapTaxCategoryIdentityMatches(current, resolvedIAPID) {
				return fmt.Errorf("web iap tax-category set failed: preflight response did not identify In-App Purchase %q", resolvedIAPID)
			}

			selectedCategory := findIAPTaxCategory(catalog.Categories, resolvedCategoryID)
			if selectedCategory == nil {
				return shared.UsageErrorf("category %q was not returned by the ADDON tax catalog", resolvedCategoryID)
			}
			if err := validateIAPTaxCategorySelection(catalog.Categories, selectedCategory); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := validateIAPTaxConditions(catalog.Categories, selectedCategory, desiredConditions); err != nil {
				return shared.UsageError(err.Error())
			}

			changed := current == nil || !current.Configured || strings.TrimSpace(current.CategoryID) != resolvedCategoryID || !sameWebTaxConditionSet(current.EnabledConditionIDs, desiredConditions)
			if changed {
				err = withWebSpinner("Saving In-App Purchase tax category", func() error {
					return client.SaveIAPTaxCategory(requestCtx, resolvedIAPID, resolvedCategoryID, desiredConditions, current)
				})
				if err != nil {
					return fmt.Errorf("web iap tax-category set failed: write outcome may be ambiguous; do not retry automatically: %w", withWebAuthHint(err, "web iap tax-category set"))
				}
				current, err = client.GetIAPTaxCategory(requestCtx, resolvedIAPID)
				if err != nil {
					return fmt.Errorf("web iap tax-category set failed: write may have succeeded but post-read verification failed; do not retry automatically: %w", withWebAuthHint(err, "web iap tax-category set"))
				}
			}
			if !iapTaxCategoryStateMatches(current, resolvedIAPID, resolvedCategoryID, desiredConditions) {
				return fmt.Errorf("web iap tax-category set failed: Apple did not confirm In-App Purchase %q category %q and the requested condition set; do not retry automatically", resolvedIAPID, resolvedCategoryID)
			}

			receipt := &asc.WebIAPTaxCategorySetResult{
				IAPID:        resolvedIAPID,
				CategoryID:   resolvedCategoryID,
				CategoryName: strings.TrimSpace(selectedCategory.Name),
				ConditionIDs: desiredConditions,
				Changed:      changed,
				Verified:     true,
			}
			if err := shared.PrintOutput(receipt, *output.Output, *output.Pretty); err != nil {
				return fmt.Errorf("web iap tax-category set In-App Purchase %q category %q was %s and verified, but receipt output failed; do not retry automatically: %w", receipt.IAPID, receipt.CategoryID, iapTaxCategoryWriteSummary(receipt.Changed), err)
			}
			return nil
		},
	}
}

// WebIAPTaxCategoryResetCommand removes an explicit IAP tax-category resource.
// It never sends DELETE for an already inherited state.
func WebIAPTaxCategoryResetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web iap tax-category reset", flag.ExitOnError)
	iapID := fs.String("iap", "", "In-App Purchase ID")
	confirm := fs.Bool("confirm", false, "Confirm removing the In-App Purchase tax category")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "reset",
		ShortUsage: "asc web iap tax-category reset --iap IAP_ID --confirm [flags]",
		ShortHelp:  "Reset an In-App Purchase tax category to inherited state.",
		LongHelp: `WEB SESSION WORKFLOWS

Remove the explicit In-App Purchase tax-category resource so the purchase
inherits its parent app's selection. The command reads first, sends DELETE only
when an explicit resource is configured, and verifies an explicit null result.
An ambiguous delete is reported without an automatic retry.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedIAPID := strings.TrimSpace(*iapID)
			if resolvedIAPID == "" {
				return shared.UsageError("--iap is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			var current *webcore.IAPTaxCategory
			err = withWebSpinner("Reading In-App Purchase tax category", func() error {
				current, err = client.GetIAPTaxCategory(requestCtx, resolvedIAPID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web iap tax-category reset")
			}
			if current == nil {
				return fmt.Errorf("web iap tax-category reset failed: missing preflight selection for In-App Purchase %q", resolvedIAPID)
			}
			if !iapTaxCategoryIdentityMatches(current, resolvedIAPID) {
				return fmt.Errorf("web iap tax-category reset failed: preflight response did not identify In-App Purchase %q", resolvedIAPID)
			}

			changed := current.Configured
			if changed {
				err = withWebSpinner("Resetting In-App Purchase tax category", func() error {
					return client.DeleteIAPTaxCategory(requestCtx, resolvedIAPID, current)
				})
				if err != nil {
					return fmt.Errorf("web iap tax-category reset failed: delete outcome may be ambiguous; do not retry automatically: %w", withWebAuthHint(err, "web iap tax-category reset"))
				}
				current, err = client.GetIAPTaxCategory(requestCtx, resolvedIAPID)
				if err != nil {
					return fmt.Errorf("web iap tax-category reset failed: delete may have succeeded but post-read verification failed; do not retry automatically: %w", withWebAuthHint(err, "web iap tax-category reset"))
				}
			}
			if !iapTaxCategoryIsExplicitNull(current, resolvedIAPID) {
				return fmt.Errorf("web iap tax-category reset failed: Apple did not confirm an explicit null tax category for In-App Purchase %q; do not retry automatically", resolvedIAPID)
			}

			receipt := &asc.WebIAPTaxCategoryResetResult{
				IAPID:    resolvedIAPID,
				Changed:  changed,
				Verified: true,
			}
			if err := shared.PrintOutput(receipt, *output.Output, *output.Pretty); err != nil {
				return fmt.Errorf("web iap tax-category reset In-App Purchase %q was %s and verified, but receipt output failed; do not retry automatically: %w", receipt.IAPID, iapTaxCategoryWriteSummary(receipt.Changed), err)
			}
			return nil
		},
	}
}

func findIAPTaxCategory(categories []webcore.TaxCategory, id string) *webcore.TaxCategory {
	for index := range categories {
		category := &categories[index]
		if strings.TrimSpace(category.ID) != id {
			continue
		}
		productType := strings.TrimSpace(category.ProductType)
		if !strings.EqualFold(productType, "ADDON") {
			return nil
		}
		return category
	}
	return nil
}

func validateIAPTaxCategorySelection(categories []webcore.TaxCategory, selected *webcore.TaxCategory) error {
	if selected == nil {
		return fmt.Errorf("selected category is missing from the ADDON tax catalog")
	}
	if !selected.SubcategoryRequired {
		return nil
	}
	selectedID := strings.TrimSpace(selected.ID)
	for _, category := range categories {
		for _, subcategory := range category.Subcategories {
			if strings.TrimSpace(subcategory.ID) == selectedID {
				return nil
			}
		}
	}
	return fmt.Errorf("category %q requires a subcategory selection; select one of its listed subcategories", selectedID)
}

func validateIAPTaxConditions(categories []webcore.TaxCategory, category *webcore.TaxCategory, conditionIDs []string) error {
	if category == nil {
		return fmt.Errorf("selected category is missing from the ADDON tax catalog")
	}
	compatible := make(map[string]struct{}, len(category.Conditions))
	for _, condition := range effectiveWebTaxConditions(categories, category) {
		if condition.Type != "taxConditions" {
			continue
		}
		if id := strings.TrimSpace(condition.ID); id != "" {
			compatible[id] = struct{}{}
		}
	}
	for _, conditionID := range conditionIDs {
		if _, ok := compatible[conditionID]; !ok {
			return fmt.Errorf("condition %q is not compatible with ADDON category %q according to the tax catalog", conditionID, strings.TrimSpace(category.ID))
		}
	}
	return nil
}

func webIAPTaxCategoryViewHeaders() []string {
	return []string{"Field", "Value"}
}

func webIAPTaxCategoryViewRows(result *webcore.IAPTaxCategory) [][]string {
	if result == nil {
		return nil
	}
	categoryName := strings.TrimSpace(result.CategoryName)
	if !result.Configured {
		categoryName = iapTaxCategoryInheritedLabel
	}
	return [][]string{
		{"IAP ID", strings.TrimSpace(result.IAPID)},
		{"ID", strings.TrimSpace(result.ID)},
		{"Configured", fmt.Sprintf("%t", result.Configured)},
		{"Category ID", strings.TrimSpace(result.CategoryID)},
		{"Category Name", categoryName},
		{"Enabled Condition IDs", strings.Join(result.EnabledConditionIDs, ",")},
	}
}

func iapTaxCategoryIdentityMatches(result *webcore.IAPTaxCategory, iapID string) bool {
	if result == nil {
		return false
	}
	identifier := strings.TrimSpace(result.IAPID)
	if identifier == "" {
		return false
	}
	return identifier == iapID
}

func iapTaxCategoryStateMatches(result *webcore.IAPTaxCategory, iapID, categoryID string, conditionIDs []string) bool {
	return iapTaxCategoryIdentityMatches(result, iapID) && result.Configured && strings.TrimSpace(result.CategoryID) == categoryID && sameWebTaxConditionSet(result.EnabledConditionIDs, conditionIDs)
}

func iapTaxCategoryIsExplicitNull(result *webcore.IAPTaxCategory, iapID string) bool {
	return iapTaxCategoryIdentityMatches(result, iapID) && !result.Configured && strings.TrimSpace(result.CategoryID) == "" && len(normalizedWebTaxConditionIDsForComparison(result.EnabledConditionIDs)) == 0
}

func iapTaxCategoryWriteSummary(changed bool) string {
	if changed {
		return "written"
	}
	return "already matched"
}
