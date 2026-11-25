package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func reviewDetailsFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReviewDetailsSetValidatesWholeFileBeforeRequests(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{"notes":"valid","unknown":true}`, "unknown field"},
		{`{"notes":"a","notes":"b"}`, "duplicate field"},
		{`{"notes":null}`, "cannot be null"},
		{`{}`, "at least one field"},
		{`{"notes":"valid"} {}`, "trailing data"},
		{`{"notes":"` + strings.Repeat("a", 4001) + `"}`, "4,000"},
		{`{"contactPhone":"+1 123"}`, "10 digits"},
		{`{"demoAccountPassword":"plaintext"}`, "must reference"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), "review", "details", "set", "--version-id", "v1", "--file", reviewDetailsFile(t, tc.body))
			if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
			}
		})
	}
}

func TestReviewDetailsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	for _, command := range []string{"plan", "set"} {
		t.Run(command, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "review", "details", command, "--version-id", "v1", "--file", path)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--file does not exist: "+path) || len(reg.Requests()) != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, len(reg.Requests()))
			}
		})
	}
}

func TestReviewDetailsPlanSetAndRepeat(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_DEMO_ACCOUNT_PASSWORD", "fixture-password")
	t.Setenv("ASC_APP_ID", "com.example.review")
	var current map[string]any
	var writes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/v1/apps" && r.URL.Query().Get("filter[bundleId]") == "com.example.review" {
			if _, err := w.Write([]byte(`{"data":[{"type":"apps","id":"100000002","attributes":{"bundleId":"com.example.review"}}]}`)); err != nil {
				t.Error(err)
			}
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/v1" {
			if r.URL.Query().Get("include") != "app" {
				t.Errorf("scoped version read must include app ownership: %s", r.URL.RawQuery)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "v1", "type": "appStoreVersions", "relationships": map[string]any{"app": map[string]any{"data": map[string]any{"type": "apps", "id": "100000002"}}}}}); err != nil {
				t.Error(err)
			}
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/v1/appStoreReviewDetail" {
			if current == nil {
				w.WriteHeader(http.StatusNotFound)
				if _, err := w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"missing"}]}`)); err != nil {
					t.Error(err)
				}
				return
			}
		} else if r.Method == http.MethodPost && r.URL.Path == "/v1/appStoreReviewDetails" || r.Method == http.MethodPatch && r.URL.Path == "/v1/appStoreReviewDetails/d1" {
			var body struct {
				Data struct {
					Attributes    map[string]any `json:"attributes"`
					Relationships map[string]any `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode: %v", err)
				w.WriteHeader(400)
				return
			}
			if r.Method == http.MethodPost && body.Data.Relationships["appStoreVersion"] == nil {
				t.Error("missing version relationship")
			}
			if current == nil {
				current = map[string]any{}
			}
			for k, v := range body.Data.Attributes {
				current[k] = v
			}
			writes++
		} else {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "d1", "type": "appStoreReviewDetails", "attributes": current}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := stubServerClient(t, server)
	initial := reviewDetailsFile(t, `{"contactFirstName":"Dev","contactPhone":"+1 408 555 0100","demoAccountRequired":true,"demoAccountName":"reviewer","demoAccountPassword":"@env:ASC_DEMO_ACCOUNT_PASSWORD","notes":"Start here"}`)
	update := reviewDetailsFile(t, `{"notes":"Updated notes"}`)
	for _, tc := range []struct {
		command, path, action string
		writes                int
	}{
		{"plan", initial, "create", 0},
		{"set", initial, "create", 1},
		{"set", initial, "noop", 1},
		{"plan", update, "update", 1},
		{"set", update, "update", 2},
		{"set", update, "noop", 2},
	} {
		stdout, stderr, err := runRootWithDeps(t, client, "review", "details", tc.command, "--version-id", "v1", "--file", tc.path, "--output", "json")
		if err != nil {
			t.Fatalf("%s: %v stderr=%s", tc.command, err, stderr)
		}
		var result struct {
			Action  string            `json:"action"`
			Changes []json.RawMessage `json:"changes"`
		}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Action != tc.action || writes != tc.writes {
			t.Fatalf("result=%s writes=%d expected %s / %d", stdout, writes, tc.action, tc.writes)
		}
		if strings.Contains(stdout+stderr, "fixture-password") {
			t.Fatal("password leaked")
		}
	}
	if current["demoAccountName"] != "reviewer" || current["notes"] != "Updated notes" {
		t.Fatalf("unexpected state %#v", current)
	}
}

func TestReviewDetailsSetRejectsIncompleteCreateAndDemoState(t *testing.T) {
	for _, tc := range []struct{ remote, body, want string }{
		{`{"data":null}`, `{"notes":"hello"}`, "contactFirstName"},
		{`{"data":null}`, `{"contactFirstName":"Dev","contactPhone":"+1 408 555 0100"}`, "demoAccountRequired"},
		{`{"data":{"type":"appStoreReviewDetails","id":"d1","attributes":{"demoAccountRequired":true}}}`, `{"notes":"hello"}`, "demoAccountName"},
	} {
		reg := asctest.NewRegistry(t)
		reg.Register(asctest.GET("/v1/appStoreVersions/v1"), asctest.JSON(200, `{"data":{"id":"v1","type":"appStoreVersions"}}`))
		reg.Register(asctest.GET("/v1/appStoreVersions/v1/appStoreReviewDetail"), asctest.JSON(200, tc.remote))
		_, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details", "set", "--version-id", "v1", "--file", reviewDetailsFile(t, tc.body))
		if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, tc.want) {
			t.Fatalf("stderr=%q err=%v", stderr, err)
		}
	}
}

func TestReviewDetailsEnforcesSelectedAppBeforeReadingDetails(t *testing.T) {
	env := func(value string) *string { return &value }
	for _, command := range []string{"plan", "set"} {
		for _, tc := range []struct {
			name, flagApp, configApp string
			envApp                   *string
			scoped, allowed          bool
		}{
			{name: "explicit mismatch", flagApp: "100000001", envApp: env("100000002"), configApp: "100000002", scoped: true},
			{name: "environment mismatch", envApp: env("100000001"), configApp: "100000002", scoped: true},
			{name: "default mismatch", configApp: "100000001", scoped: true},
			{name: "explicit overrides environment and default", flagApp: "100000002", envApp: env("100000001"), configApp: "100000001", scoped: true, allowed: true},
			{name: "environment overrides default", envApp: env("100000002"), configApp: "100000001", scoped: true, allowed: true},
			{name: "matching default", configApp: "100000002", scoped: true, allowed: true},
			{name: "empty environment disables default", envApp: env(""), configApp: "100000001", allowed: true},
			{name: "resource only", allowed: true},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				configPath := filepath.Join(t.TempDir(), "config.json")
				configData, err := json.Marshal(map[string]string{"app_id": tc.configApp})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(configPath, configData, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("ASC_CONFIG_PATH", configPath)
				t.Setenv("ASC_APP_ID", "")
				if tc.envApp == nil {
					if err := os.Unsetenv("ASC_APP_ID"); err != nil {
						t.Fatal(err)
					}
				} else {
					t.Setenv("ASC_APP_ID", *tc.envApp)
				}
				reg := asctest.NewRegistry(t)
				var query []string
				if tc.scoped {
					query = []string{"include=app"}
				}
				reg.Register(asctest.GET("/v1/appStoreVersions/v1", query...),
					asctest.JSON(http.StatusOK, `{"data":{"id":"v1","type":"appStoreVersions","relationships":{"app":{"data":{"id":"100000002","type":"apps"}}}}}`))
				if tc.allowed {
					reg.Register(asctest.GET("/v1/appStoreVersions/v1/appStoreReviewDetail"),
						asctest.JSON(http.StatusOK, `{"data":{"id":"d1","type":"appStoreReviewDetails","attributes":{"notes":"Unchanged"}}}`))
				}
				args := []string{"review", "details", command, "--version-id", "v1", "--file", reviewDetailsFile(t, `{"notes":"Unchanged"}`)}
				if tc.flagApp != "" {
					args = append(args, "--app", tc.flagApp)
				}
				stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
				if tc.allowed {
					if code != 0 || !strings.Contains(stdout, `"action":"noop"`) {
						t.Fatalf("stdout=%q stderr=%q code=%d", stdout, stderr, code)
					}
				} else {
					if code != 1 || stdout != "" || !strings.Contains(stderr, "belongs to app") || len(reg.Requests()) != 1 {
						t.Fatalf("stdout=%q stderr=%q code=%d request count=%d", stdout, stderr, code, len(reg.Requests()))
					}
				}
			})
		}
	}
}
