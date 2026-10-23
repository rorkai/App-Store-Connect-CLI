package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	TwoFactorCodeCommand string

	AutoRename bool
	Output     string
	Pretty     bool

	// IfExists selects how an Apple 409 for an app that already exists is
	// handled. Empty means fail, the historical behavior.
	IfExists shared.IfExistsMode

	Access string
	Users  []string
}

const (
	appCreateAutoRenameAttempts   = 5
	appCreateDefaultPrimaryLocale = "en-US"
	appCreateDefaultPlatform      = "IOS"
	appCreateDefaultVersion       = "1.0"
)

var (
	appCreateAskOneFn                     = survey.AskOne
	resolveAppCreateSessionFn         any = resolveAppCreateSession
	appCreateCanPromptInteractivelyFn     = appCreateCanPromptInteractively
	appCreateCanPromptForFieldsFn         = appCreateCanPromptForFields
)

func callResolveAppCreateSessionFn(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	return callSessionResolverHook(ctx, resolveAppCreateSessionFn, "app create session resolver", appleID, password, twoFactorCode, twoFactorCodeCommand)
}

func appCreateCanPromptInteractively() bool {
	if tty, err := openTTYFn(); err == nil {
		_ = tty.Close()
		return true
	}
	return termIsTerminalFn(int(os.Stdin.Fd()))
}

// The field wizard reads stdin and renders to stdout, so a reachable
// controlling TTY is not enough: piped or captured streams must fail fast.
func appCreateCanPromptForFields() bool {
	return termIsTerminalFn(int(os.Stdin.Fd())) && termIsTerminalFn(int(os.Stdout.Fd()))
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
	opts.TwoFactorCodeCommand = strings.TrimSpace(opts.TwoFactorCodeCommand)
	opts.Output = strings.TrimSpace(opts.Output)
	opts.Access = strings.TrimSpace(opts.Access)
	if len(opts.Users) > 0 {
		users := make([]string, 0, len(opts.Users))
		for _, userID := range opts.Users {
			users = append(users, strings.TrimSpace(userID))
		}
		opts.Users = users
	}
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

func explainAppsCreateError(err error) error {
	if webcore.IsMissingCompanyNameError(err) {
		return fmt.Errorf(
			"web apps create failed: Apple requires a company name for this account; retry with --company-name \"Your Company\": %w",
			err,
		)
	}
	return fmt.Errorf("web apps create failed: %w", err)
}

func promptAppsCreateFields(opts *AppsCreateRunOptions) error {
	if opts == nil {
		return fmt.Errorf("app create options are required")
	}
	fullWizard := strings.TrimSpace(opts.Name) == "" && strings.TrimSpace(opts.BundleID) == "" && strings.TrimSpace(opts.SKU) == ""

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Create a new app in App Store Connect")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Note: App creation uses Apple's web-session create flow.")
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
		return newMissingWebSessionError("", "")
	}
	return promptAppsCreateAppleID(appleID)
}

// soleMissingFlag names the failing parameter only when exactly one required
// flag is absent. Multi-parameter requirements stay unattributed so the
// telemetry dimension never guesses which flag the caller meant to pass.
func soleMissingFlag(missingFlags []string) string {
	if len(missingFlags) != 1 {
		return ""
	}
	return missingFlags[0]
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
		// The terminal was available and the prompt came back empty: that is
		// missing input, not a missing session.
		return "", passwordRequiredUsageError()
	}
	return password, nil
}

func persistFreshAppCreateSession(session *webcore.AuthSession) error {
	// App creation can proceed with the in-memory session even if cache persistence fails.
	_ = persistWebSessionFn(session)
	return nil
}

func persistAutoReauthAppCreateSession(session *webcore.AuthSession) {
	_ = persistFreshAppCreateSession(session)
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
		persistAutoReauth:    persistAutoReauthAppCreateSession,
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

	access, userIDs, err := normalizeAppCreateAccess(opts.Access, opts.Users)
	if err != nil {
		return err
	}
	if opts.IfExists, err = shared.ParseOptionalIfExistsMode(string(opts.IfExists), webAppCreateIfExistsModes...); err != nil {
		return err
	}
	if opts.IfExists == shared.IfExistsSkip && access != "" {
		return shared.UsageError("--if-exists skip cannot be combined with --access: skip leaves an existing app unchanged, so the access change would not be applied")
	}

	missingName := opts.Name == ""
	missingBundleID := opts.BundleID == ""
	missingSKU := opts.SKU == ""
	if missingName || missingBundleID || missingSKU {
		if !appCreateCanPromptForFieldsFn() {
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
			return shared.WithDiagnostic(
				shared.UsageError(fmt.Sprintf("missing required flags: %s", strings.Join(missingFlags, ", "))),
				shared.DiagnosticRequiredInputMissing,
				soleMissingFlag(missingFlags),
			)
		}
		if err := promptAppsCreateFields(&opts); err != nil {
			return err
		}
	}

	opts = normalizeAppsCreateRunOptions(opts)

	var ifExistsClient *asc.Client
	if opts.IfExists == shared.IfExistsSkip {
		ifExistsClient, err = shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("web apps create failed: --if-exists skip requires official App Store Connect API authentication for the read-back: %w", err)
		}
	}

	var accessClient *asc.Client
	if access != "" {
		accessClient, err = shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("web apps create failed: --access requires official App Store Connect API authentication: %w", err)
		}
		if len(userIDs) > 0 {
			lookupCtx, lookupCancel := shared.ContextWithTimeout(ctx)
			defer lookupCancel()
			if lookupErr := withWebSpinner("Checking users", func() error {
				return ensureAppCreateUsersExist(lookupCtx, accessClient, userIDs)
			}); lookupErr != nil {
				return lookupErr
			}
		} else {
			probeCtx, probeCancel := shared.ContextWithTimeout(ctx)
			defer probeCancel()
			if probeErr := withWebSpinner("Checking App Store Connect API access", func() error {
				return ensureAppCreateAPIAccess(probeCtx, accessClient)
			}); probeErr != nil {
				return fmt.Errorf("web apps create failed: --access requires working App Store Connect API authentication: %w", probeErr)
			}
		}
	}

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
		"",
		"",
		opts.TwoFactorCodeCommand,
	)
	if err != nil {
		return err
	}

	requestCtx, cancel := newWebRequestContext(ctx)
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

	createdBundleID, preflightErr := withWebSpinnerValue("Checking or creating Bundle ID", func() (bool, error) {
		return ensureBundleIDFn(requestCtx, opts.BundleID, opts.Name, opts.Platform)
	})
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

	app, err := withWebSpinnerValue("Creating app via Apple web API", func() (*webcore.AppResponse, error) {
		return createWebAppFn(requestCtx, client, attrs)
	})
	// --if-exists skip runs before --auto-rename: a 409 for an app this
	// account already owns must never be retried under a new name. Only when
	// the read-back finds neither the bundle ID nor the SKU on this account
	// does the historical rename path below run.
	conflictHandled := false
	if err != nil && ifExistsClient != nil {
		existing, handled, resolveErr := resolveWebAppCreateConflict(ctx, ifExistsClient, opts, err)
		if handled {
			conflictHandled = true
			if resolveErr == nil {
				fmt.Fprintf(os.Stderr, "web apps create: app %s already exists with bundle ID %q; left unchanged (--if-exists skip)\n", existing.ID, existing.BundleID)
				return shared.PrintOutput(existing, opts.Output, opts.Pretty)
			}
			err = resolveErr
		}
	}
	if err != nil && !conflictHandled && opts.AutoRename && webcore.IsDuplicateAppNameError(err) {
		suffix := bundleIDNameSuffix(opts.BundleID)
		if suffix != "" {
			for i := 0; i < appCreateAutoRenameAttempts; i++ {
				tryName := autoRenameCandidate(opts.Name, suffix, i)
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
		return explainAppsCreateError(err)
	}

	fmt.Fprintf(os.Stderr, "Created app successfully (id=%s)\n", strings.TrimSpace(app.Data.ID))
	if access == "" {
		row := []string{app.Data.ID, attrs.Name, attrs.BundleID, attrs.SKU}
		return shared.PrintOutputRows(app, opts.Output, opts.Pretty, []string{"ID", "Name", "Bundle ID", "SKU"}, [][]string{row})
	}

	accessCtx, accessCancel := shared.ContextWithTimeout(ctx)
	defer accessCancel()
	receipt, applyErr := withWebSpinnerValue("Applying app access", func() (*asc.WebAppCreateResult, error) {
		return applyAndReadAppCreateAccess(accessCtx, accessClient, strings.TrimSpace(app.Data.ID), access, userIDs)
	})
	if applyErr != nil {
		return applyErr
	}
	return shared.PrintOutput(receipt, opts.Output, opts.Pretty)
}
