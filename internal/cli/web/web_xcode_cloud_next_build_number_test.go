package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebNextBuildNumberHierarchy(t *testing.T) {
	d, _, _ := newWebTestDeps()
	settings := findSub(newWebXcodeCloudCommand(d), "settings")
	if settings == nil {
		t.Fatal("expected settings subcommand")
	}
	group := findSub(settings, "next-build-number")
	if group == nil {
		t.Fatal("expected next-build-number subcommand")
	}
	for _, name := range []string{"show", "set"} {
		if command := findSub(group, name); command == nil || command.UsageFunc == nil {
			t.Fatalf("expected %q subcommand with UsageFunc", name)
		}
	}
	if findSub(group, "show").FlagSet.Lookup("value") != nil || findSub(group, "show").FlagSet.Lookup("confirm") != nil {
		t.Fatal("show must not accept mutation-only flags")
	}
}

func TestWebNextBuildNumberShowOmitsSensitiveURL(t *testing.T) {
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/ci/api/teams/team-uuid/products/product-uuid/next-build-number" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":102,"testflight_url":"https://example.invalid/sensitive?token=secret"}`), nil
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := webNextBuildNumberShow(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; output=%q", err, stdout)
	}
	if result["productId"] != "product-uuid" || result["nextBuildNumber"] != float64(102) {
		t.Fatalf("unexpected output: %#v", result)
	}
	if strings.Contains(stdout, "testflight") || strings.Contains(stdout, "token=secret") {
		t.Fatalf("sensitive URL leaked in output: %q", stdout)
	}
}

func TestWebNextBuildNumberSetReadsWritesAndVerifies(t *testing.T) {
	var calls []string
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method)
		switch req.Method {
		case http.MethodGet:
			getCount++
			value := "101"
			if getCount == 2 {
				value = "102"
			}
			return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":`+value+`,"testflight_url":"https://example.invalid/?token=secret"}`), nil
		case http.MethodPut:
			if req.URL.Path != "/ci/api/teams/team-uuid/products/product-uuid/next-build-number" {
				t.Fatalf("PUT path = %q", req.URL.Path)
			}
			if got := req.URL.Query().Get("next_build_number"); got != "102" {
				t.Fatalf("next_build_number = %q", got)
			}
			if req.Body != nil {
				body, err := io.ReadAll(req.Body)
				if err != nil || len(body) != 0 {
					t.Fatalf("PUT body = %q, err = %v", body, err)
				}
			}
			return nextBuildNumberResponse(req, http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := webNextBuildNumberSet(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if got := strings.Join(calls, ","); got != "GET,PUT,GET" {
		t.Fatalf("call order = %q", got)
	}
	for _, want := range []string{`"productId":"product-uuid"`, `"previousNextBuildNumber":101`, `"nextBuildNumber":102`, `"updated":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("output missing %s: %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "token=secret") {
		t.Fatalf("sensitive URL leaked in output: %q", stdout)
	}
}

func TestWebNextBuildNumberSetRejectsNonIncreasingValue(t *testing.T) {
	requests := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet {
			t.Fatalf("unexpected mutation: %s", req.Method)
		}
		return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":102}`), nil
	})
	d, _, _ := newWebTestDeps()
	cmd := webNextBuildNumberSet(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "greater than current value 102") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one pre-read", requests)
	}
}

func TestWebNextBuildNumberSetValidatesBeforeSession(t *testing.T) {
	original := resolveSessionFn
	called := false
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing product", args: []string{"--value", "1", "--confirm"}, want: "--product-id is required"},
		{name: "invalid value", args: []string{"--product-id", "product", "--value", "0", "--confirm"}, want: "--value must be greater than 0"},
		{name: "missing confirm", args: []string{"--product-id", "product", "--value", "1"}, want: "--confirm is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := webNextBuildNumberSet(d)
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			var execErr error
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				execErr = cmd.Exec(context.Background(), nil)
			})
			if execErr == nil || (!strings.Contains(execErr.Error(), tt.want) && !strings.Contains(stderr, tt.want)) {
				t.Fatalf("error = %v, stderr = %q, want containing %q", execErr, stderr, tt.want)
			}
		})
	}
	if called {
		t.Fatal("session resolution must not run for invalid input")
	}
}

func TestWebNextBuildNumberSetReportsVerificationMismatch(t *testing.T) {
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			return nextBuildNumberResponse(req, http.StatusNoContent, ""), nil
		}
		getCount++
		return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":101}`), nil
	})
	d, _, _ := newWebTestDeps()
	cmd := webNextBuildNumberSet(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "verification failed: got 101, expected 102") {
		t.Fatalf("error = %v", err)
	}
	if getCount != 2 {
		t.Fatalf("GET count = %d, want 2", getCount)
	}
}

func TestWebNextBuildNumberSetReconcilesAmbiguousWrite(t *testing.T) {
	var calls []string
	var firstReadContext context.Context
	getCount := 0
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method)
		switch req.Method {
		case http.MethodGet:
			getCount++
			if getCount == 1 {
				firstReadContext = req.Context()
				return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":101}`), nil
			}
			if req.Context() == firstReadContext {
				t.Fatal("reconciliation must use a fresh request context")
			}
			return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":102}`), nil
		case http.MethodPut:
			return nil, io.ErrUnexpectedEOF
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := webNextBuildNumberSet(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if got := strings.Join(calls, ","); got != "GET,PUT,GET" {
		t.Fatalf("call order = %q", got)
	}
	if !strings.Contains(stdout, `"nextBuildNumber":102`) || !strings.Contains(stdout, `"updated":true`) {
		t.Fatalf("reconciled output = %q", stdout)
	}
}

func TestWebNextBuildNumberSetReportsAmbiguousWriteOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		recheckBody string
		recheckErr  error
		want        string
	}{
		{
			name:        "unchanged state is retry safe",
			recheckBody: `{"next_build_number":101}`,
			want:        "remote still reports 101; the write was not applied",
		},
		{
			name:       "failed reconciliation remains unverified",
			recheckErr: io.ErrUnexpectedEOF,
			want:       "may have succeeded but reconciliation failed",
		},
		{
			name:        "divergent state remains unverified",
			recheckBody: `{"next_build_number":103}`,
			want:        "reports 103, which is neither the previous value 101 nor the requested value 102",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getCount := 0
			stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
				switch req.Method {
				case http.MethodGet:
					getCount++
					if getCount == 1 {
						return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":101}`), nil
					}
					if tt.recheckErr != nil {
						return nil, tt.recheckErr
					}
					return nextBuildNumberResponse(req, http.StatusOK, tt.recheckBody), nil
				case http.MethodPut:
					return nil, io.ErrUnexpectedEOF
				default:
					t.Fatalf("unexpected method %s", req.Method)
					return nil, nil
				}
			})

			d, _, _ := newWebTestDeps()
			cmd := webNextBuildNumberSet(d)
			if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm"}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestWebNextBuildNumberSetReportsRequiredRole(t *testing.T) {
	stubNextBuildNumberSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return nextBuildNumberResponse(req, http.StatusOK, `{"next_build_number":101}`), nil
		}
		return nextBuildNumberResponse(req, http.StatusForbidden, `{"errors":[{"status":"403"}]}`), nil
	})

	d, _, _ := newWebTestDeps()
	cmd := webNextBuildNumberSet(d)
	if err := cmd.FlagSet.Parse([]string{"--product-id", "product-uuid", "--value", "102", "--confirm"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "requires the App Store Connect Admin or App Manager role") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "run 'asc web auth login'") {
		t.Fatalf("role failure must not recommend re-authentication: %v", err)
	}
}

func stubNextBuildNumberSession(t *testing.T, transport roundTripFunc) {
	t.Helper()
	original := resolveSessionFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client:           &http.Client{Transport: transport},
		}, "cache", nil
	}
	t.Cleanup(func() { resolveSessionFn = original })
}

func nextBuildNumberResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
