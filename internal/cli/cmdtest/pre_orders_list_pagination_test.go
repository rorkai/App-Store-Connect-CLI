package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPreOrdersListPaginate(t *testing.T) {
	setupAuth(t)

	const firstURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2Cterritory&include=territory&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2Cterritory&include=territory&cursor=Mg"

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
			body := `{"data":[{"type":"territoryAvailabilities","id":"ta-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"territoryAvailabilities","id":"ta-2"}],"links":{"next":""}}`
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
		if err := root.Parse([]string{
			"pre-orders", "list",
			"--availability", "availability-1",
			"--paginate",
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
	if !strings.Contains(stdout, `"id":"ta-1"`) || !strings.Contains(stdout, `"id":"ta-2"`) {
		t.Fatalf("expected paginated territory availabilities in output, got %q", stdout)
	}
}

func TestPreOrdersListLimit(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/appAvailabilities/availability-1/territoryAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Query().Get("limit") != "175" {
			t.Fatalf("expected limit=175, got %q", req.URL.Query().Get("limit"))
		}
		body := `{"data":[{"type":"territoryAvailabilities","id":"ta-175"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pre-orders", "list",
			"--availability", "availability-1",
			"--limit", "175",
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
	if !strings.Contains(stdout, `"id":"ta-175"`) {
		t.Fatalf("expected territory availability output, got %q", stdout)
	}
}

func TestPreOrdersListNext(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2Cterritory&include=territory&cursor=Mg"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != nextURL {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `{"data":[{"type":"territoryAvailabilities","id":"ta-next"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pre-orders", "list",
			"--next", nextURL,
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
	if !strings.Contains(stdout, `"id":"ta-next"`) {
		t.Fatalf("expected territory availability output, got %q", stdout)
	}
}

func TestPreOrdersListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pre-orders", "list",
			"--availability", "availability-1",
			"--limit", "201",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "pre-orders list: --limit must be between 1 and 200") {
		t.Fatalf("expected invalid limit error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPreOrdersListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pre-orders", "list",
			"--next", "http://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?cursor=AQ",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "pre-orders list: --next must be an App Store Connect URL") {
		t.Fatalf("expected invalid next url error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
