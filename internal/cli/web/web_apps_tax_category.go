package web

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const defaultAppTaxCategoryName = "App Store Software"

var (
	listWebAppTaxCategoriesFn = func(ctx context.Context, client *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return client.ListTaxCategories(ctx)
	}
	getWebAppTaxCategoryFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppTaxCategory, error) {
		return client.GetAppTaxCategory(ctx, appID)
	}
	saveWebAppTaxCategoryFn = func(ctx context.Context, client *webcore.Client, appID, categoryID string, conditionIDs []string, configured bool) error {
		return client.SaveAppTaxCategory(ctx, appID, categoryID, conditionIDs, configured)
	}
)

// WebAppsTaxCategoryCommand returns the App Information tax-category command
// group. The underlying endpoints are Apple's internal web-session API.
func WebAppsTaxCategoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps tax-category", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "tax-category",
		ShortUsage: "asc web apps tax-category <subcommand> [flags]",
		ShortHelp:  "Read or set an app's App Store tax category via a web session.",
		LongHelp: `WEB SESSION WORKFLOWS

Read and set the App Information tax category through Apple's internal
web-session API. This surface is separate from the public App Store Connect
API.

Use ` + "`list`" + ` to inspect Apple's application tax-category catalog, ` + "`view`" + `
to read the app's explicit selection, and ` + "`set`" + ` to converge the app on a
category and complete condition set. A missing explicit selection is reported
as Apple's App Store Software default. Omit ` + "`--condition`" + ` on ` + "`set`" + ` to
send an explicit empty condition relationship and clear stale conditions.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsTaxCategoryListCommand(),
			WebAppsTaxCategoryViewCommand(),
			WebAppsTaxCategorySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsTaxCategoryListCommand lists the captured application tax catalog.
func WebAppsTaxCategoryListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps tax-category list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web apps tax-category list [flags]",
		ShortHelp:  "List application tax categories and compatible conditions.",
		LongHelp: `WEB SESSION WORKFLOWS

List the application tax categories and conditions exposed by the App Store
Connect App Information tax picker. The IDs returned here are the opaque IDs
accepted by ` + "`set`" + `.

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
			err = withWebSpinner("Fetching application tax category catalog", func() error {
				catalog, err = listWebAppTaxCategoriesFn(requestCtx, newWebClientFn(session))
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps tax-category list")
			}
			return printWebAppTaxCategoryCatalog(catalog, *output.Output, *output.Pretty)
		},
	}
}

// WebAppsTaxCategoryViewCommand reads the app's current tax selection and its
// effective UI default when no explicit resource exists.
func WebAppsTaxCategoryViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps tax-category view", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web apps tax-category view --app APP_ID [flags]",
		ShortHelp:  "View an app's explicit and effective tax category.",
		LongHelp: `WEB SESSION WORKFLOWS

Read the app's explicit App Information tax category and enabled conditions.
When Apple's appTaxCategories resource is absent, ` + "`configured`" + ` is false and
the effective category is reported as App Store Software, matching the UI
default observed in App Store Connect.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			var current *webcore.AppTaxCategory
			var catalog webcore.TaxCategoryCatalog
			err = withWebSpinner("Fetching application tax category", func() error {
				current, err = getWebAppTaxCategoryFn(requestCtx, client, resolvedAppID)
				if err != nil {
					return err
				}
				catalog, err = listWebAppTaxCategoriesFn(requestCtx, client)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps tax-category view")
			}
			if current == nil {
				return fmt.Errorf("web apps tax-category view failed: missing current selection")
			}

			result := asc.WebAppTaxCategoryViewResult{
				AppID:               resolvedAppID,
				Configured:          current.Configured,
				CategoryID:          strings.TrimSpace(current.CategoryID),
				CategoryName:        strings.TrimSpace(current.CategoryName),
				EnabledConditionIDs: append([]string(nil), current.EnabledConditionIDs...),
			}
			if current.Configured {
				result.EffectiveCategoryID = result.CategoryID
				result.EffectiveCategoryName = result.CategoryName
				if result.EffectiveCategoryName == "" {
					result.EffectiveCategoryName = taxCategoryName(catalog.Categories, result.EffectiveCategoryID)
				}
			} else {
				result.EffectiveCategoryName = defaultAppTaxCategoryName
				result.EffectiveCategoryID = taxCategoryIDByName(catalog.Categories, defaultAppTaxCategoryName)
			}
			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// WebAppsTaxCategorySetCommand writes a complete desired tax category and
// condition set. It never retries an ambiguous write.
func WebAppsTaxCategorySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps tax-category set", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	categoryID := fs.String("category", "", "Tax category resource ID from `asc web apps tax-category list`")
	var conditionIDs shared.MultiStringFlag
	fs.Var(&conditionIDs, "condition", "Compatible tax condition resource ID (repeatable; omit to clear conditions)")
	confirm := fs.Bool("confirm", false, "Confirm changing the app's tax category and conditions")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps tax-category set --app APP_ID --category CATEGORY_ID [--condition CONDITION_ID ...] --confirm [flags]",
		ShortHelp:  "Set an app's tax category and complete condition set.",
		LongHelp: `WEB SESSION WORKFLOWS

Set the App Information tax category through Apple's internal web-session API.
The category and every ` + "`--condition`" + ` value are validated against the
captured application catalog before the write. The desired condition set is
declarative: omitting ` + "`--condition`" + ` sends enabledConditions.data=[] and
clears existing conditions. The result is re-read and verified. If Apple returns
an ambiguous write failure, do not retry automatically.

Examples:
  asc web apps tax-category set --app "123456789" --category "CATEGORY_ID" --confirm
  asc web apps tax-category set --app "123456789" --category "CATEGORY_ID" --condition "CONDITION_ID" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			resolvedCategoryID := strings.TrimSpace(*categoryID)
			if resolvedCategoryID == "" {
				return shared.UsageError("--category is required")
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

			var catalog webcore.TaxCategoryCatalog
			var current *webcore.AppTaxCategory
			err = withWebSpinner("Reading application tax category", func() error {
				catalog, err = listWebAppTaxCategoriesFn(requestCtx, client)
				if err != nil {
					return err
				}
				current, err = getWebAppTaxCategoryFn(requestCtx, client, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps tax-category set")
			}
			selectedCategory := findTaxCategory(catalog.Categories, resolvedCategoryID)
			if selectedCategory == nil {
				return fmt.Errorf("web apps tax-category set: category %q was not returned by the application tax catalog", resolvedCategoryID)
			}
			if err := validateWebTaxCategorySelection(catalog.Categories, selectedCategory); err != nil {
				return err
			}
			desiredConditions, err := normalizeWebTaxConditionIDs([]string(conditionIDs))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if err := validateWebTaxConditions(catalog.Categories, selectedCategory, desiredConditions); err != nil {
				return err
			}

			changed := current == nil || !current.Configured || strings.TrimSpace(current.CategoryID) != resolvedCategoryID || !sameWebTaxConditionSet(current.EnabledConditionIDs, desiredConditions)
			if changed {
				err = withWebSpinner("Saving application tax category", func() error {
					return saveWebAppTaxCategoryFn(requestCtx, client, resolvedAppID, resolvedCategoryID, desiredConditions, current != nil && current.Configured)
				})
				if err != nil {
					return fmt.Errorf("web apps tax-category set failed: write outcome may be ambiguous; do not retry automatically: %w", err)
				}
				current, err = getWebAppTaxCategoryFn(requestCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("web apps tax-category set failed: write may have succeeded but post-read verification failed; do not retry automatically: %w", err)
				}
			}
			if current == nil || !current.Configured || strings.TrimSpace(current.CategoryID) != resolvedCategoryID || !sameWebTaxConditionSet(current.EnabledConditionIDs, desiredConditions) {
				return fmt.Errorf("web apps tax-category set failed: Apple did not confirm category %q and the requested condition set; do not retry automatically", resolvedCategoryID)
			}

			receipt := &asc.WebAppTaxCategorySetResult{
				AppID:        resolvedAppID,
				CategoryID:   resolvedCategoryID,
				CategoryName: strings.TrimSpace(selectedCategory.Name),
				ConditionIDs: desiredConditions,
				Changed:      changed,
				Verified:     true,
			}
			if err := shared.PrintOutput(receipt, *output.Output, *output.Pretty); err != nil {
				conditionSummary := strings.Join(receipt.ConditionIDs, ",")
				if receipt.Changed {
					return fmt.Errorf("web apps tax-category set app %q category %q condition set %q was written and verified, but receipt output failed; do not retry automatically: %w", receipt.AppID, receipt.CategoryID, conditionSummary, err)
				}
				return fmt.Errorf("web apps tax-category set app %q category %q condition set %q already matched and was verified; no write occurred, but receipt output failed; do not retry automatically: %w", receipt.AppID, receipt.CategoryID, conditionSummary, err)
			}
			return nil
		},
	}
}

func findTaxCategory(categories []webcore.TaxCategory, id string) *webcore.TaxCategory {
	for i := range categories {
		if strings.TrimSpace(categories[i].ID) == id {
			return &categories[i]
		}
	}
	return nil
}

func validateWebTaxCategorySelection(categories []webcore.TaxCategory, selected *webcore.TaxCategory) error {
	if selected == nil || !selected.SubcategoryRequired {
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
	return fmt.Errorf("web apps tax-category set: category %q requires a subcategory selection; select one of its listed subcategories", selectedID)
}

func taxCategoryName(categories []webcore.TaxCategory, id string) string {
	if category := findTaxCategory(categories, id); category != nil {
		return strings.TrimSpace(category.Name)
	}
	return ""
}

func taxCategoryIDByName(categories []webcore.TaxCategory, name string) string {
	for _, category := range categories {
		if strings.EqualFold(strings.TrimSpace(category.Name), name) {
			return strings.TrimSpace(category.ID)
		}
	}
	return ""
}

func normalizeWebTaxConditionIDs(conditionIDs []string) ([]string, error) {
	result := make([]string, 0, len(conditionIDs))
	seen := make(map[string]struct{}, len(conditionIDs))
	for _, conditionID := range conditionIDs {
		conditionID = strings.TrimSpace(conditionID)
		if conditionID == "" {
			return nil, fmt.Errorf("--condition must not be empty")
		}
		if _, exists := seen[conditionID]; exists {
			continue
		}
		seen[conditionID] = struct{}{}
		result = append(result, conditionID)
	}
	return result, nil
}

func normalizedWebTaxConditionIDsForComparison(conditionIDs []string) []string {
	result := make([]string, 0, len(conditionIDs))
	seen := make(map[string]struct{}, len(conditionIDs))
	for _, conditionID := range conditionIDs {
		conditionID = strings.TrimSpace(conditionID)
		if conditionID == "" {
			continue
		}
		if _, exists := seen[conditionID]; exists {
			continue
		}
		seen[conditionID] = struct{}{}
		result = append(result, conditionID)
	}
	return result
}

func effectiveWebTaxConditions(categories []webcore.TaxCategory, category *webcore.TaxCategory) []webcore.TaxCategoryReference {
	if category == nil {
		return nil
	}
	conditions := make([]webcore.TaxCategoryReference, 0, len(category.Conditions))
	seen := make(map[string]struct{}, len(category.Conditions))
	addConditions := func(candidates []webcore.TaxCategoryReference) {
		for _, condition := range candidates {
			id := strings.TrimSpace(condition.ID)
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			conditions = append(conditions, condition)
		}
	}

	addConditions(category.Conditions)
	selectedID := strings.TrimSpace(category.ID)
	for _, parent := range categories {
		for _, subcategory := range parent.Subcategories {
			if strings.TrimSpace(subcategory.ID) == selectedID {
				addConditions(parent.Conditions)
				break
			}
		}
	}
	return conditions
}

func validateWebTaxConditions(categories []webcore.TaxCategory, category *webcore.TaxCategory, conditionIDs []string) error {
	if category == nil {
		return fmt.Errorf("web apps tax-category set: selected category is missing")
	}
	compatible := make(map[string]struct{}, len(category.Conditions))
	for _, condition := range effectiveWebTaxConditions(categories, category) {
		if id := strings.TrimSpace(condition.ID); id != "" {
			compatible[id] = struct{}{}
		}
	}
	for _, conditionID := range conditionIDs {
		if _, ok := compatible[conditionID]; !ok {
			return fmt.Errorf("web apps tax-category set: condition %q is not compatible with category %q according to the application tax catalog", conditionID, strings.TrimSpace(category.ID))
		}
	}
	return nil
}

func sameWebTaxConditionSet(left, right []string) bool {
	left = normalizedWebTaxConditionIDsForComparison(left)
	right = normalizedWebTaxConditionIDsForComparison(right)
	if len(left) != len(right) {
		return false
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func printWebAppTaxCategoryCatalog(result webcore.TaxCategoryCatalog, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			rows := make([][]string, 0, len(result.Categories)+len(result.Conditions))
			for index := range result.Categories {
				category := &result.Categories[index]
				rows = append(rows, []string{"category", category.ID, category.Name, category.ProductType, fmt.Sprintf("%t", category.SubcategoryRequired), taxCategoryConditionIDs(effectiveWebTaxConditions(result.Categories, category))})
			}
			for _, condition := range result.Conditions {
				rows = append(rows, []string{"condition", condition.ID, condition.Name, "", "", ""})
			}
			asc.RenderTable([]string{"kind", "id", "name", "product_type", "subcategory_required", "compatible_conditions"}, rows)
			return nil
		},
		func() error {
			rows := make([][]string, 0, len(result.Categories)+len(result.Conditions))
			for index := range result.Categories {
				category := &result.Categories[index]
				rows = append(rows, []string{"category", category.ID, category.Name, category.ProductType, fmt.Sprintf("%t", category.SubcategoryRequired), taxCategoryConditionIDs(effectiveWebTaxConditions(result.Categories, category))})
			}
			for _, condition := range result.Conditions {
				rows = append(rows, []string{"condition", condition.ID, condition.Name, "", "", ""})
			}
			asc.RenderMarkdown([]string{"kind", "id", "name", "product_type", "subcategory_required", "compatible_conditions"}, rows)
			return nil
		},
	)
}

func taxCategoryConditionIDs(conditions []webcore.TaxCategoryReference) string {
	ids := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		if id := strings.TrimSpace(condition.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return strings.Join(ids, ",")
}
