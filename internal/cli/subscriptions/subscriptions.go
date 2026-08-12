package subscriptions

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var subscriptionQueryClientFactory = shared.GetASCClient

// SubscriptionsCommand returns the subscriptions command group.
func SubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc subscriptions <subcommand> [flags]",
		ShortHelp:  "Manage subscription groups and subscriptions.",
		LongHelp: `Manage subscription groups and subscriptions.

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
  asc subscriptions review screenshots create --subscription-id "SUB_ID" --file "./review.png"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsCommand(),
			SubscriptionsListCommand(),
			SubscriptionsCreateCommand(),
			SubscriptionsSetupCommand(),
			SubscriptionsGetCommand(),
			SubscriptionsUpdateCommand(),
			SubscriptionsDeleteCommand(),
			SubscriptionsPricingCommand(),
			SubscriptionsOffersCommand(),
			SubscriptionsReviewCommand(),
			SubscriptionsPromotedPurchasesCommand(),
			SubscriptionsLocalizationsCommand(),
			SubscriptionsImagesCommand(),
			SubscriptionsVersionsCommand(),
			SubscriptionsGracePeriodsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGroupsCommand returns the subscriptions groups command group.
func SubscriptionsGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsListCommand(),
			SubscriptionsGroupsCreateCommand(),
			SubscriptionsGroupsGetCommand(),
			SubscriptionsGroupsUpdateCommand(),
			SubscriptionsGroupsDeleteCommand(),
			SubscriptionsGroupsLocalizationsCommand(),
			SubscriptionsGroupsVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGroupsListCommand returns the groups list subcommand.
func SubscriptionsGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	include := fs.String("include", "", "Include relationships: subscriptions,subscriptionGroupLocalizations,versions")
	fields := fs.String("fields", "", "Group fields: referenceName,subscriptions,subscriptionGroupLocalizations,versions")
	versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions groups list [flags]",
		ShortHelp:  "List subscription groups for an app.",
		LongHelp: `List subscription groups for an app.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions groups list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions groups list: %w", err)
			}
			if strings.TrimSpace(*next) != "" && subscriptionGroupAnyFlagSet(fs, "app") {
				return shared.UsageError("subscriptions groups list: --next cannot be combined with --app")
			}
			if strings.TrimSpace(*next) != "" && subscriptionGroupAnyFlagSet(fs, "limit", "include", "fields", "version-fields", "versions-limit") {
				return shared.UsageError("subscriptions groups list: --next cannot be combined with query flags")
			}
			if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
				return shared.UsageError("subscriptions groups list: --versions-limit must be between 1 and 50")
			}
			includes, err := shared.NormalizeSelection(*include, []string{"subscriptions", "subscriptionGroupLocalizations", "versions"}, "--include")
			if err != nil {
				return shared.UsageError("subscriptions groups list: " + err.Error())
			}
			groupFields, err := shared.NormalizeSelection(*fields, subscriptionGroupVersionGroupFields, "--fields")
			if err != nil {
				return shared.UsageError("subscriptions groups list: " + err.Error())
			}
			includedVersionFields, err := shared.NormalizeSelection(*versionFields, subscriptionGroupVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("subscriptions groups list: " + err.Error())
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := subscriptionGroupVersionClientFactory()
			if err != nil {
				return fmt.Errorf("subscriptions groups list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionGroupsOption{
				asc.WithSubscriptionGroupsLimit(*limit),
				asc.WithSubscriptionGroupsNextURL(*next),
				asc.WithSubscriptionGroupsInclude(includes),
				asc.WithSubscriptionGroupsFields(groupFields),
				asc.WithSubscriptionGroupsVersionFields(includedVersionFields),
				asc.WithSubscriptionGroupsVersionsLimit(*versionsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionGroupsLimit(200))
				firstPage, err := client.GetSubscriptionGroups(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions groups list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionGroups(ctx, resolvedAppID, asc.WithSubscriptionGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionGroups(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsCreateCommand returns the groups create subcommand.
func SubscriptionsGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions groups create [flags]",
		ShortHelp:  "Create a subscription group.",
		LongHelp: `Create a subscription group.

Examples:
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupCreateAttributes{
				ReferenceName: name,
			}

			resp, err := client.CreateSubscriptionGroup(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsGetCommand returns the groups get subcommand.
func SubscriptionsGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups view", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	include := fs.String("include", "", "Include relationships: subscriptions,subscriptionGroupLocalizations,versions")
	fields := fs.String("fields", "", "Group fields: referenceName,subscriptions,subscriptionGroupLocalizations,versions")
	versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions groups view --id \"GROUP_ID\"",
		ShortHelp:  "View a subscription group by ID.",
		LongHelp: `View a subscription group by ID.

Examples:
  asc subscriptions groups view --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if *versionsLimit != 0 && (*versionsLimit < 1 || *versionsLimit > 50) {
				return shared.UsageError("subscriptions groups view: --versions-limit must be between 1 and 50")
			}
			includes, err := shared.NormalizeSelection(*include, []string{"subscriptions", "subscriptionGroupLocalizations", "versions"}, "--include")
			if err != nil {
				return shared.UsageError("subscriptions groups view: " + err.Error())
			}
			groupFields, err := shared.NormalizeSelection(*fields, subscriptionGroupVersionGroupFields, "--fields")
			if err != nil {
				return shared.UsageError("subscriptions groups view: " + err.Error())
			}
			includedVersionFields, err := shared.NormalizeSelection(*versionFields, subscriptionGroupVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("subscriptions groups view: " + err.Error())
			}

			client, err := subscriptionGroupVersionClientFactory()
			if err != nil {
				return fmt.Errorf("subscriptions groups view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionGroup(
				requestCtx, id,
				asc.WithSubscriptionGroupsInclude(includes),
				asc.WithSubscriptionGroupsFields(groupFields),
				asc.WithSubscriptionGroupsVersionFields(includedVersionFields),
				asc.WithSubscriptionGroupsVersionsLimit(*versionsLimit),
			)
			if err != nil {
				return fmt.Errorf("subscriptions groups view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsUpdateCommand returns the groups update subcommand.
func SubscriptionsGroupsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups update", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions groups update [flags]",
		ShortHelp:  "Update a subscription group.",
		LongHelp: `Update a subscription group.

Examples:
  asc subscriptions groups update --id "GROUP_ID" --reference-name "Premium"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupUpdateAttributes{
				ReferenceName: &name,
			}

			resp, err := client.UpdateSubscriptionGroup(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsDeleteCommand returns the groups delete subcommand.
func SubscriptionsGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups delete", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a subscription group.",
		LongHelp: `Delete a subscription group.

Examples:
  asc subscriptions groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionGroup(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions groups delete: failed to delete: %w", err)
			}

			result := &asc.SubscriptionGroupDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsListCommand returns the subscriptions list subcommand.
func SubscriptionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Subscription group ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env); lists subscriptions across all groups")
	fields := fs.String("fields", "", "Sparse fields for subscriptions")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	include := fs.String("include", "", "Include relationships (supports versions)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	legacyVersionLimit := shared.BindDeprecatedIntFlagAlias(fs, "version-limit", "versions-limit")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions list (--group-id \"GROUP_ID\" | --app \"APP_ID\") [flags]",
		ShortHelp:  "List subscriptions in a group or app.",
		LongHelp: `List subscriptions in a group or across all groups in an app.

Examples:
  asc subscriptions list --group-id "GROUP_ID"
  asc subscriptions list --group-id "GROUP_ID" --include versions --versions-limit 10
  asc subscriptions list --group-id "GROUP_ID" --paginate
  asc subscriptions list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyVersionLimit.Apply(versionsLimit); err != nil {
				return err
			}
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions list: %w", err)
			}
			if err := validateRelationshipLimit("--versions-limit", *versionsLimit); err != nil {
				return err
			}

			id := strings.TrimSpace(*groupID)
			appFlag := strings.TrimSpace(*appID)
			nextURL := strings.TrimSpace(*next)
			if id != "" && appFlag != "" {
				return shared.UsageError("--group-id and --app are mutually exclusive")
			}
			if appFlag != "" && nextURL != "" {
				return shared.UsageError("--next cannot be combined with --app; use --group-id with the group-scoped next URL")
			}
			if err := validateNextFlagConflicts(
				nextURL,
				flagConflict{"--app", flagWasProvided(fs, "app")},
				flagConflict{"--group-id", flagWasProvided(fs, "group-id")},
				flagConflict{"--fields", flagWasProvided(fs, "fields")},
				flagConflict{"--version-fields", flagWasProvided(fs, "version-fields")},
				flagConflict{"--include", flagWasProvided(fs, "include")},
				flagConflict{"--versions-limit", flagWasProvided(fs, "versions-limit") || legacyVersionLimit.WasProvided()},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(fs, *include, "--include", subscriptionIncludeList())
			if err != nil {
				return err
			}
			resolvedAppID := ""
			if appFlag != "" || (id == "" && nextURL == "") {
				resolvedAppID = shared.ResolveAppID(*appID)
			}
			if resolvedAppID != "" && (strings.TrimSpace(*fields) != "" || strings.TrimSpace(*versionFields) != "" || strings.TrimSpace(*include) != "" || *versionsLimit != 0) {
				return shared.UsageError("--fields, --version-fields, --include, and --versions-limit require --group-id")
			}
			if id == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id or --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := subscriptionQueryClientFactory()
			if err != nil {
				return fmt.Errorf("subscriptions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if resolvedAppID != "" {
				resp, err := listSubscriptionsForApp(requestCtx, client, resolvedAppID, *limit, true)
				if err != nil {
					return fmt.Errorf("subscriptions list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.SubscriptionsOption{
				asc.WithSubscriptionsLimit(*limit),
				asc.WithSubscriptionsNextURL(nextURL),
				asc.WithSubscriptionsFields(fieldValues),
				asc.WithSubscriptionsVersionFields(versionFieldValues),
				asc.WithSubscriptionsInclude(includeValues),
				asc.WithSubscriptionsVersionLimit(*versionsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionsLimit(200))
				firstPage, err := client.GetSubscriptions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptions(ctx, id, asc.WithSubscriptionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func listSubscriptionsForApp(ctx context.Context, client *asc.Client, appID string, limit int, paginate bool) (*asc.SubscriptionsResponse, error) {
	pageLimit := limit
	if pageLimit == 0 {
		pageLimit = 200
	}
	groupsResp, err := client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsLimit(pageLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}

	if paginate {
		paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
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
		subsResp, err := client.GetSubscriptions(ctx, group.ID, asc.WithSubscriptionsLimit(pageLimit))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch subscriptions for group %s: %w", group.ID, err)
		}

		if paginate {
			paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetSubscriptions(ctx, group.ID, asc.WithSubscriptionsNextURL(nextURL))
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

// SubscriptionsCreateCommand returns the subscriptions create subcommand.
func SubscriptionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Subscription group ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.sub)")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions create --group-id \"GROUP_ID\" --reference-name \"NAME\" --product-id \"PRODUCT_ID\" [flags]",
		ShortHelp:  "Create a subscription.",
		LongHelp: `Create a subscription.

Examples:
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Monthly" --product-id "com.example.sub.monthly"
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Monthly" --product-id "com.example.sub.monthly" --subscription-period ONE_MONTH
  asc subscriptions create --group-id "GROUP_ID" --reference-name "Family" --product-id "com.example.sub.family" --family-sharable`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return shared.MissingRequiredUsageError()
			}

			product := strings.TrimSpace(*productID)
			if product == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return shared.MissingRequiredUsageError()
			}

			period, err := normalizeSubscriptionPeriod(*subscriptionPeriod, false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionCreateAttributes{
				Name:      name,
				ProductID: product,
			}
			if period != "" {
				attrs.SubscriptionPeriod = string(period)
			}
			if *familySharable {
				val := true
				attrs.FamilySharable = &val
			}

			resp, err := client.CreateSubscription(requestCtx, group, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGetCommand returns the subscriptions view subcommand.
func SubscriptionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	legacySubscriptionID := shared.BindDeprecatedStringFlagAlias(fs, "subscription-id", "id")
	fields := fs.String("fields", "", "Sparse fields for subscriptions")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	include := fs.String("include", "", "Include relationships (supports versions)")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	legacyVersionLimit := shared.BindDeprecatedIntFlagAlias(fs, "version-limit", "versions-limit")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions view --id \"SUB_ID\"",
		ShortHelp:  "View a subscription by ID.",
		LongHelp: `View a subscription by ID.

Examples:
  asc subscriptions view --id "SUB_ID"
  asc subscriptions view --id "SUB_ID" --include versions --versions-limit 10`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyVersionLimit.Apply(versionsLimit); err != nil {
				return err
			}
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := legacySubscriptionID.Apply(subID); err != nil {
				return err
			}
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if err := validateRelationshipLimit("--versions-limit", *versionsLimit); err != nil {
				return err
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(fs, *include, "--include", subscriptionIncludeList())
			if err != nil {
				return err
			}

			client, err := subscriptionQueryClientFactory()
			if err != nil {
				return fmt.Errorf("subscriptions view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscription(
				requestCtx, id,
				asc.WithSubscriptionFields(fieldValues),
				asc.WithSubscriptionIncludedVersionFields(versionFieldValues),
				asc.WithSubscriptionInclude(includeValues),
				asc.WithSubscriptionVersionLimit(*versionsLimit),
			)
			if err != nil {
				return fmt.Errorf("subscriptions view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsUpdateCommand returns the subscriptions update subcommand.
func SubscriptionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	reviewNote := fs.String("review-note", "", "Review note for App Review")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	var groupLevel optionalInt
	fs.Var(&groupLevel, "group-level", "Subscription ordering level (positive integer)")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")
	output := shared.BindOutputFlags(fs)

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
  asc subscriptions update --id "SUB_ID" --family-sharable`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			name := strings.TrimSpace(*referenceName)
			note := strings.TrimSpace(*reviewNote)
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if visited["review-note"] && note == "" {
				fmt.Fprintln(os.Stderr, "Error: --review-note cannot be empty")
				return flag.ErrHelp
			}
			period, err := normalizeSubscriptionPeriod(*subscriptionPeriod, false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if groupLevel.IsSet() && groupLevel.Value() <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --group-level must be a positive integer")
				return flag.ErrHelp
			}

			if name == "" && note == "" && period == "" && !*familySharable && !groupLevel.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionUpdateAttributes{}
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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

// SubscriptionsDeleteCommand returns the subscriptions delete subcommand.
func SubscriptionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions delete --id \"SUB_ID\" --confirm",
		ShortHelp:  "Delete a subscription.",
		LongHelp: `Delete a subscription.

Examples:
  asc subscriptions delete --id "SUB_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscription(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions delete: failed to delete: %w", err)
			}

			result := &asc.SubscriptionDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricesCommand returns the subscriptions prices command group.
func SubscriptionsPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions prices <subcommand> [flags]",
		ShortHelp:  "Manage subscription pricing.",
		LongHelp: `Manage subscription pricing.

Examples:
  asc subscriptions prices list --subscription-id "SUB_ID"
  asc subscriptions prices add --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions prices import --subscription-id "SUB_ID" --input "./prices.csv" --confirm
  asc subscriptions prices delete --price-id "PRICE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricesListCommand(),
			SubscriptionsPricesAddCommand(),
			SubscriptionsPricesImportCommand(),
			SubscriptionsPricesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricesListCommand returns the subscriptions prices list subcommand.
func SubscriptionsPricesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices list", flag.ExitOnError)

	subID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	planType := fs.String("plan-type", "", "Filter by plan type: MONTHLY or UPFRONT")
	territory := fs.String("territory", "", "Filter by territory (accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions prices list --subscription-id \"SUB_ID\" [--plan-type MONTHLY|UPFRONT] [--territory USA]",
		ShortHelp:  "List prices for a subscription.",
		LongHelp: `List prices for a subscription.

Use --plan-type to filter by MONTHLY or UPFRONT billing plan prices.
Use --territory to filter by a single territory.

Examples:
  asc subscriptions prices list --subscription-id "SUB_ID"
  asc subscriptions prices list --subscription-id "SUB_ID" --paginate
  asc subscriptions prices list --subscription-id "SUB_ID" --resolved
  asc subscriptions prices list --subscription-id "SUB_ID" --resolved --territory USA
  asc subscriptions prices list --subscription-id "SUB_ID" --plan-type MONTHLY`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions prices list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions prices list: %w", err)
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(os.Stderr, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*subID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
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
					return shared.UsageError("invalid value for --plan-type: cannot be empty")
				}
				normalized, err := normalizeSubscriptionPlanType(*planType)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				planTypeFilter = normalized
			}

			territoryFilter := strings.TrimSpace(*territory)
			territoryProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "territory" {
					territoryProvided = true
				}
			})
			if territoryProvided {
				if territoryFilter == "" {
					return shared.UsageError("invalid value for --territory: cannot be empty")
				}
				normalizedTerritory, normalizeErr := ascterritory.Normalize(territoryFilter)
				if normalizeErr != nil {
					return shared.UsageError(normalizeErr.Error())
				}
				territoryFilter = normalizedTerritory
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions prices list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			if *resolved {
				resp, err := fetchResolvedSubscriptionPrices(ctx, client, id, *limit, *next, time.Now().UTC(), planTypeFilter, territoryFilter)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: failed to resolve: %w", err)
				}
				return shared.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			nextURL := strings.TrimSpace(*next)
			if nextURL != "" && (planTypeFilter != "" || territoryFilter != "") {
				nextURL, err = mergeSubscriptionPricesListFilters(nextURL, planTypeFilter, territoryFilter)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: %w", err)
				}
			}
			opts := []asc.SubscriptionPricesOption{
				asc.WithSubscriptionPricesLimit(*limit),
				asc.WithSubscriptionPricesNextURL(nextURL),
			}
			if planTypeFilter != "" && nextURL == "" {
				opts = append(opts, asc.WithSubscriptionPricesPlanType(planTypeFilter))
			}
			if territoryFilter != "" && nextURL == "" {
				opts = append(opts, asc.WithSubscriptionPricesTerritory(territoryFilter))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionPricesLimit(200))
				firstPage, err := client.GetSubscriptionPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions prices list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					nextURL, err := mergeSubscriptionPricesListFilters(nextURL, planTypeFilter, territoryFilter)
					if err != nil {
						return nil, err
					}
					return client.GetSubscriptionPrices(ctx, id, asc.WithSubscriptionPricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions prices list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions prices list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func mergeSubscriptionPricesPlanType(next string, planType asc.SubscriptionPlanType) (string, error) {
	return mergeSubscriptionPricesListFilters(next, planType, "")
}

func mergeSubscriptionPricesListFilters(next string, planType asc.SubscriptionPlanType, territory string) (string, error) {
	if planType == "" && strings.TrimSpace(territory) == "" {
		return next, nil
	}

	additions := url.Values{}
	if planType != "" {
		additions.Set("filter[planType]", string(planType))
	}
	if strings.TrimSpace(territory) != "" {
		additions.Set("filter[territory]", strings.ToUpper(strings.TrimSpace(territory)))
	}

	return mergeSubscriptionPricesNextQuery(next, additions)
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

func findMatchingSubscriptionPrice(ctx context.Context, client *asc.Client, subID, pricePointID, territoryID string, attrs asc.SubscriptionPriceCreateAttributes) (*asc.SubscriptionPriceResponse, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))

	opts := []asc.SubscriptionPricesOption{
		asc.WithSubscriptionPricesLimit(200),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
	}
	if territoryID != "" {
		opts = append(opts, asc.WithSubscriptionPricesTerritory(territoryID))
	}
	if attrs.PlanType != "" {
		opts = append(opts, asc.WithSubscriptionPricesPlanType(attrs.PlanType))
	}

	for {
		resp, err := client.GetSubscriptionPrices(ctx, subID, opts...)
		if err != nil {
			return nil, err
		}

		for _, price := range resp.Data {
			if subscriptionPriceMatchesTarget(price, pricePointID, territoryID, attrs) {
				return &asc.SubscriptionPriceResponse{Data: price}, nil
			}
		}

		next := strings.TrimSpace(resp.Links.Next)
		if next == "" {
			return nil, nil
		}
		nextURL, err := mergeSubscriptionPricesPlanType(next, attrs.PlanType)
		if err != nil {
			return nil, err
		}
		opts = []asc.SubscriptionPricesOption{asc.WithSubscriptionPricesNextURL(nextURL)}
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
	if strings.ToUpper(strings.TrimSpace(territoryID)) != actualTerritory {
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

// SubscriptionsPricesAddCommand returns the subscriptions prices add subcommand.
func SubscriptionsPricesAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices add", flag.ExitOnError)

	subID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := fs.String("app", "", subscriptionLookupAppUsage)
	pricePointID := fs.String("price-point", "", "Subscription price point ID")
	tier := fs.Int("tier", 0, "Pricing tier number (mutually exclusive with --price-point and --price)")
	price := fs.String("price", "", "Customer price to select price point (mutually exclusive with --price-point and --tier)")
	territory := fs.String("territory", "", "Territory input (accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD)")
	preserved := fs.Bool("preserved", false, "Preserve existing prices")
	force := fs.Bool("force", false, "Re-save the complete equalized price matrix even when the selected price is unchanged")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := shared.BindOutputFlags(fs)

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

By default, an identical existing price is returned without sending another
write. Use --force with --territory to rebuild and atomically re-save the full
equalized price matrix when repairing Apple's MISSING_METADATA state.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			pricePoint := strings.TrimSpace(*pricePointID)
			tierValue := *tier
			priceValue := strings.TrimSpace(*price)

			if err := shared.ValidatePriceSelectionFlags(pricePoint, tierValue, priceValue); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}
			if err := shared.ValidateFinitePriceFlag("--price", priceValue); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			territoryID := strings.TrimSpace(*territory)
			if territoryID != "" {
				normalizedTerritory, normalizeErr := ascterritory.Normalize(territoryID)
				if normalizeErr != nil {
					return shared.UsageError(normalizeErr.Error())
				}
				territoryID = normalizedTerritory
			}
			if tierValue > 0 || priceValue != "" {
				if territoryID == "" {
					fmt.Fprintln(os.Stderr, "Error: --territory is required when using --tier or --price")
					return shared.MissingRequiredUsageError()
				}
			}
			if *force && territoryID == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory is required with --force")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions prices add: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			if tierValue > 0 || priceValue != "" {
				tierCtx, tierCancel := shared.ContextWithTimeout(ctx)
				tiers, err := shared.ResolveSubscriptionTiers(tierCtx, client, id, territoryID, *refresh)
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

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Check if the subscription already has prices.
			// New subscriptions without prices require PATCH /v1/subscriptions/{id}
			// with inline price resources, while subscriptions that already have
			// prices use POST /v1/subscriptionPrices for price changes.
			existingPrices, pricesErr := client.GetSubscriptionPricesRelationships(requestCtx, id)
			if pricesErr != nil {
				return fmt.Errorf("subscriptions prices add: failed to check existing prices: %w", pricesErr)
			}
			hasExistingPrices := len(existingPrices.Data) > 0

			attrs := asc.SubscriptionPriceCreateAttributes{
				StartDate: strings.TrimSpace(*startDate),
			}
			if *preserved {
				attrs.Preserved = preserved
			}

			if !hasExistingPrices {
				// Initial price: use PATCH with inline resources
				subResp, err := client.SetSubscriptionInitialPrice(requestCtx, id, pricePoint, territoryID, attrs)
				if err != nil {
					return fmt.Errorf("subscriptions prices add: failed to set initial price: %w", err)
				}
				return shared.PrintOutput(subResp, *output.Output, *output.Pretty)
			}

			matchingPrice, err := findMatchingSubscriptionPrice(requestCtx, client, id, pricePoint, territoryID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions prices add: failed to check matching price: %w", err)
			}
			if matchingPrice != nil && !*force {
				return shared.PrintOutput(matchingPrice, *output.Output, *output.Pretty)
			}
			if matchingPrice != nil && *force {
				equalizations, equalizationsErr := fetchEqualizations(requestCtx, client, pricePoint, territoryID)
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
				resp, matrixErr := client.SetSubscriptionPriceMatrix(requestCtx, id, matrix)
				if matrixErr != nil {
					return fmt.Errorf("subscriptions prices add: failed to re-save price matrix: %w", matrixErr)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			// Existing prices: use POST /v1/subscriptionPrices for a price change
			resp, err := client.CreateSubscriptionPrice(requestCtx, id, pricePoint, territoryID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions prices add: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricesDeleteCommand returns the subscriptions prices delete subcommand.
func SubscriptionsPricesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices delete", flag.ExitOnError)

	priceID := fs.String("price-id", "", "Subscription price ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions prices delete --price-id \"PRICE_ID\" --confirm",
		ShortHelp:  "Delete a subscription price.",
		LongHelp: `Delete a subscription price.

Examples:
  asc subscriptions prices delete --price-id "PRICE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*priceID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions prices delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionPrice(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions prices delete: failed to delete: %w", err)
			}

			result := &asc.SubscriptionPriceDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsAvailabilityCommand returns the subscriptions pricing availability command group.
func SubscriptionsAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc subscriptions pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage subscription availability (deprecated by Apple).",
		LongHelp: `Manage subscription availability.

Deprecated: the underlying Subscription availability resource is deprecated in
App Store Connect API 4.4 in favor of Subscription plan availability. These
commands keep working for now; for plan-based availability use
` + "`asc subscriptions pricing monthly-commitment`" + ` (enable/disable/list).

Examples:
  asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"
  asc subscriptions pricing availability available-territories --availability-id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsAvailabilityViewCommand(),
			SubscriptionsAvailabilityAvailableTerritoriesCommand(),
			SubscriptionsAvailabilityEditCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsAvailabilityViewCommand returns the availability view subcommand.
func SubscriptionsAvailabilityViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability view", flag.ExitOnError)

	availabilityID := fs.String("availability-id", "", "Subscription availability ID")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions pricing availability view --availability-id \"AVAILABILITY_ID\"",
		ShortHelp:  "View subscription availability by ID or subscription.",
		LongHelp: `View subscription availability by ID or subscription.

Examples:
  asc subscriptions pricing availability view --availability-id "AVAILABILITY_ID"
  asc subscriptions pricing availability view --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			availabilityValue := strings.TrimSpace(*availabilityID)
			subscriptionValue := strings.TrimSpace(*subscriptionID)
			if availabilityValue == "" && subscriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --availability-id or --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}
			if availabilityValue != "" && subscriptionValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --availability-id and --subscription-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability view: %w", err)
			}

			if availabilityValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetSubscriptionAvailability(requestCtx, availabilityValue)
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability view: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			subscriptionValue, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, subscriptionValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAvailabilityForSubscription(requestCtx, subscriptionValue)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsAvailabilityAvailableTerritoriesCommand returns the available territories subcommand.
func SubscriptionsAvailabilityAvailableTerritoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability available-territories", flag.ExitOnError)

	availabilityID := fs.String("availability-id", "", "Subscription availability ID")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions pricing availability available-territories: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions pricing availability available-territories: %w", err)
			}
			if err := validateNextExclusiveFlags(fs, *next, "availability-id", "subscription-id", "app", "limit"); err != nil {
				return err
			}

			availabilityValue := strings.TrimSpace(*availabilityID)
			subscriptionValue := strings.TrimSpace(*subscriptionID)
			if availabilityValue == "" && subscriptionValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --availability-id or --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}
			if availabilityValue != "" && subscriptionValue != "" {
				return shared.UsageError("--availability-id and --subscription-id are mutually exclusive")
			}
			if subscriptionValue == "" && flagWasProvided(fs, "app") {
				return shared.UsageError("--app requires --subscription-id")
			}
			resolvedAppID := shared.ResolveAppID(*appID)
			if err := shared.RequireAppForStableSelector(resolvedAppID, subscriptionValue, "--subscription-id"); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability available-territories: %w", err)
			}
			if subscriptionValue != "" {
				subscriptionValue, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, resolvedAppID, subscriptionValue)
				if err != nil {
					return err
				}

				availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
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

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionAvailabilityTerritoriesOption{
				asc.WithSubscriptionAvailabilityTerritoriesLimit(*limit),
				asc.WithSubscriptionAvailabilityTerritoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
				firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability available-territories: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionAvailabilityAvailableTerritories(ctx, availabilityValue, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability available-territories: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityValue, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability available-territories: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsAvailabilityEditCommand returns the availability edit subcommand.
func SubscriptionsAvailabilityEditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability edit", flag.ExitOnError)

	subID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	territories := fs.String("territories", "", "Territory IDs, comma-separated")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new territories automatically")
	billingMode := fs.String("billing-mode", string(subscriptionBillingModeUpfront), "[experimental] Billing mode: upfront or monthly-commitment")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc subscriptions pricing availability edit [flags]",
		ShortHelp:  "Edit subscription availability in territories.",
		LongHelp: `Edit subscription availability in territories.

Examples:
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "US,Canada"
  asc subscriptions pricing availability edit --subscription-id "SUB_ID" --billing-mode monthly-commitment --territories "Norway,Germany"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}

			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territories is required")
				return shared.MissingRequiredUsageError()
			}
			normalizedBillingMode, err := normalizeSubscriptionBillingMode(*billingMode)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if normalizedBillingMode == subscriptionBillingModeMonthlyCommitment {
				if *availableInNew {
					return shared.UsageError("--available-in-new-territories is not supported for MONTHLY plan availability")
				}
				territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
				printMonthlyCommitmentTerritoryWarning(excluded)
				if len(territoryIDs) == 0 {
					return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability edit: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			if normalizedBillingMode == subscriptionBillingModeMonthlyCommitment {
				listCtx, listCancel := shared.ContextWithTimeout(ctx)
				existing, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(listCtx, id)
				listCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability edit: failed to fetch monthly-commitment plan availability: %w", err)
				}

				var resp *asc.SubscriptionPlanAvailabilityResponse
				if monthlyPlan, ok := findMonthlySubscriptionPlanAvailability(existing); ok {
					updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
					resp, err = client.UpdateSubscriptionPlanAvailability(updateCtx, monthlyPlan.ID, territoryIDs, nil)
					updateCancel()
				} else {
					createCtx, createCancel := shared.ContextWithTimeout(ctx)
					resp, err = client.CreateSubscriptionPlanAvailability(createCtx, id, territoryIDs, asc.SubscriptionPlanAvailabilityAttributes{
						PlanType: asc.SubscriptionPlanTypeMonthly,
					})
					createCancel()
				}
				if err != nil {
					return fmt.Errorf("subscriptions pricing availability edit: failed to set monthly-commitment plan availability: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionAvailabilityAttributes{
				AvailableInNewTerritories: *availableInNew,
			}

			resp, err := client.CreateSubscriptionAvailability(requestCtx, id, territoryIDs, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions pricing availability edit: failed to set: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
