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
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SubscriptionsVersionLocalizationsListCommand lists related localizations.
func SubscriptionsVersionLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations list", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
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
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
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
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionLocalizations(
				requestCtx, id,
				asc.WithSubscriptionVersionLocalizationsLimit(*limit),
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
	versionID := fs.String("version-id", "", "Subscription version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "links", ShortUsage: "asc subscriptions versions localizations links [flags]", ShortHelp: "List raw localization linkages.",
		LongHelp: "List raw localization relationship linkages for a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
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
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations links: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionVersionLocalizationsRelationships(requestCtx, id, asc.WithLinkagesLimit(*limit), asc.WithLinkagesNextURL(*next))
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations links: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionVersionLocalizationsRelationships(pageCtx, id, asc.WithLinkagesNextURL(nextURL))
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
	fs := flag.NewFlagSet("versions localizations view", flag.ExitOnError)
	id := fs.String("id", "", "Subscription localization ID")
	fields := fs.String("fields", "", "Sparse fields for subscriptionLocalizations")
	versionFields := fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
	include := fs.String("include", "", "Include relationships: version")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: "asc subscriptions versions localizations view --id \"LOCALIZATION_ID\"", ShortHelp: "View a version-scoped localization.",
		LongHelp: "View a version-scoped subscription localization by ID.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			localizationID := strings.TrimSpace(*id)
			if localizationID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
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
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionLocalizationV2(
				requestCtx, localizationID,
				asc.WithSubscriptionLocalizationV2Fields(fieldValues),
				asc.WithSubscriptionLocalizationV2VersionFields(versionFieldValues),
				asc.WithSubscriptionLocalizationV2Include(includeValues),
			)
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsVersionLocalizationsCreateCommand creates a v2 localization.
func SubscriptionsVersionLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription version ID")
	locale := fs.String("locale", "", "Localization locale")
	name := fs.String("name", "", "Localized display name")
	var description optionalString
	fs.Var(&description, "description", "Localized description (may be empty)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: "asc subscriptions versions localizations create [flags]", ShortHelp: "Create a version-scoped localization.",
		LongHelp: "Create a localization associated with a subscription version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			id, localeValue, nameValue := strings.TrimSpace(*versionID), strings.TrimSpace(*locale), strings.TrimSpace(*name)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError("--locale")
			}
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}
			attrs := asc.SubscriptionLocalizationV2CreateAttributes{Name: nameValue, Locale: localeValue}
			if description.set {
				attrs.Description = &description.value
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
	id := fs.String("id", "", "Subscription localization ID")
	var name, description optionalString
	fs.Var(&name, "name", "Localized display name (may be empty)")
	fs.Var(&description, "description", "Localized description (may be empty)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: "asc subscriptions versions localizations update [flags]", ShortHelp: "Update a version-scoped localization.",
		LongHelp: "Update a version-scoped subscription localization. Locale is immutable.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			localizationID := strings.TrimSpace(*id)
			if localizationID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !name.set && !description.set {
				return shared.UsageError("at least one of --name or --description is required")
			}
			attrs := asc.SubscriptionLocalizationV2UpdateAttributes{}
			if name.set {
				attrs.Name = &name.value
			}
			if description.set {
				attrs.Description = &description.value
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
	fs := flag.NewFlagSet("versions localizations delete", flag.ExitOnError)
	id := fs.String("id", "", "Subscription localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortUsage: "asc subscriptions versions localizations delete --id \"LOCALIZATION_ID\" --confirm", ShortHelp: "Delete a version-scoped localization.",
		LongHelp: "Delete a version-scoped subscription localization.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			localizationID := strings.TrimSpace(*id)
			if localizationID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions versions localizations delete: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if err := client.DeleteSubscriptionLocalizationV2(requestCtx, localizationID); err != nil {
				return fmt.Errorf("subscriptions versions localizations delete: failed to delete: %w", err)
			}
			return shared.PrintOutput(&asc.AssetDeleteResult{ID: localizationID, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
