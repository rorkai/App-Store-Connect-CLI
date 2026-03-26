package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// WebAppsCommand returns the detached web apps command group.
func WebAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc web apps <subcommand> [flags]",
		ShortHelp:  "[experimental] Unofficial app management via web sessions; canonical path for app creation.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage app operations using Apple web sessions and internal APIs.
This command group is detached from official App Store Connect API flows.
Use ` + "`asc web apps create`" + ` as the canonical app-creation command.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsCreateCommand(),
			WebAppsAvailabilityCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

const maxAppNameLen = 30

var (
	newWebClientFn   = webcore.NewClient
	ensureBundleIDFn = ensureBundleIDExists
	deleteBundleIDFn = deleteBundleIDByIdentifier
	createWebAppFn   = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		return client.CreateApp(ctx, attrs)
	}
)

var errBundleIDPreflightAuthUnavailable = errors.New("bundle id preflight official auth unavailable")

func isDuplicateBundleIDError(err error) bool {
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	code := strings.ToUpper(strings.TrimSpace(apiErr.Code))
	title := strings.ToLower(strings.TrimSpace(apiErr.Title))
	detail := strings.ToLower(strings.TrimSpace(apiErr.Detail))
	if strings.Contains(code, "DUPLICATE") {
		return true
	}
	if strings.Contains(detail, "already") && (strings.Contains(detail, "identifier") || strings.Contains(detail, "bundle")) {
		return true
	}
	if strings.Contains(title, "already") && (strings.Contains(title, "identifier") || strings.Contains(title, "bundle")) {
		return true
	}
	return false
}

func bundleIDPlatformForWebApp(platform string) (asc.Platform, error) {
	switch strings.ToUpper(strings.TrimSpace(platform)) {
	case "", "IOS":
		return asc.PlatformIOS, nil
	case "MAC_OS":
		return asc.PlatformMacOS, nil
	case "TV_OS":
		return asc.PlatformTVOS, nil
	case "UNIVERSAL":
		// Bundle ID creation does not accept UNIVERSAL; IOS is the compatible preflight platform.
		return asc.PlatformIOS, nil
	default:
		return "", fmt.Errorf("platform must be one of IOS, MAC_OS, TV_OS, UNIVERSAL")
	}
}

func ensureBundleIDExists(ctx context.Context, bundleID, appName, platform string) (bool, error) {
	client, err := shared.GetASCClient()
	if err != nil {
		return false, errors.Join(errBundleIDPreflightAuthUnavailable, err)
	}

	platformValue, err := bundleIDPlatformForWebApp(platform)
	if err != nil {
		return false, err
	}

	existing, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(bundleID), asc.WithBundleIDsLimit(1))
	if err != nil {
		return false, err
	}
	if existing != nil && len(existing.Data) > 0 {
		return false, nil
	}

	_, err = client.CreateBundleID(ctx, asc.BundleIDCreateAttributes{
		Name:       appName,
		Identifier: bundleID,
		Platform:   platformValue,
	})
	if err != nil {
		if isDuplicateBundleIDError(err) {
			existing, findErr := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(bundleID), asc.WithBundleIDsLimit(1))
			if findErr == nil && existing != nil && len(existing.Data) > 0 {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

func deleteBundleIDByIdentifier(ctx context.Context, bundleID string) error {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return err
	}

	existing, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(bundleID), asc.WithBundleIDsLimit(1))
	if err != nil {
		return err
	}
	if existing == nil || len(existing.Data) == 0 {
		return nil
	}

	return client.DeleteBundleID(ctx, strings.TrimSpace(existing.Data[0].ID))
}

func bundleIDNameSuffix(bundleID string) string {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return ""
	}
	parts := strings.Split(bundleID, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		part := sanitizeAppNameSuffix(strings.TrimSpace(parts[i]))
		if part != "" {
			return part
		}
	}
	return ""
}

func sanitizeAppNameSuffix(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
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
	return strings.Trim(b.String(), "-")
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

// WebAppsCreateCommand creates apps using the internal web API.
func WebAppsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps create", flag.ExitOnError)

	name := fs.String("name", "", "App display name")
	bundleID := fs.String("bundle-id", "", "Bundle ID (for example: com.example.app)")
	sku := fs.String("sku", "", "Unique SKU for the app")
	primaryLocale := fs.String("primary-locale", "en-US", "Primary locale (for example: en-US)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, UNIVERSAL")
	version := fs.String("version", "1.0", "Initial version string")
	companyName := fs.String("company-name", "", "Company name (optional)")

	appleID := fs.String("apple-id", "", "Apple Account email (required when no cache is available)")
	password := fs.String("password", "", "Apple Account password (temporary compatibility flag; will prompt if not provided)")
	twoFactorCode := bindDeprecatedTwoFactorCodeFlag(fs)
	twoFactorCodeCommand := fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required")
	autoRename := fs.Bool("auto-rename", true, "Retry with unique name suffix if app name is already taken")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps create [flags]",
		ShortHelp:  "[experimental] Create app via unofficial Apple web API.",
		LongHelp: fmt.Sprintf(`EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create an app through Apple's internal web API using a web-session login.
This is the canonical app-creation path for web-session based flows and is
detached from official API-key workflows.

If required fields are omitted in an interactive terminal, the CLI will prompt
for the missing app-creation inputs.

Authentication:
  --apple-id with one of:
    - secure interactive prompt (default and recommended for local use)
    - %s environment variable
    - temporary direct-password compatibility flag during the apps-create deprecation window
  Two-factor verification can use --two-factor-code-command
  or %s if a fresh login is required.
  The legacy --two-factor-code flag still works as a deprecated compatibility alias.
  If you already have a cached web session, --apple-id can be omitted.

Bundle ID preflight:
  If official ASC API authentication is available, the CLI will check or create
  the Bundle ID before app creation. Otherwise it assumes the Bundle ID already exists.

`+webWarningText+`

Examples:
  asc web apps create
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com"
  %s asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com"
  %s='osascript /path/to/get-apple-2fa-code.scpt' asc web apps create --apple-id "user@example.com"`,
			webPasswordEnvDisplay(),
			webTwoFactorCodeCommandEnv,
			webPasswordEnvAssignmentExample(),
			webTwoFactorCodeCommandEnv,
		),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnDeprecatedTwoFactorCodeFlag(*twoFactorCode)
			return RunAppsCreate(ctx, AppsCreateRunOptions{
				Name:                 *name,
				BundleID:             *bundleID,
				SKU:                  *sku,
				PrimaryLocale:        *primaryLocale,
				Platform:             *platform,
				Version:              *version,
				CompanyName:          *companyName,
				AppleID:              *appleID,
				Password:             *password,
				TwoFactorCode:        *twoFactorCode,
				TwoFactorCodeCommand: *twoFactorCodeCommand,
				AutoRename:           *autoRename,
				Output:               *output.Output,
				Pretty:               *output.Pretty,
			})
		},
	}
}
