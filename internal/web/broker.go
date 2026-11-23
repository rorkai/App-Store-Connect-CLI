package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	// WebBrokerURLEnv names the broker that holds the Apple web session.
	WebBrokerURLEnv = "ASC_WEB_BROKER_URL"
	// WebBrokerTokenEnv names the bearer token the broker accepts.
	WebBrokerTokenEnv = "ASC_WEB_BROKER_TOKEN"
	// WebBrokerUpstreamHeader marks a broker response that relays Apple's own
	// answer. A response without it is the broker's refusal.
	WebBrokerUpstreamHeader = "X-Asc-Web-Broker-Upstream"

	brokerErrorBodyLimit = 64 << 10
)

// BrokerAgreementsHandOff says who reads agreement status, and where, when
// the web session is lent through a broker: the agreement pages are not among
// the requests a broker lends.
const BrokerAgreementsHandOff = "ask the Account Holder to check Business agreements, such as the Paid Applications Schedule, " +
	"in App Store Connect under Business > Agreements (https://appstoreconnect.apple.com/business), " +
	"and the Apple Developer Program License Agreement at https://developer.apple.com/account"

// ErrWebBrokerConfig reports an incomplete or invalid broker configuration.
var ErrWebBrokerConfig = errors.New("invalid web session broker configuration")

// ErrWebBrokerRequest reports a request for Apple that is not handed to the
// broker because the broker could route it somewhere else.
var ErrWebBrokerRequest = errors.New("web session broker cannot relay the request")

// Broker is a service that holds an Apple web session and sends requests to
// Apple on the caller's behalf, so the session's cookies never reach this
// process.
type Broker struct {
	URL   *url.URL
	Token string
}

// BrokerFromEnv reads the broker from ASC_WEB_BROKER_URL and
// ASC_WEB_BROKER_TOKEN. ok is false when neither is set.
func BrokerFromEnv() (broker Broker, ok bool, err error) {
	rawURL := strings.TrimSpace(os.Getenv(WebBrokerURLEnv))
	token := strings.TrimSpace(os.Getenv(WebBrokerTokenEnv))
	if rawURL == "" && token == "" {
		return Broker{}, false, nil
	}
	if rawURL == "" || token == "" {
		return Broker{}, true, fmt.Errorf("%w: set both %s and %s", ErrWebBrokerConfig, WebBrokerURLEnv, WebBrokerTokenEnv)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Broker{}, true, fmt.Errorf("%w: %s must be an absolute URL without credentials, query, or fragment", ErrWebBrokerConfig, WebBrokerURLEnv)
	}
	loopbackHTTP := parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())
	if parsed.Scheme != "https" && !loopbackHTTP {
		return Broker{}, true, fmt.Errorf("%w: %s must use https (http only for a loopback host)", ErrWebBrokerConfig, WebBrokerURLEnv)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return Broker{URL: parsed, Token: token}, true, nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// BrokerError is the broker's own refusal, as opposed to Apple's answer it
// relays.
type BrokerError struct {
	Status  int
	Code    string
	Message string
}

func (e *BrokerError) Error() string {
	code := e.Code
	if code == "" {
		code = fmt.Sprintf("status %d", e.Status)
	}
	if e.Message == "" {
		return fmt.Sprintf("web session broker refused the request (%s)", code)
	}
	return fmt.Sprintf("web session broker refused the request (%s): %s", code, e.Message)
}

// HTTPStatusCode returns the broker's status.
func (e *BrokerError) HTTPStatusCode() int {
	return e.Status
}

// brokerTransport sends every request for an apple.com host to the broker as
// <broker URL>/<host><path>?<query> with the broker token. It holds no cookies;
// the broker attaches the session's, and any cookie the broker relays back is
// dropped before the caller sees the response. Requests for other hosts, such
// as the presigned storage URLs review attachments download from, carry no
// Apple session and go out directly.
type brokerTransport struct {
	broker Broker
	next   http.RoundTripper
}

func (t *brokerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(strings.TrimSuffix(req.URL.Hostname(), "."))
	if !isAppleSessionHost(host) {
		return t.next.RoundTrip(req)
	}
	// The broker routes on the first path segment and reaches Apple on the
	// default port, so a request it could route elsewhere is refused here.
	if reason := unrelayableRequest(req.URL); reason != "" {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, fmt.Errorf("%w: only %s are relayed", ErrWebBrokerRequest, reason)
	}

	target := *t.broker.URL
	target.Path = t.broker.URL.Path + "/" + host + req.URL.Path
	target.RawPath = t.broker.URL.EscapedPath() + "/" + host + req.URL.EscapedPath()
	target.RawQuery = req.URL.RawQuery

	out := req.Clone(req.Context())
	out.URL = &target
	out.Host = ""
	out.Header.Del("Cookie")
	out.Header.Set("Authorization", "Bearer "+t.broker.Token)

	resp, err := t.next.RoundTrip(out)
	if err != nil {
		return nil, err
	}
	if resp.Header.Get(WebBrokerUpstreamHeader) == "" {
		return nil, readBrokerError(resp)
	}
	for _, name := range []string{WebBrokerUpstreamHeader, "Set-Cookie", "Set-Cookie2"} {
		resp.Header.Del(name)
	}
	resp.Request = req
	return resp, nil
}

func isAppleSessionHost(host string) bool {
	return host == "apple.com" || strings.HasSuffix(host, ".apple.com")
}

func unrelayableRequest(target *url.URL) string {
	if target.Scheme != "https" {
		return "https requests"
	}
	if port := target.Port(); port != "" && port != "443" {
		return "requests for the default https port"
	}
	for segment := range strings.SplitSeq(target.Path, "/") {
		if segment == "." || segment == ".." {
			return "paths without dot segments"
		}
	}
	return ""
}

func readBrokerError(resp *http.Response) error {
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, brokerErrorBodyLimit))
	brokerErr := &BrokerError{Status: resp.StatusCode}
	var payload struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.Error != nil {
			brokerErr.Code, brokerErr.Message = payload.Error.Code, payload.Error.Message
		} else {
			brokerErr.Code, brokerErr.Message = payload.Code, payload.Message
		}
	}
	brokerErr.Code = sanitizeWebAuthDiagnosticValue(brokerErr.Code)
	brokerErr.Message = sanitizeWebAuthDiagnosticValue(brokerErr.Message)
	return brokerErr
}

// OpenBrokeredSession returns a session whose requests go through broker. It
// reads the account and provider through the broker and never reads or writes
// the session cache, logs in, or holds a cookie.
func OpenBrokeredSession(ctx context.Context, broker Broker) (*AuthSession, error) {
	return openBrokeredSession(ctx, broker, http.DefaultTransport)
}

func openBrokeredSession(ctx context.Context, broker Broker, base http.RoundTripper) (*AuthSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newWebHTTPClient(nil, base)
	client.Transport = &brokerTransport{broker: broker, next: client.Transport}
	info, err := getSessionInfo(ctx, client)
	if brokerErr, ok := errors.AsType[*BrokerError](err); ok {
		return nil, brokerErr
	}
	if err != nil {
		return nil, fmt.Errorf("brokered web session: %w", err)
	}
	session := &AuthSession{Client: client, Brokered: true}
	applySessionInfo(session, info)
	return session, nil
}
