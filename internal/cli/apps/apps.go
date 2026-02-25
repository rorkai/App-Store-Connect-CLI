package apps

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/iris"
)

func appsListFlags(fs *flag.FlagSet) (output shared.OutputFlags, bundleID *string, name *string, sku *string, sort *string, limit *int, next *string, paginate *bool) {
	output = shared.BindOutputFlags(fs)
	bundleID = fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	name = fs.String("name", "", "Filter by app name(s), comma-separated")
	sku = fs.String("sku", "", "Filter by SKU(s), comma-separated")
	sort = fs.String("sort", "", "Sort by name, -name, bundleId, or -bundleId")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	return
}

// AppsCommand returns the apps command factory.
func AppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps", flag.ExitOnError)

	output, bundleID, name, sku, sort, limit, next, paginate := appsListFlags(fs)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc apps <subcommand> [flags]",
		ShortHelp:  "List and manage apps in App Store Connect.",
		LongHelp: `List and manage apps in App Store Connect.

Examples:
  asc apps
  asc apps list --bundle-id "com.example.app"
  asc apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123"
  asc apps wall
  asc apps get --id "APP_ID"
  asc apps ci-product get --id "APP_ID"
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps subscription-grace-period get --app "APP_ID"
  asc apps --limit 10
  asc apps --sort name
  asc apps --output table
  asc apps --next "<links.next>"
  asc apps --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsListCommand(),
			AppsCreateCommand(),
			AppsWallCommand(),
			AppsGetCommand(),
			AppsCIProductCommand(),
			AppsUpdateCommand(),
			AppsRemoveBetaTestersCommand(),
			AppsSubscriptionGracePeriodCommand(),
			AppsSearchKeywordsCommand(),
			AppEncryptionDeclarationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n", strings.TrimSpace(args[0]))
				return flag.ErrHelp
			}
			return appsList(ctx, *output.Output, *output.Pretty, *bundleID, *name, *sku, *sort, *limit, *next, *paginate)
		},
	}
}

// AppsListCommand returns the apps list subcommand.
func AppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps list", flag.ExitOnError)

	output, bundleID, name, sku, sort, limit, next, paginate := appsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps list [flags]",
		ShortHelp:  "List apps from App Store Connect.",
		LongHelp: `List apps from App Store Connect.

Examples:
  asc apps list
  asc apps list --bundle-id "com.example.app"
  asc apps list --name "My App"
  asc apps list --limit 10
  asc apps list --sort name
  asc apps list --output table
  asc apps list --next "<links.next>"
  asc apps list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return appsList(ctx, *output.Output, *output.Pretty, *bundleID, *name, *sku, *sort, *limit, *next, *paginate)
		},
	}
}

// AppsGetCommand returns the apps get subcommand.
func AppsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps get", flag.ExitOnError)

	id := fs.String("id", "", "App Store Connect app ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc apps get --id APP_ID",
		ShortHelp:  "Get app details by ID.",
		LongHelp: `Get app details by ID.

Examples:
  asc apps get --id "APP_ID"
  asc apps get --id "APP_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			app, err := client.GetApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("apps get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(app, *output.Output, *output.Pretty)
		},
	}
}

// AppsCreateCommand returns the apps create subcommand.
func AppsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps create", flag.ExitOnError)

	name := fs.String("name", "", "App name")
	bundleID := fs.String("bundle-id", "", "Bundle ID (e.g., com.example.app)")
	sku := fs.String("sku", "", "Unique SKU for the app")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, UNIVERSAL")
	appleID := fs.String("apple-id", "", "Apple ID (email) for authentication")
	password := fs.String("password", "", "Apple ID password (will prompt if not provided)")
	twoFactorCode := fs.String("two-factor-code", "", "2FA verification code (if prompted)")
	autoRename := fs.Bool("auto-rename", true, "Auto-retry with a unique app name when the chosen name is already in use (default: true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc apps create [flags]",
		ShortHelp:  "Create a new app in App Store Connect.",
		LongHelp: `Create a new app in App Store Connect.

NOTE: App creation requires Apple ID authentication (not API key).
If 2FA is enabled on your account, you may need to complete authentication in a browser first.

If flags are not provided, an interactive prompt will guide you through the required fields.
The bundle ID must be a valid identifier that matches an App ID in your developer account.
You can create bundle IDs using: asc bundle-ids create

Examples:
  asc apps create
  asc apps create --name "My App" --bundle-id "com.example.myapp" --sku "MYAPP123"
  asc apps create --name "My App" --bundle-id "com.example.myapp" --sku "MYAPP123" --primary-locale "en-GB" --platform IOS
  asc apps create --apple-id "user@example.com" --password`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			var nameValue, bundleIDValue, skuValue, localeValue, platformValue string
			var appleIDValue, passwordValue string
			var session *iris.AuthSession

			// Best-effort: reuse a cached web session to avoid repeated 2FA prompts.
			// We only do this when the user didn't explicitly provide a password.
			if strings.TrimSpace(*password) == "" {
				if strings.TrimSpace(*appleID) != "" {
					if resumed, ok, err := iris.TryResumeSession(strings.TrimSpace(*appleID)); err == nil && ok {
						session = resumed
						appleIDValue = resumed.UserEmail
					}
				} else {
					if resumed, ok, err := iris.TryResumeLastSession(); err == nil && ok {
						session = resumed
						appleIDValue = resumed.UserEmail
					}
				}
			}

			// If no flags provided, run interactive mode
			if *name == "" && *bundleID == "" && *sku == "" {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Create a new app in App Store Connect")
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Note: App creation requires your Apple ID credentials.")
				fmt.Fprintln(os.Stderr)

				if session == nil {
					// Prompt for Apple ID
					if err := survey.AskOne(&survey.Input{
						Message: "Apple ID (email):",
						Help:    "Your Apple ID email address",
					}, &appleIDValue, survey.WithValidator(survey.Required)); err != nil {
						return err
					}

					// Prompt for password
					if err := survey.AskOne(&survey.Password{
						Message: "Apple ID password:",
						Help:    "Your Apple ID password",
					}, &passwordValue, survey.WithValidator(survey.Required)); err != nil {
						return err
					}
				}

				// Prompt for name
				if err := survey.AskOne(&survey.Input{
					Message: "App name:",
					Help:    "The name of your app as it will appear in App Store Connect",
				}, &nameValue, survey.WithValidator(survey.Required)); err != nil {
					return err
				}

				// Prompt for bundle ID
				if err := survey.AskOne(&survey.Input{
					Message: "Bundle ID:",
					Help:    "The bundle identifier (e.g., com.example.myapp). Must match an App ID in your developer account.",
				}, &bundleIDValue, survey.WithValidator(survey.Required)); err != nil {
					return err
				}

				// Prompt for SKU
				if err := survey.AskOne(&survey.Input{
					Message: "SKU:",
					Help:    "A unique identifier for your app (alphanumeric, used internally by Apple)",
				}, &skuValue, survey.WithValidator(survey.Required)); err != nil {
					return err
				}

				// Prompt for primary locale
				if err := survey.AskOne(&survey.Input{
					Message: "Primary locale:",
					Default: "en-US",
					Help:    "The primary language for your app (e.g., en-US, en-GB, de-DE)",
				}, &localeValue); err != nil {
					return err
				}

				// Prompt for platform
				if err := survey.AskOne(&survey.Select{
					Message: "Platform:",
					Options: []string{"IOS", "MAC_OS", "UNIVERSAL"},
					Default: "IOS",
					Help:    "The primary platform for your app",
				}, &platformValue); err != nil {
					return err
				}
			} else {
				// Use provided flags
				nameValue = strings.TrimSpace(*name)
				if nameValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --name is required")
					return flag.ErrHelp
				}
				bundleIDValue = strings.TrimSpace(*bundleID)
				if bundleIDValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
					return flag.ErrHelp
				}
				skuValue = strings.TrimSpace(*sku)
				if skuValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --sku is required")
					return flag.ErrHelp
				}
				localeValue = strings.TrimSpace(*primaryLocale)
				if localeValue == "" {
					localeValue = "en-US"
				}
				platformValue = strings.TrimSpace(*platform)

				if session == nil {
					// Get Apple ID credentials
					appleIDValue = strings.TrimSpace(*appleID)
					if appleIDValue == "" {
						if err := survey.AskOne(&survey.Input{
							Message: "Apple ID (email):",
							Help:    "Your Apple ID email address",
						}, &appleIDValue, survey.WithValidator(survey.Required)); err != nil {
							return err
						}
					}

					passwordValue = strings.TrimSpace(*password)
					if passwordValue == "" {
						if err := survey.AskOne(&survey.Password{
							Message: "Apple ID password:",
							Help:    "Your Apple ID password",
						}, &passwordValue, survey.WithValidator(survey.Required)); err != nil {
							return err
						}
					}
				}
			}

			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  Name:      %s\n", nameValue)
			fmt.Fprintf(os.Stderr, "  Bundle ID: %s\n", bundleIDValue)
			fmt.Fprintf(os.Stderr, "  SKU:       %s\n", skuValue)
			fmt.Fprintf(os.Stderr, "  Locale:    %s\n", localeValue)
			if platformValue != "" {
				fmt.Fprintf(os.Stderr, "  Platform:  %s\n", platformValue)
			}
			fmt.Fprintln(os.Stderr)

			if session == nil {
				fmt.Fprintln(os.Stderr, "Authenticating with Apple ID...")
				var err error
				session, err = iris.Login(iris.LoginCredentials{
					Username: appleIDValue,
					Password: passwordValue,
				})
				if err != nil {
					var tfaErr *iris.TwoFactorRequiredError
					if session != nil && errors.As(err, &tfaErr) {
						codeValue := strings.TrimSpace(*twoFactorCode)
						if codeValue == "" {
							if err := survey.AskOne(&survey.Input{
								Message: "2FA code:",
								Help:    "Enter the verification code from your trusted device or SMS",
							}, &codeValue, survey.WithValidator(func(ans interface{}) error {
								s, _ := ans.(string)
								s = strings.TrimSpace(s)
								if len(s) != 6 {
									return fmt.Errorf("2FA code must be 6 digits")
								}
								for _, r := range s {
									if r < '0' || r > '9' {
										return fmt.Errorf("2FA code must be numeric")
									}
								}
								return nil
							})); err != nil {
								return err
							}
						}

						fmt.Fprintln(os.Stderr, "Verifying 2FA code...")
						if err := iris.SubmitTwoFactorCode(session, codeValue); err != nil {
							return fmt.Errorf("apps create: 2FA verification failed: %w", err)
						}
					} else {
						return fmt.Errorf("apps create: authentication failed: %w", err)
					}
				}
			}
			iris.PersistSession(session)
			fmt.Fprintf(os.Stderr, "✓ Authenticated as %s\n", session.UserEmail)

			client := iris.NewClient(session)

			attrs := iris.AppCreateAttributes{
				Name:          nameValue,
				BundleID:      bundleIDValue,
				SKU:           skuValue,
				PrimaryLocale: localeValue,
				Platform:      platformValue,
			}

			fmt.Fprintln(os.Stderr, "Creating app...")
			app, err := client.CreateApp(attrs)
			if err != nil && *autoRename && iris.IsDuplicateAppNameError(err) {
				suffix := bundleIDNameSuffix(bundleIDValue)
				if suffix != "" {
					for i := 0; i < 5; i++ {
						trySuffix := suffix
						if i > 0 {
							trySuffix = fmt.Sprintf("%s-%d", suffix, i+1)
						}
						tryName := formatAppNameWithSuffix(nameValue, trySuffix)
						if tryName == "" || tryName == attrs.Name {
							continue
						}
						fmt.Fprintf(os.Stderr, "App name already in use; retrying with %q...\n", tryName)
						attrs.Name = tryName
						app, err = client.CreateApp(attrs)
						if err == nil || !iris.IsDuplicateAppNameError(err) {
							break
						}
					}
				}
			}
			if err != nil {
				return fmt.Errorf("apps create: failed to create: %w", err)
			}

			fmt.Fprintf(os.Stderr, "✓ App created successfully! App ID: %s\n", app.Data.ID)
			return shared.PrintOutput(app, *output.Output, *output.Pretty)
		},
	}
}

const maxAppNameLen = 30

func bundleIDNameSuffix(bundleID string) string {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return ""
	}
	parts := strings.Split(bundleID, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			// Keep it ASCII-ish and friendly for display names.
			p = sanitizeAppNameSuffix(p)
			return p
		}
	}
	return ""
}

func sanitizeAppNameSuffix(s string) string {
	// Allow letters/digits and convert other characters to '-'.
	var b strings.Builder
	b.Grow(len(s))
	lastDash := false
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}

func formatAppNameWithSuffix(baseName, suffix string) string {
	baseName = strings.TrimSpace(baseName)
	suffix = strings.TrimSpace(suffix)
	if baseName == "" || suffix == "" {
		return ""
	}

	sep := " - "
	maxBase := maxAppNameLen - len(sep) - len(suffix)
	if maxBase <= 0 {
		// If suffix is too long, fall back to a truncated suffix-only name.
		if len(suffix) > maxAppNameLen {
			return suffix[:maxAppNameLen]
		}
		return suffix
	}
	if len(baseName) > maxBase {
		baseName = strings.TrimSpace(baseName[:maxBase])
		baseName = strings.TrimRight(baseName, "-")
		baseName = strings.TrimSpace(baseName)
	}
	if baseName == "" {
		return suffix
	}
	return baseName + sep + suffix
}

// AppsUpdateCommand returns the apps update subcommand.
func AppsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps update", flag.ExitOnError)

	id := fs.String("id", "", "App Store Connect app ID")
	bundleID := fs.String("bundle-id", "", "Update bundle ID")
	primaryLocale := fs.String("primary-locale", "", "Update primary locale (e.g., en-US)")
	contentRights := fs.String("content-rights", "", "Content rights declaration: DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc apps update --id APP_ID [--bundle-id BUNDLE_ID] [--primary-locale LOCALE] [--content-rights DECLARATION]",
		ShortHelp:  "Update an app's bundle ID, primary locale, or content rights declaration.",
		LongHelp: `Update an app's bundle ID, primary locale, or content rights declaration.

Examples:
  asc apps update --id "APP_ID" --bundle-id "com.example.app"
  asc apps update --id "APP_ID" --primary-locale "en-US"
  asc apps update --id "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.AppUpdateAttributes{}
			if bundleValue := strings.TrimSpace(*bundleID); bundleValue != "" {
				attrs.BundleID = &bundleValue
			}
			if localeValue := strings.TrimSpace(*primaryLocale); localeValue != "" {
				attrs.PrimaryLocale = &localeValue
			}
			if rightsValue := strings.TrimSpace(*contentRights); rightsValue != "" {
				normalizedRights := asc.ContentRightsDeclaration(strings.ToUpper(rightsValue))
				switch normalizedRights {
				case asc.ContentRightsDeclarationDoesNotUseThirdPartyContent,
					asc.ContentRightsDeclarationUsesThirdPartyContent:
					attrs.ContentRightsDeclaration = &normalizedRights
				default:
					fmt.Fprintf(os.Stderr, "Error: --content-rights must be %s or %s\n", asc.ContentRightsDeclarationDoesNotUseThirdPartyContent, asc.ContentRightsDeclarationUsesThirdPartyContent)
					return flag.ErrHelp
				}
			}
			if attrs.BundleID == nil && attrs.PrimaryLocale == nil && attrs.ContentRightsDeclaration == nil {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id, --primary-locale, or --content-rights is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			app, err := client.UpdateApp(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("apps update: failed to update: %w", err)
			}

			return shared.PrintOutput(app, *output.Output, *output.Pretty)
		},
	}
}

func appsList(ctx context.Context, output string, pretty bool, bundleID string, name string, sku string, sort string, limit int, next string, paginate bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("apps: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("apps: %w", err)
	}
	if err := shared.ValidateSort(sort, "name", "-name", "bundleId", "-bundleId"); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.AppsOption{
		asc.WithAppsBundleIDs(shared.SplitCSV(bundleID)),
		asc.WithAppsNames(shared.SplitCSV(name)),
		asc.WithAppsSKUs(shared.SplitCSV(sku)),
		asc.WithAppsLimit(limit),
		asc.WithAppsNextURL(next),
	}
	if strings.TrimSpace(sort) != "" {
		opts = append(opts, asc.WithAppsSort(sort))
	}

	if paginate {
		paginateOpts := append(opts, asc.WithAppsLimit(200))
		apps, err := shared.PaginateWithSpinner(requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetApps(ctx, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("apps: %w", err)
		}

		return shared.PrintOutput(apps, output, pretty)
	}

	apps, err := client.GetApps(requestCtx, opts...)
	if err != nil {
		return fmt.Errorf("apps: failed to fetch: %w", err)
	}

	return shared.PrintOutput(apps, output, pretty)
}
