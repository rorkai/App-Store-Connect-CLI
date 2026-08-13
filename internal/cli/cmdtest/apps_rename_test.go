package cmdtest

import (
	"context"
	"encoding/json"
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
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestAppsRenameValidatesRequiredFlagsBeforeClient(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "missing app", args: []string{"--locale", "en-US", "--name", "New Name"}, wantStderr: "Error: --app is required (or set ASC_APP_ID)\n"},
		{name: "blank app", args: []string{"--app", "   ", "--locale", "en-US", "--name", "New Name"}, wantStderr: "Error: --app is required (or set ASC_APP_ID)\n"},
		{name: "missing locale", args: []string{"--app", "app-1", "--name", "New Name"}, wantStderr: "Error: --locale is required\n"},
		{name: "missing name", args: []string{"--app", "app-1", "--locale", "en-US"}, wantStderr: "Error: --name is required\n"},
		{name: "invalid locale", args: []string{"--app", "app-1", "--locale", "not_a_locale", "--name", "New Name"}, wantStderr: "Error: invalid locale \"not_a_locale\": must match pattern like en or en-US\n"},
		{name: "name too long", args: []string{"--app", "app-1", "--locale", "en-US", "--name", strings.Repeat("x", validation.LimitName+1)}, wantStderr: fmt.Sprintf("Error: --name exceeds %d characters\n", validation.LimitName)},
		{name: "Unicode name too long", args: []string{"--app", "app-1", "--locale", "ja", "--name", strings.Repeat("名", validation.LimitName+1)}, wantStderr: fmt.Sprintf("Error: --name exceeds %d characters\n", validation.LimitName)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, errors.New("client should not be created")
			}))

			stdout, stderr, runErr := runAppsRename(t, test.args...)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantStderr)
			}
			if strings.Count(stderr, strings.TrimSpace(test.wantStderr)) != 1 {
				t.Fatalf("stderr contains duplicate diagnostic: %q", stderr)
			}
			if clientFactoryCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}

func TestAppsRenameUpdatesExistingLocalization(t *testing.T) {
	steps := []appsRenameHTTPStep{
		appInfoOwnershipStep(),
		{
			method:       http.MethodGet,
			uri:          "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=en-US&limit=200",
			responseBody: `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Old Name"}}]}`,
		},
		{
			method:       http.MethodPatch,
			uri:          "/v1/appInfoLocalizations/loc-1",
			requestBody:  `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"name":"New Name"}}}`,
			responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"New Name"}}}`,
		},
	}

	stdout, stderr, runErr := runAppsRenameWithServer(
		t, steps,
		"--name", "New Name",
		"--app-info", "info-1",
		"--locale", "en-US",
		"--app", "app-1",
		"--output", "json",
	)
	if runErr != nil {
		t.Fatalf("run error: %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var got asc.AppRenameResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if got.AppID != "app-1" || got.AppInfoID != "info-1" || got.Locale != "en-US" || got.Name != "New Name" || got.Action != "update" || got.LocalizationID != "loc-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestAppsRenameCreatesMissingLocalization(t *testing.T) {
	steps := []appsRenameHTTPStep{
		appInfoOwnershipStep(),
		{
			method:       http.MethodGet,
			uri:          "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=fr-FR&limit=200",
			responseBody: `{"data":[]}`,
		},
		{
			method:       http.MethodPost,
			uri:          "/v1/appInfoLocalizations",
			status:       http.StatusCreated,
			requestBody:  `{"data":{"type":"appInfoLocalizations","attributes":{"locale":"fr-FR","name":"Nouveau nom"},"relationships":{"appInfo":{"data":{"type":"appInfos","id":"info-1"}}}}}`,
			responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-created","attributes":{"locale":"fr-FR","name":"Nouveau nom"}}}`,
		},
	}

	stdout, stderr, runErr := runAppsRenameWithServer(
		t, steps,
		"--app", "app-1",
		"--locale", "fr-FR",
		"--name", "Nouveau nom",
		"--app-info", "info-1",
		"--output", "table",
	)
	if runErr != nil {
		t.Fatalf("run error: %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, value := range []string{"App ID", "App Info ID", "Locale", "Name", "Action", "Localization ID", "app-1", "info-1", "fr-FR", "Nouveau nom", "create", "loc-created"} {
		if !strings.Contains(stdout, value) {
			t.Fatalf("table output missing %q: %s", value, stdout)
		}
	}
}

func TestAppsRenameRendersMarkdown(t *testing.T) {
	steps := []appsRenameHTTPStep{
		appInfoOwnershipStep(),
		{
			method:       http.MethodGet,
			uri:          "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=en-US&limit=200",
			responseBody: `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Old Name"}}]}`,
		},
		{
			method:       http.MethodPatch,
			uri:          "/v1/appInfoLocalizations/loc-1",
			requestBody:  `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"name":"New Name"}}}`,
			responseBody: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"New Name"}}}`,
		},
	}

	stdout, stderr, runErr := runAppsRenameWithServer(
		t, steps,
		"--app", "app-1", "--app-info", "info-1", "--locale", "en-US", "--name", "New Name", "--output", "markdown",
	)
	if runErr != nil {
		t.Fatalf("run error: %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, value := range []string{"| App ID |", "| app-1", "New Name", "update", "loc-1"} {
		if !strings.Contains(stdout, value) {
			t.Fatalf("markdown output missing %q: %s", value, stdout)
		}
	}
}

func TestAppsRenameReturnsAPIError(t *testing.T) {
	steps := []appsRenameHTTPStep{
		appInfoOwnershipStep(),
		{
			method:       http.MethodGet,
			uri:          "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=en-US&limit=200",
			status:       http.StatusUnprocessableEntity,
			responseBody: `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid Attribute","detail":"The app info cannot be edited."}]}`,
		},
	}

	stdout, _, runErr := runAppsRenameWithServer(
		t, steps,
		"--app", "app-1", "--app-info", "info-1", "--locale", "en-US", "--name", "New Name",
	)
	if runErr == nil || !strings.Contains(runErr.Error(), "apps rename") || !strings.Contains(runErr.Error(), "The app info cannot be edited") {
		t.Fatalf("run error = %v, want contextual API error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestAppsRenameRejectsAppInfoOwnedByAnotherAppBeforeMutation(t *testing.T) {
	steps := []appsRenameHTTPStep{{
		method:       http.MethodGet,
		uri:          "/v1/appInfos/info-foreign?include=app",
		responseBody: `{"data":{"type":"appInfos","id":"info-foreign","relationships":{"app":{"data":{"type":"apps","id":"app-other"}}}},"included":[{"type":"apps","id":"app-other"}]}`,
	}}

	stdout, _, runErr := runAppsRenameWithServer(
		t, steps,
		"--app", "app-1", "--app-info", "info-foreign", "--locale", "en-US", "--name", "New Name",
	)
	if runErr == nil || !strings.Contains(runErr.Error(), `app info "info-foreign" belongs to app "app-other", not "app-1"`) {
		t.Fatalf("run error = %v, want app ownership error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestAppsRenameRejectsAmbiguousOrEmptyLocalizationIDsBeforeMutation(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantError string
	}{
		{
			name: "multiple localizations",
			response: `{"data":[
				{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},
				{"type":"appInfoLocalizations","id":"loc-2","attributes":{"locale":"en-US"}}
			]}`,
			wantError: `multiple app info localizations found for locale "en-US"`,
		},
		{
			name:      "empty localization id",
			response:  `{"data":[{"type":"appInfoLocalizations","id":"","attributes":{"locale":"en-US"}}]}`,
			wantError: "localization id is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			steps := []appsRenameHTTPStep{
				appInfoOwnershipStep(),
				{
					method:       http.MethodGet,
					uri:          "/v1/appInfos/info-1/appInfoLocalizations?filter%5Blocale%5D=en-US&limit=200",
					responseBody: test.response,
				},
			}
			stdout, _, runErr := runAppsRenameWithServer(
				t, steps,
				"--app", "app-1", "--app-info", "info-1", "--locale", "en-US", "--name", "New Name",
			)
			if runErr == nil || !strings.Contains(runErr.Error(), test.wantError) {
				t.Fatalf("run error = %v, want %q", runErr, test.wantError)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

type appsRenameHTTPStep struct {
	method       string
	uri          string
	requestBody  string
	responseBody string
	status       int
}

func appInfoOwnershipStep() appsRenameHTTPStep {
	const (
		appID     = "app-1"
		appInfoID = "info-1"
	)
	return appsRenameHTTPStep{
		method:       http.MethodGet,
		uri:          "/v1/appInfos/" + appInfoID + "?include=app",
		responseBody: fmt.Sprintf(`{"data":{"type":"appInfos","id":%q,"relationships":{"app":{"data":{"type":"apps","id":%q}}}},"included":[{"type":"apps","id":%q}]}`, appInfoID, appID, appID),
	}
}

func runAppsRenameWithServer(t *testing.T, steps []appsRenameHTTPStep, args ...string) (string, string, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if requestCount >= len(steps) {
			t.Errorf("unexpected request %d: %s %s", requestCount+1, req.Method, req.URL.RequestURI())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		step := steps[requestCount]
		requestCount++
		if req.Method != step.method || req.URL.RequestURI() != step.uri {
			t.Errorf("request %d = %s %s, want %s %s", requestCount, req.Method, req.URL.RequestURI(), step.method, step.uri)
		}
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("request %d is missing bearer authorization", requestCount)
		}
		if step.requestBody != "" {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			} else if strings.TrimSpace(string(body)) != step.requestBody {
				t.Errorf("request body = %s, want %s", body, step.requestBody)
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
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return transport.RoundTrip(cloned)
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	stdout, stderr, runErr := runAppsRename(t, args...)
	if requestCount != len(steps) {
		t.Errorf("request count = %d, want %d", requestCount, len(steps))
	}
	return stdout, stderr, runErr
}

func runAppsRename(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		parseArgs := append([]string{"apps", "rename"}, args...)
		if err := root.Parse(parseArgs); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}
