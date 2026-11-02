package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBetaGroupsCreateInternalSetsAttributeOnCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups" {
				t.Fatalf("expected path /v1/betaGroups, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"name":"Internal Testers"`) {
				t.Fatalf("expected group name in body, got %s", string(payload))
			}
			if !strings.Contains(string(payload), `"isInternalGroup":true`) {
				t.Fatalf("expected isInternalGroup=true in body, got %s", string(payload))
			}
			for _, field := range []string{"publicLinkEnabled", "publicLinkLimitEnabled", "publicLinkLimit"} {
				if strings.Contains(string(payload), field) {
					t.Fatalf("did not expect internal group create payload to include %s, got %s", field, string(payload))
				}
			}
			if !strings.Contains(string(payload), `"type":"apps"`) || !strings.Contains(string(payload), `"id":"app-1"`) {
				t.Fatalf("expected app relationship in body, got %s", string(payload))
			}

			body := `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Internal Testers","isInternalGroup":true}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Internal Testers", "--internal"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"isInternalGroup":true`) {
		t.Fatalf("expected isInternalGroup in output, got %q", stdout)
	}
}

func TestBetaGroupsCreateWithoutInternalMakesOneCall(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups" {
				t.Fatalf("expected path /v1/betaGroups, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"betaGroups","id":"bg-2","attributes":{"name":"Beta"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"bg-2"`) {
		t.Fatalf("expected beta group id in output, got %q", stdout)
	}
}

func TestBetaGroupsCreateSendsDistributionControls(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/betaGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		for _, attribute := range []string{
			`"hasAccessToAllBuilds":true`,
			`"publicLinkEnabled":true`,
			`"publicLinkLimitEnabled":true`,
			`"publicLinkLimit":250`,
			`"feedbackEnabled":true`,
		} {
			if !strings.Contains(body, attribute) {
				t.Fatalf("expected %s in body, got %s", attribute, body)
			}
		}

		response := `{"data":{"type":"betaGroups","id":"bg-3","attributes":{"name":"Public Preview","isInternalGroup":false,"hasAccessToAllBuilds":true,"feedbackEnabled":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Public Preview",
			"--access-all-builds",
			"--public-link-enabled",
			"--public-link-limit-enabled",
			"--public-link-limit", "250",
			"--feedback-enabled",
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
	if callCount != 1 {
		t.Fatalf("expected one request, got %d", callCount)
	}
}

func TestBetaGroupsCreateSendsExplicitFalseControls(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		for _, attribute := range []string{
			`"isInternalGroup":false`,
			`"hasAccessToAllBuilds":false`,
			`"publicLinkEnabled":false`,
			`"publicLinkLimitEnabled":false`,
			`"feedbackEnabled":false`,
		} {
			if !strings.Contains(body, attribute) {
				t.Errorf("expected %s in body, got %s", attribute, body)
			}
		}

		response := `{"data":{"type":"betaGroups","id":"bg-false","attributes":{"name":"Explicit Defaults"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Explicit Defaults",
			"--internal=false",
			"--access-all-builds=false",
			"--public-link-enabled=false",
			"--public-link-limit-enabled=false",
			"--feedback-enabled=false",
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
}

func TestBetaGroupsCreateRejectsInternalPublicLinkControlsBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return nil, errors.New("unexpected request")
	})

	tests := []struct {
		name string
		args []string
	}{
		{name: "enabled", args: []string{"--public-link-enabled"}},
		{name: "limit enabled", args: []string{"--public-link-limit-enabled"}},
		{name: "limit", args: []string{"--public-link-limit", "250"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := []string{"testflight", "groups", "create", "--app", "app-1", "--name", "Internal", "--internal"}
			args = append(args, tt.args...)
			var runErr error
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			wantErr := shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--internal")
			if runErr.Error() != wantErr.Error() {
				t.Fatalf("error = %q, want %q", runErr, wantErr)
			}
			if !strings.Contains(stderr, "--internal cannot be combined with public link controls") {
				t.Fatalf("expected conflict diagnostic, got %q", stderr)
			}
			if callCount != 0 {
				t.Fatalf("expected no requests, got %d", callCount)
			}
		})
	}
}

func TestBetaGroupsCreateRejectsInvalidPublicLinkLimitBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "create",
			"--app", "app-1",
			"--name", "Beta",
			"--public-link-limit", "10001",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	wantErr := shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--public-link-limit")
	if runErr.Error() != wantErr.Error() {
		t.Fatalf("error = %q, want %q", runErr, wantErr)
	}
	if !strings.Contains(stderr, "--public-link-limit must be between 1 and 10000") {
		t.Fatalf("expected limit diagnostic, got %q", stderr)
	}
}

func createRequestRelationships(t *testing.T, req *http.Request) string {
	t.Helper()

	var payload struct {
		Data struct {
			Relationships json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body error: %v", err)
	}
	return string(payload.Data.Relationships)
}

func TestBetaGroupsCreateSendsRelationshipsInOneRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const app = `"app":{"data":{"type":"apps","id":"app-1"}}`
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "omitted flags send only the app",
			want: `{` + app + `}`,
		},
		{
			name: "builds",
			args: []string{"--build-id", "build-1, build-2,build-1"},
			want: `{` + app + `,"builds":{"data":[{"type":"builds","id":"build-1"},{"type":"builds","id":"build-2"}]}}`,
		},
		{
			name: "testers",
			args: []string{"--tester", "tester-1,tester-2"},
			want: `{` + app + `,"betaTesters":{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}}`,
		},
		{
			name: "builds and testers",
			args: []string{"--build-id", "build-1", "--tester", "tester-1"},
			want: `{` + app + `,"builds":{"data":[{"type":"builds","id":"build-1"}]},"betaTesters":{"data":[{"type":"betaTesters","id":"tester-1"}]}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			callCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if req.Method != http.MethodPost || req.URL.Path != "/v1/betaGroups" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				if got := createRequestRelationships(t, req); got != test.want {
					t.Fatalf("relationships = %s, want %s", got, test.want)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Beta"}}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			args := append([]string{"testflight", "groups", "create", "--app", "app-1", "--name", "Beta"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			if callCount != 1 {
				t.Fatalf("expected one request, got %d", callCount)
			}
		})
	}
}
