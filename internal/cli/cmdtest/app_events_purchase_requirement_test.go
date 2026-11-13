package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAppEventsCreateAllowsOptionalEventTypeAndNormalizesPurchaseRequirement(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents" {
			t.Fatalf("expected /v1/appEvents path, got %s", req.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		data, ok := payload["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data object, got %T", payload["data"])
		}
		if data["type"] != "appEvents" {
			t.Fatalf("expected data type appEvents, got %v", data["type"])
		}
		attrs, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("expected attributes object, got %T", data["attributes"])
		}

		if attrs["purchaseRequirement"] != "NO_COST_ASSOCIATED" {
			t.Fatalf("expected purchaseRequirement NO_COST_ASSOCIATED, got %v", attrs["purchaseRequirement"])
		}
		if _, ok := attrs["badge"]; ok {
			t.Fatalf("expected optional badge to be omitted, got %v", attrs["badge"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch"}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "APP_ID",
			"--name", "Launch",
			"--purchase-requirement", "noCostAssociated",
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
	var response asc.AppEventResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if response.Data.ID != "event-1" {
		t.Fatalf("expected created event id event-1, got %q", response.Data.ID)
	}
}

func TestAppEventsRejectsUnsupportedPurchaseRequirement(t *testing.T) {
	runValidationTests(t, []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "create invalid value",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--event-type", "CHALLENGE", "--purchase-requirement", "free"},
			wantErr: "Error: --purchase-requirement currently supports only: NO_COST_ASSOCIATED",
		},
		{
			name:    "update known unsupported value",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID", "--purchase-requirement", "IAP_REQUIRED"},
			wantErr: "known 500 UNEXPECTED_ERROR",
		},
	})
}
