package cmdtest

import (
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"
)

func TestOptimizeKeywordsDiscoverUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"optimize", "keywords", "discover"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "non numeric app",
			args:    []string{"optimize", "keywords", "discover", "--app", "com.example.app"},
			wantErr: "--app must be a numeric App Store app ID",
		},
		{
			name:    "invalid limit",
			args:    []string{"optimize", "keywords", "discover", "--app", "1234567890", "--limit", "0"},
			wantErr: "--limit must be at least 1",
		},
		{
			name:    "invalid country",
			args:    []string{"optimize", "keywords", "discover", "--app", "1234567890", "--country", "usa"},
			wantErr: `--country "usa" is not a supported App Store storefront`,
		},
		{
			name:    "positional argument",
			args:    []string{"optimize", "keywords", "discover", "--app", "1234567890", "extra"},
			wantErr: "optimize keywords discover does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_APP_ID", "")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request before input validation: %s", req.URL.String())
				return nil, errors.New("unexpected request")
			})

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

func TestOptimizeKeywordsDiscoverFailsActionablyWithoutAppleAdsCredentials(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	t.Setenv("ASC_ADS_CLIENT_ID", "")
	t.Setenv("ASC_ADS_CLIENT_SECRET", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request without Apple Ads credentials: %s", req.URL.String())
		return nil, errors.New("unexpected request")
	})

	stdout, _, runErr := runCommand(t, []string{
		"optimize", "keywords", "discover",
		"--app", "1234567890",
		"--country", "US",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected an error when Apple Ads is the only source and is unavailable")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"optimize keywords discover", "--ad-account", "--ads-profile", "asc ads auth login"} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("error = %v, want it to mention %q", runErr, want)
		}
	}
}
