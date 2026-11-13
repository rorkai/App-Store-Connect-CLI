package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	pricingcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
)

func TestPricingAvailabilityCreate_SendsPublicAPIRequest(t *testing.T) {
	for _, available := range []bool{true, false} {
		t.Run(strconv.FormatBool(available), func(t *testing.T) {
			setupAuth(t)

			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method == http.MethodGet && req.URL.Path == "/v1/territories" {
					w.Header().Set("Content-Type", "application/json")
					if req.URL.Query().Get("cursor") == "next" {
						_, _ = io.WriteString(w, `{"data":[{"type":"territories","id":"CAN"}],"links":{}}`)
					} else {
						_, _ = io.WriteString(w, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/territories?cursor=next"}}`)
					}
					return
				}
				if req.Method != http.MethodPost || req.URL.Path != "/v2/appAvailabilities" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				var payload asc.AppAvailabilityV2CreateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if payload.Data.Relationships.App.Data.ID != "app-1" {
					t.Fatalf("expected app-1, got %q", payload.Data.Relationships.App.Data.ID)
				}
				if payload.Data.Attributes == nil || payload.Data.Attributes.AvailableInNewTerritories == nil || !*payload.Data.Attributes.AvailableInNewTerritories {
					t.Fatalf("expected availableInNewTerritories=true, got %#v", payload.Data.Attributes)
				}
				if payload.Data.Relationships.TerritoryAvailabilities == nil || len(payload.Data.Relationships.TerritoryAvailabilities.Data) != 3 {
					t.Fatalf("expected three territory availability relationships, got %#v", payload.Data.Relationships.TerritoryAvailabilities)
				}
				if got := payload.Data.Relationships.TerritoryAvailabilities.Data[0].ID; got != "${local-usa}" {
					t.Fatalf("expected first local ID ${local-usa}, got %q", got)
				}
				if got := payload.Data.Relationships.TerritoryAvailabilities.Data[1].ID; got != "${local-fra}" {
					t.Fatalf("expected second local ID ${local-fra}, got %q", got)
				}
				if len(payload.Included) != 3 {
					t.Fatalf("expected three inline territory availabilities, got %d", len(payload.Included))
				}
				for _, included := range payload.Included {
					if included.Type != asc.ResourceTypeTerritoryAvailabilities {
						t.Fatalf("unexpected included type %q", included.Type)
					}
					if included.Attributes == nil || included.Attributes.Available != (available && included.Relationships.Territory.Data.ID != "CAN") {
						t.Fatalf("unexpected included territory availability, got %#v", included.Attributes)
					}
					if included.Relationships == nil || included.Relationships.Territory.Data.ID == "" {
						t.Fatalf("expected included territory relationship, got %#v", included.Relationships)
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, `{"data":{"type":"appAvailabilities","id":"app-1","attributes":{"availableInNewTerritories":true}}}`)
			}))
			t.Cleanup(server.Close)

			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			})
			client, err := asc.NewClientWithHTTPClient(
				"TEST_KEY",
				"TEST_ISSUER",
				os.Getenv("ASC_PRIVATE_KEY_PATH"),
				&http.Client{Transport: transport},
			)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			restore := pricingcli.SetAvailabilityClientFactory(func() (*asc.Client, error) {
				return client, nil
			})
			t.Cleanup(restore)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"pricing", "availability", "create",
					"--app", "app-1",
					"--territory", "US,USA,France",
					"--available", strconv.FormatBool(available),
					"--available-in-new-territories", "true",
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requestCount != 3 {
				t.Fatalf("expected two catalog pages and one mutation, got %d", requestCount)
			}
			var output asc.AppAvailabilityV2Response
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("parse stdout JSON: %v; stdout=%q", err, stdout)
			}
			if output.Data.ID != "app-1" || !output.Data.Attributes.AvailableInNewTerritories {
				t.Fatalf("unexpected output: %#v", output.Data)
			}
		})
	}
}

func TestPricingAvailabilityCreate_RelationshipRejectionExplainsFallback(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/territories" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"type":"territories","id":"USA"}],"links":{}}`)
			return
		}
		if req.Method != http.MethodPost || req.URL.Path != "/v2/appAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'territoryAvailabilities.territory' expects an included resource with type 'territories' and id 'TTO' but no matching resource was included."}]}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	restore := pricingcli.SetAvailabilityClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "availability", "create",
			"--app", "app-1",
			"--territory", "USA",
			"--available", "true",
			"--available-in-new-territories", "true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected relationship rejection")
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitConflict {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitConflict, got)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	want := "pricing availability create: Apple rejected the initial availability request through the public API; availability was not configured. Authenticate a web session with \"asc web auth login --apple-id EMAIL\", then retry with \"asc web apps availability create\", or configure Pricing and Availability in App Store Connect: The provided entity includes a relationship with an invalid value: The relationship 'territoryAvailabilities.territory' expects an included resource with type 'territories' and id 'TTO' but no matching resource was included."
	if runErr.Error() != want {
		t.Fatalf("unexpected error:\n got: %q\nwant: %q", runErr.Error(), want)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) {
		t.Fatalf("expected original API error cause, got %T", runErr)
	}
	if apiErr.Code != "ENTITY_ERROR.RELATIONSHIP.INVALID" || apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}

func TestPricingAvailabilityCreate_RejectsInvalidInputWithUsageExit(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "unknown territory",
			args:       []string{"pricing", "availability", "create", "--app", "app-1", "--territory", "Atlantis", "--available", "true", "--available-in-new-territories", "true"},
			wantStderr: `territory "Atlantis" could not be mapped`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)
			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d (err=%v)", cmd.ExitUsage, got, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
	}
}

func TestPricingAvailabilityCreate_CatalogFailureDoesNotCreate(t *testing.T) {
	for _, tc := range []struct{ name, catalog, want string }{
		{"empty", `{"data":[],"links":{}}`, "territory catalog is empty"},
		{"missing selected", `{"data":[{"type":"territories","id":"CAN"}],"links":{}}`, "missing from Apple's territory catalog"},
		{"invalid id", `{"data":[{"type":"territories","id":""}],"links":{}}`, "empty ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/territories" {
					posts++
					http.Error(w, "unexpected mutation", 500)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.catalog)
			}))
			t.Cleanup(server.Close)
			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			})
			client, err := asc.NewClientWithHTTPClient(
				"TEST_KEY",
				"TEST_ISSUER",
				os.Getenv("ASC_PRIVATE_KEY_PATH"),
				&http.Client{Transport: transport},
			)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			restore := pricingcli.SetAvailabilityClientFactory(func() (*asc.Client, error) {
				return client, nil
			})
			t.Cleanup(restore)

			root := RootCommand("test")
			if err := root.Parse([]string{"pricing", "availability", "create", "--app", "app-1", "--territory", "USA", "--available", "true", "--available-in-new-territories", "true"}); err != nil {
				t.Fatal(err)
			}
			err = root.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if posts != 0 {
				t.Fatalf("catalog failure caused %d mutations", posts)
			}
		})
	}
}
