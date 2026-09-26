package cmdtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsDSYMExistingFileMalformedSignedURLIsRedacted(t *testing.T) {
	setupAuth(t)
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "com.example.app-42.dSYM.zip"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	restoreTransport(t)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "" {
			return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		}
		if req.URL.Path != "/v1/builds/build-1" || req.URL.Query().Get("include") != "buildBundles" {
			t.Fatalf("unexpected request: %s", req.URL.Path)
		}
		return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}},"included":[{"type":"buildBundles","id":"bundle","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/bad\u007f.zip?X-Amz-Signature=secret-sentinel"}}]}`), nil
	})
	stdout, stderr, err := runDSYMErr(t, "builds", "dsyms", "--build-id", "build-1", "--output-dir", outputDir)
	if err == nil {
		t.Fatal("expected malformed URL error")
	}
	if strings.Contains(err.Error()+stderr+stdout, "secret-sentinel") {
		t.Fatalf("signed URL leaked: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
}

func TestBuildsDSYMExactVersionDownloadsNewestMatchingBuild(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/builds" && req.URL.Query().Get("include") == "preReleaseVersion":
			body := `{"data":[
				{"type":"builds","id":"build-old","attributes":{"version":"8","uploadedDate":"2026-01-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-old"}}}},
				{"type":"builds","id":"build-match-old","attributes":{"version":"40","uploadedDate":"2026-02-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-match"}}}},
				{"type":"builds","id":"build-match","attributes":{"version":"41","uploadedDate":"2026-03-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-match"}}}}
			],"included":[
				{"type":"preReleaseVersions","id":"prv-old","attributes":{"version":"1.0","platform":"IOS"}},
				{"type":"preReleaseVersions","id":"prv-match","attributes":{"version":"1.2.3","platform":"IOS"}}
			],"links":{}}`
			return dsymJSON(body), nil
		case req.URL.Path == "/v1/builds/build-match" && req.URL.Query().Get("include") == "buildBundles":
			body := `{"data":{"type":"builds","id":"build-match"},"included":[{"type":"buildBundles","id":"bundle","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/match.zip"}}]}`
			return dsymJSON(body), nil
		case req.URL.Host == "downloads.example.com" && req.URL.Path == "/match.zip":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("match")), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	stdout, stderr := runDSYM(t, outputDir, "builds", "dsyms", "--app", "123456789", "--version", "1.2.3", "--output", "json")
	if strings.Contains(stderr, "--build-id, --latest, or --build-number is required") {
		t.Fatalf("exact version was rejected: %s", stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-match"`) || strings.Contains(stdout, "build-old") || strings.Contains(stdout, "build-match-old") {
		t.Fatalf("stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestBuildsDSYMVersionLiveDownloadsNewestReleasedBuild(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			if !strings.Contains(req.URL.Query().Get("filter[appStoreState]"), "READY_FOR_SALE") || !strings.Contains(req.URL.Query().Get("filter[appStoreState]"), "PREORDER_READY_FOR_SALE") {
				t.Fatalf("live version query = %s", req.URL.RawQuery)
			}
			body := `{"data":[
				{"type":"appStoreVersions","id":"ver-old","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE","createdDate":"2024-01-01T00:00:00Z"}},
				{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","versionString":"2.0","appStoreState":"READY_FOR_SALE","createdDate":"2026-02-01T00:00:00Z"}}
			],"links":{}}`
			return dsymJSON(body), nil
		case req.URL.Path == "/v1/builds":
			if req.URL.Query().Get("include") != "preReleaseVersion" || req.URL.Query().Get("filter[app]") != "123456789" {
				t.Fatalf("builds query = %s", req.URL.RawQuery)
			}
			body := `{"data":[
				{"type":"builds","id":"build-old","attributes":{"version":"9","uploadedDate":"2025-01-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-old"}}}},
				{"type":"builds","id":"build-live-old","attributes":{"version":"20","uploadedDate":"2026-01-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-live"}}}},
				{"type":"builds","id":"build-live","attributes":{"version":"21","uploadedDate":"2026-03-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-live"}}}}
			],"included":[
				{"type":"preReleaseVersions","id":"prv-old","attributes":{"version":"1.0","platform":"IOS"}},
				{"type":"preReleaseVersions","id":"prv-live","attributes":{"version":"2.0","platform":"IOS"}}
			],"links":{}}`
			return dsymJSON(body), nil
		case req.URL.Path == "/v1/builds/build-live" && req.URL.Query().Get("include") == "buildBundles":
			body := `{"data":{"type":"builds","id":"build-live","attributes":{"version":"21"}},"included":[{"type":"buildBundles","id":"bundle-live","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/live.dSYM.zip"}}]}`
			return dsymJSON(body), nil
		case req.URL.Host == "downloads.example.com" && req.URL.Path == "/live.dSYM.zip":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("livedata")), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	stdout, stderr := runDSYM(t, outputDir, "builds", "dsyms", "--app", "123456789", "--version", "live", "--output", "json")
	if !strings.Contains(stderr, "Resolved build build-live") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "build-old") || strings.Contains(stdout, "build-live-old") {
		t.Fatalf("downloaded unexpected builds: %s", stdout)
	}
	if !strings.Contains(stdout, `"buildId":"build-live"`) || !strings.Contains(stdout, `"sha256":"`+sha256Hex("livedata")+`"`) {
		t.Fatalf("stdout = %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "com.example.app-2.0-21.dSYM.zip")); err != nil {
		t.Fatalf("expected live dSYM file: %v", err)
	}
}

func TestBuildsDSYMMinVersionSelectsTheRange(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/builds" && req.URL.Query().Get("include") == "preReleaseVersion":
			body := `{"data":[
				{"type":"builds","id":"build-11","attributes":{"version":"1","uploadedDate":"2026-01-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-11"}}}},
				{"type":"builds","id":"build-12","attributes":{"version":"2","uploadedDate":"2026-02-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-12"}}}},
				{"type":"builds","id":"build-20","attributes":{"version":"3","uploadedDate":"2026-03-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-20"}}}}
			],"included":[
				{"type":"preReleaseVersions","id":"prv-11","attributes":{"version":"1.1","platform":"IOS"}},
				{"type":"preReleaseVersions","id":"prv-12","attributes":{"version":"1.2","platform":"IOS"}},
				{"type":"preReleaseVersions","id":"prv-20","attributes":{"version":"2.0","platform":"IOS"}}
			],"links":{}}`
			return dsymJSON(body), nil
		case strings.HasPrefix(req.URL.Path, "/v1/builds/build-") && req.URL.Query().Get("include") == "buildBundles":
			id := strings.TrimPrefix(req.URL.Path, "/v1/builds/")
			body := `{"data":{"type":"builds","id":"` + id + `"},"included":[{"type":"buildBundles","id":"bundle","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/` + id + `.zip"}}]}`
			return dsymJSON(body), nil
		case req.URL.Host == "downloads.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(req.URL.Path)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	stdout, _ := runDSYM(t, outputDir, "builds", "dsyms", "--app", "123456789", "--min-version", "1.2", "--output", "json")
	if strings.Contains(stdout, "build-11") {
		t.Fatalf("included build below min version: %s", stdout)
	}
	if !strings.Contains(stdout, "build-12") || !strings.Contains(stdout, "build-20") {
		t.Fatalf("missing ranged builds: %s", stdout)
	}
}

func TestBuildsDSYMWaitSucceedsAfterPolling(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)
	bundleCalls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "":
			return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"7"}}}`), nil
		case req.URL.Path == "/v1/builds/build-1" && req.URL.Query().Get("include") == "buildBundles":
			bundleCalls++
			url := ""
			if bundleCalls >= 2 {
				url = `"dSYMUrl":"https://downloads.example.com/ready.zip",`
			}
			body := `{"data":{"type":"builds","id":"build-1"},"included":[{"type":"buildBundles","id":"bundle","attributes":{"bundleId":"com.example.app",` + url + `"fileName":"App"}}]}`
			return dsymJSON(body), nil
		case req.URL.Host == "downloads.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ready")), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr := runDSYM(t, outputDir, "builds", "dsyms", "--build-id", "build-1", "--wait", "--poll-interval", "1ms", "--timeout", "2s", "--output", "json")
	if bundleCalls < 2 {
		t.Fatalf("bundle calls = %d, want at least 2", bundleCalls)
	}
	if !strings.Contains(stderr, "Waiting for dSYM files for build build-1") || !strings.Contains(stdout, `"sha256":"`+sha256Hex("ready")+`"`) {
		t.Fatalf("stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestBuildsDSYMWaitTimeoutReportsNotReady(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "":
			return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"7"}}}`), nil
		case req.URL.Path == "/v1/builds/build-1" && req.URL.Query().Get("include") == "buildBundles":
			body := `{"data":{"type":"builds","id":"build-1"},"included":[{"type":"buildBundles","id":"bundle","attributes":{"bundleId":"com.example.app"}}]}`
			return dsymJSON(body), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, err := runDSYMErr(t, "builds", "dsyms", "--build-id", "build-1", "--wait", "--poll-interval", "5ms", "--timeout", "30ms", "--output-dir", outputDir, "--output", "json")
	if err == nil || !strings.Contains(err.Error(), "dsym_not_ready") {
		t.Fatalf("error = %v, want dsym_not_ready", err)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok || diagnostic.Code != shared.DiagnosticDSYMNotReady || diagnostic.Parameter != "--wait" {
		t.Fatalf("diagnostic = %+v ok=%t", diagnostic, ok)
	}
	if !strings.Contains(stdout, `"status":"dsym_not_ready"`) {
		t.Fatalf("stdout = %s stderr = %s", stdout, stderr)
	}
}

func TestBuildsDSYMRejectsConflictingSelectorsBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	_, stderr, err := runDSYMErr(t, "builds", "dsyms", "--build-id", "build-1", "--all")
	if err == nil || !strings.Contains(stderr, "--build-id cannot be combined") {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("validated after auth: %s", stderr)
	}
}

func runDSYM(t *testing.T, outputDir string, args ...string) (string, string) {
	t.Helper()
	args = append(args, "--output-dir", outputDir)
	stdout, stderr, err := runDSYMErr(t, args...)
	if err != nil {
		t.Fatalf("run error: %v\nstderr: %s", err, stderr)
	}
	return stdout, stderr
}

func runDSYMErr(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func restoreTransport(t *testing.T) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func dsymJSON(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestBuildsDSYMAllKeepsSameNamedPlatformArtifactsSeparate(t *testing.T) {
	setupAuth(t)
	outputDir := filepath.Join(t.TempDir(), "dsyms")
	restoreTransport(t)
	onlyMac := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/builds":
			if onlyMac {
				return dsymJSON(`{"data":[{"type":"builds","id":"mac","attributes":{"version":"42","uploadedDate":"2026-03-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-mac"}}}}],"included":[{"type":"preReleaseVersions","id":"prv-mac","attributes":{"version":"1.0","platform":"MAC_OS"}}],"links":{}}`), nil
			}
			return dsymJSON(`{"data":[
    {"type":"builds","id":"ios","attributes":{"version":"42","uploadedDate":"2026-03-02T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-ios"}}}},
    {"type":"builds","id":"mac","attributes":{"version":"42","uploadedDate":"2026-03-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv-mac"}}}}
   ],"included":[
    {"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.0","platform":"IOS"}},
    {"type":"preReleaseVersions","id":"prv-mac","attributes":{"version":"1.0","platform":"MAC_OS"}}
   ],"links":{}}`), nil
		case strings.HasPrefix(req.URL.Path, "/v1/builds/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/builds/")
			return dsymJSON(`{"data":{"type":"builds","id":"` + id + `"},"included":[{"type":"buildBundles","id":"bundle-` + id + `","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/` + id + `"}}]}`), nil
		case req.URL.Host == "downloads.example.com":
			data := strings.TrimPrefix(req.URL.Path, "/")
			return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(data)), Header: http.Header{"Content-Length": {"3"}}, Body: io.NopCloser(strings.NewReader(data))}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})
	stdout, _ := runDSYM(t, outputDir, "builds", "dsyms", "--app", "123456789", "--all", "--output", "json")
	var result asc.DSYMDownloadResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %#v", result.Files)
	}
	paths := map[string]bool{}
	var macPath string
	for _, file := range result.Files {
		if paths[file.FilePath] {
			t.Fatalf("different builds share output file %s", file.FilePath)
		}
		paths[file.FilePath] = true
		if file.BuildID == "mac" {
			macPath = file.FilePath
		}
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != file.BuildID || file.SHA256 != sha256Hex(file.BuildID) || file.Skipped {
			t.Fatalf("build %s received data %q, receipt %#v", file.BuildID, data, file)
		}
	}
	// Selecting only one platform later must keep the same build-specific path.
	onlyMac = true
	stdout, _ = runDSYM(t, outputDir, "builds", "dsyms", "--app", "123456789", "--all", "--platform", "MAC_OS", "--output", "json")
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].FilePath != macPath || !result.Files[0].Skipped || result.Files[0].SHA256 != sha256Hex("mac") {
		t.Fatalf("repeat download reused the wrong artifact: %#v", result.Files)
	}
}
