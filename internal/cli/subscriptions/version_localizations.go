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

type optionalString struct {
	value string
	set   bool
}

func (v *optionalString) String() string         { return v.value }
func (v *optionalString) Set(value string) error { v.value, v.set = value, true; return nil }

// newSubscriptionsVersionLocalizationsCommand returns version localization commands.
func newSubscriptionsVersionLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions localizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "localizations", ShortUsage: "asc subscriptions versions localizations <subcommand> [flags]",
		ShortHelp: "Manage version-scoped subscription localizations.",
		LongHelp: `Manage version-scoped subscription localizations.

Examples:
  asc subscriptions versions localizations list --version-id "VERSION_ID"
  asc subscriptions versions localizations create --version-id "VERSION_ID" --locale "en-US" --name "Premium"`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsVersionLocalizationsListCommand(d),
			newSubscriptionsVersionLocalizationsLinksCommand(d),
			newSubscriptionsVersionLocalizationsViewCommand(d),
			newSubscriptionsVersionLocalizationsCreateCommand(d),
			newSubscriptionsVersionLocalizationsUpdateCommand(d),
			newSubscriptionsVersionLocalizationsDeleteCommand(d),
			newSubscriptionsVersionLocalizationsImportCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// newSubscriptionsVersionLocalizationsListCommand lists related localizations.
func newSubscriptionsVersionLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions localizations list", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name; lists its newest version's localizations")
	appID := addSubscriptionLookupAppFlag(fs)
	fields := fs.String("fields", "", "Sparse fields for subscriptionLocalizations")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	include := fs.String("include", "", "Include relationships: version")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc subscriptions versions localizations list [flags]", ShortHelp: "List localizations for a subscription version.",
		LongHelp: `List version-scoped subscription localizations.

Pass --subscription-id to list the localizations of the subscription's newest
version: the draft being edited when there is one, else the live version.

Examples:
  asc subscriptions versions localizations list --version-id "VERSION_ID"
  asc subscriptions versions localizations list --subscription-id "SUBSCRIPTION_ID"`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions versions localizations list", 200, "subscription-id", "fields", "version-fields", "include", "limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			subscription := strings.TrimSpace(*subscriptionID)
			if id != "" && subscription != "" {
				return d.UsageError("--version-id and --subscription-id are mutually exclusive")
			}
			if subscription == "" && flagWasProvided(fs, "app") {
				return d.UsageError("--app requires --subscription-id")
			}
			if id == "" && subscription == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--version-id")
			}
			fieldValues, err := normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionVersionLocalizationFieldsList())
			if err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(d, fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(d, fs, *include, "--include", subscriptionLocalizationV2IncludeList())
			if err != nil {
				return err
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations list: %w", err)
			}
			if subscription != "" {
				id, err = resolveNewestSubscriptionVersionID(ctx, d, client, *appID, subscription)
				if err != nil {
					return fmt.Errorf("subscriptions versions localizations list: %w", err)
				}
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			resp, err := client.GetSubscriptionVersionLocalizations(
				requestCtx, id,
				asc.Limit[asc.SubscriptionVersionLocalizationsOption](firstPageLimit),
				asc.WithSubscriptionVersionLocalizationsNextURL(*next),
				asc.WithSubscriptionVersionLocalizationsFields(fieldValues),
				asc.WithSubscriptionVersionLocalizationsVersionFields(versionFieldValues),
				asc.WithSubscriptionVersionLocalizationsInclude(includeValues),
			)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionLocalizations(pageCtx, id, asc.WithSubscriptionVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions localizations list: %w", err)
				}
				resp = aggregated.(*asc.SubscriptionLocalizationsV2Response)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionLocalizationsLinksCommand lists raw localization linkages.
func newSubscriptionsVersionLocalizationsLinksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions localizations links", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions localizations links [flags]", ShortHelp: "List raw localization linkages.",
		LongHelp: "List raw localization relationship linkages for a subscription version.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions versions localizations links", 200, "limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--version-id")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations links: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			resp, err := client.GetSubscriptionVersionLocalizationsRelationships(requestCtx, id, asc.Limit[asc.LinkagesOption](firstPageLimit), asc.NextURL[asc.LinkagesOption](*next))
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations links: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionLocalizationsRelationships(pageCtx, id, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions versions localizations links: %w", err)
				}
				resp = aggregated.(*asc.LinkagesResponse)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionLocalizationsViewCommand views a v2 localization.
func newSubscriptionsVersionLocalizationsViewCommand(d *shared.Deps) *ffcli.Command {
	var fieldValues, versionFieldValues, includeValues []string
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "versions localizations view",
		Name:        "view", ShortUsage: "asc subscriptions versions localizations view --id \"LOCALIZATION_ID\"", ShortHelp: "View a version-scoped localization.",
		LongHelp: "View a version-scoped subscription localization by ID.",
		IDUsage:  "Subscription localization ID", IDType: "subscriptionLocalizations", RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Sparse fields for subscriptionLocalizations")
			versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
			include := fs.String("include", "", "Include relationships: version")
			return func() (err error) {
				if fieldValues, err = normalizeSelectionFlag(d, fs, *fields, "--fields", subscriptionVersionLocalizationFieldsList()); err != nil {
					return err
				}
				if versionFieldValues, err = normalizeSelectionFlag(d, fs, *versionFields, "--version-fields", subscriptionVersionFieldsList()); err != nil {
					return err
				}
				includeValues, err = normalizeSelectionFlag(d, fs, *include, "--include", subscriptionLocalizationV2IncludeList())
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionLocalizationV2(
				ctx, id,
				asc.WithSubscriptionLocalizationV2Fields(fieldValues),
				asc.WithSubscriptionLocalizationV2VersionFields(versionFieldValues),
				asc.WithSubscriptionLocalizationV2Include(includeValues),
			)
		},
	})
}

// newSubscriptionsVersionLocalizationsCreateCommand creates a v2 localization.
func newSubscriptionsVersionLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions localizations create", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	locale := fs.String("locale", "", "Localization locale")
	name := fs.String("name", "", "Localized display name")
	var description optionalString
	fs.Var(&description, "description", "Localized description (may be empty)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: "asc subscriptions versions localizations create [flags]", ShortHelp: "Create a version-scoped localization.",
		LongHelp: "Create a localization associated with a subscription version.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id, localeValue, nameValue := strings.TrimSpace(*versionID), strings.TrimSpace(*locale), strings.TrimSpace(*name)
			if id == "" {
				return d.RequiredFlag("--version-id")
			}
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}
			attrs := asc.SubscriptionLocalizationV2CreateAttributes{Name: nameValue, Locale: localeValue}
			if description.set {
				attrs.Description = &asc.NullableString{Value: &description.value}
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations create: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations create: failed to create: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionLocalizationsUpdateCommand updates a v2 localization.
func newSubscriptionsVersionLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions localizations update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "subscriptionLocalizations", "Subscription localization ID")
	var name, description optionalString
	fs.Var(&name, "name", "Localized display name (may be empty)")
	fs.Var(&description, "description", "Localized description (may be empty)")
	clearName := fs.Bool("clear-name", false, "Set the localized display name to null")
	clearDescription := fs.Bool("clear-description", false, "Set the localized description to null")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: "asc subscriptions versions localizations update [flags]", ShortHelp: "Update a version-scoped localization.",
		LongHelp: `Update a version-scoped subscription localization. Locale is immutable.

Examples:
  asc subscriptions versions localizations update --id "LOCALIZATION_ID" --name "Pro Plus"
  asc subscriptions versions localizations update --id "LOCALIZATION_ID" --clear-description`, FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localizationID := strings.TrimSpace(*id)
			if localizationID == "" {
				return d.RequiredFlag("--id")
			}
			if name.set && *clearName {
				return d.UsageError("--name cannot be used with --clear-name")
			}
			if description.set && *clearDescription {
				return d.UsageError("--description cannot be used with --clear-description")
			}
			if !name.set && !description.set && !*clearName && !*clearDescription {
				return d.UpdateFlagRequired(fs, &name, &description, clearName, clearDescription)
			}
			attrs := asc.SubscriptionLocalizationV2UpdateAttributes{}
			if name.set {
				attrs.Name = &asc.NullableString{Value: &name.value}
			} else if *clearName {
				attrs.Name = &asc.NullableString{Value: nil}
			}
			if description.set {
				attrs.Description = &asc.NullableString{Value: &description.value}
			} else if *clearDescription {
				attrs.Description = &asc.NullableString{Value: nil}
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations update: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.UpdateSubscriptionLocalizationV2(requestCtx, localizationID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations update: failed to update: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsVersionLocalizationsDeleteCommand deletes a v2 localization.
func newSubscriptionsVersionLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:               d,
		FlagSetName:        "versions localizations delete",
		Name:               "delete",
		ShortUsage:         "asc subscriptions versions localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:          "Delete a version-scoped localization.",
		LongHelp:           "Delete a version-scoped subscription localization.",
		IDFlag:             "id",
		IDUsage:            "Subscription localization ID",
		IDType:             "subscriptionLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}

// resolveNewestSubscriptionVersionID returns the subscription's highest
// numbered version. App Store Connect opens a new version only after the
// previous one is approved, so that version is the draft being edited when
// one exists and the live version otherwise.
func resolveNewestSubscriptionVersionID(ctx context.Context, d *shared.Deps, client *asc.Client, appValue, selector string) (string, error) {
	subscriptionID, err := resolveSubscriptionLookupIDWithTimeout(ctx, d, client, appValue, selector)
	if err != nil {
		return "", err
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()
	resp, err := client.GetSubscriptionVersions(requestCtx, subscriptionID, asc.Limit[asc.SubscriptionVersionsOption](200))
	if err != nil {
		return "", fmt.Errorf("failed to fetch versions: %w", err)
	}
	aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetSubscriptionVersions(pageCtx, subscriptionID, asc.WithSubscriptionVersionsNextURL(nextURL))
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch versions: %w", err)
	}
	newestID, newest := "", -1
	for _, version := range aggregated.(*asc.SubscriptionVersionsResponse).Data {
		if id := strings.TrimSpace(version.ID); id != "" && version.Attributes.Version > newest {
			newestID, newest = id, version.Attributes.Version
		}
	}
	if newestID == "" {
		return "", fmt.Errorf("subscription %s has no versions; create one with: asc subscriptions versions create --subscription-id %q", subscriptionID, subscriptionID)
	}
	return newestID, nil
}
