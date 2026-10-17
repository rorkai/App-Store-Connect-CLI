package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var errNoCachedWebSession = errors.New("no cached web session is available")

type webSessionFlags struct {
	flagSet              *flag.FlagSet
	appleID              *string
	twoFactorCodeCommand *string
	twoFactorMethod      *string
	twoFactorPhoneID     *int
	providerID           *int64
	publicProviderID     *string
	sessionFromEnv       *bool
}

func bindWebSessionFlags(fs *flag.FlagSet) webSessionFlags {
	return webSessionFlags{
		flagSet:              fs,
		appleID:              fs.String("apple-id", "", "Apple Account email used to scope a user-owned session cache (defaults to "+webAppleIDEnv+", then the last or only cached session)"),
		twoFactorCodeCommand: fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required"),
		twoFactorMethod:      fs.String("two-factor-method", "", "Two-factor verification delivery method: 'sms' (or 'phone') or 'trusted-device'"),
		twoFactorPhoneID:     fs.Int("two-factor-phone-id", 0, "Specific trusted phone number ID for SMS verification (defaults to "+webTwoFactorPhoneIDEnv+")"),
		providerID:           fs.Int64("provider-id", 0, "Numeric App Store Connect provider ID to select for this web session"),
		publicProviderID:     fs.String("public-provider-id", "", "Public App Store Connect provider/team ID to select for this web session"),
	}
}

func bindWebSessionFlagsWithSessionFromEnv(fs *flag.FlagSet) webSessionFlags {
	flags := bindWebSessionFlags(fs)
	flags.sessionFromEnv = fs.Bool("session-from-env", false, "Use ASC_WEB_SESSION in memory without persisting it to the session cache or keychain")
	return flags
}

// newWebRequestContext returns the bounded context for work that runs after web
// authentication finished. It is derived from the untimed parent context, so the
// timeout budget covers the request instead of the human in front of the prompt,
// while parent cancellation (Ctrl-C) still propagates.
func newWebRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
}

// resolveWebSessionForCommand resolves the web session for a command and returns
// the request context that command must use for its own work.
//
// Session resolution can block on an interactive password prompt, an interactive
// 2FA prompt, or a configured two-factor code command, none of which belong in a
// request timeout budget. Callers therefore pass their parent context and receive
// a request context whose timeout starts only once authentication is done; the
// returned cancel func is never nil, so it is safe to defer before checking err.
func resolveWebSessionForCommand(ctx context.Context, flags webSessionFlags) (*webcore.AuthSession, context.Context, context.CancelFunc, error) {
	if flags.sessionFromEnv != nil && *flags.sessionFromEnv {
		session, err := resolveSessionFromEnv(ctx, flags)
		requestCtx, cancel := newWebRequestContext(ctx)
		return session, requestCtx, cancel, err
	}
	var method string
	if flags.twoFactorMethod != nil {
		method = *flags.twoFactorMethod
	}
	var phoneID int
	if flags.twoFactorPhoneID != nil {
		phoneID = *flags.twoFactorPhoneID
	}
	ctx = contextWithTwoFactorOptions(ctx, method, phoneID)
	selection := providerSelectionFromFlags(flags)
	session, _, err := callResolveSessionForProviderSelection(
		ctx,
		*flags.appleID,
		"",
		"",
		*flags.twoFactorCodeCommand,
		selection,
	)
	requestCtx, cancel := newWebRequestContext(ctx)
	if err != nil {
		return nil, requestCtx, cancel, err
	}
	if err := selectResolvedWebSessionProvider(requestCtx, session, selection); err != nil {
		return nil, requestCtx, cancel, err
	}
	return session, requestCtx, cancel, nil
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
		ProviderID:       providerID,
		PublicProviderID: publicProviderID,
	}
}

func withWebAuthHint(err error, operation string) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), operation+" failed:") {
		return err
	}
	if webcore.IsSessionAuthFailure(err) {
		return fmt.Errorf("%s failed: web session is unauthorized or expired (run 'asc web auth login'): %w", operation, err)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}
