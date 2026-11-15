package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// versionTrainTransport serves one App Store version (version-1, versionString
// 1.2.0) and its pre-release train prv-1, and hands /v1/builds lookups to
// builds after checking they are scoped to that train.
type versionTrainTransport struct {
	t        *testing.T
	platform string
	builds   func(req *http.Request) string
	build    func(id string) string

	mu       sync.Mutex
	attached []string
	requests []string
}

func (tr *versionTrainTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.mu.Lock()
	tr.requests = append(tr.requests, req.Method+" "+req.URL.Path)
	tr.mu.Unlock()

	query := req.URL.Query()
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
		if query.Get("filter[versionString]") != "1.2.0" || query.Get("filter[platform]") != tr.platform {
			tr.t.Fatalf("unexpected version lookup: %s", req.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"`+tr.platform+`"}}]}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
		return jsonResponse(http.StatusOK, `{
			"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"`+tr.platform+`","versionString":"1.2.0"},
				"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}},
			"included":[{"type":"apps","id":"123456789","attributes":{"bundleId":"com.example.app","name":"Example"}}]
		}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
		if query.Get("filter[app]") != "123456789" || query.Get("filter[version]") != "1.2.0" || query.Get("filter[platform]") != tr.platform {
			tr.t.Fatalf("pre-release lookup must be scoped to the version train, got %s", req.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"`+tr.platform+`"}}],"links":{}}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
		if query.Get("filter[app]") != "123456789" || query.Get("filter[preReleaseVersion]") != "prv-1" {
			tr.t.Fatalf("build lookup must be scoped to the version train, got %s", req.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, tr.builds(req))
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/builds/") && tr.build != nil:
		return jsonResponse(http.StatusOK, tr.build(strings.TrimPrefix(req.URL.Path, "/v1/builds/")))
	case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
		body, _ := io.ReadAll(req.Body)
		var payload struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			tr.t.Fatalf("decode attach body: %v", err)
		}
		tr.mu.Lock()
		tr.attached = append(tr.attached, payload.Data.ID)
		tr.mu.Unlock()
		return jsonResponse(http.StatusNoContent, "")
	default:
		tr.t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		return nil, nil
	}
}

func runVersionBuildSelector(t *testing.T, transport http.RoundTripper, args ...string) (int, string, string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installDefaultTransport(t, transport)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(args, "1.2.3")
	})
	return code, stdout, stderr
}

func buildsCollection(id, number, state string) string {
	return `{"data":[{"type":"builds","id":"` + id + `","attributes":{"version":"` + number + `","processingState":"` + state + `","uploadedDate":"2026-10-01T10:00:00Z"}}],"links":{}}`
}

func buildResource(id, number, state string) string {
	return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"` + number + `","processingState":"` + state + `"}}}`
}

func TestVersionsAttachBuildByBuildNumberResolvesVersionAndBuild(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
		if got := req.URL.Query().Get("filter[version]"); got != "45" {
			t.Fatalf("expected filter[version]=45, got %q", got)
		}
		return buildsCollection("build-45", "45", "VALID")
	}}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--build-number", "45", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if stdout != `{"versionId":"version-1","buildId":"build-45","attached":true}`+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "Selected build 45 (build-45) for version 1.2.0 on IOS") {
		t.Fatalf("expected selection note on stderr, got %q", stderr)
	}
	if len(transport.attached) != 1 || transport.attached[0] != "build-45" {
		t.Fatalf("attached = %v, want [build-45]", transport.attached)
	}
}

func TestVersionsAttachBuildLatestUsesTrainOfVersionID(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "MAC_OS", builds: func(req *http.Request) string {
		query := req.URL.Query()
		if query.Get("sort") != "-uploadedDate" || query.Get("limit") != "1" {
			t.Fatalf("expected newest-build query, got %s", req.URL.RawQuery)
		}
		return buildsCollection("build-46", "46", "VALID")
	}}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--app", "123456789", "--version-id", "version-1", "--latest", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-46"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if len(transport.attached) != 1 || transport.attached[0] != "build-46" {
		t.Fatalf("attached = %v, want [build-46]", transport.attached)
	}
}

func TestVersionsAttachBuildWaitsForProcessingBeforeAttaching(t *testing.T) {
	polls := 0
	var transport *versionTrainTransport
	transport = &versionTrainTransport{
		t:        t,
		platform: "IOS",
		builds: func(*http.Request) string {
			return buildsCollection("build-46", "46", "PROCESSING")
		},
		build: func(id string) string {
			if len(transport.attached) > 0 {
				t.Fatal("build attached before processing finished")
			}
			polls++
			if polls < 3 {
				return buildResource(id, "46", "PROCESSING")
			}
			return buildResource(id, "46", "VALID")
		},
	}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--build-number", "46",
		"--wait", "--poll-interval", "1ms", "--timeout", "5s", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-46"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-46... (PROCESSING") {
		t.Fatalf("expected processing progress on stderr, got %q", stderr)
	}
	if len(transport.attached) != 1 || transport.attached[0] != "build-46" {
		t.Fatalf("attached = %v, want [build-46]", transport.attached)
	}
}

func TestVersionsAttachBuildWaitStopsOnFailedProcessing(t *testing.T) {
	transport := &versionTrainTransport{
		t:        t,
		platform: "IOS",
		builds:   func(*http.Request) string { return buildsCollection("build-46", "46", "PROCESSING") },
		build:    func(id string) string { return buildResource(id, "46", "INVALID") },
	}
	// Failure details are looked up best effort; answer them with nothing.
	wrapped := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("include") == "buildUpload" || strings.HasSuffix(req.URL.Path, "/buildUploads") || strings.HasSuffix(req.URL.Path, "/app") {
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		}
		return transport.RoundTrip(req)
	})

	code, _, stderr := runVersionBuildSelector(t, wrapped,
		"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--build-number", "46",
		"--wait", "--poll-interval", "1ms", "--timeout", "5s")

	if code == rootcmd.ExitSuccess {
		t.Fatalf("expected failure for an INVALID build; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "build processing failed with state INVALID") {
		t.Fatalf("expected processing failure on stderr, got %q", stderr)
	}
	if len(transport.attached) != 0 {
		t.Fatalf("attached = %v, want nothing attached", transport.attached)
	}
}

func TestVersionsAttachBuildWithoutWaitRejectsProcessingBuild(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(*http.Request) string {
		return buildsCollection("build-46", "46", "PROCESSING")
	}}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--latest")

	if code == rootcmd.ExitSuccess {
		t.Fatal("expected failure for a build that is still processing")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "build 46 (build-46) is still processing; add --wait") {
		t.Fatalf("expected --wait guidance on stderr, got %q", stderr)
	}
	if len(transport.attached) != 0 {
		t.Fatalf("attached = %v, want nothing attached", transport.attached)
	}
}

func TestVersionsAttachBuildByIDOnlyAttaches(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS"}
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	// A default app must not turn the ID-only form into a lookup.
	t.Setenv("ASC_APP_ID", "123456789")
	installDefaultTransport(t, transport)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1", "--output", "json"}, "1.2.3")
	})

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if stdout != `{"versionId":"version-1","buildId":"build-1","attached":true}`+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if got := strings.Join(transport.requests, ","); got != "PATCH /v1/appStoreVersions/version-1/relationships/build" {
		t.Fatalf("requests = %s, want only the attach request", got)
	}
}

func TestVersionBuildSelectorValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "attach-build build id and latest",
			args:    []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1", "--latest"},
			wantErr: "Error: --build-id, --build-number, and --latest are mutually exclusive",
		},
		{
			name:    "attach-build version and version id",
			args:    []string{"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--version-id", "version-1", "--build-id", "build-1"},
			wantErr: "Error: --version and --version-id are mutually exclusive",
		},
		{
			name:    "attach-build latest without app",
			args:    []string{"versions", "attach-build", "--version-id", "version-1", "--latest"},
			wantErr: "Error: --app is required with --version, --platform, --build-number, or --latest (or set ASC_APP_ID)",
		},
		{
			name:    "attach-build version without app",
			args:    []string{"versions", "attach-build", "--version", "1.2.0", "--build-id", "build-1"},
			wantErr: "Error: --app is required with --version, --platform, --build-number, or --latest (or set ASC_APP_ID)",
		},
		{
			name:    "attach-build timeout without wait",
			args:    []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1", "--timeout", "1m"},
			wantErr: "Error: --timeout and --poll-interval require --wait",
		},
		{
			name:    "review submit build id and latest",
			args:    []string{"review", "submit", "--app", "123456789", "--version", "1.2.0", "--build-id", "build-1", "--latest", "--confirm"},
			wantErr: "Error: --build-id, --build-number, and --latest are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupUsageExitCodeEnv(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("validation must fail before HTTP: %s %s", req.Method, req.URL.String())
				return nil, errors.New("unexpected request")
			}))

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want containing %q", stderr, test.wantErr)
			}
		})
	}
}

func TestReviewSubmitBuildNumberSelectsBuildFromVersionTrain(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
		if got := req.URL.Query().Get("filter[version]"); got != "45" {
			t.Fatalf("expected filter[version]=45, got %q", got)
		}
		return buildsCollection("build-45", "45", "VALID")
	}}
	reviewFlow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions" && isReleasedVersionStateQuery(req.URL.Query()):
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appInfos":
			return jsonResponse(http.StatusOK, submitPreflightAppInfosJSON("123456789", completeAgeRatingAttributes))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && (req.URL.Path == "/v1/appStoreVersions/version-1/build" || req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission"):
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return transport.RoundTrip(req)
	})

	code, stdout, stderr := runVersionBuildSelector(t, reviewFlow,
		"review", "submit", "--app", "123456789", "--version-id", "version-1", "--build-number", "45", "--dry-run", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	var payload struct {
		BuildID         string `json:"buildId"`
		BuildAttachment struct {
			BuildID     string `json:"buildId"`
			WouldAttach bool   `json:"wouldAttach"`
		} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload.BuildID != "build-45" || payload.BuildAttachment.BuildID != "build-45" || !payload.BuildAttachment.WouldAttach {
		t.Fatalf("payload = %+v, want build-45 selected for attachment", payload)
	}
	if !strings.Contains(stderr, "Selected build 45 (build-45) for version 1.2.0 on IOS") {
		t.Fatalf("expected selection note on stderr, got %q", stderr)
	}
}
