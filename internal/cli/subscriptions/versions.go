package subscriptions

import (
	"context"
	"flag"
	"fmt"
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

// newSubscriptionsVersionsCommand returns the version-scoped subscription command group.
func newSubscriptionsVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc subscriptions versions <subcommand> [flags]",
		ShortHelp:  "Manage version-scoped subscription metadata.",
		LongHelp: `Manage version-scoped subscription metadata.

Version IDs are distinct from subscription product IDs. The product-scoped
"subscriptions localizations" and "subscriptions images" commands are
deprecated; use these version-scoped commands for new workflows.

Examples:
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions create --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions localizations list --version-id "VERSION_ID"
  asc subscriptions versions images upload --version-id "VERSION_ID" --file "./image.png"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsVersionsCreateCommand(d),
			newSubscriptionsVersionsListCommand(d),
			newSubscriptionsVersionsViewCommand(d),
			newSubscriptionsVersionsLinksCommand(d),
			newSubscriptionsVersionLocalizationsCommand(d),
			newSubscriptionsVersionImagesCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// newSubscriptionsVersionsCreateCommand returns the versions create command.
func newSubscriptionsVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions create", flag.ExitOnError)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions versions create --subscription-id \"SUBSCRIPTION_ID\"",
		ShortHelp:  "Create a subscription version.",
		LongHelp: `Create a subscription version.

Examples:
  asc subscriptions versions create --subscription-id "SUBSCRIPTION_ID"`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions create: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions versions create: failed to create: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionsListCommand returns the versions list command.
func newSubscriptionsVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions list", flag.ExitOnError)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	state := fs.String("state", "", "Filter by version state (comma-separated)")
	fields := fs.String("fields", "", "Sparse fields for subscriptionVersions")
	subscriptionFields := fs.String("subscription-fields", "", "Sparse fields for included subscriptions")
	imageFields := fs.String("image-fields", "", "Sparse fields for included subscriptionImages")
	localizationFields := fs.String("localization-fields", "", "Sparse fields for included subscriptionLocalizations")
	include := fs.String("include", "", "Include relationships: subscription,image,images,localizations")
	imagesLimit := fs.Int("images-limit", 0, "Maximum included images (1-50)")
	localizationsLimit := fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc subscriptions versions list [flags]", ShortHelp: "List versions for a subscription.",
		LongHelp: `List versions for a subscription.

Examples:
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID"
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID" --state PREPARE_FOR_SUBMISSION --include localizations --localizations-limit 10
  asc subscriptions versions list --subscription-id "SUBSCRIPTION_ID" --include images --images-limit 10`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions versions list", 200, "state", "fields", "subscription-fields", "image-fields", "localization-fields", "include", "images-limit", "localizations-limit", "limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--subscription-id")
			}
			if err := validateVersionListLimits(d, *imagesLimit, *localizationsLimit); err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			subscriptionFieldValues, err := normalizeSelectionFlag(d, fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList())
			if err != nil {
				return err
			}
			imageFieldValues, err := normalizeSelectionFlag(d, fs, *imageFields, "--image-fields", subscriptionVersionImageFieldsList())
			if err != nil {
				return err
			}
			localizationFieldValues, err := normalizeSelectionFlag(d, fs, *localizationFields, "--localization-fields", subscriptionVersionLocalizationFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(d, fs, *include, "--include", subscriptionVersionIncludeList())
			if err != nil {
				return err
			}
			if flagWasProvided(fs, "state") && len(csvValues(*state)) == 0 {
				return d.UsageError("--state must not be empty")
			}
			states, err := parseSubscriptionVersionStates(*state)
			if err != nil {
				return d.UsageErrorf("subscriptions versions list: %v", err)
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions list: %w", err)
			}
			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
				if err != nil {
					return err
				}
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			opts := []asc.SubscriptionVersionsOption{
				asc.Limit[asc.SubscriptionVersionsOption](firstPageLimit), asc.WithSubscriptionVersionsNextURL(*next),
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
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionsViewCommand returns the versions view command.
func newSubscriptionsVersionsViewCommand(d *shared.Deps) *ffcli.Command {
	var (
		imagesLimit, localizationsLimit                                                                       *int
		versionFieldValues, subscriptionFieldValues, imageFieldValues, localizationFieldValues, includeValues []string
	)
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "versions view",
		Name:        "view", ShortUsage: "asc subscriptions versions view --id \"VERSION_ID\"", ShortHelp: "View a subscription version.",
		LongHelp: `View a subscription version by ID.

Examples:
  asc subscriptions versions view --id "VERSION_ID"
  asc subscriptions versions view --id "VERSION_ID" --include images,localizations --images-limit 10 --localizations-limit 10`,
		IDUsage: "Subscription version ID", IDType: "subscriptionVersions", RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			shared.BindFlagAlias(fs, "version-id", "id")
			fields := fs.String("fields", "", "Sparse fields for subscriptionVersions")
			subscriptionFields := fs.String("subscription-fields", "", "Sparse fields for included subscriptions")
			imageFields := fs.String("image-fields", "", "Sparse fields for included subscriptionImages")
			localizationFields := fs.String("localization-fields", "", "Sparse fields for included subscriptionLocalizations")
			include := fs.String("include", "", "Include relationships: subscription,image,images,localizations")
			imagesLimit = fs.Int("images-limit", 0, "Maximum included images (1-50)")
			localizationsLimit = fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
			return func() (err error) {
				if err := validateRelationshipLimit(d, "--images-limit", *imagesLimit); err != nil {
					return err
				}
				if err := validateRelationshipLimit(d, "--localizations-limit", *localizationsLimit); err != nil {
					return err
				}
				if versionFieldValues, err = normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionVersionFieldsList()); err != nil {
					return err
				}
				if subscriptionFieldValues, err = normalizeSelectionFlag(d, fs, *subscriptionFields, "--subscription-fields", subscriptionFieldsList()); err != nil {
					return err
				}
				if imageFieldValues, err = normalizeSelectionFlag(d, fs, *imageFields, "--image-fields", subscriptionVersionImageFieldsList()); err != nil {
					return err
				}
				if localizationFieldValues, err = normalizeSelectionFlag(d, fs, *localizationFields, "--localization-fields", subscriptionVersionLocalizationFieldsList()); err != nil {
					return err
				}
				includeValues, err = normalizeSelectionFlag(d, fs, *include, "--include", subscriptionVersionIncludeList())
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionVersion(
				ctx, id,
				asc.WithSubscriptionVersionFields(versionFieldValues),
				asc.WithSubscriptionVersionSubscriptionFields(subscriptionFieldValues),
				asc.WithSubscriptionVersionImageFields(imageFieldValues),
				asc.WithSubscriptionVersionLocalizationFields(localizationFieldValues),
				asc.WithSubscriptionVersionInclude(includeValues),
				asc.WithSubscriptionVersionImageLimit(*imagesLimit),
				asc.WithSubscriptionVersionLocalizationLimit(*localizationsLimit),
			)
		},
	})
}

// newSubscriptionsVersionsLinksCommand returns the subscription-to-version linkage command.
func newSubscriptionsVersionsLinksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions links", flag.ExitOnError)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions links [flags]", ShortHelp: "List raw subscription version linkages.",
		LongHelp: "List raw version relationship linkages for a subscription.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions versions links", 200, "limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--subscription-id")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions links: %w", err)
			}
			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
				if err != nil {
					return err
				}
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			resp, err := client.GetSubscriptionVersionsRelationships(requestCtx, id, asc.Limit[asc.LinkagesOption](firstPageLimit), asc.NextURL[asc.LinkagesOption](*next))
			if err != nil {
				return fmt.Errorf("subscriptions versions links: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionsRelationships(pageCtx, id, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions links: %w", err)
				}
				resp = aggregated.(*asc.LinkagesResponse)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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

func validateRelationshipLimit(d *shared.Deps, name string, limit int) error {
	if limit != 0 && (limit < 1 || limit > 50) {
		return d.UsageErrorf("%s must be between 1 and 50", name)
	}
	return nil
}

func validateVersionListLimits(d *shared.Deps, imagesLimit, localizationsLimit int) error {
	if err := validateRelationshipLimit(d, "--images-limit", imagesLimit); err != nil {
		return err
	}
	return validateRelationshipLimit(d, "--localizations-limit", localizationsLimit)
}
