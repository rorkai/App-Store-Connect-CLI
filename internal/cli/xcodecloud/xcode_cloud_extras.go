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

// XcodeCloudProductsCommand returns the xcode-cloud products command with subcommands.
func XcodeCloudProductsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("products", flag.ExitOnError)

	flags := bindCiProductsListFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudProductsListCommand(),
			XcodeCloudProductsGetCommand(),
			XcodeCloudProductsAppCommand(),
			XcodeCloudProductsBuildRunsCommand(),
			XcodeCloudProductsWorkflowsCommand(),
			XcodeCloudProductsPrimaryRepositoriesCommand(),
			XcodeCloudProductsAdditionalRepositoriesCommand(),
			XcodeCloudProductsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudProductsList(ctx, flags)
		},
	}
}

func XcodeCloudProductsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	flags := bindCiProductsListFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudProductsList(ctx, flags)
		},
	}
}

func XcodeCloudProductsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc xcode-cloud products view --id \"PRODUCT_ID\"",
		ShortHelp:   "View details for a product.",
		LongHelp: `View details for a product.

Examples:
  asc xcode-cloud products view --id "PRODUCT_ID"
  asc xcode-cloud products view --id "PRODUCT_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Product ID",
		ErrorPrefix: "xcode-cloud products view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiProduct(ctx, id)
		},
	})
}

func XcodeCloudProductsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)
	id := fs.String("id", "", "Product ID")
	appInfoFields := fs.String("app-info-fields", "", "Sparse fields for included app info records: kidsAgeBand (deprecated; removed from API 4.5; prefer age-rating data)")
	iapFields := fs.String("iap-fields", "", "Sparse fields for included in-app purchases: versions")
	subscriptionGroupFields := fs.String("subscription-group-fields", "", "Sparse fields for included subscription groups: versions")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc xcode-cloud products app --id \"PRODUCT_ID\"",
		ShortHelp:  "Get the app for a product.",
		LongHelp: `Get the app for a product.

Examples:
  asc xcode-cloud products app --id "PRODUCT_ID"
  asc xcode-cloud products app --id "PRODUCT_ID" --app-info-fields kidsAgeBand --iap-fields versions
  asc xcode-cloud products app --id "PRODUCT_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return shared.UsageError("--id is required")
			}
			appInfoFieldValues, err := normalizeCiProductAppSparseField(fs, *appInfoFields, ciProductAppInfoSparseFields441, "--app-info-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			iapFieldValues, err := normalizeCiProductAppSparseField(fs, *iapFields, ciProductAppInAppPurchaseSparseFields441, "--iap-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			groupFieldValues, err := normalizeCiProductAppSparseField(fs, *subscriptionGroupFields, ciProductAppSubscriptionGroupSparseFields441, "--subscription-group-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			shared.WarnDeprecatedAppInfoFields(appInfoFieldValues, "")

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: %w", err)
			}
			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
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
			resp, err := client.GetCiProductApp(
				requestCtx, idValue,
				asc.WithCiProductAppAppInfoFields(appInfoFieldValues),
				asc.WithCiProductAppInAppPurchaseFields(iapFieldValues),
				asc.WithCiProductAppSubscriptionGroupFields(groupFieldValues),
				asc.WithCiProductAppInclude(includeValues),
			)
			if err != nil {
				return fmt.Errorf("xcode-cloud products app: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudProductsBuildRunsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "build-runs",
		Name:        "build-runs",
		ShortUsage:  "asc xcode-cloud products build-runs [flags]",
		ShortHelp:   "List build runs for a product.",
		LongHelp: `List build runs for a product.

Examples:
  asc xcode-cloud products build-runs --id "PRODUCT_ID"
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products build-runs --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products build-runs",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiBuildRunsOption{
				asc.WithCiBuildRunsLimit(limit),
				asc.WithCiBuildRunsNextURL(next),
			}
			return client.GetCiProductBuildRuns(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsWorkflowsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "workflows",
		Name:        "workflows",
		ShortUsage:  "asc xcode-cloud products workflows [flags]",
		ShortHelp:   "List workflows for a product.",
		LongHelp: `List workflows for a product.

Examples:
  asc xcode-cloud products workflows --id "PRODUCT_ID"
  asc xcode-cloud products workflows --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products workflows --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products workflows",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiWorkflowsOption{
				asc.WithCiWorkflowsLimit(limit),
				asc.WithCiWorkflowsNextURL(next),
			}
			return client.GetCiWorkflows(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsPrimaryRepositoriesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "primary-repositories",
		Name:        "primary-repositories",
		ShortUsage:  "asc xcode-cloud products primary-repositories [flags]",
		ShortHelp:   "List primary repositories for a product.",
		LongHelp: `List primary repositories for a product.

Examples:
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID"
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products primary-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products primary-repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(limit),
				asc.WithCiProductRepositoriesNextURL(next),
			}
			return client.GetCiProductPrimaryRepositories(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsAdditionalRepositoriesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "additional-repositories",
		Name:        "additional-repositories",
		ShortUsage:  "asc xcode-cloud products additional-repositories [flags]",
		ShortHelp:   "List additional repositories for a product.",
		LongHelp: `List additional repositories for a product.

Examples:
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID"
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --limit 50
  asc xcode-cloud products additional-repositories --id "PRODUCT_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Product ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud products additional-repositories",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiProductRepositoriesOption{
				asc.WithCiProductRepositoriesLimit(limit),
				asc.WithCiProductRepositoriesNextURL(next),
			}
			return client.GetCiProductAdditionalRepositories(ctx, productID, opts...)
		},
	})
}

func XcodeCloudProductsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc xcode-cloud products delete --id \"PRODUCT_ID\" --confirm",
		ShortHelp:   "Delete a product.",
		LongHelp: `Delete a product.

Examples:
  asc xcode-cloud products delete --id "PRODUCT_ID" --confirm`,
		IDFlag:      "id",
		IDUsage:     "Product ID",
		ErrorPrefix: "xcode-cloud products delete",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			if err := client.DeleteCiProduct(ctx, id); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			return nil
		},
		Result: func(id string) any {
			return &asc.CiProductDeleteResult{ID: id, Deleted: true}
		},
	})
}

func xcodeCloudProductsList(ctx context.Context, flags ciProductsListFlags) error {
	if *flags.limit != 0 && (*flags.limit < 1 || *flags.limit > 200) {
		return shared.UsageError("xcode-cloud products: --limit must be between 1 and 200")
	}

	next := strings.TrimSpace(*flags.next)
	if err := shared.ValidateNextURL(next); err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	if err := shared.RejectNextFlagConflicts(
		flags.flagSet,
		next,
		"xcode-cloud products",
		"product-type",
		"fields",
		"app-fields",
		"bundle-id-fields",
		"scm-repository-fields",
		"include",
		"primary-repositories-limit",
		"limit",
	); err != nil {
		return err
	}

	productTypes, err := normalizeCiProductsProductTypes(flags.flagSet, *flags.productType)
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	fields, err := normalizeCiProductsSelection(flags.flagSet, *flags.fields, ciProductsFieldsValues, "--fields")
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	appFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.appFields, ciProductsAppFieldsValues, "--app-fields")
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	bundleIDFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.bundleIDFields, ciProductsBundleIDFieldsValues, "--bundle-id-fields")
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	scmRepositoryFields, err := normalizeCiProductsSelection(flags.flagSet, *flags.scmRepositoryFields, ciProductsScmRepositoryFieldsValues, "--scm-repository-fields")
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	include, err := normalizeCiProductsSelection(flags.flagSet, *flags.include, ciProductsIncludeValues, "--include")
	if err != nil {
		return shared.UsageErrorf("xcode-cloud products: %v", err)
	}
	if ciProductsFlagProvided(flags.flagSet, "primary-repositories-limit") && (*flags.primaryRepositoriesLimit < 1 || *flags.primaryRepositoriesLimit > 50) {
		return shared.UsageError("xcode-cloud products: --primary-repositories-limit must be between 1 and 50")
	}
	if len(appFields) > 0 && !shared.HasInclude(include, "app") {
		return shared.UsageError("xcode-cloud products: --app-fields requires --include app")
	}
	if len(bundleIDFields) > 0 && !shared.HasInclude(include, "bundleId") {
		return shared.UsageError("xcode-cloud products: --bundle-id-fields requires --include bundleId")
	}
	if len(scmRepositoryFields) > 0 && !shared.HasInclude(include, "primaryRepositories") {
		return shared.UsageError("xcode-cloud products: --scm-repository-fields requires --include primaryRepositories")
	}
	if *flags.primaryRepositoriesLimit > 0 && !shared.HasInclude(include, "primaryRepositories") {
		return shared.UsageError("xcode-cloud products: --primary-repositories-limit requires --include primaryRepositories")
	}

	hydrateBundleIDs := shouldHydrateCiProductBundleIDs(*flags.output)
	// Continuation pages replay the server-supplied query verbatim, so the app
	// include can only be added when this command builds the query itself.
	if hydrateBundleIDs && next == "" {
		include = addCiProductsValue(include, "app")
		appFields = addCiProductsValue(appFields, "bundleId")
	}

	resolvedAppID := shared.ResolveAppID(*flags.appID)
	buildOptions := func(appID string) []asc.CiProductsOption {
		opts := []asc.CiProductsOption{
			asc.WithCiProductsProductTypes(productTypes),
			asc.WithCiProductsFields(fields),
			asc.WithCiProductsAppFields(appFields),
			asc.WithCiProductsBundleIDFields(bundleIDFields),
			asc.WithCiProductsScmRepositoryFields(scmRepositoryFields),
			asc.WithCiProductsInclude(include),
			asc.WithCiProductsPrimaryRepositoriesLimit(*flags.primaryRepositoriesLimit),
			asc.WithCiProductsLimit(*flags.limit),
			asc.WithCiProductsNextURL(next),
		}
		if next == "" && strings.TrimSpace(appID) != "" {
			opts = append(opts, asc.WithCiProductsAppID(appID))
		}
		return opts
	}
	opts := buildOptions(resolvedAppID)

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud products: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	if next == "" && resolvedAppID != "" {
		resolvedAppID, err = resolveXcodeCloudAppID(requestCtx, client, resolvedAppID)
		if err != nil {
			return fmt.Errorf("xcode-cloud products: %w", err)
		}
		opts = buildOptions(resolvedAppID)
	}

	if *flags.paginate {
		paginateOpts := append(opts, asc.WithCiProductsLimit(200))
		paginatedResp, err := shared.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiProducts(ctx, asc.WithCiProductsNextURL(nextURL))
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

		return shared.PrintOutput(resp, *flags.output, *flags.pretty)
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

	return shared.PrintOutput(resp, *flags.output, *flags.pretty)
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

func xcodeCloudVersionListFlags(fs *flag.FlagSet) (limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudMacOSVersionsCommand returns the xcode-cloud macos-versions command.
func XcodeCloudMacOSVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("macos-versions", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "macos-versions",
		ShortUsage: "asc xcode-cloud macos-versions [flags]",
		ShortHelp:  "Manage Xcode Cloud macOS versions.",
		LongHelp: `Manage Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudMacOSVersionsListCommand(),
			XcodeCloudMacOSVersionsGetCommand(),
			XcodeCloudMacOSVersionsXcodeVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudMacOSVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudMacOSVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud macos-versions list [flags]",
		ShortHelp:  "List Xcode Cloud macOS versions.",
		LongHelp: `List Xcode Cloud macOS versions.

Examples:
  asc xcode-cloud macos-versions list
  asc xcode-cloud macos-versions list --limit 50
  asc xcode-cloud macos-versions list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudMacOSVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudMacOSVersionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc xcode-cloud macos-versions view --id \"MACOS_VERSION_ID\"",
		ShortHelp:   "View details for a macOS version.",
		LongHelp: `View details for a macOS version.

Examples:
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions view --id "MACOS_VERSION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "macOS version ID",
		ErrorPrefix: "xcode-cloud macos-versions view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiMacOsVersion(ctx, id)
		},
	})
}

func XcodeCloudMacOSVersionsXcodeVersionsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "xcode-versions",
		Name:        "xcode-versions",
		ShortUsage:  "asc xcode-cloud macos-versions xcode-versions [flags]",
		ShortHelp:   "List Xcode versions for a macOS version.",
		LongHelp: `List Xcode versions for a macOS version.

Examples:
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID"
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --limit 50
  asc xcode-cloud macos-versions xcode-versions --id "MACOS_VERSION_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "macOS version ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud macos-versions xcode-versions",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, macOSVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiXcodeVersionsOption{
				asc.WithCiXcodeVersionsLimit(limit),
				asc.WithCiXcodeVersionsNextURL(next),
			}
			return client.GetCiMacOsVersionXcodeVersions(ctx, macOSVersionID, opts...)
		},
	})
}

func xcodeCloudMacOSVersionsList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud macos-versions",
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(
				ctx,
				asc.WithCiMacOsVersionsLimit(limit),
				asc.WithCiMacOsVersionsNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return client.GetCiMacOsVersions(ctx, asc.WithCiMacOsVersionsNextURL(next))
		},
	)
}

// XcodeCloudXcodeVersionsCommand returns the xcode-cloud xcode-versions command.
func XcodeCloudXcodeVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode-versions", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "xcode-versions",
		ShortUsage: "asc xcode-cloud xcode-versions [flags]",
		ShortHelp:  "Manage Xcode Cloud Xcode versions.",
		LongHelp: `Manage Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudXcodeVersionsListCommand(),
			XcodeCloudXcodeVersionsGetCommand(),
			XcodeCloudXcodeVersionsMacOSVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudXcodeVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudXcodeVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit, next, paginate, output, pretty := xcodeCloudVersionListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud xcode-versions list [flags]",
		ShortHelp:  "List Xcode Cloud Xcode versions.",
		LongHelp: `List Xcode Cloud Xcode versions.

Examples:
  asc xcode-cloud xcode-versions list
  asc xcode-cloud xcode-versions list --limit 50
  asc xcode-cloud xcode-versions list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudXcodeVersionsList(ctx, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudXcodeVersionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc xcode-cloud xcode-versions view --id \"XCODE_VERSION_ID\"",
		ShortHelp:   "View details for an Xcode version.",
		LongHelp: `View details for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions view --id "XCODE_VERSION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Xcode version ID",
		ErrorPrefix: "xcode-cloud xcode-versions view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiXcodeVersion(ctx, id)
		},
	})
}

func XcodeCloudXcodeVersionsMacOSVersionsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "macos-versions",
		Name:        "macos-versions",
		ShortUsage:  "asc xcode-cloud xcode-versions macos-versions [flags]",
		ShortHelp:   "List macOS versions for an Xcode version.",
		LongHelp: `List macOS versions for an Xcode version.

Examples:
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID"
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID" --limit 50
  asc xcode-cloud xcode-versions macos-versions --id "XCODE_VERSION_ID" --paginate`,
		ParentFlag:  "id",
		ParentUsage: "Xcode version ID",
		LimitMax:    200,
		ErrorPrefix: "xcode-cloud xcode-versions macos-versions",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, xcodeVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CiMacOsVersionsOption{
				asc.WithCiMacOsVersionsLimit(limit),
				asc.WithCiMacOsVersionsNextURL(next),
			}
			return client.GetCiXcodeVersionMacOsVersions(ctx, xcodeVersionID, opts...)
		},
	})
}

func xcodeCloudXcodeVersionsList(ctx context.Context, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud xcode-versions",
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiXcodeVersions(
				ctx,
				asc.WithCiXcodeVersionsLimit(limit),
				asc.WithCiXcodeVersionsNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return client.GetCiXcodeVersions(ctx, asc.WithCiXcodeVersionsNextURL(next))
		},
	)
}
