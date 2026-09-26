package cmdtest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestLocalizationsUpdateClearsNullableFields(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		path         string
		response     string
		wantContains []string
		wantMissing  []string
	}{
		{
			name:         "app-info subtitle",
			args:         []string{"--type", "app-info", "--id", "loc-1", "--clear-subtitle", "--confirm"},
			path:         "/v1/appInfoLocalizations/loc-1",
			response:     `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","subtitle":null}}}`,
			wantContains: []string{`"subtitle":null`},
			wantMissing:  []string{`"name"`, `"privacyPolicyUrl"`, `"privacyChoicesUrl"`, `"privacyPolicyText"`},
		},
		{
			name:         "app-info privacy policy URL",
			args:         []string{"--type", "app-info", "--id", "loc-1", "--clear-privacy-policy-url", "--confirm"},
			path:         "/v1/appInfoLocalizations/loc-1",
			response:     `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","privacyPolicyUrl":null}}}`,
			wantContains: []string{`"privacyPolicyUrl":null`},
			wantMissing:  []string{`"name"`, `"subtitle"`, `"privacyChoicesUrl"`, `"privacyPolicyText"`},
		},
		{
			name:         "app-info subtitle cleared alongside name update",
			args:         []string{"--type", "app-info", "--id", "loc-1", "--name", "New Name", "--clear-subtitle", "--confirm"},
			path:         "/v1/appInfoLocalizations/loc-1",
			response:     `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"New Name","subtitle":null}}}`,
			wantContains: []string{`"subtitle":null`, `"name":"New Name"`},
			wantMissing:  []string{`"privacyPolicyUrl"`, `"privacyChoicesUrl"`, `"privacyPolicyText"`},
		},
		{
			name:         "false clear does not conflict with subtitle update",
			args:         []string{"--type", "app-info", "--id", "loc-1", "--subtitle", "Set", "--clear-subtitle=false"},
			path:         "/v1/appInfoLocalizations/loc-1",
			response:     `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","subtitle":"Set"}}}`,
			wantContains: []string{`"subtitle":"Set"`},
			wantMissing:  []string{`"subtitle":null`},
		},
		{
			name:         "false clear for another type is ignored",
			args:         []string{"--id", "loc-1", "--promotional-text", "Set", "--clear-subtitle=false"},
			path:         "/v1/appStoreVersionLocalizations/loc-1",
			response:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","promotionalText":"Set"}}}`,
			wantContains: []string{`"promotionalText":"Set"`},
			wantMissing:  []string{`"subtitle"`, `"promotionalText":null`},
		},
		{
			name:         "version promotional text",
			args:         []string{"--id", "loc-1", "--clear-promotional-text", "--confirm"},
			path:         "/v1/appStoreVersionLocalizations/loc-1",
			response:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","promotionalText":null}}}`,
			wantContains: []string{`"promotionalText":null`},
			wantMissing:  []string{`"description"`, `"keywords"`, `"whatsNew"`, `"supportUrl"`, `"marketingUrl"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocUpdateAuth(t)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			requestCount := 0
			patchBody := ""
			http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch || req.URL.Path != test.path {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				requestCount++
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				patchBody = string(body)
				return locUpdateJSONResponse(test.response)
			})

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(append([]string{"localizations", "update"}, test.args...), "1.2.3")
				if code != cmd.ExitSuccess {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requestCount != 1 {
				t.Fatalf("request count = %d, want one PATCH", requestCount)
			}
			for _, want := range test.wantContains {
				if !strings.Contains(patchBody, want) {
					t.Fatalf("expected patch body to contain %s, got %s", want, patchBody)
				}
			}
			for _, unwanted := range test.wantMissing {
				if strings.Contains(patchBody, unwanted) {
					t.Fatalf("expected patch body to omit %s, got %s", unwanted, patchBody)
				}
			}
			if !strings.Contains(stdout, `"id":"loc-1"`) {
				t.Fatalf("expected updated localization output, got %q", stdout)
			}
		})
	}
}

func TestLocalizationsUpdateRequiresConfirmForClearBeforeClient(t *testing.T) {
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client should not be created")
	}))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"localizations", "update", "--type", "app-info", "--id", "loc-1", "--clear-subtitle"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected usage exit, got %d", code)
		}
	})
	if stdout != "" || !strings.Contains(stderr, "--confirm is required to clear localization fields") {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
	}
	if clientCalls != 0 {
		t.Fatalf("client calls = %d, want zero", clientCalls)
	}
}

func TestLocalizationsUpdateRejectsClearFlagConflicts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subtitle",
			args:    []string{"--type", "app-info", "--id", "loc-1", "--subtitle", "Set", "--clear-subtitle"},
			wantErr: "--subtitle and --clear-subtitle are mutually exclusive",
		},
		{
			name:    "privacy policy URL",
			args:    []string{"--type", "app-info", "--id", "loc-1", "--privacy-policy-url", "https://example.com/privacy", "--clear-privacy-policy-url"},
			wantErr: "--privacy-policy-url and --clear-privacy-policy-url are mutually exclusive",
		},
		{
			name:    "promotional text",
			args:    []string{"--id", "loc-1", "--promotional-text", "Set", "--clear-promotional-text"},
			wantErr: "--promotional-text and --clear-promotional-text are mutually exclusive",
		},
		{
			name:    "empty set flag still conflicts",
			args:    []string{"--type", "app-info", "--id", "loc-1", "--subtitle", "", "--clear-subtitle"},
			wantErr: "--subtitle and --clear-subtitle are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client should not be created")
			}))

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(append([]string{"localizations", "update"}, test.args...), "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("expected no client creation, got %d", clientCalls)
			}
		})
	}
}

func TestLocalizationsUpdateRejectsClearFlagsForWrongType(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subtitle on version localization",
			args:    []string{"--id", "loc-1", "--clear-subtitle"},
			wantErr: "--clear-subtitle requires --type app-info",
		},
		{
			name:    "privacy policy URL on version localization",
			args:    []string{"--id", "loc-1", "--clear-privacy-policy-url"},
			wantErr: "--clear-privacy-policy-url requires --type app-info",
		},
		{
			name:    "promotional text on app-info localization",
			args:    []string{"--type", "app-info", "--id", "loc-1", "--clear-promotional-text"},
			wantErr: "--clear-promotional-text requires --type version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client should not be created")
			}))

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(append([]string{"localizations", "update"}, test.args...), "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("expected no client creation, got %d", clientCalls)
			}
		})
	}
}

func TestLocalizationsUpdateClearFailureListsClearedField(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusConflict,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"Invalid","detail":"(-50)"}]}`)),
		}, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--id", "loc-1",
			"--clear-promotional-text",
			"--confirm",
		}, "1.2.3")
		if code == cmd.ExitSuccess {
			t.Fatal("expected non-zero exit code")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{
		"PATCH /v1/appStoreVersionLocalizations/loc-1",
		"promotionalText",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}
