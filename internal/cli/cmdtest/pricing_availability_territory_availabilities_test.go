package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPricingAvailabilityTerritoryAvailabilitiesPaginate(t *testing.T) {
	setupAuth(t)

	const firstURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2CpreOrderPublishDate%2CcontentStatuses%2Cterritory&include=territory&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2CpreOrderPublishDate%2CcontentStatuses%2Cterritory&include=territory&cursor=Mg"

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
			"pricing", "availability", "territory-availabilities",
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

func TestPricingAvailabilityTerritoryAvailabilitiesLimit(t *testing.T) {
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
			"pricing", "availability", "territory-availabilities",
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

func TestPricingAvailabilityTerritoryAvailabilitiesNext(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2CpreOrderPublishDate%2CcontentStatuses%2Cterritory&include=territory&cursor=Mg"

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
			"pricing", "availability", "territory-availabilities",
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

func TestPricingAvailabilityTerritoryAvailabilitiesUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "RejectsInvalidLimit",
			args:    []string{"pricing", "availability", "territory-availabilities", "--availability", "availability-1", "--limit", "201"},
			wantErr: "Error: pricing availability territory-availabilities: --limit must be between 1 and 200",
		},
		{
			name:    "RejectsInvalidNextURL",
			args:    []string{"pricing", "availability", "territory-availabilities", "--next", "http://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?cursor=AQ"},
			wantErr: "Error: pricing availability territory-availabilities: --next must be an App Store Connect URL",
		},
	})
}

func TestPricingAvailabilityTerritoryAvailabilitiesByAppOrAvailability(t *testing.T) {
	const territoryURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2CpreOrderPublishDate%2CcontentStatuses%2Cterritory&include=territory"

	tests := []struct {
		name         string
		args         []string
		wantRequests []string
	}{
		{
			name:         "availability",
			args:         []string{"--availability", "availability-1"},
			wantRequests: []string{territoryURL},
		},
		{
			name: "app",
			args: []string{"--app", "app-1"},
			wantRequests: []string{
				"https://api.appstoreconnect.apple.com/v1/apps/app-1/appAvailabilityV2",
				territoryURL,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")

			var requests []string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method: %s %s", req.Method, req.URL.String())
				}
				requests = append(requests, req.URL.String())
				body := `{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true,"preOrderEnabled":true,"preOrderPublishDate":"2026-11-01"}}],"links":{"next":""}}`
				if req.URL.Path == "/v1/apps/app-1/appAvailabilityV2" {
					body = `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"pricing", "availability", "territory-availabilities"}, tt.args...)
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if strings.Join(requests, "\n") != strings.Join(tt.wantRequests, "\n") {
				t.Fatalf("requests = %q, want %q", requests, tt.wantRequests)
			}
			if !strings.Contains(stdout, `"preOrderPublishDate":"2026-11-01"`) {
				t.Fatalf("expected preOrderPublishDate in output, got %q", stdout)
			}
		})
	}
}

func TestPricingAvailabilityTerritoryAvailabilitiesRejectsAppUsageBeforeLookup(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "availability",
			args:       []string{"--app", "app-1", "--availability", "availability-1"},
			wantStderr: "Error: --app and --availability are mutually exclusive",
		},
		{
			name:       "next",
			args:       []string{"--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?cursor=Mg"},
			wantStderr: "Error: --app and --next are mutually exclusive",
		},
		{
			name:       "limit",
			args:       []string{"--app", "app-1", "--limit", "201"},
			wantStderr: "Error: pricing availability territory-availabilities: --limit must be between 1 and 200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("expected no HTTP request, got %s %s", req.Method, req.URL.String())
				return nil, errors.New("unexpected request")
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"pricing", "availability", "territory-availabilities"}, tt.args...)
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Fatalf("expected %q in stderr, got %q", tt.wantStderr, stderr)
			}
		})
	}
}
