package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const webSessionSourceBroker = "broker"

var openBrokeredSessionFn = webcore.OpenBrokeredSession

// webBrokerFromEnv reports whether ASC_WEB_BROKER_URL or ASC_WEB_BROKER_TOKEN
// is set; an incomplete or invalid pair is a usage error, never a fallback to
// the session cache.
func webBrokerFromEnv(d *shared.Deps) (webcore.Broker, bool, error) {
	broker, ok, err := webcore.BrokerFromEnv()
	if err != nil {
		return broker, ok, d.UsageError(err.Error())
	}
	return broker, ok, nil
}

// refuseUnderWebBroker stops commands that sign in, sign out, or move session
// cookies: under a broker the session lives with the broker and never in this
// process, and the local session cache is left alone.
func refuseUnderWebBroker(d *shared.Deps, command string) error {
	if _, ok, err := webBrokerFromEnv(d); !ok || err != nil {
		return err
	}
	return d.UsageError(command + " is unavailable while " + webcore.WebBrokerURLEnv + " is set: the broker holds the Apple web session, so sign in to Apple through the broker's owner instead")
}

// refuseSignInFlagsUnderWebBroker stops a command given a flag that only
// steers asc's own sign-in, which never runs under a broker.
func refuseSignInFlagsUnderWebBroker(d *shared.Deps, fs *flag.FlagSet) error {
	if fs == nil {
		return nil
	}
	var signIn string
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "two-factor-code-command", "two-factor-method", "two-factor-phone-id":
			if signIn == "" {
				signIn = "--" + f.Name
			}
		}
	})
	if signIn == "" {
		return nil
	}
	return refuseUnderWebBroker(d, signIn)
}

// webSignInAdvice names what to do about an ended web session: sign in with
// asc, or, under a broker, where the session lives.
func webSignInAdvice() string {
	if _, brokered, _ := webcore.BrokerFromEnv(); brokered {
		return "sign in to Apple again through the broker's owner"
	}
	return "run 'asc web auth login'"
}

func resolveBrokeredWebSession(ctx context.Context, d *shared.Deps, broker webcore.Broker, appleID string) (*webcore.AuthSession, error) {
	shared.ApplyRootLoggingOverrides()
	requestCtx, cancel := newWebRequestContextWith(ctx, d)
	defer cancel()
	session, err := openBrokeredSessionFn(requestCtx, broker)
	if webcore.IsSessionAuthFailure(err) {
		return nil, fmt.Errorf("%w (%s)", err, webSignInAdvice())
	}
	if err != nil {
		return nil, err
	}
	source := "--apple-id"
	if appleID = strings.TrimSpace(appleID); appleID == "" {
		appleID, source = envAppleID(), webAppleIDEnv
	}
	if appleID != "" && !strings.EqualFold(appleID, session.UserEmail) {
		return nil, d.UsageError(source + " does not match the account of the brokered web session")
	}
	return session, nil
}

// agreementsBrokerRefusalError explains a broker's 403 refusal of an agreement
// read. The broker decides what a lent session may send, so asc sends the
// request and reports the broker's answer rather than assuming its policy.
// It returns nil for any other error, including a broker's rate limit or
// outage, which a retry can fix.
func agreementsBrokerRefusalError(operation string, err error) error {
	refusal, ok := errors.AsType[*webcore.BrokerError](err)
	if !ok || refusal.Status != http.StatusForbidden {
		return nil
	}
	return shared.WithDiagnostic(
		&shared.HintedError{
			Err:  fmt.Errorf("%s failed: agreement status can't be read through the lent web session: %w", operation, err),
			Hint: "The web session broker does not lend agreement pages, so do not retry; " + webcore.BrokerAgreementsHandOff + ".",
		},
		shared.DiagnosticDependencyFailed,
		"",
	)
}
