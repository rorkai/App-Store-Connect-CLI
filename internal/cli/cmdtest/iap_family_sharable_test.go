package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIAPCreateFamilySharable(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases" {
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			resp := `{"data":{"type":"inAppPurchases","id":"new-iap","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","familySharable":true}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "create", "--app", "APP1", "--type", "CONSUMABLE", "--ref-name", "Pro", "--product-id", "com.example.pro", "--family-sharable"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	// Verify the request body contains familySharable: true
	var req asc.InAppPurchaseV2CreateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if !req.Data.Attributes.FamilySharable {
		t.Fatalf("expected familySharable=true in request, got %+v", req.Data.Attributes)
	}

	// Verify output contains the response
	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}

func TestIAPUpdateFamilySharableAsSoleFlag(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/inAppPurchases/") {
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			resp := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","familySharable":true}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "update", "--id", "iap-1", "--family-sharable"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	// Verify the request body contains familySharable: true
	var req asc.InAppPurchaseV2UpdateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if req.Data.Attributes == nil || req.Data.Attributes.FamilySharable == nil || !*req.Data.Attributes.FamilySharable {
		t.Fatalf("expected familySharable=true in request, got %+v", req.Data.Attributes)
	}
	// Name should not be set when only --family-sharable is provided
	if req.Data.Attributes.Name != nil {
		t.Fatalf("expected Name to be nil when only --family-sharable is passed, got %q", *req.Data.Attributes.Name)
	}

	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}
