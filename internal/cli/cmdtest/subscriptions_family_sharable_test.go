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

func TestSubscriptionsCreateFamilySharable(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptions") {
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			resp := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Family","productId":"com.example.sub.family","familySharable":true}}}`
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
		if err := root.Parse([]string{"subscriptions", "create", "--group", "GRP1", "--ref-name", "Family", "--product-id", "com.example.sub.family", "--family-sharable"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var req asc.SubscriptionCreateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if req.Data.Attributes.FamilySharable == nil || !*req.Data.Attributes.FamilySharable {
		t.Fatalf("expected familySharable=true in request, got %+v", req.Data.Attributes)
	}

	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}

func TestSubscriptionsUpdateFamilySharableAsSoleFlag(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/subscriptions/") {
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			resp := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.sub.monthly","familySharable":true}}}`
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
		if err := root.Parse([]string{"subscriptions", "update", "--id", "sub-1", "--family-sharable"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var req asc.SubscriptionUpdateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if req.Data.Attributes.FamilySharable == nil || !*req.Data.Attributes.FamilySharable {
		t.Fatalf("expected familySharable=true in request, got %+v", req.Data.Attributes)
	}
	// Name and period should not be set when only --family-sharable is provided
	if req.Data.Attributes.Name != nil {
		t.Fatalf("expected Name to be nil when only --family-sharable is passed, got %q", *req.Data.Attributes.Name)
	}
	if req.Data.Attributes.SubscriptionPeriod != nil {
		t.Fatalf("expected SubscriptionPeriod to be nil when only --family-sharable is passed, got %q", *req.Data.Attributes.SubscriptionPeriod)
	}

	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}
