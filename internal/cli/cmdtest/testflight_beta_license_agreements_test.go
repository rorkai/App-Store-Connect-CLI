package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFlightBetaLicenseAgreementsListOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements" {
			t.Fatalf("expected path /v1/betaLicenseAgreements, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[app]") != "app-1" {
			t.Fatalf("expected app filter app-1, got %q", query.Get("filter[app]"))
		}
		if query.Get("limit") != "2" {
			t.Fatalf("expected limit 2, got %q", query.Get("limit"))
		}
		body := `{"data":[{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Terms"}}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "agreements", "list", "--app", "app-1", "--limit", "2"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"agree-1"`) {
		t.Fatalf("expected agreement id in output, got %q", stdout)
	}
}

func TestTestFlightBetaLicenseAgreementsGetByIDOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements/agree-1" {
			t.Fatalf("expected path /v1/betaLicenseAgreements/agree-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[betaLicenseAgreements]") != "agreementText" {
			t.Fatalf("expected fields agreementText, got %q", req.URL.Query().Get("fields[betaLicenseAgreements]"))
		}
		body := `{"data":{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Terms"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "agreements", "view", "--id", "agree-1", "--fields", "agreementText"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"agree-1"`) {
		t.Fatalf("expected agreement id in output, got %q", stdout)
	}
}

func TestTestFlightBetaLicenseAgreementsGetByAppOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/betaLicenseAgreement" {
			t.Fatalf("expected path /v1/apps/app-1/betaLicenseAgreement, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[betaLicenseAgreements]") != "agreementText" {
			t.Fatalf("expected fields agreementText, got %q", req.URL.Query().Get("fields[betaLicenseAgreements]"))
		}
		body := `{"data":{"type":"betaLicenseAgreements","id":"agree-2","attributes":{"agreementText":"Terms"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "agreements", "view", "--app", "app-1", "--fields", "agreementText"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"agree-2"`) {
		t.Fatalf("expected agreement id in output, got %q", stdout)
	}
}

func TestTestFlightBetaLicenseAgreementsUpdateOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements/agree-1" {
			t.Fatalf("expected path /v1/betaLicenseAgreements/agree-1, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(payload), `"agreementText":"Updated terms"`) {
			t.Fatalf("expected agreement text in body, got %s", string(payload))
		}
		body := `{"data":{"type":"betaLicenseAgreements","id":"agree-1","attributes":{"agreementText":"Updated terms"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "agreements", "edit", "--id", "agree-1", "--agreement-text", "Updated terms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"agree-1"`) {
		t.Fatalf("expected agreement id in output, got %q", stdout)
	}
}

func TestTestFlightBetaLicenseAgreementsUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "view missing id and app",
			args:    []string{"testflight", "agreements", "view"},
			wantErr: "Error: --id or --app is required",
		},
		{
			name:    "view id and app both set",
			args:    []string{"testflight", "agreements", "view", "--id", "AGREEMENT_ID", "--app", "APP_ID"},
			wantErr: "Error: --id and --app are mutually exclusive",
		},
		{
			name:    "view app with include",
			args:    []string{"testflight", "agreements", "view", "--app", "APP_ID", "--include", "app"},
			wantErr: "Error: --app-fields and --include are only valid with --id",
		},
		{
			name:    "edit missing id",
			args:    []string{"testflight", "agreements", "edit", "--agreement-text", "Updated"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "edit missing agreement text",
			args:    []string{"testflight", "agreements", "edit", "--id", "AGREEMENT_ID"},
			wantErr: "Error: --agreement-text is required",
		},
	})
}
