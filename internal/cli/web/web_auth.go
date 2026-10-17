package web

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	webPasswordEnv             = "ASC_WEB_PASSWORD"
	webAppleIDEnv              = "ASC_WEB_APPLE_ID"
	webDontStorePasswordEnv    = "ASC_WEB_DONT_STORE_PASSWORD"
	webTwoFactorCodeCommandEnv = "ASC_WEB_2FA_CODE_COMMAND"
	webTwoFactorMethodEnv      = "ASC_WEB_2FA_METHOD"
	webTwoFactorPhoneIDEnv     = "ASC_WEB_2FA_PHONE_ID"
	webTwoFactorCommandTimeout = 60 * time.Second
	passwordReadPollInterval   = 100 * time.Millisecond
)

func webPasswordEnvDisplay() string {
	return webPasswordEnv
}

func webPasswordEnvAssignmentExample() string {
	return webPasswordEnvDisplay() + "=\"...\""
}

var (
	openTTYFn                                = openTTY
	promptTwoFactorCodeFn                    = promptTwoFactorCodeInteractive
	promptSelectTrustedPhoneFn               = promptSelectTrustedPhoneInteractive
	twoFactorPromptAvailableFn               = twoFactorPromptAvailable
	promptPasswordFn                         = promptPasswordInteractive
	readTwoFactorCodeFromCommandFn           = readTwoFactorCodeFromCommand
	webLoginFn                               = webcore.Login
	prepareTwoFactorChallengeFn              = webcore.PrepareTwoFactorChallenge
	ensureTwoFactorCodeRequestedFn           = webcore.EnsureTwoFactorCodeRequested
	persistWebSessionFn                      = webcore.PersistSession
	submitTwoFactorCodeFn                    = webcore.SubmitTwoFactorCode
	signalProcessInterruptFn                 = signalProcessInterrupt
	termReadPasswordFn                       = term.ReadPassword
	termIsTerminalFn                         = term.IsTerminal
	tryResumeSessionFn                       = webcore.TryResumeSession
	tryResumeSessionFromSourceFn             = webcore.TryResumeSessionFromSource
	tryResumeLastFn                          = webcore.TryResumeLastSession
	loadCachedSessionFn                      = webcore.LoadCachedSession
	loadCachedSessionFromSourceFn            = webcore.LoadCachedSessionFromSource
	loadLastCachedSessionFn                  = webcore.LoadLastCachedSession
	defaultCachedAppleIDFn                   = webcore.DefaultCachedAppleIDWithSource
	webLoginWithClientFn                     = webcore.LoginWithClient
	loadStoredWebPasswordFn                  = webcore.LoadPassword
	storeStoredWebPasswordFn                 = webcore.StorePassword
	storedWebPasswordExistsFn                = webcore.PasswordStored
	deleteStoredWebPasswordFn                = webcore.DeletePassword
	deleteAllStoredWebPasswordsFn            = webcore.DeleteAllPasswords
	deleteWebSessionFn                       = webcore.DeleteSession
	deleteStaleWebSessionFn                  = webcore.DeleteSessionIfMatches
	deleteAllWebSessionsFn                   = webcore.DeleteAllSessions
	selectWebProviderFn                      = webcore.SelectProvider
	resolveSessionFn               any       = resolveSession
	resolveSessionWithoutPersistFn any       = resolveSessionWithoutPersist
	twoFactorStatusWriter          io.Writer = os.Stderr
	sessionExpiredWriter           io.Writer = os.Stderr
	sessionCacheWarningWriter      io.Writer = os.Stderr
	sessionDefaultNoticeWriter     io.Writer = os.Stderr
	passwordStoreWarningWriter     io.Writer = os.Stderr
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

func webPasswordStorageDisabled() bool {
	if webPasswordStorageOptedOut() {
		return true
	}
	return webcore.PasswordStoreBypassed()
}

func webPasswordStorageOptedOut() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(webDontStorePasswordEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "", "0", "false", "no", "off":
		return false
	default:
		warnInvalidWebPasswordOptOutOnce(value)
		return true
	}
}

func warnInvalidWebPasswordOptOutOnce(value string) {
	invalidWebPasswordOptOutMu.Lock()
	if _, warned := invalidWebPasswordOptOutValues[value]; warned {
		invalidWebPasswordOptOutMu.Unlock()
		return
	}
	invalidWebPasswordOptOutValues[value] = struct{}{}
	invalidWebPasswordOptOutMu.Unlock()

	if passwordStoreWarningWriter != nil {
		_, _ = fmt.Fprintf(
			passwordStoreWarningWriter,
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
}

func printWebAuthStatus(status webAuthStatus, format string, pretty bool) error {
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
	return shared.PrintOutputRows(status, format, pretty, []string{"Field", "Value"}, rows)
}

func expiredWebAuthStatus(appleID string) webAuthStatus {
	status := webAuthStatus{
		Authenticated: false,
		AppleID:       strings.TrimSpace(appleID),
	}
	cached, ok, err := loadExpiredWebAuthCache(status.AppleID)
	if err == nil && ok && cached != nil {
		if status.AppleID == "" {
			status.AppleID = strings.TrimSpace(cached.UserEmail)
		}
		status.DeveloperTeamID = strings.TrimSpace(cached.DeveloperTeamID)
	}
	status.PasswordStored = storedWebPasswordStatus(status.AppleID)
	return status
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

func callResolveSessionFn(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	return callSessionResolverHook(ctx, resolveSessionFn, "web session resolver", appleID, password, twoFactorCode, twoFactorCodeCommand)
}

func readPasswordFromInput(ctx context.Context) (string, error) {
	password := readPasswordFromEnv()
	if webPasswordProvided(password) {
		return password, nil
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

func readPasswordFromTerminalFD(ctx context.Context, writer io.Writer) (string, error) {
	if writer == nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	if _, err := fmt.Fprint(writer, "Apple Account password: "); err != nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	passwordBytes, err := termReadPasswordFn(0)
	_, _ = fmt.Fprintln(writer)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("password prompt interrupted: %w", ctxErr)
		}
		return "", fmt.Errorf("failed to read password")
	}
	password := string(passwordBytes)
	if !webPasswordProvided(password) {
		return "", fmt.Errorf("password is required")
	}
	return password, nil
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

func readTwoFactorCodeFrom(reader io.Reader, writer io.Writer) (string, error) {
	if reader == nil || writer == nil {
		return "", fmt.Errorf("2fa required: unable to prompt for code")
	}
	if _, err := fmt.Fprint(writer, "Two-factor code required. Enter 2FA code: "); err != nil {
		return "", fmt.Errorf("2fa required: unable to prompt for code")
	}
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("2fa required: failed to read 2fa code")
	}
	code := strings.TrimSpace(line)
	if code == "" {
		return "", fmt.Errorf("2fa required: empty 2fa code")
	}
	return code, nil
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

func twoFactorPromptAvailable() bool {
	if tty, err := openTTYFn(); err == nil {
		_ = tty.Close()
		return true
	}
	return termIsTerminalFn(int(os.Stdin.Fd()))
}

func twoFactorInputError(err error, code shared.DiagnosticCode) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), code, "--two-factor-code-command")
}

func promptSelectTrustedPhoneInteractive(phones []appleauth.TrustedPhoneNumber) (*appleauth.TrustedPhoneNumber, error) {
	if tty, err := openTTYFn(); err == nil {
		defer func() { _ = tty.Close() }()
		return readPhoneSelectionFromTerminal(tty, tty, phones)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readPhoneSelectionFromTerminal(os.Stdin, os.Stderr, phones)
	}
	return nil, &appleauth.AmbiguousTrustedPhoneNumberError{Available: phones}
}

func readPhoneSelectionFromTerminal(reader io.Reader, writer io.Writer, phones []appleauth.TrustedPhoneNumber) (*appleauth.TrustedPhoneNumber, error) {
	if writer == nil || reader == nil {
		return nil, &appleauth.AmbiguousTrustedPhoneNumberError{Available: phones}
	}
	_, _ = fmt.Fprintln(writer, "Multiple trusted phone numbers available for verification:")
	for i, phone := range phones {
		dest := strings.TrimSpace(phone.NumberWithDialCode)
		if dest != "" {
			_, _ = fmt.Fprintf(writer, "  %d) %s (id: %d)\n", i+1, dest, phone.ID)
		} else {
			_, _ = fmt.Fprintf(writer, "  %d) id: %d\n", i+1, phone.ID)
		}
	}
	_, _ = fmt.Fprintf(writer, "Select a phone number [1-%d] or enter phone ID: ", len(phones))
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to read phone selection: %w", err)
	}
	input := strings.TrimSpace(line)
	if input == "" {
		return nil, &appleauth.AmbiguousTrustedPhoneNumberError{Available: phones}
	}
	idx, err := strconv.Atoi(input)
	if err == nil {
		if idx >= 1 && idx <= len(phones) {
			return &phones[idx-1], nil
		}
		for i := range phones {
			if phones[i].ID == idx {
				return &phones[i], nil
			}
		}
	}
	return nil, &appleauth.TrustedPhoneNumberNotFoundError{ID: idx, Available: phones}
}

func classifyTwoFactorChallengeSetupError(err error) error {
	if err == nil {
		return nil
	}
	prefix := "2fa challenge setup failed: "
	msg := err.Error()
	if strings.HasPrefix(msg, "2fa failed: ") {
		msg = strings.TrimPrefix(msg, "2fa failed: ")
	}
	var ambiguous *appleauth.AmbiguousTrustedPhoneNumberError
	if errors.As(err, &ambiguous) {
		return shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("%s%s", prefix, msg)),
			shared.DiagnosticRequiredInputMissing,
			"--two-factor-phone-id",
		)
	}
	var notFound *appleauth.TrustedPhoneNumberNotFoundError
	if errors.As(err, &notFound) {
		return shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("%s%s", prefix, msg)),
			shared.DiagnosticInvalidInput,
			"--two-factor-phone-id",
		)
	}
	var unsupported *appleauth.UnsupportedTwoFactorMethodError
	if errors.As(err, &unsupported) {
		return shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("%s%s", prefix, msg)),
			shared.DiagnosticInvalidInput,
			"--two-factor-method",
		)
	}
	if errors.Is(err, appleauth.ErrNoTrustedPhoneNumbers) {
		return shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("%s%s", prefix, msg)),
			shared.DiagnosticDependencyFailed,
			"--two-factor-method",
		)
	}
	return fmt.Errorf("2fa challenge setup failed: %w", err)
}

type webTwoFactorOptionsContextKey struct{}

type webTwoFactorOptions struct {
	method  string
	phoneID int
}

func contextWithTwoFactorOptions(ctx context.Context, method string, phoneID int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, webTwoFactorOptionsContextKey{}, webTwoFactorOptions{
		method:  strings.TrimSpace(method),
		phoneID: phoneID,
	})
}

func twoFactorOptionsFromContext(ctx context.Context) (string, int) {
	if ctx == nil {
		return "", 0
	}
	if opts, ok := ctx.Value(webTwoFactorOptionsContextKey{}).(webTwoFactorOptions); ok {
		return opts.method, opts.phoneID
	}
	return "", 0
}

func twoFactorCodeCommandShellArgs(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/d", "/s", "/c", command}
	}
	// Avoid login-shell startup noise contaminating stdout before the 2FA code.
	return []string{"-c", command}
}

func readTwoFactorCodeFromCommand(ctx context.Context, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("2fa required: empty 2fa code command")
	}

	commandCtx, cancel := shared.ContextWithResolvedTimeout(shared.ContextWithoutTimeout(ctx), webTwoFactorCommandTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(commandCtx, "cmd", twoFactorCodeCommandShellArgs(command)...)
	} else {
		cmd = exec.CommandContext(commandCtx, "/bin/sh", twoFactorCodeCommandShellArgs(command)...)
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

type webSignInContextKey struct{}

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
	message := "no Apple web session is cached"
	trimmedAppleID := strings.TrimSpace(appleID)
	if trimmedAppleID != "" {
		message = "no usable Apple web session for " + shared.SanitizeTerminal(trimmedAppleID)
	}
	hint := fmt.Sprintf(
		"asc web commands need a signed-in Apple Account session, and signing in needs an interactive terminal for the password and two-factor code. Run 'asc web auth login --apple-id EMAIL' in a terminal, or load a session exported elsewhere with 'asc web auth import --file FILE'. Unattended sign-in needs %s and %s, plus --apple-id or %s.",
		webPasswordEnvDisplay(),
		webTwoFactorCodeCommandEnv,
		webAppleIDEnv,
	)
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

// passwordRequiredUsageError is the usage error for a sign-in that reached the
// password step without one: `asc web auth login` without a password source,
// or an interactive `web apps create` prompt answered with an empty password.
func passwordRequiredUsageError() error {
	return shared.UsageError(fmt.Sprintf("password is required: run in a terminal for an interactive prompt or set %s", webPasswordEnvDisplay()))
}

// missingAppleIDUsageError is the usage error `asc web auth login` returns
// when it has no Apple ID to sign in with: nothing was passed and the session
// cache holds nothing to default to. It carries errNoCachedWebSession so
// command-specific session diagnostics can tell a missing session from an
// expired one.
func missingAppleIDUsageError() error {
	return shared.NewErrorWithCause(
		shared.WithDiagnostic(
			shared.UsageError("--apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'"),
			shared.DiagnosticRequiredInputMissing,
			"--apple-id",
		),
		errNoCachedWebSession,
	)
}

// ambiguousAppleIDUsageError is the usage error for a web command that could
// default to any of several cached sessions and therefore refuses to guess.
func ambiguousAppleIDUsageError(appleIDs []string) error {
	return shared.WithDiagnostic(
		shared.UsageError(fmt.Sprintf("--apple-id is required: multiple cached web sessions are available (%s); pass --apple-id to choose one", strings.Join(appleIDs, ", "))),
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
func printEnvAppleIDNotice(appleID string) {
	if sessionDefaultNoticeWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(sessionDefaultNoticeWriter, "Using web session for %s from %s; pass --apple-id to override\n", shared.SanitizeTerminal(appleID), webAppleIDEnv)
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
func resolveDefaultCachedAppleID() (appleID string, source webcore.CachedSessionSource, cacheEmpty bool, err error) {
	appleID, source, err = defaultCachedAppleIDFn()
	if err == nil {
		appleID = strings.TrimSpace(appleID)
		if appleID == "" {
			return "", webcore.CachedSessionSourceUnknown, true, webcore.ErrNoCachedSession
		}
		if sessionDefaultNoticeWriter != nil {
			_, _ = fmt.Fprintf(sessionDefaultNoticeWriter, "Using cached web session for %s; pass --apple-id to override\n", shared.SanitizeTerminal(appleID))
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
		if sessionCacheWarningWriter != nil {
			_, _ = fmt.Fprintf(sessionCacheWarningWriter, "Warning: listing cached web sessions failed: %v\n", err)
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

func resolveNonInteractiveWebPassword(appleID, provided string, skipStored bool) resolvedWebPassword {
	if webPasswordProvided(provided) {
		return resolvedWebPassword{value: provided, source: webPasswordSourceProvided}
	}
	if password := readPasswordFromEnv(); webPasswordProvided(password) {
		return resolvedWebPassword{value: password, source: webPasswordSourceEnv}
	}
	if skipStored || webPasswordStorageDisabled() || strings.TrimSpace(appleID) == "" {
		return resolvedWebPassword{}
	}
	password, ok, err := loadStoredWebPasswordFn(appleID)
	if err != nil {
		printPasswordStoreWarning(passwordStoreWarningWriter, "load", err)
		return resolvedWebPassword{}
	}
	if !ok || !webPasswordProvided(password) {
		return resolvedWebPassword{}
	}
	return resolvedWebPassword{value: password, source: webPasswordSourceStored}
}

func persistPromptedWebPassword(appleID string, password resolvedWebPassword) {
	if password.source != webPasswordSourcePrompted || webPasswordStorageDisabled() {
		return
	}
	if err := storeStoredWebPasswordFn(appleID, password.value); err != nil {
		printPasswordStoreWarning(passwordStoreWarningWriter, "save", err)
	}
}

func printStoredPasswordRejected() {
	if passwordStoreWarningWriter != nil {
		_, _ = fmt.Fprintln(passwordStoreWarningWriter, "Saved Apple Account password was rejected; enter the current password to replace it.")
	}
}

func storedWebPasswordStatus(appleID string) bool {
	if webPasswordStorageDisabled() || strings.TrimSpace(appleID) == "" {
		return false
	}
	stored, err := storedWebPasswordExistsFn(appleID)
	if err != nil {
		printPasswordStoreWarning(passwordStoreWarningWriter, "check", err)
		return false
	}
	return stored
}

// twoFactorSubmitFailure describes a failed 2FA submission. Apple accepting the
// code and then failing the App Store Connect session bootstrap is reported as a
// finalization failure carrying the HTTP status, because calling it a
// verification failure misleads users into retrying a code that was accepted.
func twoFactorSubmitFailure(err error, afterPhoneDelivery bool) error {
	stage := "2fa verification failed"
	var finalizeErr *webcore.TwoFactorFinalizationError
	if errors.As(err, &finalizeErr) {
		stage = "2fa finalization failed"
	}
	if afterPhoneDelivery {
		stage += " after switching to phone delivery"
	}
	err = fmt.Errorf("%s: %w", stage, err)
	if errors.Is(err, webcore.ErrTwoFactorCodeRejected) {
		return shared.WithDiagnostic(err, shared.DiagnosticAuthenticationRejected, "")
	}
	return err
}

func loginWithOptionalTwoFactorUsing(ctx context.Context, progressMessage, appleID, password, twoFactorCode string, loginFn func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error), twoFactorStarted func(), readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	// Credential/keychain prompts may consume the initial command budget.
	ctx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	session, err := withWebSpinnerValue(progressMessage, func() (*webcore.AuthSession, error) {
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
		if code == "" && command == "" && !twoFactorPromptAvailableFn() {
			return nil, twoFactorInputError(errNoTwoFactorCodeSource, shared.DiagnosticRequiredInputMissing)
		}
		method, phoneID := twoFactorOptionsFromContext(ctx)
		session.SetRequestedTwoFactor(method, phoneID)
		if twoFactorPromptAvailableFn() {
			session.SetSelectPhone(func(ctx context.Context, phones []appleauth.TrustedPhoneNumber) (*appleauth.TrustedPhoneNumber, error) {
				return promptSelectTrustedPhoneFn(phones)
			})
		}
		challenge, prepErr := prepareTwoFactorChallengeFn(ctx, session)
		if prepErr != nil {
			return nil, classifyTwoFactorChallengeSetupError(prepErr)
		}

		writeDeliveryNotice := func(destination string) {
			destination = strings.TrimSpace(destination)
			if destination == "" || twoFactorStatusWriter == nil {
				return
			}
			_, _ = fmt.Fprintf(twoFactorStatusWriter, "Verification code sent to %s.\n", destination)
		}
		writeFallbackGuidance := func(usingCommand bool) {
			if twoFactorStatusWriter == nil {
				return
			}
			if usingCommand {
				_, _ = fmt.Fprintln(twoFactorStatusWriter, "Trusted-device verification was rejected. Re-running the configured 2FA code command for the phone verification code.")
				return
			}
			_, _ = fmt.Fprintln(twoFactorStatusWriter, "Trusted-device verification was rejected. Enter the phone verification code that was just sent.")
		}
		writeInitialPhoneFallbackGuidance := func() {
			if challenge == nil || !challenge.PhoneFallbackAvailable || twoFactorStatusWriter == nil {
				return
			}
			destination := strings.TrimSpace(challenge.Destination)
			if destination != "" {
				_, _ = fmt.Fprintf(twoFactorStatusWriter, "Need a phone verification code? Enter an incorrect trusted-device code once; Apple will then deliver a verification code to %s.\n", destination)
				return
			}
			_, _ = fmt.Fprintln(twoFactorStatusWriter, "Need a phone verification code? Enter an incorrect trusted-device code once; Apple will then deliver a verification code to your registered phone number.")
		}
		readCode := func() (string, error) {
			if command != "" {
				read := readTwoFactorCodeFromCommandFn
				if readCommandCode != nil {
					read = readCommandCode
				}
				code, err := read(ctx, command)
				if err != nil {
					return "", twoFactorInputError(err, shared.DiagnosticDependencyFailed)
				}
				return code, nil
			}
			code, err := promptTwoFactorCodeFn()
			if err != nil {
				return "", twoFactorInputError(err, shared.DiagnosticRequiredInputMissing)
			}
			return code, nil
		}
		if code == "" {
			if command == "" && (challenge == nil || !challenge.IsPhoneMethod()) {
				writeInitialPhoneFallbackGuidance()
			}
			if challenge != nil && challenge.IsPhoneMethod() {
				challenge, prepErr = ensureTwoFactorCodeRequestedFn(ctx, session)
				if prepErr != nil {
					return nil, classifyTwoFactorChallengeSetupError(prepErr)
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
			verifyCtx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
			defer cancel()
			return withWebSpinner("Verifying two-factor code", func() error {
				return submitTwoFactorCodeFn(verifyCtx, session, code)
			})
		}
		if err := submitCode(code); err != nil {
			var phoneCodeRequestedErr *appleauth.PhoneCodeRequestedError
			if errors.As(err, &phoneCodeRequestedErr) {
				writeDeliveryNotice(phoneCodeRequestedErr.Destination)
				writeFallbackGuidance(command != "")
				resolvedCode, codeErr := readCode()
				if codeErr != nil {
					return nil, codeErr
				}
				if err := submitCode(resolvedCode); err != nil {
					return nil, twoFactorSubmitFailure(err, true)
				}
				return session, nil
			}
			return nil, twoFactorSubmitFailure(err, false)
		}
		return session, nil
	}
	return nil, err
}

func loginWithOptionalTwoFactor(ctx context.Context, appleID, password, twoFactorCode string, twoFactorStarted func(), readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	return loginWithOptionalTwoFactorUsing(ctx, "Signing in to Apple web session", appleID, password, twoFactorCode, webLoginFn, twoFactorStarted, readCommandCode, twoFactorCodeCommand...)
}

func loginWithOptionalTwoFactorClientTracked(ctx context.Context, client *http.Client, appleID, password, twoFactorCode string, readCommandCode twoFactorCodeCommandReader, twoFactorCodeCommand ...string) (*webcore.AuthSession, bool, error) {
	twoFactorStarted := false
	session, err := loginWithOptionalTwoFactorUsing(ctx, "Refreshing expired web session", appleID, password, twoFactorCode, func(ctx context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return webLoginWithClientFn(ctx, client, credentials)
	}, func() {
		twoFactorStarted = true
	}, readCommandCode, twoFactorCodeCommand...)
	return session, twoFactorStarted, err
}

func tryResumeWebSession(ctx context.Context, appleID string) (*webcore.AuthSession, bool, error) {
	var (
		session *webcore.AuthSession
		ok      bool
	)
	err := withWebSpinner("Checking cached web session", func() error {
		var err error
		session, ok, err = tryResumeSessionFn(ctx, appleID)
		return err
	})
	return session, ok, err
}

func tryResumeLastWebSession(ctx context.Context) (*webcore.AuthSession, bool, error) {
	var (
		session *webcore.AuthSession
		ok      bool
	)
	err := withWebSpinner("Checking cached web session", func() error {
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
	twoFactorMethod      string
	twoFactorPhoneID     int
}

func tryResumeKnownWebSession(ctx context.Context, appleID string) (*webcore.AuthSession, bool, bool, error) {
	if appleID != "" {
		resumed, ok, err := tryResumeWebSession(ctx, appleID)
		return resumed, ok, errors.Is(err, webcore.ErrCachedSessionExpired), err
	}
	resumed, ok, err := tryResumeLastWebSession(ctx)
	return resumed, ok, errors.Is(err, webcore.ErrCachedSessionExpired), err
}

func resolveKnownWebSession(ctx context.Context, appleID string) (*webcore.AuthSession, bool, bool, error) {
	resumed, ok, cacheExpired, err := tryResumeKnownWebSession(ctx, appleID)
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

func resolveWebSession(ctx context.Context, appleID, password, twoFactorCode string, opts webSessionResolveOptions) (*webcore.AuthSession, string, error) {
	shared.ApplyRootLoggingOverrides()
	if err := webcore.SessionBackendConflict(); err != nil {
		return nil, "", shared.UsageError(err.Error())
	}

	resolvedAppleID := strings.TrimSpace(appleID)
	// ASC_WEB_APPLE_ID sits between the flag and the cached-session default: it
	// names the account before any cache lookup, so neither the last-session
	// pointer nor an ambiguous cache can decide for a caller who already said
	// which Apple Account this invocation belongs to.
	if resolvedAppleID == "" {
		if fromEnv := envAppleID(); fromEnv != "" {
			resolvedAppleID = fromEnv
			printEnvAppleIDNotice(resolvedAppleID)
		}
	}
	twoFactorCode = strings.TrimSpace(twoFactorCode)
	command := strings.TrimSpace(opts.twoFactorCodeCommand)
	if command == "" {
		command = strings.TrimSpace(os.Getenv(webTwoFactorCodeCommandEnv))
	}
	resolvedTwoFactorMethod := strings.TrimSpace(opts.twoFactorMethod)
	resolvedTwoFactorPhoneID := opts.twoFactorPhoneID

	if resolvedTwoFactorMethod == "" {
		ctxMethod, _ := twoFactorOptionsFromContext(ctx)
		resolvedTwoFactorMethod = ctxMethod
	}
	if resolvedTwoFactorMethod == "" {
		resolvedTwoFactorMethod = strings.TrimSpace(os.Getenv(webTwoFactorMethodEnv))
	}

	if resolvedTwoFactorPhoneID == 0 {
		_, ctxPhoneID := twoFactorOptionsFromContext(ctx)
		resolvedTwoFactorPhoneID = ctxPhoneID
	}
	if resolvedTwoFactorPhoneID == 0 {
		if envPhone := strings.TrimSpace(os.Getenv(webTwoFactorPhoneIDEnv)); envPhone != "" {
			parsed, err := strconv.Atoi(envPhone)
			if err != nil {
				return nil, "", shared.WithDiagnostic(
					shared.NewValidationError(fmt.Errorf("invalid %s: %q must be an integer", webTwoFactorPhoneIDEnv, envPhone)),
					shared.DiagnosticInvalidInput,
					"--two-factor-phone-id",
				)
			}
			resolvedTwoFactorPhoneID = parsed
		}
	}

	ctx = contextWithTwoFactorOptions(ctx, resolvedTwoFactorMethod, resolvedTwoFactorPhoneID)
	expiredNoticePrinted := false
	printExpiredNotice := func() {
		if expiredNoticePrinted {
			return
		}
		printExpiredSessionNotice(sessionExpiredWriter)
		expiredNoticePrinted = true
	}

	var (
		expiredCachedSession  *webcore.AuthSession
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
		twoFactorCodeConsumed = true
		// The retry raises a brand-new Apple challenge, so a replacement code has
		// just been delivered. Only a prompted operator has to be told which one
		// to type: the burned digits are still on screen from the first prompt,
		// and retyping them earns nothing but an opaque rejection. A configured
		// command fetches its own replacement and needs no notice. This is
		// reached only once a replacement is known to be obtainable.
		if command == "" && twoFactorStatusWriter != nil {
			_, _ = fmt.Fprintln(twoFactorStatusWriter, "The previous verification code was consumed by the expired session. Enter the new code Apple just sent, not the previous one.")
		}
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
			resumed, ok, cacheExpired, err = resolveKnownWebSession(ctx, targetAppleID)
		} else {
			resumed, ok, cacheExpired, err = resolveKnownWebSessionFromSource(ctx, targetAppleID, source)
		}
		if errors.Is(err, webcore.ErrSessionKeychainBypassed) {
			return nil, "", false, shared.UsageError(err.Error())
		}
		if err != nil {
			printCacheLookupWarning(sessionCacheWarningWriter, err)
			return nil, "", false, nil
		}
		if ok {
			return resumed, "cache", true, nil
		}
		if !cacheExpired {
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
			printCacheLookupWarning(sessionCacheWarningWriter, fmt.Errorf("loading expired cached web session failed: %w", err))
			expiredCachedSession = nil
			printExpiredNotice()
			return nil, "", false, nil
		}
		if !cachedOK || expiredCachedSession == nil || expiredCachedSession.Client == nil {
			expiredCachedSession = nil
			printExpiredNotice()
			return nil, "", false, nil
		}

		reauthAppleID := strings.TrimSpace(targetAppleID)
		if reauthAppleID == "" {
			reauthAppleID = strings.TrimSpace(expiredCachedSession.UserEmail)
		}
		if reauthAppleID == "" {
			return nil, "", false, shared.UsageError("last cached web session predates stored Apple ID metadata; re-run once with --apple-id to refresh the cache")
		}
		if resolvedAppleID == "" {
			resolvedAppleID = reauthAppleID
		}

		silentPassword := resolveNonInteractiveWebPassword(reauthAppleID, password, skipStoredPassword)
		if !webPasswordProvided(silentPassword.value) {
			printExpiredNotice()
			return nil, "", false, nil
		}

		session, twoFactorStarted, loginErr := loginWithOptionalTwoFactorClientTracked(ctx, expiredCachedSession.Client, reauthAppleID, silentPassword.value, twoFactorCode, readCommandTwoFactorCode, command)
		if loginErr == nil {
			if opts.persistAutoReauth != nil {
				opts.persistAutoReauth(session)
			}
			return session, "auto-reauth", true, nil
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
			printCacheLookupWarning(sessionCacheWarningWriter, staleSessionDiscardWarning(discardErr))
			markTwoFactorCodeConsumed()
		}
		if errors.Is(loginErr, webcore.ErrInvalidAppleAccountCredentials) {
			if silentPassword.source != webPasswordSourceStored {
				return nil, "", false, fmt.Errorf("web auth auto-reauth failed: %w", loginErr)
			}
			printStoredPasswordRejected()
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
		defaultAppleID, defaultSource, cacheEmpty, defaultErr := resolveDefaultCachedAppleID()
		switch {
		case defaultErr == nil:
			resolvedAppleID = defaultAppleID
		case opts.promptAppleID == nil || !cacheEmpty:
			var ambiguous *webcore.AmbiguousCachedSessionError
			if errors.As(defaultErr, &ambiguous) {
				return nil, "", ambiguousAppleIDUsageError(ambiguous.AppleIDs)
			}
			if isWebSignIn(ctx) {
				return nil, "", missingAppleIDUsageError()
			}
			return nil, "", newMissingWebSessionError("", publicAPIAlternativeFromContext(ctx))
		default:
			if err := opts.promptAppleID(&resolvedAppleID); err != nil {
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
	resolvedPassword := fallbackPassword
	if !webPasswordProvided(resolvedPassword.value) {
		resolvedPassword = resolveNonInteractiveWebPassword(resolvedAppleID, password, skipStoredPassword)
	}
	if !webPasswordProvided(resolvedPassword.value) {
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
			return nil, "", passwordRequiredUsageError()
		}
		// No usable cached session and no way to sign in here: the command
		// needs a session it cannot create. Say so with the sign-in hint rather
		// than a missing-password usage page.
		return nil, "", newMissingWebSessionError(resolvedAppleID, publicAPIAlternativeFromContext(ctx))
	}
	// Without a cached cookie jar there is no trusted-device cookie, so Apple
	// always answers the password step with a 2FA challenge. Refuse before
	// signing in when nothing here can supply the code.
	if expiredCachedSession == nil && twoFactorCode == "" && command == "" && !twoFactorPromptAvailableFn() {
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
		if _, discardErr := deleteStaleWebSessionFn(strings.TrimSpace(serverRejectedAppleID), serverRejectedCache); discardErr != nil && sessionCacheWarningWriter != nil {
			_, _ = fmt.Fprintf(sessionCacheWarningWriter, "Warning: %v.\n", staleSessionDiscardWarning(discardErr))
		}
	}
	login := func(candidate resolvedWebPassword) (*webcore.AuthSession, bool, error) {
		// Interactive password and 2FA entry can outlast the caller's request
		// deadline, so bound every attempt with a fresh timeout derived from the
		// untimed parent context instead of an already-expired one.
		loginCtx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
		defer cancel()
		code := twoFactorCode
		if twoFactorCodeConsumed {
			code = ""
		}
		if expiredCachedSession != nil && expiredCachedSession.Client != nil {
			return loginWithOptionalTwoFactorClientTracked(loginCtx, expiredCachedSession.Client, resolvedAppleID, candidate.value, code, readCommandTwoFactorCode, command)
		}
		reachedTwoFactor := false
		session, err := loginWithOptionalTwoFactor(loginCtx, resolvedAppleID, candidate.value, code, func() {
			reachedTwoFactor = true
		}, readCommandTwoFactorCode, command)
		if err == nil || reachedTwoFactor {
			discardServerRejectedCache()
		}
		return session, false, err
	}
	loginWithPromptedFreshFallback := func(candidate resolvedWebPassword) (*webcore.AuthSession, error) {
		session, twoFactorStarted, err := login(candidate)
		blockedByTwoFactor := twoFactorStarted && !webcore.IsStaleSessionAfterTwoFactor(err)
		if err == nil || candidate.source != webPasswordSourcePrompted || expiredCachedSession == nil || blockedByTwoFactor || errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) {
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
			printCacheLookupWarning(sessionCacheWarningWriter, staleSessionDiscardWarning(discardErr))
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
		printStoredPasswordRejected()
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
		err = fmt.Errorf("web auth login failed: %w", err)
		if errors.Is(err, webcore.ErrAppleAccountActionRequired) {
			return nil, "", shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "")
		}
		return nil, "", err
	}
	persistPromptedWebPassword(resolvedAppleID, resolvedPassword)
	if opts.persistFresh != nil {
		if err := opts.persistFresh(session); err != nil {
			return nil, "", err
		}
	}
	return session, "fresh", nil
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

func persistAutoReauthResolvedSession(session *webcore.AuthSession) {
	_ = persistWebSessionFn(session)
}

func selectResolvedWebSessionProvider(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
	if selection.ProviderID == 0 && strings.TrimSpace(selection.PublicProviderID) == "" {
		return nil
	}
	// Interactive 2FA may outlive the request budget used to start login.
	// Renew only the internal timeout, preserving caller cancellation.
	providerCtx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	if err := selectWebProviderFn(providerCtx, session, selection); err != nil {
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

func resolveSession(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = twoFactorCodeCommand[0]
	}
	method, phoneID := twoFactorOptionsFromContext(ctx)
	return resolveWebSession(ctx, appleID, password, twoFactorCode, webSessionResolveOptions{
		resolvePassword:      resolveSessionPassword,
		persistFresh:         persistFreshResolvedSession,
		persistAutoReauth:    persistAutoReauthResolvedSession,
		twoFactorCodeCommand: command,
		twoFactorMethod:      method,
		twoFactorPhoneID:     phoneID,
	})
}

func resolveSessionWithoutPersist(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = twoFactorCodeCommand[0]
	}
	method, phoneID := twoFactorOptionsFromContext(ctx)
	return resolveWebSession(ctx, appleID, password, twoFactorCode, webSessionResolveOptions{
		resolvePassword:      resolveSessionPassword,
		twoFactorCodeCommand: command,
		twoFactorMethod:      method,
		twoFactorPhoneID:     phoneID,
	})
}

func hasProviderSelection(selection webcore.ProviderSelection) bool {
	return selection.ProviderID != 0 || strings.TrimSpace(selection.PublicProviderID) != ""
}

func callResolveSessionForProviderSelection(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string, selection webcore.ProviderSelection) (*webcore.AuthSession, string, error) {
	if !hasProviderSelection(selection) {
		return callResolveSessionFn(ctx, appleID, password, twoFactorCode, twoFactorCodeCommand)
	}
	return callSessionResolverHook(ctx, resolveSessionWithoutPersistFn, "resolveSessionWithoutPersistFn", appleID, password, twoFactorCode, twoFactorCodeCommand)
}

// WebAuthCommand returns the detached web auth command group.
func WebAuthCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc web auth <subcommand> [flags]",
		ShortHelp:  "Manage Apple web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Apple web-session authentication used by "asc web" commands.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAuthLoginCommand(),
			WebAuthStatusCommand(),
			WebAuthCapabilitiesCommand(),
			WebAuthExportCommand(),
			WebAuthImportCommand(),
			WebAuthLogoutCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAuthLoginCommand creates or refreshes a web session.
func WebAuthLoginCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth login", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email (defaults to "+webAppleIDEnv+", then the last or only cached session)")
	twoFactorCodeCommand := fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required")
	twoFactorMethod := fs.String("two-factor-method", "", "Two-factor verification delivery method: 'sms' (or 'phone') or 'trusted-device'")
	twoFactorPhoneID := fs.Int("two-factor-phone-id", 0, "Specific trusted phone number ID for SMS verification (defaults to "+webTwoFactorPhoneIDEnv+")")
	providerID := fs.Int64("provider-id", 0, "Numeric App Store Connect provider ID to select for this web session")
	publicProviderID := fs.String("public-provider-id", "", "Public App Store Connect provider/team ID to select for this web session")
	output := shared.BindOutputFlags(fs)

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
  - --two-factor-method ('sms' or 'trusted-device'; defaults to %s)
  - --two-factor-phone-id (specific phone ID for SMS; defaults to %s)

Without a terminal, a fresh sign-in needs a 2FA code command; without one it
fails before the password is sent, so Apple does not send a code nobody can enter.

Phone-code fallback (including SMS):
  - interactive: if Apple offers a registered phone fallback, enter an incorrect trusted-device code once
  - Apple then delivers a phone verification code and asc prompts again
  - automated: asc reruns the configured 2FA code command after phone fallback

Provider selection:
  - --public-provider-id selects the public App Store Connect provider/team ID
  - --provider-id selects Apple's numeric App Store Connect provider ID

Examples:
  # Interactive (in a terminal)
  asc web auth login --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com" --two-factor-method sms
  asc web auth login --apple-id "user@example.com" --public-provider-id "Z4N6A5FQKW"

  # Non-interactive (agents, CI): supply the password and a 2FA code source
  %s %s='osascript /path/to/get-apple-2fa-code.scpt' asc web auth login --apple-id "user@example.com"
  %s %s=sms %s=1 %s='osascript /path/to/get-apple-2fa-code.scpt' asc web auth login --apple-id "user@example.com"`,
			webAppleIDEnv,
			webPasswordEnvDisplay(),
			webDontStorePasswordEnv,
			webTwoFactorCodeCommandEnv,
			webTwoFactorMethodEnv,
			webTwoFactorPhoneIDEnv,
			webPasswordEnvAssignmentExample(),
			webTwoFactorCodeCommandEnv,
			webPasswordEnvAssignmentExample(),
			webTwoFactorMethodEnv,
			webTwoFactorPhoneIDEnv,
			webTwoFactorCodeCommandEnv,
		),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selection := webcore.ProviderSelection{
				ProviderID:       *providerID,
				PublicProviderID: *publicProviderID,
			}
			ctx = contextWithTwoFactorOptions(ctx, *twoFactorMethod, *twoFactorPhoneID)
			session, source, err := callResolveSessionForProviderSelection(contextForWebSignIn(ctx), *appleID, "", "", *twoFactorCodeCommand, selection)
			if err != nil {
				return err
			}

			// Provider selection is a request, so its budget starts once the
			// interactive login finished.
			requestCtx, cancel := newWebRequestContext(ctx)
			defer cancel()

			if err := selectResolvedWebSessionProvider(requestCtx, session, selection); err != nil {
				return err
			}

			status := webAuthStatus{
				Authenticated:    true,
				PasswordStored:   storedWebPasswordStatus(session.UserEmail),
				Source:           source,
				AppleID:          session.UserEmail,
				TeamID:           session.TeamID,
				ProviderID:       session.ProviderID,
				PublicProviderID: session.PublicProviderID,
				DeveloperTeamID:  session.DeveloperTeamID,
			}
			return printWebAuthStatus(status, *output.Output, *output.Pretty)
		},
	}
}

// WebAuthStatusCommand checks whether a cached session is currently valid.
func WebAuthStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth status", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email (checks this account cache; default checks last cached session)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc web auth status [--apple-id EMAIL]",
		ShortHelp:  "Show web-session status.",
		LongHelp: `WEB SESSION WORKFLOWS

Check whether an existing cached web session can be resumed.
If --apple-id is not provided, this checks the last cached session.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			trimmedAppleID := strings.TrimSpace(*appleID)
			var (
				session *webcore.AuthSession
				ok      bool
				err     error
			)
			if trimmedAppleID != "" {
				session, ok, err = tryResumeWebSession(requestCtx, trimmedAppleID)
			} else {
				session, ok, err = tryResumeLastWebSession(requestCtx)
			}
			if err != nil {
				if errors.Is(err, webcore.ErrCachedSessionExpired) {
					status := expiredWebAuthStatus(trimmedAppleID)
					return printWebAuthStatus(status, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("web auth status failed: %w", err)
			}

			if !ok || session == nil {
				return printWebAuthStatus(webAuthStatus{
					Authenticated:  false,
					PasswordStored: storedWebPasswordStatus(trimmedAppleID),
					AppleID:        trimmedAppleID,
				}, *output.Output, *output.Pretty)
			}
			return printWebAuthStatus(webAuthStatus{
				Authenticated:    true,
				PasswordStored:   storedWebPasswordStatus(session.UserEmail),
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

// WebAuthLogoutCommand clears cached web sessions.
func WebAuthLogoutCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth logout", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppleID := strings.TrimSpace(*appleID)
			if *all && trimmedAppleID != "" {
				return shared.UsageError("--all and --apple-id are mutually exclusive")
			}
			if *confirm && !*forgetPassword {
				return shared.UsageError("--confirm requires --forget-password")
			}
			if *forgetPassword && !*confirm {
				return shared.UsageError("--forget-password requires --confirm")
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
					_, _ = fmt.Fprintln(os.Stdout, "Removed all cached web sessions and stored passwords.")
					return nil
				}
				_, _ = fmt.Fprintln(os.Stdout, "Removed all cached web sessions.")
				return nil
			}
			if trimmedAppleID == "" {
				return shared.UsageError("provide --apple-id or --all")
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
				_, _ = fmt.Fprintf(os.Stdout, "Removed cached web session and stored password for %s.\n", trimmedAppleID)
				return nil
			}
			_, _ = fmt.Fprintf(os.Stdout, "Removed cached web session for %s.\n", trimmedAppleID)
			return nil
		},
	}
}
