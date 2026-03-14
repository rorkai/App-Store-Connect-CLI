package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

const (
	iapPricesDeprecationWarning         = "Warning: `asc iap prices` is deprecated. Use `asc iap pricing summary`."
	iapPricePointsDeprecationWarning    = "Warning: `asc iap price-points list` is deprecated. Use `asc iap pricing price-points list`."
	iapPriceSchedulesDeprecationWarning = "Warning: `asc iap price-schedules get` is deprecated. Use `asc iap pricing schedules get`."
	iapAvailabilityDeprecationWarning   = "Warning: `asc iap availability get` is deprecated. Use `asc iap pricing availability get`."
	iapAvailabilitiesDeprecationWarning = "Warning: `asc iap availabilities get` is deprecated. Use `asc iap pricing availabilities get`."
)

func findCommandByPath(t *testing.T, path ...string) *ffcli.Command {
	t.Helper()

	current := RootCommand("1.2.3")
	for _, name := range path {
		found := false
		for _, sub := range current.Subcommands {
			if sub != nil && sub.Name == name {
				current = sub
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected command path %q to exist", strings.Join(path, " "))
		}
	}
	return current
}

func usageForCommand(t *testing.T, path ...string) string {
	t.Helper()

	cmd := findCommandByPath(t, path...)
	if cmd.UsageFunc == nil {
		t.Fatalf("expected usage func for %q", strings.Join(path, " "))
	}
	return cmd.UsageFunc(cmd)
}

func runRootCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestIAPHelpShowsCanonicalPricingRoot(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"iap"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "\n  pricing") {
		t.Fatalf("expected iap help to list pricing root, got %q", stderr)
	}
	for _, hidden := range []string{
		"\n  prices",
		"\n  price-points",
		"\n  price-schedules",
		"\n  availability",
		"\n  availabilities",
	} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected iap help to hide deprecated pricing alias %q, got %q", strings.TrimSpace(hidden), stderr)
		}
	}
}

func TestIAPPricingHelpShowsCanonicalSubcommands(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "pricing"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	for _, want := range []string{"summary", "price-points", "schedules", "availability", "availabilities"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected pricing help to contain %q, got %q", want, stderr)
		}
	}
	for _, hidden := range []string{"prices", "price-schedules"} {
		if strings.Contains(stderr, "\n  "+hidden+"\t") {
			t.Fatalf("expected pricing help to avoid legacy alias %q, got %q", hidden, stderr)
		}
	}
}

func TestIAPPricingMovedSurfacesUseCanonicalHelp(t *testing.T) {
	tests := []struct {
		name         string
		path         []string
		wantUsage    string
		wantContains []string
		wantNotShown []string
	}{
		{
			name:         "summary",
			path:         []string{"iap", "pricing", "summary"},
			wantUsage:    "asc iap pricing summary [flags]",
			wantContains: []string{"Show consolidated in-app purchase pricing summary."},
			wantNotShown: []string{"asc iap prices [flags]"},
		},
		{
			name:         "price points",
			path:         []string{"iap", "pricing", "price-points"},
			wantUsage:    "asc iap pricing price-points <subcommand> [flags]",
			wantContains: []string{"list", "equalizations"},
			wantNotShown: []string{"asc iap price-points <subcommand> [flags]"},
		},
		{
			name:         "schedules",
			path:         []string{"iap", "pricing", "schedules"},
			wantUsage:    "asc iap pricing schedules <subcommand> [flags]",
			wantContains: []string{"get", "base-territory", "create", "manual-prices", "automatic-prices"},
			wantNotShown: []string{"asc iap price-schedules <subcommand> [flags]"},
		},
		{
			name:         "availability",
			path:         []string{"iap", "pricing", "availability"},
			wantUsage:    "asc iap pricing availability <subcommand> [flags]",
			wantContains: []string{"get", "set"},
			wantNotShown: []string{"asc iap availability <subcommand> [flags]"},
		},
		{
			name:         "availabilities",
			path:         []string{"iap", "pricing", "availabilities"},
			wantUsage:    "asc iap pricing availabilities <subcommand> [flags]",
			wantContains: []string{"get", "available-territories"},
			wantNotShown: []string{"asc iap availabilities <subcommand> [flags]"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := usageForCommand(t, test.path...)
			if !strings.Contains(usage, test.wantUsage) {
				t.Fatalf("expected usage to contain %q, got %q", test.wantUsage, usage)
			}
			for _, want := range test.wantContains {
				if !strings.Contains(usage, want) {
					t.Fatalf("expected usage to contain %q, got %q", want, usage)
				}
			}
			for _, hidden := range test.wantNotShown {
				if strings.Contains(usage, hidden) {
					t.Fatalf("expected usage to hide %q, got %q", hidden, usage)
				}
			}
		})
	}
}

func TestDeprecatedIAPPricingAliasHelpPointsToCanonicalPaths(t *testing.T) {
	tests := []struct {
		name         string
		path         []string
		wantUsage    string
		wantContains []string
		wantNotShown []string
	}{
		{
			name:         "prices alias",
			path:         []string{"iap", "prices"},
			wantUsage:    "asc iap pricing summary [flags]",
			wantContains: []string{"DEPRECATED: use `asc iap pricing summary`."},
			wantNotShown: []string{"asc iap prices [flags]"},
		},
		{
			name:         "price points alias",
			path:         []string{"iap", "price-points"},
			wantUsage:    "asc iap pricing price-points <subcommand> [flags]",
			wantContains: []string{"Compatibility alias: use `asc iap pricing price-points ...`."},
			wantNotShown: []string{"asc iap price-points <subcommand> [flags]"},
		},
		{
			name:         "schedules alias",
			path:         []string{"iap", "price-schedules"},
			wantUsage:    "asc iap pricing schedules <subcommand> [flags]",
			wantContains: []string{"Compatibility alias: use `asc iap pricing schedules ...`."},
			wantNotShown: []string{"asc iap price-schedules <subcommand> [flags]"},
		},
		{
			name:         "availability alias",
			path:         []string{"iap", "availability"},
			wantUsage:    "asc iap pricing availability <subcommand> [flags]",
			wantContains: []string{"Compatibility alias: use `asc iap pricing availability ...`."},
			wantNotShown: []string{"asc iap availability <subcommand> [flags]"},
		},
		{
			name:         "availabilities alias",
			path:         []string{"iap", "availabilities"},
			wantUsage:    "asc iap pricing availabilities <subcommand> [flags]",
			wantContains: []string{"Compatibility alias: use `asc iap pricing availabilities ...`."},
			wantNotShown: []string{"asc iap availabilities <subcommand> [flags]"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := usageForCommand(t, test.path...)
			if !strings.Contains(usage, test.wantUsage) {
				t.Fatalf("expected usage to contain %q, got %q", test.wantUsage, usage)
			}
			for _, want := range test.wantContains {
				if !strings.Contains(usage, want) {
					t.Fatalf("expected usage to contain %q, got %q", want, usage)
				}
			}
			for _, hidden := range test.wantNotShown {
				if strings.Contains(usage, hidden) {
					t.Fatalf("expected usage to hide %q, got %q", hidden, usage)
				}
			}
		})
	}
}

func TestCanonicalIAPPricingValidationPaths(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "summary missing selector",
			args:    []string{"iap", "pricing", "summary"},
			wantErr: "--app or --iap-id is required",
		},
		{
			name:    "price points missing iap id",
			args:    []string{"iap", "pricing", "price-points", "list"},
			wantErr: "--iap-id is required",
		},
		{
			name:    "schedules missing selector",
			args:    []string{"iap", "pricing", "schedules", "get"},
			wantErr: "--iap-id or --schedule-id is required",
		},
		{
			name:    "availability missing iap id",
			args:    []string{"iap", "pricing", "availability", "get"},
			wantErr: "--iap-id is required",
		},
		{
			name:    "availabilities missing id",
			args:    []string{"iap", "pricing", "availabilities", "get"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLegacyIAPPricingAliasesWarnAndDelegateOnValidationPaths(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name        string
		args        []string
		wantErr     string
		wantWarning string
	}{
		{
			name:        "prices alias",
			args:        []string{"iap", "prices"},
			wantErr:     "--app or --iap-id is required",
			wantWarning: iapPricesDeprecationWarning,
		},
		{
			name:        "price points alias",
			args:        []string{"iap", "price-points", "list"},
			wantErr:     "--iap-id is required",
			wantWarning: iapPricePointsDeprecationWarning,
		},
		{
			name:        "schedules alias",
			args:        []string{"iap", "price-schedules", "get"},
			wantErr:     "--iap-id or --schedule-id is required",
			wantWarning: iapPriceSchedulesDeprecationWarning,
		},
		{
			name:        "availability alias",
			args:        []string{"iap", "availability", "get"},
			wantErr:     "--iap-id is required",
			wantWarning: iapAvailabilityDeprecationWarning,
		},
		{
			name:        "availabilities alias",
			args:        []string{"iap", "availabilities", "get"},
			wantErr:     "--id is required",
			wantWarning: iapAvailabilitiesDeprecationWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			requireStderrContainsWarning(t, stderr, test.wantWarning)
			if strings.Contains(stderr, `unknown subcommand "`) {
				t.Fatalf("expected alias to delegate to canonical leaf, got %q", stderr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLegacyIAPSummaryAliasWarnsAndMatchesCanonicalOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v2/inAppPurchases/iap-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}
				}`)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/iapPriceSchedule":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":{
						"type":"inAppPurchasePriceSchedules",
						"id":"schedule-1",
						"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
					},
					"included":[
						{
							"type":"inAppPurchasePrices",
							"id":"iap-price-1",
							"attributes":{"startDate":"2024-01-01","manual":true},
							"relationships":{
								"territory":{"data":{"type":"territories","id":"USA"}},
								"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-1"}}
							}
						},
						{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
					]
				}`)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/pricePoints":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"data":[{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}],
					"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
					"links":{"next":""}
				}`)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	run := func(args []string) (string, string) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"iap", "pricing", "summary", "--iap-id", "iap-1", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"iap", "prices", "--iap-id", "iap-1", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	requireStderrContainsWarning(t, aliasStderr, iapPricesDeprecationWarning)

	var canonicalPayload map[string]any
	if err := json.Unmarshal([]byte(canonicalStdout), &canonicalPayload); err != nil {
		t.Fatalf("parse canonical stdout: %v", err)
	}
	var aliasPayload map[string]any
	if err := json.Unmarshal([]byte(aliasStdout), &aliasPayload); err != nil {
		t.Fatalf("parse alias stdout: %v", err)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}
