package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightDistributionEditRejectsRemovedExternalTestingFlag(t *testing.T) {
	clientFactoryCalled := false
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not be called")
	}))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "enable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=true"},
		},
		{
			name: "disable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=false"},
		},
		{
			name: "mixed with supported update",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify", "--external-testing", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled = false
			assertRemovedFlagGuidance(t, test.args, "--external-testing")
			if clientFactoryCalled {
				t.Fatal("expected removed flag to fail before client creation or HTTP")
			}
		})
	}
}

func TestTestFlightRecruitmentSetUpdatesExisting(t *testing.T) {
	setupAuth(t)
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
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups/group-1/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaGroups/group-1/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaRecruitmentCriteria/criteria-1" {
				t.Fatalf("expected path /v1/betaRecruitmentCriteria/criteria-1, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"deviceFamilyOsVersionFilters"`) {
				t.Fatalf("expected filters in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
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
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-1", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-1"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetCreatesWhenMissing(t *testing.T) {
	setupAuth(t)
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
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups/group-2/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaGroups/group-2/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			body := `{"errors":[{"code":"NOT_FOUND","title":"Not Found"}]}`
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-2"`) {
				t.Fatalf("expected group id in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-2","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
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
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-2", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-2"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetReturnsErrorOnFetchFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"errors":[{"code":"FORBIDDEN","title":"Forbidden"}]}`
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-3", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected non-help error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(runErr.Error(), "failed to fetch existing criteria") {
		t.Fatalf("expected fetch error, got %v (stderr: %q)", runErr, stderr)
	}
}

func TestTestFlightDistributionAndRecruitmentSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "distribution view",
			args:       []string{"testflight", "distribution", "view", "--build-id", "build-1", "--limit", "1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/buildBetaDetails",
			wantQuery:  url.Values{"filter[build]": {"build-1"}, "limit": {"1"}},
			respBody:   `{"data":[{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "distribution build view",
			args:       []string{"testflight", "distribution", "build", "view", "--id", "detail-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/buildBetaDetails/detail-1/build",
			respBody:   `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0"}}}`,
			wantStdout: []string{`"id":"build-1"`},
		},
		{
			name:       "distribution edit",
			args:       []string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify"},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/buildBetaDetails/detail-1",
			wantBody:   `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`,
			respBody:   `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`,
			wantStdout: []string{`"id":"detail-1"`},
		},
		{
			name:       "recruitment options",
			args:       []string{"testflight", "recruitment", "options", "--fields", "deviceFamilyOsVersions", "--limit", "1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaRecruitmentCriterionOptions",
			wantQuery:  url.Values{"fields[betaRecruitmentCriterionOptions]": {"deviceFamilyOsVersions"}, "limit": {"1"}},
			respBody:   `{"data":[{"type":"betaRecruitmentCriterionOptions","id":"opt-1","attributes":{"deviceFamilyOsVersions":[{"deviceFamily":"IPHONE","osVersions":["26"]}]}}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"opt-1"`},
		},
		{
			name:       "recruitment delete",
			args:       []string{"testflight", "recruitment", "delete", "--id", "criteria-1", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaRecruitmentCriteria/criteria-1",
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"id":"criteria-1"`, `"deleted":true`},
		},
	})
}
