package web

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	loadSigninBackoffFn   = webcore.LoadSigninBackoff
	recordSigninBackoffFn = webcore.RecordSigninBackoff
	clearSigninBackoffFn  = webcore.ClearSigninBackoff
)

// webAutoSignInDisabled reports whether ASC_WEB_NO_AUTO_LOGIN restricts every
// web command other than `asc web auth login` to the cached session. An
// unrecognized value is a usage error rather than a silent default.
func webAutoSignInDisabled() (bool, error) {
	value := strings.TrimSpace(os.Getenv(webNoAutoLoginEnv))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, nil
	case "", "0", "false", "no", "off":
		return false, nil
	default:
		return false, shared.UsageError(fmt.Sprintf("invalid %s value %q (expected true/false, 1/0, yes/no, or on/off)", webNoAutoLoginEnv, value))
	}
}

// newAutoSignInDisabledError reports a missing or expired session for a web
// command that ASC_WEB_NO_AUTO_LOGIN forbids from signing in. It keeps the
// missing-session exit code and classification, with a hint naming the policy.
func newAutoSignInDisabledError(appleID, alternative string) error {
	account := "EMAIL"
	if trimmed := strings.TrimSpace(appleID); trimmed != "" {
		account = shared.SanitizeTerminal(trimmed)
	}
	hint := fmt.Sprintf(
		"The cached web session is missing or expired, and %s is set, so this command does not sign in to Apple automatically. Run 'asc web auth login --apple-id %s' to sign in, then retry.",
		webNoAutoLoginEnv,
		account,
	)
	return missingWebSessionErrorWithHint(appleID, hint, alternative)
}

// errSigninBackoffActive marks a refusal to sign in automatically while a
// recorded Apple sign-in 5xx cooldown is still running.
var errSigninBackoffActive = errors.New("automatic web sign-in is paused after an Apple sign-in failure")

// checkSigninBackoff refuses an automatic sign-in for appleID while a recorded
// cooldown is active, before anything is sent to Apple. An unreadable record
// only warns: the cooldown protects Apple's limit, not the user's data.
func checkSigninBackoff(appleID string) error {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return nil
	}
	backoff, ok, err := loadSigninBackoffFn(appleID)
	if err != nil {
		printSigninBackoffWarning(fmt.Errorf("reading the sign-in backoff record failed: %w", err))
		return nil
	}
	if !ok {
		return nil
	}
	now := signinBackoffNowFn()
	notBefore, active := backoff.ActiveUntil(now)
	if !active {
		return nil
	}
	if backoff.Locked {
		err = fmt.Errorf(
			"%w: Apple reported %s locked for API sign-in at %s, so asc will not sign in automatically again until %s (in %s). %s",
			errSigninBackoffActive,
			shared.SanitizeTerminal(appleID),
			backoff.FailedAt.UTC().Format(time.RFC3339),
			notBefore.UTC().Format(time.RFC3339),
			notBefore.Sub(now).Round(time.Second),
			appleAccountLockedHint(appleID),
		)
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "")
	}
	status := "a server error"
	if backoff.Status > 0 {
		status = fmt.Sprintf("HTTP %d", backoff.Status)
	}
	err = fmt.Errorf(
		"%w: Apple's sign-in service answered %s for %s at %s, so asc will not sign in automatically again until %s (in %s); every attempt counts toward Apple's limit. To sign in now anyway, run `asc web auth login --apple-id %q`",
		errSigninBackoffActive,
		status,
		shared.SanitizeTerminal(appleID),
		backoff.FailedAt.UTC().Format(time.RFC3339),
		notBefore.UTC().Format(time.RFC3339),
		notBefore.Sub(now).Round(time.Second),
		appleID,
	)
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "")
}

// appleAccountLockedHint tells the account owner how to clear Apple's API
// sign-in lock (#3111). A browser sign-in to App Store Connect can keep
// working while the lock stands, so the hint says so.
func appleAccountLockedHint(appleID string) string {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		appleID = "EMAIL"
	}
	return fmt.Sprintf("Apple has locked this Apple Account for security reasons on the sign-in path asc uses; signing in to App Store Connect in a browser can still work and does not clear the lock. Unlock the account at https://iforgot.apple.com (usually by resetting the password), then run `asc web auth login --apple-id %q`", appleID)
}

// recordSigninBackoff remembers that Apple's sign-in service answered appleID
// with a 5xx, or reported the account locked, and returns when automatic
// sign-in may resume. A write failure
// only warns: the sign-in error itself is still reported.
func recordSigninBackoff(appleID string, serviceErr *webcore.SigninServiceError, observedAt time.Time) (time.Time, bool) {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" || serviceErr == nil {
		return time.Time{}, false
	}
	backoff := webcore.NewSigninBackoff(serviceErr, observedAt)
	if !backoff.Active() {
		// Apple allowed an immediate retry, so no earlier pause applies either.
		clearSigninBackoff(appleID, observedAt)
		return time.Time{}, false
	}
	recorded, err := recordSigninBackoffFn(appleID, backoff, signinBackoffNowFn())
	if err != nil {
		printSigninBackoffWarning(fmt.Errorf("recording the sign-in backoff failed: %w", err))
		return time.Time{}, false
	}
	return backoff.NotBefore, recorded
}

// clearSigninBackoff drops any cooldown after Apple accepted a sign-in.
func clearSigninBackoff(appleID string, succeededAt time.Time) {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return
	}
	if err := clearSigninBackoffFn(appleID, succeededAt, signinBackoffNowFn()); err != nil {
		printSigninBackoffWarning(fmt.Errorf("clearing the sign-in backoff record failed: %w", err))
	}
}

func printSigninBackoffWarning(err error) {
	if sessionCacheWarningWriter == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(sessionCacheWarningWriter, "Warning: %v.\n", err)
}
