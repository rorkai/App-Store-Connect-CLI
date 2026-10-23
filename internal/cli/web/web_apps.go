package web

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
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// WebAppsCommand returns the detached web apps command group.
func WebAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc web apps <subcommand> [flags]",
		ShortHelp:  "App management via web sessions; canonical path for app creation.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage app operations using Apple web sessions and internal APIs.
Use ` + "`asc web apps create`" + ` as the canonical app-creation command.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsCreateCommand(),
			WebAppsDeleteCommand(),
			WebAppsAvailabilityCommand(),
			WebAppsCompatibilityCommand(),
			WebAppsDistributionCommand(),
			WebAppsHistoryCommand(),
			WebAppsTransferCommand(),
			WebAppsDeclarationsCommand(),
			WebAppsMedicalDeviceCommand(),
			WebAppsTaxCategoryCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

const maxAppNameLen = 30

// webAppValueOrUnknown renders an attribute App Store Connect omitted as
// "unknown" in table output instead of an empty cell or a guessed default.
func webAppValueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

var (
	newWebClientFn   = webcore.NewClient
	ensureBundleIDFn = ensureBundleIDExists
	deleteBundleIDFn = deleteBundleIDByIdentifier
	createWebAppFn   = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		return client.CreateApp(ctx, attrs)
	}
	findWebAppFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.AppResponse, error) {
		return client.FindApp(ctx, bundleID)
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		return client.DeleteApp(ctx, appID)
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

func bundleIDPlatformForWebApp(platform string) (asc.BundleIDPlatform, error) {
	switch strings.ToUpper(strings.TrimSpace(platform)) {
	case "", "IOS":
		return asc.BundleIDPlatformIOS, nil
	case "MAC_OS":
		return asc.BundleIDPlatformMacOS, nil
	case "TV_OS":
		return asc.BundleIDPlatformIOS, nil
	case "UNIVERSAL":
		return asc.BundleIDPlatformUniversal, nil
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

	if _, err := shared.FindBundleID(ctx, client, bundleID); !errors.Is(err, shared.ErrBundleIDNotFound) {
		return false, err
	}

	_, err = client.CreateBundleID(ctx, asc.BundleIDCreateAttributes{
		Name:       appName,
		Identifier: bundleID,
		Platform:   platformValue,
	})
	if err != nil {
		if isDuplicateBundleIDError(err) {
			if _, findErr := shared.FindBundleID(ctx, client, bundleID); findErr == nil {
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

	existing, err := shared.FindBundleID(ctx, client, bundleID)
	if errors.Is(err, shared.ErrBundleIDNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return client.DeleteBundleID(ctx, strings.TrimSpace(existing.Data.ID))
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

// autoRenameCandidate returns the name --auto-rename tries on the given
// zero-based attempt: the bundle-ID suffix first, then suffix-2, suffix-3, ...
func autoRenameCandidate(baseName, suffix string, attempt int) string {
	trySuffix := suffix
	if attempt > 0 {
		trySuffix = fmt.Sprintf("%s-%d", suffix, attempt+1)
	}
	return formatAppNameWithSuffix(baseName, trySuffix)
}

func formatAppNameWithSuffix(baseName, suffix string) string {
	baseName = strings.TrimSpace(baseName)
	suffix = strings.TrimSpace(suffix)
	if baseName == "" || suffix == "" {
		return ""
	}
	sep := " - "
	suffixRunes := []rune(suffix)
	maxBase := maxAppNameLen - len([]rune(sep)) - len(suffixRunes)
	if maxBase <= 0 {
		if len(suffixRunes) > maxAppNameLen {
			return string(suffixRunes[:maxAppNameLen])
		}
		return suffix
	}
	baseNameRunes := []rune(baseName)
	if len(baseNameRunes) > maxBase {
		baseName = strings.TrimSpace(string(baseNameRunes[:maxBase]))
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

	appleID := fs.String("apple-id", "", "Apple Account email (defaults to "+webAppleIDEnv+", then the last or only cached session)")
	password := fs.String("password", "", "Deprecated: Apple Account password; use a cached web session or "+webPasswordEnv+" instead")
	twoFactorCodeCommand := fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required")
	twoFactorDelivery := bindTwoFactorDeliveryFlags(fs)
	autoRename := fs.Bool("auto-rename", true, "Retry with unique name suffix if app name is already taken")
	ifExists := shared.BindIfExistsFlag(fs, webAppCreateIfExistsModes...)
	access := fs.String("access", "", "App access after create: full or limited")
	var users shared.MultiStringFlag
	fs.Var(&users, "user", "User ID granted Limited Access (repeatable; requires --access limited)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps create [flags]",
		ShortHelp:  "Create app via Apple web API.",
		LongHelp: fmt.Sprintf(
			`WEB SESSION WORKFLOWS

Create an app through Apple's web API using a web-session login.
This is the canonical app-creation path for web-session based flows.

If required fields are omitted in an interactive terminal (stdin and stdout are
both terminals), the CLI will prompt for the missing app-creation inputs.
Otherwise it exits with a usage error listing the missing flags.

--access full|limited applies team access after create through the public
users API. Limited access requires at least one --user. Omitting --access
keeps the historical create request body.

Authentication:
  --apple-id with one of:
    - secure interactive prompt (default and recommended for local use)
    - %s environment variable
    - --password (deprecated; removed in the next major release)
  Two-factor verification can use --two-factor-code-command
  or %s if a fresh login is required; add --two-factor-method sms
  to have Apple text the code to a trusted phone number.
  If you already have a cached web session, --apple-id can be omitted.

Bundle ID preflight:
  If official ASC API authentication is available, the CLI will check or create
  the Bundle ID before app creation. Otherwise it assumes the Bundle ID already exists.

Existing apps (--if-exists fail|skip):
  fail (default) returns Apple's 409 unchanged. skip treats a 409 duplicate
  conflict as success when the public API read-back finds an app on this
  account with the requested bundle ID, SKU, and name (or, with --auto-rename,
  one of the suffixed names it would have tried); the app is left unchanged and
  a receipt with alreadyExists and action is printed. skip runs before
  --auto-rename: if the bundle ID or SKU belongs to a different app the command
  fails without renaming. skip needs official App Store Connect API
  authentication and cannot be combined with --access. update is not offered
  because none of the remaining create inputs has a safe matching write.

Examples:
  asc web apps create
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com"
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --access limited --user USER_ID
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --if-exists skip
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
			if flagProvided(fs, "password") {
				fmt.Fprintf(os.Stderr, "Warning: `asc web apps create --password` is deprecated and will be removed in the next major release. Run `asc web auth login --apple-id EMAIL` once and rerun without it to reuse the cached web session; unattended runs can set %s instead.\n", webPasswordEnv)
			}
			ifExistsMode, err := shared.ParseIfExistsMode(*ifExists, webAppCreateIfExistsModes...)
			if err != nil {
				return err
			}
			preference, err := twoFactorDelivery.resolve()
			if err != nil {
				return err
			}
			return RunAppsCreate(contextWithTwoFactorPreference(ctx, preference), AppsCreateRunOptions{
				Name:                 *name,
				BundleID:             *bundleID,
				SKU:                  *sku,
				PrimaryLocale:        *primaryLocale,
				Platform:             *platform,
				Version:              *version,
				CompanyName:          *companyName,
				AppleID:              *appleID,
				Password:             *password,
				TwoFactorCodeCommand: *twoFactorCodeCommand,
				AutoRename:           *autoRename,
				IfExists:             ifExistsMode,
				Access:               *access,
				Users:                append([]string(nil), users...),
				Output:               *output.Output,
				Pretty:               *output.Pretty,
			})
		},
	}
}
