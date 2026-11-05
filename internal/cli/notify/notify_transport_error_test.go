package notify

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// The webhook path is itself the credential, so the secret sentinel lives in the
// path segments a Slack incoming webhook uses.
const slackWebhookSecretSentinel = "asc-red-sentinel-slack-webhook-3fd914"

func slackWebhookWithSecret() string {
	return "http://127.0.0.1:1/services/T00000000/B00000000/" + slackWebhookSecretSentinel
}

type failingSlackTransport struct {
	err error
}

func (t *failingSlackTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

// runSlackWithSecretWebhook sends a message to the sentinel webhook through
// transport and returns the captured stderr and the command error.
func runSlackWithSecretWebhook(t *testing.T, transport http.RoundTripper) (string, error) {
	t.Helper()
	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	original := slackHTTPClient
	t.Cleanup(func() {
		slackHTTPClient = original
	})
	slackHTTPClient = func() *http.Client {
		return &http.Client{Transport: transport}
	}

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"--webhook", slackWebhookWithSecret(),
			"--message", "Build uploaded",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stderr, runErr
}

func TestNotifySlackTransportFailuresNeverExposeWebhookSecret(t *testing.T) {
	tests := []struct {
		name         string
		transportErr error
		wantContext  string
	}{
		{name: "dns", transportErr: &net.DNSError{Err: "no such host", Name: "hooks.slack.com"}, wantContext: ""},
		{name: "tls", transportErr: errors.New("tls: failed to verify certificate"), wantContext: ""},
		{name: "connection refused", transportErr: errors.New("connect: connection refused"), wantContext: ""},
		{name: "proxy", transportErr: errors.New("proxyconnect tcp: bad proxy"), wantContext: ""},
		{name: "redirect", transportErr: errors.New("stopped after 10 redirects"), wantContext: ""},
		{name: "timeout", transportErr: context.DeadlineExceeded, wantContext: "timeout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stderr, runErr := runSlackWithSecretWebhook(t, &failingSlackTransport{err: test.transportErr})
			if runErr == nil {
				t.Fatal("expected a transport error")
			}
			if strings.Contains(runErr.Error(), slackWebhookSecretSentinel) {
				t.Fatalf("error leaked the webhook secret: %q", runErr.Error())
			}
			if strings.Contains(stderr, slackWebhookSecretSentinel) {
				t.Fatalf("stderr leaked the webhook secret: %q", stderr)
			}
			if !strings.Contains(runErr.Error(), "notify slack: failed to send") {
				t.Fatalf("error dropped the operation context: %q", runErr.Error())
			}
			if !errors.Is(runErr, test.transportErr) {
				t.Fatalf("errors.Is lost the wrapped transport error: %v", runErr)
			}
			if test.wantContext != "" && !strings.Contains(runErr.Error(), test.wantContext) {
				t.Fatalf("error dropped %q context: %q", test.wantContext, runErr.Error())
			}
		})
	}
}

type respondingSlackTransport struct {
	status int
	body   string
}

func (t *respondingSlackTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(t.body)),
	}, nil
}

func TestNotifySlackNon2xxResponseBodyNeverExposesWebhookSecret(t *testing.T) {
	// Some servers and intercepting proxies echo the requested URL or path in
	// their error body, and for an incoming webhook the path is the secret.
	tests := []struct {
		name string
		body string
		keep string
	}{
		{name: "echoed full URL", body: "cannot POST " + slackWebhookWithSecret(), keep: "cannot POST"},
		{
			name: "echoed path",
			body: "no service at /services/T00000000/B00000000/" + slackWebhookSecretSentinel,
			keep: "no service at",
		},
		{name: "echoed token", body: "unknown token " + slackWebhookSecretSentinel, keep: "unknown token"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stderr, runErr := runSlackWithSecretWebhook(t, &respondingSlackTransport{status: http.StatusNotFound, body: test.body})
			if runErr == nil {
				t.Fatal("expected an unexpected-response error")
			}
			if strings.Contains(runErr.Error(), slackWebhookSecretSentinel) {
				t.Fatalf("error leaked the webhook secret: %q", runErr.Error())
			}
			if strings.Contains(stderr, slackWebhookSecretSentinel) {
				t.Fatalf("stderr leaked the webhook secret: %q", stderr)
			}
			if !strings.Contains(runErr.Error(), "unexpected response 404") {
				t.Fatalf("error dropped the status context: %q", runErr.Error())
			}
			if !strings.Contains(runErr.Error(), test.keep) {
				t.Fatalf("error dropped the harmless body context %q: %q", test.keep, runErr.Error())
			}
		})
	}
}

func TestNotifySlackNon2xxResponseBodyRemovesTerminalControls(t *testing.T) {
	splitSecret := slackWebhookSecretSentinel[:12] + "\u202e" + slackWebhookSecretSentinel[12:]
	stderr, runErr := runSlackWithSecretWebhook(t, &respondingSlackTransport{
		status: http.StatusBadGateway,
		body:   "proxy error " + splitSecret + "\x1b]8;;https://evil.invalid\x07click\x1b]8;;\x07\u202egpj.exe",
	})
	if runErr == nil {
		t.Fatal("expected an unexpected-response error")
	}
	if asc.HasInterpretedTerminalSequence(runErr.Error()) {
		t.Fatalf("error contains interpreted terminal sequences: %q", runErr.Error())
	}
	if asc.HasInterpretedTerminalSequence(stderr) {
		t.Fatalf("stderr contains interpreted terminal sequences: %q", stderr)
	}
	if strings.Contains(runErr.Error(), slackWebhookSecretSentinel) {
		t.Fatalf("error reconstructed the webhook secret after sanitization: %q", runErr.Error())
	}
	if !strings.Contains(runErr.Error(), "unexpected response 502") {
		t.Fatalf("error dropped the status context: %q", runErr.Error())
	}
	if !strings.Contains(runErr.Error(), "proxy error") {
		t.Fatalf("error dropped harmless body context: %q", runErr.Error())
	}
}
