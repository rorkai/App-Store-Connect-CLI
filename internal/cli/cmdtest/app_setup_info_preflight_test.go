package cmdtest

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const appSetupInfoLocalizationsURI = "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=en-US&limit=200"

var appSetupInfoOwnershipStep = appSetupInfoHTTPStep{
	method:       http.MethodGet,
	uri:          "/v1/appInfos/info-1?include=app",
	responseBody: `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}},"included":[{"type":"apps","id":"app-1"}]}`,
}

type appSetupInfoHTTPStep struct {
	method       string
	uri          string
	requestBody  string
	responseBody string
	status       int
}

func TestAppSetupInfoSetRejectsOverLimitLocalizationBeforeClient(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		field     string
		limit     int
		fieldFlag string
	}{
		{field: "name", limit: validation.LimitName, fieldFlag: "--name"},
		{field: "subtitle", limit: validation.LimitSubtitle, fieldFlag: "--subtitle"},
	}

	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			clientFactoryCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, errors.New("client should not be created")
			}))

			stdout, stderr, runErr := runAppSetupInfoSet(
				t,
				"--app", "app-1",
				"--bundle-id", "com.example.changed",
				"--locale", "en-US",
				test.fieldFlag, strings.Repeat("x", test.limit+1),
			)
			wantError := fmt.Sprintf("--%s exceeds %d characters", test.field, test.limit)
			if runErr == nil || !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), wantError) {
				t.Errorf("expected %q usage error, got %v", wantError, runErr)
			}
			if stdout != "" || !strings.Contains(stderr, "Error: "+wantError) {
				t.Errorf("stdout = %q, stderr = %q, want only %q diagnostic", stdout, stderr, "Error: "+wantError)
			}
			if clientFactoryCalls != 0 {
				t.Errorf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}

func TestAppSetupInfoSetPlanningFailuresDoNotMutate(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		steps     []appSetupInfoHTTPStep
		wantError string
		usage     bool
	}{
		{
			name: "ambiguous app info",
			args: []string{"--app", "app-1", "--bundle-id", "com.example.changed", "--locale", "en-US", "--name", "Example App"},
			steps: []appSetupInfoHTTPStep{{
				method: http.MethodGet,
				uri:    "/v1/apps/app-1/appInfos",
				responseBody: `{"data":[
					{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}},
					{"type":"appInfos","id":"info-2","attributes":{"state":"READY_FOR_SALE"}}
				]}`,
			}},
			wantError: "multiple app infos found",
		},
		{
			name:      "create without name",
			args:      []string{"--app", "app-1", "--app-info", "info-1", "--bundle-id", "com.example.changed", "--locale", "en-US", "--subtitle", "Subtitle"},
			steps:     []appSetupInfoHTTPStep{appSetupInfoOwnershipStep, {method: http.MethodGet, uri: appSetupInfoLocalizationsURI, responseBody: `{"data":[]}`}},
			wantError: "--name is required when creating an app info localization",
			usage:     true,
		},
		{
			name: "ambiguous localization",
			args: []string{"--app", "app-1", "--app-info", "info-1", "--bundle-id", "com.example.changed", "--locale", "en-US", "--name", "Example App"},
			steps: []appSetupInfoHTTPStep{{
				method:       appSetupInfoOwnershipStep.method,
				uri:          appSetupInfoOwnershipStep.uri,
				responseBody: appSetupInfoOwnershipStep.responseBody,
			}, {
				method:       http.MethodGet,
				uri:          appSetupInfoLocalizationsURI,
				responseBody: `{"data":[{"type":"appInfoLocalizations","id":"loc-1"},{"type":"appInfoLocalizations","id":"loc-2"}]}`,
			}},
			wantError: `multiple app info localizations found for locale "en-US"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, mutationCount, runErr := runAppSetupInfoSetWithServer(t, test.steps, test.args...)
			if runErr == nil || !strings.Contains(runErr.Error(), test.wantError) {
				t.Fatalf("expected %q error, got %v", test.wantError, runErr)
			}
			if test.usage && !errors.Is(runErr, flag.ErrHelp) {
				t.Errorf("error = %v, want usage classification", runErr)
			}
			if stdout != "" || mutationCount != 0 {
				t.Errorf("stdout = %q, mutation count = %d; want empty output and zero mutations", stdout, mutationCount)
			}
			if test.usage && !strings.Contains(stderr, "Error: "+test.wantError) {
				t.Errorf("stderr = %q, want %q", stderr, "Error: "+test.wantError)
			}
		})
	}
}

func TestAppSetupInfoSetPlansExplicitLocalizationTargetBeforeWrites(t *testing.T) {
	appPatch := appSetupInfoHTTPStep{
		method:       http.MethodPatch,
		uri:          "/v1/apps/app-1",
		requestBody:  `{"data":{"type":"apps","id":"app-1","attributes":{"bundleId":"com.example.changed"}}}`,
		responseBody: `{"data":{"type":"apps","id":"app-1","attributes":{"bundleId":"com.example.changed"}}}`,
	}
	tests := []struct {
		name              string
		args              []string
		localizations     string
		localizationWrite appSetupInfoHTTPStep
	}{
		{
			name:          "create",
			args:          []string{"--name", "Example App", "--subtitle", "Subtitle"},
			localizations: `{"data":[]}`,
			localizationWrite: appSetupInfoHTTPStep{
				method:      http.MethodPost,
				uri:         "/v1/appInfoLocalizations",
				status:      http.StatusCreated,
				requestBody: `{"data":{"type":"appInfoLocalizations","attributes":{"locale":"en-US","name":"Example App","subtitle":"Subtitle"},"relationships":{"appInfo":{"data":{"type":"appInfos","id":"info-1"}}}}}`,
				responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-created","attributes":{
					"locale":"en-US","name":"Example App","subtitle":"Subtitle"}}}`,
			},
		},
		{
			name:          "update without name",
			args:          []string{"--subtitle", "Updated Subtitle", "--privacy-policy-url", "https://example.com/privacy"},
			localizations: `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Existing App"}}]}`,
			localizationWrite: appSetupInfoHTTPStep{
				method:      http.MethodPatch,
				uri:         "/v1/appInfoLocalizations/loc-1",
				requestBody: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"subtitle":"Updated Subtitle","privacyPolicyUrl":"https://example.com/privacy"}}}`,
				responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{
					"locale":"en-US","name":"Existing App","subtitle":"Updated Subtitle","privacyPolicyUrl":"https://example.com/privacy"}}}`,
			},
		},
		{
			name:          "update privacy policy without name",
			args:          []string{"--privacy-policy-url", "https://example.com/privacy"},
			localizations: `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Existing App"}}]}`,
			localizationWrite: appSetupInfoHTTPStep{
				method:       http.MethodPatch,
				uri:          "/v1/appInfoLocalizations/loc-1",
				requestBody:  `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"privacyPolicyUrl":"https://example.com/privacy"}}}`,
				responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Existing App","privacyPolicyUrl":"https://example.com/privacy"}}}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			steps := []appSetupInfoHTTPStep{
				appSetupInfoOwnershipStep,
				{method: http.MethodGet, uri: appSetupInfoLocalizationsURI, responseBody: test.localizations},
				appPatch,
				test.localizationWrite,
			}
			args := []string{"--app", "app-1", "--app-info", "info-1", "--bundle-id", "com.example.changed", "--locale", "en-US"}
			args = append(args, test.args...)
			stdout, stderr, mutationCount, runErr := runAppSetupInfoSetWithServer(t, steps, args...)
			if runErr != nil {
				t.Fatalf("run error: %v (stderr=%q)", runErr, stderr)
			}
			if !strings.Contains(stdout, `"appId":"app-1"`) || mutationCount != 2 {
				t.Errorf("stdout = %q, mutation count = %d; want app result and two ordered writes", stdout, mutationCount)
			}
		})
	}
}

func runAppSetupInfoSetWithServer(t *testing.T, steps []appSetupInfoHTTPStep, args ...string) (string, string, int, error) {
	t.Helper()
	setupAppSetupInfoPreflightAuth(t)

	var mu sync.Mutex
	requestCount := 0
	mutationCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		stepIndex := requestCount
		requestCount++
		if appSetupInfoMutation(req.Method) {
			mutationCount++
		}
		mu.Unlock()

		if stepIndex >= len(steps) {
			t.Errorf("unexpected request %d: %s %s", stepIndex+1, req.Method, req.URL.RequestURI())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		step := steps[stepIndex]
		if req.Method != step.method || req.URL.RequestURI() != step.uri {
			t.Errorf("request %d = %s %s, want %s %s", stepIndex+1, req.Method, req.URL.RequestURI(), step.method, step.uri)
		}
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("request %d is missing bearer authorization", stepIndex+1)
		}
		if step.requestBody != "" {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read request %d body: %v", stepIndex+1, err)
			} else if strings.TrimSpace(string(body)) != step.requestBody {
				t.Errorf("request %d body = %s, want %s", stepIndex+1, body, step.requestBody)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		status := step.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, step.responseBody)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return serverTransport.RoundTrip(cloned)
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	stdout, stderr, runErr := runAppSetupInfoSet(t, args...)
	mu.Lock()
	gotRequestCount := requestCount
	gotMutationCount := mutationCount
	mu.Unlock()
	if gotRequestCount != len(steps) {
		t.Errorf("request count = %d, want %d", gotRequestCount, len(steps))
	}
	return stdout, stderr, gotMutationCount, runErr
}

func setupAppSetupInfoPreflightAuth(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
}

func runAppSetupInfoSet(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		parseArgs := append([]string{"app-setup", "info", "set"}, args...)
		if err := root.Parse(parseArgs); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func appSetupInfoMutation(method string) bool {
	switch method {
	case http.MethodPatch, http.MethodPost, http.MethodDelete:
		return true
	default:
		return false
	}
}
