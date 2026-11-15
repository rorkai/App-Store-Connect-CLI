package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func validateSessionFromEnvFlags(d *shared.Deps, flags webSessionFlags) error {
	if flags.flagSet == nil {
		return nil
	}
	var unsupported string
	flags.flagSet.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "provider-id", "public-provider-id", "two-factor-code-command", "two-factor-method", "two-factor-phone-id":
			if unsupported == "" {
				unsupported = f.Name
			}
		}
	})
	if unsupported != "" {
		return d.UsageError("--" + unsupported + " is unsupported with --session-from-env")
	}
	return nil
}

func resolveSessionFromEnv(ctx context.Context, d *shared.Deps, flags webSessionFlags) (*webcore.AuthSession, error) {
	if err := validateSessionFromEnvFlags(d, flags); err != nil {
		return nil, err
	}
	for _, name := range []string{"ASC_WEB_SESSION_PROVIDER", "ASC_WEB_SESSION_CSRF"} {
		if os.Getenv(name) != "" {
			return nil, d.UsageError(name + " is unsupported with --session-from-env")
		}
	}
	value := os.Getenv(webSessionBundleEnvName)
	if strings.TrimSpace(value) == "" {
		return nil, d.UsageError(webSessionBundleEnvName + " is unset or empty when --session-from-env is used")
	}
	if len(value) > webcore.MaxSessionBundleSize {
		return nil, d.UsageError(fmt.Sprintf("%s exceeds %d-byte limit", webSessionBundleEnvName, webcore.MaxSessionBundleSize))
	}
	bundle, err := webcore.DecodeSessionBundle([]byte(value))
	if err != nil {
		// Decoder errors may contain caller-controlled credential fields.
		return nil, d.UsageError("invalid session bundle from " + webSessionBundleEnvName)
	}
	if appleID := strings.TrimSpace(*flags.appleID); appleID != "" && !strings.EqualFold(appleID, strings.TrimSpace(bundle.AppleID)) {
		return nil, d.UsageError("session bundle does not match --apple-id")
	}
	shared.ApplyRootLoggingOverrides()
	validationCtx, cancel := newWebRequestContextWith(ctx, d)
	defer cancel()
	session, err := webcore.OpenValidatedSessionBundle(validationCtx, bundle)
	if err != nil {
		return nil, fmt.Errorf("web session validation failed; supply a fresh canonical ASC_WEB_SESSION bundle")
	}
	return session, nil
}
