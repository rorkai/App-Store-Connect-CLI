package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/securitykey"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	webPasswordEnv             = "ASC_WEB_PASSWORD"
	webAppleIDEnv              = "ASC_WEB_APPLE_ID"
	webDontStorePasswordEnv    = "ASC_WEB_DONT_STORE_PASSWORD"
	webTwoFactorCodeCommandEnv = "ASC_WEB_2FA_CODE_COMMAND"
	webSecurityKeyPINEnv       = "ASC_WEB_SECURITY_KEY_PIN"
	// webNoAutoLoginEnv opts out of automatic sign-in: only `asc web auth
	// login` may sign in, and every other web command uses the cached session
	// or fails without contacting Apple.
	webNoAutoLoginEnv = "ASC_WEB_NO_AUTO_LOGIN"
	// webTwoFactorCodeRequestedAtEnv tells the 2FA code command when asc asked
	// Apple for the current code, so it can ignore codes from earlier attempts.
	webTwoFactorCodeRequestedAtEnv = "ASC_WEB_2FA_CODE_REQUESTED_AT"
	webTwoFactorCommandTimeout     = 60 * time.Second
	passwordReadPollInterval       = 100 * time.Millisecond
)

func webPasswordEnvDisplay() string {
	return webPasswordEnv
}

func webPasswordEnvAssignmentExample() string {
	return webPasswordEnvDisplay() + "=\"...\""
}

var (
	openTTYFn                      = openTTY
	promptTwoFactorCodeFn          = promptTwoFactorCodeInteractive
	twoFactorPromptAvailableFn     = terminalPromptAvailable
	promptPasswordFn               = promptPasswordInteractive
	readTwoFactorCodeFromCommandFn = readTwoFactorCodeFromCommand
	webLoginFn                     = webcore.Login
	prepareTwoFactorChallengeFn    = webcore.PrepareTwoFactorChallenge
	ensureTwoFactorCodeRequestedFn = webcore.EnsureTwoFactorCodeRequested
	persistWebSessionFn            = webcore.PersistSession
	submitTwoFactorCodeFn          = webcore.SubmitTwoFactorCode
	submitSecurityKeyFn            = webcore.SubmitSecurityKey
	checkSecurityKeyFn             = securitykey.Check
	securityKeyAvailableFn         = securitykey.Available
	errSecurityKeyPreflight        = errors.New("security-key preflight failed")
	signalProcessInterruptFn       = signalProcessInterrupt
	termReadPasswordFn             = term.ReadPassword
	termIsTerminalFn               = term.IsTerminal
	tryResumeSessionFn             = webcore.TryResumeSession
	tryResumeSessionFromSourceFn   = webcore.TryResumeSessionFromSource
	tryResumeLastFn                = webcore.TryResumeLastSession
	loadCachedSessionFn            = webcore.LoadCachedSession
	loadCachedSessionFromSourceFn  = webcore.LoadCachedSessionFromSource
	loadLastCachedSessionFn        = webcore.LoadLastCachedSession
	defaultCachedAppleIDFn         = webcore.DefaultCachedAppleIDWithSource
	webLoginWithClientFn           = webcore.LoginWithClient
	loadStoredWebPasswordFn        = webcore.LoadPassword
	storeStoredWebPasswordFn       = webcore.StorePassword
	storedWebPasswordExistsFn      = webcore.PasswordStored
	deleteStoredWebPasswordFn      = webcore.DeletePassword
	deleteAllStoredWebPasswordsFn  = webcore.DeleteAllPasswords
	deleteWebSessionFn             = webcore.DeleteSession
	deleteStaleWebSessionFn        = webcore.DeleteSessionIfMatches
	deleteAllWebSessionsFn         = webcore.DeleteAllSessions
	selectWebProviderFn            = webcore.SelectProvider
	// resolveSessionFn and resolveSessionWithoutPersistFn replace the session
	// resolvers in tests; nil runs the real resolver on the command's d.
	resolveSessionFn               any
	resolveSessionWithoutPersistFn any
	signinBackoffNowFn             = time.Now
	invalidWebPasswordOptOutMu     sync.Mutex
	invalidWebPasswordOptOutValues = map[string]struct{}{}
)

func callSessionResolverHook(ctx context.Context, hook any, hookName, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	switch fn := hook.(type) {
	case func(context.Context, string, string, string) (*webcore.AuthSession, string, error):
		if strings.TrimSpace(twoFactorCodeCommand) != "" {
			return nil, "", fmt.Errorf("internal error: %s test hook cannot accept --two-factor-code-command", hookName)
		}
		return fn(ctx, appleID, password, twoFactorCode)
	case func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error):
		return fn(ctx, appleID, password, twoFactorCode, twoFactorCodeCommand)
	case func(context.Context, string, string, string, ...string) (*webcore.AuthSession, string, error):
		return fn(ctx, appleID, password, twoFactorCode, twoFactorCodeCommand)
	default:
		return nil, "", fmt.Errorf("internal error: unsupported %s type %T", hookName, hook)
	}
}

func webPasswordProvided(password string) bool {
	return strings.TrimSpace(password) != ""
}

func webPasswordStorageDisabled(d *shared.Deps) bool {
	if webPasswordStorageOptedOut(d) {
		return true
	}
	return webcore.PasswordStoreBypassed()
}

func webPasswordStorageOptedOut(d *shared.Deps) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(webDontStorePasswordEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "", "0", "false", "no", "off":
		return false
	default:
		warnInvalidWebPasswordOptOutOnce(d, value)
		return true
	}
}

func warnInvalidWebPasswordOptOutOnce(d *shared.Deps, value string) {
	invalidWebPasswordOptOutMu.Lock()
	if _, warned := invalidWebPasswordOptOutValues[value]; warned {
		invalidWebPasswordOptOutMu.Unlock()
		return
	}
	invalidWebPasswordOptOutValues[value] = struct{}{}
	invalidWebPasswordOptOutMu.Unlock()

	if d.IO.ErrOut != nil {
		_, _ = fmt.Fprintf(
			d.IO.ErrOut,
			"Warning: invalid %s value %q (expected true/false, 1/0, yes/no, or on/off); password storage disabled.\n",
			webDontStorePasswordEnv,
			value,
		)
	}
}

func resetInvalidWebPasswordOptOutWarnings() {
	invalidWebPasswordOptOutMu.Lock()
	defer invalidWebPasswordOptOutMu.Unlock()
	invalidWebPasswordOptOutValues = map[string]struct{}{}
}

type webPasswordSource string

const (
	webPasswordSourceProvided webPasswordSource = "provided"
	webPasswordSourceEnv      webPasswordSource = "environment"
	webPasswordSourceStored   webPasswordSource = "stored"
	webPasswordSourcePrompted webPasswordSource = "prompted"
)

type resolvedWebPassword struct {
	value  string
	source webPasswordSource
}

func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

type webAuthStatus struct {
	Authenticated    bool   `json:"authenticated"`
	PasswordStored   bool   `json:"passwordStored"`
	Source           string `json:"source,omitempty"`
	AppleID          string `json:"appleId,omitempty"`
	TeamID           string `json:"teamId,omitempty"`
	ProviderID       int64  `json:"providerId,omitempty"`
	PublicProviderID string `json:"publicProviderId,omitempty"`
	DeveloperTeamID  string `json:"developerTeamId,omitempty"`
	// AutoSignIn is set when there is no usable session: whether the next asc
	// web command signs in by itself with the stored password.
	AutoSignIn            *bool  `json:"autoSignIn,omitempty"`
	AutoSignInPausedUntil string `json:"autoSignInPausedUntil,omitempty"`
}

func printWebAuthStatus(d *shared.Deps, status webAuthStatus, format string, pretty bool) error {
	providerID := ""
	if status.ProviderID != 0 {
		providerID = strconv.FormatInt(status.ProviderID, 10)
	}
	rows := [][]string{
		{"Authenticated", strconv.FormatBool(status.Authenticated)},
		{"Password Stored", strconv.FormatBool(status.PasswordStored)},
		{"Source", status.Source},
		{"Apple ID", status.AppleID},
		{"Team ID", status.TeamID},
		{"Provider ID", providerID},
		{"Public Provider ID", status.PublicProviderID},
		{"Developer Team ID", status.DeveloperTeamID},
	}
	if status.AutoSignIn != nil {
		rows = append(rows, []string{"Auto Sign-In", strconv.FormatBool(*status.AutoSignIn)})
	}
	if status.AutoSignInPausedUntil != "" {
		rows = append(rows, []string{"Auto Sign-In Paused Until", status.AutoSignInPausedUntil})
	}
	return d.PrintOutputRows(status, format, pretty, []string{"Field", "Value"}, rows)
}

func expiredWebAuthStatus(d *shared.Deps, appleID string) (webAuthStatus, *webcore.AuthSession) {
	status := webAuthStatus{
		Authenticated: false,
		AppleID:       strings.TrimSpace(appleID),
	}
	cached, ok, err := loadExpiredWebAuthCache(status.AppleID)
	if err != nil || !ok {
		cached = nil
	}
	if cached != nil {
		if status.AppleID == "" {
			status.AppleID = strings.TrimSpace(cached.UserEmail)
		}
		status.DeveloperTeamID = strings.TrimSpace(cached.DeveloperTeamID)
	}
	status.PasswordStored = storedWebPasswordStatus(d, status.AppleID)
	return status, cached
}

func loadExpiredWebAuthCache(appleID string) (*webcore.AuthSession, bool, error) {
	if appleID != "" {
		return loadCachedSessionFn(appleID)
	}
	return loadLastCachedSessionFn()
}

func signalProcessInterrupt() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return process.Signal(os.Interrupt)
}

func callResolveSessionFn(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	if resolveSessionFn == nil {
		return resolveSession(ctx, d, appleID, password, twoFactorCode, twoFactorCodeCommand)
	}
	return callSessionResolverHook(ctx, resolveSessionFn, "web session resolver", appleID, password, twoFactorCode, twoFactorCodeCommand)
}

func readPasswordFromInput(ctx context.Context) (string, error) {
	password := readPasswordFromEnv()
	if webPasswordProvided(password) {
		return password, nil
	}
	if webPromptsDisabled(ctx) {
		return "", nil
	}
	password, err := promptPasswordFn(ctx)
	if err != nil {
		return "", err
	}
	if !webPasswordProvided(password) {
		return "", nil
	}
	return password, nil
}

func readPasswordFromEnv() string {
	return os.Getenv(webPasswordEnv)
}

func readPasswordFromTerminal(ctx context.Context, terminal *os.File, writer io.Writer, closeTerminal bool) (string, error) {
	if terminal == nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	if closeTerminal {
		defer func() { _ = terminal.Close() }()
	}
	if writer == nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	if _, err := fmt.Fprint(writer, "Apple Account password: "); err != nil {
		return "", fmt.Errorf("password prompt unavailable")
	}

	oldState, err := term.MakeRaw(int(terminal.Fd()))
	if err != nil {
		_, _ = fmt.Fprintln(writer)
		return "", fmt.Errorf("failed to read password")
	}
	defer func() {
		_ = term.Restore(int(terminal.Fd()), oldState)
		_, _ = fmt.Fprintln(writer)
	}()

	passwordBytes := make([]byte, 0, 64)
	readBuf := make([]byte, 1)
	for {
		n, err := readTerminalByteWithContext(ctx, terminal, readBuf)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", fmt.Errorf("password prompt interrupted: %w", err)
			}
			return "", fmt.Errorf("failed to read password")
		}
		if n == 0 {
			continue
		}

		switch readBuf[0] {
		case '\r', '\n':
			password := string(passwordBytes)
			if !webPasswordProvided(password) {
				return "", fmt.Errorf("password is required")
			}
			return password, nil
		case 3:
			// Raw mode consumes VINTR as a byte, so re-emit SIGINT to preserve
			// top-level cancellation behavior for the rest of the CLI lifecycle.
			_ = signalProcessInterruptFn()
			return "", fmt.Errorf("password prompt interrupted: %w", context.Canceled)
		case 4:
			if len(passwordBytes) == 0 {
				return "", fmt.Errorf("password prompt interrupted: %w", context.Canceled)
			}
			password := string(passwordBytes)
			if !webPasswordProvided(password) {
				return "", fmt.Errorf("password is required")
			}
			return password, nil
		case 8, 127:
			if len(passwordBytes) > 0 {
				passwordBytes = passwordBytes[:len(passwordBytes)-1]
			}
		default:
			passwordBytes = append(passwordBytes, readBuf[0])
		}
	}
}

func promptPasswordInteractive(ctx context.Context) (string, error) {
	if tty, err := openTTYFn(); err == nil {
		return readPasswordFromTerminal(ctx, tty, tty, true)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readPasswordFromTerminal(ctx, os.Stdin, os.Stderr, false)
	}
	return "", nil
}

func readTwoFactorCodeFromTerminalFD(fd int, writer io.Writer) (string, error) {
	if writer == nil {
		return "", fmt.Errorf("2fa required: unable to prompt for code")
	}
	if _, err := fmt.Fprint(writer, "Two-factor code required. Enter 2FA code: "); err != nil {
		return "", fmt.Errorf("2fa required: unable to prompt for code")
	}
	codeBytes, err := termReadPasswordFn(fd)
	_, _ = fmt.Fprintln(writer)
	if err != nil {
		return "", fmt.Errorf("2fa required: failed to read 2fa code")
	}
	code := strings.TrimSpace(string(codeBytes))
	if code == "" {
		return "", fmt.Errorf("2fa required: empty 2fa code")
	}
	return code, nil
}

var errNoTwoFactorCodeSource = errors.New("2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, or set " + webTwoFactorCodeCommandEnv)

func promptTwoFactorCodeInteractive() (string, error) {
	if tty, err := openTTYFn(); err == nil {
		defer func() { _ = tty.Close() }()
		return readTwoFactorCodeFromTerminalFD(int(tty.Fd()), tty)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readTwoFactorCodeFromTerminalFD(int(os.Stdin.Fd()), os.Stderr)
	}
	return "", errNoTwoFactorCodeSource
}

func terminalPromptAvailable() bool {
	if tty, err := openTTYFn(); err == nil {
		_ = tty.Close()
		return true
	}
	return termIsTerminalFn(int(os.Stdin.Fd()))
}

func twoFactorInputError(err error, code shared.DiagnosticCode) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), code, "--two-factor-code-command")
}

type twoFactorCodeRequestedAtContextKey struct{}

// contextWithTwoFactorCodeRequestedAt records when asc asked Apple for the code
// the next 2FA code command run should return.
func contextWithTwoFactorCodeRequestedAt(ctx context.Context, requestedAt time.Time) context.Context {
	return context.WithValue(ctx, twoFactorCodeRequestedAtContextKey{}, requestedAt)
}

func twoFactorCodeRequestedAt(ctx context.Context) (time.Time, bool) {
	requestedAt, ok := ctx.Value(twoFactorCodeRequestedAtContextKey{}).(time.Time)
	return requestedAt, ok
}

func readTwoFactorCodeFromCommand(ctx context.Context, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("2fa required: empty 2fa code command")
	}
	// Read before the timeout strip below, which drops layered context values.
	requestedAt, hasRequestedAt := twoFactorCodeRequestedAt(ctx)

	commandCtx, cancel := shared.ContextWithResolvedTimeout(shared.ContextWithoutTimeout(ctx), webTwoFactorCommandTimeout)
	defer cancel()
	cmd := shared.ShellCommand(commandCtx, command)

	if hasRequestedAt {
		cmd.Env = append(os.Environ(), webTwoFactorCodeRequestedAtEnv+"="+requestedAt.UTC().Format(time.RFC3339Nano))
	}
	output, err := cmd.Output()
	if err != nil {
		if ctxErr := commandCtx.Err(); ctxErr != nil {
			return "", fmt.Errorf("2fa required: two-factor code command interrupted: %w", ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return "", fmt.Errorf("2fa required: two-factor code command failed: %s", stderr)
			}
		}
		return "", fmt.Errorf("2fa required: two-factor code command failed: %w", err)
	}

	code := strings.TrimSpace(string(output))
	if code == "" {
		return "", fmt.Errorf("2fa required: two-factor code command returned empty output")
	}
	return code, nil
}

// A one-time code stays valid for the rest of its generator's time window, so
// a TOTP-style --two-factor-code-command keeps printing the digits that the
// failed attempt already burned. Bound how long the stale-session retry waits
// for the command to roll over to a new value.
var (
	webTwoFactorCodeRotationTimeout      = 35 * time.Second
	webTwoFactorCodeRotationPollInterval = 2 * time.Second
)

// twoFactorCodeCommandReader fetches a 2FA code from the configured command.
// resolveWebSession supplies its own reader so it can remember which value the
// failed attempt consumed and refuse to hand the same one back on the retry.
type twoFactorCodeCommandReader func(ctx context.Context, command string) (string, error)

// readRotatedTwoFactorCodeFromCommand re-runs the configured 2FA code command
// until it prints a value other than the one Apple already consumed. Nothing is
// retried until a code is known to be consumed, so the ordinary first read keeps
// the plain per-command timeout.
//
// The rotation deadline bounds the whole retry, not just the gaps between polls:
// every poll inherits it, so a slow or blocking command cannot stretch the wait
// by its own timeout (60s by default, and larger under an ASC_TIMEOUT override).
// The wait honours caller cancellation and never falls back to a prompt, so a
// scripted invocation still terminates on its own.
func readRotatedTwoFactorCodeFromCommand(ctx context.Context, command, consumed string) (string, error) {
	if consumed == "" {
		return readTwoFactorCodeFromCommandFn(ctx, command)
	}

	// Derive the budget from the untimed parent so the caller's cancellation
	// still lands while the login deadline, which the 2FA steps re-derive
	// anyway, cannot cut the rotation wait short. Cancellation is then read from
	// that same parent: the budget outlives the login deadline by design, so by
	// the time the window closes the login context has normally expired, and
	// reading it would report every ordinary exhaustion as an interruption.
	waitCtx := shared.ContextWithoutTimeout(ctx)
	rotationCtx, cancel := context.WithTimeout(waitCtx, webTwoFactorCodeRotationTimeout)
	defer cancel()
	// Stripping the timeout also drops the code request time layered above it.
	if requestedAt, ok := twoFactorCodeRequestedAt(ctx); ok {
		rotationCtx = contextWithTwoFactorCodeRequestedAt(rotationCtx, requestedAt)
	}
	rotationExhausted := func() error {
		return fmt.Errorf("2fa required: the configured two-factor code command did not produce a code other than the one the expired session already consumed within %s: wait for it to produce a new code, then re-run", webTwoFactorCodeRotationTimeout)
	}
	for {
		code, err := readTwoFactorCodeFromCommandFn(rotationCtx, command)
		if err != nil {
			// Only the rotation budget expiring is ours to explain; a cancelled
			// caller or a genuinely failing command keeps its own error.
			if rotationCtx.Err() != nil && waitCtx.Err() == nil {
				return "", rotationExhausted()
			}
			return "", err
		}
		if code != consumed {
			return code, nil
		}
		timer := time.NewTimer(webTwoFactorCodeRotationPollInterval)
		select {
		case <-rotationCtx.Done():
			timer.Stop()
			if waitCtx.Err() != nil {
				return "", fmt.Errorf("2fa required: waiting for the two-factor code command to produce a new code was interrupted: %w", waitCtx.Err())
			}
			return "", rotationExhausted()
		case <-timer.C:
		}
	}
}

func printExpiredSessionNotice(writer io.Writer) {
	if writer == nil {
		return
	}
	_, _ = fmt.Fprintln(writer, "Session expired.")
}

// staleSessionDiscardWarning wraps a failed discard of a proven-stale cached
// session for the retry paths, where the fresh login can still overwrite the
// cached entry and the failure is therefore a warning rather than a hard error.
func staleSessionDiscardWarning(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("discarding the stale cached web session failed: %w", err)
}

type (
	webSignInContextKey   struct{}
	webNoPromptContextKey struct{}
)

func webPromptsDisabled(ctx context.Context) bool {
	disabled, _ := ctx.Value(webNoPromptContextKey{}).(bool)
	return disabled
}

// consumedCodeNoticeContextKey marks a fresh sign-in that replaces one whose
// verification code was already consumed.
type consumedCodeNoticeContextKey struct{}

type publicAPIAlternativeContextKey struct{}

// contextForWebSignIn marks ctx as belonging to `asc web auth login`. That
// command exists to create the session, so a missing Apple Account or password
// stays an ordinary usage error with its usage page for it; every other web
// command reports the missing session as shared.MissingWebSessionError, which
// keeps the usage exit code but prints a sign-in hint instead of the page.
func contextForWebSignIn(ctx context.Context) context.Context {
	return context.WithValue(ctx, webSignInContextKey{}, true)
}

func isWebSignIn(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	signIn, _ := ctx.Value(webSignInContextKey{}).(bool)
	return signIn
}

// securityKeyPINForSignIn permits a PIN only for an explicit auth login. Call
// before sending credentials when key preference is already known, and again
// when Apple reveals a key challenge for an automatic delivery preference.
func securityKeyPINForSignIn(ctx context.Context, preference appleauth.TwoFactorPreference) (string, error) {
	if isWebSignIn(ctx) {
		return os.Getenv(webSecurityKeyPINEnv), nil
	}
	if preference.Delivery == appleauth.TwoFactorMethodSecurityKey && os.Getenv(webSecurityKeyPINEnv) != "" {
		return "", fmt.Errorf("automatic security-key sign-in does not use ASC_WEB_SECURITY_KEY_PIN; run 'asc web auth login --two-factor-method security-key' to sign in explicitly")
	}
	return "", nil
}

// securityKeyPreflight stops a key sign-in this invocation cannot complete,
// before a password is requested or sent. Failures are local and say nothing
// about the cached cookie jar.
func securityKeyPreflight(ctx context.Context, d *shared.Deps, preference appleauth.TwoFactorPreference) error {
	if _, err := securityKeyPINForSignIn(ctx, preference); err != nil {
		return fmt.Errorf("%w: %w", errSecurityKeyPreflight, err)
	}
	if preference.Delivery != appleauth.TwoFactorMethodSecurityKey {
		return nil
	}
	if err := checkSecurityKeyFn(ctx); err != nil {
		if errors.Is(err, securitykey.ErrUnavailable) {
			err = d.UsageError(err.Error())
		}
		return fmt.Errorf("%w: %w", errSecurityKeyPreflight, err)
	}
	return nil
}

// contextWithPublicAPIAlternative records a sentence naming the App Store
// Connect API command that answers the same question without a web session, so
// a missing-session error can point at it.
func contextWithPublicAPIAlternative(ctx context.Context, sentence string) context.Context {
	return context.WithValue(ctx, publicAPIAlternativeContextKey{}, strings.TrimSpace(sentence))
}

func publicAPIAlternativeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sentence, _ := ctx.Value(publicAPIAlternativeContextKey{}).(string)
	return sentence
}

// newMissingWebSessionError reports that a web command has no usable Apple web
// session and no way to sign in without a terminal. appleID names the selected
// account, if any. Without one the error also carries errNoCachedWebSession so
// command-specific session diagnostics can tell a missing session from an
// expired one, and the required_input_missing --apple-id diagnostic the usage
// error it replaced reported. alternative, when set, is appended to the hint.
func newMissingWebSessionError(appleID, alternative string) error {
	hint := fmt.Sprintf(
		"asc web commands need a signed-in Apple Account session, and signing in needs an interactive terminal for the password and two-factor code. Run 'asc web auth login --apple-id EMAIL' in a terminal, or load a session exported elsewhere with 'asc web auth import --file FILE'. Unattended sign-in needs %s and %s, plus --apple-id or %s.",
		webPasswordEnvDisplay(),
		webTwoFactorCodeCommandEnv,
		webAppleIDEnv,
	)
	return missingWebSessionErrorWithHint(appleID, hint, alternative)
}

func missingWebSessionErrorWithHint(appleID, hint, alternative string) error {
	message := "no Apple web session is cached"
	trimmedAppleID := strings.TrimSpace(appleID)
	if trimmedAppleID != "" {
		message = "no usable Apple web session for " + shared.SanitizeTerminal(trimmedAppleID)
	}
	if alternative = strings.TrimSpace(alternative); alternative != "" {
		hint += " " + alternative
	}
	err := &shared.MissingWebSessionError{Message: message, Hint: hint}
	if trimmedAppleID == "" {
		return shared.NewErrorWithCause(
			shared.WithDiagnostic(err, shared.DiagnosticRequiredInputMissing, "--apple-id"),
			errNoCachedWebSession,
		)
	}
	return err
}

// passwordRequiredUsageErrorWith is the usage error for a sign-in that reached the
// password step without one: `asc web auth login` without a password source,
// or an interactive `web apps create` prompt answered with an empty password.
func passwordRequiredUsageErrorWith(d *shared.Deps) error {
	return d.UsageError(fmt.Sprintf("password is required: run in a terminal for an interactive prompt or set %s", webPasswordEnvDisplay()))
}

// missingAppleIDUsageError is the usage error `asc web auth login` returns
// when it has no Apple ID to sign in with: nothing was passed and the session
// cache holds nothing to default to. It carries errNoCachedWebSession so
// command-specific session diagnostics can tell a missing session from an
// expired one.
func missingAppleIDUsageError(d *shared.Deps) error {
	return shared.NewErrorWithCause(
		shared.WithDiagnostic(
			d.UsageError("--apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'"),
			shared.DiagnosticRequiredInputMissing,
			"--apple-id",
		),
		errNoCachedWebSession,
	)
}

// ambiguousAppleIDUsageError is the usage error for a web command that could
// default to any of several cached sessions and therefore refuses to guess.
func ambiguousAppleIDUsageError(d *shared.Deps, appleIDs []string) error {
	return shared.WithDiagnostic(
		d.UsageError(fmt.Sprintf("--apple-id is required: multiple cached web sessions are available (%s); pass --apple-id to choose one", strings.Join(appleIDs, ", "))),
		shared.DiagnosticRequiredInputMissing,
		"--apple-id",
	)
}

// envAppleID returns the Apple Account named by ASC_WEB_APPLE_ID, the
// environment fallback for --apple-id. The value is trimmed and an empty one is
// ignored, so an exported-but-unset CI secret behaves like no variable at all
// instead of selecting a nameless account.
func envAppleID() string {
	return strings.TrimSpace(os.Getenv(webAppleIDEnv))
}

// printEnvAppleIDNotice announces the account the environment selected. The
// cached-session default prints the same shape; naming the variable keeps a
// caller who exported it in one shell profile from mistaking the choice for a
// cache hit.
func printEnvAppleIDNotice(d *shared.Deps, appleID string) {
	if d.IO.ErrOut == nil {
		return
	}
	_, _ = fmt.Fprintf(d.IO.ErrOut, "Using web session for %s from %s; pass --apple-id to override\n", shared.SanitizeTerminal(appleID), webAppleIDEnv)
}

// resolveDefaultCachedAppleID picks the Apple ID of the only cached web session
// when --apple-id names none. The chosen account is announced on stderr so a
// caller who later caches several accounts can tell which one served the
// request. This discovery helper deliberately does not construct usage errors:
// UsageError writes to stderr, while an empty cache may still be recovered by
// an interactive prompt. A cache that cannot be listed degrades to the empty
// result with a warning.
// The boolean reports the empty-cache case, the only one a command may still
// answer with an interactive Apple ID prompt.
func resolveDefaultCachedAppleID(d *shared.Deps) (appleID string, source webcore.CachedSessionSource, cacheEmpty bool, err error) {
	appleID, source, err = defaultCachedAppleIDFn()
	if err == nil {
		appleID = strings.TrimSpace(appleID)
		if appleID == "" {
			return "", webcore.CachedSessionSourceUnknown, true, webcore.ErrNoCachedSession
		}
		if d.IO.ErrOut != nil {
			_, _ = fmt.Fprintf(d.IO.ErrOut, "Using cached web session for %s; pass --apple-id to override\n", shared.SanitizeTerminal(appleID))
		}
		return appleID, source, false, nil
	}
	var ambiguous *webcore.AmbiguousCachedSessionError
	switch {
	case errors.As(err, &ambiguous):
		return "", source, false, ambiguous
	case errors.Is(err, webcore.ErrNoCachedSession):
		return "", source, true, webcore.ErrNoCachedSession
	default:
		if d.IO.ErrOut != nil {
			_, _ = fmt.Fprintf(d.IO.ErrOut, "Warning: listing cached web sessions failed: %v\n", err)
		}
		return "", source, true, webcore.ErrNoCachedSession
	}
}

func printCacheLookupWarning(writer io.Writer, err error) {
	if writer == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "Warning: %v; continuing with fresh login.\n", err)
}

func printPasswordStoreWarning(writer io.Writer, action string, err error) {
	if writer == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "Warning: could not %s saved Apple Account password: %v.\n", action, err)
}

func resolveNonInteractiveWebPassword(d *shared.Deps, appleID, provided string, skipStored bool) resolvedWebPassword {
	if webPasswordProvided(provided) {
		return resolvedWebPassword{value: provided, source: webPasswordSourceProvided}
	}
	if password := readPasswordFromEnv(); webPasswordProvided(password) {
		return resolvedWebPassword{value: password, source: webPasswordSourceEnv}
	}
	if skipStored || webPasswordStorageDisabled(d) || strings.TrimSpace(appleID) == "" {
		return resolvedWebPassword{}
	}
	password, ok, err := loadStoredWebPasswordFn(appleID)
	if err != nil {
		printPasswordStoreWarning(d.IO.ErrOut, "load", err)
		return resolvedWebPassword{}
	}
	if !ok || !webPasswordProvided(password) {
		return resolvedWebPassword{}
	}
	return resolvedWebPassword{value: password, source: webPasswordSourceStored}
}

func persistPromptedWebPassword(d *shared.Deps, appleID string, password resolvedWebPassword) {
	if password.source != webPasswordSourcePrompted || webPasswordStorageDisabled(d) {
		return
	}
	if err := storeStoredWebPasswordFn(appleID, password.value); err != nil {
		printPasswordStoreWarning(d.IO.ErrOut, "save", err)
	}
}

func webSignInNoticeWriter(d *shared.Deps) io.Writer {
	if twoFactorPromptAvailableFn() {
		return d.IO.OperatorStderr()
	}
	return d.IO.ErrOut
}

func forgetRejectedStoredPassword(d *shared.Deps, appleID string) {
	writer := webSignInNoticeWriter(d)
	if writer != nil {
		_, _ = fmt.Fprintln(writer, "Saved Apple Account password was rejected; removing it. Enter the current password to save a new one.")
	}
	if err := deleteStoredWebPasswordFn(appleID); err != nil {
		printPasswordStoreWarning(writer, "remove", err)
	}
}

func storedWebPasswordStatus(d *shared.Deps, appleID string) bool {
	if webPasswordStorageDisabled(d) || strings.TrimSpace(appleID) == "" {
		return false
	}
	stored, err := storedWebPasswordExistsFn(appleID)
	if err != nil {
		printPasswordStoreWarning(d.IO.ErrOut, "check", err)
		return false
	}
	return stored
}

// twoFactorSubmitFailure describes a failed 2FA submission. Apple accepting a
// code or key assertion and then failing session bootstrap is reported as a
// finalization failure carrying the HTTP status, because calling it a
// verification failure misleads users into retrying a code that was accepted.
func twoFactorSubmitFailure(err error, afterPhoneDelivery, smsCode bool) error {
	stage := "2fa verification failed"
	var finalizeErr *webcore.TwoFactorFinalizationError
	if errors.As(err, &finalizeErr) {
		stage = "2fa finalization failed"
	}
	if afterPhoneDelivery {
		stage += " after switching to phone delivery"
	}
	var keyRejected *webcore.SecurityKeyAssertionRejectedError
	if errors.As(err, &keyRejected) {
		err = fmt.Errorf("%s: Apple rejected the security-key assertion: %w", stage, err)
		return shared.WithDiagnostic(err, shared.DiagnosticAuthenticationRejected, "")
	}
	if !errors.Is(err, webcore.ErrTwoFactorCodeRejected) {
		return fmt.Errorf("%s: %w", stage, err)
	}
	// Apple can still show a code on trusted devices during SMS delivery,
	// but only the texted code verifies against the phone endpoint.
	reason := "Apple rejected the verification code"
	if smsCode {
		reason += "; enter the code from the latest SMS, not one shown on a trusted device"
	}
	err = fmt.Errorf("%s: %s: %w", stage, reason, err)
	return shared.WithDiagnostic(err, shared.DiagnosticAuthenticationRejected, "")
}

func loginWithOptionalTwoFactorUsing(ctx context.Context, d *shared.Deps, progressMessage, appleID, password, twoFactorCode string, loginFn func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error), twoFactorStarted func(), readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	// Read before the timeout strip below, which drops layered context values.
	twoFactorPreference, err := twoFactorPreferenceForSignIn(ctx, d)
	if err != nil {
		return nil, err
	}
	explicitSignIn := isWebSignIn(ctx)
	consumedCodeNotice, _ := ctx.Value(consumedCodeNoticeContextKey{}).(bool)
	// Credential/keychain prompts may consume the initial command budget.
	ctx, cancel := d.Timeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	// Keep explicit-login authority when stripping a caller's timeout values.
	if explicitSignIn {
		ctx = contextForWebSignIn(ctx)
	}
	if err := securityKeyPreflight(ctx, d, twoFactorPreference); err != nil {
		return nil, err
	}
	session, err := withWebSpinnerValueWith(d, progressMessage, func() (*webcore.AuthSession, error) {
		return loginFn(ctx, webcore.LoginCredentials{
			Username: appleID,
			Password: password,
		})
	})
	if err == nil {
		return session, nil
	}

	var tfaErr *webcore.TwoFactorRequiredError
	if session != nil && errors.As(err, &tfaErr) {
		if twoFactorStarted != nil {
			twoFactorStarted()
		}
		code := strings.TrimSpace(twoFactorCode)
		command := ""
		if len(twoFactorCodeCommand) > 0 {
			command = strings.TrimSpace(twoFactorCodeCommand[0])
		}
		// Stop before asking Apple for the challenge, which can deliver a code
		// to the account's devices or phone that nobody here can enter.
		if twoFactorPreference.Delivery != appleauth.TwoFactorMethodSecurityKey && code == "" && command == "" && (webPromptsDisabled(ctx) || !twoFactorPromptAvailableFn()) {
			return nil, twoFactorInputError(errNoTwoFactorCodeSource, shared.DiagnosticRequiredInputMissing)
		}
		session.SetTwoFactorPreference(twoFactorPreference)
		challenge, prepErr := prepareTwoFactorChallengeFn(ctx, session)
		if prepErr != nil {
			var phoneSelection *appleauth.TrustedPhoneSelectionError
			if errors.As(prepErr, &phoneSelection) {
				return nil, fmt.Errorf("2fa challenge setup failed: %w; pass --two-factor-phone-id or set %s", prepErr, webTwoFactorPhoneIDEnv)
			}
			return nil, fmt.Errorf("2fa challenge setup failed: %w", prepErr)
		}

		operatorStderr := d.IO.OperatorStderr()
		if challenge != nil && challenge.Method == appleauth.TwoFactorMethodSecurityKey {
			if twoFactorPreference.Delivery != appleauth.TwoFactorMethodSecurityKey && !securityKeyAvailableFn() {
				err := fmt.Errorf("security key required by Apple: %w; after installing a supported build, run 'asc web auth login --two-factor-method security-key' explicitly", securitykey.ErrUnavailable)
				return nil, shared.WithDiagnostic(err, shared.DiagnosticConflictingInput, "")
			}
			pin, err := securityKeyPINForSignIn(ctx, appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorMethodSecurityKey})
			if err != nil {
				return nil, err
			}
			if twoFactorPreference.Delivery != appleauth.TwoFactorMethodSecurityKey {
				err := errors.New("security key required by Apple; retry with --two-factor-method security-key")
				return nil, shared.WithDiagnostic(err, shared.DiagnosticConflictingInput, "")
			}
			if operatorStderr != nil {
				_, _ = fmt.Fprintln(operatorStderr, "Touch your security key to complete Apple sign-in.")
			}
			interruptCtx, stopInterrupt := signal.NotifyContext(shared.ContextWithoutTimeout(ctx), os.Interrupt)
			defer stopInterrupt()
			// Waiting for physical touch must not consume the HTTP budget used
			// to verify the assertion and establish the trusted session.
			verifyCtx, verifyCancel := shared.ContextWithTimeoutDuration(interruptCtx, securitykey.AssertionTimeout+d.ResolveTimeout(shared.RequestTimeoutSetting, asc.DefaultTimeout))
			defer verifyCancel()
			if err := submitSecurityKeyFn(verifyCtx, session, pin); err != nil {
				return nil, twoFactorSubmitFailure(err, false, false)
			}
			return session, nil
		}
		if consumedCodeNotice && operatorStderr != nil {
			_, _ = fmt.Fprintln(operatorStderr, "The previous verification code was consumed by the expired session. Enter the new code Apple just sent, not the previous one.")
		}

		writeDeliveryNotice := func(destination string) {
			destination = strings.TrimSpace(destination)
			stderr := operatorStderr
			if command != "" {
				stderr = d.IO.ErrOut
			}
			if destination == "" || stderr == nil {
				return
			}
			_, _ = fmt.Fprintf(stderr, "Verification code sent to %s.\n", destination)
		}
		writeFallbackGuidance := func(usingCommand bool) {
			if operatorStderr == nil {
				return
			}
			if usingCommand {
				_, _ = fmt.Fprintln(d.IO.ErrOut, "Trusted-device verification was rejected. Re-running the configured 2FA code command for the phone verification code.")
				return
			}
			_, _ = fmt.Fprintln(operatorStderr, "Trusted-device verification was rejected. Enter the phone verification code that was just sent.")
		}
		writeInitialPhoneFallbackGuidance := func() {
			if challenge == nil || !challenge.PhoneFallbackAvailable || operatorStderr == nil {
				return
			}
			destination := strings.TrimSpace(challenge.Destination)
			if destination != "" {
				_, _ = fmt.Fprintf(operatorStderr, "Need a phone verification code? Enter an incorrect trusted-device code once; Apple will then deliver a verification code to %s.\n", destination)
				return
			}
			_, _ = fmt.Fprintln(operatorStderr, "Need a phone verification code? Enter an incorrect trusted-device code once; Apple will then deliver a verification code to your registered phone number.")
		}
		codeRequestedAt := d.Now()
		readCode := func() (string, error) {
			if command != "" {
				read := readTwoFactorCodeFromCommandFn
				if readCommandCode != nil {
					read = readCommandCode
				}
				code, err := read(contextWithTwoFactorCodeRequestedAt(ctx, codeRequestedAt), command)
				if err != nil {
					return "", twoFactorInputError(err, shared.DiagnosticDependencyFailed)
				}
				return code, nil
			}
			if webPromptsDisabled(ctx) {
				return "", twoFactorInputError(errNoTwoFactorCodeSource, shared.DiagnosticRequiredInputMissing)
			}
			code, err := promptTwoFactorCodeFn()
			if err != nil {
				return "", twoFactorInputError(err, shared.DiagnosticRequiredInputMissing)
			}
			return code, nil
		}
		if code == "" {
			if command == "" {
				writeInitialPhoneFallbackGuidance()
			}
			if challenge != nil && challenge.IsPhoneMethod() {
				codeRequestedAt = d.Now()
				challenge, prepErr = ensureTwoFactorCodeRequestedFn(ctx, session)
				if prepErr != nil {
					return nil, fmt.Errorf("2fa challenge setup failed: %w", prepErr)
				}
				if challenge != nil && challenge.Requested {
					writeDeliveryNotice(challenge.Destination)
				}
			}
			resolvedCode, codeErr := readCode()
			if codeErr != nil {
				return nil, codeErr
			}
			code = resolvedCode
		}
		submitCode := func(code string) error {
			verifyCtx, cancel := d.Timeout(shared.ContextWithoutTimeout(ctx))
			defer cancel()
			return withWebSpinnerWith(d, "Verifying two-factor code", func() error {
				return submitTwoFactorCodeFn(verifyCtx, session, code)
			})
		}
		// Phone delivery can also be a voice call, so only SMS gets SMS guidance.
		smsDelivery := func() bool {
			return session.TwoFactorMethod() == appleauth.TwoFactorMethodPhone && session.TwoFactorPhoneMode() == appleauth.TwoFactorDeliverySMS
		}
		// A rejected trusted-device code makes Apple send a phone code during
		// this submission, so a fallback read must ignore anything older.
		submittedAt := d.Now()
		if err := submitCode(code); err != nil {
			var phoneCodeRequestedErr *appleauth.PhoneCodeRequestedError
			if errors.As(err, &phoneCodeRequestedErr) {
				writeDeliveryNotice(phoneCodeRequestedErr.Destination)
				writeFallbackGuidance(command != "")
				codeRequestedAt = submittedAt
				resolvedCode, codeErr := readCode()
				if codeErr != nil {
					return nil, codeErr
				}
				if err := submitCode(resolvedCode); err != nil {
					return nil, twoFactorSubmitFailure(err, true, smsDelivery())
				}
				return session, nil
			}
			return nil, twoFactorSubmitFailure(err, false, smsDelivery())
		}
		return session, nil
	}
	return nil, err
}

func loginWithOptionalTwoFactor(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode string, twoFactorStarted func(), readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	return loginWithOptionalTwoFactorUsing(ctx, d, "Signing in to Apple web session", appleID, password, twoFactorCode, webLoginFn, twoFactorStarted, readCommandCode, twoFactorCodeCommand...)
}

func loginWithOptionalTwoFactorClientTracked(ctx context.Context, d *shared.Deps, client *http.Client, appleID, password, twoFactorCode string, readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, bool, error) {
	twoFactorStarted := false
	session, err := loginWithOptionalTwoFactorUsing(ctx, d, "Refreshing expired web session", appleID, password, twoFactorCode, func(ctx context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return webLoginWithClientFn(ctx, client, credentials)
	}, func() {
		twoFactorStarted = true
	}, readCommandCode, twoFactorCodeCommand...)
	return session, twoFactorStarted, err
}

func tryResumeWebSession(ctx context.Context, d *shared.Deps, appleID string) (*webcore.AuthSession, bool, error) {
	var (
		session *webcore.AuthSession
		ok      bool
	)
	err := withWebSpinnerWith(d, "Checking cached web session", func() error {
		var err error
		session, ok, err = tryResumeSessionFn(ctx, appleID)
		return err
	})
	return session, ok, err
}

func tryResumeLastWebSession(ctx context.Context, d *shared.Deps) (*webcore.AuthSession, bool, error) {
	var (
		session *webcore.AuthSession
		ok      bool
	)
	err := withWebSpinnerWith(d, "Checking cached web session", func() error {
		var err error
		session, ok, err = tryResumeLastFn(ctx)
		return err
	})
	return session, ok, err
}

type webSessionResolveOptions struct {
	promptAppleID        func(*string) error
	resolvePassword      func(context.Context, string) (string, error)
	persistFresh         func(*webcore.AuthSession) error
	persistAutoReauth    func(*webcore.AuthSession)
	twoFactorCodeCommand string
	providerSelection    webcore.ProviderSelection
}

func tryResumeKnownWebSession(ctx context.Context, d *shared.Deps, appleID string) (*webcore.AuthSession, bool, bool, error) {
	if appleID != "" {
		resumed, ok, err := tryResumeWebSession(ctx, d, appleID)
		return resumed, ok, errors.Is(err, webcore.ErrCachedSessionExpired), err
	}
	resumed, ok, err := tryResumeLastWebSession(ctx, d)
	return resumed, ok, errors.Is(err, webcore.ErrCachedSessionExpired), err
}

func resolveKnownWebSession(ctx context.Context, d *shared.Deps, appleID string) (*webcore.AuthSession, bool, bool, error) {
	resumed, ok, cacheExpired, err := tryResumeKnownWebSession(ctx, d, appleID)
	if err == nil {
		return resumed, ok, false, nil
	}
	if cacheExpired {
		return nil, false, true, nil
	}
	return nil, false, false, fmt.Errorf("checking cached web session failed: %w", err)
}

func resolveKnownWebSessionFromSource(ctx context.Context, appleID string, source webcore.CachedSessionSource) (*webcore.AuthSession, bool, bool, error) {
	resumed, ok, err := tryResumeSessionFromSourceFn(ctx, appleID, source)
	if err == nil {
		return resumed, ok, false, nil
	}
	if errors.Is(err, webcore.ErrCachedSessionExpired) {
		return nil, false, true, nil
	}
	return nil, false, false, fmt.Errorf("checking cached web session failed: %w", err)
}

func resolveWebSessionWith(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode string, opts webSessionResolveOptions) (*webcore.AuthSession, string, error) {
	if broker, ok, err := webBrokerFromEnv(d); ok {
		if err != nil {
			return nil, "", err
		}
		session, err := resolveBrokeredWebSession(ctx, d, broker, appleID)
		return session, webSessionSourceBroker, err
	}
	shared.ApplyRootLoggingOverrides()
	if err := webcore.SessionBackendConflict(); err != nil {
		return nil, "", d.UsageError(err.Error())
	}
	noAutoSignIn, err := webAutoSignInDisabled(d)
	if err != nil {
		return nil, "", err
	}
	policy := &webSignInPolicy{
		automatic:    !isWebSignIn(ctx),
		noAutoSignIn: noAutoSignIn,
		alternative:  publicAPIAlternativeFromContext(ctx),
	}
	session, source, err := resolveWebSessionWithPolicy(ctx, d, appleID, password, twoFactorCode, opts, policy)
	return session, source, policy.finish(d, err)
}

// webSignInPolicy decides whether a resolution may contact Apple's sign-in
// service and remembers the outcome of the attempts it made. Automatic sign-in
// is any sign-in started by a command other than `asc web auth login`.
type webSignInPolicy struct {
	automatic    bool
	noAutoSignIn bool
	alternative  string
	// failure is Apple's most recent sign-in 5xx that no later answer from
	// Apple superseded; it becomes the recorded cooldown if the resolution fails.
	failure         *webcore.SigninServiceError
	failureAppleID  string
	failureAt       time.Time
	signedInAppleID string
	signedInAt      time.Time
}

// allowSignIn runs before anything is sent to Apple for appleID. It refuses an
// automatic sign-in when ASC_WEB_NO_AUTO_LOGIN is set or while a recorded
// sign-in cooldown is active; explicit `asc web auth login` is never refused.
func (p *webSignInPolicy) allowSignIn(d *shared.Deps, appleID string) error {
	if !p.automatic {
		return nil
	}
	if p.noAutoSignIn {
		return newAutoSignInDisabledError(appleID, p.alternative)
	}
	return checkSigninBackoff(d, appleID)
}

// observe records the outcome of one sign-in attempt for appleID. A 5xx from
// Apple's sign-in service becomes the pending failure; any answer that shows
// the service accepted the request (success, a two-factor challenge, or a
// credential rejection) supersedes it. Local failures change nothing.
func (p *webSignInPolicy) observe(appleID string, err error, reachedTwoFactor bool) {
	// Outcomes are stamped when Apple answers, not when they are persisted, so
	// concurrent processes can order them.
	var serviceErr *webcore.SigninServiceError
	switch {
	case err == nil:
		p.failure, p.failureAppleID = nil, ""
		p.signedInAppleID, p.signedInAt = appleID, signinBackoffNowFn()
	case errors.As(err, &serviceErr) && (serviceErr.IsServerError() || serviceErr.AccountLocked()) && !reachedTwoFactor:
		p.failure, p.failureAppleID, p.failureAt = serviceErr, appleID, signinBackoffNowFn()
	case reachedTwoFactor, errors.Is(err, webcore.ErrInvalidAppleAccountCredentials):
		p.failure, p.failureAppleID = nil, ""
	}
}

// finish persists the outcome: a resolution that failed after Apple's sign-in
// service last answered with a 5xx, or reported the account locked, records a
// cooldown, and one in which Apple accepted a sign-in clears it, even if a
// later local step such as caching the session failed.
func (p *webSignInPolicy) finish(d *shared.Deps, err error) error {
	if p.signedInAppleID != "" {
		clearSigninBackoff(d, p.signedInAppleID, p.signedInAt)
	}
	if err == nil || p.failure == nil {
		return err
	}
	notBefore, recorded := recordSigninBackoff(d, p.failureAppleID, p.failure, p.failureAt)
	if recorded && (isSigninServerError(err) || errors.Is(err, webcore.ErrAppleAccountLocked)) {
		return fmt.Errorf("%w. Other asc web commands will not sign in automatically before %s; `asc web auth login` can still be run explicitly", err, notBefore.UTC().Format(time.RFC3339))
	}
	return err
}

func resolveWebSessionWithPolicy(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode string, opts webSessionResolveOptions, policy *webSignInPolicy) (*webcore.AuthSession, string, error) {
	twoFactorPreference, err := twoFactorPreferenceForSignIn(ctx, d)
	if err != nil {
		return nil, "", err
	}
	ctx = contextWithTwoFactorPreference(ctx, twoFactorPreference)
	refuseProviderSwitchUnder := opts.providerSelection.RefuseSwitchUnder
	if refuseProviderSwitchUnder == "" {
		refuseProviderSwitchUnder = providerSwitchRefusalSetting(nil)
	}
	restoreCachedProvider := !hasProviderSelection(opts.providerSelection) || refuseProviderSwitchUnder != ""

	resolvedAppleID := strings.TrimSpace(appleID)
	// ASC_WEB_APPLE_ID sits between the flag and the cached-session default: it
	// names the account before any cache lookup, so neither the last-session
	// pointer nor an ambiguous cache can decide for a caller who already said
	// which Apple Account this invocation belongs to.
	if resolvedAppleID == "" {
		if fromEnv := envAppleID(); fromEnv != "" {
			resolvedAppleID = fromEnv
			printEnvAppleIDNotice(d, resolvedAppleID)
		}
	}
	twoFactorCode = strings.TrimSpace(twoFactorCode)
	command := strings.TrimSpace(opts.twoFactorCodeCommand)
	if command == "" {
		command = strings.TrimSpace(os.Getenv(webTwoFactorCodeCommandEnv))
	}
	if twoFactorPreference.Delivery == appleauth.TwoFactorMethodSecurityKey && (twoFactorCode != "" || command != "") {
		return nil, "", d.UsageError("--two-factor-method security-key cannot be combined with a verification code or 2FA code command; unset ASC_WEB_2FA_CODE_COMMAND")
	}
	expiredNoticePrinted := false
	printExpiredNotice := func() {
		if expiredNoticePrinted {
			return
		}
		printExpiredSessionNotice(webSignInNoticeWriter(d))
		expiredNoticePrinted = true
	}

	var (
		expiredCachedSession  *webcore.AuthSession
		cachedProviderSession *webcore.AuthSession
		fallbackPassword      resolvedWebPassword
		skipStoredPassword    bool
		twoFactorCodeConsumed bool
		consumedTwoFactorCode string
		lastCommandTwoFactor  string
		staleSessionDiscarded bool
		// serverRejectedCache is the loaded cache entry whose cookie jar IdMSA
		// answered with a 5xx (#2806). It is discarded only if the clean-jar
		// retry then signs in or reaches 2FA, which proves the jar was the cause.
		serverRejectedCache    *webcore.AuthSession
		serverRejectedAppleID  string
		staleSessionDiscardErr error
	)

	// Every code the reader hands over is submitted, so each one becomes the
	// baseline for the next read: the retry, and the phone fallback that follows
	// a rejected trusted-device code, both have to wait past the value they just
	// burned rather than only past the one the cached attempt consumed. Before
	// the command has produced anything the literal two-factor code stands in,
	// and with nothing consumed at all this is a plain pass-through.
	readCommandTwoFactorCode := func(ctx context.Context, command string) (string, error) {
		burned := lastCommandTwoFactor
		if burned == "" {
			burned = consumedTwoFactorCode
		}
		code, err := readRotatedTwoFactorCodeFromCommand(ctx, command, burned)
		if err != nil {
			return "", err
		}
		lastCommandTwoFactor = code
		return code, nil
	}
	// Apple invalidates a code the moment it is submitted, so record which value
	// the failed attempt burned before the retry asks for a replacement.
	markTwoFactorCodeConsumed := func() {
		// A fresh key challenge obtains a new assertion without a verification code.
		if twoFactorPreference.Delivery == appleauth.TwoFactorMethodSecurityKey {
			return
		}
		twoFactorCodeConsumed = true
		if consumedTwoFactorCode != "" {
			return
		}
		consumedTwoFactorCode = lastCommandTwoFactor
		if consumedTwoFactorCode == "" {
			consumedTwoFactorCode = twoFactorCode
		}
	}

	// Apple consumes a 2FA code as soon as it is accepted, so a literal
	// two-factor code value cannot be resubmitted on the fresh retry. Only a
	// configured code command may produce a replacement: supplying the code up
	// front is a non-interactive choice, so falling back to a prompt could hang
	// a scripted invocation that happens to own a terminal.
	canReplaceConsumedTwoFactorCode := func() bool {
		return twoFactorCode == "" || command != ""
	}
	// A cached jar is proven unusable once it fails the post-2FA session
	// bootstrap, so discard it as soon as that is detected rather than relying on
	// a successful fresh login to overwrite it. A retry that never lands - a bail
	// out, or a fresh login that fails on its own - would otherwise leave the jar
	// on disk for the next invocation to reload, consume another code against,
	// and fail identically. The result is memoized so one resolution deletes the
	// cached entry at most once and every caller reports the same outcome.
	// Deleting by Apple ID alone would take out a valid session that a
	// concurrent process persisted while this one worked through 2FA, so the
	// discard is scoped to the entry this resolution actually loaded.
	discardProvenStaleSession := func(targetAppleID string) error {
		if staleSessionDiscarded {
			return staleSessionDiscardErr
		}
		staleSessionDiscarded = true
		_, staleSessionDiscardErr = deleteStaleWebSessionFn(strings.TrimSpace(targetAppleID), expiredCachedSession)
		return staleSessionDiscardErr
	}
	consumedTwoFactorCodeError := func(loginErr, discardErr error) error {
		if discardErr != nil {
			return fmt.Errorf("%w; the supplied two-factor code was already consumed by the expired session and the stale cached session could not be discarded (%w): run `asc web auth logout` before re-running with a new code", loginErr, discardErr)
		}
		return fmt.Errorf("%w; the supplied two-factor code was already consumed by the expired session, so the stale cached session was discarded: re-run with a new code, or configure --two-factor-code-command or %s to fetch one automatically", loginErr, webTwoFactorCodeCommandEnv)
	}

	tryKnownSession := func(targetAppleID string, source webcore.CachedSessionSource) (*webcore.AuthSession, string, bool, error) {
		var resumed *webcore.AuthSession
		var ok, cacheExpired bool
		var err error
		if source == webcore.CachedSessionSourceUnknown {
			resumed, ok, cacheExpired, err = resolveKnownWebSession(ctx, d, targetAppleID)
		} else {
			resumed, ok, cacheExpired, err = resolveKnownWebSessionFromSource(ctx, targetAppleID, source)
		}
		if errors.Is(err, webcore.ErrSessionKeychainBypassed) {
			return nil, "", false, d.UsageError(err.Error())
		}
		if err != nil {
			printCacheLookupWarning(d.IO.ErrOut, err)
			return nil, "", false, nil
		}
		if ok {
			return resumed, "cache", true, nil
		}
		if !cacheExpired {
			if refuseProviderSwitchUnder != "" && resumed != nil && resumed.Client == nil && targetAppleID != "" && strings.EqualFold(resumed.UserEmail, targetAppleID) {
				cachedProviderSession = resumed
			}
			return nil, "", false, nil
		}

		var cachedOK bool
		if strings.TrimSpace(targetAppleID) != "" {
			if source == webcore.CachedSessionSourceUnknown {
				expiredCachedSession, cachedOK, err = loadCachedSessionFn(targetAppleID)
			} else {
				expiredCachedSession, cachedOK, err = loadCachedSessionFromSourceFn(targetAppleID, source)
			}
		} else {
			expiredCachedSession, cachedOK, err = loadLastCachedSessionFn()
		}
		if err != nil {
			printCacheLookupWarning(d.IO.ErrOut, fmt.Errorf("loading expired cached web session failed: %w", err))
			expiredCachedSession = nil
			printExpiredNotice()
			return nil, "", false, nil
		}
		if !cachedOK || expiredCachedSession == nil || expiredCachedSession.Client == nil {
			expiredCachedSession = nil
			printExpiredNotice()
			return nil, "", false, nil
		}
		cachedProviderSession = expiredCachedSession

		reauthAppleID := strings.TrimSpace(targetAppleID)
		if reauthAppleID == "" {
			reauthAppleID = strings.TrimSpace(expiredCachedSession.UserEmail)
		}
		if reauthAppleID == "" {
			return nil, "", false, d.UsageError("last cached web session predates stored Apple ID metadata; re-run once with --apple-id to refresh the cache")
		}
		if resolvedAppleID == "" {
			resolvedAppleID = reauthAppleID
		}
		if err := policy.allowSignIn(d, reauthAppleID); err != nil {
			return nil, "", false, err
		}

		if twoFactorPreference.Delivery == appleauth.TwoFactorMethodSecurityKey && !securityKeyAvailableFn() {
			return nil, "", false, d.UsageError(securitykey.ErrUnavailable.Error())
		}
		silentPassword := resolveNonInteractiveWebPassword(d, reauthAppleID, password, skipStoredPassword)
		if !webPasswordProvided(silentPassword.value) {
			printExpiredNotice()
			return nil, "", false, nil
		}

		session, twoFactorStarted, loginErr := loginWithOptionalTwoFactorClientTracked(ctx, d, expiredCachedSession.Client, reauthAppleID, silentPassword.value, twoFactorCode, readCommandTwoFactorCode, command)
		policy.observe(reauthAppleID, loginErr, twoFactorStarted)
		if loginErr == nil {
			if restoreCachedProvider {
				if err := restoreCachedWebProvider(ctx, d, session, expiredCachedSession, refuseProviderSwitchUnder); err != nil {
					return nil, "", false, err
				}
			}
			if opts.persistAutoReauth != nil {
				opts.persistAutoReauth(session)
			}
			return session, "auto-reauth", true, nil
		}
		if errors.Is(loginErr, errSecurityKeyPreflight) {
			return nil, "", false, loginErr
		}
		// A post-2FA session bootstrap 401/403 means the reused cookie jar is
		// stale, not that the code was wrong. Fall through to one fresh login.
		if twoFactorStarted {
			if !webcore.IsStaleSessionAfterTwoFactor(loginErr) {
				return nil, "", false, fmt.Errorf("web auth auto-reauth failed: %w", loginErr)
			}
			discardErr := discardProvenStaleSession(reauthAppleID)
			if !canReplaceConsumedTwoFactorCode() {
				return nil, "", false, fmt.Errorf("web auth auto-reauth failed: %w", consumedTwoFactorCodeError(loginErr, discardErr))
			}
			printCacheLookupWarning(d.IO.ErrOut, staleSessionDiscardWarning(discardErr))
			markTwoFactorCodeConsumed()
		}
		if errors.Is(loginErr, webcore.ErrAppleAccountLocked) {
			// Apple locked the account (#3111); a fresh jar cannot change that,
			// and every attempt is one more against a locked account.
			return nil, "", false, appleAccountLockedError(reauthAppleID, loginErr)
		}
		if errors.Is(loginErr, webcore.ErrInvalidAppleAccountCredentials) {
			if silentPassword.source != webPasswordSourceStored {
				return nil, "", false, fmt.Errorf("web auth auto-reauth failed: %w", loginErr)
			}
			forgetRejectedStoredPassword(d, reauthAppleID)
			skipStoredPassword = true
			printExpiredNotice()
			return nil, "", false, nil
		}

		// Apple's sign-in service answered the cached cookie jar with a 5xx
		// (#2806). Discard that entry before the single clean-jar retry so a
		// retry that also fails cannot leave it for the next invocation to
		// replay. A successful fresh login replaces the entry anyway.
		if !twoFactorStarted && isSigninServerError(loginErr) {
			serverRejectedCache, serverRejectedAppleID = expiredCachedSession, reauthAppleID
		}

		// A cached jar can become unusable independently of the credentials,
		// either before 2FA begins or when the post-2FA session bootstrap is
		// rejected. Preserve the password source for one fresh fallback.
		fallbackPassword = silentPassword
		expiredCachedSession = nil
		printExpiredNotice()
		return nil, "", false, nil
	}

	if session, source, ok, err := tryKnownSession(resolvedAppleID, webcore.CachedSessionSourceUnknown); err != nil {
		return nil, "", err
	} else if ok {
		return session, source, nil
	}

	if resolvedAppleID == "" {
		// The last-session pointer resolved nothing, so fall back to the only
		// cached account when there is exactly one. Only an empty cache may
		// still prompt (apps create); an ambiguous cache never guesses.
		defaultAppleID, defaultSource, cacheEmpty, defaultErr := resolveDefaultCachedAppleID(d)
		// Prompting for an Apple ID only leads to a sign-in, which the
		// no-automatic-sign-in policy forbids.
		promptAppleID := opts.promptAppleID
		if policy.automatic && policy.noAutoSignIn {
			promptAppleID = nil
		}
		switch {
		case defaultErr == nil:
			resolvedAppleID = defaultAppleID
		case promptAppleID == nil || !cacheEmpty:
			var ambiguous *webcore.AmbiguousCachedSessionError
			if errors.As(defaultErr, &ambiguous) {
				return nil, "", ambiguousAppleIDUsageError(d, ambiguous.AppleIDs)
			}
			if isWebSignIn(ctx) {
				return nil, "", missingAppleIDUsageError(d)
			}
			if policy.noAutoSignIn {
				return nil, "", newAutoSignInDisabledError("", policy.alternative)
			}
			return nil, "", newMissingWebSessionError("", publicAPIAlternativeFromContext(ctx))
		default:
			if err := promptAppleID(&resolvedAppleID); err != nil {
				return nil, "", err
			}
			resolvedAppleID = strings.TrimSpace(resolvedAppleID)
			defaultSource = webcore.CachedSessionSourceUnknown
		}
		if session, source, ok, err := tryKnownSession(resolvedAppleID, defaultSource); err != nil {
			return nil, "", err
		} else if ok {
			return session, source, nil
		}
	}

	if opts.resolvePassword == nil {
		return nil, "", fmt.Errorf("password resolver is required")
	}
	if err := policy.allowSignIn(d, resolvedAppleID); err != nil {
		return nil, "", err
	}
	if twoFactorPreference.Delivery == appleauth.TwoFactorMethodSecurityKey && !securityKeyAvailableFn() {
		return nil, "", d.UsageError(securitykey.ErrUnavailable.Error())
	}
	resolvedPassword := fallbackPassword
	if !webPasswordProvided(resolvedPassword.value) {
		resolvedPassword = resolveNonInteractiveWebPassword(d, resolvedAppleID, password, skipStoredPassword)
	}
	if !webPasswordProvided(resolvedPassword.value) {
		if err := securityKeyPreflight(ctx, d, twoFactorPreference); err != nil {
			return nil, "", err
		}
		promptedPassword, err := opts.resolvePassword(ctx, "")
		if err != nil {
			return nil, "", err
		}
		if webPasswordProvided(promptedPassword) {
			resolvedPassword = resolvedWebPassword{value: promptedPassword, source: webPasswordSourcePrompted}
		}
	}
	if !webPasswordProvided(resolvedPassword.value) {
		if isWebSignIn(ctx) {
			if webPromptsDisabled(ctx) {
				return nil, "", d.UsageError("password is required with --no-prompt: set " + webPasswordEnv)
			}
			return nil, "", passwordRequiredUsageErrorWith(d)
		}
		// No usable cached session and no way to sign in here: the command
		// needs a session it cannot create. Say so with the sign-in hint rather
		// than a missing-password usage page.
		return nil, "", newMissingWebSessionError(resolvedAppleID, publicAPIAlternativeFromContext(ctx))
	}
	// Without a cached cookie jar there is no trusted-device cookie, so Apple
	// always answers the password step with a 2FA challenge. Refuse before
	// signing in when nothing here can supply the code.
	if twoFactorPreference.Delivery != appleauth.TwoFactorMethodSecurityKey && expiredCachedSession == nil && twoFactorCode == "" && command == "" && (webPromptsDisabled(ctx) || !twoFactorPromptAvailableFn()) {
		return nil, "", twoFactorInputError(errNoTwoFactorCodeSource, shared.DiagnosticRequiredInputMissing)
	}

	// A 5xx on the cached jar alone does not prove the jar is stale: Apple also
	// answers outages and throttling with 5xx, and the entry may still hold a
	// valid trust cookie. Only when a clean-jar retry signs in or reaches 2FA
	// has Apple shown the cached jar to be the cause; a local or transport
	// failure proves nothing. Discard it then, before any later exit path (a
	// failed persist, a cancelled prompt) can leave it to be replayed.
	discardServerRejectedCache := func() {
		if serverRejectedCache == nil || staleSessionDiscarded {
			return
		}
		staleSessionDiscarded = true
		if _, discardErr := deleteStaleWebSessionFn(strings.TrimSpace(serverRejectedAppleID), serverRejectedCache); discardErr != nil && d.IO.ErrOut != nil {
			_, _ = fmt.Fprintf(d.IO.ErrOut, "Warning: %v.\n", staleSessionDiscardWarning(discardErr))
		}
	}
	login := func(candidate resolvedWebPassword) (*webcore.AuthSession, bool, error) {
		// Interactive password and 2FA entry can outlast the caller's request
		// deadline, so bound every attempt with a fresh timeout derived from the
		// untimed parent context instead of an already-expired one.
		loginCtx, cancel := d.Timeout(shared.ContextWithoutTimeout(ctx))
		defer cancel()
		// Stripping the timeout also drops values layered above it.
		loginCtx = contextWithTwoFactorPreference(loginCtx, twoFactorPreference)
		if isWebSignIn(ctx) {
			loginCtx = contextForWebSignIn(loginCtx)
		}
		code := twoFactorCode
		if twoFactorCodeConsumed {
			code = ""
			// Only a prompted operator has to be told which code to type: the
			// burned digits are still on screen from the first prompt. The notice
			// waits until the fresh sign-in has raised its own challenge.
			if command == "" {
				loginCtx = context.WithValue(loginCtx, consumedCodeNoticeContextKey{}, true)
			}
		}
		if expiredCachedSession != nil && expiredCachedSession.Client != nil {
			session, twoFactorStarted, err := loginWithOptionalTwoFactorClientTracked(loginCtx, d, expiredCachedSession.Client, resolvedAppleID, candidate.value, code, readCommandTwoFactorCode, command)
			policy.observe(resolvedAppleID, err, twoFactorStarted)
			return session, twoFactorStarted, err
		}
		reachedTwoFactor := false
		session, err := loginWithOptionalTwoFactor(loginCtx, d, resolvedAppleID, candidate.value, code, func() {
			reachedTwoFactor = true
		}, readCommandTwoFactorCode, command)
		policy.observe(resolvedAppleID, err, reachedTwoFactor)
		if err == nil || reachedTwoFactor {
			discardServerRejectedCache()
		}
		return session, false, err
	}
	loginWithPromptedFreshFallback := func(candidate resolvedWebPassword) (*webcore.AuthSession, error) {
		session, twoFactorStarted, err := login(candidate)
		blockedByTwoFactor := twoFactorStarted && !webcore.IsStaleSessionAfterTwoFactor(err)
		if err == nil || candidate.source != webPasswordSourcePrompted || expiredCachedSession == nil || blockedByTwoFactor || errors.Is(err, errSecurityKeyPreflight) || errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) || errors.Is(err, webcore.ErrAppleAccountLocked) {
			return session, err
		}
		// Match the non-interactive reauth path: a non-credential failure can
		// mean that the cached cookie jar itself is unusable, including when the
		// post-2FA session bootstrap is rejected. Retry once with a fresh client
		// while preserving the already-resolved password.
		if twoFactorStarted {
			discardErr := discardProvenStaleSession(resolvedAppleID)
			if !canReplaceConsumedTwoFactorCode() {
				return nil, consumedTwoFactorCodeError(err, discardErr)
			}
			printCacheLookupWarning(d.IO.ErrOut, staleSessionDiscardWarning(discardErr))
			markTwoFactorCodeConsumed()
		} else if isSigninServerError(err) {
			serverRejectedCache, serverRejectedAppleID = expiredCachedSession, resolvedAppleID
		}
		expiredCachedSession = nil
		session, _, err = login(candidate)
		return session, err
	}

	session, err := loginWithPromptedFreshFallback(resolvedPassword)
	if errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) && resolvedPassword.source == webPasswordSourceStored {
		forgetRejectedStoredPassword(d, resolvedAppleID)
		promptedPassword, promptErr := opts.resolvePassword(ctx, "")
		if promptErr != nil {
			return nil, "", promptErr
		}
		if webPasswordProvided(promptedPassword) {
			resolvedPassword = resolvedWebPassword{value: promptedPassword, source: webPasswordSourcePrompted}
			session, err = loginWithPromptedFreshFallback(resolvedPassword)
		}
	}
	if err != nil {
		if isSigninServerError(err) {
			return nil, "", fmt.Errorf("web auth login failed: %w; %s", err, signinServerErrorHint(resolvedAppleID))
		}
		if errors.Is(err, webcore.ErrAppleAccountLocked) {
			return nil, "", appleAccountLockedError(resolvedAppleID, err)
		}
		err = fmt.Errorf("web auth login failed: %w", err)
		if errors.Is(err, webcore.ErrAppleAccountActionRequired) {
			return nil, "", shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "")
		}
		return nil, "", err
	}
	persistPromptedWebPassword(d, resolvedAppleID, resolvedPassword)
	if restoreCachedProvider {
		if err := restoreCachedWebProvider(ctx, d, session, cachedProviderSession, refuseProviderSwitchUnder); err != nil {
			return nil, "", err
		}
	}
	if opts.persistFresh != nil {
		if err := opts.persistFresh(session); err != nil {
			return nil, "", err
		}
	}
	return session, "fresh", nil
}

// appleAccountLockedError reports Apple's API sign-in lock (#3111) with the
// unlock guidance. Like a pending Apple Account prompt, it is a state only the
// account owner can change, not an authentication failure asc can retry.
func appleAccountLockedError(appleID string, err error) error {
	err = fmt.Errorf("web auth login failed: %w; %s", err, appleAccountLockedHint(appleID))
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "")
}

// isSigninServerError reports a 5xx from an Apple IdMSA sign-in stage, which
// Apple returns both for outages and for throttled sign-in attempts.
func isSigninServerError(err error) bool {
	var serviceErr *webcore.SigninServiceError
	return errors.As(err, &serviceErr) && serviceErr.IsServerError()
}

func signinServerErrorHint(appleID string) string {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		appleID = "EMAIL"
	}
	return fmt.Sprintf("Apple's sign-in service is unavailable or is throttling sign-ins for this account. Wait several minutes before retrying, because every attempt counts toward Apple's limit. If it keeps failing, run `asc web auth logout --apple-id %q` to clear the cached web session, then sign in again", appleID)
}

func resolveSessionPassword(ctx context.Context, password string) (string, error) {
	if webPasswordProvided(password) {
		return password, nil
	}
	return readPasswordFromInput(ctx)
}

func persistFreshResolvedSession(session *webcore.AuthSession) error {
	if err := persistWebSessionFn(session); err != nil {
		return fmt.Errorf("web auth login succeeded but failed to cache session: %w", err)
	}
	return nil
}

func persistAutoReauthResolvedSession(d *shared.Deps, session *webcore.AuthSession) {
	printSessionCacheWriteWarningWith(d, persistWebSessionFn(session))
}

func printSessionCacheWriteWarningWith(d *shared.Deps, err error) {
	if d.IO.ErrOut == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(d.IO.ErrOut, "Warning: signed in, but caching the web session failed: %v; the next asc web command may need to sign in again.\n", err)
}

// restoreCachedWebProvider keeps a re-signed-in session on its cached provider.
// Restoration requires a provider switch when Apple chooses a different default.
func restoreCachedWebProvider(ctx context.Context, d *shared.Deps, session, cached *webcore.AuthSession, refuseSwitchUnder string) error {
	if cached == nil || cached.ProviderID == 0 {
		return nil
	}
	if session.ProviderID != cached.ProviderID {
		if refuseSwitchUnder != "" {
			refused := &webcore.ProviderSwitchRefusedError{
				Setting:             refuseSwitchUnder,
				CurrentProvider:     fmt.Sprint(session.ProviderID),
				RequestedProvider:   fmt.Sprint(cached.ProviderID),
				RequestedProviderID: cached.ProviderID,
			}
			how := "with neither --read-only nor ASC_READ_ONLY set, then retry"
			if refuseSwitchUnder == "--dry-run" {
				how = "first (or run this command without --dry-run), then retry"
			}
			hint := fmt.Sprintf("Sign-in landed on provider %d. To refresh the session for cached provider %d, run 'asc web auth login --provider-id %d' %s.", session.ProviderID, cached.ProviderID, cached.ProviderID, how)
			return reportProviderSwitchRefusal(d, refused, hint)
		}
		providerCtx, cancel := d.Timeout(shared.ContextWithoutTimeout(ctx))
		defer cancel()
		if err := selectWebProviderFn(providerCtx, session, webcore.ProviderSelection{ProviderID: cached.ProviderID}); err != nil {
			return fmt.Errorf("web auth signed in, but could not switch back to provider %d used by the cached web session: %w; run 'asc web auth login --provider-id ID' to choose another provider", cached.ProviderID, err)
		}
	}
	session.DeveloperTeamID = cached.DeveloperTeamID
	return nil
}

// providerSwitchRefusal renders a refused provider switch with the command
// that does switch. Under read-only mode it stays a read-only refusal (exit
// 6); under --dry-run it is a usage error (exit 2) printed without the usage
// page, like the other --dry-run conflicts.
func providerSwitchRefusal(d *shared.Deps, refused *webcore.ProviderSwitchRefusedError) error {
	var hint string
	if refused.Setting == "--dry-run" {
		hint = fmt.Sprintf("The session stays on provider %s. To switch, run 'asc web auth login --provider-id %d' first (or run this command without --dry-run); a dry run on that provider then needs no switch.", refused.CurrentProvider, refused.RequestedProviderID)
	} else {
		hint = fmt.Sprintf("The session stays on provider %s. To switch, run 'asc web auth login --provider-id %d' with neither --read-only nor ASC_READ_ONLY set; later commands then run on provider %d.", refused.CurrentProvider, refused.RequestedProviderID, refused.RequestedProviderID)
	}
	return reportProviderSwitchRefusal(d, refused, hint)
}

func reportProviderSwitchRefusal(d *shared.Deps, refused *webcore.ProviderSwitchRefusedError, hint string) error {
	if refused.Setting == "--dry-run" {
		fmt.Fprintf(d.IO.ErrOut, "Error: %s\nHint: %s\n", refused.Error(), hint)
		return shared.NewErrorWithCause(shared.NewReportedUsageError(shared.UsageErrorOther, refused.Error()), refused)
	}
	return &shared.HintedError{Err: refused, Hint: hint}
}

func selectResolvedWebSessionProviderWith(ctx context.Context, d *shared.Deps, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
	if selection.ProviderID == 0 && strings.TrimSpace(selection.PublicProviderID) == "" {
		return nil
	}
	// Interactive 2FA may outlive the request budget used to start login.
	// Renew only the internal timeout, preserving caller cancellation.
	providerCtx, cancel := d.Timeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	if err := selectWebProviderFn(providerCtx, session, selection); err != nil {
		if refused, ok := errors.AsType[*webcore.ProviderSwitchRefusedError](err); ok {
			// The session is still on its own provider. Cache it anyway, so a
			// sign-in this command just made is not lost with the refusal.
			printSessionCacheWriteWarningWith(d, persistWebSessionFn(session))
			return providerSwitchRefusal(d, refused)
		}
		err = fmt.Errorf("web provider selection failed: %w", err)
		if !errors.Is(err, webcore.ErrInvalidProviderSelection) {
			return err
		}
		parameter := "--provider-id"
		if strings.TrimSpace(selection.PublicProviderID) != "" {
			parameter = "--public-provider-id"
		}
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, parameter)
	}
	if err := persistWebSessionFn(session); err != nil {
		return fmt.Errorf("web provider selection succeeded but failed to cache session: %w", err)
	}
	return nil
}

func resolveSession(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = twoFactorCodeCommand[0]
	}
	return resolveSessionWithSelection(ctx, d, appleID, password, twoFactorCode, command, webcore.ProviderSelection{})
}

func resolveSessionWithSelection(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode, twoFactorCodeCommand string, selection webcore.ProviderSelection) (*webcore.AuthSession, string, error) {
	opts := webSessionResolveOptions{
		resolvePassword:      resolveSessionPassword,
		twoFactorCodeCommand: twoFactorCodeCommand,
		providerSelection:    selection,
	}
	if !hasProviderSelection(selection) {
		opts.persistFresh = persistFreshResolvedSession
		opts.persistAutoReauth = func(session *webcore.AuthSession) {
			persistAutoReauthResolvedSession(d, session)
		}
	}
	return resolveWebSessionWith(ctx, d, appleID, password, twoFactorCode, opts)
}

func hasProviderSelection(selection webcore.ProviderSelection) bool {
	return selection.ProviderID != 0 || strings.TrimSpace(selection.PublicProviderID) != ""
}

func callResolveSessionForProviderSelectionWith(ctx context.Context, d *shared.Deps, appleID, password, twoFactorCode, twoFactorCodeCommand string, selection webcore.ProviderSelection) (*webcore.AuthSession, string, error) {
	if selection.ProviderID < 0 {
		return nil, "", shared.WithDiagnostic(d.UsageError("--provider-id must be non-negative"), shared.DiagnosticInvalidInput, "--provider-id")
	}
	if hasProviderSelection(selection) {
		if resolveSessionWithoutPersistFn != nil {
			return callSessionResolverHook(ctx, resolveSessionWithoutPersistFn, "resolveSessionWithoutPersistFn", appleID, password, twoFactorCode, twoFactorCodeCommand)
		}
	} else if resolveSessionFn != nil {
		return callResolveSessionFn(ctx, d, appleID, password, twoFactorCode, twoFactorCodeCommand)
	}
	return resolveSessionWithSelection(ctx, d, appleID, password, twoFactorCode, twoFactorCodeCommand, selection)
}

// newWebAuthCommand returns the detached web auth command group.
func newWebAuthCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web auth", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc web auth <subcommand> [flags]",
		ShortHelp:  "Manage Apple web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Apple web-session authentication used by "asc web" commands.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAuthLoginCommand(d),
			newWebAuthStatusCommand(d),
			newWebAuthCapabilitiesCommand(d),
			newWebAuthMessagesCodeCommand(d),
			newWebAuthExportCommand(d),
			newWebAuthImportCommand(d),
			newWebAuthLogoutCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAuthLoginCommand creates or refreshes a web session.
func newWebAuthLoginCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web auth login", flag.ExitOnError)
	noPrompt := fs.Bool("no-prompt", false, "Disable interactive password and verification-code prompts")

	appleID := fs.String("apple-id", "", "Apple Account email (defaults to "+webAppleIDEnv+", then the last or only cached session)")
	twoFactorCodeCommand := fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required")
	twoFactorDelivery := bindTwoFactorDeliveryFlags(fs)
	providerID := fs.Int64("provider-id", 0, "Numeric App Store Connect provider ID to select for this web session (refused in read-only mode when it names another provider)")
	publicProviderID := fs.String("public-provider-id", "", "Public App Store Connect provider/team ID to select for this web session (refused in read-only mode when it names another provider)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc web auth login [--apple-id EMAIL] [--public-provider-id TEAM_ID]",
		ShortHelp:  "Authenticate Apple web session.",
		LongHelp: fmt.Sprintf(
			`WEB SESSION WORKFLOWS

Authenticate using Apple web-session behavior for "asc web" workflows.

Apple Account input options:
  - --apple-id
  - %s environment variable
  - the last or only cached web session

Password input options:
  - secure interactive prompt (default; saved in the native credential store after successful login)
  - %s environment variable
  - %s=1 disables saved-password reads and writes

Two-factor input options:
  - secure interactive prompt (default for manual use; needs a terminal)
  - --two-factor-code-command
  - %s environment variable (recommended for automation)

With --no-prompt or without a terminal, a fresh sign-in needs a 2FA code command or explicit
--two-factor-method security-key; otherwise it fails before the password is sent.

Security-key sign-in (macOS):
  - official macOS release builds bundle the native dependencies
  - source builds need CGO_ENABLED=1 go build -tags fido2, libfido2 1.14 or newer, and pkg-config
  - --two-factor-method security-key checks one connected USB FIDO2 key before sending the password
  - auto can detect a key challenge only with a terminal or code source; it never signs with the key
  - use explicit security-key mode for non-terminal key-only sign-in
  - set ASC_WEB_SECURITY_KEY_PIN for this explicit login if the key requires a PIN; it is never saved
  - automatic reauthentication never uses the PIN; sign in with asc web auth login when it is configured
  - touch the key when requested; wrong PINs are never retried automatically

Automatic sign-in:
  - other web commands sign in again on their own when the cached session expires
  - %s=1 makes this command the only one that signs in
  - after Apple's sign-in service answers with a 5xx, other web commands pause
    automatic sign-in for that account (Retry-After, else 15 minutes; at most
    1 hour); this command is never paused, and a successful login ends the pause
  - after Apple reports the account locked for security reasons (403, service
    error -20209), other web commands pause automatic sign-in for 1 hour; unlock
    the account at https://iforgot.apple.com, then run this command

Phone-code fallback (including SMS):
  - interactive: if Apple offers a registered phone fallback, enter an incorrect trusted-device code once
  - Apple then delivers a phone verification code and asc prompts again
  - automated: asc reruns the configured 2FA code command after phone fallback

SMS delivery:
  - --two-factor-method sms (or %s=sms) asks Apple to text the code to a
    trusted phone number up front, even when trusted devices are available
  - --two-factor-phone-id (or %s) selects the number when the account has several;
    without it, asc fails and lists the available IDs
  - asc requests the SMS before prompting or running the 2FA code command
  - Apple may still show a sign-in alert on trusted devices; only the texted
    code completes this sign-in

Security-key example:
  asc web auth login --apple-id "user@example.com" --two-factor-method security-key

Provider selection:
  - --public-provider-id selects the public App Store Connect provider/team ID
  - --provider-id selects Apple's numeric App Store Connect provider ID
  - Under --read-only or ASC_READ_ONLY, signing in still works, but switching
    an existing session to another provider is refused (exit 6)

Examples:
  # Interactive (in a terminal)
  asc web auth login --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com" --public-provider-id "Z4N6A5FQKW"

  # Non-interactive (agents, CI): supply the password and a 2FA code source
  %s %s='osascript /path/to/get-apple-2fa-code.scpt' asc web auth login --apple-id "user@example.com"

  # Text the code to a trusted phone number instead of trusted devices
  asc web auth login --apple-id "user@example.com" --two-factor-method sms --two-factor-code-command "/path/to/read-apple-sms-code"

  # macOS: read an SMS code forwarded to this Mac's Messages app
  asc web auth login --apple-id "user@example.com" --two-factor-method sms --two-factor-code-command "asc web auth messages-code"`,
			webAppleIDEnv,
			webPasswordEnvDisplay(),
			webDontStorePasswordEnv,
			webTwoFactorCodeCommandEnv,
			webNoAutoLoginEnv,
			webTwoFactorMethodEnv,
			webTwoFactorPhoneIDEnv,
			webPasswordEnvAssignmentExample(),
			webTwoFactorCodeCommandEnv,
		),
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := refuseUnderWebBroker(d, "asc web auth login"); err != nil {
				return err
			}
			preference, err := twoFactorDelivery.resolveWith(d)
			if err != nil {
				return err
			}
			selection := webcore.ProviderSelection{
				ProviderID:        *providerID,
				PublicProviderID:  *publicProviderID,
				RefuseSwitchUnder: providerSwitchRefusalSetting(nil),
			}
			signInCtx := contextWithTwoFactorPreference(contextForWebSignIn(ctx), preference)
			if *noPrompt {
				signInCtx = context.WithValue(signInCtx, webNoPromptContextKey{}, true)
			}
			session, source, err := callResolveSessionForProviderSelectionWith(signInCtx, d, *appleID, "", "", *twoFactorCodeCommand, selection)
			if err != nil {
				return err
			}

			// Provider selection is a request, so its budget starts once the
			// interactive login finished.
			requestCtx, cancel := newWebRequestContextWith(ctx, d)
			defer cancel()

			if err := selectResolvedWebSessionProviderWith(requestCtx, d, session, selection); err != nil {
				return err
			}

			status := webAuthStatus{
				Authenticated:    true,
				PasswordStored:   storedWebPasswordStatus(d, session.UserEmail),
				Source:           source,
				AppleID:          session.UserEmail,
				TeamID:           session.TeamID,
				ProviderID:       session.ProviderID,
				PublicProviderID: session.PublicProviderID,
				DeveloperTeamID:  session.DeveloperTeamID,
			}
			return printWebAuthStatus(d, status, *output.Output, *output.Pretty)
		},
	}
}

// newWebAuthStatusCommand checks whether a cached session is currently valid.
func newWebAuthStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web auth status", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email (checks this account cache; default checks last cached session)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc web auth status [--apple-id EMAIL]",
		ShortHelp:  "Show web-session status.",
		LongHelp: `WEB SESSION WORKFLOWS

Check whether an existing cached web session can be resumed.
If --apple-id is not provided, this checks the last cached session.
Without a usable session, autoSignIn says whether the next web command signs
in by itself, and autoSignInPausedUntil when a sign-in cooldown is active.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			trimmedAppleID := strings.TrimSpace(*appleID)
			if broker, ok, err := webBrokerFromEnv(d); ok {
				if err != nil {
					return err
				}
				session, err := resolveBrokeredWebSession(requestCtx, d, broker, trimmedAppleID)
				if err != nil {
					return fmt.Errorf("web auth status failed: %w", err)
				}
				return printWebAuthStatus(d, webAuthStatus{
					Authenticated:    true,
					Source:           webSessionSourceBroker,
					AppleID:          session.UserEmail,
					TeamID:           session.TeamID,
					ProviderID:       session.ProviderID,
					PublicProviderID: session.PublicProviderID,
				}, *output.Output, *output.Pretty)
			}
			var (
				session *webcore.AuthSession
				ok      bool
				err     error
			)
			if trimmedAppleID != "" {
				session, ok, err = tryResumeWebSession(requestCtx, d, trimmedAppleID)
			} else {
				session, ok, err = tryResumeLastWebSession(requestCtx, d)
			}
			if err != nil {
				if errors.Is(err, webcore.ErrCachedSessionExpired) {
					status, cached := expiredWebAuthStatus(d, trimmedAppleID)
					if err := addAutoSignInStatus(requestCtx, d, &status, trimmedAppleID, cached); err != nil {
						return err
					}
					return printWebAuthStatus(d, status, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("web auth status failed: %w", err)
			}

			if !ok || session == nil {
				status := webAuthStatus{
					Authenticated:  false,
					PasswordStored: storedWebPasswordStatus(d, trimmedAppleID),
					AppleID:        trimmedAppleID,
				}
				if err := addAutoSignInStatus(requestCtx, d, &status, trimmedAppleID, nil); err != nil {
					return err
				}
				return printWebAuthStatus(d, status, *output.Output, *output.Pretty)
			}
			return printWebAuthStatus(d, webAuthStatus{
				Authenticated:    true,
				PasswordStored:   storedWebPasswordStatus(d, session.UserEmail),
				Source:           "cache",
				AppleID:          session.UserEmail,
				TeamID:           session.TeamID,
				ProviderID:       session.ProviderID,
				PublicProviderID: session.PublicProviderID,
				DeveloperTeamID:  session.DeveloperTeamID,
			}, *output.Output, *output.Pretty)
		},
	}
}

// newWebAuthLogoutCommand clears cached web sessions.
func newWebAuthLogoutCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web auth logout", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email to remove from cache")
	all := fs.Bool("all", false, "Remove all cached web sessions")
	forgetPassword := fs.Bool("forget-password", false, "Also remove the saved Apple Account password")
	confirm := fs.Bool("confirm", false, "Confirm removal of saved password credentials")

	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc web auth logout [--apple-id EMAIL | --all] [--forget-password --confirm]",
		ShortHelp:  "Clear web-session cache.",
		LongHelp: `WEB SESSION WORKFLOWS

Remove cached web-session credentials for "asc web" commands.
Pass --forget-password --confirm to also remove the native credential-store password.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := refuseUnderWebBroker(d, "asc web auth logout"); err != nil {
				return err
			}
			trimmedAppleID := strings.TrimSpace(*appleID)
			if *all && trimmedAppleID != "" {
				return d.UsageError("--all and --apple-id are mutually exclusive")
			}
			if *confirm && !*forgetPassword {
				return d.UsageError("--confirm requires --forget-password")
			}
			if *forgetPassword && !*confirm {
				return d.UsageError("--forget-password requires --confirm")
			}
			if *all {
				if *forgetPassword {
					if err := deleteAllStoredWebPasswordsFn(); err != nil {
						return fmt.Errorf("web auth logout failed to forget saved passwords: %w", err)
					}
				}
				if err := deleteAllWebSessionsFn(); err != nil {
					if *forgetPassword {
						return fmt.Errorf("web auth logout forgot saved passwords but failed to remove sessions: %w", err)
					}
					return fmt.Errorf("web auth logout failed: %w", err)
				}
				if *forgetPassword {
					_, _ = fmt.Fprintln(d.IO.Out, "Removed all cached web sessions and stored passwords.")
					return nil
				}
				_, _ = fmt.Fprintln(d.IO.Out, "Removed all cached web sessions.")
				return nil
			}
			if trimmedAppleID == "" {
				return d.UsageError("provide --apple-id or --all")
			}
			if *forgetPassword {
				if err := deleteStoredWebPasswordFn(trimmedAppleID); err != nil {
					return fmt.Errorf("web auth logout failed to forget saved password: %w", err)
				}
			}
			if err := deleteWebSessionFn(trimmedAppleID); err != nil {
				if *forgetPassword {
					return fmt.Errorf("web auth logout forgot saved password but failed to remove session: %w", err)
				}
				return fmt.Errorf("web auth logout failed: %w", err)
			}
			if *forgetPassword {
				_, _ = fmt.Fprintf(d.IO.Out, "Removed cached web session and stored password for %s.\n", trimmedAppleID)
				return nil
			}
			_, _ = fmt.Fprintf(d.IO.Out, "Removed cached web session for %s.\n", trimmedAppleID)
			return nil
		},
	}
}
