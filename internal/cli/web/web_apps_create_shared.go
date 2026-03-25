package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// AppsCreateRunOptions configures the canonical web-backed app-create flow.
type AppsCreateRunOptions struct {
	Name          string
	BundleID      string
	SKU           string
	PrimaryLocale string
	Platform      string
	Version       string
	CompanyName   string

	AppleID              string
	Password             string
	TwoFactorCode        string
	TwoFactorCodeCommand string

	AutoRename bool
	Output     string
	Pretty     bool

	// Deprecated shim compatibility: when a direct password is provided without an
	// Apple ID, preserve the old behavior of prompting for account selection
	// instead of silently reusing the last cached session.
	PromptForAppleIDWithPassword bool

	// Deprecated shim compatibility: preserve the old apps-create contract and
	// avoid official ASC bundle-ID preflight side effects.
	DisableBundleIDPreflight bool
}

const (
	appCreateDefaultPrimaryLocale = "en-US"
	appCreateDefaultPlatform      = "IOS"
	appCreateDefaultVersion       = "1.0"
)

var (
	appCreateAskOneFn                     = survey.AskOne
	resolveAppCreateSessionFn         any = resolveAppCreateSession
	appCreateCanPromptInteractivelyFn     = appCreateCanPromptInteractively
)

func callResolveAppCreateSessionFn(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	return callSessionResolverHook(ctx, resolveAppCreateSessionFn, "app create session resolver", appleID, password, twoFactorCode, twoFactorCodeCommand)
}

func appCreateCanPromptInteractively() bool {
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		_ = tty.Close()
		return true
	}
	return termIsTerminalFn(int(os.Stdin.Fd()))
}

func trimAppsCreateRunOptions(opts AppsCreateRunOptions) AppsCreateRunOptions {
	opts.Name = strings.TrimSpace(opts.Name)
	opts.BundleID = strings.TrimSpace(opts.BundleID)
	opts.SKU = strings.TrimSpace(opts.SKU)
	opts.PrimaryLocale = strings.TrimSpace(opts.PrimaryLocale)
	opts.Platform = strings.ToUpper(strings.TrimSpace(opts.Platform))
	opts.Version = strings.TrimSpace(opts.Version)
	opts.CompanyName = strings.TrimSpace(opts.CompanyName)
	opts.AppleID = strings.TrimSpace(opts.AppleID)
	opts.TwoFactorCode = strings.TrimSpace(opts.TwoFactorCode)
	opts.TwoFactorCodeCommand = strings.TrimSpace(opts.TwoFactorCodeCommand)
	opts.Output = strings.TrimSpace(opts.Output)
	return opts
}

func normalizeAppsCreateRunOptions(opts AppsCreateRunOptions) AppsCreateRunOptions {
	if opts.PrimaryLocale == "" {
		opts.PrimaryLocale = appCreateDefaultPrimaryLocale
	}
	if opts.Platform == "" {
		opts.Platform = appCreateDefaultPlatform
	}
	if opts.Version == "" {
		opts.Version = appCreateDefaultVersion
	}
	return opts
}

func promptAppsCreateFields(opts *AppsCreateRunOptions) error {
	if opts == nil {
		return fmt.Errorf("app create options are required")
	}
	fullWizard := strings.TrimSpace(opts.Name) == "" && strings.TrimSpace(opts.BundleID) == "" && strings.TrimSpace(opts.SKU) == ""

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Create a new app in App Store Connect")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Note: App creation uses Apple's unofficial web-session create flow.")
	fmt.Fprintln(os.Stderr)

	nameValue := strings.TrimSpace(opts.Name)
	if nameValue == "" {
		if err := appCreateAskOneFn(&survey.Input{
			Message: "App name:",
			Help:    "The name of your app as it will appear in App Store Connect",
		}, &nameValue, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	bundleIDValue := strings.TrimSpace(opts.BundleID)
	if bundleIDValue == "" {
		if err := appCreateAskOneFn(&survey.Input{
			Message: "Bundle ID:",
			Help:    "The bundle identifier (for example, com.example.myapp). Must match an App ID in your developer account.",
		}, &bundleIDValue, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	skuValue := strings.TrimSpace(opts.SKU)
	if skuValue == "" {
		if err := appCreateAskOneFn(&survey.Input{
			Message: "SKU:",
			Help:    "A unique identifier for your app used internally by Apple",
		}, &skuValue, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	localeValue := strings.TrimSpace(opts.PrimaryLocale)
	platformValue := strings.TrimSpace(opts.Platform)
	if fullWizard {
		if localeValue == "" {
			localeValue = appCreateDefaultPrimaryLocale
		}
		if err := appCreateAskOneFn(&survey.Input{
			Message: "Primary locale:",
			Default: localeValue,
			Help:    "The primary language for your app (for example, en-US, en-GB, de-DE)",
		}, &localeValue); err != nil {
			return err
		}

		if platformValue == "" {
			platformValue = appCreateDefaultPlatform
		}
		if err := appCreateAskOneFn(&survey.Select{
			Message: "Platform:",
			Options: []string{"IOS", "MAC_OS", "TV_OS", "UNIVERSAL"},
			Default: platformValue,
			Help:    "The primary platform for your app",
		}, &platformValue); err != nil {
			return err
		}
	}

	opts.Name = strings.TrimSpace(nameValue)
	opts.BundleID = strings.TrimSpace(bundleIDValue)
	opts.SKU = strings.TrimSpace(skuValue)
	opts.PrimaryLocale = strings.TrimSpace(localeValue)
	opts.Platform = strings.ToUpper(strings.TrimSpace(platformValue))
	return nil
}

func promptAppsCreateAppleID(appleID *string) error {
	if appleID == nil {
		return fmt.Errorf("apple id target is required")
	}
	value := strings.TrimSpace(*appleID)
	if err := appCreateAskOneFn(&survey.Input{
		Message: "Apple ID (email):",
		Help:    "Your Apple ID email address",
	}, &value, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	*appleID = strings.TrimSpace(value)
	return nil
}

func promptAppsCreatePassword(password *string) error {
	if password == nil {
		return fmt.Errorf("password target is required")
	}
	value := *password
	if err := appCreateAskOneFn(&survey.Password{
		Message: "Apple ID password:",
		Help:    "Your Apple ID password",
	}, &value, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	*password = value
	return nil
}

func promptAppsCreateSessionAppleID(appleID *string) error {
	if !appCreateCanPromptInteractivelyFn() {
		return shared.UsageError("--apple-id is required when no cached web session is available")
	}
	return promptAppsCreateAppleID(appleID)
}

func appCreatePasswordInputProvided(password string) bool {
	return password != ""
}

func resolveAppCreatePassword(_ context.Context, password string) (string, error) {
	if webPasswordProvided(password) {
		return password, nil
	}
	password = os.Getenv(webPasswordEnv)
	if webPasswordProvided(password) {
		return password, nil
	}
	if !appCreateCanPromptInteractivelyFn() {
		return "", nil
	}
	if err := promptAppsCreatePassword(&password); err != nil {
		return "", err
	}
	if !webPasswordProvided(password) {
		return "", nil
	}
	return password, nil
}

func persistFreshAppCreateSession(session *webcore.AuthSession) error {
	// App creation can proceed with the in-memory session even if cache persistence fails.
	_ = persistWebSessionFn(session)
	return nil
}

func rollbackCreatedBundleID(ctx context.Context, bundleID string) error {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil
	}
	rollbackCtx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	return withWebSpinner("Rolling back created Bundle ID", func() error {
		return deleteBundleIDFn(rollbackCtx, bundleID)
	})
}

func resolveAppCreateSession(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = twoFactorCodeCommand[0]
	}
	session, source, err := resolveWebSession(ctx, appleID, password, twoFactorCode, webSessionResolveOptions{
		promptAppleID:        promptAppsCreateSessionAppleID,
		resolvePassword:      resolveAppCreatePassword,
		persistFresh:         persistFreshAppCreateSession,
		twoFactorCodeCommand: command,
	})
	if err != nil {
		return nil, "", err
	}
	return session, source, nil
}

// RunAppsCreate executes the canonical web-backed app-create flow.
func RunAppsCreate(ctx context.Context, opts AppsCreateRunOptions) error {
	opts = trimAppsCreateRunOptions(opts)

	missingName := opts.Name == ""
	missingBundleID := opts.BundleID == ""
	missingSKU := opts.SKU == ""
	if missingName || missingBundleID || missingSKU {
		if !appCreateCanPromptInteractivelyFn() {
			missingFlags := make([]string, 0, 3)
			if missingName {
				missingFlags = append(missingFlags, "--name")
			}
			if missingBundleID {
				missingFlags = append(missingFlags, "--bundle-id")
			}
			if missingSKU {
				missingFlags = append(missingFlags, "--sku")
			}
			return shared.UsageError(fmt.Sprintf("missing required flags: %s", strings.Join(missingFlags, ", ")))
		}
		if err := promptAppsCreateFields(&opts); err != nil {
			return err
		}
	}

	if opts.PromptForAppleIDWithPassword && opts.AppleID == "" && appCreatePasswordInputProvided(opts.Password) {
		if err := promptAppsCreateSessionAppleID(&opts.AppleID); err != nil {
			return err
		}
	}

	opts = normalizeAppsCreateRunOptions(opts)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Name:      %s\n", opts.Name)
	fmt.Fprintf(os.Stderr, "  Bundle ID: %s\n", opts.BundleID)
	fmt.Fprintf(os.Stderr, "  SKU:       %s\n", opts.SKU)
	fmt.Fprintf(os.Stderr, "  Locale:    %s\n", opts.PrimaryLocale)
	if opts.Platform != "" {
		fmt.Fprintf(os.Stderr, "  Platform:  %s\n", opts.Platform)
	}
	fmt.Fprintln(os.Stderr)

	session, source, err := callResolveAppCreateSessionFn(
		ctx,
		opts.AppleID,
		opts.Password,
		opts.TwoFactorCode,
		opts.TwoFactorCodeCommand,
	)
	if err != nil {
		return err
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if source == "fresh" {
		fmt.Fprintln(os.Stderr, "Authenticated via fresh web login.")
	} else {
		fmt.Fprintln(os.Stderr, "Using cached web session.")
	}

	client := newWebClientFn(session)
	attrs := webcore.AppCreateAttributes{
		Name:          opts.Name,
		BundleID:      opts.BundleID,
		SKU:           opts.SKU,
		PrimaryLocale: opts.PrimaryLocale,
		Platform:      opts.Platform,
		VersionString: opts.Version,
		CompanyName:   opts.CompanyName,
	}

	createdBundleID := false
	if !opts.DisableBundleIDPreflight {
		preflightCreatedBundleID, preflightErr := withWebSpinnerValue("Checking or creating Bundle ID", func() (bool, error) {
			return ensureBundleIDFn(requestCtx, opts.BundleID, opts.Name, opts.Platform)
		})
		createdBundleID = preflightCreatedBundleID
		if preflightErr != nil {
			if errors.Is(preflightErr, shared.ErrMissingAuth) || errors.Is(preflightErr, errBundleIDPreflightAuthUnavailable) {
				fmt.Fprintln(os.Stderr, "Skipping Bundle ID preflight because official ASC API authentication is unavailable or misconfigured.")
				createdBundleID = false
			} else {
				return fmt.Errorf("web apps create failed: bundle id preflight failed: %w", preflightErr)
			}
		}
		if createdBundleID {
			fmt.Fprintf(os.Stderr, "Bundle ID %q was missing; created automatically.\n", opts.BundleID)
		}
	}

	app, err := withWebSpinnerValue("Creating app via Apple web API", func() (*webcore.AppResponse, error) {
		return createWebAppFn(requestCtx, client, attrs)
	})
	if err != nil && opts.AutoRename && webcore.IsDuplicateAppNameError(err) {
		suffix := bundleIDNameSuffix(opts.BundleID)
		if suffix != "" {
			for i := 0; i < 5; i++ {
				trySuffix := suffix
				if i > 0 {
					trySuffix = fmt.Sprintf("%s-%d", suffix, i+1)
				}
				tryName := formatAppNameWithSuffix(opts.Name, trySuffix)
				if tryName == "" || tryName == attrs.Name {
					continue
				}
				fmt.Fprintf(os.Stderr, "App name in use; retrying with %q...\n", tryName)
				attrs.Name = tryName
				app, err = withWebSpinnerValue("Creating app via Apple web API", func() (*webcore.AppResponse, error) {
					return createWebAppFn(requestCtx, client, attrs)
				})
				if err == nil || !webcore.IsDuplicateAppNameError(err) {
					break
				}
			}
		}
	}
	if err != nil {
		if createdBundleID {
			if rollbackErr := rollbackCreatedBundleID(ctx, opts.BundleID); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to roll back created bundle id %q: %w", opts.BundleID, rollbackErr))
			}
		}
		return fmt.Errorf("web apps create failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Created app successfully (id=%s)\n", strings.TrimSpace(app.Data.ID))
	return shared.PrintOutput(app, opts.Output, opts.Pretty)
}
