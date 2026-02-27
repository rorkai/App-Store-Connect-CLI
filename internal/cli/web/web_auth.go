package web

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const webPasswordEnv = "ASC_WEB_PASSWORD"

var (
	promptTwoFactorCodeFn = promptTwoFactorCodeInteractive
	promptPasswordFn      = promptPasswordInteractive
	webLoginFn            = webcore.Login
	submitTwoFactorCodeFn = webcore.SubmitTwoFactorCode
	termReadPasswordFn    = term.ReadPassword
	termIsTerminalFn      = term.IsTerminal
	resolveSessionFn      = resolveSession
)

type webAuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Source        string `json:"source,omitempty"`
	AppleID       string `json:"appleId,omitempty"`
	TeamID        string `json:"teamId,omitempty"`
	ProviderID    int64  `json:"providerId,omitempty"`
}

func readPasswordFromInput(useStdin bool) (string, error) {
	if useStdin {
		if termIsTerminalFn(int(os.Stdin.Fd())) {
			return "", shared.UsageError("--password-stdin requires piped input; omit it to use the interactive password prompt")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	password := strings.TrimSpace(os.Getenv(webPasswordEnv))
	if password != "" {
		return password, nil
	}
	password, err := promptPasswordFn()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

func readPasswordFromTerminalFD(fd int, writer io.Writer) (string, error) {
	if writer == nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	if _, err := fmt.Fprint(writer, "Apple ID password: "); err != nil {
		return "", fmt.Errorf("password prompt unavailable")
	}
	passwordBytes, err := termReadPasswordFn(fd)
	_, _ = fmt.Fprintln(writer)
	if err != nil {
		return "", fmt.Errorf("failed to read password")
	}
	return strings.TrimSpace(string(passwordBytes)), nil
}

func promptPasswordInteractive() (string, error) {
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer func() { _ = tty.Close() }()
		return readPasswordFromTerminalFD(int(tty.Fd()), tty)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readPasswordFromTerminalFD(int(os.Stdin.Fd()), os.Stderr)
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
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer func() { _ = tty.Close() }()
		return readTwoFactorCodeFromTerminalFD(int(tty.Fd()), tty)
	}
	if termIsTerminalFn(int(os.Stdin.Fd())) {
		return readTwoFactorCodeFromTerminalFD(int(os.Stdin.Fd()), os.Stderr)
	}
	return "", fmt.Errorf("2fa required: re-run with --two-factor-code")
}

func loginWithOptionalTwoFactor(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, error) {
	session, err := webLoginFn(ctx, webcore.LoginCredentials{
		Username: appleID,
		Password: password,
	})
	if err == nil {
		return session, nil
	}

	var tfaErr *webcore.TwoFactorRequiredError
	if session != nil && errors.As(err, &tfaErr) {
		code := strings.TrimSpace(twoFactorCode)
		if code == "" {
			var promptErr error
			code, promptErr = promptTwoFactorCodeFn()
			if promptErr != nil {
				return nil, promptErr
			}
		}
		if err := submitTwoFactorCodeFn(ctx, session, code); err != nil {
			return nil, fmt.Errorf("2fa verification failed: %w", err)
		}
		return session, nil
	}
	return nil, err
}

func resolveSession(ctx context.Context, appleID, password, twoFactorCode string, usePasswordStdin bool) (*webcore.AuthSession, string, error) {
	shared.ApplyRootLoggingOverrides()

	appleID = strings.TrimSpace(appleID)
	twoFactorCode = strings.TrimSpace(twoFactorCode)

	if appleID != "" {
		if resumed, ok, err := webcore.TryResumeSession(ctx, appleID); err == nil && ok {
			return resumed, "cache", nil
		}
	}

	if appleID == "" {
		return nil, "", shared.UsageError("--apple-id is required")
	}

	password = strings.TrimSpace(password)
	if password == "" {
		var err error
		password, err = readPasswordFromInput(usePasswordStdin)
		if err != nil {
			return nil, "", err
		}
	}
	if password == "" {
		return nil, "", shared.UsageError("password is required: run in a terminal for an interactive prompt, provide --password-stdin, or set ASC_WEB_PASSWORD")
	}

	session, err := loginWithOptionalTwoFactor(ctx, appleID, password, twoFactorCode)
	if err != nil {
		return nil, "", fmt.Errorf("web auth login failed: %w", err)
	}
	if err := webcore.PersistSession(session); err != nil {
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
		ShortHelp:  "EXPERIMENTAL: Manage unofficial Apple web sessions (discouraged).",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage Apple web-session authentication used by "asc web" commands.
This is not the official App Store Connect API-key auth flow.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAuthLoginCommand(),
			WebAuthStatusCommand(),
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

	appleID := fs.String("apple-id", "", "Apple ID email")
	passwordStdin := fs.Bool("password-stdin", false, "Read Apple ID password from stdin")
	twoFactorCode := fs.String("two-factor-code", "", "2FA code for accounts requiring verification")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "login",
		ShortUsage: "asc web auth login --apple-id EMAIL [--password-stdin] [--two-factor-code CODE]",
		ShortHelp:  "EXPERIMENTAL: Authenticate unofficial Apple web session.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Authenticate using Apple web-session behavior for detached "asc web" workflows.

Password input options:
  - secure interactive prompt (default and recommended for local use)
  - --password-stdin (automation/CI)
  - ASC_WEB_PASSWORD environment variable

` + webWarningText + `

Examples:
  asc web auth login --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com" --password-stdin
  ASC_WEB_PASSWORD="..." asc web auth login --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com" --password-stdin --two-factor-code 123456`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, source, err := resolveSessionFn(requestCtx, *appleID, "", *twoFactorCode, *passwordStdin)
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

	appleID := fs.String("apple-id", "", "Apple ID email (checks this account cache; default checks last cached session)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc web auth status [--apple-id EMAIL]",
		ShortHelp:  "EXPERIMENTAL: Show unofficial web-session status.",
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
				session, ok, err = webcore.TryResumeSession(requestCtx, trimmedAppleID)
			} else {
				session, ok, err = webcore.TryResumeLastSession(requestCtx)
			}
			if err != nil {
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

	appleID := fs.String("apple-id", "", "Apple ID email to remove from cache")
	all := fs.Bool("all", false, "Remove all cached web sessions")

	return &ffcli.Command{
		Name:       "logout",
		ShortUsage: "asc web auth logout [--apple-id EMAIL | --all]",
		ShortHelp:  "EXPERIMENTAL: Clear unofficial web-session cache.",
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
