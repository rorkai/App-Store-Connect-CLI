package cmdtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// isolateWebSessionCache points the web-session cache at an empty temporary
// directory and pins the file backend so no test can read, write, or prompt
// for the developer's real Apple session.
func isolateWebSessionCache(t *testing.T) string {
	t.Helper()
	cacheDir := t.TempDir()
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", cacheDir)
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_IRIS_SESSION_CACHE", "0")
	t.Setenv("ASC_IRIS_SESSION_CACHE_DIR", t.TempDir())
	previousTransport := http.DefaultTransport
	http.DefaultTransport = webSessionInfoTransport{base: previousTransport}
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	return cacheDir
}

type webSessionInfoTransport struct {
	base      http.RoundTripper
	calls     *int
	sawCookie *bool
	status    int
	email     string
}

func (t webSessionInfoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && req.URL.Scheme == "https" && req.URL.Host == "appstoreconnect.apple.com" && req.URL.Path == "/olympus/v1/session" {
		if t.calls != nil {
			*t.calls = *t.calls + 1
		}
		if t.sawCookie != nil && strings.Contains(req.Header.Get("Cookie"), "myacinfo=") {
			*t.sawCookie = true
		}
		status := t.status
		if status == 0 {
			status = http.StatusOK
		}
		email := t.email
		if email == "" {
			email = "user@example.com"
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"provider":{"providerId":42},"user":{"emailAddress":"` + email + `"}}`)),
			Request:    req,
		}, nil
	}
	return t.base.RoundTrip(req)
}

const webSessionBundleFixture = `{
  "kind": "asc-web-session",
  "version": 1,
  "exportedAt": "2026-09-02T10:00:00Z",
  "appleId": "user@example.com",
  "cookies": [
    {"url": "https://appstoreconnect.apple.com/", "name": "myacinfo", "value": "super-secret-token"},
    {"url": "https://idmsa.apple.com/", "name": "dslang", "value": "US-EN"}
  ]
}
`

func writeWebSessionBundleFixture(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestWebAuthImportThenExportRoundTripsSession(t *testing.T) {
	isolateWebSessionCache(t)
	workDir := t.TempDir()
	bundlePath := writeWebSessionBundleFixture(t, workDir, "session.json", webSessionBundleFixture)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if !strings.Contains(stderr, "asc web auth status") || !strings.Contains(stderr, "local bundle validation") {
		t.Fatalf("expected import to explain local validation and point at the live validation command, got stderr=%q", stderr)
	}

	var imported struct {
		AppleID               string `json:"appleId"`
		CookieCount           int    `json:"cookieCount"`
		SkippedExpiredCookies int    `json:"skippedExpiredCookies"`
		Imported              bool   `json:"imported"`
	}
	if err := json.Unmarshal([]byte(stdout), &imported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if imported.AppleID != "user@example.com" || imported.CookieCount != 2 || !imported.Imported {
		t.Fatalf("unexpected import receipt: %+v", imported)
	}
	if strings.Contains(stdout, "super-secret-token") {
		t.Fatalf("import receipt leaked a cookie value: %q", stdout)
	}

	exportPath := filepath.Join(workDir, "exported.json")
	stdout, stderr = captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", exportPath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("export exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if !strings.Contains(stderr, "secret") {
		t.Fatalf("expected export to warn that the file is a credential, got stderr=%q", stderr)
	}

	var exported struct {
		Path        string `json:"path"`
		AppleID     string `json:"appleId"`
		CookieCount int    `json:"cookieCount"`
		Overwritten bool   `json:"overwritten"`
	}
	if err := json.Unmarshal([]byte(stdout), &exported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if exported.AppleID != "user@example.com" || exported.CookieCount != 2 || exported.Overwritten {
		t.Fatalf("unexpected export receipt: %+v", exported)
	}
	if strings.Contains(stdout, "super-secret-token") {
		t.Fatalf("export receipt leaked a cookie value: %q", stdout)
	}

	info, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("exported bundle mode = %v, want 0600", info.Mode().Perm())
	}

	raw, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var roundTripped struct {
		Kind    string `json:"kind"`
		Version int    `json:"version"`
		AppleID string `json:"appleId"`
		Cookies []struct {
			URL   string `json:"url"`
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal(bundle) error = %v; bundle=%q", err, raw)
	}
	if roundTripped.Kind != "asc-web-session" || roundTripped.Version != 1 {
		t.Fatalf("unexpected bundle envelope: %+v", roundTripped)
	}
	if roundTripped.AppleID != "user@example.com" || len(roundTripped.Cookies) != 2 {
		t.Fatalf("unexpected bundle payload: %+v", roundTripped)
	}
	values := map[string]string{}
	for _, cookie := range roundTripped.Cookies {
		values[cookie.Name] = cookie.Value
	}
	if values["myacinfo"] != "super-secret-token" || values["dslang"] != "US-EN" {
		t.Fatalf("exported cookies = %#v, want the imported values", values)
	}
}

func TestWebAuthImportFromEnvImportsCanonicalBundleOffline(t *testing.T) {
	cacheDir := isolateWebSessionCache(t)
	t.Setenv("ASC_WEB_SESSION", webSessionBundleFixture)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--from-env", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("environment import exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitSuccess, stdout, stderr)
	}
	var imported struct {
		Path        string `json:"path"`
		AppleID     string `json:"appleId"`
		CookieCount int    `json:"cookieCount"`
		Imported    bool   `json:"imported"`
	}
	if err := json.Unmarshal([]byte(stdout), &imported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if imported.Path != "ASC_WEB_SESSION" || imported.AppleID != "user@example.com" || imported.CookieCount != 2 || !imported.Imported {
		t.Fatalf("unexpected environment import receipt: %+v", imported)
	}
	if strings.Contains(stdout, "super-secret-token") || strings.Contains(stderr, "super-secret-token") {
		t.Fatalf("environment import leaked a cookie value: stdout=%q stderr=%q", stdout, stderr)
	}
	if entries, err := os.ReadDir(cacheDir); err != nil {
		t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
	} else if len(entries) == 0 {
		t.Fatalf("environment import did not persist a session in %q", cacheDir)
	}
}

func TestWebAuthImportFromEnvRejectsSourceConflictsAndEmptyInput(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		envValue   string
		want       string
		wantExit   int
		unexpected []string
	}{
		{
			name:       "both sources",
			args:       []string{"web", "auth", "import", "--file", filepath.Join("/does-not-exist", "session.json"), "--from-env", "--output", "json"},
			envValue:   webSessionBundleFixture,
			want:       "mutually exclusive",
			wantExit:   cmd.ExitUsage,
			unexpected: []string{"open --file", "super-secret-token"},
		},
		{
			name:     "unset",
			args:     []string{"web", "auth", "import", "--from-env", "--output", "json"},
			want:     "ASC_WEB_SESSION",
			wantExit: cmd.ExitUsage,
		},
		{
			name:       "requires explicit flag",
			args:       []string{"web", "auth", "import", "--output", "json"},
			envValue:   webSessionBundleFixture,
			want:       "one of --file or --from-env is required",
			wantExit:   cmd.ExitUsage,
			unexpected: []string{"super-secret-token"},
		},
		{
			name:     "blank",
			args:     []string{"web", "auth", "import", "--from-env", "--output", "json"},
			envValue: " \t\n ",
			want:     "ASC_WEB_SESSION",
			wantExit: cmd.ExitUsage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := isolateWebSessionCache(t)
			if tc.name == "unset" {
				t.Setenv("ASC_WEB_SESSION", "")
				if err := os.Unsetenv("ASC_WEB_SESSION"); err != nil {
					t.Fatalf("Unsetenv() error = %v", err)
				}
			} else {
				t.Setenv("ASC_WEB_SESSION", tc.envValue)
			}

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(tc.args, "1.0.0")
			})
			if code != tc.wantExit {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, tc.wantExit, stdout, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("expected %q on stderr, got %q", tc.want, stderr)
			}
			for _, unwanted := range tc.unexpected {
				if strings.Contains(stderr, unwanted) || strings.Contains(stdout, unwanted) {
					t.Fatalf("unexpected %q in command output: stdout=%q stderr=%q", unwanted, stdout, stderr)
				}
			}
			if entries, err := os.ReadDir(cacheDir); err != nil {
				t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
			} else if len(entries) != 0 {
				t.Fatalf("rejected environment import changed cache: %v", entries)
			}
		})
	}
}

func TestWebAuthImportFromEnvRejectsOversizedOrMalformedSecretWithoutCacheWrite(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		want     string
		unwanted string
	}{
		{name: "oversized", value: strings.Repeat("x", webcore.MaxSessionBundleSize+1), want: "exceeds", unwanted: strings.Repeat("x", 32)},
		{name: "unknown field", value: `{"kind":"asc-web-session","version":1,"appleId":"user@example.com","cookies":[{"url":"https://appstoreconnect.apple.com/","name":"myacinfo","value":"token"}],"super-secret-token":"value"}`, want: "invalid session bundle from ASC_WEB_SESSION", unwanted: "super-secret-token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "oversized" && runtime.GOOS == "windows" {
				t.Skip("Windows rejects a 1 MiB environment value before asc can apply its shared bundle ceiling")
			}
			cacheDir := isolateWebSessionCache(t)
			t.Setenv("ASC_WEB_SESSION", tc.value)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"web", "auth", "import", "--from-env", "--output", "json"}, "1.0.0")
			})
			if code != cmd.ExitError {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitError, stdout, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("expected %q on stderr, got %q", tc.want, stderr)
			}
			if strings.Contains(stdout, tc.unwanted) || strings.Contains(stderr, tc.unwanted) {
				t.Fatalf("environment payload leaked in output: stdout=%q stderr=%q", stdout, stderr)
			}
			if entries, err := os.ReadDir(cacheDir); err != nil {
				t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
			} else if len(entries) != 0 {
				t.Fatalf("rejected environment import changed cache: %v", entries)
			}
		})
	}
}

func TestWebAuthImportValidatePersistsAfterAppleCheck(t *testing.T) {
	cacheDir := isolateWebSessionCache(t)
	calls := 0
	sawCookie := false
	previousTransport := http.DefaultTransport
	http.DefaultTransport = webSessionInfoTransport{base: previousTransport, calls: &calls, sawCookie: &sawCookie}
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", webSessionBundleFixture)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"web", "auth", "import",
			"--file", bundlePath,
			"--validate",
			"--output", "json",
		}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("validated import exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitSuccess, stdout, stderr)
	}
	if calls != 1 || !sawCookie {
		t.Fatalf("session validation requests = %d, sawCookie = %t; want one request carrying the imported cookie", calls, sawCookie)
	}
	if !strings.Contains(strings.ToLower(stderr), "validation") {
		t.Fatalf("expected validated import diagnostic, got stderr=%q", stderr)
	}

	var imported struct {
		AppleID     string `json:"appleId"`
		CookieCount int    `json:"cookieCount"`
		Imported    bool   `json:"imported"`
	}
	if err := json.Unmarshal([]byte(stdout), &imported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if imported.AppleID != "user@example.com" || imported.CookieCount != 2 || !imported.Imported {
		t.Fatalf("unexpected validated import receipt: %+v", imported)
	}
	if strings.Contains(stdout, "super-secret-token") || strings.Contains(stderr, "super-secret-token") {
		t.Fatalf("validated import leaked a cookie value: stdout=%q stderr=%q", stdout, stderr)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("validated import did not persist a session in %q", cacheDir)
	}
}

func snapshotWebSessionCache(t *testing.T, cacheDir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}
		}
		t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
	}
	snapshot := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected directory %q in web session cache", entry.Name())
		}
		contents, err := os.ReadFile(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", entry.Name(), err)
		}
		snapshot[entry.Name()] = string(contents)
	}
	return snapshot
}

func TestWebAuthImportValidateRejectsAccountMismatchWithoutChangingCache(t *testing.T) {
	for _, source := range []string{"--file", "--from-env"} {
		t.Run(source, func(t *testing.T) {
			cacheDir := isolateWebSessionCache(t)
			bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", webSessionBundleFixture)

			var code int
			_, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
			})
			if code != cmd.ExitSuccess {
				t.Fatalf("seed import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
			}
			before := snapshotWebSessionCache(t, cacheDir)
			sourceArgs := []string{"--file", bundlePath}
			if source == "--from-env" {
				t.Setenv("ASC_WEB_SESSION", webSessionBundleFixture)
				sourceArgs = []string{"--from-env"}
			}

			calls := 0
			previousTransport := http.DefaultTransport
			http.DefaultTransport = webSessionInfoTransport{
				base:  previousTransport,
				calls: &calls,
				email: "other@example.com",
			}
			t.Cleanup(func() { http.DefaultTransport = previousTransport })

			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"web", "auth", "import"}, sourceArgs...)
				code = cmd.Run(append(args, "--validate", "--overwrite", "--output", "json"), "1.0.0")
			})
			if code != cmd.ExitError {
				t.Fatalf("mismatched validated import exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitError, stdout, stderr)
			}
			if calls != 1 {
				t.Fatalf("session validation requests = %d, want one request", calls)
			}
			if !strings.Contains(stderr, "live session validation") || !strings.Contains(stderr, "does not match") {
				t.Fatalf("expected a safe account mismatch diagnostic, got stderr=%q", stderr)
			}
			if strings.Contains(stdout, "super-secret-token") || strings.Contains(stderr, "super-secret-token") {
				t.Fatalf("mismatched validation leaked a cookie value: stdout=%q stderr=%q", stdout, stderr)
			}
			after := snapshotWebSessionCache(t, cacheDir)
			if len(after) != len(before) {
				t.Fatalf("validation failure changed cache entry count: before=%v after=%v", before, after)
			}
			for name, contents := range before {
				if got, ok := after[name]; !ok || got != contents {
					t.Fatalf("validation failure changed cache entry %q: before=%q after=%q", name, contents, got)
				}
			}
		})
	}
}

func TestWebAuthExportRefusesExistingFileWithoutOverwrite(t *testing.T) {
	isolateWebSessionCache(t)
	workDir := t.TempDir()
	bundlePath := writeWebSessionBundleFixture(t, workDir, "session.json", webSessionBundleFixture)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	exportPath := writeWebSessionBundleFixture(t, workDir, "exported.json", "existing contents\n")
	_, stderr = captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", exportPath, "--output", "json"}, "1.0.0")
	})
	if code == cmd.ExitSuccess {
		t.Fatalf("expected export to refuse an existing destination; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--overwrite") {
		t.Fatalf("expected --overwrite guidance, got stderr=%q", stderr)
	}
	if contents, err := os.ReadFile(exportPath); err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	} else if string(contents) != "existing contents\n" {
		t.Fatalf("refused export modified the destination: %q", contents)
	}

	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", exportPath, "--overwrite", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("export --overwrite exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	var exported struct {
		Overwritten bool `json:"overwritten"`
	}
	if err := json.Unmarshal([]byte(stdout), &exported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if !exported.Overwritten {
		t.Fatalf("expected overwritten=true receipt, got %q", stdout)
	}
}

func TestWebAuthExportPreservesWhitespaceInOutputPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("trailing whitespace in file names is not portable on Windows")
	}
	isolateWebSessionCache(t)
	workDir := t.TempDir()
	bundlePath := writeWebSessionBundleFixture(t, workDir, "session.json", webSessionBundleFixture)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	trimmedPath := filepath.Join(workDir, "exported.json")
	if err := os.WriteFile(trimmedPath, []byte("keep me\n"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	outPath := trimmedPath + " "
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", outPath, "--overwrite", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("export exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	if body, err := os.ReadFile(outPath); err != nil || len(body) == 0 {
		t.Fatalf("selected path %q content err=%v; want the exported bundle", outPath, err)
	}
	if body, err := os.ReadFile(trimmedPath); err != nil || string(body) != "keep me\n" {
		t.Fatalf("trimmed path %q content = %q, err = %v; must not be replaced", trimmedPath, body, err)
	}
	var exported struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &exported); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout)
	}
	if exported.Path != outPath {
		t.Fatalf("receipt path = %q, want the selected path %q unchanged", exported.Path, outPath)
	}
}

func TestWebAuthExportWithoutCachedSessionFails(t *testing.T) {
	isolateWebSessionCache(t)
	exportPath := filepath.Join(t.TempDir(), "exported.json")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", exportPath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitError, stdout, stderr)
	}
	if !strings.Contains(stderr, "no cached web session") {
		t.Fatalf("expected a missing-session error, got stderr=%q", stderr)
	}
	if _, err := os.Stat(exportPath); !os.IsNotExist(err) {
		t.Fatalf("expected no output file, Stat() error = %v", err)
	}
}

func TestWebAuthSessionTransferRequiresPathFlags(t *testing.T) {
	isolateWebSessionCache(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "export", args: []string{"web", "auth", "export", "--output", "json"}, want: "--output-path is required"},
		{name: "import", args: []string{"web", "auth", "import", "--output", "json"}, want: "one of --file or --from-env is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var code int
			_, stderr := captureOutput(t, func() {
				code = cmd.Run(tc.args, "1.0.0")
			})
			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("expected %q on stderr, got %q", tc.want, stderr)
			}
		})
	}
}

func TestWebAuthImportRejectsInvalidBundles(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		want     string
	}{
		{
			name:     "wrong kind",
			contents: `{"kind":"cookies.txt","version":1,"appleId":"user@example.com","cookies":[{"url":"https://appstoreconnect.apple.com/","name":"myacinfo","value":"t"}]}`,
			want:     "kind",
		},
		{
			name:     "unsupported version",
			contents: `{"kind":"asc-web-session","version":99,"appleId":"user@example.com","cookies":[{"url":"https://appstoreconnect.apple.com/","name":"myacinfo","value":"t"}]}`,
			want:     "version",
		},
		{
			name:     "unsupported origin",
			contents: `{"kind":"asc-web-session","version":1,"appleId":"user@example.com","cookies":[{"url":"https://evil.example.com/","name":"myacinfo","value":"t"}]}`,
			want:     "unsupported url",
		},
		{
			name:     "malformed json",
			contents: "not json",
			want:     "decode web session bundle",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := isolateWebSessionCache(t)
			bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", tc.contents)

			var code int
			_, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
			})
			if code != cmd.ExitError {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("expected %q on stderr, got %q", tc.want, stderr)
			}
			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				t.Fatalf("ReadDir() error = %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("refused import wrote to the session cache: %v", entries)
			}
		})
	}
}

func TestWebAuthImportReportsMissingFile(t *testing.T) {
	isolateWebSessionCache(t)
	missing := filepath.Join(t.TempDir(), "absent.json")

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", missing, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if !strings.Contains(stderr, "open --file") {
		t.Fatalf("expected an open failure for --file, got stderr=%q", stderr)
	}
}

func TestWebAuthImportRefusesExistingSessionWithoutOverwrite(t *testing.T) {
	isolateWebSessionCache(t)
	workDir := t.TempDir()
	bundlePath := writeWebSessionBundleFixture(t, workDir, "session.json", webSessionBundleFixture)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("first import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code == cmd.ExitSuccess {
		t.Fatalf("expected import to refuse an existing cached session; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--overwrite") {
		t.Fatalf("expected --overwrite guidance, got stderr=%q", stderr)
	}

	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--overwrite", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import --overwrite exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if strings.Contains(stdout, "super-secret-token") {
		t.Fatalf("import receipt leaked a cookie value: %q", stdout)
	}
}

func TestWebAuthImportOverwriteReplacesMalformedCache(t *testing.T) {
	cacheDir := isolateWebSessionCache(t)
	sum := sha256.Sum256([]byte("user@example.com"))
	malformedPath := filepath.Join(cacheDir, "session-"+hex.EncodeToString(sum[:])+".json")
	if err := os.WriteFile(malformedPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", webSessionBundleFixture)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code == cmd.ExitSuccess {
		t.Fatalf("expected import to refuse a malformed cached session; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--overwrite") {
		t.Fatalf("expected --overwrite guidance, got stderr=%q", stderr)
	}

	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--overwrite", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import --overwrite exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if strings.Contains(stdout, "super-secret-token") {
		t.Fatalf("import receipt leaked a cookie value: %q", stdout)
	}
}

func TestWebAuthImportRejectsAppleIDMismatch(t *testing.T) {
	isolateWebSessionCache(t)
	bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", webSessionBundleFixture)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"web", "auth", "import",
			"--file", bundlePath,
			"--apple-id", "other@example.com",
			"--output", "json",
		}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitError, stdout, stderr)
	}
	if !strings.Contains(stderr, "--apple-id") {
		t.Fatalf("expected an Apple Account mismatch error, got stderr=%q", stderr)
	}
	if strings.Contains(stderr, "super-secret-token") {
		t.Fatalf("mismatch error leaked a cookie value: %q", stderr)
	}
}

func TestWebAuthImportRejectsCookieDomainTheJarCannotStore(t *testing.T) {
	contents := `{
  "kind": "asc-web-session",
  "version": 1,
  "exportedAt": "2026-09-02T10:00:00Z",
  "appleId": "user@example.com",
  "cookies": [
    {"url": "https://appstoreconnect.apple.com/", "name": "myacinfo", "value": "super-secret-token", "domain": "evil.example"}
  ]
}
`
	cacheDir := isolateWebSessionCache(t)
	bundlePath := writeWebSessionBundleFixture(t, t.TempDir(), "session.json", contents)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if !strings.Contains(stderr, "domain") {
		t.Fatalf("expected a domain rejection, got stderr=%q", stderr)
	}
	if strings.Contains(stderr, "super-secret-token") {
		t.Fatalf("domain error leaked a cookie value: %q", stderr)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("refused import wrote to the session cache: %v", entries)
	}
}

func TestWebAuthExportTableOutputOmitsCookieValues(t *testing.T) {
	isolateWebSessionCache(t)
	workDir := t.TempDir()
	bundlePath := writeWebSessionBundleFixture(t, workDir, "session.json", webSessionBundleFixture)

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "import", "--file", bundlePath, "--output", "table"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("import exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	exportPath := filepath.Join(workDir, "exported.json")
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "export", "--output-path", exportPath, "--output", "table"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("export exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	for _, header := range []string{"Path", "Apple ID", "Cookies", "Overwritten"} {
		if !strings.Contains(stdout, header) {
			t.Fatalf("expected %q column in table output, got %q", header, stdout)
		}
	}
	if strings.Contains(stdout, "super-secret-token") {
		t.Fatalf("table output leaked a cookie value: %q", stdout)
	}
}
