package xcodecloud

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeCloudProductsCommand returns the xcode-cloud products command with subcommands.
func newXcodeCloudProductsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("products", flag.ExitOnError)

	flags := bindCiProductsListFlags(d, fs)

	return &ffcli.Command{
		Name:       "products",
		ShortUsage: "asc xcode-cloud products [flags]",
		ShortHelp:  "Manage Xcode Cloud products.",
		LongHelp: `Manage Xcode Cloud products.

Examples:
  asc xcode-cloud products --app "APP_ID"
  asc xcode-cloud products list --app "APP_ID"
  asc xcode-cloud products list --product-type APP --include app
  asc xcode-cloud products view --id "PRODUCT_ID"
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newXcodeCloudProductsListCommand(d),
			newXcodeCloudProductsGetCommand(d),
			newXcodeCloudProductsAppCommand(d),
			newXcodeCloudProductsBuildRunsCommand(d),
			newXcodeCloudProductsWorkflowsCommand(d),
			newXcodeCloudProductsPrimaryRepositoriesCommand(d),
			newXcodeCloudProductsAdditionalRepositoriesCommand(d),
			newXcodeCloudProductsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, flags.flagSet, "xcode-cloud products", 200, "product-type", "fields", "app-fields", "bundle-id-fields", "scm-repository-fields", "include", "primary-repositories-limit", "limit"); err != nil {
				return err
			}
			return xcodeCloudProductsList(ctx, d, flags)
		},
	}
}

func newXcodeCloudProductsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	flags := bindCiProductsListFlags(d, fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud products list [flags]",
		ShortHelp:  "List Xcode Cloud products.",
		LongHelp: `List Xcode Cloud products.

Examples:
  asc xcode-cloud products list
  asc xcode-cloud products list --app "APP_ID"
  asc xcode-cloud products list --product-type APP --include app
  asc xcode-cloud products list --fields "name,productType" --output json
  asc xcode-cloud products list --limit 50
  asc xcode-cloud products list --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, flags.flagSet, "xcode-cloud products list", 200, "product-type", "fields", "app-fields", "bundle-id-fields", "scm-repository-fields", "include", "primary-repositories-limit", "limit"); err != nil {
				return err
			}
			return xcodeCloudProductsList(ctx, d, flags)
		},
	}
}

func newXcodeCloudProductsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud products view --id \"PRODUCT_ID\"",
		ShortHelp:  "View details for a product.",
		LongHelp: `View details for a product.

Examples:
  asc xcode-cloud products view --id "PRODUCT_ID"
  asc xcode-cloud products view --id "PRODUCT_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Product ID",
		IDType:           "ciProducts",
		FetchErrorPrefix: "xcode-cloud products view",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiProduct(ctx, id)
		},
	})
}

func newXcodeCloudProductsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "ciProducts", "Product ID")
	appInfoFields := fs.String("app-info-fields", "", "Sparse fields for included app info records: "+strings.Join(shared.AppInfoSparseFields, ", ")+" (deprecated; removed from API 4.5; prefer age-rating data)")
	iapFields := fs.String("iap-fields", "", "Sparse fields for included in-app purchases: "+strings.Join(shared.AppInAppPurchaseSparseFields, ", "))
	subscriptionGroupFields := fs.String("subscription-group-fields", "", "Sparse fields for included subscription groups: "+strings.Join(shared.SubscriptionGroupSparseFields, ", "))
	retentionMessageFields := fs.String("retention-message-fields", "", "Sparse fields for included retention messages: "+strings.Join(shared.RetentionMessageSparseFields, ", "))
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc xcode-cloud products app --id \"PRODUCT_ID\"",
		ShortHelp:  "Get the app for a product.",
		LongHelp: `Get the app for a product.

Examples:
  asc xcode-cloud products app --id "PRODUCT_ID"
  asc xcode-cloud products app --id "PRODUCT_ID" --app-info-fields state,primaryCategory --iap-fields versions
  asc xcode-cloud products app --id "PRODUCT_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.UsageError("--id is required")
			}
			appInfoFieldValues, err := normalizeCiProductAppSparseField(fs, *appInfoFields, shared.AppInfoSparseFields, "--app-info-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			iapFieldValues, err := normalizeCiProductAppSparseField(fs, *iapFields, shared.AppInAppPurchaseSparseFields, "--iap-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			groupFieldValues, err := normalizeCiProductAppSparseField(fs, *subscriptionGroupFields, shared.SubscriptionGroupSparseFields, "--subscription-group-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}
			retentionMessageFieldValues, err := normalizeCiProductAppSparseField(fs, *retentionMessageFields, shared.RetentionMessageSparseFields, "--retention-message-fields")
			if err != nil {
				return d.UsageError(err.Error())
			}

			d.WarnDeprecatedAppInfoFields(appInfoFieldValues, "")

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: %w", err)
			}
			requestCtx, cancel := contextWithXcodeCloudTimeout(d, ctx, 0)
			defer cancel()

			includeValues := []string{}
			if len(appInfoFieldValues) > 0 {
				includeValues = addCiProductAppInclude(includeValues, "appInfos")
			}
			if len(iapFieldValues) > 0 {
				includeValues = addCiProductAppInclude(includeValues, "inAppPurchases")
			}
			if len(groupFieldValues) > 0 {
				includeValues = addCiProductAppInclude(includeValues, "subscriptionGroups")
			}
			if len(retentionMessageFieldValues) > 0 {
				includeValues = addCiProductAppInclude(includeValues, "retentionMessages")
			}
			resp, err := client.GetCiProductApp(
				requestCtx, idValue,
				asc.WithCiProductAppAppInfoFields(appInfoFieldValues),
				asc.WithCiProductAppInAppPurchaseFields(iapFieldValues),
				asc.WithCiProductAppSubscriptionGroupFields(groupFieldValues),
				asc.WithCiProductAppRetentionMessageFields(retentionMessageFieldValues),
				asc.WithCiProductAppInclude(includeValues),
			)
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: failed to fetch: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newXcodeCloudProductsBuildRunsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "build-runs",
		ShortUsage: "asc xcode-cloud products build-runs [flags]",
		ShortHelp:  "List build runs for a product.",
		LongHelp: `List build runs for a product.

Examples:
  asc xcode-cloud products build-runs --id "PRODUCT_ID"
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "Product ID",
		ParentType:       "ciProducts",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud products build-runs",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiBuildRunsOption{
				asc.Limit[asc.CiBuildRunsOption](limit),
				asc.NextURL[asc.CiBuildRunsOption](next),
			}
			return client.GetCiProductBuildRuns(ctx, productID, opts...)
		},
	})
}

func newXcodeCloudProductsWorkflowsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "workflows",
		ShortUsage: "asc xcode-cloud products workflows [flags]",
		ShortHelp:  "List workflows for a product.",
		LongHelp: `List workflows for a product.

Examples:
  asc xcode-cloud products workflows --id "PRODUCT_ID"
  asc xcode-cloud products workflows --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products workflows --id "PRODUCT_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "Product ID",
		ParentType:       "ciProducts",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud products workflows",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiWorkflowsOption{
				asc.Limit[asc.CiWorkflowsOption](limit),
				asc.NextURL[asc.CiWorkflowsOption](next),
			}
			return client.GetCiWorkflows(ctx, productID, opts...)
		},
	})
}

func newXcodeCloudProductsPrimaryRepositoriesCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "primary-repositories",
		ShortUsage: "asc xcode-cloud products primary-repositories [flags]",
		ShortHelp:  "List primary repositories for a product.",
		LongHelp: `List primary repositories for a product.

Examples:
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID"
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "Product ID",
		ParentType:       "ciProducts",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud products primary-repositories",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.Limit[asc.CiProductRepositoriesOption](limit),
				asc.NextURL[asc.CiProductRepositoriesOption](next),
			}
			return client.GetCiProductPrimaryRepositories(ctx, productID, opts...)
		},
	})
}

func newXcodeCloudProductsAdditionalRepositoriesCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "additional-repositories",
		ShortUsage: "asc xcode-cloud products additional-repositories [flags]",
		ShortHelp:  "List additional repositories for a product.",
		LongHelp: `List additional repositories for a product.

Examples:
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID"
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "Product ID",
		ParentType:       "ciProducts",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud products additional-repositories",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.Limit[asc.CiProductRepositoriesOption](limit),
				asc.NextURL[asc.CiProductRepositoriesOption](next),
			}
			return client.GetCiProductAdditionalRepositories(ctx, productID, opts...)
		},
	})
}

func newXcodeCloudProductsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc xcode-cloud products delete --id \"PRODUCT_ID\" --confirm",
		ShortHelp:  "Delete a product.",
		LongHelp: `Delete a product.

Examples:
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		IDFlag:            "id",
		IDUsage:           "Product ID",
		IDType:            "ciProducts",
		DeleteErrorPrefix: "xcode-cloud products delete",
		Deps:              d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			if err := client.DeleteCiProduct(ctx, id); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			return nil
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

func xcodeCloudProductsList(ctx context.Context, d *shared.Deps, flags ciProductsListFlags) error {
	next := strings.TrimSpace(*flags.next)

	productTypes, err := normalizeCiProductsProductTypes(flags.flagSet, *flags.productType)
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	fields, err := normalizeCiProductsSelection(flags.flagSet, *flags.fields, ciProductsFieldsValues, "--fields")
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	appFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.appFields, ciProductsAppFieldsValues, "--app-fields")
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	bundleIDFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.bundleIDFields, ciProductsBundleIDFieldsValues, "--bundle-id-fields")
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	scmRepositoryFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.scmRepositoryFields, ciProductsScmRepositoryFieldsValues, "--scm-repository-fields")
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	include, err := normalizeCiProductsSelection(flags.flagSet, *flags.include, ciProductsIncludeValues, "--include")
	if err != nil {
		return d.UsageErrorf("xcode-cloud products: %v", err)
	}
	if flagProvided(flags.flagSet, "primary-repositories-limit") && (*flags.primaryRepositoriesLimit < 1 || *flags.primaryRepositoriesLimit > 50) {
		return d.UsageError("xcode-cloud products: --primary-repositories-limit must be between 1 and 50")
	}
	if len(appFields) > 0 && !shared.HasInclude(include, "app") {
		return d.UsageError("xcode-cloud products: --app-fields requires --include app")
	}
	if len(bundleIDFields) > 0 && !shared.HasInclude(include, "bundleId") {
		return d.UsageError("xcode-cloud products: --bundle-id-fields requires --include bundleId")
	}
	if len(scmRepositoryFields) > 0 && !shared.HasInclude(include, "primaryRepositories") {
		return d.UsageError("xcode-cloud products: --scm-repository-fields requires --include primaryRepositories")
	}
	if *flags.primaryRepositoriesLimit > 0 && !shared.HasInclude(include, "primaryRepositories") {
		return d.UsageError("xcode-cloud products: --primary-repositories-limit requires --include primaryRepositories")
	}

	hydrateBundleIDs := shouldHydrateCiProductBundleIDs(*flags.output)
	// Continuation pages replay the server-supplied query verbatim, so the app
	// include can only be added when this command builds the query itself.
	if hydrateBundleIDs && next == "" {
		include = addCiProductsValue(include, "app")
		appFields = addCiProductsValue(appFields, "bundleId")
	}

	appSelector := d.ResolveAppSelector(*flags.appID)
	resolvedAppID := appSelector.Value
	buildOptions := func(appID string) []asc.CiProductsOption {
		opts := []asc.CiProductsOption{
			asc.WithCiProductsProductTypes(productTypes),
			asc.WithCiProductsFields(fields),
			asc.WithCiProductsAppFields(appFields),
			asc.WithCiProductsBundleIDFields(bundleIDFields),
			asc.WithCiProductsScmRepositoryFields(scmRepositoryFields),
			asc.WithCiProductsInclude(include),
			asc.WithCiProductsPrimaryRepositoriesLimit(*flags.primaryRepositoriesLimit),
			asc.Limit[asc.CiProductsOption](*flags.limit),
			asc.NextURL[asc.CiProductsOption](next),
		}
		if next == "" && strings.TrimSpace(appID) != "" {
			opts = append(opts, asc.WithCiProductsAppID(appID))
		}
		return opts
	}
	opts := buildOptions(resolvedAppID)

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(d, ctx, 0)
	defer cancel()

	if next == "" && resolvedAppID != "" {
		resolvedAppID, err = resolveXcodeCloudAppID(requestCtx, client, appSelector)
		if err != nil {
			return fmt.Errorf("xcode-cloud products: %w", err)
		}
		opts = buildOptions(resolvedAppID)
	}

	if *flags.paginate {
		paginateOpts := append(opts, asc.Limit[asc.CiProductsOption](200))
		paginatedResp, err := d.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, asc.NextURL[asc.CiProductsOption](nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("xcode-cloud products: %w", err)
		}

		resp, ok := paginatedResp.(*asc.CiProductsResponse)
		if !ok {
			return fmt.Errorf("xcode-cloud products: unexpected response type %T", paginatedResp)
		}
		if hydrateBundleIDs {
			if err := hydrateCiProductBundleIDs(requestCtx, client, resp, include); err != nil {
				return fmt.Errorf("xcode-cloud products: %w", err)
			}
		}

		return d.PrintOutput(resp, *flags.output, *flags.pretty)
	}

	resp, err := client.GetCiProducts(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}
	if hydrateBundleIDs {
		if err := hydrateCiProductBundleIDs(requestCtx, client, resp, include); err != nil {
			return fmt.Errorf("xcode-cloud products: %w", err)
		}
	}

	return d.PrintOutput(resp, *flags.output, *flags.pretty)
}

func shouldHydrateCiProductBundleIDs(output string) bool {
	switch shared.NormalizeOutputFormat(output) {
	case "table", "markdown", "md":
		return true
	default:
		return false
	}
}

// hydrateCiProductBundleIDs fills missing product bundle IDs from included apps
// when the list query asked for them. A paginated next URL is server-supplied
// and may omit that include, so any product still missing a bundle ID falls
// back to a per-product lookup.
func hydrateCiProductBundleIDs(ctx context.Context, client *asc.Client, resp *asc.CiProductsResponse, include []string) error {
	if resp == nil {
		return nil
	}
	if shared.HasInclude(include, "app") {
		if err := hydrateCiProductBundleIDsFromIncluded(resp); err != nil {
			return err
		}
	}
	// A later page can replay a server next URL that dropped include=app.
	// Products already filled from included are skipped; the rest still look up.
	return hydrateCiProductBundleIDsFromRelatedApps(ctx, client, resp)
}

func hydrateCiProductBundleIDsFromIncluded(resp *asc.CiProductsResponse) error {
	bundleIDs, err := includedAppBundleIDs(resp.Included)
	if err != nil {
		return err
	}

	for i := range resp.Data {
		if strings.TrimSpace(resp.Data[i].Attributes.BundleID) != "" {
			continue
		}

		appID := ciProductAppID(resp.Data[i].Relationships)
		if appID == "" {
			continue
		}
		resp.Data[i].Attributes.BundleID = bundleIDs[appID]
	}

	return nil
}

func hydrateCiProductBundleIDsFromRelatedApps(ctx context.Context, client *asc.Client, resp *asc.CiProductsResponse) error {
	if client == nil {
		return nil
	}

	for i := range resp.Data {
		if strings.TrimSpace(resp.Data[i].Attributes.BundleID) != "" {
			continue
		}

		productID := strings.TrimSpace(resp.Data[i].ID)
		if productID == "" {
			continue
		}

		appResp, err := client.GetCiProductApp(ctx, productID)
		if err != nil {
			return fmt.Errorf("resolve app for product %q: %w", productID, err)
		}

		resp.Data[i].Attributes.BundleID = strings.TrimSpace(appResp.Data.Attributes.BundleID)
	}

	return nil
}

func includedAppBundleIDs(included json.RawMessage) (map[string]string, error) {
	if len(included) == 0 {
		return nil, nil
	}

	var records []struct {
		Type       asc.ResourceType `json:"type"`
		ID         string           `json:"id"`
		Attributes struct {
			BundleID string `json:"bundleId"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(included, &records); err != nil {
		return nil, fmt.Errorf("parse included resources: %w", err)
	}

	bundleIDs := make(map[string]string, len(records))
	for _, record := range records {
		if record.Type != asc.ResourceTypeApps {
			continue
		}
		id := strings.TrimSpace(record.ID)
		if id == "" {
			continue
		}
		bundleIDs[id] = strings.TrimSpace(record.Attributes.BundleID)
	}
	return bundleIDs, nil
}

func ciProductAppID(relationships *asc.CiProductRelationships) string {
	if relationships == nil || relationships.App == nil || relationships.App.Data == nil {
		return ""
	}
	return strings.TrimSpace(relationships.App.Data.ID)
}

// newXcodeCloudMacOSVersionsCommand returns the xcode-cloud macos-versions command.
func newXcodeCloudMacOSVersionsCommand(d *shared.Deps) *ffcli.Command {
	cmd := xcodeCloudMacOSVersionsListCommand(
		d,
		"macos-versions",
		"asc xcode-cloud macos-versions [flags]",
		"Manage Xcode Cloud macOS versions.",
		`Manage Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"`,
	)
	cmd.Subcommands = []*ffcli.Command{
		newXcodeCloudMacOSVersionsListCommand(d),
		newXcodeCloudMacOSVersionsGetCommand(d),
		newXcodeCloudMacOSVersionsXcodeVersionsCommand(d),
	}
	return cmd
}

func newXcodeCloudMacOSVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return xcodeCloudMacOSVersionsListCommand(
		d,
		"list",
		"asc xcode-cloud macos-versions list [flags]",
		"List Xcode Cloud macOS versions.",
		`List Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions list --limit 50
  asc xcode-cloud macos-versions list --paginate`,
	)
}

func xcodeCloudMacOSVersionsListCommand(d *shared.Deps, name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud macos-versions",
		FetchErrorPrefix:     "xcode-cloud macos-versions",
		Deps:                 d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(ctx, asc.Limit[asc.CiMacOsVersionsOption](limit), asc.NextURL[asc.CiMacOsVersionsOption](next))
		},
	})
}

func newXcodeCloudMacOSVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud macos-versions view --id \"MACOS_VERSION_ID\"",
		ShortHelp:  "View details for a macOS version.",
		LongHelp: `View details for a macOS version.

Examples:
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "macOS version ID",
		IDType:           "ciMacOsVersions",
		FetchErrorPrefix: "xcode-cloud macos-versions view",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiMacOsVersion(ctx, id)
		},
	})
}

func newXcodeCloudMacOSVersionsXcodeVersionsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "xcode-versions",
		ShortUsage: "asc xcode-cloud macos-versions xcode-versions [flags]",
		ShortHelp:  "List Xcode versions for a macOS version.",
		LongHelp: `List Xcode versions for a macOS version.

Examples:
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --limit 50
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "macOS version ID",
		ParentType:       "ciMacOsVersions",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud macos-versions xcode-versions",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, macOSVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiXcodeVersionsOption{
				asc.Limit[asc.CiXcodeVersionsOption](limit),
				asc.NextURL[asc.CiXcodeVersionsOption](next),
			}
			return client.GetCiMacOsVersionXcodeVersions(ctx, macOSVersionID, opts...)
		},
	})
}

// newXcodeCloudXcodeVersionsCommand returns the xcode-cloud xcode-versions command.
func newXcodeCloudXcodeVersionsCommand(d *shared.Deps) *ffcli.Command {
	cmd := xcodeCloudXcodeVersionsListCommand(
		d,
		"xcode-versions",
		"asc xcode-cloud xcode-versions [flags]",
		"Manage Xcode Cloud Xcode versions.",
		`Manage Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID"`,
	)
	cmd.Subcommands = []*ffcli.Command{
		newXcodeCloudXcodeVersionsListCommand(d),
		newXcodeCloudXcodeVersionsGetCommand(d),
		newXcodeCloudXcodeVersionsMacOSVersionsCommand(d),
	}
	return cmd
}

func newXcodeCloudXcodeVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return xcodeCloudXcodeVersionsListCommand(
		d,
		"list",
		"asc xcode-cloud xcode-versions list [flags]",
		"List Xcode Cloud Xcode versions.",
		`List Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions list --limit 50
  asc xcode-cloud xcode-versions list --paginate`,
	)
}

func xcodeCloudXcodeVersionsListCommand(d *shared.Deps, name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud xcode-versions",
		FetchErrorPrefix:     "xcode-cloud xcode-versions",
		Deps:                 d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiXcodeVersions(ctx, asc.Limit[asc.CiXcodeVersionsOption](limit), asc.NextURL[asc.CiXcodeVersionsOption](next))
		},
	})
}

func newXcodeCloudXcodeVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud xcode-versions view --id \"XCODE_VERSION_ID\"",
		ShortHelp:  "View details for an Xcode version.",
		LongHelp: `View details for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Xcode version ID",
		IDType:           "ciXcodeVersions",
		FetchErrorPrefix: "xcode-cloud xcode-versions view",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiXcodeVersion(ctx, id)
		},
	})
}

func newXcodeCloudXcodeVersionsMacOSVersionsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "macos-versions",
		ShortUsage: "asc xcode-cloud xcode-versions macos-versions [flags]",
		ShortHelp:  "List macOS versions for an Xcode version.",
		LongHelp: `List macOS versions for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID" --limit 50
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID" --paginate`,
		ParentFlag:       "id",
		ParentUsage:      "Xcode version ID",
		ParentType:       "ciXcodeVersions",
		LimitMax:         200,
		FetchErrorPrefix: "xcode-cloud xcode-versions macos-versions",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, xcodeVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiMacOsVersionsOption{
				asc.Limit[asc.CiMacOsVersionsOption](limit),
				asc.NextURL[asc.CiMacOsVersionsOption](next),
			}
			return client.GetCiXcodeVersionMacOsVersions(ctx, xcodeVersionID, opts...)
		},
	})
}
