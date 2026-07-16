package subscriptions

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var errSubscriptionLocalizationFound = errors.New("subscription localization found")

// SubscriptionsLocalizationsCommand returns the subscription localizations command group.
func SubscriptionsLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc subscriptions localizations <subcommand> [flags]",
		ShortHelp:  "Manage subscription localizations.",
		LongHelp: `Manage subscription localizations.

Examples:
  asc subscriptions localizations list --subscription-id "SUB_ID"
  asc subscriptions localizations create --subscription-id "SUB_ID" --locale "en-US" --name "Pro"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsLocalizationsListCommand(),
			SubscriptionsLocalizationsGetCommand(),
			SubscriptionsLocalizationsCreateCommand(),
			SubscriptionsLocalizationsUpdateCommand(),
			SubscriptionsLocalizationsDeleteCommand(),
			SubscriptionsLocalizationsSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsLocalizationsListCommand returns the localizations list subcommand.
func SubscriptionsLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions localizations list [flags]",
		ShortHelp:  "List subscription localizations.",
		LongHelp: `List subscription localizations.

Examples:
  asc subscriptions localizations list --subscription-id "SUB_ID"
  asc subscriptions localizations list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions localizations list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionLocalizationsOption{
				asc.WithSubscriptionLocalizationsLimit(*limit),
				asc.WithSubscriptionLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionLocalizationsLimit(200))
				firstPage, err := client.GetSubscriptionLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionLocalizations(ctx, id, asc.WithSubscriptionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsLocalizationsGetCommand returns the localizations get subcommand.
func SubscriptionsLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations view", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions localizations view --id \"LOC_ID\"",
		ShortHelp:  "View a subscription localization by ID.",
		LongHelp: `View a subscription localization by ID.

Examples:
  asc subscriptions localizations view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions localizations view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsLocalizationsCreateCommand returns the localizations create subcommand.
func SubscriptionsLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions localizations create [flags]",
		ShortHelp:  "Create a subscription localization.",
		LongHelp: `Create a subscription localization.

Examples:
  asc subscriptions localizations create --subscription-id "SUB_ID" --locale "en-US" --name "Pro"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError()
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations create: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionLocalizationCreateAttributes{
				Name:   nameValue,
				Locale: localeValue,
			}
			if desc := strings.TrimSpace(*description); desc != "" {
				attrs.Description = desc
			}

			existing, found, err := findSubscriptionLocalizationByLocale(requestCtx, client, id, localeValue)
			if err != nil {
				return fmt.Errorf("subscriptions localizations create: failed to check existing localizations: %w", err)
			}
			if found {
				if subscriptionLocalizationMatchesCreateAttributes(existing, attrs) {
					resp := &asc.SubscriptionLocalizationResponse{Data: existing}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}
				message := fmt.Sprintf(
					"localization for locale %q already exists as %s; use subscriptions localizations update --id %s to change it",
					localeValue,
					strings.TrimSpace(existing.ID),
					strings.TrimSpace(existing.ID),
				)
				return fmt.Errorf("subscriptions localizations create: %w", &asc.APIError{
					Code:       "CONFLICT",
					Title:      "Conflict",
					Detail:     message,
					StatusCode: http.StatusConflict,
				})
			}

			resp, err := client.CreateSubscriptionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func findSubscriptionLocalizationByLocale(ctx context.Context, client *asc.Client, subscriptionID, locale string) (asc.Resource[asc.SubscriptionLocalizationAttributes], bool, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, nil
	}
	firstPage, err := client.GetSubscriptionLocalizations(ctx, subscriptionID, asc.WithSubscriptionLocalizationsLimit(200))
	if err != nil {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, err
	}
	if firstPage == nil {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, nil
	}

	var found asc.Resource[asc.SubscriptionLocalizationAttributes]
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionLocalizations(ctx, subscriptionID, asc.WithSubscriptionLocalizationsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionLocalizationsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription localizations pagination type %T", page)
			}
			for _, localization := range resp.Data {
				if !strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), locale) {
					continue
				}
				found = localization
				return errSubscriptionLocalizationFound
			}
			return nil
		},
	); err != nil && !errors.Is(err, errSubscriptionLocalizationFound) {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, err
	}

	return found, strings.TrimSpace(found.ID) != "", nil
}

func subscriptionLocalizationMatchesCreateAttributes(localization asc.Resource[asc.SubscriptionLocalizationAttributes], attrs asc.SubscriptionLocalizationCreateAttributes) bool {
	return strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), strings.TrimSpace(attrs.Locale)) &&
		strings.TrimSpace(localization.Attributes.Name) == strings.TrimSpace(attrs.Name) &&
		strings.TrimSpace(localization.Attributes.Description) == strings.TrimSpace(attrs.Description)
}

// SubscriptionsLocalizationsUpdateCommand returns the localizations update subcommand.
func SubscriptionsLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions localizations update [flags]",
		ShortHelp:  "Update a subscription localization.",
		LongHelp: `Update a subscription localization.

Examples:
  asc subscriptions localizations update --id "LOC_ID" --name "Pro+"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			nameValue := strings.TrimSpace(*name)
			descriptionValue := strings.TrimSpace(*description)
			if nameValue == "" && descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionLocalizationUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if descriptionValue != "" {
				attrs.Description = &descriptionValue
			}

			resp, err := client.UpdateSubscriptionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsLocalizationsDeleteCommand returns the localizations delete subcommand.
func SubscriptionsLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a subscription localization.",
		LongHelp: `Delete a subscription localization.

Examples:
  asc subscriptions localizations delete --id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
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
				return fmt.Errorf("subscriptions localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions localizations delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{ID: id, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
