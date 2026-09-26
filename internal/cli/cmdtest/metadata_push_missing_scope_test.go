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
	"sync"
	"testing"
)

type metadataScopeFixture struct {
	mu       sync.Mutex
	requests []string
}

func (f *metadataScopeFixture) record(req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req.Method+" "+req.URL.Path)
}

func (f *metadataScopeFixture) seen(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matches []string
	for _, request := range f.requests {
		if strings.HasPrefix(request, prefix) {
			matches = append(matches, request)
		}
	}
	return matches
}

func metadataScopeJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// installMetadataScopeTransport serves one app info with en-US and fr-FR
// localizations and one version with en-US and fr-FR localizations.
func installMetadataScopeTransport(t *testing.T) *metadataScopeFixture {
	t.Helper()
	fixture := &metadataScopeFixture{}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		fixture.record(req)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":[
				{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}},
				{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr-FR","name":"App FR"}}
			],"links":{"next":""}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":[
				{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description","keywords":"one,two","supportUrl":"https://example.com/support","whatsNew":"Fixes"}},
				{"type":"appStoreVersionLocalizations","id":"loc-ver-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"un,deux","supportUrl":"https://example.com/fr","whatsNew":"Corrections"}}
			],"links":{"next":""}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-ver-en":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Local description"}}}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-ver-fr":
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appInfoLocalizations/loc-app-en":
			return metadataScopeJSONResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","subtitle":"Local subtitle"}}}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return metadataScopeJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`), nil
		}
	})
	return fixture
}

type metadataScopePlan struct {
	Adds []struct {
		Key   string `json:"key"`
		Scope string `json:"scope"`
	} `json:"adds"`
	Updates []struct {
		Key   string `json:"key"`
		Scope string `json:"scope"`
	} `json:"updates"`
	Deletes []struct {
		Key   string `json:"key"`
		Scope string `json:"scope"`
	} `json:"deletes"`
}

func writeMetadataScopeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runMetadataScopePush(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(append([]string{"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--output", "json"}, args...)); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func decodeMetadataScopePlan(t *testing.T, stdout string) metadataScopePlan {
	t.Helper()
	var plan metadataScopePlan
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	return plan
}

func setupMetadataScopeEnv(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
}

func TestMetadataPushAbsentAppInfoDirLeavesAppInfoScopeUnmanaged(t *testing.T) {
	setupMetadataScopeEnv(t)
	dir := t.TempDir()
	writeMetadataScopeFile(t, filepath.Join(dir, "version", "1.2.3", "en-US.json"), `{"description":"Local description"}`)
	writeMetadataScopeFile(t, filepath.Join(dir, "version", "1.2.3", "fr-FR.json"), `{"description":"Description FR"}`)

	t.Run("dry-run plans no app-info entries", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		stdout, stderr, err := runMetadataScopePush(t, "--dir", dir, "--dry-run")
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		plan := decodeMetadataScopePlan(t, stdout)
		if len(plan.Deletes) != 0 {
			t.Fatalf("expected no deletes for an absent app-info directory, got %+v", plan.Deletes)
		}
		if len(plan.Adds) != 0 {
			t.Fatalf("expected no adds, got %+v", plan.Adds)
		}
		if len(plan.Updates) != 1 || plan.Updates[0].Key != "version:1.2.3:en-US:description" {
			t.Fatalf("expected only the version description update, got %+v", plan.Updates)
		}
		if got := fixture.seen("GET /v1/appInfos/appinfo-1/appInfoLocalizations"); len(got) != 0 {
			t.Fatalf("expected unmanaged app-info localizations not to be fetched, got %v", got)
		}
		// Resolving the app info could fail as ambiguous for apps with several
		// app infos, so an unmanaged app-info scope must not resolve it.
		if got := fixture.seen("GET /v1/apps/app-1/appInfos"); len(got) != 0 {
			t.Fatalf("expected unmanaged app-info scope not to resolve the app info, got %v", got)
		}
		var identity struct {
			AppInfoID string `json:"appInfoId"`
		}
		if err := json.Unmarshal([]byte(stdout), &identity); err != nil {
			t.Fatalf("unmarshal identity: %v", err)
		}
		if identity.AppInfoID != "" {
			t.Fatalf("expected empty appInfoId for an unmanaged app-info scope, got %q", identity.AppInfoID)
		}
	})

	t.Run("apply succeeds without allow-deletes", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		stdout, stderr, err := runMetadataScopePush(t, "--dir", dir)
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		if strings.Contains(stderr, "--allow-deletes") {
			t.Fatalf("expected no allow-deletes error, got %q", stderr)
		}
		if got := fixture.seen("PATCH /v1/appStoreVersionLocalizations/loc-ver-en"); len(got) != 1 {
			t.Fatalf("expected one version update, got %v (stdout=%q)", got, stdout)
		}
		if got := fixture.seen("DELETE "); len(got) != 0 {
			t.Fatalf("expected no deletes, got %v", got)
		}
	})

	t.Run("allow-deletes does not delete unmanaged app-info locales", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		versionOnlyDir := t.TempDir()
		writeMetadataScopeFile(t, filepath.Join(versionOnlyDir, "version", "1.2.3", "en-US.json"), `{"description":"Local description"}`)
		stdout, stderr, err := runMetadataScopePush(t, "--dir", versionOnlyDir, "--allow-deletes", "--confirm")
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		if got := fixture.seen("DELETE /v1/appInfoLocalizations/"); len(got) != 0 {
			t.Fatalf("expected no app-info deletes, got %v (stdout=%q)", got, stdout)
		}
		// fr-FR exists remotely but not locally inside the managed version scope.
		if got := fixture.seen("DELETE /v1/appStoreVersionLocalizations/loc-ver-fr"); len(got) != 1 {
			t.Fatalf("expected managed version scope delete for fr, got %v", fixture.seen("DELETE "))
		}
	})
}

func TestMetadataPushAbsentVersionDirLeavesVersionScopeUnmanaged(t *testing.T) {
	setupMetadataScopeEnv(t)
	dir := t.TempDir()
	writeMetadataScopeFile(t, filepath.Join(dir, "app-info", "en-US.json"), `{"name":"App Name","subtitle":"Local subtitle"}`)
	writeMetadataScopeFile(t, filepath.Join(dir, "app-info", "fr-FR.json"), `{"name":"App FR"}`)

	t.Run("dry-run plans no version entries", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		stdout, stderr, err := runMetadataScopePush(t, "--dir", dir, "--dry-run")
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		plan := decodeMetadataScopePlan(t, stdout)
		if len(plan.Deletes) != 0 {
			t.Fatalf("expected no deletes for an absent version directory, got %+v", plan.Deletes)
		}
		if len(plan.Updates) != 1 || plan.Updates[0].Key != "app-info:en-US:subtitle" {
			t.Fatalf("expected only the app-info subtitle update, got %+v", plan.Updates)
		}
		if got := fixture.seen("GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations"); len(got) != 0 {
			t.Fatalf("expected unmanaged version localizations not to be fetched, got %v", got)
		}
	})

	t.Run("apply succeeds without allow-deletes", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		_, stderr, err := runMetadataScopePush(t, "--dir", dir)
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		if got := fixture.seen("PATCH /v1/appInfoLocalizations/loc-app-en"); len(got) != 1 {
			t.Fatalf("expected one app-info update, got %v", got)
		}
		if got := fixture.seen("DELETE "); len(got) != 0 {
			t.Fatalf("expected no deletes, got %v", got)
		}
	})
}

func TestMetadataPushEmptyAppInfoDirStillManagesAppInfoScope(t *testing.T) {
	setupMetadataScopeEnv(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	writeMetadataScopeFile(t, filepath.Join(dir, "version", "1.2.3", "en-US.json"), `{"description":"Local description"}`)
	writeMetadataScopeFile(t, filepath.Join(dir, "version", "1.2.3", "fr-FR.json"), `{"description":"Description FR"}`)

	t.Run("dry-run plans app-info deletes", func(t *testing.T) {
		installMetadataScopeTransport(t)
		stdout, stderr, err := runMetadataScopePush(t, "--dir", dir, "--dry-run")
		if err != nil {
			t.Fatalf("run error: %v\nstderr=%q", err, stderr)
		}
		plan := decodeMetadataScopePlan(t, stdout)
		if len(plan.Deletes) == 0 {
			t.Fatalf("expected an explicitly present app-info directory to plan deletes, got none")
		}
		for _, item := range plan.Deletes {
			if item.Scope != "app-info" {
				t.Fatalf("expected only app-info deletes, got %+v", plan.Deletes)
			}
		}
	})

	t.Run("apply requires allow-deletes", func(t *testing.T) {
		fixture := installMetadataScopeTransport(t)
		_, stderr, err := runMetadataScopePush(t, "--dir", dir)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if !strings.Contains(stderr, "Error: --allow-deletes is required to apply delete operations") {
			t.Fatalf("expected allow-deletes error, got %q", stderr)
		}
		for _, method := range []string{"PATCH ", "POST ", "DELETE "} {
			if got := fixture.seen(method); len(got) != 0 {
				t.Fatalf("expected no mutations, got %v", got)
			}
		}
	})
}

func TestMetadataPushRejectsTreeWithOnlyEmptyScopeDir(t *testing.T) {
	setupMetadataScopeEnv(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	fixture := installMetadataScopeTransport(t)
	_, stderr, err := runMetadataScopePush(t, "--dir", dir, "--allow-deletes", "--confirm")
	if err == nil {
		t.Fatal("expected an empty metadata tree to be rejected")
	}
	if !strings.Contains(stderr, "no metadata .json files found") {
		t.Fatalf("expected no-files error, got %q", stderr)
	}
	if got := fixture.seen(""); len(got) != 0 {
		t.Fatalf("expected no requests for an empty metadata tree, got %v", got)
	}
}
