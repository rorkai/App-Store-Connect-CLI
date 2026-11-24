package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var errNoCachedWebSession = errors.New("no cached web session is available")

type webSessionFlags struct {
	flagSet              *flag.FlagSet
	appleID              *string
	twoFactorCodeCommand *string
	twoFactorDelivery    twoFactorDeliveryFlags
	providerID           *int64
	publicProviderID     *string
	sessionFromEnv       *bool
}

func bindWebSessionFlags(fs *flag.FlagSet) webSessionFlags {
	return webSessionFlags{
		flagSet:              fs,
		appleID:              fs.String("apple-id", "", "Apple Account email used to scope a user-owned session cache (defaults to "+webAppleIDEnv+", then the last or only cached session)"),
		twoFactorCodeCommand: fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required"),
		twoFactorDelivery:    bindTwoFactorDeliveryFlags(fs),
		providerID:           fs.Int64("provider-id", 0, "Numeric App Store Connect provider ID to select for this web session (refused under --read-only or --dry-run when it names another provider)"),
		publicProviderID:     fs.String("public-provider-id", "", "Public App Store Connect provider/team ID to select for this web session (refused under --read-only or --dry-run when it names another provider)"),
	}
}

func bindWebSessionFlagsWithSessionFromEnv(fs *flag.FlagSet) webSessionFlags {
	flags := bindWebSessionFlags(fs)
	flags.sessionFromEnv = fs.Bool("session-from-env", false, "Use ASC_WEB_SESSION in memory without persisting it to the session cache or keychain")
	return flags
}

// newWebRequestContextWith returns the bounded context for work that runs after
// web authentication finished. It is derived from the untimed parent context, so
// the timeout budget covers the request instead of the human in front of the
// prompt, while parent cancellation (Ctrl-C) still propagates.
func newWebRequestContextWith(ctx context.Context, d *shared.Deps) (context.Context, context.CancelFunc) {
	return d.Timeout(shared.ContextWithoutTimeout(ctx))
}

// resolveWebSessionForCommandWith resolves the web session for a command and
// returns the request context that command must use for its own work.
//
// Session resolution can block on an interactive password prompt, an interactive
// 2FA prompt, or a configured two-factor code command, none of which belong in a
// request timeout budget. Callers therefore pass their parent context and receive
// a request context whose timeout starts only once authentication is done; the
// returned cancel func is never nil, so it is safe to defer before checking err.
// The password and two-factor prompts still read and write the terminal itself.
func resolveWebSessionForCommandWith(ctx context.Context, d *shared.Deps, flags webSessionFlags) (*webcore.AuthSession, context.Context, context.CancelFunc, error) {
	if flags.sessionFromEnv != nil && *flags.sessionFromEnv {
		if err := refuseUnderWebBroker(d, "--session-from-env"); err != nil {
			requestCtx, cancel := newWebRequestContextWith(ctx, d)
			return nil, requestCtx, cancel, err
		}
		session, err := resolveSessionFromEnv(ctx, d, flags)
		requestCtx, cancel := newWebRequestContextWith(ctx, d)
		return session, requestCtx, cancel, err
	}
	preference, err := flags.twoFactorDelivery.resolveWith(d)
	if err != nil {
		requestCtx, cancel := newWebRequestContextWith(ctx, d)
		return nil, requestCtx, cancel, err
	}
	selection := providerSelectionFromFlags(flags)
	session, _, err := callResolveSessionForProviderSelectionWith(
		contextWithTwoFactorPreference(ctx, preference),
		d,
		*flags.appleID,
		"",
		"",
		*flags.twoFactorCodeCommand,
		selection,
	)
	requestCtx, cancel := newWebRequestContextWith(ctx, d)
	if err != nil {
		return nil, requestCtx, cancel, err
	}
	if err := selectResolvedWebSessionProviderWith(requestCtx, d, session, selection); err != nil {
		return nil, requestCtx, cancel, err
	}
	return session, requestCtx, cancel, nil
}

// providerSwitchRefusalSetting names the setting under which this command must
// not move the web session to another provider: read-only mode, or the
// command's own --dry-run. Signing in stays allowed under both.
func providerSwitchRefusalSetting(fs *flag.FlagSet) string {
	if readonly.Enabled() {
		return readonly.Source()
	}
	if fs != nil {
		if dryRun := fs.Lookup("dry-run"); dryRun != nil && dryRun.Value.String() == "true" {
			return "--dry-run"
		}
	}
	return ""
}

func providerSelectionFromFlags(flags webSessionFlags) webcore.ProviderSelection {
	var providerID int64
	if flags.providerID != nil {
		providerID = *flags.providerID
	}
	var publicProviderID string
	if flags.publicProviderID != nil {
		publicProviderID = *flags.publicProviderID
	}
	return webcore.ProviderSelection{
		ProviderID:        providerID,
		PublicProviderID:  publicProviderID,
		RefuseSwitchUnder: providerSwitchRefusalSetting(flags.flagSet),
	}
}

func withWebAuthHint(err error, operation string) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), operation+" failed:") {
		return err
	}
	// A refused provider switch already names the setting, both providers and
	// the command that switches; it is not a failure of the web session.
	if _, ok := errors.AsType[*webcore.ProviderSwitchRefusedError](err); ok {
		return err
	}
	if webcore.IsDeveloperPortalSessionError(err) {
		return fmt.Errorf("%s failed: %w", operation, err)
	}
	if apiErr, ok := errors.AsType[*webcore.APIError](err); ok && apiErr.Status == http.StatusForbidden {
		return fmt.Errorf("%s failed: Apple denied the request (403); the account role likely lacks permission for this action, or the web session expired (run 'asc web auth status' to check): %w", operation, err)
	}
	if webcore.IsSessionAuthFailure(err) {
		return fmt.Errorf("%s failed: web session is unauthorized or expired (%s): %w", operation, webSignInAdvice(), err)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}
