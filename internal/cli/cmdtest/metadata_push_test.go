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

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestMetadataPushApplyDeleteGuardrails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected guardrail path to avoid mutations, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	t.Run("requires allow-deletes", func(t *testing.T) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", runErr)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "Error: --allow-deletes is required to apply delete operations") {
			t.Fatalf("expected allow-deletes error, got %q", stderr)
		}
	})

	t.Run("requires confirm", func(t *testing.T) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
				"--allow-deletes",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", runErr)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "Error: --confirm is required when applying delete operations") {
			t.Fatalf("expected confirm error, got %q", stderr)
		}
	})
}

func TestRunMetadataPushRejectsOverLimitKeywordBytesBeforeAuthResolution(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	body := `{"keywords":"` + strings.Repeat("語", 101) + `"}`
	if err := os.WriteFile(filepath.Join(versionDir, "ja.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "keywords exceed 100 characters") {
		t.Fatalf("expected keyword character-limit error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestMetadataPushDryRunBuildsPlanWithoutMutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description","keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write version en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"日本語説明"}`), 0o644); err != nil {
		t.Fatalf("write version ja file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-1","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}},
					{"type":"appInfoLocalizations","id":"loc-app-2","attributes":{"locale":"fr","name":"App FR"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-1","attributes":{"locale":"en-US","description":"Remote description","marketingUrl":"https://example.com/remote"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1":
			body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "creating locale ja would make it participate in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	for _, want := range []string{"keywords, supportUrl", "Fill the remaining metadata before submission."} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}

	var payload struct {
		Adds []struct {
			Key string `json:"key"`
		} `json:"adds"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	if len(payload.Adds) != 2 {
		t.Fatalf("expected 2 adds, got %d (%+v)", len(payload.Adds), payload.Adds)
	}
	if len(payload.Updates) != 2 {
		t.Fatalf("expected 2 updates, got %d (%+v)", len(payload.Updates), payload.Updates)
	}
	if len(payload.Deletes) != 1 {
		t.Fatalf("expected 1 delete, got %d (%+v)", len(payload.Deletes), payload.Deletes)
	}
}

func TestMetadataPushDryRunDoesNotWarnForCompleteCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"日本語説明","keywords":"一,二","supportUrl":"https://example.com/ja"}`), 0o644); err != nil {
		t.Fatalf("write version ja file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D") {
				body := `{"data":[],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1":
			body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		Adds []struct {
			Key string `json:"key"`
		} `json:"adds"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Adds) != 3 {
		t.Fatalf("expected three create-plan fields, got %+v", payload.Adds)
	}
}

func TestMetadataPushRejectsDuplicateCanonicalLocaleFiles(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "de-DE.json"), []byte(`{"keywords":"eins,zwei"}`), 0o644); err != nil {
		t.Fatalf("write de-DE file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "de.json"), []byte(`{"keywords":"drei,vier"}`), 0o644); err != nil {
		t.Fatalf("write de file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`duplicate canonical locale "de-DE"`, `"de-DE.json"`, `"de.json"`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestMetadataPushDryRunOmittedFieldsDoNotPlanDeletes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	for _, item := range payload.Deletes {
		if item.Key == "app-info:en-US:subtitle" {
			t.Fatalf("expected omitted subtitle to be no-op, got deletes=%+v", payload.Deletes)
		}
	}
}

func TestMetadataPushDryRunAcceptsCaseInsensitiveKeys(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"Name":"App Name","SubTitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"Description":"Local description","Whatsnew":"Local whats new"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		Adds []struct {
			Key string `json:"key"`
		} `json:"adds"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	containsKey := func(items []struct {
		Key string `json:"key"`
	}, want string,
	) bool {
		for _, item := range items {
			if item.Key == want {
				return true
			}
		}
		return false
	}

	if !containsKey(payload.Updates, "app-info:en-US:subtitle") {
		t.Fatalf("expected app-info subtitle update with canonical key, got updates=%+v", payload.Updates)
	}
	if !containsKey(payload.Updates, "version:1.2.3:en-US:description") {
		t.Fatalf("expected version description update with canonical key, got updates=%+v", payload.Updates)
	}
	if !containsKey(payload.Adds, "version:1.2.3:en-US:whatsNew") {
		t.Fatalf("expected version whatsNew add with canonical key, got adds=%+v", payload.Adds)
	}
}

func TestMetadataPushDryRunUsesDefaultLocaleFallback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "default.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"Remote EN"}},
					{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote EN description"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		Adds    []struct{} `json:"adds"`
		Updates []struct{} `json:"updates"`
		Deletes []struct{} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Deletes) != 0 {
		t.Fatalf("expected default fallback to prevent deletes, got %+v", payload.Deletes)
	}
	if len(payload.Updates) == 0 {
		t.Fatalf("expected updates from default fallback, got %+v", payload)
	}
}

func TestMetadataPushDryRunAllowDeletesDisablesDefaultLocaleFallback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "default.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected dry-run to use GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"Remote EN"}},
					{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote EN description"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
			"--allow-deletes",
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

	var payload struct {
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	foundAppInfoDelete := false
	for _, item := range payload.Deletes {
		if item.Key == "app-info:fr:name" {
			foundAppInfoDelete = true
			break
		}
	}
	if !foundAppInfoDelete {
		t.Fatalf("expected delete for remote-only locale when allow-deletes is set, got %+v", payload.Deletes)
	}
}

func TestMetadataPushApplyExecutesUpdates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	updateCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1":
			body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if req.Method == http.MethodGet {
				body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.Method == http.MethodGet {
				body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appInfoLocalizations/loc-app-en":
			if req.Method == http.MethodPatch {
				updateCount++
				body := `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations/loc-ver-en":
			if req.Method == http.MethodPatch {
				updateCount++
				body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Local description"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
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
	if updateCount != 2 {
		t.Fatalf("expected 2 update API calls, got %d", updateCount)
	}

	var payload struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Action string `json:"action"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied=true, got %+v", payload)
	}
	if len(payload.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %+v", payload.Actions)
	}
}

func TestMetadataPushApplyWarnsForIncompleteCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"日本語説明"}`), 0o644); err != nil {
		t.Fatalf("write version ja file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	createCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if req.Method == http.MethodGet {
				body := `{"data":[],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.Method == http.MethodGet {
				body := `{"data":[],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersions/version-1":
			if req.Method == http.MethodGet {
				body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations":
			if req.Method == http.MethodPost {
				createCount++
				body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-ja","attributes":{"locale":"ja","description":"日本語説明"}}}`
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if createCount != 1 {
		t.Fatalf("expected one create API call, got %d", createCount)
	}
	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected applied create warning on stderr, got %q", stderr)
	}
	for _, want := range []string{"keywords, supportUrl", "Fill the remaining metadata before submission."} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}

	var payload struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Action string `json:"action"`
			Scope  string `json:"scope"`
			Locale string `json:"locale"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied result, got %+v", payload)
	}
	if len(payload.Actions) != 1 || payload.Actions[0].Action != "create" || payload.Actions[0].Locale != "ja" || payload.Actions[0].Scope != "version" {
		t.Fatalf("expected single version create action, got %+v", payload.Actions)
	}
}

func TestMetadataPushApplyWarnsWhenUpdateRequiresWhatsNew(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"日本語説明","keywords":"日本語,キーワード","supportUrl":"https://example.com/support-ja"}`), 0o644); err != nil {
		t.Fatalf("write version ja file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	createCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D") {
				body := `{"data":[{"type":"appStoreVersions","id":"released-version","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1":
			body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersionLocalizations":
			if req.Method != http.MethodPost {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			}
			createCount++
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-ja","attributes":{"locale":"ja","description":"日本語説明","keywords":"日本語,キーワード","supportUrl":"https://example.com/support-ja"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if createCount != 1 {
		t.Fatalf("expected one create API call, got %d", createCount)
	}
	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected applied create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected whatsNew requirement in warning, got %q", stderr)
	}

	var payload struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied result, got %+v", payload)
	}
}

func TestMetadataPushApplyDeletesWhenConfirmed(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	deleteCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if req.Method == http.MethodGet {
				body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}],"links":{"next":""}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfoLocalizations/loc-app-fr":
			if req.Method == http.MethodDelete {
				deleteCount++
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appInfoLocalizations":
			if req.Method == http.MethodPost {
				body := `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name"}}}`
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--allow-deletes",
			"--confirm",
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
	if deleteCount != 1 {
		t.Fatalf("expected one delete API call, got %d", deleteCount)
	}

	var payload struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied=true, got %+v", payload)
	}
}

func TestMetadataPushDryRunPlanIsDeterministic(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-1","attributes":{"locale":"en-US","name":"Remote Name"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-1","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	runPush := func() string {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
				"--dry-run",
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
		return stdout
	}

	first := runPush()
	second := runPush()
	if first != second {
		t.Fatalf("expected deterministic JSON plan output,\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestMetadataPushRejectsAmbiguousVersionWithoutPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-ios","attributes":{"versionString":"1.2.3","platform":"IOS"}},
					{"type":"appStoreVersions","id":"version-mac","attributes":{"versionString":"1.2.3","platform":"MAC_OS"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: 2 app store versions match version "1.2.3"; pass --platform with one of:`) {
		t.Fatalf("expected ambiguous-version error, got %q", stderr)
	}
}

func TestMetadataPushRejectsAmbiguousAppInfoWithActionableRemediation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appInfos":
			body := `{
				"data":[
					{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
					{"type":"appInfos","id":"appinfo-2","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: 2 app infos match app "app-1"; pass --app-info with one of:`) {
		t.Fatalf("expected ambiguous app-info error, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc apps info list --app "app-1"`) {
		t.Fatalf("expected remediation to mention apps info list, got %q", stderr)
	}
	if !strings.Contains(stderr, `--app-info "appinfo-1"`) {
		t.Fatalf("expected remediation example with --app-info, got %q", stderr)
	}
	if !strings.Contains(stderr, "appinfo-2") {
		t.Fatalf("expected all candidate app info ids in remediation, got %q", stderr)
	}
}

func TestMetadataPushDryRunUsesExplicitAppInfoOverride(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-override/appInfoLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appInfos":
			t.Fatal("did not expect appInfos lookup when --app-info override is provided")
			return nil, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--app-info", "appinfo-override",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		AppInfoID string `json:"appInfoId"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.AppInfoID != "appinfo-override" {
		t.Fatalf("expected appInfoId appinfo-override, got %+v", payload)
	}
}

func TestMetadataPushDryRunAutoResolvesAppInfoByVersionState(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appInfos":
			body := `{
				"data":[
					{"type":"appInfos","id":"appinfo-ready","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
					{"type":"appInfos","id":"appinfo-target","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-target/appInfoLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
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

	var payload struct {
		AppInfoID string `json:"appInfoId"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.AppInfoID != "appinfo-target" {
		t.Fatalf("expected auto-resolved appInfoId appinfo-target, got %+v", payload)
	}
}

func TestMetadataPushApplyFailsOnPartialMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Chdir(t.TempDir())

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	patchCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfoLocalizations/loc-app-en":
			if req.Method == http.MethodPatch {
				patchCount++
				body := `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations/loc-ver-en":
			if req.Method == http.MethodPatch {
				patchCount++
				body := `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Error","detail":"boom"}]}`
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected reported partial failure, got %T: %v", runErr, runErr)
	}
	if patchCount != 2 {
		t.Fatalf("expected two patch attempts before failure, got %d", patchCount)
	}
	if !strings.Contains(runErr.Error(), "metadata push: 1 localization(s) failed") {
		t.Fatalf("expected batch failure summary, got %v", runErr)
	}
	var result struct {
		Succeeded           int    `json:"succeeded"`
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse partial output: %v\nstdout=%q", err, stdout)
	}
	if result.Succeeded != 1 || result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("unexpected partial summary: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on failure, got %q", stderr)
	}
}

func TestMetadataPushSupportsTableAndMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		outputFlag string
		wantText   string
	}{
		{name: "table", outputFlag: "table", wantText: "Dry Run: true"},
		{name: "markdown", outputFlag: "markdown", wantText: "**Dry Run:** true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
				t.Fatalf("mkdir app-info: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
				t.Fatalf("write app-info file: %v", err)
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/apps/app-1/appInfos":
					body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				case "/v1/apps/app-1/appStoreVersions":
					body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				case "/v1/appInfos/appinfo-1/appInfoLocalizations":
					body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name"}}],"links":{"next":""}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					body := `{"data":[],"links":{"next":""}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				default:
					t.Fatalf("unexpected path: %s", req.URL.Path)
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"metadata", "push",
					"--app", "app-1",
					"--version", "1.2.3",
					"--dir", dir,
					"--dry-run",
					"--output", test.outputFlag,
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
			if !strings.Contains(stdout, test.wantText) {
				t.Fatalf("expected %q in output, got %q", test.wantText, stdout)
			}
		})
	}
}
