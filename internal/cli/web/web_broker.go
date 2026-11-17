package web

import (
	"context"
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

// refuseUnderWebBroker stops commands that sign in or move session cookies:
// under a broker the session lives with the broker and never in this process.
func refuseUnderWebBroker(d *shared.Deps, command string) error {
	if _, ok, _ := webcore.BrokerFromEnv(); !ok {
		return nil
	}
	return d.UsageError(command + " is unavailable while " + webcore.WebBrokerURLEnv + " is set: the broker holds the Apple web session, so sign in to Apple through the broker's owner instead")
}

func resolveBrokeredWebSession(ctx context.Context, d *shared.Deps, broker webcore.Broker, appleID string) (*webcore.AuthSession, error) {
	shared.ApplyRootLoggingOverrides()
	requestCtx, cancel := newWebRequestContextWith(ctx, d)
	defer cancel()
	session, err := openBrokeredSessionFn(requestCtx, broker)
	if err != nil {
		return nil, err
	}
	if appleID = strings.TrimSpace(appleID); appleID != "" && !strings.EqualFold(appleID, session.UserEmail) {
		return nil, d.UsageError("--apple-id does not match the account of the brokered web session")
	}
	return session, nil
}
