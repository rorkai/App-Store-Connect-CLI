package reviews

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

var reviewItemsClientFactory = shared.GetASCClient

// ReviewItemsCommand returns the nested review items command group.
func ReviewItemsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "items",
		ShortUsage: "asc review items <subcommand> [flags]",
		ShortHelp:  "Manage review submission items.",
		LongHelp: `Manage review submission items.

Examples:
  asc review items list --submission "SUBMISSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"
  asc review items update --id "ITEM_ID" --resolved true
  asc review items remove --id "ITEM_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			reviewItemsGetCommand("view", "review items view", `asc review items view --id "ITEM_ID"`),
			reviewItemsListCommand("list", "review items list", `asc review items list [flags]`, `asc review items list --submission "SUBMISSION_ID"
  asc review items list --submission "SUBMISSION_ID" --paginate`),
			reviewItemsAddCommand("add", "review items add", `asc review items add [flags]`, `asc review items add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type gameCenterChallengeVersions --item-id "VERSION_ID"`),
			reviewItemsUpdateCommand("update", "review items update", `asc review items update --id "ITEM_ID" [flags]`, `asc review items update --id "ITEM_ID" --resolved true
  asc review items update --id "ITEM_ID" --clear-removed`),
			reviewItemsRemoveCommand("remove", "review items remove", `asc review items remove [flags]`, `asc review items remove --id "ITEM_ID" --confirm`),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ReviewItemsGetCommand returns the stable review items-get subcommand.
func ReviewItemsGetCommand() *ffcli.Command {
	return reviewItemsGetCommand("items-get", "review items-get", `asc review items-get --id "ITEM_ID"`)
}

func reviewItemsGetCommand(name, errorPrefix, example string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: example + " [flags]",
		ShortHelp:  "DEPRECATED: App Store Connect has no review-item detail endpoint.",
		LongHelp: `DEPRECATED: App Store Connect API 4.4.1 has no review-item detail endpoint.

Use asc review items list --submission "SUBMISSION_ID" instead.

Examples:
  ` + example,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("unexpected positional arguments"))
			}
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError(`App Store Connect API 4.4.1 has no item-detail GET; use asc review items list --submission "SUBMISSION_ID"`))
		},
	}
}

// ReviewItemsListCommand returns the review items list subcommand.
func ReviewItemsListCommand() *ffcli.Command {
	return reviewItemsListCommand("items-list", "review items-list", `asc review items-list [flags]`, `asc review items-list --submission "SUBMISSION_ID"
  asc review items-list --submission "SUBMISSION_ID" --paginate`)
}

func reviewItemsListCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	fields := fs.String("fields", "", "Review item fields: "+strings.Join(reviewSubmissionItemFields, ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewSubmissionItemIncludes, ", "))
	iapVersionFields := fs.String("iap-version-fields", "", "In-app purchase version fields: "+strings.Join(reviewSubmissionItemIAPVersionFields, ", "))
	subscriptionVersionFields := fs.String("subscription-version-fields", "", "Subscription version fields: "+strings.Join(reviewSubmissionItemSubscriptionVersionFields, ", "))
	subscriptionGroupVersionFields := fs.String("subscription-group-version-fields", "", "Subscription group version fields: "+strings.Join(reviewSubmissionItemSubscriptionGroupVersionFields, ", "))
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "List items in a review submission.",
		LongHelp: `List items in a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("unexpected positional arguments"))
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}
			if err := rejectReviewNextFlagConflicts(
				fs, *next, errorPrefix,
				"submission", "limit", "fields", "include", "iap-version-fields",
				"subscription-version-fields", "subscription-group-version-fields",
			); err != nil {
				return err
			}
			opts, err := reviewItemsListOptions(*limit, *next, *fields, *include, *iapVersionFields, *subscriptionVersionFields, *subscriptionGroupVersionFields)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError(err.Error()))
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewItemsClientFactory()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionItemsLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItems(ctx, strings.TrimSpace(*submissionID), paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItems(ctx, strings.TrimSpace(*submissionID), asc.WithReviewSubmissionItemsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("%s: %w", errorPrefix, err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetReviewSubmissionItems(requestCtx, strings.TrimSpace(*submissionID), opts...)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func rejectReviewNextFlagConflicts(fs *flag.FlagSet, next, command string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	provided := make(map[string]struct{}, len(names))
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = struct{}{}
	})
	for _, name := range names {
		if _, ok := provided[name]; ok {
			return shared.UsageErrorf("%s: --next cannot be combined with --%s", command, name)
		}
	}
	return nil
}

var reviewSubmissionItemFields = []string{
	"state", "appStoreVersion", "appCustomProductPageVersion", "appStoreVersionExperiment",
	"appStoreVersionExperimentV2", "appEvent", "backgroundAssetVersion", "gameCenterAchievementVersion",
	"gameCenterActivityVersion", "gameCenterChallengeVersion", "gameCenterLeaderboardSetVersion",
	"gameCenterLeaderboardVersion", "inAppPurchaseVersion", "subscriptionVersion", "subscriptionGroupVersion",
}

var reviewSubmissionItemIncludes = reviewSubmissionItemFields[1:]

var (
	reviewSubmissionItemIAPVersionFields               = []string{"version", "state", "inAppPurchase", "image", "images", "localizations"}
	reviewSubmissionItemSubscriptionVersionFields      = []string{"version", "state", "subscription", "image", "images", "localizations"}
	reviewSubmissionItemSubscriptionGroupVersionFields = []string{"version", "state", "subscriptionGroup", "localizations"}
)

func reviewItemsListOptions(limit int, next, fields, include, iapVersionFields, subscriptionVersionFields, subscriptionGroupVersionFields string) ([]asc.ReviewSubmissionItemsOption, error) {
	if limit != 0 && (limit < 1 || limit > 200) {
		return nil, fmt.Errorf("--limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return nil, err
	}
	if strings.TrimSpace(next) != "" && (limit != 0 || strings.TrimSpace(fields) != "" || strings.TrimSpace(include) != "" ||
		strings.TrimSpace(iapVersionFields) != "" || strings.TrimSpace(subscriptionVersionFields) != "" || strings.TrimSpace(subscriptionGroupVersionFields) != "") {
		return nil, fmt.Errorf("--next cannot be combined with --limit, --fields, --include, or version sparse-field flags")
	}

	itemFields, err := shared.NormalizeSelection(fields, reviewSubmissionItemFields, "--fields")
	if err != nil {
		return nil, err
	}
	includes, err := shared.NormalizeSelection(include, reviewSubmissionItemIncludes, "--include")
	if err != nil {
		return nil, err
	}
	iapFields, err := shared.NormalizeSelection(iapVersionFields, reviewSubmissionItemIAPVersionFields, "--iap-version-fields")
	if err != nil {
		return nil, err
	}
	subscriptionFields, err := shared.NormalizeSelection(subscriptionVersionFields, reviewSubmissionItemSubscriptionVersionFields, "--subscription-version-fields")
	if err != nil {
		return nil, err
	}
	groupFields, err := shared.NormalizeSelection(subscriptionGroupVersionFields, reviewSubmissionItemSubscriptionGroupVersionFields, "--subscription-group-version-fields")
	if err != nil {
		return nil, err
	}
	addVersionRelationship := func(versionFields []string, relationship string) {
		if len(versionFields) == 0 {
			return
		}
		if !slices.Contains(includes, relationship) {
			includes = append(includes, relationship)
		}
		if len(itemFields) != 0 && !slices.Contains(itemFields, relationship) {
			itemFields = append(itemFields, relationship)
		}
	}
	addVersionRelationship(iapFields, "inAppPurchaseVersion")
	addVersionRelationship(subscriptionFields, "subscriptionVersion")
	addVersionRelationship(groupFields, "subscriptionGroupVersion")

	return []asc.ReviewSubmissionItemsOption{
		asc.WithReviewSubmissionItemsLimit(limit),
		asc.WithReviewSubmissionItemsNextURL(next),
		asc.WithReviewSubmissionItemsFields(itemFields),
		asc.WithReviewSubmissionItemsInclude(includes),
		asc.WithReviewSubmissionItemsInAppPurchaseVersionFields(iapFields),
		asc.WithReviewSubmissionItemsSubscriptionVersionFields(subscriptionFields),
		asc.WithReviewSubmissionItemsSubscriptionGroupVersionFields(groupFields),
	}, nil
}

// ReviewItemsAddCommand returns the review items add subcommand.
func ReviewItemsAddCommand() *ffcli.Command {
	return reviewItemsAddCommand("items-add", "review items-add", `asc review items-add [flags]`, `asc review items-add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type gameCenterChallengeVersions --item-id "VERSION_ID"`)
}

func reviewItemsAddCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	itemTypeValues := strings.Join(reviewSubmissionItemTypeList(), ", ")
	itemType := fs.String("item-type", "", fmt.Sprintf("Item type: %s (required)", itemTypeValues))
	itemID := fs.String("item-id", "", "Item ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Add an item to a review submission.",
		LongHelp: `Add an item to a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("unexpected positional arguments"))
			}
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-type is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-id is required")
				return shared.MissingRequiredUsageError()
			}

			normalizedType, err := normalizeReviewSubmissionItemType(*itemType)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := reviewItemsClientFactory()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateReviewSubmissionItem(requestCtx, strings.TrimSpace(*submissionID), normalizedType, strings.TrimSpace(*itemID))
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewItemsUpdateCommand returns the review items update subcommand.
func ReviewItemsUpdateCommand() *ffcli.Command {
	return reviewItemsUpdateCommand("items-update", "review items-update", `asc review items-update --id "ITEM_ID" [flags]`, `asc review items-update --id "ITEM_ID" --resolved true
  asc review items-update --id "ITEM_ID" --clear-removed`)
}

func reviewItemsUpdateCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	fs.String("state", "", "Deprecated: no longer supported by App Store Connect")
	resolved := fs.String("resolved", "", "Whether the item is resolved: true or false")
	removed := fs.String("removed", "", "Whether the item is removed: true or false")
	clearResolved := fs.Bool("clear-resolved", false, "Set resolved to JSON null")
	clearRemoved := fs.Bool("clear-removed", false, "Set removed to JSON null")
	confirm := fs.Bool("confirm", false, "Confirm removal when --removed=true")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Update a review submission item's resolved or removed status.",
		LongHelp: `Update a review submission item.

Use --confirm when setting --removed=true.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("unexpected positional arguments"))
			}
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if reviewFlagWasProvided(fs, "state") {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("--state is deprecated and no longer supported by App Store Connect; use --resolved or --removed"))
			}
			resolvedProvided := reviewFlagWasProvided(fs, "resolved")
			removedProvided := reviewFlagWasProvided(fs, "removed")
			if resolvedProvided && *clearResolved {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("--resolved cannot be combined with --clear-resolved"))
			}
			if removedProvided && *clearRemoved {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("--removed cannot be combined with --clear-removed"))
			}
			if !resolvedProvided && !removedProvided && !*clearResolved && !*clearRemoved {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("at least one of --resolved, --removed, --clear-resolved, or --clear-removed is required"))
			}

			attrs := asc.ReviewSubmissionItemUpdateAttributes{}
			if resolvedProvided {
				value, err := parseReviewSubmissionItemBool(*resolved, "--resolved")
				if err != nil {
					return fmt.Errorf("%s: %w", errorPrefix, err)
				}
				attrs.Resolved = &asc.NullableBool{Value: &value}
			} else if *clearResolved {
				attrs.Resolved = &asc.NullableBool{}
			}
			if removedProvided {
				value, err := parseReviewSubmissionItemBool(*removed, "--removed")
				if err != nil {
					return fmt.Errorf("%s: %w", errorPrefix, err)
				}
				if value && !*confirm {
					return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("--confirm is required when --removed=true"))
				}
				attrs.Removed = &asc.NullableBool{Value: &value}
			} else if *clearRemoved {
				attrs.Removed = &asc.NullableBool{}
			}

			client, err := reviewItemsClientFactory()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateReviewSubmissionItem(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func reviewFlagWasProvided(fs *flag.FlagSet, names ...string) bool {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if _, ok := wanted[f.Name]; ok {
			provided = true
		}
	})
	return provided
}

func parseReviewSubmissionItemBool(value, name string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, shared.UsageErrorf("%s must be true or false", name)
	}
}

// ReviewItemsRemoveCommand returns the review items remove subcommand.
func ReviewItemsRemoveCommand() *ffcli.Command {
	return reviewItemsRemoveCommand("items-remove", "review items-remove", `asc review items-remove [flags]`, `asc review items-remove --id "ITEM_ID" --confirm`)
}

func reviewItemsRemoveCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm removal (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Remove an item from a review submission.",
		LongHelp: `Remove an item from a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%s: %w", errorPrefix, shared.UsageError("unexpected positional arguments"))
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to remove")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewItemsClientFactory()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteReviewSubmissionItem(requestCtx, strings.TrimSpace(*itemID)); err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			result := &asc.ReviewSubmissionItemDeleteResult{
				ID:      strings.TrimSpace(*itemID),
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeReviewSubmissionItemType(value string) (asc.ReviewSubmissionItemType, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("--item-type is required")
	}
	if strings.TrimSpace(value) == string(asc.ReviewSubmissionItemTypeAppStoreVersionExperimentTreatment) {
		return "", fmt.Errorf("--item-type %s is deprecated and no longer supported by App Store Connect; experiment treatments cannot be added as review submission items", asc.ReviewSubmissionItemTypeAppStoreVersionExperimentTreatment)
	}
	if strings.TrimSpace(value) == string(asc.ReviewSubmissionItemTypeAppCustomProductPage) {
		return "", fmt.Errorf("--item-type %s is deprecated and no longer supported by App Store Connect; pass an app custom product page version ID with --item-type %s", asc.ReviewSubmissionItemTypeAppCustomProductPage, asc.ReviewSubmissionItemTypeAppCustomProductPageVersion)
	}
	if itemType, ok := asc.ParseReviewSubmissionItemType(value); ok {
		return itemType, nil
	}
	return "", fmt.Errorf("--item-type must be one of: %s", strings.Join(reviewSubmissionItemTypeList(), ", "))
}

func reviewSubmissionItemTypeList() []string {
	return asc.ReviewSubmissionItemTypeNames()
}
