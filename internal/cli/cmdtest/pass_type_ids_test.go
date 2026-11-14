package cmdtest

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPassTypeIDsCertificateQueryFlagsRequireIncludeBeforeClient(t *testing.T) {
	const (
		certificateFieldsError = "Error: --certificate-fields requires --include certificates"
		certificateLimitError  = "Error: --limit-certificates requires --include certificates"
	)
	commands := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"pass-type-ids", "list"}},
		{name: "view", args: []string{"pass-type-ids", "view", "--pass-type-id", "PASS_ID"}},
	}
	flags := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "certificate fields", args: []string{"--certificate-fields", "name"}, wantErr: certificateFieldsError},
		{name: "certificate limit", args: []string{"--limit-certificates", "1"}, wantErr: certificateLimitError},
		{
			name:    "deterministic precedence",
			args:    []string{"--limit-certificates", "1", "--certificate-fields", "name"},
			wantErr: certificateFieldsError,
		},
	}

	clientFactoryCalls := 0

	for _, command := range commands {
		for _, flagCase := range flags {
			t.Run(command.name+"/"+flagCase.name, func(t *testing.T) {
				args := append(append([]string(nil), command.args...), flagCase.args...)
				stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) {
					clientFactoryCalls++
					return nil, errors.New("client factory must not run during validation")
				}, args...)

				if code != rootcmd.ExitUsage {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
				if stdout != "" {
					t.Errorf("stdout = %q, want empty", stdout)
				}
				if count := strings.Count(stderr, flagCase.wantErr); count != 1 {
					t.Errorf("stderr contains %q %d times, want once: %q", flagCase.wantErr, count, stderr)
				}
				if clientFactoryCalls != 0 {
					t.Errorf("client factory calls = %d, want 0", clientFactoryCalls)
				}
			})
		}
	}
}

func TestPassTypeIDsCertificateQueryFlagsPreserveValidRequests(t *testing.T) {
	certificateArgs := []string{
		"--fields", "certificates",
		"--include", "certificates",
		"--certificate-fields", "name,expirationDate",
		"--limit-certificates", "1",
		"--output", "json",
	}
	certificateQuery := url.Values{
		"fields[passTypeIds]":  {"certificates"},
		"fields[certificates]": {"name,expirationDate"},
		"include":              {"certificates"},
		"limit[certificates]":  {"1"},
	}
	tests := []struct {
		name     string
		args     []string
		path     string
		query    url.Values
		response string
		wantID   string
	}{
		{
			name:     "list",
			args:     append([]string{"pass-type-ids", "list"}, certificateArgs...),
			path:     "/v1/passTypeIds",
			query:    certificateQuery,
			response: `{"data":[{"type":"passTypeIds","id":"PASS_LIST"}]}`,
			wantID:   "PASS_LIST",
		},
		{
			name:     "view",
			args:     append([]string{"pass-type-ids", "view", "--pass-type-id", "PASS_VIEW"}, certificateArgs...),
			path:     "/v1/passTypeIds/PASS_VIEW",
			query:    certificateQuery,
			response: `{"data":{"type":"passTypeIds","id":"PASS_VIEW"}}`,
			wantID:   "PASS_VIEW",
		},
		{
			name: "relationship field without include",
			args: []string{
				"pass-type-ids", "list",
				"--fields", "certificates",
				"--output", "json",
			},
			path:     "/v1/passTypeIds",
			query:    url.Values{"fields[passTypeIds]": {"certificates"}},
			response: `{"data":[{"type":"passTypeIds","id":"PASS_FIELDS"}]}`,
			wantID:   "PASS_FIELDS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Errorf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if got := req.URL.Query().Encode(); got != test.query.Encode() {
					t.Errorf("query = %q, want %q", got, test.query.Encode())
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.response)
			}))
			t.Cleanup(server.Close)

			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
					t.Errorf("request URL = %s, want official App Store Connect host", req.URL.String())
				}
				if authorization := req.Header.Get("Authorization"); !strings.HasPrefix(authorization, "Bearer ") || authorization == "Bearer " {
					t.Errorf("Authorization = %q, want non-empty bearer token", authorization)
				}
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			})
			client, err := asc.NewClientWithHTTPClient(
				"TEST_KEY",
				"TEST_ISSUER",
				os.Getenv("ASC_PRIVATE_KEY_PATH"),
				&http.Client{Transport: transport},
			)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) {
				return client, nil
			}, test.args...)
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr: %s", code, rootcmd.ExitSuccess, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, `"id":"`+test.wantID+`"`) {
				t.Fatalf("stdout missing ID %q: %s", test.wantID, stdout)
			}
		})
	}
}

func TestPassTypeIDsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "pass-type-ids list missing pass type id for certificates list",
			args:    []string{"pass-type-ids", "certificates", "list"},
			wantErr: "--pass-type-id is required",
		},
		{
			name:    "pass-type-ids certificates view missing pass type id",
			args:    []string{"pass-type-ids", "certificates", "view"},
			wantErr: "--pass-type-id is required",
		},
		{
			name:    "pass-type-ids view missing pass type id",
			args:    []string{"pass-type-ids", "view"},
			wantErr: "--pass-type-id is required",
		},
		{
			name:    "pass-type-ids create missing identifier",
			args:    []string{"pass-type-ids", "create", "--name", "Example"},
			wantErr: "--identifier is required",
		},
		{
			name:    "pass-type-ids create missing name",
			args:    []string{"pass-type-ids", "create", "--identifier", "pass.com.example"},
			wantErr: "--name is required",
		},
		{
			name:    "pass-type-ids update missing pass type id",
			args:    []string{"pass-type-ids", "update", "--name", "Updated"},
			wantErr: "--pass-type-id is required",
		},
		{
			name:    "pass-type-ids update missing name",
			args:    []string{"pass-type-ids", "update", "--pass-type-id", "PASS_ID"},
			wantErr: "--name is required",
		},
		{
			name:    "pass-type-ids delete missing pass type id",
			args:    []string{"pass-type-ids", "delete", "--confirm"},
			wantErr: "--pass-type-id is required",
		},
		{
			name:    "pass-type-ids delete missing confirm",
			args:    []string{"pass-type-ids", "delete", "--pass-type-id", "PASS_ID"},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, nil, test.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
