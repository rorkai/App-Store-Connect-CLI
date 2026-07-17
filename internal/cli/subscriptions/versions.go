package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var subscriptionVersionStates = map[string]asc.SubscriptionVersionState{
	"PREPARE_FOR_SUBMISSION":    asc.SubscriptionVersionStatePrepareForSubmission,
	"READY_FOR_REVIEW":          asc.SubscriptionVersionStateReadyForReview,
	"WAITING_FOR_REVIEW":        asc.SubscriptionVersionStateWaitingForReview,
	"IN_REVIEW":                 asc.SubscriptionVersionStateInReview,
	"ACCEPTED":                  asc.SubscriptionVersionStateAccepted,
	"APPROVED":                  asc.SubscriptionVersionStateApproved,
	"REPLACED_WITH_NEW_VERSION": asc.SubscriptionVersionStateReplacedWithNewVersion,
	"REJECTED":                  asc.SubscriptionVersionStateRejected,
	"DEVELOPER_REJECTED":        asc.SubscriptionVersionStateDeveloperRejected,
}

// SubscriptionsVersionsCommand returns the version-scoped subscription command group.
func SubscriptionsVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc subscriptions versions <subcommand> [flags]",
		ShortHelp:  "Manage version-scoped subscription metadata.",
		LongHelp: `Manage version-scoped subscription metadata.

Version IDs are distinct from subscription product IDs. Existing
"subscriptions localizations" and "subscriptions images" commands continue to
use product-scoped v1 resources.

Examples:
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions create --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions localizations list --version-id "VERSION_ID"
  asc subscriptions versions images upload --version-id "VERSION_ID" --file "./image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsVersionsCreateCommand(),
			SubscriptionsVersionsListCommand(),
			SubscriptionsVersionsViewCommand(),
			SubscriptionsVersionsLinksCommand(),
			SubscriptionsVersionLocalizationsCommand(),
			SubscriptionsVersionImagesCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SubscriptionsVersionsCreateCommand returns the versions create command.
func SubscriptionsVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions create", flag.ExitOnError)
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions versions create --subscription-id \"SUBSCRIPTION_ID\"",
		ShortHelp:  "Create a subscription version.",
		LongHelp: `Create a subscription version.

Examples:
  asc subscriptions versions create --subscription-id "SUBSCRIPTION_ID"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError("--subscription-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions create: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions versions create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionsListCommand returns the versions list command.
func SubscriptionsVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions list", flag.ExitOnError)
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	state := fs.String("state", "", "Filter by version state (comma-separated)")
	fields := fs.String("fields", "", "Sparse fields for subscriptionVersions")
	subscriptionFields := fs.String("subscription-fields", "", "Sparse fields for included subscriptions")
	imageFields := fs.String("image-fields", "", "Sparse fields for included subscriptionImages")
	localizationFields := fs.String("localization-fields", "", "Sparse fields for included subscriptionLocalizations")
	include := fs.String("include", "", "Include relationships: subscription,image,images,localizations")
	imagesLimit := fs.Int("images-limit", 0, "Maximum included images (1-50)")
	legacyImageLimit := shared.BindDeprecatedIntFlagAlias(fs, "image-limit", "images-limit")
	localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	legacyLocalizationLimit := shared.BindDeprecatedIntFlagAlias(fs, "localization-limit", "localizations-limit")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc subscriptions versions list [flags]", ShortHelp: "List versions for a subscription.",
		LongHelp: `List versions for a subscription.

Examples:
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID" --state PREPARE_FOR_SUBMISSION --include localizations --localizations-limit 10
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID" --include images --images-limit 10`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyImageLimit.Apply(imagesLimit); err != nil {
				return err
			}
			if err := legacyLocalizationLimit.Apply(localizationsLimit); err != nil {
				return err
			}
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--subscription-id", flagWasProvided(fs, "subscription-id")},
				flagConflict{"--app", flagWasProvided(fs, "app")},
				flagConflict{"--state", flagWasProvided(fs, "state")},
				flagConflict{"--fields", flagWasProvided(fs, "fields")},
				flagConflict{"--subscription-fields", flagWasProvided(fs, "subscription-fields")},
				flagConflict{"--image-fields", flagWasProvided(fs, "image-fields")},
				flagConflict{"--localization-fields", flagWasProvided(fs, "localization-fields")},
				flagConflict{"--include", flagWasProvided(fs, "include")},
				flagConflict{"--images-limit", flagWasProvided(fs, "images-limit") || legacyImageLimit.WasProvided()},
				flagConflict{"--localizations-limit", flagWasProvided(fs, "localizations-limit") || legacyLocalizationLimit.WasProvided()},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validateVersionListLimits(*limit, *imagesLimit, *localizationsLimit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions list: %v", err)
			}
			versionFieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			subscriptionFieldValues, err := normalizeSelectionFlag(fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			imageFieldValues, err := normalizeSelectionFlag(fs, *imageFields, "--image-fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			localizationFieldValues, err := normalizeSelectionFlag(fs, *localizationFields, "--localization-fields", subscriptionVersionLocalizationFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(fs, *include, "--include", subscriptionVersionIncludeList())
			if err != nil {
				return err
			}
			if flagWasProvided(fs, "state") && len(csvValues(*state)) == 0 {
				return shared.UsageError("--state must not be empty")
			}
			states, err := parseSubscriptionVersionStates(*state)
			if err != nil {
				return shared.UsageErrorf("subscriptions versions list: %v", err)
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError("--subscription-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions list: %w", err)
			}
			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			opts := []asc.SubscriptionVersionsOption{
				asc.WithSubscriptionVersionsLimit(*limit), asc.WithSubscriptionVersionsNextURL(*next),
				asc.WithSubscriptionVersionsStates(states), asc.WithSubscriptionVersionsFields(versionFieldValues),
				asc.WithSubscriptionVersionsSubscriptionFields(subscriptionFieldValues),
				asc.WithSubscriptionVersionsImageFields(imageFieldValues),
				asc.WithSubscriptionVersionsLocalizationFields(localizationFieldValues),
				asc.WithSubscriptionVersionsInclude(includeValues),
				asc.WithSubscriptionVersionsImageLimit(*imagesLimit), asc.WithSubscriptionVersionsLocalizationLimit(*localizationsLimit),
			}
			resp, err := client.GetSubscriptionVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions versions list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersions(pageCtx, id, asc.WithSubscriptionVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions list: %w", err)
				}
				resp = aggregated.(*asc.SubscriptionVersionsResponse)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionsViewCommand returns the versions view command.
func SubscriptionsVersionsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions view", flag.ExitOnError)
	id := fs.String("id", "", "Subscription version ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionVersions")
	subscriptionFields := fs.String("subscription-fields", "", "Sparse fields for included subscriptions")
	imageFields := fs.String("image-fields", "", "Sparse fields for included subscriptionImages")
	localizationFields := fs.String("localization-fields", "", "Sparse fields for included subscriptionLocalizations")
	include := fs.String("include", "", "Include relationships: subscription,image,images,localizations")
	imagesLimit := fs.Int("images-limit", 0, "Maximum included images (1-50)")
	legacyImageLimit := shared.BindDeprecatedIntFlagAlias(fs, "image-limit", "images-limit")
	localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	legacyLocalizationLimit := shared.BindDeprecatedIntFlagAlias(fs, "localization-limit", "localizations-limit")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: "asc subscriptions versions view --id \"VERSION_ID\"", ShortHelp: "View a subscription version.",
		LongHelp: `View a subscription version by ID.

Examples:
  asc subscriptions versions view --id "VERSION_ID"
  asc subscriptions versions view --id "VERSION_ID" --include images,localizations --images-limit 10 --localizations-limit 10`, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyImageLimit.Apply(imagesLimit); err != nil {
				return err
			}
			if err := legacyLocalizationLimit.Apply(localizationsLimit); err != nil {
				return err
			}
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			versionID := strings.TrimSpace(*id)
			if versionID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if err := validateRelationshipLimit("--images-limit", *imagesLimit); err != nil {
				return err
			}
			if err := validateRelationshipLimit("--localizations-limit", *localizationsLimit); err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			subscriptionFieldValues, err := normalizeSelectionFlag(fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			imageFieldValues, err := normalizeSelectionFlag(fs, *imageFields, "--image-fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			localizationFieldValues, err := normalizeSelectionFlag(fs, *localizationFields, "--localization-fields", subscriptionVersionLocalizationFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(fs, *include, "--include", subscriptionVersionIncludeList())
			if err != nil {
				return err
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersion(
				requestCtx, versionID,
				asc.WithSubscriptionVersionFields(versionFieldValues),
				asc.WithSubscriptionVersionSubscriptionFields(subscriptionFieldValues),
				asc.WithSubscriptionVersionImageFields(imageFieldValues),
				asc.WithSubscriptionVersionLocalizationFields(localizationFieldValues),
				asc.WithSubscriptionVersionInclude(includeValues),
				asc.WithSubscriptionVersionImageLimit(*imagesLimit),
				asc.WithSubscriptionVersionLocalizationLimit(*localizationsLimit),
			)
			if err != nil {
				return fmt.Errorf("subscriptions versions view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionsLinksCommand returns the subscription-to-version linkage command.
func SubscriptionsVersionsLinksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions links", flag.ExitOnError)
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions links [flags]", ShortHelp: "List raw subscription version linkages.",
		LongHelp: "List raw version relationship linkages for a subscription.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--subscription-id", flagWasProvided(fs, "subscription-id")},
				flagConflict{"--app", flagWasProvided(fs, "app")},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validatePageLimit(*limit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions links: %v", err)
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError("--subscription-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions links: %w", err)
			}
			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionsRelationships(requestCtx, id, asc.WithLinkagesLimit(*limit), asc.WithLinkagesNextURL(*next))
			if err != nil {
				return fmt.Errorf("subscriptions versions links: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionsRelationships(pageCtx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions links: %w", err)
				}
				resp = aggregated.(*asc.LinkagesResponse)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func csvValues(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func parseSubscriptionVersionStates(value string) ([]asc.SubscriptionVersionState, error) {
	values := csvValues(strings.ToUpper(value))
	states := make([]asc.SubscriptionVersionState, 0, len(values))
	for _, value := range values {
		state, ok := subscriptionVersionStates[value]
		if !ok {
			return nil, fmt.Errorf("invalid --state %q", value)
		}
		states = append(states, state)
	}
	return states, nil
}

func validatePageLimit(limit int) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return shared.UsageError("--limit must be between 1 and 200")
	}
	return nil
}

func validateRelationshipLimit(name string, limit int) error {
	if limit != 0 && (limit < 1 || limit > 50) {
		return shared.UsageErrorf("%s must be between 1 and 50", name)
	}
	return nil
}

func validateVersionListLimits(limit, imagesLimit, localizationsLimit int) error {
	if err := validatePageLimit(limit); err != nil {
		return err
	}
	if err := validateRelationshipLimit("--images-limit", imagesLimit); err != nil {
		return err
	}
	return validateRelationshipLimit("--localizations-limit", localizationsLimit)
}

type flagConflict struct {
	name string
	set  bool
}

func validateNextFlagConflicts(next string, flags ...flagConflict) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	for _, candidate := range flags {
		if candidate.set {
			return shared.UsageErrorf("--next cannot be combined with %s", candidate.name)
		}
	}
	return nil
}

func rejectUnexpectedArgs(args []string) error {
	if len(args) > 0 {
		return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
	}
	return nil
}
