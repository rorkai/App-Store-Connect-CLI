package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func IAPVersionLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "localizations", ShortUsage: "asc iap versions localizations <subcommand> [flags]", ShortHelp: "Manage version-scoped IAP localizations.", LongHelp: "Manage version-scoped IAP localizations.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{IAPVersionLocalizationsListCommand(), IAPVersionLocalizationsCreateCommand(), IAPVersionLocalizationsViewCommand(), IAPVersionLocalizationsUpdateCommand(), IAPVersionLocalizationsDeleteCommand(), IAPVersionLocalizationsImportCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func IAPVersionLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions localizations list", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
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
			if err := shared.ValidatePaginationFlags(ctx, fs, "iap versions localizations list", 200, "limit", "include", "localization-fields", "version-fields"); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--version-id")
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
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			opts := []asc.IAPVersionLocalizationsOption{
				asc.Limit[asc.IAPVersionLocalizationsOption](firstPageLimit),
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
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	name := fs.String("name", "", "Localization name")
	locale := fs.String("locale", "", "Locale (for example, en-US)")
	description := fs.String("description", "", "Description")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc iap versions localizations create --version-id "VERSION_ID" --name "Name" --locale "en-US"`, ShortHelp: "Create a localization for an IAP version.", LongHelp: "Create a localization for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				return shared.RequiredFlag("--version-id")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return shared.RequiredFlag("--locale")
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions localizations create: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			attrs := asc.InAppPurchaseLocalizationV2CreateAttributes{Name: nameValue, Locale: localeValue}
			if descriptionValue := strings.TrimSpace(*description); descriptionValue != "" {
				attrs.Description = &asc.NullableString{Value: &descriptionValue}
			}
			resp, err := client.CreateInAppPurchaseLocalizationV2(requestCtx, vid, attrs)
			if err != nil {
				return fmt.Errorf("iap versions localizations create: failed to create: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionLocalizationsViewCommand() *ffcli.Command {
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return iapVersionClientFactory() }

	var includes, localizationFieldValues, versionFieldValues []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName:        "versions localizations view",
		Name:               "view",
		ShortUsage:         `asc iap versions localizations view --localization-id "LOCALIZATION_ID"`,
		ShortHelp:          "View a version-scoped IAP localization.",
		LongHelp:           "View a version-scoped IAP localization.",
		IDFlag:             "localization-id",
		IDUsage:            "Localization ID",
		IDType:             "inAppPurchaseLocalizations",
		RequiredDiagnostic: true,
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include relationship: version")
			localizationFields := fs.String("localization-fields", "", "fields[inAppPurchaseLocalizations] (comma-separated)")
			versionFields := fs.String("version-fields", "", "fields[inAppPurchaseVersions] (comma-separated)")
			return func() (err error) {
				includes, err = shared.NormalizeSelection(*include, []string{"version"}, "--include")
				if err != nil {
					return shared.UsageError("iap versions localizations view: " + err.Error())
				}
				localizationFieldValues, err = shared.NormalizeSelection(*localizationFields, iapVersionLocalizationFields, "--localization-fields")
				if err != nil {
					return shared.UsageError("iap versions localizations view: " + err.Error())
				}
				versionFieldValues, err = shared.NormalizeSelection(*versionFields, iapVersionFields, "--version-fields")
				if err != nil {
					return shared.UsageError("iap versions localizations view: " + err.Error())
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetInAppPurchaseLocalizationV2(
				ctx, id,
				asc.WithIAPLocalizationV2Include(includes),
				asc.WithIAPLocalizationV2Fields(localizationFieldValues),
				asc.WithIAPLocalizationV2VersionFields(versionFieldValues),
			)
		},
	})
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
	id := shared.BindResourceIDFlag(fs, "localization-id", "inAppPurchaseLocalizations", "Localization ID")
	name := fs.String("name", "", "Localization name")
	clearName := fs.Bool("clear-name", false, "Clear the localization name")
	description := fs.String("description", "", "Description")
	clearDescription := fs.Bool("clear-description", false, "Clear the description")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: `asc iap versions localizations update --localization-id "LOCALIZATION_ID" [flags]`, ShortHelp: "Update a version-scoped IAP localization.", LongHelp: "Update a version-scoped IAP localization.\n\nExamples:\n  asc iap versions localizations update --localization-id \"LOCALIZATION_ID\" --name \"New Name\"\n  asc iap versions localizations update --localization-id \"LOCALIZATION_ID\" --clear-description", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				return shared.RequiredFlag("--localization-id")
			}
			nameSet, descriptionSet := flagSet(fs, "name"), flagSet(fs, "description")
			if nameSet && *clearName {
				return shared.UsageError("iap versions localizations update: --name and --clear-name are mutually exclusive")
			}
			if descriptionSet && *clearDescription {
				return shared.UsageError("iap versions localizations update: --description and --clear-description are mutually exclusive")
			}
			if !nameSet && !descriptionSet && !*clearName && !*clearDescription {
				return shared.UpdateFlagRequired(fs, name, clearName, description, clearDescription)
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
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return iapVersionClientFactory() }

	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName:        "versions localizations delete",
		Name:               "delete",
		ShortUsage:         `asc iap versions localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		ShortHelp:          "Delete a version-scoped IAP localization.",
		LongHelp:           "Delete a version-scoped IAP localization.",
		IDFlag:             "localization-id",
		IDUsage:            "Localization ID",
		IDType:             "inAppPurchaseLocalizations",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteInAppPurchaseLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
