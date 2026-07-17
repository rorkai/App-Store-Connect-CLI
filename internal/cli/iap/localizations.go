package iap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var errIAPLocalizationFound = errors.New("iap localization found")

// IAPLocalizationsCreateCommand returns the localizations create subcommand.
func IAPLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := fs.String("iap-id", "", "In-app purchase ID, product ID, or exact current name")
	name := fs.String("name", "", "Localization name")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	description := fs.String("description", "", "Description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap localizations create --iap-id \"IAP_ID\" --name \"Name\" --locale \"en-US\"",
		ShortHelp:  "Create an in-app purchase localization.",
		LongHelp: `Create an in-app purchase localization.

Examples:
  asc iap localizations create --iap-id "IAP_ID" --name "Title" --locale "en-US"
  asc iap localizations create --iap-id "IAP_ID" --name "Titre" --locale "fr-FR" --description "Detail"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
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
				return fmt.Errorf("iap localizations create: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.InAppPurchaseLocalizationCreateAttributes{
				Name:        nameValue,
				Locale:      localeValue,
				Description: strings.TrimSpace(*description),
			}

			existing, found, err := findIAPLocalizationByLocale(requestCtx, client, iapValue, localeValue)
			if err != nil {
				return fmt.Errorf("iap localizations create: failed to check existing localizations: %w", err)
			}
			if found {
				if iapLocalizationMatchesCreateAttributes(existing, attrs) {
					resp := &asc.InAppPurchaseLocalizationResponse{Data: existing}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}
				return shared.UsageError(fmt.Sprintf(
					"localization for locale %q already exists as %s; use iap localizations update --localization-id %s to change it",
					localeValue,
					strings.TrimSpace(existing.ID),
					strings.TrimSpace(existing.ID),
				))
			}

			resp, err := client.CreateInAppPurchaseLocalization(requestCtx, iapValue, attrs)
			if err != nil {
				return fmt.Errorf("iap localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func findIAPLocalizationByLocale(ctx context.Context, client *asc.Client, iapID, locale string) (asc.Resource[asc.InAppPurchaseLocalizationAttributes], bool, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return asc.Resource[asc.InAppPurchaseLocalizationAttributes]{}, false, nil
	}
	firstPage, err := client.GetInAppPurchaseLocalizations(ctx, iapID, asc.WithIAPLocalizationsLimit(200))
	if err != nil {
		return asc.Resource[asc.InAppPurchaseLocalizationAttributes]{}, false, err
	}
	if firstPage == nil {
		return asc.Resource[asc.InAppPurchaseLocalizationAttributes]{}, false, nil
	}

	var found asc.Resource[asc.InAppPurchaseLocalizationAttributes]
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetInAppPurchaseLocalizations(ctx, iapID, asc.WithIAPLocalizationsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.InAppPurchaseLocalizationsResponse)
			if !ok {
				return fmt.Errorf("unexpected in-app purchase localizations pagination type %T", page)
			}
			for _, localization := range resp.Data {
				if !strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), locale) {
					continue
				}
				found = localization
				return errIAPLocalizationFound
			}
			return nil
		},
	); err != nil && !errors.Is(err, errIAPLocalizationFound) {
		return asc.Resource[asc.InAppPurchaseLocalizationAttributes]{}, false, err
	}

	return found, strings.TrimSpace(found.ID) != "", nil
}

func iapLocalizationMatchesCreateAttributes(localization asc.Resource[asc.InAppPurchaseLocalizationAttributes], attrs asc.InAppPurchaseLocalizationCreateAttributes) bool {
	return strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), strings.TrimSpace(attrs.Locale)) &&
		strings.TrimSpace(localization.Attributes.Name) == strings.TrimSpace(attrs.Name) &&
		strings.TrimSpace(localization.Attributes.Description) == strings.TrimSpace(attrs.Description)
}

// IAPLocalizationsUpdateCommand returns the localizations update subcommand.
func IAPLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	legacyID := shared.BindDeprecatedStringFlagAlias(fs, "id", "localization-id")
	name := fs.String("name", "", "Localization name")
	clearName := fs.Bool("clear-name", false, "Clear the localization name")
	description := fs.String("description", "", "Description")
	clearDescription := fs.Bool("clear-description", false, "Clear the description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update an in-app purchase localization.",
		LongHelp: `Update an in-app purchase localization.

Examples:
  asc iap localizations update --localization-id "LOC_ID" --name "New Name"
  asc iap localizations update --localization-id "LOC_ID" --description "New Description"
  asc iap localizations update --localization-id "LOC_ID" --clear-description`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			if err := legacyID.Apply(localizationID); err != nil {
				return err
			}
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			nameSet := flagSet(fs, "name")
			descriptionSet := flagSet(fs, "description")
			if nameSet && *clearName {
				return shared.UsageError("iap localizations update: --name and --clear-name are mutually exclusive")
			}
			if descriptionSet && *clearDescription {
				return shared.UsageError("iap localizations update: --description and --clear-description are mutually exclusive")
			}
			if !nameSet && !descriptionSet && !*clearName && !*clearDescription {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := iapQueryClientFactory()
			if err != nil {
				return fmt.Errorf("iap localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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

			resp, err := client.UpdateInAppPurchaseLocalization(requestCtx, locValue, attrs)
			if err != nil {
				return fmt.Errorf("iap localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPLocalizationsDeleteCommand returns the localizations delete subcommand.
func IAPLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc iap localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete an in-app purchase localization.",
		LongHelp: `Delete an in-app purchase localization.

Examples:
  asc iap localizations delete --localization-id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteInAppPurchaseLocalization(requestCtx, locValue); err != nil {
				return fmt.Errorf("iap localizations delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{
				ID:      locValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
