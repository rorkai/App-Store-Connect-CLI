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

func TestTerritoryAvailabilitiesListPagination(t *testing.T) {
	const baseURL = "https://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?fields%5BterritoryAvailabilities%5D=available%2CreleaseDate%2CpreOrderEnabled%2CpreOrderPublishDate%2CcontentStatuses%2Cterritory&include=territory"
	const firstURL = baseURL + "&limit=200"
	const nextURL = baseURL + "&cursor=Mg"

	for _, command := range [][]string{
		{"pricing", "availability", "territory-availabilities"},
		{"pre-orders", "list"},
	} {
		commandName := strings.Join(command, " ")
		withArgs := func(args ...string) []string {
			return append(append([]string{}, command...), args...)
		}

		tests := []struct {
			name       string
			args       []string
			wantURLs   []string
			wantLimit  string
			bodies     []string
			wantOutIDs []string
		}{
			{
				name:       "paginate",
				args:       withArgs("--availability", "availability-1", "--paginate"),
				wantURLs:   []string{firstURL, nextURL},
				bodies:     []string{`{"data":[{"type":"territoryAvailabilities","id":"ta-1"}],"links":{"next":"` + nextURL + `"}}`, `{"data":[{"type":"territoryAvailabilities","id":"ta-2"}],"links":{"next":""}}`},
				wantOutIDs: []string{"ta-1", "ta-2"},
			},
			{
				name:       "limit",
				args:       withArgs("--availability", "availability-1", "--limit", "175"),
				wantLimit:  "175",
				bodies:     []string{`{"data":[{"type":"territoryAvailabilities","id":"ta-175"}],"links":{"next":""}}`},
				wantOutIDs: []string{"ta-175"},
			},
			{
				name:       "next",
				args:       withArgs("--next", nextURL),
				wantURLs:   []string{nextURL},
				bodies:     []string{`{"data":[{"type":"territoryAvailabilities","id":"ta-next"}],"links":{"next":""}}`},
				wantOutIDs: []string{"ta-next"},
			},
		}

		for _, test := range tests {
			t.Run(commandName+"/"+test.name, func(t *testing.T) {
				setupAuth(t)

				originalTransport := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = originalTransport })

				requestCount := 0
				http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requestCount++
					if requestCount > len(test.bodies) {
						t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
					}
					if req.Method != http.MethodGet || req.URL.Path != "/v2/appAvailabilities/availability-1/territoryAvailabilities" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					}
					if test.wantURLs != nil && req.URL.String() != test.wantURLs[requestCount-1] {
						t.Fatalf("request %d = %s, want %s", requestCount, req.URL.String(), test.wantURLs[requestCount-1])
					}
					if test.wantLimit != "" && req.URL.Query().Get("limit") != test.wantLimit {
						t.Fatalf("expected limit=%s, got %q", test.wantLimit, req.URL.Query().Get("limit"))
					}
					return jsonHTTPResponse(http.StatusOK, test.bodies[requestCount-1]), nil
				})

				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)

				stdout, stderr := captureOutput(t, func() {
					if err := root.Parse(test.args); err != nil {
						t.Fatalf("parse error: %v", err)
					}
					if err := root.Run(context.Background()); err != nil {
						t.Fatalf("run error: %v", err)
					}
				})

				if stderr != "" {
					t.Fatalf("expected empty stderr, got %q", stderr)
				}
				if requestCount != len(test.bodies) {
					t.Fatalf("requests = %d, want %d", requestCount, len(test.bodies))
				}
				for _, id := range test.wantOutIDs {
					if !strings.Contains(stdout, `"id":"`+id+`"`) {
						t.Fatalf("expected %s in output, got %q", id, stdout)
					}
				}
			})
		}

		runUsageExitCases(t, []usageExitCase{
			{
				name:    commandName + "/RejectsInvalidLimit",
				args:    withArgs("--availability", "availability-1", "--limit", "201"),
				wantErr: "Error: " + commandName + ": --limit must be between 1 and 200",
			},
			{
				name:    commandName + "/RejectsInvalidNextURL",
				args:    withArgs("--next", "http://api.appstoreconnect.apple.com/v2/appAvailabilities/availability-1/territoryAvailabilities?cursor=AQ"),
				wantErr: "Error: " + commandName + ": --next must be an App Store Connect URL",
			},
		})
	}
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
