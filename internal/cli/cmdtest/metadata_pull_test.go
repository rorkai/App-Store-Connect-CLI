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
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMetadataPullValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name          string
		args          []string
		wantErr       string
		wantStderr    string
		wantParameter string
		wantConcise   bool
	}{
		{
			name:          "missing app",
			args:          []string{"metadata", "pull", "--version", "1.2.3", "--dir", "./metadata"},
			wantStderr:    "Error: --app is required (or set ASC_APP_ID)\n",
			wantParameter: "--app",
			wantConcise:   true,
		},
		{
			name:          "missing version",
			args:          []string{"metadata", "pull", "--app", "app-1", "--dir", "./metadata"},
			wantStderr:    "Error: --version is required\nFind versions:\n  asc versions list --app \"APP_ID\" --paginate\n",
			wantParameter: "--version",
			wantConcise:   true,
		},
		{
			name:          "missing dir",
			args:          []string{"metadata", "pull", "--app", "app-1", "--version", "1.2.3"},
			wantStderr:    "Error: --dir is required\n",
			wantParameter: "--dir",
			wantConcise:   true,
		},
		{
			name:    "invalid include",
			args:    []string{"metadata", "pull", "--app", "app-1", "--version", "1.2.3", "--dir", "./metadata", "--include", "screenshots"},
			wantErr: "Error: --include supports only \"localizations\"",
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

			if test.wantConcise {
				if errors.Is(runErr, flag.ErrHelp) || !shared.IsReportedUsageError(runErr) {
					t.Fatalf("expected reported usage error without ErrHelp, got %v", runErr)
				}
				diagnostic, ok := shared.DiagnosticFromError(runErr)
				if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != test.wantParameter {
					t.Fatalf("diagnostic = %+v, found=%t", diagnostic, ok)
				}
			} else if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if test.wantConcise && stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !test.wantConcise && !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestMetadataPullWritesCanonicalLayout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := filepath.Join(t.TempDir(), "metadata")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{
				"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{
				"data":[
					{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App Name","subtitle":"Great app"}},
					{"type":"appInfoLocalizations","id":"appinfo-loc-2","attributes":{"locale":"bn-BD","name":"Bangla App"}},
					{"type":"appInfoLocalizations","id":"appinfo-loc-3","attributes":{"locale":"ja","name":"アプリ"}}
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
					{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","description":"English description","keywords":"one,two","whatsNew":"Bug fixes"}},
					{"type":"appStoreVersionLocalizations","id":"version-loc-2","attributes":{"locale":"bn-BD","description":"Bangla description"}},
					{"type":"appStoreVersionLocalizations","id":"version-loc-3","attributes":{"locale":"ja","description":"日本語説明"}}
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
			"metadata", "pull",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", outputDir,
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

	paths := []string{
		filepath.Join(outputDir, "app-info", "bn-BD.json"),
		filepath.Join(outputDir, "app-info", "en-US.json"),
		filepath.Join(outputDir, "app-info", "ja.json"),
		filepath.Join(outputDir, "version", "1.2.3", "bn-BD.json"),
		filepath.Join(outputDir, "version", "1.2.3", "en-US.json"),
		filepath.Join(outputDir, "version", "1.2.3", "ja.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected file %q to exist: %v", path, err)
		}
		if !json.Valid(data) {
			t.Fatalf("expected valid JSON in %q, got %q", path, string(data))
		}
	}

	appInfoData, err := os.ReadFile(filepath.Join(outputDir, "app-info", "en-US.json"))
	if err != nil {
		t.Fatalf("read app-info file: %v", err)
	}
	if !strings.Contains(string(appInfoData), `"name":"App Name"`) {
		t.Fatalf("expected app-info content in file, got %q", string(appInfoData))
	}

	versionData, err := os.ReadFile(filepath.Join(outputDir, "version", "1.2.3", "en-US.json"))
	if err != nil {
		t.Fatalf("read version file: %v", err)
	}
	if !strings.Contains(string(versionData), `"description":"English description"`) {
		t.Fatalf("expected version description in file, got %q", string(versionData))
	}

	var payload struct {
		FileCount int      `json:"fileCount"`
		Files     []string `json:"files"`
		Includes  []string `json:"includes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	if payload.FileCount != 6 {
		t.Fatalf("expected fileCount 6, got %d", payload.FileCount)
	}
	if len(payload.Files) != 6 {
		t.Fatalf("expected 6 files in output, got %d", len(payload.Files))
	}
	sortedFiles := append([]string(nil), payload.Files...)
	slices.Sort(sortedFiles)
	if !slices.Equal(payload.Files, sortedFiles) {
		t.Fatalf("expected deterministic sorted file list, got %v", payload.Files)
	}
	if len(payload.Includes) != 1 || payload.Includes[0] != "localizations" {
		t.Fatalf("expected includes [localizations], got %v", payload.Includes)
	}
}

func TestMetadataPullRequiresForceToOverwriteExistingFiles(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := filepath.Join(t.TempDir(), "metadata")
	if err := os.MkdirAll(filepath.Join(outputDir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	existingPath := filepath.Join(outputDir, "app-info", "en-US.json")
	if err := os.WriteFile(existingPath, []byte(`{"name":"existing"}`), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
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
			body := `{"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App Name"}}],"links":{"next":""}}`
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

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "pull",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", outputDir,
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
	if !strings.Contains(stderr, "use --force") {
		t.Fatalf("expected force overwrite guidance, got %q", stderr)
	}
}

func TestMetadataPullRejectsAmbiguousVersionWithoutPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := filepath.Join(t.TempDir(), "metadata")

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
			"metadata", "pull",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", outputDir,
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
	if !strings.Contains(stderr, "\n  IOS     version version-ios\n  MAC_OS  version version-mac\n") {
		t.Fatalf("expected platform candidates in stderr, got %q", stderr)
	}
}

func TestMetadataPullVersionNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := filepath.Join(t.TempDir(), "metadata")

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

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "pull",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", outputDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected version-not-found error")
	}
	if !strings.Contains(runErr.Error(), `metadata pull: app store version not found for version "1.2.3"`) {
		t.Fatalf("expected wrapped version-not-found error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestMetadataPullPaginatesLocalizations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := filepath.Join(t.TempDir(), "metadata")

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
			if req.URL.RawQuery == "limit=200" {
				body := `{
					"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App EN"}}],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/appinfo-1/appInfoLocalizations?cursor=app-2"}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			if req.URL.RawQuery == "cursor=app-2" {
				body := `{
					"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-2","attributes":{"locale":"ja","name":"App JA"}}],
					"links":{"next":""}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.URL.RawQuery == "limit=200" {
				body := `{
					"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","description":"Desc EN"}}],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=ver-2"}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			if req.URL.RawQuery == "cursor=ver-2" {
				body := `{
					"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-2","attributes":{"locale":"ja","description":"Desc JA"}}],
					"links":{"next":""}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "pull",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", outputDir,
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
		FileCount int      `json:"fileCount"`
		Locales   []string `json:"locales"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.FileCount != 4 {
		t.Fatalf("expected 4 files after pagination, got %d", payload.FileCount)
	}
	if !slices.Equal(payload.Locales, []string{"en-US", "ja"}) {
		t.Fatalf("expected locales [en-US ja], got %v", payload.Locales)
	}
}

func TestMetadataPullSupportsTableAndMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		outputFlag string
		wantText   string
	}{
		{name: "table", outputFlag: "table", wantText: "File Count: 2"},
		{name: "markdown", outputFlag: "markdown", wantText: "**App ID:** app-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputDir := filepath.Join(t.TempDir(), "metadata")

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
					body := `{"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App EN"}}],"links":{"next":""}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					body := `{"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","description":"Desc EN"}}],"links":{"next":""}}`
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
					"metadata", "pull",
					"--app", "app-1",
					"--version", "1.2.3",
					"--dir", outputDir,
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
