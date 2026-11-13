package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestWebhooksListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"webhooks", "list"}, wantErr: "webhooks list: --next"},
	})
}

func TestWebhooksListRejectsNextQueryFlagConflicts(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-2/webhooks?cursor=AQ"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app before next",
			args:    []string{"webhooks", "list", "--app", "app-1", "--next", nextURL},
			wantErr: "webhooks list: --next cannot be combined with --app",
		},
		{
			name:    "explicit empty app after next",
			args:    []string{"webhooks", "list", "--next", nextURL, "--app", ""},
			wantErr: "webhooks list: --next cannot be combined with --app",
		},
		{
			name:    "limit after next",
			args:    []string{"webhooks", "list", "--next", nextURL, "--limit", "5"},
			wantErr: "webhooks list: --next cannot be combined with --limit",
		},
		{
			name:    "out of range limit before next",
			args:    []string{"webhooks", "list", "--limit", "201", "--next", nextURL},
			wantErr: "webhooks list: --next cannot be combined with --limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			assertUsageExit(t, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestWebhooksListPaginateFromNext(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/webhooks?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/webhooks?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhooks","id":"wh-next-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhooks","id":"wh-next-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"webhooks", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"wh-next-1"`) || !strings.Contains(stdout, `"id":"wh-next-2"`) {
		t.Fatalf("expected paginated webhooks in output, got %q", stdout)
	}
}

func TestWebhookDeliveriesPaginateFromNext(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/deliveries?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/deliveries?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhookDeliveries","id":"delivery-next-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhookDeliveries","id":"delivery-next-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"webhooks", "deliveries", "--webhook-id", "wh-1", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"delivery-next-1"`) || !strings.Contains(stdout, `"id":"delivery-next-2"`) {
		t.Fatalf("expected paginated deliveries in output, got %q", stdout)
	}
}

func TestWebhookDeliveriesRelationshipsRejectsInvalidNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
		// usageError marks a pre-request flag check that reports its own
		// diagnostic and exits with usage semantics (#518). The --next ID
		// extraction below still fails as an ordinary runtime error.
		usageError bool
	}{
		{
			name:       "invalid scheme",
			next:       "http://api.appstoreconnect.apple.com/v1/webhooks/wh-1/relationships/deliveries?cursor=AQ",
			wantErr:    "webhooks deliveries links: --next must be an App Store Connect URL",
			usageError: true,
		},
		{
			name:    "invalid path for id extraction",
			next:    "https://api.appstoreconnect.apple.com/v1/webhooks//relationships/deliveries?cursor=AQ",
			wantErr: "webhooks deliveries links: invalid --next URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"webhooks", "deliveries", "links", "--next", test.next}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if test.usageError {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", runErr)
				}
				if !strings.Contains(stderr, "Error: "+test.wantErr) {
					t.Fatalf("expected stderr to contain %q, got %q", "Error: "+test.wantErr, stderr)
				}
				return
			}
			if errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("errors.Is(flag.ErrHelp) = true, want generic exit semantics: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestWebhookDeliveriesRelationshipsPaginateFromNextWithoutWebhookID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/relationships/deliveries?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/relationships/deliveries?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhookDeliveries","id":"delivery-rel-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"webhookDeliveries","id":"delivery-rel-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"webhooks", "deliveries", "links", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"delivery-rel-1"`) || !strings.Contains(stdout, `"id":"delivery-rel-2"`) {
		t.Fatalf("expected paginated relationship deliveries in output, got %q", stdout)
	}
}
