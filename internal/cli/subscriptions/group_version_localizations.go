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

// SubscriptionsGroupsVersionLocalizationsCommand manages version-scoped v2 localizations.
func SubscriptionsGroupsVersionLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "localizations", ShortUsage: "asc subscriptions groups versions localizations <subcommand> [flags]", ShortHelp: "Manage version-scoped subscription group localizations.",
		LongHelp: `Manage version-scoped subscription group localizations.

These commands use the v2 localization API. Existing group-scoped commands at
asc subscriptions groups localizations remain unchanged.`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsVersionLocalizationsListCommand(),
			SubscriptionsGroupsVersionLocalizationsCreateCommand(),
			SubscriptionsGroupsVersionLocalizationsViewCommand(),
			SubscriptionsGroupsVersionLocalizationsUpdateCommand(),
			SubscriptionsGroupsVersionLocalizationsDeleteCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func subscriptionGroupVersionLocalizationOptions(includeValue, fieldsValue, versionFieldsValue string, limit int, next string) ([]asc.SubscriptionGroupVersionLocalizationsOption, error) {
	if limit != 0 && (limit < 1 || limit > 200) {
		return nil, fmt.Errorf("--limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return nil, err
	}
	include, err := shared.NormalizeSelection(includeValue, []string{"version"}, "--include")
	if err != nil {
		return nil, err
	}
	fields, err := shared.NormalizeSelection(fieldsValue, subscriptionGroupVersionLocalizationFields, "--fields")
	if err != nil {
		return nil, err
	}
	versionFields, err := shared.NormalizeSelection(versionFieldsValue, subscriptionGroupVersionFields, "--version-fields")
	if err != nil {
		return nil, err
	}
	return []asc.SubscriptionGroupVersionLocalizationsOption{
		asc.WithSubscriptionGroupVersionLocalizationsInclude(include),
		asc.WithSubscriptionGroupVersionLocalizationsFields(fields),
		asc.WithSubscriptionGroupVersionLocalizationsVersionFields(versionFields),
		asc.WithSubscriptionGroupVersionLocalizationsLimit(limit),
		asc.WithSubscriptionGroupVersionLocalizationsNextURL(next),
	}, nil
}

// SubscriptionsGroupsVersionLocalizationsListCommand lists localizations owned by a version.
func SubscriptionsGroupsVersionLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations list", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription group version ID")
	include := fs.String("include", "", "Include relationship: version")
	fields := fs.String("fields", "", "Localization fields: name,customAppName,locale,version")
	versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc subscriptions groups versions localizations list --version-id "VERSION_ID" [flags]`, ShortHelp: "List localizations for a subscription group version.",
		LongHelp: "List localizations for a subscription group version.\n\nExamples:\n  asc subscriptions groups versions localizations list --version-id \"VERSION_ID\" --paginate", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			opts, err := subscriptionGroupVersionLocalizationOptions(*include, *fields, *versionFields, *limit, *next)
			if err != nil {
				return shared.UsageError("subscriptions groups versions localizations list: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionGroupVersionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionGroupVersionLocalizations(ctx, id, asc.WithSubscriptionGroupVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups versions localizations list: %w", err)
				}
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsVersionLocalizationsCreateCommand creates a v2 localization.
func SubscriptionsGroupsVersionLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "Subscription group version ID")
	name := fs.String("name", "", "Localized name")
	locale := fs.String("locale", "", "Locale (for example, en-US)")
	customAppName := fs.String("custom-app-name", "", "Custom app name")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc subscriptions groups versions localizations create --version-id "VERSION_ID" --name "Premium" --locale "en-US"`, ShortHelp: "Create a localization for a subscription group version.",
		LongHelp: "Create a localization for a subscription group version.\n\nExamples:\n  asc subscriptions groups versions localizations create --version-id \"VERSION_ID\" --name \"Premium\" --locale \"en-US\"", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionGroupLocalizationV2(requestCtx, vid, asc.SubscriptionGroupLocalizationV2CreateAttributes{
				Name: nameValue, Locale: localeValue, CustomAppName: strings.TrimSpace(*customAppName),
			})
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsVersionLocalizationsViewCommand retrieves a v2 localization.
func SubscriptionsGroupsVersionLocalizationsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations view", flag.ExitOnError)
	id := fs.String("id", "", "Subscription group localization ID")
	include := fs.String("include", "", "Include relationship: version")
	fields := fs.String("fields", "", "Localization fields: name,customAppName,locale,version")
	versionFields := fs.String("version-fields", "", "Included version fields (comma-separated)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc subscriptions groups versions localizations view --id "LOCALIZATION_ID"`, ShortHelp: "View a version-scoped subscription group localization.", LongHelp: "View a version-scoped subscription group localization.",
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			opts, err := subscriptionGroupVersionLocalizationOptions(*include, *fields, *versionFields, 0, "")
			if err != nil {
				return shared.UsageError("subscriptions groups versions localizations view: " + err.Error())
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetSubscriptionGroupLocalizationV2(requestCtx, value, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func subscriptionGroupFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// SubscriptionsGroupsVersionLocalizationsUpdateCommand updates nullable v2 attributes.
func SubscriptionsGroupsVersionLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations update", flag.ExitOnError)
	id := fs.String("id", "", "Subscription group localization ID")
	name := fs.String("name", "", "Localized name")
	customAppName := fs.String("custom-app-name", "", "Custom app name")
	clearName := fs.Bool("clear-name", false, "Set the localized name to null")
	clearCustomAppName := fs.Bool("clear-custom-app-name", false, "Set the custom app name to null")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: `asc subscriptions groups versions localizations update --id "LOCALIZATION_ID" [flags]`, ShortHelp: "Update a version-scoped subscription group localization.",
		LongHelp: "Update a version-scoped subscription group localization.\n\nExamples:\n  asc subscriptions groups versions localizations update --id \"LOCALIZATION_ID\" --name \"Premium Plus\"\n  asc subscriptions groups versions localizations update --id \"LOCALIZATION_ID\" --clear-custom-app-name", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			nameSet := subscriptionGroupFlagSet(fs, "name")
			customNameSet := subscriptionGroupFlagSet(fs, "custom-app-name")
			if nameSet && *clearName {
				fmt.Fprintln(os.Stderr, "Error: --name cannot be used with --clear-name")
				return shared.MissingRequiredUsageError()
			}
			if customNameSet && *clearCustomAppName {
				fmt.Fprintln(os.Stderr, "Error: --custom-app-name cannot be used with --clear-custom-app-name")
				return shared.MissingRequiredUsageError()
			}
			if !nameSet && !customNameSet && !*clearName && !*clearCustomAppName {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}
			attrs := asc.SubscriptionGroupLocalizationV2UpdateAttributes{}
			if nameSet {
				nameValue := strings.TrimSpace(*name)
				attrs.Name = &asc.NullableString{Value: &nameValue}
			} else if *clearName {
				attrs.Name = &asc.NullableString{Value: nil}
			}
			if customNameSet {
				customValue := strings.TrimSpace(*customAppName)
				attrs.CustomAppName = &asc.NullableString{Value: &customValue}
			} else if *clearCustomAppName {
				attrs.CustomAppName = &asc.NullableString{Value: nil}
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations update: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.UpdateSubscriptionGroupLocalizationV2(requestCtx, value, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGroupsVersionLocalizationsDeleteCommand deletes a v2 localization.
func SubscriptionsGroupsVersionLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups versions localizations delete", flag.ExitOnError)
	id := fs.String("id", "", "Subscription group localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortUsage: `asc subscriptions groups versions localizations delete --id "LOCALIZATION_ID" --confirm`, ShortHelp: "Delete a version-scoped subscription group localization.", LongHelp: "Delete a version-scoped subscription group localization.",
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups versions localizations delete: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if err := client.DeleteSubscriptionGroupLocalizationV2(requestCtx, value); err != nil {
				return fmt.Errorf("subscriptions groups versions localizations delete: failed to delete: %w", err)
			}
			return shared.PrintOutput(&asc.AssetDeleteResult{ID: value, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
