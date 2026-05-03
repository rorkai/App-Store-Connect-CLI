package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppsRegistrySyncDryRunMergesAndPreservesLocalFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	registryPath := filepath.Join(dir, "app_registry.json")
	initialRegistry := `{
  "apps": [
    {
      "key": "existing-key",
      "name": "Old Name",
      "asc_app_id": "app-1",
      "bundle_id": "old.bundle",
      "platform": "IOS",
      "primary_locale": "en-US",
      "repo_path": "/tmp/existing",
      "ga4_property_id": "123",
      "aliases": ["old", "alias"]
    },
    {
      "key": "local-only",
      "name": "Local Only",
      "asc_app_id": "local-1",
      "bundle_id": "local.bundle",
      "platform": "TV_OS",
      "primary_locale": "en-US",
      "repo_path": null,
      "ga4_property_id": null,
      "aliases": []
    }
  ]
}`
	if err := os.WriteFile(registryPath, []byte(initialRegistry), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("expected apps path, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
		}
		if req.URL.Query().Get("sort") != "name" {
			t.Fatalf("expected sort=name, got %q", req.URL.Query().Get("sort"))
		}

		body := `{"data":[` +
			`{"type":"apps","id":"app-2","attributes":{"name":"New App!","bundleId":"com.example.new","sku":"NEW","primaryLocale":"en-GB"}},` +
			`{"type":"apps","id":"app-1","attributes":{"name":"Fresh Name","bundleId":"com.example.fresh","sku":"FRESH","primaryLocale":"en-US"}}` +
			`],"links":{"next":""}}`
		return appsRegistryJSONResponse(body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", registryPath,
			"--dry-run",
			"--output", "json",
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
	if got, err := os.ReadFile(registryPath); err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	} else if strings.TrimSpace(string(got)) != strings.TrimSpace(initialRegistry) {
		t.Fatalf("dry-run changed registry file:\n%s", string(got))
	}

	var result struct {
		DryRun    bool `json:"dryRun"`
		Total     int  `json:"total"`
		Created   int  `json:"created"`
		Updated   int  `json:"updated"`
		Unchanged int  `json:"unchanged"`
		Preserved int  `json:"preserved"`
		Pruned    int  `json:"pruned"`
		Registry  struct {
			Apps []map[string]any `json:"apps"`
		} `json:"registry"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if !result.DryRun || result.Total != 3 || result.Created != 1 || result.Updated != 1 || result.Unchanged != 0 || result.Preserved != 1 || result.Pruned != 0 {
		t.Fatalf("unexpected summary: %+v", result)
	}

	byID := registryAppsByID(t, result.Registry.Apps)
	existing := byID["app-1"]
	if existing["key"] != "existing-key" || existing["platform"] != "IOS" || existing["repo_path"] != "/tmp/existing" || existing["ga4_property_id"] != "123" {
		t.Fatalf("local fields were not preserved: %#v", existing)
	}
	if existing["name"] != "Fresh Name" || existing["bundle_id"] != "com.example.fresh" {
		t.Fatalf("ASC fields were not updated: %#v", existing)
	}
	if byID["app-2"]["key"] != "new-app" {
		t.Fatalf("expected generated key new-app, got %#v", byID["app-2"]["key"])
	}
	if _, ok := byID["local-1"]; !ok {
		t.Fatalf("expected local-only app to be preserved, got %#v", byID)
	}
}

func TestAppsRegistrySyncWritesRegistryFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	registryPath := filepath.Join(t.TempDir(), "nested", "app-registry.json")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Write Me","bundleId":"com.example.write","sku":"WRITE","primaryLocale":"en-US"}}],"links":{"next":""}}`
		return appsRegistryJSONResponse(body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", registryPath,
			"--output", "json",
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
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if _, ok := result["registry"]; ok {
		t.Fatalf("expected registry payload to be omitted for non-dry-run output: %#v", result["registry"])
	}

	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var registry struct {
		Apps []map[string]any `json:"apps"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("failed to parse written registry %q: %v", string(data), err)
	}
	if len(registry.Apps) != 1 {
		t.Fatalf("expected one app, got %#v", registry.Apps)
	}
	app := registry.Apps[0]
	if app["key"] != "write-me" || app["asc_app_id"] != "app-1" || app["bundle_id"] != "com.example.write" {
		t.Fatalf("unexpected written app: %#v", app)
	}
	if app["platform"] != nil || app["repo_path"] != nil || app["ga4_property_id"] != nil {
		t.Fatalf("expected unknown local fields to be null, got %#v", app)
	}
}

func TestAppsRegistrySyncPrunesMissingWhenRequested(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	registryPath := filepath.Join(t.TempDir(), "app_registry.json")
	if err := os.WriteFile(registryPath, []byte(`{"apps":[{"key":"gone","name":"Gone","asc_app_id":"gone-1","bundle_id":"gone.bundle","platform":null,"primary_locale":"en-US","repo_path":null,"ga4_property_id":null,"aliases":[]}]}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return appsRegistryJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", registryPath,
			"--prune-missing",
			"--dry-run",
			"--output", "json",
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
	var result struct {
		Total    int `json:"total"`
		Pruned   int `json:"pruned"`
		Registry struct {
			Apps []map[string]any `json:"apps"`
		} `json:"registry"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result.Total != 0 || result.Pruned != 1 || len(result.Registry.Apps) != 0 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
}

func TestAppsRegistrySyncRejectsInvalidRegistryJSON(t *testing.T) {
	registryPath := filepath.Join(t.TempDir(), "app_registry.json")
	if err := os.WriteFile(registryPath, []byte(`{"apps":[`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", registryPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime error, got ErrHelp")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "invalid registry JSON") {
		t.Fatalf("expected invalid JSON error, got %v", runErr)
	}
}

func TestAppsRegistrySyncRejectsDuplicateExistingASCAppIDs(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	registryPath := filepath.Join(t.TempDir(), "app_registry.json")
	duplicateRegistry := `{"apps":[` +
		`{"key":"first","name":"First","asc_app_id":"app-1","bundle_id":"one.bundle","platform":null,"primary_locale":"en-US","repo_path":null,"ga4_property_id":null,"aliases":[]},` +
		`{"key":"second","name":"Second","asc_app_id":"app-1","bundle_id":"two.bundle","platform":null,"primary_locale":"en-US","repo_path":null,"ga4_property_id":null,"aliases":[]}` +
		`]}`
	if err := os.WriteFile(registryPath, []byte(duplicateRegistry), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return appsRegistryJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", registryPath,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), `duplicate asc_app_id "app-1"`) {
		t.Fatalf("expected duplicate asc_app_id error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty output, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestAppsRegistrySyncRejectsDuplicateASCResponseIDs(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":[` +
			`{"type":"apps","id":"app-1","attributes":{"name":"One","bundleId":"one.bundle","sku":"ONE","primaryLocale":"en-US"}},` +
			`{"type":"apps","id":"app-1","attributes":{"name":"Two","bundleId":"two.bundle","sku":"TWO","primaryLocale":"en-US"}}` +
			`],"links":{"next":""}}`
		return appsRegistryJSONResponse(body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", filepath.Join(t.TempDir(), "registry.json"),
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), `duplicate app id "app-1"`) {
		t.Fatalf("expected duplicate app id error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty output, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestAppsRegistrySyncParserPermutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return appsRegistryJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	}))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root flag before subcommand",
			args: []string{"--debug=false", "apps", "registry", "sync", "--path", filepath.Join(t.TempDir(), "registry.json"), "--dry-run", "--output", "json"},
		},
		{
			name: "mixed flag order",
			args: []string{"apps", "registry", "sync", "--output", "json", "--dry-run", "--path", filepath.Join(t.TempDir(), "registry.json")},
		},
		{
			name: "flag value equals subcommand name",
			args: []string{"apps", "registry", "sync", "--path", filepath.Join(t.TempDir(), "sync"), "--dry-run", "--output", "json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
			}
			if result["dryRun"] != true {
				t.Fatalf("expected dryRun=true, got %#v", result["dryRun"])
			}
		})
	}
}

func TestAppsRegistrySyncInvalidFlagValues(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return appsRegistryJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	}))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "empty path",
			args:    []string{"apps", "registry", "sync", "--path", ""},
			wantErr: "--path is required",
		},
		{
			name:    "output value equals subcommand",
			args:    []string{"apps", "registry", "sync", "--path", filepath.Join(t.TempDir(), "registry.json"), "--dry-run", "--output", "sync"},
			wantErr: "unsupported format: sync",
		},
		{
			name:    "pretty with table output",
			args:    []string{"apps", "registry", "sync", "--path", filepath.Join(t.TempDir(), "registry.json"), "--dry-run", "--output", "table", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected help error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppsRegistrySyncTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return appsRegistryJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "registry", "sync",
			"--path", filepath.Join(t.TempDir(), "registry.json"),
			"--dry-run",
			"--output", "table",
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
	if !strings.Contains(stdout, "Path") || !strings.Contains(stdout, "Dry Run") {
		t.Fatalf("expected table output, got %q", stdout)
	}
}

func registryAppsByID(t *testing.T, apps []map[string]any) map[string]map[string]any {
	t.Helper()

	byID := make(map[string]map[string]any, len(apps))
	for _, app := range apps {
		id, ok := app["asc_app_id"].(string)
		if !ok || id == "" {
			t.Fatalf("app missing asc_app_id: %#v", app)
		}
		byID[id] = app
	}
	return byID
}

func appsRegistryJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
