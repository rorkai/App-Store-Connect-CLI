package web

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	webPasswordEnv             = "ASC_WEB_PASSWORD"
	webTwoFactorCodeCommandEnv = "ASC_WEB_2FA_CODE_COMMAND"
	webTwoFactorCommandTimeout = 60 * time.Second
)

var errAutoReauthRequiresAppleID = errors.New("cached web session is missing stored apple id metadata")

var (
	openTTYFn                                = openTTY
	promptTwoFactorCodeFn                    = promptTwoFactorCodeInteractive
	promptPasswordFn                         = promptPasswordInteractive
	readTwoFactorCodeFromCommandFn           = readTwoFactorCodeFromCommand
	webLoginFn                               = webcore.Login
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
	resolveSessionFn               any       = resolveSession
	sessionExpiredWriter           io.Writer = os.Stderr
)

func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

type webAuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Source        string `json:"source,omitempty"`
	AppleID       string `json:"appleId,omitempty"`
	TeamID        string `json:"teamId,omitempty"`
	ProviderID    int64  `json:"providerId,omitempty"`
}

func signalProcessInterrupt() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return process.Signal(os.Interrupt)
}

func callResolveSessionFn(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
	switch fn := resolveSessionFn.(type) {
	case func(context.Context, string, string, string) (*webcore.AuthSession, string, error):
		return fn(ctx, appleID, password, twoFactorCode)
	case func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error):
		return fn(ctx, appleID, password, twoFactorCode, twoFactorCodeCommand)
	case func(context.Context, string, string, string, ...string) (*webcore.AuthSession, string, error):
		return fn(ctx, appleID, password, twoFactorCode, twoFactorCodeCommand)
	default:
		return nil, "", fmt.Errorf("internal error: unsupported web session resolver type %T", resolveSessionFn)
	}
}

func readPasswordFromInput(ctx context.Context) (string, error) {
	password := readPasswordFromEnv()
	if password != "" {
		return password, nil
	}
	password, err := promptPasswordFn(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

func readPasswordFromEnv() string {
	return strings.TrimSpace(os.Getenv(webPasswordEnv))
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
	return strings.TrimSpace(string(passwordBytes)), nil
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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("password prompt interrupted: %w", ctxErr)
		}

		n, err := terminal.Read(readBuf)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", fmt.Errorf("password prompt interrupted: %w", ctxErr)
			}
			return "", fmt.Errorf("failed to read password")
		}
		if n == 0 {
			continue
		}

		switch readBuf[0] {
		case '\r', '\n':
			return strings.TrimSpace(string(passwordBytes)), nil
		case 3:
			// Raw mode consumes VINTR as a byte, so re-emit SIGINT to preserve
			// top-level cancellation behavior for the rest of the CLI lifecycle.
			_ = signalProcessInterruptFn()
			return "", fmt.Errorf("password prompt interrupted: %w", context.Canceled)
		case 4:
			if len(passwordBytes) == 0 {
				return "", fmt.Errorf("password prompt interrupted: %w", context.Canceled)
			}
			return strings.TrimSpace(string(passwordBytes)), nil
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

func loginWithOptionalTwoFactor(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, error) {
	session, err := withWebSpinnerValue("Signing in to Apple web session", func() (*webcore.AuthSession, error) {
		return webLoginFn(ctx, webcore.LoginCredentials{
			Username: appleID,
			Password: password,
		})
	})
	if err == nil {
		return session, nil
	}

	var tfaErr *webcore.TwoFactorRequiredError
	if session != nil && errors.As(err, &tfaErr) {
		code := strings.TrimSpace(twoFactorCode)
		command := ""
		if len(twoFactorCodeCommand) > 0 {
			command = strings.TrimSpace(twoFactorCodeCommand[0])
		}
		if code == "" {
			if command != "" {
				resolvedCode, codeErr := readTwoFactorCodeFromCommandFn(ctx, command)
				if codeErr != nil {
					return nil, codeErr
				}
				code = resolvedCode
			} else {
				var promptErr error
				code, promptErr = promptTwoFactorCodeFn()
				if promptErr != nil {
					return nil, promptErr
				}
			}
		}
		verifyCtx, cancel := shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
		defer cancel()
		if err := withWebSpinner("Verifying two-factor code", func() error {
			return submitTwoFactorCodeFn(verifyCtx, session, code)
		}); err != nil {
			return nil, fmt.Errorf("2fa verification failed: %w", err)
		}
		return session, nil
	}
	return nil, err
}

func tryAutoReauthWebSession(ctx context.Context, appleID, password string) (*webcore.AuthSession, string, bool, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, "", false, nil
	}

	var (
		cached *webcore.AuthSession
		ok     bool
		err    error
	)
	if strings.TrimSpace(appleID) != "" {
		cached, ok, err = loadCachedSessionFn(appleID)
	} else {
		cached, ok, err = loadLastCachedSessionFn()
	}
	if err != nil || !ok || cached == nil {
		return nil, "", false, err
	}
	if cached.Client == nil {
		return nil, "", false, nil
	}

	reauthAppleID := strings.TrimSpace(appleID)
	if reauthAppleID == "" {
		reauthAppleID = strings.TrimSpace(cached.UserEmail)
	}
	if reauthAppleID == "" {
		return nil, "", false, errAutoReauthRequiresAppleID
	}

	session, err := withWebSpinnerValue("Refreshing expired web session", func() (*webcore.AuthSession, error) {
		return webLoginWithClientFn(ctx, cached.Client, webcore.LoginCredentials{
			Username: reauthAppleID,
			Password: password,
		})
	})
	if err != nil {
		if errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) {
			return nil, "", false, err
		}
		// Fall back to the pre-existing fresh-login path when the cached-jar
		// attempt cannot be completed. The cache format is intentionally
		// best-effort and may not preserve enough cookie metadata for relogin.
		return nil, "", false, nil
	}
	_ = persistWebSessionFn(session)
	return session, "auto-reauth", true, nil
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

func resolveSession(ctx context.Context, appleID, password, twoFactorCode string, twoFactorCodeCommand ...string) (*webcore.AuthSession, string, error) {
	shared.ApplyRootLoggingOverrides()

	appleID = strings.TrimSpace(appleID)
	password = strings.TrimSpace(password)
	twoFactorCode = strings.TrimSpace(twoFactorCode)
	command := ""
	if len(twoFactorCodeCommand) > 0 {
		command = strings.TrimSpace(twoFactorCodeCommand[0])
	}
	if command == "" {
		command = strings.TrimSpace(os.Getenv(webTwoFactorCodeCommandEnv))
	}
	cacheExpired := false

	if appleID != "" {
		if resumed, ok, err := tryResumeWebSession(ctx, appleID); err == nil && ok {
			return resumed, "cache", nil
		} else if errors.Is(err, webcore.ErrCachedSessionExpired) {
			cacheExpired = true
		}
	} else {
		if resumed, ok, err := tryResumeLastWebSession(ctx); err == nil && ok {
			return resumed, "cache", nil
		} else if errors.Is(err, webcore.ErrCachedSessionExpired) {
			cacheExpired = true
		}
	}
	if cacheExpired {
		silentPassword := password
		if silentPassword == "" {
			silentPassword = readPasswordFromEnv()
		}
		if session, source, ok, err := tryAutoReauthWebSession(ctx, appleID, silentPassword); err != nil {
			if errors.Is(err, errAutoReauthRequiresAppleID) {
				return nil, "", shared.UsageError("last cached web session predates stored Apple ID metadata; re-run once with --apple-id to refresh the cache")
			}
			return nil, "", fmt.Errorf("web auth auto-reauth failed: %w", err)
		} else if ok {
			return session, source, nil
		}
		printExpiredSessionNotice(sessionExpiredWriter)
	}

	if appleID == "" {
		return nil, "", shared.UsageError("--apple-id is required when no cached web session is available")
	}

	if password == "" {
		var err error
		password, err = readPasswordFromInput(ctx)
		if err != nil {
			return nil, "", err
		}
	}
	if password == "" {
		return nil, "", shared.UsageError("password is required: run in a terminal for an interactive prompt or set ASC_WEB_PASSWORD")
	}

	session, err := loginWithOptionalTwoFactor(ctx, appleID, password, twoFactorCode, command)
	if err != nil {
		return nil, "", fmt.Errorf("web auth login failed: %w", err)
	}
	if err := persistWebSessionFn(session); err != nil {
		return nil, "", fmt.Errorf("web auth login succeeded but failed to cache session: %w", err)
	}
	return session, "fresh", nil
}

// WebAuthCommand returns the detached web auth command group.
func WebAuthCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "auth",
		ShortUsage: "asc web auth <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage unofficial Apple web sessions (discouraged).",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage Apple web-session authentication used by "asc web" commands.
This is not the official App Store Connect API-key auth flow.

` + webWarningText,
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc web auth login --apple-id EMAIL [--two-factor-code-command CMD]",
		ShortHelp:  "[experimental] Authenticate unofficial Apple web session.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Authenticate using Apple web-session behavior for detached "asc web" workflows.

Password input options:
  - secure interactive prompt (default and recommended for local use)
  - ASC_WEB_PASSWORD environment variable

Two-factor input options:
  - secure interactive prompt (default for manual use)
  - --two-factor-code-command
  - ` + webTwoFactorCodeCommandEnv + ` environment variable (recommended for automation)
  - --two-factor-code (deprecated compatibility alias when the code is already known)

` + webWarningText + `

Examples:
  asc web auth login --apple-id "user@example.com"
  ASC_WEB_PASSWORD="..." asc web auth login --apple-id "user@example.com"
  ` + webTwoFactorCodeCommandEnv + `='osascript /path/to/get-apple-2fa-code.scpt' asc web auth login --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			warnDeprecatedTwoFactorCodeFlag(*twoFactorCode)
			session, source, err := callResolveSessionFn(requestCtx, *appleID, "", *twoFactorCode, *twoFactorCodeCommand)
			if err != nil {
				return err
			}

			status := webAuthStatus{
				Authenticated: true,
				Source:        source,
				AppleID:       session.UserEmail,
				TeamID:        session.TeamID,
				ProviderID:    session.ProviderID,
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
		ShortHelp:  "[experimental] Show unofficial web-session status.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Check whether an existing cached web session can be resumed.
If --apple-id is not provided, this checks the last cached session.

` + webWarningText,
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
					return shared.PrintOutput(webAuthStatus{Authenticated: false}, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("web auth status failed: %w", err)
			}

			if !ok || session == nil {
				return shared.PrintOutput(webAuthStatus{Authenticated: false}, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(webAuthStatus{
				Authenticated: true,
				Source:        "cache",
				AppleID:       session.UserEmail,
				TeamID:        session.TeamID,
				ProviderID:    session.ProviderID,
			}, *output.Output, *output.Pretty)
		},
	}
}

// WebAuthLogoutCommand clears cached web sessions.
func WebAuthLogoutCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth logout", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email to remove from cache")
	all := fs.Bool("all", false, "Remove all cached web sessions")

	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc web auth logout [--apple-id EMAIL | --all]",
		ShortHelp:  "[experimental] Clear unofficial web-session cache.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Remove cached web-session credentials for detached "asc web" commands.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppleID := strings.TrimSpace(*appleID)
			if *all && trimmedAppleID != "" {
				return shared.UsageError("--all and --apple-id are mutually exclusive")
			}
			if *all {
				if err := webcore.DeleteAllSessions(); err != nil {
					return fmt.Errorf("web auth logout failed: %w", err)
				}
				_, _ = fmt.Fprintln(os.Stdout, "Removed all cached web sessions.")
				return nil
			}
			if trimmedAppleID == "" {
				return shared.UsageError("provide --apple-id or --all")
			}
			if err := webcore.DeleteSession(trimmedAppleID); err != nil {
				return fmt.Errorf("web auth logout failed: %w", err)
			}
			_, _ = fmt.Fprintf(os.Stdout, "Removed cached web session for %s.\n", trimmedAppleID)
			return nil
		},
	}
}
