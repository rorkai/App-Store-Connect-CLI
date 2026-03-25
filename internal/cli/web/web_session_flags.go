package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type webSessionFlags struct {
	appleID              *string
	twoFactorCode        *string
	twoFactorCodeCommand *string
}

const deprecatedTwoFactorCodeFlagName = "two-factor-code"

func bindDeprecatedTwoFactorCodeFlag(fs *flag.FlagSet) *string {
	return fs.String(deprecatedTwoFactorCodeFlagName, "", "Deprecated: direct 2FA code if verification is required; prefer --two-factor-code-command")
}

func bindWebSessionFlags(fs *flag.FlagSet) webSessionFlags {
	return webSessionFlags{
		appleID:              fs.String("apple-id", "", "Apple Account email used to scope a user-owned session cache (optional when a cached session exists)"),
		twoFactorCode:        bindDeprecatedTwoFactorCodeFlag(fs),
		twoFactorCodeCommand: fs.String("two-factor-code-command", "", "Shell command that prints the 2FA code to stdout if verification is required"),
	}
}

func warnDeprecatedTwoFactorCodeFlag(twoFactorCode string) {
	if strings.TrimSpace(twoFactorCode) == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: `--%s` is deprecated. Use `--two-factor-code-command` or `%s` for automation.\n", deprecatedTwoFactorCodeFlagName, webTwoFactorCodeCommandEnv)
}

func resolveWebSessionForCommand(ctx context.Context, flags webSessionFlags) (*webcore.AuthSession, error) {
	warnDeprecatedTwoFactorCodeFlag(*flags.twoFactorCode)
	session, _, err := callResolveSessionFn(
		ctx,
		*flags.appleID,
		"",
		*flags.twoFactorCode,
		*flags.twoFactorCodeCommand,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func withWebAuthHint(err error, operation string) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), operation+" failed:") {
		return err
	}
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403) {
		return fmt.Errorf("%s failed: web session is unauthorized or expired (run 'asc web auth login'): %w", operation, err)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}
