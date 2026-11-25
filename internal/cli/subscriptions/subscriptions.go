package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewSubscriptionsCommand returns the subscriptions command group built on d.
func NewSubscriptionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("subscriptions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc subscriptions <subcommand> [flags]",
		ShortHelp:  "Manage subscription groups and subscriptions.",
		LongHelp: `Manage subscription groups and subscriptions.

Submitting a subscription version for review runs through the review command
group:
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions list --group-id "GROUP_ID"
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Monthly" --product-id "com.example.sub.monthly"
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH --price "3.99" --price-territory "United States" --territories "US,Canada"
  asc subscriptions pricing summary --app "APP_ID"
  asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"
  asc subscriptions offers offer-codes generate --offer-code-id "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc subscriptions offers win-back list --subscription-id "SUB_ID"
  asc subscriptions retention-messages list --app "APP_ID"
  asc subscriptions review screenshots create --subscription-id "SUB_ID" --file "./review.png"
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsGroupsCommand(d),
			newSubscriptionsListCommand(d),
			newSubscriptionsCreateCommand(d),
			newSubscriptionsSetupCommand(d),
			newSubscriptionsGetCommand(d),
			newSubscriptionsUpdateCommand(d),
			newSubscriptionsDeleteCommand(d),
			newSubscriptionsPricingCommand(d),
			newSubscriptionsOffersCommand(d),
			newSubscriptionsReviewCommand(d),
			newSubscriptionsPromotedPurchasesCommand(d),
			newSubscriptionsVersionsCommand(d),
			newSubscriptionsGracePeriodsCommand(d),
			newSubscriptionsRetentionMessagesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsGroupsCommand returns the subscriptions groups command group.
func newSubscriptionsGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "groups",
		ShortUsage: "asc subscriptions groups <subcommand> [flags]",
		ShortHelp:  "Manage subscription groups.",
		LongHelp: `Manage subscription groups.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"
  asc subscriptions groups view --id "GROUP_ID"
  asc subscriptions groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsGroupsListCommand(d),
			newSubscriptionsGroupsCreateCommand(d),
			newSubscriptionsGroupsGetCommand(d),
			newSubscriptionsGroupsUpdateCommand(d),
			newSubscriptionsGroupsDeleteCommand(d),
			newSubscriptionsGroupsVersionsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsGroupsListCommand returns the groups list subcommand.
func newSubscriptionsGroupsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		versionsLimit                                                                                        *int
		includes, groupFields, includedVersionFields, includedSubscriptionFields, includedLocalizationFields []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "groups list",
		Name:        "list",
		ShortUsage:  "asc subscriptions groups list [flags]",
		ShortHelp:   "List subscription groups for an app.",
		LongHelp: `List subscription groups for an app.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups list --app "APP_ID" --paginate
  asc subscriptions groups list --app "APP_ID" --subscription-fields name,productId,state`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ParentType:           "apps",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"limit", "include", "fields", "version-fields", "subscription-fields", "localization-fields", "versions-limit"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include relationships: subscriptions,subscriptionGroupLocalizations,versions")
			fields := fs.String("fields", "", "Group fields: referenceName,subscriptions,subscriptionGroupLocalizations,versions")
			versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
			subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
			localizationFields := fs.String("localization-fields", "", "Included localization fields (comma-separated)")
			versionsLimit = fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
			return func() error {
				if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
					return d.UsageError("subscriptions groups list: --versions-limit must be between 1 and 50")
				}
				var err error
				includes, err = shared.NormalizeSelection(*include, []string{"subscriptions", "subscriptionGroupLocalizations", "versions"}, "--include")
				if err != nil {
					return d.UsageError("subscriptions groups list: " + err.Error())
				}
				groupFields, err = shared.NormalizeSelection(*fields, subscriptionGroupVersionGroupFields, "--fields")
				if err != nil {
					return d.UsageError("subscriptions groups list: " + err.Error())
				}
				includedVersionFields, err = shared.NormalizeSelection(*versionFields, subscriptionGroupVersionFields, "--version-fields")
				if err != nil {
					return d.UsageError("subscriptions groups list: " + err.Error())
				}
				includedSubscriptionFields, err = normalizeSelectionFlag(d, fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList())
				if err != nil {
					return err
				}
				includedLocalizationFields, err = normalizeSelectionFlag(d, fs, *localizationFields, "--localization-fields", subscriptionGroupLocalizationFieldsList())
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(ctx, appID,
				asc.Limit[asc.SubscriptionGroupsOption](limit),
				asc.NextURL[asc.SubscriptionGroupsOption](next),
				asc.WithSubscriptionGroupsInclude(includes),
				asc.WithSubscriptionGroupsFields(groupFields),
				asc.WithSubscriptionGroupsVersionFields(includedVersionFields),
				asc.WithSubscriptionGroupsSubscriptionFields(includedSubscriptionFields),
				asc.WithSubscriptionGroupsLocalizationFields(includedLocalizationFields),
				asc.WithSubscriptionGroupsVersionsLimit(*versionsLimit),
			)
		},
	})
}

// newSubscriptionsGroupsCreateCommand returns the groups create subcommand.
func newSubscriptionsGroupsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("groups create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions groups create [flags]",
		ShortHelp:  "Create a subscription group.",
		LongHelp: `Create a subscription group.

Examples:
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions groups create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupCreateAttributes{
				ReferenceName: name,
			}

			resp, err := client.CreateSubscriptionGroup(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsGroupsGetCommand returns the groups get subcommand.
func newSubscriptionsGroupsGetCommand(d *shared.Deps) *ffcli.Command {
	var opts []asc.SubscriptionGroupsOption

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "groups view",
		Name:        "view",
		ShortUsage:  "asc subscriptions groups view --id \"GROUP_ID\"",
		ShortHelp:   "View a subscription group by ID.",
		LongHelp: `View a subscription group by ID.

Examples:
  asc subscriptions groups view --id "GROUP_ID"
  asc subscriptions groups view --id "GROUP_ID" --localization-fields name,locale,state`,
		IDFlag:             "id",
		IDUsage:            "Subscription group ID",
		IDType:             "subscriptionGroups",
		RequiredDiagnostic: true,
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include relationships: subscriptions,subscriptionGroupLocalizations,versions")
			fields := fs.String("fields", "", "Group fields: referenceName,subscriptions,subscriptionGroupLocalizations,versions")
			versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
			subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
			localizationFields := fs.String("localization-fields", "", "Included localization fields (comma-separated)")
			versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
			return func() error {
				if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
					return d.UsageError("subscriptions groups view: --versions-limit must be between 1 and 50")
				}
				includes, err := shared.NormalizeSelection(*include, []string{"subscriptions", "subscriptionGroupLocalizations", "versions"}, "--include")
				if err != nil {
					return d.UsageError("subscriptions groups view: " + err.Error())
				}
				groupFields, err := shared.NormalizeSelection(*fields, subscriptionGroupVersionGroupFields, "--fields")
				if err != nil {
					return d.UsageError("subscriptions groups view: " + err.Error())
				}
				includedVersionFields, err := shared.NormalizeSelection(*versionFields, subscriptionGroupVersionFields, "--version-fields")
				if err != nil {
					return d.UsageError("subscriptions groups view: " + err.Error())
				}
				includedSubscriptionFields, err := normalizeSelectionFlag(d, fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList())
				if err != nil {
					return err
				}
				includedLocalizationFields, err := normalizeSelectionFlag(d, fs, *localizationFields, "--localization-fields", subscriptionGroupLocalizationFieldsList())
				if err != nil {
					return err
				}
				opts = []asc.SubscriptionGroupsOption{
					asc.WithSubscriptionGroupsInclude(includes),
					asc.WithSubscriptionGroupsFields(groupFields),
					asc.WithSubscriptionGroupsVersionFields(includedVersionFields),
					asc.WithSubscriptionGroupsSubscriptionFields(includedSubscriptionFields),
					asc.WithSubscriptionGroupsLocalizationFields(includedLocalizationFields),
					asc.WithSubscriptionGroupsVersionsLimit(*versionsLimit),
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionGroup(ctx, id, opts...)
		},
	})
}

// newSubscriptionsGroupsUpdateCommand returns the groups update subcommand.
func newSubscriptionsGroupsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.SubscriptionGroupUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Deps:        d,
		FlagSetName: "groups update",
		Name:        "update",
		ShortUsage:  "asc subscriptions groups update [flags]",
		ShortHelp:   "Update a subscription group.",
		LongHelp: `Update a subscription group.

Examples:
  asc subscriptions groups update --id "GROUP_ID" --reference-name "Premium"`,
		IDUsage: "Subscription group ID",
		IDType:  "subscriptionGroups",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.ReferenceName, "reference-name", "Reference name"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateSubscriptionGroup(ctx, id, attrs)
		},
	})
}

// newSubscriptionsGroupsDeleteCommand returns the groups delete subcommand.
func newSubscriptionsGroupsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "groups delete",
		Name:        "delete",
		ShortUsage:  "asc subscriptions groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:   "Delete a subscription group.",
		LongHelp: `Delete a subscription group.

Examples:
  asc subscriptions groups delete --id "GROUP_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Subscription group ID",
		IDType:             "subscriptionGroups",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionGroup(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newSubscriptionsListCommand returns the subscriptions list subcommand.
func newSubscriptionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "subscriptionGroups", "Subscription group ID")
	shared.BindFlagAlias(fs, "group", "group-id")
	shared.BindFlagAlias(fs, "subscription-group", "group-id")
	shared.BindFlagAlias(fs, "subscription-group-id", "group-id")
	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env); lists subscriptions across all groups")
	fields := fs.String("fields", "", "Sparse fields for subscriptions")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	relatedFields := bindSubscriptionRelatedFieldFlags(fs)
	include := fs.String("include", "", "Include relationships (comma-separated): "+strings.Join(subscriptionIncludeList(), ", "))
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	shared.BindFlagAlias(fs, "version-limit", "versions-limit")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions list (--group-id \"GROUP_ID\" | --app \"APP_ID\") [flags]",
		ShortHelp:  "List subscriptions in a group or app.",
		LongHelp: `List subscriptions in a group or across all groups in an app.

Examples:
  asc subscriptions list --group-id "GROUP_ID"
  asc subscriptions list --group-id "GROUP_ID" --include versions --versions-limit 10
  asc subscriptions list --group-id "GROUP_ID" --include subscriptionLocalizations,appStoreReviewScreenshot
  asc subscriptions list --group-id "GROUP_ID" --fields name --localization-fields name,locale
  asc subscriptions list --group-id "GROUP_ID" --paginate
  asc subscriptions list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			conflicts := append([]string{"app", "fields", "version-fields", "include", "versions-limit", "limit"}, subscriptionRelatedFieldFlagNames()...)
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions list", 200, conflicts...); err != nil {
				return err
			}

			id := strings.TrimSpace(*groupID)
			appFlag := strings.TrimSpace(*appID)
			nextURL := strings.TrimSpace(*next)
			resolvedAppID := ""
			if appFlag != "" || (id == "" && nextURL == "") {
				resolvedAppID = d.ResolveAppID(*appID)
			}
			if id == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --group-id or --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}
			if err := validateRelationshipLimit(d, "--versions-limit", *versionsLimit); err != nil {
				return err
			}
			if id != "" && appFlag != "" {
				return d.UsageError("--group-id and --app are mutually exclusive")
			}
			fieldValues, err := normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(d, fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			relatedFieldValues, err := subscriptionRelatedFieldValues(d, fs, relatedFields)
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(d, fs, *include, "--include", subscriptionIncludeList())
			if err != nil {
				return err
			}
			if resolvedAppID != "" && (strings.TrimSpace(*fields) != "" || strings.TrimSpace(*versionFields) != "" || strings.TrimSpace(*include) != "" || *versionsLimit != 0 || flagWasProvided(fs, subscriptionRelatedFieldFlagNames()...)) {
				return d.UsageError("--fields, --version-fields, --include, --versions-limit, and included resource field flags require --group-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if resolvedAppID != "" {
				resp, err := listSubscriptionsForApp(requestCtx, client, resolvedAppID, *limit, true)
				if err != nil {
					return fmt.Errorf("subscriptions list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.SubscriptionsOption{
				asc.Limit[asc.SubscriptionsOption](*limit),
				asc.NextURL[asc.SubscriptionsOption](nextURL),
				asc.WithSubscriptionsFields(fieldValues),
				asc.WithSubscriptionsVersionFields(versionFieldValues),
				asc.WithSubscriptionsRelatedFields(relatedFieldValues),
				asc.WithSubscriptionsInclude(includeValues),
				asc.WithSubscriptionsVersionLimit(*versionsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.SubscriptionsOption](200))
				firstPage, err := client.GetSubscriptions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptions(ctx, id, asc.NextURL[asc.SubscriptionsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func listSubscriptionsForApp(ctx context.Context, client *asc.Client, appID string, limit int, paginate bool) (*asc.SubscriptionsResponse, error) {
	pageLimit := limit
	if pageLimit == 0 {
		pageLimit = 200
	}
	groupsResp, err := client.GetSubscriptionGroups(ctx, appID, asc.Limit[asc.SubscriptionGroupsOption](pageLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}

	if paginate {
		paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(ctx, appID, asc.NextURL[asc.SubscriptionGroupsOption](nextURL))
		})
		if err != nil {
			return nil, fmt.Errorf("paginate groups: %w", err)
		}
		var ok bool
		groupsResp, ok = paginatedGroups.(*asc.SubscriptionGroupsResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected groups response type %T", paginatedGroups)
		}
	}

	result := &asc.SubscriptionsResponse{Data: []asc.Resource[asc.SubscriptionAttributes]{}}
	for _, group := range groupsResp.Data {
		subsResp, err := client.GetSubscriptions(ctx, group.ID, asc.Limit[asc.SubscriptionsOption](pageLimit))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch subscriptions for group %s: %w", group.ID, err)
		}

		if paginate {
			paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetSubscriptions(ctx, group.ID, asc.NextURL[asc.SubscriptionsOption](nextURL))
			})
			if err != nil {
				return nil, fmt.Errorf("paginate subscriptions for group %s: %w", group.ID, err)
			}
			var ok bool
			subsResp, ok = paginatedSubs.(*asc.SubscriptionsResponse)
			if !ok {
				return nil, fmt.Errorf("unexpected subscriptions response type %T", paginatedSubs)
			}
		}

		result.Data = append(result.Data, subsResp.Data...)
	}
	return result, nil
}

// newSubscriptionsCreateCommand returns the subscriptions create subcommand.
func newSubscriptionsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "subscriptionGroups", "Subscription group ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.sub)")
	reviewNote := fs.String("review-note", "", "Review note for App Review")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	var groupLevel optionalInt
	fs.Var(&groupLevel, "group-level", "Subscription ordering level (positive integer)")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions create --group-id \"GROUP_ID\" --reference-name \"NAME\" --product-id \"PRODUCT_ID\" [flags]",
		ShortHelp:  "Create a subscription.",
		LongHelp: `Create a subscription.

Examples:
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Monthly" --product-id "com.example.sub.monthly"
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Monthly" --product-id "com.example.sub.monthly" --subscription-period ONE_MONTH
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Family" --product-id "com.example.sub.family" --family-sharable
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Yearly" --product-id "com.example.sub.yearly" --group-level 1 --review-note "Same paywall structure, design may differ"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group == "" {
				return d.RequiredFlag("--group-id")
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}

			product := strings.TrimSpace(*productID)
			if product == "" {
				return d.RequiredFlag("--product-id")
			}

			note := strings.TrimSpace(*reviewNote)
			if err := validateSubscriptionReviewFlags(d, fs, note, groupLevel); err != nil {
				return err
			}

			period, err := normalizeSubscriptionPeriod(*subscriptionPeriod, false)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionCreateAttributes{
				Name:       name,
				ProductID:  product,
				ReviewNote: note,
			}
			if period != "" {
				attrs.SubscriptionPeriod = string(period)
			}
			if groupLevel.IsSet() {
				level := groupLevel.Value()
				attrs.GroupLevel = &level
			}
			if *familySharable {
				val := true
				attrs.FamilySharable = &val
			}

			resp, err := client.CreateSubscription(requestCtx, group, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsGetCommand returns the subscriptions view subcommand.
func newSubscriptionsGetCommand(d *shared.Deps) *ffcli.Command {
	var opts []asc.SubscriptionOption

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc subscriptions view --id \"SUB_ID\"",
		ShortHelp:  "View a subscription by ID.",
		LongHelp: `View a subscription by ID.

Examples:
  asc subscriptions view --id "SUB_ID"
  asc subscriptions view --id "SUB_ID" --include versions --versions-limit 10
  asc subscriptions view --id "SUB_ID" --include subscriptionLocalizations,appStoreReviewScreenshot
  asc subscriptions view --id "SUB_ID" --fields name --price-fields startDate,planType`,
		IDFlag:             "id",
		IDUsage:            "Subscription ID",
		IDType:             "subscriptions",
		RequiredDiagnostic: true,
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			shared.BindFlagAlias(fs, "subscription-id", "id")
			fields := fs.String("fields", "", "Sparse fields for subscriptions")
			versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
			relatedFields := bindSubscriptionRelatedFieldFlags(fs)
			include := fs.String("include", "", "Include relationships (comma-separated): "+strings.Join(subscriptionIncludeList(), ", "))
			versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
			shared.BindFlagAlias(fs, "version-limit", "versions-limit")
			return func() error {
				if err := validateRelationshipLimit(d, "--versions-limit", *versionsLimit); err != nil {
					return err
				}
				fieldValues, err := normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionFieldsList())
				if err != nil {
					return err
				}
				versionFieldValues, err := normalizeSelectionFlag(d, fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
				if err != nil {
					return err
				}
				relatedFieldValues, err := subscriptionRelatedFieldValues(d, fs, relatedFields)
				if err != nil {
					return err
				}
				includeValues, err := normalizeSelectionFlag(d, fs, *include, "--include", subscriptionIncludeList())
				if err != nil {
					return err
				}
				opts = []asc.SubscriptionOption{
					asc.WithSubscriptionFields(fieldValues),
					asc.WithSubscriptionIncludedVersionFields(versionFieldValues),
					asc.WithSubscriptionRelatedFields(relatedFieldValues),
					asc.WithSubscriptionInclude(includeValues),
					asc.WithSubscriptionVersionLimit(*versionsLimit),
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscription(ctx, id, opts...)
		},
	})
}

// newSubscriptionsUpdateCommand returns the subscriptions update subcommand.
func newSubscriptionsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	subID := shared.BindResourceIDFlag(fs, "id", "subscriptions", "Subscription ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	reviewNote := fs.String("review-note", "", "Review note for App Review")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	var groupLevel optionalInt
	fs.Var(&groupLevel, "group-level", "Subscription ordering level (positive integer)")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	marketSettings := fs.String("market-settings", "", "Markets (comma-separated): APP_STORE, APPLE_SCHOOL, APPLE_BUSINESS")
	multiSeatStatus := fs.String("multi-seat-status", "", "Multi-seat status: ENABLED, DISABLED (Apple default: ENABLED)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions update [flags]",
		ShortHelp:  "Update a subscription.",
		LongHelp: `Update a subscription.

Examples:
  asc subscriptions update --id "SUB_ID" --reference-name "New Name"
  asc subscriptions update --id "SUB_ID" --review-note "Same paywall structure, design may differ"
  asc subscriptions update --id "SUB_ID" --subscription-period ONE_YEAR
  asc subscriptions update --id "SUB_ID" --group-level 3
  asc subscriptions update --id "SUB_ID" --family-sharable
  asc subscriptions update --id "SUB_ID" --market-settings APP_STORE,APPLE_BUSINESS`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			name := strings.TrimSpace(*referenceName)
			note := strings.TrimSpace(*reviewNote)
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if err := validateSubscriptionReviewFlags(d, fs, note, groupLevel); err != nil {
				return err
			}
			markets, err := normalizeSelectionFlag(d, fs, strings.ToUpper(*marketSettings), "--market-settings", []string{"APP_STORE", "APPLE_SCHOOL", "APPLE_BUSINESS"})
			if err != nil {
				return err
			}
			seatStatus := strings.ToUpper(strings.TrimSpace(*multiSeatStatus))
			if visited["multi-seat-status"] {
				if seatStatus == "" {
					return d.UsageError("--multi-seat-status must not be empty")
				}
				if seatStatus != "ENABLED" && seatStatus != "DISABLED" {
					return d.UsageError("--multi-seat-status must be one of: ENABLED, DISABLED")
				}
			}
			period, err := normalizeSubscriptionPeriod(*subscriptionPeriod, false)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			if name == "" && note == "" && period == "" && !*familySharable && !groupLevel.IsSet() && len(markets) == 0 && seatStatus == "" {
				return d.UpdateFlagRequired(fs, referenceName, reviewNote, subscriptionPeriod, &groupLevel, familySharable, marketSettings, multiSeatStatus)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionUpdateAttributes{}
			if len(markets) > 0 {
				attrs.MarketSettings = &markets
			}
			if seatStatus != "" {
				attrs.MultiSeatStatus = &seatStatus
			}
			if name != "" {
				attrs.Name = &name
			}
			if note != "" {
				attrs.ReviewNote = &note
			}
			if period != "" {
				periodValue := string(period)
				attrs.SubscriptionPeriod = &periodValue
			}
			if *familySharable {
				val := true
				attrs.FamilySharable = &val
			}
			if groupLevel.IsSet() {
				level := groupLevel.Value()
				attrs.GroupLevel = &level
			}

			resp, err := client.UpdateSubscription(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func validateSubscriptionReviewFlags(d *shared.Deps, fs *flag.FlagSet, reviewNote string, groupLevel optionalInt) error {
	reviewNoteSet := false
	fs.Visit(func(f *flag.Flag) {
		reviewNoteSet = reviewNoteSet || f.Name == "review-note"
	})
	if reviewNoteSet && reviewNote == "" {
		fmt.Fprintln(d.IO.ErrOut, "Error: --review-note cannot be empty")
		return flag.ErrHelp
	}
	if groupLevel.IsSet() && groupLevel.Value() <= 0 {
		fmt.Fprintln(d.IO.ErrOut, "Error: --group-level must be a positive integer")
		return flag.ErrHelp
	}
	return nil
}

type optionalInt struct {
	set   bool
	value int
}

func (i *optionalInt) Set(value string) error {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("must be an integer")
	}
	i.value = parsed
	i.set = true
	return nil
}

func (i *optionalInt) String() string {
	if !i.set {
		return ""
	}
	return strconv.Itoa(i.value)
}

func (i optionalInt) IsSet() bool {
	return i.set
}

func (i optionalInt) Value() int {
	return i.value
}

// newSubscriptionsDeleteCommand returns the subscriptions delete subcommand.
func newSubscriptionsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc subscriptions delete --id \"SUB_ID\" --confirm",
		ShortHelp:  "Delete a subscription.",
		LongHelp: `Delete a subscription.

Examples:
  asc subscriptions delete --id "SUB_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Subscription ID",
		IDType:             "subscriptions",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscription(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newSubscriptionsPricesListCommand returns the subscriptions prices list subcommand.
func newSubscriptionsPricesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("prices list", flag.ExitOnError)

	subID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	planType := fs.String("plan-type", "", "Filter by plan type: MONTHLY or UPFRONT")
	territory := fs.String("territory", "", "Filter by territories (comma-separated; accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	pricePointIDs := fs.String("price-point-id", "", "Filter by subscription price point IDs (comma-separated)")
	fields := fs.String("fields", "", "Subscription price fields (comma-separated)")
	territoryFields := fs.String("territory-fields", "", "Included territory fields (comma-separated): currency")
	pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
	priceTierFields := fs.String("price-tier-fields", "", "Included subscription price tier fields (comma-separated): minQuantity, subscriptionPricePoint")
	additionalTiersLimit := fs.Int("additional-price-tiers-limit", 0, "Maximum included additional price tiers (1-50)")
	include := fs.String("include", "", "Relationships to include (comma-separated): territory, subscriptionPricePoint, additionalPriceTiers")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions prices list --subscription-id \"SUB_ID\" [flags]",
		ShortHelp:  "List prices for a subscription.",
		LongHelp: `List prices for a subscription.

Use --plan-type to filter by MONTHLY or UPFRONT billing plan prices.
Use --territory to filter by one or more comma-separated territories. With
--resolved, the current price is returned for each listed territory.
Use --price-point-id to filter by subscription price point IDs. Use --fields,
--price-point-fields, --price-tier-fields, and --territory-fields for sparse fields;
the corresponding relationship is included automatically when needed.

Examples:
  asc subscriptions prices list --subscription-id "SUB_ID"
  asc subscriptions prices list --subscription-id "SUB_ID" --paginate
  asc subscriptions prices list --subscription-id "SUB_ID" --resolved
  asc subscriptions prices list --subscription-id "SUB_ID" --resolved --territory USA,GBR
  asc subscriptions prices list --subscription-id "SUB_ID" --plan-type MONTHLY
  asc subscriptions prices list --subscription-id "SUB_ID" --price-point-id "PRICE_POINT_ID" --include subscriptionPricePoint --price-point-fields customerPrice`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions prices list", 200, "price-point-id", "fields", "territory-fields", "price-point-fields", "price-tier-fields", "additional-price-tiers-limit", "include"); err != nil {
				return err
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}
			if *resolved {
				for _, name := range []string{"price-point-id", "fields", "territory-fields", "price-point-fields", "price-tier-fields", "additional-price-tiers-limit", "include"} {
					if flagWasProvided(fs, name) {
						return d.UsageErrorf("--resolved cannot be combined with --%s", name)
					}
				}
			}

			id := strings.TrimSpace(*subID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--subscription-id")
			}

			var planTypeFilter asc.SubscriptionPlanType
			planTypeProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "plan-type" {
					planTypeProvided = true
				}
			})
			if planTypeProvided {
				if strings.TrimSpace(*planType) == "" {
					return d.UsageError("invalid value for --plan-type: cannot be empty")
				}
				normalized, err := normalizeSubscriptionPlanType(*planType)
				if err != nil {
					return d.UsageError(err.Error())
				}
				planTypeFilter = normalized
			}

			selectedTerritories, err := normalizeOptionalTerritoryFilter(fs, *territory)
			if err != nil {
				return d.UsageError(err.Error())
			}
			territoryFilter := strings.Join(selectedTerritories, ",")

			selectedPricePointIDs, err := normalizeOptionalCSVFilter(fs, "price-point-id", *pricePointIDs, false)
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedPriceFields, err := normalizeOptionalSelection(fs, "fields", *fields, subscriptionPriceFieldsList())
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedTerritoryFields, err := normalizeOptionalSelection(fs, "territory-fields", *territoryFields, []string{"currency"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedPricePointFields, err := normalizeOptionalSelection(fs, "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedTierFields, err := normalizeOptionalSelection(fs, "price-tier-fields", *priceTierFields, []string{"minQuantity", "subscriptionPricePoint"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			if flagWasProvided(fs, "additional-price-tiers-limit") && (*additionalTiersLimit < 1 || *additionalTiersLimit > 50) {
				return d.UsageError("--additional-price-tiers-limit must be between 1 and 50")
			}
			selectedIncludes, err := normalizeOptionalSelection(fs, "include", *include, []string{"territory", "subscriptionPricePoint", "additionalPriceTiers"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			if slices.Contains(selectedPriceFields, "territory") || len(selectedTerritoryFields) != 0 {
				selectedIncludes = appendIncludeForFields(selectedIncludes, []string{"territory"}, "territory")
			}
			if slices.Contains(selectedPriceFields, "subscriptionPricePoint") || len(selectedPricePointFields) != 0 {
				selectedIncludes = appendIncludeForFields(selectedIncludes, []string{"subscriptionPricePoint"}, "subscriptionPricePoint")
			}
			if slices.Contains(selectedPriceFields, "additionalPriceTiers") || len(selectedTierFields) > 0 || *additionalTiersLimit > 0 {
				selectedIncludes = appendIncludeForFields(selectedIncludes, []string{"additionalPriceTiers"}, "additionalPriceTiers")
			}
			if len(selectedPriceFields) > 0 {
				for _, relationship := range selectedIncludes {
					if !slices.Contains(selectedPriceFields, relationship) {
						selectedPriceFields = append(selectedPriceFields, relationship)
					}
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions prices list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
				if err != nil {
					return err
				}
			}

			if *resolved {
				resp, err := fetchResolvedSubscriptionPrices(ctx, d, client, id, *limit, *next, subscriptionPricingToday(d), planTypeFilter, territoryFilter)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: failed to resolve: %w", err)
				}
				return d.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			nextURL := strings.TrimSpace(*next)
			if nextURL != "" && (planTypeFilter != "" || territoryFilter != "") {
				nextURL, err = mergeSubscriptionPricesListFilters(nextURL, planTypeFilter, territoryFilter)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: %w", err)
				}
			}
			opts := []asc.SubscriptionPricesOption{
				asc.Limit[asc.SubscriptionPricesOption](*limit),
				asc.NextURL[asc.SubscriptionPricesOption](nextURL),
				asc.WithSubscriptionPricesPricePointIDs(selectedPricePointIDs),
				asc.WithSubscriptionPricesFields(selectedPriceFields),
				asc.WithSubscriptionPricesTerritoryFields(selectedTerritoryFields),
				asc.WithSubscriptionPricesPricePointFields(selectedPricePointFields),
				asc.WithSubscriptionPricesInclude(selectedIncludes),
				asc.WithSubscriptionPricesTierFields(selectedTierFields),
				asc.WithSubscriptionPricesAdditionalTiersLimit(*additionalTiersLimit),
			}
			if planTypeFilter != "" && nextURL == "" {
				opts = append(opts, asc.WithSubscriptionPricesPlanType(planTypeFilter))
			}
			if territoryFilter != "" && nextURL == "" {
				opts = append(opts, asc.WithSubscriptionPricesTerritory(territoryFilter))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.SubscriptionPricesOption](200))
				firstPage, err := client.GetSubscriptionPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: failed to fetch: %w", err)
				}

				additions := url.Values{}
				setSubscriptionPricesCSVQuery(additions, "fields[subscriptionPriceTiers]", selectedTierFields)
				if *additionalTiersLimit > 0 {
					additions.Set("limit[additionalPriceTiers]", strconv.Itoa(*additionalTiersLimit))
				}
				if nextURL != "" {
					parsed, _ := url.Parse(nextURL) // validated by ValidatePaginationFlags
					saved := parsed.Query()
					for _, key := range []string{
						"filter[planType]", "filter[territory]", "filter[subscriptionPricePoint]",
						"fields[subscriptionPrices]", "fields[territories]", "fields[subscriptionPricePoints]",
						"fields[subscriptionPriceTiers]", "include", "limit[additionalPriceTiers]",
					} {
						if saved.Has(key) {
							additions[key] = saved[key]
						}
					}
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					nextURL, err := mergeSubscriptionPricesListQuery(
						nextURL,
						planTypeFilter,
						territoryFilter,
						selectedPricePointIDs,
						selectedPriceFields,
						selectedTerritoryFields,
						selectedPricePointFields,
						selectedIncludes,
					)
					if err != nil {
						return nil, err
					}
					nextURL, err = mergeSubscriptionPricesNextQuery(nextURL, additions)
					if err != nil {
						return nil, err
					}
					return client.GetSubscriptionPrices(ctx, id, asc.NextURL[asc.SubscriptionPricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions prices list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions prices list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func mergeSubscriptionPricesPlanType(next string, planType asc.SubscriptionPlanType) (string, error) {
	return mergeSubscriptionPricesListFilters(next, planType, "")
}

func mergeSubscriptionPricesListFilters(next string, planType asc.SubscriptionPlanType, territory string) (string, error) {
	return mergeSubscriptionPricesListQuery(next, planType, territory, nil, nil, nil, nil, nil)
}

func mergeSubscriptionPricesListQuery(
	next string,
	planType asc.SubscriptionPlanType,
	territory string,
	pricePointIDs, priceFields, territoryFields, pricePointFields, include []string,
) (string, error) {
	if planType == "" && strings.TrimSpace(territory) == "" {
		if len(pricePointIDs) == 0 && len(priceFields) == 0 && len(territoryFields) == 0 && len(pricePointFields) == 0 && len(include) == 0 {
			return next, nil
		}
	}

	additions := url.Values{}
	if planType != "" {
		additions.Set("filter[planType]", string(planType))
	}
	if strings.TrimSpace(territory) != "" {
		additions.Set("filter[territory]", strings.ToUpper(strings.TrimSpace(territory)))
	}
	setSubscriptionPricesCSVQuery(additions, "filter[subscriptionPricePoint]", pricePointIDs)
	setSubscriptionPricesCSVQuery(additions, "fields[subscriptionPrices]", priceFields)
	setSubscriptionPricesCSVQuery(additions, "fields[territories]", territoryFields)
	setSubscriptionPricesCSVQuery(additions, "fields[subscriptionPricePoints]", pricePointFields)
	setSubscriptionPricesCSVQuery(additions, "include", include)

	return mergeSubscriptionPricesNextQuery(next, additions)
}

func setSubscriptionPricesCSVQuery(values url.Values, key string, items []string) {
	items = shared.SplitCSV(strings.Join(items, ","))
	if len(items) == 0 {
		return
	}
	values.Set(key, strings.Join(items, ","))
}

func mergeSubscriptionPricesNextQuery(next string, additions url.Values) (string, error) {
	next = strings.TrimSpace(next)
	if next == "" {
		return "", nil
	}

	parsed, err := url.Parse(next)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() || parsed.Host != "" {
		return shared.MergeNextURLQuery(next, additions)
	}

	query := parsed.Query()
	for key, values := range additions {
		query.Del(key)
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				query.Add(key, value)
			}
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type subscriptionPriceRelationshipData struct {
	SubscriptionPricePoint *asc.Relationship `json:"subscriptionPricePoint"`
	Territory              *asc.Relationship `json:"territory"`
}

func findMatchingSubscriptionPrice(ctx context.Context, client *asc.Client, subID, pricePointID, territoryID string, attrs asc.SubscriptionPriceCreateAttributes, tiers ...asc.ProposedSubscriptionPriceTier) (*asc.SubscriptionPriceResponse, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))

	opts := []asc.SubscriptionPricesOption{
		asc.Limit[asc.SubscriptionPricesOption](200),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory", "additionalPriceTiers"}),
	}
	if territoryID != "" {
		opts = append(opts, asc.WithSubscriptionPricesTerritory(territoryID))
	}
	if attrs.PlanType != "" {
		opts = append(opts, asc.WithSubscriptionPricesPlanType(attrs.PlanType))
	}

	var count priceCollectionCount
	guardedTerritories := map[string]bool{}
	seenPriceIDs := map[string]bool{}
	for {
		resp, err := client.GetSubscriptionPrices(ctx, subID, opts...)
		if err != nil {
			return nil, err
		}

		if err := checkPriceCollectionPage(&count, resp, "price"); err != nil {
			return nil, err
		}
		for _, price := range resp.Data {
			if id := strings.TrimSpace(price.ID); territoryID == "" && tiers == nil && price.Type == asc.ResourceTypeSubscriptionPrices && id != "" {
				seenPriceIDs[id] = true
			}
			if tiers == nil && subscriptionPriceRequiresTierIntent(price) {
				guardedTerritories[verifiedSubscriptionPriceTerritory(price.Relationships)] = true
			}
			if !subscriptionPriceMatchesTarget(price, pricePointID, territoryID, attrs) {
				continue
			}
			if tiers != nil {
				matches, err := subscriptionPriceTiersMatch(ctx, client, price.ID, tiers)
				if err != nil {
					return nil, err
				}
				if !matches {
					continue
				}
			}
			return &asc.SubscriptionPriceResponse{Data: price}, nil
		}

		next := strings.TrimSpace(resp.Links.Next)
		if next == "" {
			if len(guardedTerritories) > 0 {
				// Only a complete set of distinct prices can rule out tiers in the target storefront.
				if territoryID != "" || guardedTerritories[""] || len(seenPriceIDs) != count.read {
					return nil, errSubscriptionPriceTierIntentRequired
				}
				point, err := client.GetSubscriptionPricePoint(ctx, url.PathEscape(pricePointID), asc.WithSubscriptionPricePointInclude([]string{"territory"}))
				if err != nil {
					return nil, fmt.Errorf("resolve territory for price point %q: %w", pricePointID, err)
				}
				resolved := verifiedSubscriptionPriceTerritory(point.Data.Relationships)
				if point.Data.Type != asc.ResourceTypeSubscriptionPricePoints || point.Data.ID != pricePointID || resolved == "" || guardedTerritories[resolved] {
					return nil, errSubscriptionPriceTierIntentRequired
				}
			}
			return nil, nil
		}
		canonical, err := client.ValidateNoRedirectReadContinuationQuery(ctx, next, "/v1/subscriptions/"+url.PathEscape(subID)+"/prices", url.Values{
			"filter[planType]": {string(attrs.PlanType)}, "filter[territory]": {territoryID},
		})
		if err != nil {
			return nil, err
		}
		nextURL, err := mergeSubscriptionPricesListQuery(canonical, attrs.PlanType, territoryID, nil, nil, nil, nil, []string{"subscriptionPricePoint", "territory", "additionalPriceTiers"})
		if err != nil {
			return nil, err
		}
		opts = []asc.SubscriptionPricesOption{asc.NextURL[asc.SubscriptionPricesOption](nextURL)}
	}
}

func subscriptionPriceMatchesTarget(price asc.Resource[asc.SubscriptionPriceAttributes], pricePointID, territoryID string, attrs asc.SubscriptionPriceCreateAttributes) bool {
	if strings.TrimSpace(pricePointID) == "" {
		return false
	}

	var relationships subscriptionPriceRelationshipData
	if len(price.Relationships) > 0 {
		if err := json.Unmarshal(price.Relationships, &relationships); err != nil {
			return false
		}
	}
	if relationships.SubscriptionPricePoint == nil || relationships.SubscriptionPricePoint.Data.ID != pricePointID {
		return false
	}

	actualTerritory := ""
	if relationships.Territory != nil {
		actualTerritory = strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID))
	}
	if territoryID != "" && territoryID != actualTerritory {
		return false
	}

	if strings.TrimSpace(price.Attributes.StartDate) != strings.TrimSpace(attrs.StartDate) {
		return false
	}
	targetPreserved := attrs.Preserved != nil && *attrs.Preserved
	if price.Attributes.Preserved != targetPreserved {
		return false
	}
	targetPlanType := attrs.PlanType
	if targetPlanType == "" {
		targetPlanType = asc.SubscriptionPlanTypeUpfront
	}
	actualPlanType := price.Attributes.PlanType
	if actualPlanType == "" {
		actualPlanType = asc.SubscriptionPlanTypeUpfront
	}
	if actualPlanType != targetPlanType {
		return false
	}

	return true
}

// newSubscriptionsPricesAddCommand returns the subscriptions prices add subcommand.
func newSubscriptionsPricesAddCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("prices add", flag.ExitOnError)

	subID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := fs.String("app", "", subscriptionLookupAppUsage)
	pricePointID := shared.BindResourceIDFlag(fs, "price-point", "subscriptionPricePoints", "Subscription price point ID")
	tier := fs.Int("tier", 0, "Pricing tier number (mutually exclusive with --price-point and --price)")
	price := fs.String("price", "", "Customer price to select price point (mutually exclusive with --price-point and --tier)")
	territory := fs.String("territory", "", "Territory input (accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	additionalTiers := fs.String("additional-price-tiers", "", "JSON file of additional tiers [{minQuantity, subscriptionPricePointId}]; [] selects a single-tier price")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD)")
	preserved := fs.Bool("preserved", false, "Preserve existing prices")
	planType := fs.String("plan-type", "", "Billing plan: MONTHLY or UPFRONT (default Apple's UPFRONT)")
	force := fs.Bool("force", false, "Re-save the complete equalized price matrix even when the selected price is unchanged")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc subscriptions prices add [flags]",
		ShortHelp:  "Set a subscription price.",
		LongHelp: `Set a subscription price.

Examples:
  asc subscriptions prices add --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions prices add --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID" --territory "United States"
  asc subscriptions prices add --subscription-id "SUB_ID" --tier 5 --territory "US"
  asc subscriptions prices add --subscription-id "SUB_ID" --price "4.99" --territory "France"
  asc subscriptions prices add --subscription-id "SUB_ID" --price "4.99" --territory "France" --force
  asc subscriptions prices add --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID" --territory "Norway" --plan-type MONTHLY --start-date "2026-03-01"

By default, an identical existing price is returned without sending another
write. Use --force with --territory to rebuild and atomically re-save the full
equalized price matrix when repairing Apple's MISSING_METADATA state.

Use --additional-price-tiers with a JSON array of {"minQuantity": 10,
"subscriptionPricePointId": "PRICE_POINT_ID"} objects. Supply up to four tiers
in ascending quantity order, starting at 2 or more seats. Tiers require UPFRONT
prices. Use [] to select a single-tier price. Changing an existing volume price
requires this file so its removal is explicit. This flag cannot combine with --force.

--plan-type MONTHLY sets a Monthly with 12-Month Commitment price. It cannot be
combined with --force or USA, and it needs MONTHLY plan availability.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}

			pricePoint := strings.TrimSpace(*pricePointID)
			tierValue := *tier
			priceValue := strings.TrimSpace(*price)

			if err := shared.ValidatePriceSelectionFlags(pricePoint, tierValue, priceValue); err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}
			if err := shared.ValidateFinitePriceFlag("--price", priceValue); err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}

			territoryID := strings.TrimSpace(*territory)
			if territoryID != "" {
				normalizedTerritory, normalizeErr := ascterritory.Normalize(territoryID)
				if normalizeErr != nil {
					return d.UsageError(normalizeErr.Error())
				}
				territoryID = normalizedTerritory
			}
			if tierValue > 0 || priceValue != "" {
				if territoryID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --territory is required when using --tier or --price")
					return shared.MissingRequiredUsageError("--territory")
				}
			}
			if *force && territoryID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --territory is required with --force")
				return shared.MissingRequiredUsageError("--territory")
			}
			planTypeValue, err := parseSubscriptionPricePlanType(d, fs, *planType)
			if err != nil {
				return err
			}
			if err := validatePlanTypeTerritory(planTypeValue, territoryID, "--territory"); err != nil {
				return d.UsageError(err.Error())
			}
			if *force && planTypeValue == asc.SubscriptionPlanTypeMonthly {
				return d.UsageError("--force only repairs UPFRONT prices")
			}

			var tiers []asc.ProposedSubscriptionPriceTier
			if flagWasProvided(fs, "additional-price-tiers") {
				if *force {
					return d.UsageError("--additional-price-tiers cannot be combined with --force")
				}
				if strings.TrimSpace(*additionalTiers) == "" {
					return d.UsageError("--additional-price-tiers must not be empty")
				}
				input, err := readSubscriptionPriceTiersInput(d, *additionalTiers)
				if err != nil {
					return d.UsageError(err.Error())
				}
				if input == nil || *input == nil {
					return d.UsageError("--additional-price-tiers must contain an array")
				}
				tiers = *input
				if len(tiers) > 0 && planTypeValue == asc.SubscriptionPlanTypeMonthly {
					return d.UsageError("--additional-price-tiers requires UPFRONT plan type")
				}
			}

			ctx = asc.WithNoRedirectReads(ctx)
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions prices add: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}

			if tierValue > 0 || priceValue != "" {
				tierCtx, tierCancel := d.Timeout(ctx)
				tiers, err := d.ResolveSubscriptionTiers(tierCtx, client, id, territoryID, *refresh)
				tierCancel()
				if err != nil {
					return fmt.Errorf("subscriptions prices add: resolve tiers: %w", err)
				}

				if tierValue > 0 {
					pricePoint, err = shared.ResolvePricePointByTier(tiers, tierValue)
				} else {
					pricePoint, err = shared.ResolvePricePointByPrice(tiers, priceValue)
				}
				if err != nil {
					return fmt.Errorf("subscriptions prices add: %w", err)
				}
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			// Check if the subscription already has prices.
			// New subscriptions without prices require PATCH /v1/subscriptions/{id}
			// with inline price resources, while subscriptions that already have
			// prices use POST /v1/subscriptionPrices for price changes.
			hasExistingPrices, pricesErr := subscriptionHasExistingPrices(requestCtx, client, id)
			if pricesErr != nil {
				return fmt.Errorf("subscriptions prices add: failed to check existing prices: %w", pricesErr)
			}

			attrs := asc.SubscriptionPriceCreateAttributes{
				StartDate: strings.TrimSpace(*startDate),
				PlanType:  planTypeValue,
			}
			if *preserved || flagWasProvided(fs, "preserved") || (tiers != nil && len(tiers) == 0) {
				attrs.Preserved = preserved
			}

			if !hasExistingPrices {
				// Initial price: use PATCH with inline resources
				subResp, err := asc.RetrySubscriptionPriceWrite(requestCtx, func(attemptCtx context.Context, replay bool) (*asc.SubscriptionResponse, error) {
					if replay {
						exists, err := subscriptionHasExistingPrices(attemptCtx, client, id)
						if err != nil {
							return nil, err
						}
						if exists {
							match, err := findMatchingSubscriptionPrice(attemptCtx, client, id, pricePoint, territoryID, attrs, tiers...)
							if err != nil {
								return nil, err
							}
							if match == nil {
								return nil, fmt.Errorf("subscription %s already has prices; cannot replay the initial price matrix", id)
							}
							return client.GetSubscription(attemptCtx, id)
						}
					}
					return client.SetSubscriptionInitialPrice(attemptCtx, id, pricePoint, territoryID, attrs, tiers...)
				})
				if err != nil {
					return fmt.Errorf("subscriptions prices add: failed to set initial price: %w", err)
				}
				return d.PrintOutput(subResp, *output.Output, *output.Pretty)
			}

			matchingPrice, err := findMatchingSubscriptionPrice(requestCtx, client, id, pricePoint, territoryID, attrs, tiers...)
			if errors.Is(err, errSubscriptionPriceTierIntentRequired) {
				return d.UsageError(err.Error())
			}
			if err != nil {
				return fmt.Errorf("subscriptions prices add: failed to check matching price: %w", err)
			}
			if matchingPrice != nil && !*force {
				return d.PrintOutput(matchingPrice, *output.Output, *output.Pretty)
			}
			if matchingPrice != nil && *force {
				checkRepair := func(checkCtx context.Context) error {
					hasTiers, err := subscriptionHasPriceTiers(checkCtx, d, client, id, nil)
					if err != nil {
						return fmt.Errorf("subscriptions prices add: check volume tiers before repair: %w", err)
					}
					if hasTiers {
						return d.UsageError("--force cannot rebuild a subscription with volume tiers; use --additional-price-tiers without --force")
					}
					return nil
				}
				if err := checkRepair(requestCtx); err != nil {
					return err
				}
				equalizations, equalizationsErr := fetchEqualizations(requestCtx, d, client, pricePoint, territoryID)
				if equalizationsErr != nil {
					return fmt.Errorf("subscriptions prices add: build repair matrix: %w", equalizationsErr)
				}
				matrixAttrs := attrs
				matrixAttrs.PlanType = matchingPrice.Data.Attributes.PlanType
				if matrixAttrs.PlanType == "" {
					matrixAttrs.PlanType = asc.SubscriptionPlanTypeUpfront
				}
				matrix, matrixErr := buildSubscriptionSetupPriceMatrix(pricePoint, territoryID, matrixAttrs, equalizations)
				if matrixErr != nil {
					return fmt.Errorf("subscriptions prices add: build repair matrix: %w", matrixErr)
				}
				resp, matrixErr := asc.RetrySubscriptionPriceWrite(requestCtx, func(attemptCtx context.Context, replay bool) (*asc.SubscriptionResponse, error) {
					if replay {
						if err := checkRepair(attemptCtx); err != nil {
							return nil, err
						}
						match, err := findMatchingSubscriptionPrice(attemptCtx, client, id, pricePoint, territoryID, attrs)
						if err != nil {
							return nil, err
						}
						if match == nil {
							return nil, fmt.Errorf("subscription %s base price changed; cannot replay the repair matrix", id)
						}
					}
					return client.SetSubscriptionPriceMatrix(attemptCtx, id, matrix)
				})
				if matrixErr != nil {
					return fmt.Errorf("subscriptions prices add: failed to re-save price matrix: %w", matrixErr)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			// Existing prices: use POST /v1/subscriptionPrices for a price change
			resp, err := asc.RetrySubscriptionPriceWrite(requestCtx, func(attemptCtx context.Context, replay bool) (*asc.SubscriptionPriceResponse, error) {
				if replay && tiers == nil && planTypeValue != asc.SubscriptionPlanTypeMonthly {
					match, err := findMatchingSubscriptionPrice(attemptCtx, client, id, pricePoint, territoryID, attrs)
					if err != nil || match != nil {
						return match, err
					}
				}
				return client.CreateSubscriptionPrice(attemptCtx, id, pricePoint, territoryID, attrs, tiers...)
			})
			if errors.Is(err, errSubscriptionPriceTierIntentRequired) {
				return d.UsageError(err.Error())
			}
			if err != nil {
				return fmt.Errorf("subscriptions prices add: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsPricesDeleteCommand returns the subscriptions prices delete subcommand.
func newSubscriptionsPricesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "prices delete",
		Name:        "delete",
		ShortUsage:  "asc subscriptions prices delete --price-id \"PRICE_ID\" --confirm",
		ShortHelp:   "Delete a subscription price.",
		LongHelp: `Delete a subscription price.

Examples:
  asc subscriptions prices delete --price-id "PRICE_ID" --confirm`,
		IDFlag:             "price-id",
		IDUsage:            "Subscription price ID",
		IDType:             "subscriptionPrices",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionPrice(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// NewSubscriptionsAvailabilityCommand returns the subscriptions pricing availability command group.
func NewSubscriptionsAvailabilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc subscriptions pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage subscription availability (deprecated by Apple).",
		LongHelp: `Manage subscription availability.

Deprecated: the underlying Subscription availability resource is deprecated in
App Store Connect API 4.4 in favor of Subscription plan availability. These
commands keep working for now; for plan-based availability use
` + "`asc subscriptions pricing plan-availability`" + ` (show/available-territories/set) or
` + "`asc subscriptions pricing monthly-commitment`" + ` (enable/disable/list).

Examples:
  asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"
  asc subscriptions pricing availability available-territories --availability-id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsAvailabilityViewCommand(d),
			newSubscriptionsAvailabilityAvailableTerritoriesCommand(d),
			newSubscriptionsAvailabilityEditCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsAvailabilityViewCommand returns the availability view subcommand.
func newSubscriptionsAvailabilityViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availability view", flag.ExitOnError)

	availabilityID := shared.BindResourceIDFlag(fs, "availability-id", "subscriptionAvailabilities", "Subscription availability ID")
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions pricing availability view --availability-id \"AVAILABILITY_ID\"",
		ShortHelp:  "View subscription availability by ID or subscription.",
		LongHelp: `View subscription availability by ID or subscription.

Examples:
  asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"
  asc subscriptions pricing availability view --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			availabilityValue := strings.TrimSpace(*availabilityID)
			subscriptionValue := strings.TrimSpace(*subscriptionID)
			if availabilityValue == "" && subscriptionValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --availability-id or --subscription-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if availabilityValue != "" && subscriptionValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --availability-id and --subscription-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability view: %w", err)
			}

			if availabilityValue != "" {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.GetSubscriptionAvailability(requestCtx, availabilityValue)
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability view: failed to fetch: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			subscriptionValue, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, subscriptionValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAvailabilityForSubscription(requestCtx, subscriptionValue)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsAvailabilityAvailableTerritoriesCommand returns the available territories subcommand.
func newSubscriptionsAvailabilityAvailableTerritoriesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availability available-territories", flag.ExitOnError)

	availabilityID := shared.BindResourceIDFlag(fs, "availability-id", "subscriptionAvailabilities", "Subscription availability ID")
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "available-territories",
		ShortUsage: "asc subscriptions pricing availability available-territories [flags]",
		ShortHelp:  "List available territories for a subscription availability.",
		LongHelp: `List available territories by subscription availability or subscription.
Provide exactly one of --availability-id or --subscription-id for an initial request.
Use --next instead of either selector to continue from a previous response.

Examples:
  asc subscriptions pricing availability available-territories --availability-id "AVAILABILITY_ID"
  asc subscriptions pricing availability available-territories --subscription-id "SUB_ID"
  asc subscriptions pricing availability available-territories --availability-id "AVAILABILITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions pricing availability available-territories", 200, "limit", "app", "subscription-id", "availability-id"); err != nil {
				return err
			}

			availabilityValue := strings.TrimSpace(*availabilityID)
			subscriptionValue := strings.TrimSpace(*subscriptionID)
			nextURL := strings.TrimSpace(*next)
			expectedPath := ""
			if nextURL != "" {
				var err error
				nextURL, expectedPath, err = validateSubscriptionAvailabilityTerritoriesNextURL(nextURL)
				if err != nil {
					return d.UsageError(fmt.Sprintf("--next: %v", err))
				}
			}
			if availabilityValue == "" && subscriptionValue == "" && nextURL == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --availability-id or --subscription-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if availabilityValue != "" && subscriptionValue != "" {
				return d.UsageError("--availability-id and --subscription-id are mutually exclusive")
			}
			if subscriptionValue == "" && flagWasProvided(fs, "app") {
				return d.UsageError("--app requires --subscription-id")
			}
			appSelector := d.ResolveAppSelector(*appID)
			if err := d.RequireAppForStableSelector(appSelector.Value, subscriptionValue, "--subscription-id"); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability available-territories: %w", err)
			}
			requestContext := ctx
			if subscriptionValue != "" && appSelector.Value != "" {
				requestContext = asc.WithNoRedirectReads(requestContext)
			}
			if subscriptionValue != "" && nextURL == "" {
				lookupCtx, lookupCancel := d.Timeout(requestContext)
				subscriptionValue, err = shared.ResolveSubscriptionIDWithAppSelector(lookupCtx, client, appSelector, subscriptionValue)
				lookupCancel()
				if err != nil {
					return err
				}

				availabilityCtx, availabilityCancel := d.Timeout(requestContext)
				availability, fetchErr := client.GetSubscriptionAvailabilityForSubscription(availabilityCtx, subscriptionValue)
				availabilityCancel()
				if fetchErr != nil {
					return fmt.Errorf("subscriptions pricing availability available-territories: failed to resolve availability: %w", fetchErr)
				}
				if availability == nil || strings.TrimSpace(availability.Data.ID) == "" {
					return fmt.Errorf("subscriptions pricing availability available-territories: subscription availability response did not include an ID")
				}
				availabilityValue = strings.TrimSpace(availability.Data.ID)
			}
			if nextURL == "" {
				expectedPath = fmt.Sprintf("/v1/subscriptionAvailabilities/%s/availableTerritories", availabilityValue)
			}

			requestCtx, cancel := d.Timeout(requestContext)
			defer cancel()

			opts := []asc.SubscriptionAvailabilityTerritoriesOption{
				asc.Limit[asc.SubscriptionAvailabilityTerritoriesOption](*limit),
				asc.NextURL[asc.SubscriptionAvailabilityTerritoriesOption](nextURL),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.SubscriptionAvailabilityTerritoriesOption](200))
				firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability available-territories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					scopedNextURL, err := shared.ScopedPaginationURL(nextURL, expectedPath)
					if err != nil {
						return nil, err
					}
					return client.GetSubscriptionAvailabilityAvailableTerritories(ctx, availabilityValue, asc.NextURL[asc.SubscriptionAvailabilityTerritoriesOption](scopedNextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability available-territories: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityValue, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability available-territories: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func validateSubscriptionAvailabilityTerritoriesNextURL(nextURL string) (string, string, error) {
	trimmed := strings.TrimSpace(nextURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid pagination URL: %w", err)
	}

	const (
		prefix = "/v1/subscriptionAvailabilities/"
		suffix = "/availableTerritories"
	)
	if !strings.HasPrefix(parsed.Path, prefix) || !strings.HasSuffix(parsed.Path, suffix) {
		return "", "", fmt.Errorf("pagination URL must target a subscription availability territories collection")
	}
	availabilityID := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, prefix), suffix)
	if availabilityID == "" || strings.Contains(availabilityID, "/") {
		return "", "", fmt.Errorf("pagination URL must target one subscription availability")
	}

	canonical, err := shared.ScopedPaginationURL(trimmed, parsed.Path)
	if err != nil {
		return "", "", err
	}
	return canonical, parsed.Path, nil
}

// newSubscriptionsAvailabilityEditCommand returns the availability edit subcommand.
func newSubscriptionsAvailabilityEditCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("availability edit", flag.ExitOnError)

	subID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	territories := fs.String("territories", "", "Territory IDs, comma-separated")
	lastBool := &lastVisitedBoolFlag{}
	availableInNewFlag := bindVisitedBoolFlag(fs, lastBool, "available-in-new-territories", "Include new territories automatically")
	availableInNew := &availableInNewFlag.value
	billingMode := fs.String("billing-mode", string(subscriptionBillingModeUpfront), "Billing mode: upfront or monthly-commitment")
	confirmFlag := bindVisitedBoolFlag(fs, lastBool, "confirm", "Confirm monthly-commitment availability changes")
	confirm := &confirmFlag.value
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc subscriptions pricing availability edit [flags]",
		ShortHelp:  "Edit subscription availability in territories.",
		LongHelp: `Edit subscription availability in territories.

Examples:
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --billing-mode monthly-commitment --territories "Norway,Germany" --confirm

Confirmation is required when --billing-mode monthly-commitment is selected.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			var trailingBool *bool
			switch lastBool.name {
			case "available-in-new-territories":
				trailingBool = availableInNew
			case "confirm":
				trailingBool = confirm
			}
			if err := d.RecoverBoolFlagTailArgs(fs, args, trailingBool); err != nil {
				return err
			}

			id := strings.TrimSpace(*subID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return d.RequiredFlag("--territories")
			}
			normalizedBillingMode, err := normalizeSubscriptionBillingMode(*billingMode)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if normalizedBillingMode == subscriptionBillingModeMonthlyCommitment {
				if *availableInNew {
					return d.UsageError("--available-in-new-territories is not supported for MONTHLY plan availability")
				}
				territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
				if len(territoryIDs) == 0 {
					return d.UsageError("no eligible monthly-commitment territories remain after excluding USA")
				}
				if !*confirm {
					return d.UsageError("--confirm is required for monthly-commitment availability changes")
				}
				printMonthlyCommitmentTerritoryWarning(d, excluded)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability edit: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}

			if normalizedBillingMode == subscriptionBillingModeMonthlyCommitment {
				listCtx, listCancel := d.Timeout(ctx)
				existing, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(listCtx, id)
				listCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability edit: failed to fetch monthly-commitment plan availability: %w", err)
				}

				var resp *asc.SubscriptionPlanAvailabilityResponse
				if monthlyPlan, ok := findMonthlySubscriptionPlanAvailability(existing); ok {
					updateCtx, updateCancel := d.Timeout(ctx)
					resp, err = client.UpdateSubscriptionPlanAvailability(updateCtx, monthlyPlan.ID, territoryIDs, nil)
					updateCancel()
				} else {
					createCtx, createCancel := d.Timeout(ctx)
					resp, err = client.CreateSubscriptionPlanAvailability(createCtx, id, territoryIDs, asc.SubscriptionPlanAvailabilityAttributes{
						PlanType: asc.SubscriptionPlanTypeMonthly,
					})
					createCancel()
				}
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability edit: failed to set monthly-commitment plan availability: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionAvailabilityAttributes{
				AvailableInNewTerritories: *availableInNew,
			}

			resp, err := client.CreateSubscriptionAvailability(requestCtx, id, territoryIDs, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability edit: failed to set: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
