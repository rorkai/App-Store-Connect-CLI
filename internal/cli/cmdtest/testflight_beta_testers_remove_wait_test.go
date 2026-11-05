package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func setupBetaTesterRemoveWaitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func betaTesterRemoveWaitTransport(t *testing.T, pollResponses func(pollCount int) (int, string)) {
	t.Helper()

	pollCount := 0
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaTesters":
			body := `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com","state":"INSTALLED"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/betaTesters/tester-1":
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaTesters/tester-1":
			pollCount++
			status, body := pollResponses(pollCount)
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})
}

func TestBetaTestersRemoveWaitCompletesWhenTesterIsGone(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "not found", status: http.StatusNotFound, body: `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`},
		{name: "revoked", status: http.StatusOK, body: `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"state":"REVOKED"}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupBetaTesterRemoveWaitEnv(t)
			betaTesterRemoveWaitTransport(t, func(int) (int, string) {
				return tt.status, tt.body
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse([]string{
					"testflight", "testers", "remove",
					"--app", "app-1", "--email", "tester@example.com", "--confirm",
					"--wait", "--poll-interval", "10ms", "--timeout", "5s",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !strings.Contains(stdout, `"deleted":true`) {
				t.Fatalf("expected delete receipt, got %q", stdout)
			}
		})
	}
}

func TestBetaTestersRemoveWaitTimesOutButPrintsReceipt(t *testing.T) {
	setupBetaTesterRemoveWaitEnv(t)
	betaTesterRemoveWaitTransport(t, func(int) (int, string) {
		return http.StatusOK, `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"state":"INSTALLED"}}}`
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "testers", "remove",
			"--app", "app-1", "--email", "tester@example.com", "--confirm",
			"--wait", "--poll-interval", "20ms", "--timeout", "150ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "not yet visible") {
		t.Fatalf("expected visibility timeout error, got %v", runErr)
	}
	if !strings.Contains(stdout, `"deleted":true`) {
		t.Fatalf("timeout must still print the delete receipt, got %q", stdout)
	}
}

func TestBetaTestersRemoveWaitValidatesDurations(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantError string
	}{
		{
			name:      "zero poll interval",
			args:      []string{"--wait", "--poll-interval", "0s"},
			wantError: "--poll-interval must be greater than 0",
		},
		{
			name:      "zero timeout",
			args:      []string{"--wait", "--timeout", "0s"},
			wantError: "--timeout must be greater than 0",
		},
		{
			name:      "poll interval without wait",
			args:      []string{"--poll-interval", "10s"},
			wantError: "--poll-interval requires --wait",
		},
		{
			name:      "timeout without wait",
			args:      []string{"--timeout", "1m"},
			wantError: "--timeout requires --wait",
		},
		{
			name:      "both wait flags without wait",
			args:      []string{"--poll-interval", "10s", "--timeout", "1m"},
			wantError: "--poll-interval and --timeout require --wait",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBetaTesterRemoveWaitEnv(t)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{
				"testflight", "testers", "remove",
				"--app", "app-1", "--email", "tester@example.com", "--confirm",
			}, tc.args...)
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = root.Run(context.Background())
			})
			if runErr == nil {
				t.Fatal("expected usage error")
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if runErr.Error() != tc.wantError {
				t.Fatalf("error = %q, want %q", runErr.Error(), tc.wantError)
			}
			wantDiagnostic := "Error: " + tc.wantError + "\n"
			if !strings.HasPrefix(stderr, wantDiagnostic) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, wantDiagnostic)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty output", stdout)
			}
		})
	}
}

func TestBetaTestersExportForwardsNameFilters(t *testing.T) {
	setupBetaTesterRemoveWaitEnv(t)

	var gotQuery string
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.RawQuery
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	csvPath := filepath.Join(t.TempDir(), "testers.csv")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "testers", "export",
			"--app", "app-1", "--first-name", "Ada", "--last-name", "Lovelace",
			"--output", csvPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	for _, want := range []string{"filter%5BfirstName%5D=Ada", "filter%5BlastName%5D=Lovelace"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("expected %s in query, got %q", want, gotQuery)
		}
	}
}
