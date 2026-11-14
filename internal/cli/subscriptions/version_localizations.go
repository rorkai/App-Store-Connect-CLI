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

// SubscriptionsVersionLocalizationsCommand returns version localization commands.
func SubscriptionsVersionLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "localizations", ShortUsage: "asc subscriptions versions localizations <subcommand> [flags]",
		ShortHelp: "Manage version-scoped subscription localizations.",
		LongHelp: `Manage version-scoped subscription localizations.

Examples:
  asc subscriptions versions localizations list --version-id "VERSION_ID"
  asc subscriptions versions localizations create --version-id "VERSION_ID" --locale "en-US" --name "Premium"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsVersionLocalizationsListCommand(),
			SubscriptionsVersionLocalizationsLinksCommand(),
			SubscriptionsVersionLocalizationsViewCommand(),
			SubscriptionsVersionLocalizationsCreateCommand(),
			SubscriptionsVersionLocalizationsUpdateCommand(),
			SubscriptionsVersionLocalizationsDeleteCommand(),
			SubscriptionsVersionLocalizationsImportCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SubscriptionsVersionLocalizationsListCommand lists related localizations.
func SubscriptionsVersionLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations list", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionLocalizations")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	include := fs.String("include", "", "Include relationships: version")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: "asc subscriptions versions localizations list [flags]", ShortHelp: "List localizations for a subscription version.",
		LongHelp: "List version-scoped subscription localizations.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--version-id", flagWasProvided(fs, "version-id")},
				flagConflict{"--fields", flagWasProvided(fs, "fields")},
				flagConflict{"--version-fields", flagWasProvided(fs, "version-fields")},
				flagConflict{"--include", flagWasProvided(fs, "include")},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validatePageLimit(*limit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions localizations list: %v", err)
			}
			fieldValues, err := normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionLocalizationFieldsList())
			if err != nil {
				return err
			}
			versionFieldValues, err := normalizeSelectionFlag(fs, *versionFields, "--version-fields", subscriptionVersionFieldsList())
			if err != nil {
				return err
			}
			includeValues, err := normalizeSelectionFlag(fs, *include, "--include", subscriptionLocalizationV2IncludeList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionLocalizationsLinksCommand lists raw localization linkages.
func SubscriptionsVersionLocalizationsLinksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations links", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions localizations links [flags]", ShortHelp: "List raw localization linkages.",
		LongHelp: "List raw localization relationship linkages for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateNextFlagConflicts(
				*next,
				flagConflict{"--version-id", flagWasProvided(fs, "version-id")},
				flagConflict{"--limit", flagWasProvided(fs, "limit")},
			); err != nil {
				return err
			}
			if err := validatePageLimit(*limit); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("subscriptions versions localizations links: %v", err)
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations links: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionLocalizationsViewCommand views a v2 localization.
func SubscriptionsVersionLocalizationsViewCommand() *ffcli.Command {
	var fieldValues, versionFieldValues, includeValues []string
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "versions localizations view",
		Name:        "view", ShortUsage: "asc subscriptions versions localizations view --id \"LOCALIZATION_ID\"", ShortHelp: "View a version-scoped localization.",
		LongHelp: "View a version-scoped subscription localization by ID.",
		IDUsage:  "Subscription localization ID", IDType: "subscriptionLocalizations", RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Sparse fields for subscriptionLocalizations")
			versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
			include := fs.String("include", "", "Include relationships: version")
			return func() (err error) {
				if fieldValues, err = normalizeSelectionFlag(fs, *fields, "--fields", subscriptionVersionLocalizationFieldsList()); err != nil {
					return err
				}
				if versionFieldValues, err = normalizeSelectionFlag(fs, *versionFields, "--version-fields", subscriptionVersionFieldsList()); err != nil {
					return err
				}
				includeValues, err = normalizeSelectionFlag(fs, *include, "--include", subscriptionLocalizationV2IncludeList())
				return err
			}
		},
		ErrorPrefix:      "subscriptions versions localizations view",
		FetchErrorPrefix: "subscriptions versions localizations view: failed to fetch",
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

// SubscriptionsVersionLocalizationsCreateCommand creates a v2 localization.
func SubscriptionsVersionLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations create", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "subscriptionVersions", "Subscription version ID")
	locale := fs.String("locale", "", "Localization locale")
	name := fs.String("name", "", "Localized display name")
	var description optionalString
	fs.Var(&description, "description", "Localized description (may be empty)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: "asc subscriptions versions localizations create [flags]", ShortHelp: "Create a version-scoped localization.",
		LongHelp: "Create a localization associated with a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id, localeValue, nameValue := strings.TrimSpace(*versionID), strings.TrimSpace(*locale), strings.TrimSpace(*name)
			if id == "" {
				return shared.RequiredFlag("--version-id")
			}
			if localeValue == "" {
				return shared.RequiredFlag("--locale")
			}
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}
			attrs := asc.SubscriptionLocalizationV2CreateAttributes{Name: nameValue, Locale: localeValue}
			if description.set {
				attrs.Description = &asc.NullableString{Value: &description.value}
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionLocalizationsUpdateCommand updates a v2 localization.
func SubscriptionsVersionLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "subscriptionLocalizations", "Subscription localization ID")
	var name, description optionalString
	fs.Var(&name, "name", "Localized display name (may be empty)")
	fs.Var(&description, "description", "Localized description (may be empty)")
	clearName := fs.Bool("clear-name", false, "Set the localized display name to null")
	clearDescription := fs.Bool("clear-description", false, "Set the localized description to null")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: "asc subscriptions versions localizations update [flags]", ShortHelp: "Update a version-scoped localization.",
		LongHelp: `Update a version-scoped subscription localization. Locale is immutable.

Examples:
  asc subscriptions versions localizations update --id "LOCALIZATION_ID" --name "Pro Plus"
  asc subscriptions versions localizations update --id "LOCALIZATION_ID" --clear-description`, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localizationID := strings.TrimSpace(*id)
			if localizationID == "" {
				return shared.RequiredFlag("--id")
			}
			if name.set && *clearName {
				return shared.UsageError("--name cannot be used with --clear-name")
			}
			if description.set && *clearDescription {
				return shared.UsageError("--description cannot be used with --clear-description")
			}
			if !name.set && !description.set && !*clearName && !*clearDescription {
				return shared.UsageError("at least one of --name, --description, --clear-name, or --clear-description is required")
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
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations update: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.UpdateSubscriptionLocalizationV2(requestCtx, localizationID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionLocalizationsDeleteCommand deletes a v2 localization.
func SubscriptionsVersionLocalizationsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName:        "versions localizations delete",
		Name:               "delete",
		ShortUsage:         "asc subscriptions versions localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:          "Delete a version-scoped localization.",
		LongHelp:           "Delete a version-scoped subscription localization.",
		IDFlag:             "id",
		IDUsage:            "Subscription localization ID",
		IDType:             "subscriptionLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "subscriptions versions localizations delete",
		DeleteErrorPrefix:  "subscriptions versions localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
