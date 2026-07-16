package iap

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

func IAPVersionLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "localizations", ShortUsage: "asc iap versions localizations <subcommand> [flags]", ShortHelp: "Manage version-scoped IAP localizations.", LongHelp: "Manage version-scoped IAP localizations.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{IAPVersionLocalizationsListCommand(), IAPVersionLocalizationsCreateCommand(), IAPVersionLocalizationsViewCommand(), IAPVersionLocalizationsUpdateCommand(), IAPVersionLocalizationsDeleteCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func IAPVersionLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations list", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	include := fs.String("include", "", "Include relationship: version")
	localizationFields := fs.String("localization-fields", "", "fields[inAppPurchaseLocalizations] (comma-separated)")
	versionFields := fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc iap versions localizations list --version-id "VERSION_ID" [flags]`, ShortHelp: "List localizations for an IAP version.", LongHelp: "List localizations for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			if flagSet(fs, "version-id") && strings.TrimSpace(*next) != "" {
				return shared.UsageError("iap versions localizations list: --next cannot be combined with --version-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap versions localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError("iap versions localizations list: " + err.Error())
			}
			if strings.TrimSpace(*next) != "" && (*limit != 0 || strings.TrimSpace(*include) != "" || strings.TrimSpace(*localizationFields) != "" || strings.TrimSpace(*versionFields) != "") {
				return shared.UsageError("iap versions localizations list: --next cannot be combined with --limit, --include, --localization-fields, or --version-fields")
			}
			includes, err := shared.NormalizeSelection(*include, []string{"version"}, "--include")
			if err != nil {
				return shared.UsageError("iap versions localizations list: " + err.Error())
			}
			localizationFieldValues, err := shared.NormalizeSelection(*localizationFields, iapVersionLocalizationFields, "--localization-fields")
			if err != nil {
				return shared.UsageError("iap versions localizations list: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, iapVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("iap versions localizations list: " + err.Error())
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			opts := []asc.IAPVersionLocalizationsOption{
				asc.WithIAPVersionLocalizationsLimit(*limit),
				asc.WithIAPVersionLocalizationsNextURL(*next),
				asc.WithIAPVersionLocalizationsInclude(includes),
				asc.WithIAPVersionLocalizationsFields(localizationFieldValues),
				asc.WithIAPVersionLocalizationsVersionFields(versionFieldValues),
			}
			resp, err := client.GetInAppPurchaseVersionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap versions localizations list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseVersionLocalizations(ctx, id, asc.WithIAPVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap versions localizations list: %w", err)
				}
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	name := fs.String("name", "", "Localization name")
	locale := fs.String("locale", "", "Locale (for example, en-US)")
	description := fs.String("description", "", "Description")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc iap versions localizations create --version-id "VERSION_ID" --name "Name" --locale "en-US"`, ShortHelp: "Create a localization for an IAP version.", LongHelp: "Create a localization for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
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
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.CreateInAppPurchaseLocalizationV2(requestCtx, vid, asc.InAppPurchaseLocalizationCreateAttributes{Name: nameValue, Locale: localeValue, Description: strings.TrimSpace(*description)})
			if err != nil {
				return fmt.Errorf("iap versions localizations create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionLocalizationsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations view", flag.ExitOnError)
	id := fs.String("localization-id", "", "Localization ID")
	include := fs.String("include", "", "Include relationship: version")
	localizationFields := fs.String("localization-fields", "", "fields[inAppPurchaseLocalizations] (comma-separated)")
	versionFields := fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc iap versions localizations view --localization-id "LOCALIZATION_ID"`, ShortHelp: "View a version-scoped IAP localization.", LongHelp: "View a version-scoped IAP localization.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			includes, err := shared.NormalizeSelection(*include, []string{"version"}, "--include")
			if err != nil {
				return shared.UsageError("iap versions localizations view: " + err.Error())
			}
			localizationFieldValues, err := shared.NormalizeSelection(*localizationFields, iapVersionLocalizationFields, "--localization-fields")
			if err != nil {
				return shared.UsageError("iap versions localizations view: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, iapVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("iap versions localizations view: " + err.Error())
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseLocalizationV2(
				requestCtx, value,
				asc.WithIAPLocalizationV2Include(includes),
				asc.WithIAPLocalizationV2Fields(localizationFieldValues),
				asc.WithIAPLocalizationV2VersionFields(versionFieldValues),
			)
			if err != nil {
				return fmt.Errorf("iap versions localizations view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func flagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func IAPVersionLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations update", flag.ExitOnError)
	id := fs.String("localization-id", "", "Localization ID")
	name := fs.String("name", "", "Localization name")
	clearName := fs.Bool("clear-name", false, "Clear the localization name")
	description := fs.String("description", "", "Description")
	clearDescription := fs.Bool("clear-description", false, "Clear the description")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: `asc iap versions localizations update --localization-id "LOCALIZATION_ID" [flags]`, ShortHelp: "Update a version-scoped IAP localization.", LongHelp: "Update a version-scoped IAP localization.\n\nExamples:\n  asc iap versions localizations update --localization-id \"LOCALIZATION_ID\" --name \"New Name\"\n  asc iap versions localizations update --localization-id \"LOCALIZATION_ID\" --clear-description", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			nameSet, descriptionSet := flagSet(fs, "name"), flagSet(fs, "description")
			if nameSet && *clearName {
				return shared.UsageError("iap versions localizations update: --name and --clear-name are mutually exclusive")
			}
			if descriptionSet && *clearDescription {
				return shared.UsageError("iap versions localizations update: --description and --clear-description are mutually exclusive")
			}
			if !nameSet && !descriptionSet && !*clearName && !*clearDescription {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}
			attrs := asc.InAppPurchaseLocalizationUpdateAttributes{}
			if nameSet {
				nameValue := strings.TrimSpace(*name)
				attrs.Name = &asc.NullableString{Value: &nameValue}
			} else if *clearName {
				attrs.Name = &asc.NullableString{}
			}
			if descriptionSet {
				descriptionValue := strings.TrimSpace(*description)
				attrs.Description = &asc.NullableString{Value: &descriptionValue}
			} else if *clearDescription {
				attrs.Description = &asc.NullableString{}
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations update: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.UpdateInAppPurchaseLocalizationV2(requestCtx, value, attrs)
			if err != nil {
				return fmt.Errorf("iap versions localizations update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations delete", flag.ExitOnError)
	id := fs.String("localization-id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortUsage: `asc iap versions localizations delete --localization-id "LOCALIZATION_ID" --confirm`, ShortHelp: "Delete a version-scoped IAP localization.", LongHelp: "Delete a version-scoped IAP localization.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations delete: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if err := client.DeleteInAppPurchaseLocalizationV2(requestCtx, value); err != nil {
				return fmt.Errorf("iap versions localizations delete: failed to delete: %w", err)
			}
			return shared.PrintOutput(&asc.AssetDeleteResult{ID: value, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
