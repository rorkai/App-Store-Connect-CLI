package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// WebAppsCommand returns the detached web apps command group.
func WebAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc web apps <subcommand> [flags]",
		ShortHelp:  "EXPERIMENTAL: Unofficial app management via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage app operations using Apple web sessions and internal APIs.
This command group is detached from official App Store Connect API flows.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
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

	appleID := fs.String("apple-id", "", "Apple ID email (required when no cache is available)")
	passwordStdin := fs.Bool("password-stdin", false, "Read Apple ID password from stdin")
	twoFactorCode := fs.String("two-factor-code", "", "2FA code if your account requires verification")
	autoRename := fs.Bool("auto-rename", true, "Retry with unique name suffix if app name is already taken")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps create --name NAME --bundle-id BUNDLE_ID --sku SKU [flags]",
		ShortHelp:  "EXPERIMENTAL: Create app via unofficial web API.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create an app through Apple's internal web API using a web-session login.
This path is detached from official API-key workflows and may break any time.

Required:
  --name, --bundle-id, --sku

Authentication:
  --apple-id with either:
    - --password-stdin
    - ASC_WEB_PASSWORD environment variable
  Cached web sessions may be reused automatically.

` + webWarningText + `

Examples:
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com" --password-stdin
  ASC_WEB_PASSWORD="..." asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			bundleIDValue := strings.TrimSpace(*bundleID)
			if bundleIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return flag.ErrHelp
			}
			skuValue := strings.TrimSpace(*sku)
			if skuValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --sku is required")
				return flag.ErrHelp
			}

			password, err := readPasswordFromInput(*passwordStdin)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, source, err := resolveSession(requestCtx, *appleID, password, *twoFactorCode, *passwordStdin, true)
			if err != nil {
				return err
			}
			if source == "fresh" {
				fmt.Fprintln(os.Stderr, "Authenticated via fresh web login.")
			} else {
				fmt.Fprintln(os.Stderr, "Using cached web session.")
			}

			client := webcore.NewClient(session)
			attrs := webcore.AppCreateAttributes{
				Name:          nameValue,
				BundleID:      bundleIDValue,
				SKU:           skuValue,
				PrimaryLocale: strings.TrimSpace(*primaryLocale),
				Platform:      strings.TrimSpace(*platform),
				VersionString: strings.TrimSpace(*version),
				CompanyName:   strings.TrimSpace(*companyName),
			}

			app, err := client.CreateApp(requestCtx, attrs)
			if err != nil && *autoRename && webcore.IsDuplicateAppNameError(err) {
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
						fmt.Fprintf(os.Stderr, "App name in use; retrying with %q...\n", tryName)
						attrs.Name = tryName
						app, err = client.CreateApp(requestCtx, attrs)
						if err == nil || !webcore.IsDuplicateAppNameError(err) {
							break
						}
					}
				}
			}
			if err != nil {
				return fmt.Errorf("web apps create failed: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Created app successfully (id=%s)\n", strings.TrimSpace(app.Data.ID))
			return shared.PrintOutput(app, *output.Output, *output.Pretty)
		},
	}
}
