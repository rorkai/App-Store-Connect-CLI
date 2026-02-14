package apps

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

// AppSetupCommand returns the app-setup command group.
func AppSetupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-setup", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-setup",
		ShortUsage: "asc app-setup <subcommand> [flags]",
		ShortHelp:  "Post-create app setup automation.",
		LongHelp: `Post-create app setup automation using public App Store Connect APIs.

Examples:
  asc app-setup info set --app "APP_ID" --primary-locale "en-US" --bundle-id "com.example.app"
  asc app-setup categories set --app "APP_ID" --primary GAMES
  asc app-setup availability set --app "APP_ID" --territory "USA,GBR" --available true
  asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID" --base-territory "USA"
  asc app-setup localizations upload --version "VERSION_ID" --path "./localizations"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupInfoCommand(),
			AppSetupCategoriesCommand(),
			AppSetupAvailabilityCommand(),
			AppSetupPricingCommand(),
			AppSetupLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupInfoCommand returns the info subcommand group.
func AppSetupInfoCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc app-setup info <subcommand> [flags]",
		ShortHelp:  "Update app info and app info localizations.",
		LongHelp: `Update app attributes and app info localizations.

Examples:
  asc app-setup info set --app "APP_ID" --primary-locale "en-US" --bundle-id "com.example.app"
  asc app-setup info set --app "APP_ID" --locale "en-US" --name "My App" --subtitle "Great app"
  asc app-setup info set --app "APP_ID" --primary-locale "en-US" --privacy-policy-url "https://example.com/privacy"
  asc app-setup info set --app "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupInfoSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupInfoSetCommand returns the info set subcommand.
func AppSetupInfoSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-setup info set", flag.ExitOnError)

	appID := fs.String("app", os.Getenv("ASC_APP_ID"), "App Store Connect app ID (required)")
	bundleID := fs.String("bundle-id", "", "Bundle ID to set")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	locale := fs.String("locale", "", "Locale for app info localization (defaults to --primary-locale)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	name := fs.String("name", "", "Localized app name")
	subtitle := fs.String("subtitle", "", "Localized app subtitle")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Localized privacy policy URL")
	privacyChoicesURL := fs.String("privacy-choices-url", "", "Localized privacy choices URL")
	privacyPolicyText := fs.String("privacy-policy-text", "", "Localized privacy policy text")
	contentRights := fs.String("content-rights", "", "Content rights declaration: DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc app-setup info set [flags]",
		ShortHelp:  "Set app attributes and app info localizations.",
		LongHelp: `Set app attributes (bundle ID, primary locale) and app info localizations.

Examples:
  asc app-setup info set --app "APP_ID" --primary-locale "en-US" --bundle-id "com.example.app"
  asc app-setup info set --app "APP_ID" --locale "en-US" --name "My App" --subtitle "Great app"
  asc app-setup info set --app "APP_ID" --primary-locale "en-US" --privacy-policy-url "https://example.com/privacy"
  asc app-setup info set --app "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appIDValue := strings.TrimSpace(*appID)
			if appIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return flag.ErrHelp
			}

			bundleIDValue := strings.TrimSpace(*bundleID)
			primaryLocaleValue := strings.TrimSpace(*primaryLocale)

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" && primaryLocaleValue != "" {
				localeValue = primaryLocaleValue
			}

			nameValue := strings.TrimSpace(*name)
			subtitleValue := strings.TrimSpace(*subtitle)
			privacyPolicyURLValue := strings.TrimSpace(*privacyPolicyURL)
			privacyChoicesURLValue := strings.TrimSpace(*privacyChoicesURL)
			privacyPolicyTextValue := strings.TrimSpace(*privacyPolicyText)
			contentRightsValue := strings.TrimSpace(*contentRights)

			hasAppUpdate := bundleIDValue != "" || primaryLocaleValue != "" || contentRightsValue != ""
			hasLocalization := nameValue != "" ||
				subtitleValue != "" ||
				privacyPolicyURLValue != "" ||
				privacyChoicesURLValue != "" ||
				privacyPolicyTextValue != ""

			if !hasAppUpdate && !hasLocalization {
				fmt.Fprintln(os.Stderr, "Error: provide at least one update flag")
				return flag.ErrHelp
			}
			if primaryLocaleValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(primaryLocaleValue); err != nil {
					return fmt.Errorf("app-setup info set: %w", err)
				}
			}
			if hasLocalization && localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required for app info localization updates")
				return flag.ErrHelp
			}
			if localeValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return fmt.Errorf("app-setup info set: %w", err)
				}
			}

			var normalizedContentRights *asc.ContentRightsDeclaration
			if contentRightsValue != "" {
				normalizedRights := asc.ContentRightsDeclaration(strings.ToUpper(contentRightsValue))
				switch normalizedRights {
				case asc.ContentRightsDeclarationDoesNotUseThirdPartyContent,
					asc.ContentRightsDeclarationUsesThirdPartyContent:
					normalizedContentRights = &normalizedRights
				default:
					fmt.Fprintf(os.Stderr, "Error: --content-rights must be %s or %s\n", asc.ContentRightsDeclarationDoesNotUseThirdPartyContent, asc.ContentRightsDeclarationUsesThirdPartyContent)
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-setup info set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var appResp *asc.AppResponse
			if hasAppUpdate {
				attrs := asc.AppUpdateAttributes{}
				if bundleIDValue != "" {
					attrs.BundleID = &bundleIDValue
				}
				if primaryLocaleValue != "" {
					attrs.PrimaryLocale = &primaryLocaleValue
				}
				if normalizedContentRights != nil {
					attrs.ContentRightsDeclaration = normalizedContentRights
				}
				appResp, err = client.UpdateApp(requestCtx, appIDValue, attrs)
				if err != nil {
					return fmt.Errorf("app-setup info set: %w", err)
				}
			}

			var appInfoResp *asc.AppInfoLocalizationResponse
			if hasLocalization {
				resolvedAppInfoID, err := shared.ResolveAppInfoID(requestCtx, client, appIDValue, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("app-setup info set: %w", err)
				}

				localizations, err := client.GetAppInfoLocalizations(
					requestCtx,
					resolvedAppInfoID,
					asc.WithAppInfoLocalizationsLimit(200),
					asc.WithAppInfoLocalizationLocales([]string{localeValue}),
				)
				if err != nil {
					return fmt.Errorf("app-setup info set: failed to fetch app info localizations: %w", err)
				}

				attrs := asc.AppInfoLocalizationAttributes{}
				if nameValue != "" {
					attrs.Name = nameValue
				}
				if subtitleValue != "" {
					attrs.Subtitle = subtitleValue
				}
				if privacyPolicyURLValue != "" {
					attrs.PrivacyPolicyURL = privacyPolicyURLValue
				}
				if privacyChoicesURLValue != "" {
					attrs.PrivacyChoicesURL = privacyChoicesURLValue
				}
				if privacyPolicyTextValue != "" {
					attrs.PrivacyPolicyText = privacyPolicyTextValue
				}

				if len(localizations.Data) == 0 {
					attrs.Locale = localeValue
					appInfoResp, err = client.CreateAppInfoLocalization(requestCtx, resolvedAppInfoID, attrs)
					if err != nil {
						return fmt.Errorf("app-setup info set: %w", err)
					}
				} else {
					localizationID := strings.TrimSpace(localizations.Data[0].ID)
					if localizationID == "" {
						return fmt.Errorf("app-setup info set: localization id is empty")
					}
					appInfoResp, err = client.UpdateAppInfoLocalization(requestCtx, localizationID, attrs)
					if err != nil {
						return fmt.Errorf("app-setup info set: %w", err)
					}
				}
			}

			result := &asc.AppSetupInfoResult{
				AppID:               appIDValue,
				App:                 appResp,
				AppInfoLocalization: appInfoResp,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppSetupCategoriesCommand returns the categories subcommand group.
func AppSetupCategoriesCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "categories",
		ShortUsage: "asc app-setup categories <subcommand> [flags]",
		ShortHelp:  "Set categories for an app.",
		LongHelp: `Set primary and secondary categories for an app.

Examples:
  asc app-setup categories set --app "APP_ID" --primary GAMES --secondary ENTERTAINMENT`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupCategoriesSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupCategoriesSetCommand returns the categories set subcommand.
func AppSetupCategoriesSetCommand() *ffcli.Command {
	return shared.NewCategoriesSetCommand(shared.CategoriesSetCommandConfig{
		FlagSetName: "app-setup categories set",
		ShortUsage:  "asc app-setup categories set --app APP_ID --primary CATEGORY_ID [--secondary CATEGORY_ID] [--app-info APP_INFO_ID]",
		ShortHelp:   "Set primary and secondary categories for an app.",
		LongHelp: `Set the primary and secondary categories for an app.

Use 'asc categories list' to find valid category IDs.

Examples:
  asc app-setup categories set --app 123456789 --primary GAMES
  asc app-setup categories set --app 123456789 --primary GAMES --secondary ENTERTAINMENT`,
		ErrorPrefix:    "app-setup categories set",
		IncludeAppInfo: true,
	})
}

// AppSetupAvailabilityCommand returns the availability subcommand group.
func AppSetupAvailabilityCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc app-setup availability <subcommand> [flags]",
		ShortHelp:  "Set app availability.",
		LongHelp: `Set app availability for territories.

Examples:
  asc app-setup availability set --app "APP_ID" --territory "USA,GBR" --available true`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupAvailabilitySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupAvailabilitySetCommand returns the availability set subcommand.
func AppSetupAvailabilitySetCommand() *ffcli.Command {
	return shared.NewAvailabilitySetCommand(shared.AvailabilitySetCommandConfig{
		FlagSetName: "app-setup availability set",
		CommandName: "set",
		ShortUsage:  "asc app-setup availability set [flags]",
		ShortHelp:   "Set app availability for territories.",
		LongHelp: `Set app availability for territories.

Examples:
  asc app-setup availability set --app "123456789" --territory "USA,GBR" --available true --available-in-new-territories true`,
		ErrorPrefix:                      "app-setup availability set",
		IncludeAvailableInNewTerritories: true,
	})
}

// AppSetupPricingCommand returns the pricing subcommand group.
func AppSetupPricingCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc app-setup pricing <subcommand> [flags]",
		ShortHelp:  "Set app pricing.",
		LongHelp: `Set app pricing using a price point.

Examples:
  asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupPricingSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupPricingSetCommand returns the pricing set subcommand.
func AppSetupPricingSetCommand() *ffcli.Command {
	return shared.NewPricingSetCommand(shared.PricingSetCommandConfig{
		FlagSetName: "app-setup pricing set",
		CommandName: "set",
		ShortUsage:  "asc app-setup pricing set [flags]",
		ShortHelp:   "Set app pricing using a price point.",
		LongHelp: `Set app pricing using a price point.

Examples:
  asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID" --base-territory "USA"
  asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID" --base-territory "USA" --start-date "2024-03-01"`,
		ErrorPrefix:           "app-setup pricing set",
		StartDateHelp:         "Start date (YYYY-MM-DD, default: today)",
		StartDateDefaultToday: true,
		ResolveBaseTerritory:  true,
	})
}

// AppSetupLocalizationsCommand returns the localizations subcommand group.
func AppSetupLocalizationsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-setup localizations <subcommand> [flags]",
		ShortHelp:  "Upload app store localizations.",
		LongHelp: `Upload app store localizations (version or app-info).

Examples:
  asc app-setup localizations upload --version "VERSION_ID" --path "./localizations"
  asc app-setup localizations upload --app "APP_ID" --type app-info --path "./localizations"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppSetupLocalizationsUploadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppSetupLocalizationsUploadCommand returns the localizations upload subcommand.
func AppSetupLocalizationsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-setup localizations upload", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	path := fs.String("path", "", "Input path (directory or .strings file)")
	dryRun := fs.Bool("dry-run", false, "Validate file without uploading")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc app-setup localizations upload [flags]",
		ShortHelp:  "Upload localizations from .strings files.",
		LongHelp: `Upload localizations from .strings files.

Examples:
  asc app-setup localizations upload --version "VERSION_ID" --path "./localizations"
  asc app-setup localizations upload --app "APP_ID" --type app-info --path "./localizations"
  asc app-setup localizations upload --version "VERSION_ID" --locale "en-US" --path "en-US.strings"
  asc app-setup localizations upload --version "VERSION_ID" --path "./localizations" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*path) == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}

			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("app-setup localizations upload: %w", err)
			}

			locales := shared.SplitCSV(*locale)

			switch normalizedType {
			case shared.LocalizationTypeVersion:
				if strings.TrimSpace(*versionID) == "" {
					fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				valuesByLocale, err := shared.ReadLocalizationStrings(*path, locales)
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				results, err := shared.UploadVersionLocalizations(requestCtx, client, strings.TrimSpace(*versionID), valuesByLocale, *dryRun)
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				result := asc.LocalizationUploadResult{
					Type:      normalizedType,
					VersionID: strings.TrimSpace(*versionID),
					DryRun:    *dryRun,
					Results:   results,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			case shared.LocalizationTypeAppInfo:
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				valuesByLocale, err := shared.ReadLocalizationStrings(*path, locales)
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				results, err := shared.UploadAppInfoLocalizations(requestCtx, client, appInfo, valuesByLocale, *dryRun)
				if err != nil {
					return fmt.Errorf("app-setup localizations upload: %w", err)
				}

				result := asc.LocalizationUploadResult{
					Type:      normalizedType,
					AppID:     resolvedAppID,
					AppInfoID: appInfo,
					DryRun:    *dryRun,
					Results:   results,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("app-setup localizations upload: unsupported type %q", normalizedType)
			}
		},
	}
}
