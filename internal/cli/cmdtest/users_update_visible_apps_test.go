package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestUsersUpdateSendsRolesAndVisibleAppsInOneRequest(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	var payload asc.UserUpdateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		requests = append(requests, req.Method+" "+req.URL.Path)
		mu.Unlock()

		switch req.URL.Path {
		case "/v1/users/user-1":
			if req.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"provisioningAllowed":true}}}`))
				return
			}
			var update asc.UserUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}

			mu.Lock()
			payload = update
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"allAppsVisible":false,"provisioningAllowed":false}},"links":{}}`))
		case "/v1/users/user-1/relationships/visibleApps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","title":"relationship update failed"}]}`))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	setUsersUpdateTestClient(t, server)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"users", "update",
			"--id", "user-1",
			"--roles", "developer",
			"--visible-app", "app-2, app-1",
			"--confirm",
			"--output", "json",
		}, "1.2.3")
	})

	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	gotPayload := payload
	mu.Unlock()

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; requests=%v payload=%+v stderr=%q", exitCode, rootcmd.ExitSuccess, gotRequests, gotPayload, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !reflect.DeepEqual(gotRequests, []string{"GET /v1/users/user-1", "PATCH /v1/users/user-1"}) {
		t.Fatalf("requests = %v, want a user read then one user PATCH", gotRequests)
	}
	if gotPayload.Data.Attributes == nil {
		t.Fatal("expected attributes in user PATCH")
	}
	if !reflect.DeepEqual(gotPayload.Data.Attributes.Roles, []string{"DEVELOPER"}) {
		t.Fatalf("roles = %v, want [DEVELOPER]", gotPayload.Data.Attributes.Roles)
	}
	if gotPayload.Data.Attributes.AllAppsVisible == nil || *gotPayload.Data.Attributes.AllAppsVisible {
		t.Fatalf("allAppsVisible = %v, want false", gotPayload.Data.Attributes.AllAppsVisible)
	}
	if gotPayload.Data.Attributes.ProvisioningAllowed == nil || !*gotPayload.Data.Attributes.ProvisioningAllowed {
		t.Fatalf("provisioningAllowed = %v, want the current true", gotPayload.Data.Attributes.ProvisioningAllowed)
	}
	if gotPayload.Data.Relationships == nil || gotPayload.Data.Relationships.VisibleApps == nil {
		t.Fatal("expected visibleApps in user PATCH")
	}
	visibleApps := gotPayload.Data.Relationships.VisibleApps.Data
	if len(visibleApps) != 2 || visibleApps[0].Type != asc.ResourceTypeApps || visibleApps[0].ID != "app-2" || visibleApps[1].Type != asc.ResourceTypeApps || visibleApps[1].ID != "app-1" {
		t.Fatalf("visibleApps = %+v, want app-2 then app-1", visibleApps)
	}

	var response asc.UserResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.ID != "user-1" {
		t.Fatalf("response user ID = %q, want user-1", response.Data.ID)
	}
}

func TestUsersUpdateRolesOnlyDoesNotRequireConfirm(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	var payload asc.UserUpdateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"provisioningAllowed":true}}}`))
			return
		}
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/users/user-1" {
			http.NotFound(w, req)
			return
		}
		mu.Lock()
		requestCount++
		mu.Unlock()
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"allAppsVisible":true,"provisioningAllowed":false}},"links":{}}`))
	}))
	defer server.Close()
	setUsersUpdateTestClient(t, server)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"users", "update",
			"--id", "user-1",
			"--roles", "developer",
			"--output", "json",
		}, "1.2.3")
	})

	mu.Lock()
	gotRequestCount := requestCount
	gotPayload := payload
	mu.Unlock()

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", exitCode, rootcmd.ExitSuccess, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if gotRequestCount != 1 {
		t.Fatalf("request count = %d, want one user PATCH", gotRequestCount)
	}
	if gotPayload.Data.Relationships != nil {
		t.Fatalf("relationships = %+v, want omitted", gotPayload.Data.Relationships)
	}
	if gotPayload.Data.Attributes == nil || !reflect.DeepEqual(gotPayload.Data.Attributes.Roles, []string{"DEVELOPER"}) {
		t.Fatalf("attributes = %+v, want DEVELOPER roles", gotPayload.Data.Attributes)
	}
	if gotPayload.Data.Attributes.AllAppsVisible != nil {
		t.Fatalf("allAppsVisible = %v, want omitted", gotPayload.Data.Attributes.AllAppsVisible)
	}
	if !strings.Contains(stdout, `"id":"user-1"`) {
		t.Fatalf("stdout = %q, want user response", stdout)
	}
}

func TestUsersUpdateVisibleAppsConfirmPairingFailsBeforeHTTP(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "visible apps without confirm", args: []string{"--roles", "developer", "--visible-app", "app-1"}, wantStderr: "Error: --confirm is required when --visible-app is set\n"},
		{name: "confirm without visible apps", args: []string{"--roles", "access_to_reports", "--confirm"}, wantStderr: "Error: --confirm requires --visible-app\n"},
		{name: "explicit false confirm without visible apps", args: []string{"--roles", "access_to_reports", "--confirm=false"}, wantStderr: "Error: --confirm requires --visible-app\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				requestCount++
				mu.Unlock()
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}))
			defer server.Close()
			setUsersUpdateTestClient(t, server)

			var exitCode int
			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"users", "update", "--id", "user-1"}, tt.args...)
				exitCode = rootcmd.Run(append(args, "--output", "json"), "1.2.3")
			})

			mu.Lock()
			gotRequestCount := requestCount
			mu.Unlock()

			if exitCode != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", exitCode, rootcmd.ExitUsage, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantStderr)
			}
			if gotRequestCount != 0 {
				t.Fatalf("request count = %d, want zero", gotRequestCount)
			}
		})
	}
}

func TestUsersUpdateRejectedRequestDoesNotChangeAccess(t *testing.T) {
	type accessState struct {
		Role       string
		VisibleApp string
	}

	initial := accessState{Role: "ADMIN", VisibleApp: "app-before"}
	state := initial
	var mu sync.Mutex
	requestCount := 0
	lastRequest := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		requestCount++
		lastRequest = req.Method + " " + req.URL.Path
		mu.Unlock()

		switch req.URL.Path {
		case "/v1/users/user-1":
			if req.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"provisioningAllowed":true}}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid user update"}]}`))
		case "/v1/users/user-1/relationships/visibleApps":
			mu.Lock()
			state.VisibleApp = "unexpected-follow-up"
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	setUsersUpdateTestClient(t, server)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"users", "update",
			"--id", "user-1",
			"--roles", "developer",
			"--visible-app", "app-after",
			"--confirm",
			"--output", "json",
		}, "1.2.3")
	})

	mu.Lock()
	gotRequestCount := requestCount
	gotLastRequest := lastRequest
	gotState := state
	mu.Unlock()

	wantExitCode := rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity)
	if exitCode != wantExitCode {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, wantExitCode, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "users update: failed to update") || !strings.Contains(stderr, "Invalid user update") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if gotRequestCount != 2 || gotLastRequest != "PATCH /v1/users/user-1" {
		t.Fatalf("requests = %d, last = %q; want a user read then one user PATCH", gotRequestCount, gotLastRequest)
	}
	if gotState != initial {
		t.Fatalf("state = %+v, want unchanged %+v", gotState, initial)
	}
}

func setUsersUpdateTestClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme+"://"+req.URL.Host != asc.BaseURL {
			t.Fatalf("request origin = %s://%s, want %s", req.URL.Scheme, req.URL.Host, asc.BaseURL)
		}
		authorization := req.Header.Get("Authorization")
		token, ok := strings.CutPrefix(authorization, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			t.Fatalf("Authorization = %q, want nonempty Bearer token", authorization)
		}
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}
