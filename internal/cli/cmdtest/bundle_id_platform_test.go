package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func setBundleIDPlatformTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create Bundle ID platform test client: %v", err)
	}
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restoreClient)
}

func TestDevicesListUsesBundleIDPlatformFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	setBundleIDPlatformTestServer(t, func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/devices" {
			t.Fatalf("expected GET /v1/devices, got %s %s", req.Method, req.URL.Path)
		}
		if got := req.URL.Query().Get("filter[platform]"); got != "UNIVERSAL" {
			t.Fatalf("filter[platform] = %q, want UNIVERSAL", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/devices"}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "list", "--platform", "universal"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestBundleIDsCreateUsesBundleIDPlatformPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	setBundleIDPlatformTestServer(t, func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected POST /v1/bundleIds, got %s %s", req.Method, req.URL.Path)
		}
		var payload struct {
			Data struct {
				Type       string `json:"type"`
				Attributes struct {
					Identifier string `json:"identifier"`
					Name       string `json:"name"`
					Platform   string `json:"platform"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload.Data.Type != "bundleIds" || payload.Data.Attributes.Identifier != "com.example.universal" || payload.Data.Attributes.Name != "Universal" || payload.Data.Attributes.Platform != "UNIVERSAL" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.universal","name":"Universal","platform":"UNIVERSAL"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"bundle-ids", "create", "--identifier", "com.example.universal", "--name", "Universal", "--platform", "universal"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestBundleIDsCreateSeedIDRequestAttributes(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  map[string]any
	}{
		{
			name: "omitted flag leaves Apple to assign the prefix",
			want: map[string]any{"identifier": "com.example.app", "name": "Example", "platform": "IOS"},
		},
		{
			name:  "chosen prefix is sent trimmed",
			flags: []string{"--seed-id", " ABCDE12345 "},
			want:  map[string]any{"identifier": "com.example.app", "name": "Example", "platform": "IOS", "seedId": "ABCDE12345"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			var got map[string]any
			setBundleIDPlatformTestServer(t, func(w http.ResponseWriter, req *http.Request) {
				var payload struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Errorf("decode body: %v", err)
				}
				got = payload.Data.Attributes
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","name":"Example","platform":"IOS"}}}`)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"bundle-ids", "create", "--identifier", "com.example.app", "--name", "Example"}, tt.flags...)
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			captureOutput(t, func() {
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("attributes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBundleIDsCreateRejectsBlankSeedIDBeforeRequest(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requestCount := 0
			setBundleIDPlatformTestServer(t, func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				http.Error(w, "unexpected request", http.StatusBadRequest)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse([]string{"bundle-ids", "create", "--identifier", "com.example.app", "--name", "Example", "--seed-id", value}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want usage error", err)
				}
			})
			assertUsageDiagnosticFirstLine(t, stderr, "--seed-id must not be empty")
			if requestCount != 0 {
				t.Fatalf("request count = %d, want 0", requestCount)
			}
		})
	}
}

func TestBundleIDPlatformCommandsRejectGeneralPlatforms(t *testing.T) {
	tests := []struct {
		name string
		args []string
		deps bool // the command is built on shared.Deps: run it on test deps
	}{
		{name: "devices list tvOS", args: []string{"devices", "list", "--platform", "TV_OS"}, deps: true},
		{name: "devices register visionOS", args: []string{"devices", "register", "--name", "Invalid", "--udid", "INVALID", "--platform", "VISION_OS"}, deps: true},
		{name: "bundle ID create tvOS", args: []string{"bundle-ids", "create", "--identifier", "com.example.invalid", "--name", "Invalid", "--platform", "TV_OS"}, deps: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr string
			var err error
			if tt.deps {
				_, stderr, err = runRootWithDeps(t, nil, tt.args...)
			} else {
				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)
				if parseErr := root.Parse(tt.args); parseErr != nil {
					t.Fatalf("parse error: %v", parseErr)
				}
				_, stderr = captureOutput(t, func() {
					err = root.Run(context.Background())
				})
			}
			if err == nil || !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			const diagnostic = "Error: --platform must be one of: IOS, MAC_OS, UNIVERSAL\n"
			if count := strings.Count(stderr, diagnostic); count != 1 {
				t.Fatalf("diagnostic count = %d in stderr %q", count, stderr)
			}
			if !strings.Contains(stderr, "USAGE\n") {
				t.Fatalf("expected usage on stderr, got %q", stderr)
			}
		})
	}
}
