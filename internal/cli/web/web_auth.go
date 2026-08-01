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
	webDontStorePasswordEnv    = "ASC_WEB_DONT_STORE_PASSWORD"
	webTwoFactorCodeCommandEnv = "ASC_WEB_2FA_CODE_COMMAND"
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
	tryResumeLastFn                          = webcore.TryResumeLastSession
	loadCachedSessionFn                      = webcore.LoadCachedSession
	loadLastCachedSessionFn                  = webcore.LoadLastCachedSession
	webLoginWithClientFn                     = webcore.LoginWithClient
	loadStoredWebPasswordFn                  = webcore.LoadPassword
	storeStoredWebPasswordFn                 = webcore.StorePassword
	storedWebPasswordExistsFn                = webcore.PasswordStored
	deleteStoredWebPasswordFn                = webcore.DeletePassword
	deleteAllStoredWebPasswordsFn            = webcore.DeleteAllPasswords
	deleteWebSessionFn                       = webcore.DeleteSession
	deleteAllWebSessionsFn                   = webcore.DeleteAllSessions
	selectWebProviderFn                      = webcore.SelectProvider
	resolveSessionFn               any       = resolveSession
	resolveSessionWithoutPersistFn any       = resolveSessionWithoutPersist
	twoFactorStatusWriter          io.Writer = os.Stderr
	sessionExpiredWriter           io.Writer = os.Stderr
	sessionCacheWarningWriter      io.Writer = os.Stderr
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

func promptTwoFactorCodeInteractive() (string, error) {
	if tty, err := openTTYFn(); err == nil {
		defer func() { _ = tty.Close() }()
		return readTwoFactorCodeFromTerminalFD(int(tty.Fd()), tty)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readTwoFactorCodeFromTerminalFD(int(os.Stdin.Fd()), os.Stderr)
	}
	return "", fmt.Errorf("2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, set %s, or re-run with deprecated --%s", webTwoFactorCodeCommandEnv, deprecatedTwoFactorCodeFlagName)
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

func printExpiredSessionNotice(writer io.Writer) {
	if writer == nil {
		return
	}
	_, _ = fmt.Fprintln(writer, "Session expired.")
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

func loginWithOptionalTwoFactorUsing(ctx context.Context, progressMessage, appleID, password, twoFactorCode string, loginFn func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error), twoFactorStarted func(), twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
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
		challenge, prepErr := prepareTwoFactorChallengeFn(ctx, session)
		if prepErr != nil {
			return nil, fmt.Errorf("2fa challenge setup failed: %w", prepErr)
		}

		code := strings.TrimSpace(twoFactorCode)
		command := ""
		if len(twoFactorCodeCommand) > 0 {
			command = strings.TrimSpace(twoFactorCodeCommand[0])
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
		readCode := func() (string, error) {
			if command != "" {
				return readTwoFactorCodeFromCommandFn(ctx, command)
			}
			return promptTwoFactorCodeFn()
		}
		if code == "" {
			if challenge != nil && challenge.IsPhoneMethod() {
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
					return nil, fmt.Errorf("2fa verification failed after switching to phone delivery: %w", err)
				}
				return session, nil
			}
			return nil, fmt.Errorf("2fa verification failed: %w", err)
		}
		return session, nil
	}
	return nil, err
}

func loginWithOptionalTwoFactor(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	return loginWithOptionalTwoFactorUsing(ctx, "Signing in to Apple web session", appleID, password, twoFactorCode, webLoginFn, nil, twoFactorCodeCommand...)
}

func loginWithOptionalTwoFactorClientTracked(ctx context.Context, client *http.Client, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, bool, error) {
	twoFactorStarted := false
	session, err := loginWithOptionalTwoFactorUsing(ctx, "Refreshing expired web session", appleID, password, twoFactorCode, func(ctx context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return webLoginWithClientFn(ctx, client, credentials)
	}, func() {
		twoFactorStarted = true
	}, twoFactorCodeCommand...)
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

func resolveWebSession(ctx context.Context, appleID, password, twoFactorCode string, opts webSessionResolveOptions) (*webcore.AuthSession, string, error) {
	shared.ApplyRootLoggingOverrides()

	resolvedAppleID := strings.TrimSpace(appleID)
	twoFactorCode = strings.TrimSpace(twoFactorCode)
	command := strings.TrimSpace(opts.twoFactorCodeCommand)
	if command == "" {
		command = strings.TrimSpace(os.Getenv(webTwoFactorCodeCommandEnv))
	}
	expiredNoticePrinted := false
	printExpiredNotice := func() {
		if expiredNoticePrinted {
			return
		}
		printExpiredSessionNotice(sessionExpiredWriter)
		expiredNoticePrinted = true
	}

	var (
		expiredCachedSession *webcore.AuthSession
		fallbackPassword     resolvedWebPassword
		skipStoredPassword   bool
	)

	tryKnownSession := func(targetAppleID string) (*webcore.AuthSession, string, bool, error) {
		resumed, ok, cacheExpired, err := resolveKnownWebSession(ctx, targetAppleID)
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
			expiredCachedSession, cachedOK, err = loadCachedSessionFn(targetAppleID)
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

		session, twoFactorStarted, loginErr := loginWithOptionalTwoFactorClientTracked(ctx, expiredCachedSession.Client, reauthAppleID, silentPassword.value, twoFactorCode, command)
		if loginErr == nil {
			if opts.persistAutoReauth != nil {
				opts.persistAutoReauth(session)
			}
			return session, "auto-reauth", true, nil
		}
		if twoFactorStarted {
			return nil, "", false, fmt.Errorf("web auth auto-reauth failed: %w", loginErr)
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

		// Before 2FA begins, a cached jar can become unusable independently of
		// the credentials. Preserve the password source for one fresh fallback.
		fallbackPassword = silentPassword
		expiredCachedSession = nil
		printExpiredNotice()
		return nil, "", false, nil
	}

	if session, source, ok, err := tryKnownSession(resolvedAppleID); err != nil {
		return nil, "", err
	} else if ok {
		return session, source, nil
	}

	if resolvedAppleID == "" {
		if opts.promptAppleID == nil {
			return nil, "", shared.UsageError("--apple-id is required when no cached web session is available")
		}
		if err := opts.promptAppleID(&resolvedAppleID); err != nil {
			return nil, "", err
		}
		resolvedAppleID = strings.TrimSpace(resolvedAppleID)
		if session, source, ok, err := tryKnownSession(resolvedAppleID); err != nil {
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
		return nil, "", shared.UsageError(fmt.Sprintf("password is required: run in a terminal for an interactive prompt or set %s", webPasswordEnvDisplay()))
	}

	login := func(candidate resolvedWebPassword) (*webcore.AuthSession, bool, error) {
		if expiredCachedSession != nil && expiredCachedSession.Client != nil {
			return loginWithOptionalTwoFactorClientTracked(ctx, expiredCachedSession.Client, resolvedAppleID, candidate.value, twoFactorCode, command)
		}
		session, err := loginWithOptionalTwoFactor(ctx, resolvedAppleID, candidate.value, twoFactorCode, command)
		return session, false, err
	}
	loginWithPromptedFreshFallback := func(candidate resolvedWebPassword) (*webcore.AuthSession, error) {
		session, twoFactorStarted, err := login(candidate)
		if err == nil || candidate.source != webPasswordSourcePrompted || expiredCachedSession == nil || twoFactorStarted || errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) {
			return session, err
		}
		// Match the non-interactive reauth path: a non-credential failure can
		// mean that the cached cookie jar itself is unusable. Retry once with a
		// fresh client while preserving the already-resolved password.
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
		return nil, "", fmt.Errorf("web auth login failed: %w", err)
	}
	persistPromptedWebPassword(resolvedAppleID, resolvedPassword)
	if opts.persistFresh != nil {
		if err := opts.persistFresh(session); err != nil {
			return nil, "", err
		}
	}
	return session, "fresh", nil
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
	if err := selectWebProviderFn(ctx, session, selection); err != nil {
		return fmt.Errorf("web provider selection failed: %w", err)
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
	return resolveWebSession(ctx, appleID, password, twoFactorCode, webSessionResolveOptions{
		resolvePassword:      resolveSessionPassword,
		persistFresh:         persistFreshResolvedSession,
		persistAutoReauth:    persistAutoReauthResolvedSession,
		twoFactorCodeCommand: command,
	})
}

func resolveSessionWithoutPersist(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = twoFactorCodeCommand[0]
	}
	return resolveWebSession(ctx, appleID, password, twoFactorCode, webSessionResolveOptions{
		resolvePassword:      resolveSessionPassword,
		twoFactorCodeCommand: command,
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

	appleID := fs.String("apple-id", "", "Apple Account email")
	twoFactorCode := bindDeprecatedTwoFactorCodeFlag(fs)
	twoFactorCodeCommand := fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required")
	providerID := fs.Int64("provider-id", 0, "Numeric App Store Connect provider ID to select for this web session")
	publicProviderID := fs.String("public-provider-id", "", "Public App Store Connect provider/team ID to select for this web session")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc web auth login --apple-id EMAIL [--public-provider-id TEAM_ID]",
		ShortHelp:  "Authenticate Apple web session.",
		LongHelp: fmt.Sprintf(
			`WEB SESSION WORKFLOWS

Authenticate using Apple web-session behavior for "asc web" workflows.

Password input options:
  - secure interactive prompt (default; saved in the native credential store after successful login)
  - %s environment variable
  - %s=1 disables saved-password reads and writes

Two-factor input options:
  - secure interactive prompt (default for manual use)
  - --two-factor-code-command
  - %s environment variable (recommended for automation)
  - --two-factor-code (deprecated compatibility alias when the code is already known)

Provider selection:
  - --public-provider-id selects the public App Store Connect provider/team ID
  - --provider-id selects Apple's numeric App Store Connect provider ID



Examples:
  asc web auth login --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com" --public-provider-id "Z4N6A5FQKW"
  %s asc web auth login --apple-id "user@example.com"
  %s='osascript /path/to/get-apple-2fa-code.scpt' asc web auth login --apple-id "user@example.com"`,
			webPasswordEnvDisplay(),
			webDontStorePasswordEnv,
			webTwoFactorCodeCommandEnv,
			webPasswordEnvAssignmentExample(),
			webTwoFactorCodeCommandEnv,
		),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			warnDeprecatedTwoFactorCodeFlag(*twoFactorCode)
			selection := webcore.ProviderSelection{
				ProviderID:       *providerID,
				PublicProviderID: *publicProviderID,
			}
			session, source, err := callResolveSessionForProviderSelection(requestCtx, *appleID, "", *twoFactorCode, *twoFactorCodeCommand, selection)
			if err != nil {
				return err
			}
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
			}
			return shared.PrintOutput(status, *output.Output, *output.Pretty)
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
					statusAppleID := trimmedAppleID
					if statusAppleID == "" {
						if cached, cachedOK, cacheErr := loadLastCachedSessionFn(); cacheErr == nil && cachedOK && cached != nil {
							statusAppleID = strings.TrimSpace(cached.UserEmail)
						}
					}
					return shared.PrintOutput(webAuthStatus{
						Authenticated:  false,
						PasswordStored: storedWebPasswordStatus(statusAppleID),
						AppleID:        statusAppleID,
					}, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("web auth status failed: %w", err)
			}

			if !ok || session == nil {
				return shared.PrintOutput(webAuthStatus{
					Authenticated:  false,
					PasswordStored: storedWebPasswordStatus(trimmedAppleID),
					AppleID:        trimmedAppleID,
				}, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(webAuthStatus{
				Authenticated:    true,
				PasswordStored:   storedWebPasswordStatus(session.UserEmail),
				Source:           "cache",
				AppleID:          session.UserEmail,
				TeamID:           session.TeamID,
				ProviderID:       session.ProviderID,
				PublicProviderID: session.PublicProviderID,
			}, *output.Output, *output.Pretty)
		},
	}
}

// WebAuthLogoutCommand clears cached web sessions.
func WebAuthLogoutCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth logout", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email to remove from cache")
	all := fs.Bool("all", false, "Remove all cached web sessions")
	forgetPassword := fs.Bool("forget-password", false, "[experimental] Also remove the saved Apple Account password")
	confirm := fs.Bool("confirm", false, "[experimental] Confirm removal of saved password credentials")

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
