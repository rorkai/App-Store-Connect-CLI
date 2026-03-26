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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

type publicPricesPayload struct {
	AppID          int64   `json:"appId"`
	Name           string  `json:"name"`
	Country        string  `json:"country"`
	CountryName    string  `json:"countryName"`
	Price          float64 `json:"price"`
	FormattedPrice string  `json:"formattedPrice"`
	Currency       string  `json:"currency"`
	IsFree         bool    `json:"isFree"`
}

type publicDescriptionsPayload struct {
	AppID       int64  `json:"appId"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryName string `json:"countryName"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type publicSearchPayload struct {
	Term    string                `json:"term"`
	Country string                `json:"country"`
	Limit   int                   `json:"limit"`
	Results []itunes.SearchResult `json:"results"`
}

func runCommand(t *testing.T, args []string) (string, string, error) {
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

func TestAppsHelpShowsPublicSubcommand(t *testing.T) {
	root := RootCommand("1.2.3")
	var appsCmd any
	for _, sub := range root.Subcommands {
		if sub != nil && sub.Name == "apps" {
			appsCmd = sub
			break
		}
	}
	if appsCmd == nil {
		t.Fatal("expected apps command in root subcommands")
	}

	usage := appsCmd.(*ffcli.Command).UsageFunc(appsCmd.(*ffcli.Command))
	if !strings.Contains(usage, "public") {
		t.Fatalf("expected apps help to show public subcommand, got %q", usage)
	}
}

func TestAppsPublicHelpShowsSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")
	publicCmd := findSubcommand(root, "apps", "public")
	if publicCmd == nil {
		t.Fatal("expected apps public command")
	}

	usage := publicCmd.UsageFunc(publicCmd)
	for _, want := range []string{"view", "search", "prices", "descriptions", "storefronts", "No authentication is required."} {
		if !strings.Contains(usage, want) {
			t.Fatalf("expected apps public help to contain %q, got %q", want, usage)
		}
	}
}

func TestAppsPublicValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "view missing app",
			args:    []string{"apps", "public", "view"},
			wantErr: "--app is required",
		},
		{
			name:    "view conflicting app aliases",
			args:    []string{"apps", "public", "view", "--app", "123", "--id", "456"},
			wantErr: "--app and --id are mutually exclusive",
		},
		{
			name:    "view invalid app id",
			args:    []string{"apps", "public", "view", "--app", "abc"},
			wantErr: "--app must be a numeric App Store app ID",
		},
		{
			name:    "view invalid country",
			args:    []string{"apps", "public", "view", "--app", "123", "--country", "usa"},
			wantErr: "unsupported country code",
		},
		{
			name:    "search missing term",
			args:    []string{"apps", "public", "search"},
			wantErr: "--term is required",
		},
		{
			name:    "search invalid limit",
			args:    []string{"apps", "public", "search", "--term", "focus", "--limit", "0"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "search invalid country",
			args:    []string{"apps", "public", "search", "--term", "focus", "--country", "usa"},
			wantErr: "unsupported country code",
		},
		{
			name:    "prices invalid country",
			args:    []string{"apps", "public", "prices", "--app", "123", "--country", "usa"},
			wantErr: "unsupported country code",
		},
		{
			name:    "descriptions invalid country",
			args:    []string{"apps", "public", "descriptions", "--app", "123", "--country", "usa"},
			wantErr: "unsupported country code",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)
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

func TestAppsPublicRejectsUnsupportedTwoLetterCountryBeforeRequest(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request for unsupported country: %s", req.URL.String())
		return nil, errors.New("unexpected request")
	})

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "view",
			args: []string{"apps", "public", "view", "--app", "123", "--country", "zz"},
		},
		{
			name: "search",
			args: []string{"apps", "public", "search", "--term", "focus", "--country", "zz"},
		},
		{
			name: "prices",
			args: []string{"apps", "public", "prices", "--app", "123", "--country", "zz"},
		},
		{
			name: "descriptions",
			args: []string{"apps", "public", "descriptions", "--app", "123", "--country", "zz"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "unsupported country code: zz") {
				t.Fatalf("expected stderr to contain unsupported country code, got %q", stderr)
			}
		})
	}
}

func TestAppsPublicRejectsSignedAppIDsBeforeRequest(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request for signed app ID: %s", req.URL.String())
		return nil, errors.New("unexpected request")
	})

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "view negative app",
			args: []string{"apps", "public", "view", "--app", "-123"},
		},
		{
			name: "prices positive app",
			args: []string{"apps", "public", "prices", "--app", "+123"},
		},
		{
			name: "descriptions negative alias",
			args: []string{"apps", "public", "descriptions", "--id", "-123"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--app must be a numeric App Store app ID") {
				t.Fatalf("expected stderr to contain numeric app ID error, got %q", stderr)
			}
		})
	}
}

func TestAppsPublicAliasIsSilentAndMatchesCanonical(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "poison")
	t.Setenv("ASC_ISSUER_ID", "poison")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/nonexistent")
	t.Setenv("ASC_PRIVATE_KEY", "poison")
	t.Setenv("ASC_PRIVATE_KEY_B64", "poison")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	lookupBody := `{
		"resultCount": 1,
		"results": [{
			"trackId": 123,
			"trackName": "Alpha",
			"bundleId": "com.example.alpha",
			"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123",
			"artworkUrl512": "https://example.com/icon.png",
			"sellerName": "Alpha Inc",
			"primaryGenreName": "Games",
			"genres": ["Games", "Action"],
			"version": "1.0.0",
			"description": "Alpha description",
			"price": 0,
			"formattedPrice": "Free",
			"currency": "USD",
			"averageUserRating": 4.5,
			"userRatingCount": 12,
			"averageUserRatingForCurrentVersion": 4.4,
			"userRatingCountForCurrentVersion": 11
		}]
	}`

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/lookup" {
			t.Fatalf("expected /lookup, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("id"); got != "123" {
			t.Fatalf("expected id=123, got %q", got)
		}
		if got := req.URL.Query().Get("country"); got != "us" {
			t.Fatalf("expected country=us, got %q", got)
		}
		if got := req.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(lookupBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	canonicalStdout, canonicalStderr, canonicalErr := runCommand(t, []string{"apps", "public", "view", "--app", "123", "--output", "json"})
	aliasStdout, aliasStderr, aliasErr := runCommand(t, []string{"apps", "public", "view", "--id", "123", "--output", "json"})
	matchingStdout, matchingStderr, matchingErr := runCommand(t, []string{"apps", "public", "view", "--app", "123", "--id", "123", "--output", "json"})

	if canonicalErr != nil {
		t.Fatalf("canonical run error: %v", canonicalErr)
	}
	if aliasErr != nil {
		t.Fatalf("alias run error: %v", aliasErr)
	}
	if matchingErr != nil {
		t.Fatalf("matching alias run error: %v", matchingErr)
	}
	if canonicalStderr != "" {
		t.Fatalf("expected canonical stderr to be empty, got %q", canonicalStderr)
	}
	if aliasStderr != "" {
		t.Fatalf("expected alias stderr to be empty, got %q", aliasStderr)
	}
	if matchingStderr != "" {
		t.Fatalf("expected matching alias stderr to be empty, got %q", matchingStderr)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias outputs to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
	if canonicalStdout != matchingStdout {
		t.Fatalf("expected canonical and matching alias outputs to match, canonical=%q matching=%q", canonicalStdout, matchingStdout)
	}

	var payload itunes.App
	if err := json.Unmarshal([]byte(canonicalStdout), &payload); err != nil {
		t.Fatalf("unmarshal view payload: %v", err)
	}
	if payload.Country != "US" {
		t.Fatalf("Country = %q, want US", payload.Country)
	}
	if payload.CountryName != "United States" {
		t.Fatalf("CountryName = %q, want United States", payload.CountryName)
	}
}

func TestAppsPublicViewAcceptsZeroPaddedAppIDAndUnlistedCountry(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/lookup" {
			t.Fatalf("expected /lookup, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("id"); got != "123" {
			t.Fatalf("expected canonical id=123, got %q", got)
		}
		if got := req.URL.Query().Get("country"); got != "kz" {
			t.Fatalf("expected country=kz, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"resultCount": 1,
				"results": [{
					"trackId": 123,
					"trackName": "Alpha",
					"bundleId": "com.example.alpha",
					"trackViewUrl": "https://apps.apple.com/kz/app/alpha/id123",
					"artworkUrl512": "https://example.com/icon.png",
					"sellerName": "Alpha Inc",
					"primaryGenreName": "Games",
					"version": "1.0.0",
					"description": "Alpha description",
					"formattedPrice": "Free",
					"currency": "KZT",
					"averageUserRating": 4.5,
					"userRatingCount": 12
				}]
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	stdout, stderr, runErr := runCommand(t, []string{"apps", "public", "view", "--app", "00123", "--country", "kz", "--output", "json"})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload itunes.App
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal view payload: %v", err)
	}
	if payload.AppID != 123 {
		t.Fatalf("AppID = %d, want 123", payload.AppID)
	}
	if payload.Country != "KZ" {
		t.Fatalf("Country = %q, want KZ", payload.Country)
	}
}

func TestAppsPublicStorefrontsListIncludesPublicOnlyCountry(t *testing.T) {
	stdout, stderr, runErr := runCommand(t, []string{"apps", "public", "storefronts", "list", "--output", "json"})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload []itunes.Storefront
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal storefronts: %v", err)
	}

	for _, storefront := range payload {
		if storefront.Country != "KZ" {
			continue
		}
		if storefront.CountryName != "Kazakhstan" {
			t.Fatalf("CountryName = %q, want Kazakhstan", storefront.CountryName)
		}
		if storefront.StorefrontID != "" {
			t.Fatalf("StorefrontID = %q, want empty string", storefront.StorefrontID)
		}
		return
	}

	t.Fatal("expected KZ storefront in public storefront list output")
}

func TestAppsPublicOutputFormats(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	lookupBody := `{
		"resultCount": 1,
		"results": [{
			"trackId": 123,
			"trackName": "Alpha",
			"bundleId": "com.example.alpha",
			"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123",
			"artworkUrl512": "https://example.com/icon.png",
			"sellerName": "Alpha Inc",
			"primaryGenreName": "Games",
			"genres": ["Games", "Action"],
			"version": "1.0.0",
			"description": "Alpha description",
			"price": 0,
			"formattedPrice": "Free",
			"currency": "USD",
			"averageUserRating": 4.5,
			"userRatingCount": 12,
			"averageUserRatingForCurrentVersion": 4.4,
			"userRatingCountForCurrentVersion": 11
		}]
	}`

	searchBody := `{
		"resultCount": 1,
		"results": [{
			"trackId": 321,
			"trackName": "Focus App",
			"bundleId": "com.example.focus",
			"trackViewUrl": "https://apps.apple.com/us/app/focus/id321",
			"artworkUrl512": "https://example.com/focus.png",
			"sellerName": "Focus Inc",
			"primaryGenreName": "Productivity",
			"formattedPrice": "$1.99",
			"currency": "USD",
			"averageUserRating": 4.8,
			"userRatingCount": 44
		}]
	}`

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/lookup":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(lookupBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/search":
			if got := req.URL.Query().Get("term"); got != "focus" {
				t.Fatalf("expected term=focus, got %q", got)
			}
			if got := req.URL.Query().Get("country"); got != "us" {
				t.Fatalf("expected country=us, got %q", got)
			}
			if got := req.URL.Query().Get("entity"); got != "software" {
				t.Fatalf("expected entity=software, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "20" {
				t.Fatalf("expected limit=20, got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(searchBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	tests := []struct {
		name        string
		args        []string
		wantStrings []string
		unmarshal   func(t *testing.T, stdout string)
	}{
		{
			name: "view json",
			args: []string{"apps", "public", "view", "--app", "123", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload itunes.App
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal view: %v", err)
				}
				if payload.Name != "Alpha" || payload.Country != "US" {
					t.Fatalf("unexpected view payload: %+v", payload)
				}
			},
		},
		{
			name:        "view table",
			args:        []string{"apps", "public", "view", "--app", "123", "--output", "table"},
			wantStrings: []string{"Field", "Value", "Alpha", "United States"},
		},
		{
			name:        "view markdown",
			args:        []string{"apps", "public", "view", "--app", "123", "--output", "markdown"},
			wantStrings: []string{"| Field", "Value", "Alpha", "United States"},
		},
		{
			name: "prices json",
			args: []string{"apps", "public", "prices", "--app", "123", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload publicPricesPayload
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal prices: %v", err)
				}
				if payload.IsFree != true || payload.Country != "US" {
					t.Fatalf("unexpected prices payload: %+v", payload)
				}
			},
		},
		{
			name:        "prices table",
			args:        []string{"apps", "public", "prices", "--app", "123", "--output", "table"},
			wantStrings: []string{"Field", "Value", "Formatted Price", "Free"},
		},
		{
			name:        "prices markdown",
			args:        []string{"apps", "public", "prices", "--app", "123", "--output", "markdown"},
			wantStrings: []string{"| Field", "Value", "Formatted Price", "Free"},
		},
		{
			name: "descriptions json",
			args: []string{"apps", "public", "descriptions", "--app", "123", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload publicDescriptionsPayload
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal descriptions: %v", err)
				}
				if payload.Version != "1.0.0" || payload.Country != "US" {
					t.Fatalf("unexpected descriptions payload: %+v", payload)
				}
			},
		},
		{
			name:        "descriptions table",
			args:        []string{"apps", "public", "descriptions", "--app", "123", "--output", "table"},
			wantStrings: []string{"Field", "Value", "Description", "Alpha description"},
		},
		{
			name:        "descriptions markdown",
			args:        []string{"apps", "public", "descriptions", "--app", "123", "--output", "markdown"},
			wantStrings: []string{"| Field", "Value", "Alpha description"},
		},
		{
			name: "search json",
			args: []string{"apps", "public", "search", "--term", "focus", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload publicSearchPayload
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal search: %v", err)
				}
				if payload.Term != "focus" || payload.Country != "US" || len(payload.Results) != 1 {
					t.Fatalf("unexpected search payload: %+v", payload)
				}
			},
		},
		{
			name:        "search table",
			args:        []string{"apps", "public", "search", "--term", "focus", "--output", "table"},
			wantStrings: []string{"Term: focus", "Country: US", "App ID", "Focus App"},
		},
		{
			name:        "search markdown",
			args:        []string{"apps", "public", "search", "--term", "focus", "--output", "markdown"},
			wantStrings: []string{"## Search Results", "| Field", "Value", "Focus App"},
		},
		{
			name: "storefronts json",
			args: []string{"apps", "public", "storefronts", "list", "--output", "json"},
			unmarshal: func(t *testing.T, stdout string) {
				t.Helper()
				var payload []itunes.Storefront
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal storefronts: %v", err)
				}
				if len(payload) == 0 || payload[0].Country != "AE" {
					t.Fatalf("unexpected storefront payload: %+v", payload[:1])
				}
			},
		},
		{
			name:        "storefronts table",
			args:        []string{"apps", "public", "storefronts", "list", "--output", "table"},
			wantStrings: []string{"Country", "Country Name", "Storefront ID", "AE"},
		},
		{
			name:        "storefronts markdown",
			args:        []string{"apps", "public", "storefronts", "list", "--output", "markdown"},
			wantStrings: []string{"| Country", "Country Name", "Storefront ID", "AE"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.unmarshal != nil {
				test.unmarshal(t, stdout)
			}
			for _, want := range test.wantStrings {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}
