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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
	case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/reviewSubmissions" && (query.Get("filter[state]") == "UNRESOLVED_ISSUES" || query.Get("filter[state]") == "READY_FOR_REVIEW"):
		return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/123456789/reviewSubmissions"}}`)
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
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/builds/") && query.Get("include") != "":
		return jsonResponse(http.StatusOK, buildScopeProof(strings.TrimPrefix(req.URL.Path, "/v1/builds/"), tr.platform))
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
		code = runWithDepsOnFakeClock(t, shared.DefaultDeps(), "1.2.3", args...)
	})
	return code, stdout, stderr
}

func buildsCollection(id, number, state string) string {
	return `{"data":[{"type":"builds","id":"` + id + `","attributes":{"version":"` + number + `","processingState":"` + state + `","uploadedDate":"2026-10-01T10:00:00Z"}}],"links":{}}`
}

func buildResource(id, number, state string) string {
	return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"` + number + `","processingState":"` + state + `"}}}`
}

func buildScopeProof(id, platform string) string {
	buildNumber := "46"
	if strings.HasSuffix(id, "45") {
		buildNumber = "45"
	}
	return buildScopeProofWithScope(id, "123456789", "prv-1", "1.2.0", platform, buildNumber)
}

func buildScopeProofWithScope(id, appID, trainID, version, platform, buildNumber string) string {
	return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"` + buildNumber + `","processingState":"VALID"},"relationships":{"app":{"data":{"type":"apps","id":"` + appID + `"}},"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"` + trainID + `"}}}},"included":[{"type":"preReleaseVersions","id":"` + trainID + `","attributes":{"version":"` + version + `","platform":"` + platform + `"}}]}`
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

func TestVersionsAttachBuildLatestDerivesScopeFromVersionIDWithoutApp(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "MAC_OS", builds: func(req *http.Request) string {
		query := req.URL.Query()
		if query.Get("sort") != "-uploadedDate" || query.Get("limit") != "1" {
			t.Fatalf("expected newest-build query, got %s", req.URL.RawQuery)
		}
		return buildsCollection("build-46", "46", "VALID")
	}}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--version-id", "version-1", "--platform", "MAC_OS", "--latest", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"versionId":"version-1"`) || !strings.Contains(stdout, `"buildId":"build-46"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if len(transport.attached) != 1 || transport.attached[0] != "build-46" {
		t.Fatalf("attached = %v, want [build-46]", transport.attached)
	}
}

func TestVersionsAttachBuildNumberDerivesScopeFromVersionIDWithoutApp(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
		if req.URL.Query().Get("filter[version]") != "45" {
			t.Fatalf("expected filter[version]=45, got %q", req.URL.Query().Get("filter[version]"))
		}
		return buildsCollection("build-45", "45", "VALID")
	}}

	code, stdout, stderr := runVersionBuildSelector(t, transport,
		"versions", "attach-build", "--version-id", "version-1", "--build-number", "45", "--output", "json")

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want success; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"versionId":"version-1"`) || !strings.Contains(stdout, `"buildId":"build-45"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if len(transport.attached) != 1 || transport.attached[0] != "build-45" {
		t.Fatalf("attached = %v, want [build-45]", transport.attached)
	}
}

func TestVersionsAttachBuildVersionIDRejectsRequestedPlatformMismatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1" {
			t.Fatalf("platform refusal sent follow-up request: %s %s", req.Method, req.URL.RequestURI())
		}
		requests++
		return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--platform", "MAC_OS", "--latest", "--output", "json")
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, `is on platform "IOS", not "MAC_OS"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if requests != 1 {
		t.Fatalf("platform mismatch requests=%d, want one version proof", requests)
	}
}

func TestVersionsAttachBuildVersionIDRejectsInvalidRequestedPlatformBeforeClient(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("invalid platform sent provider request: %s %s", req.Method, req.URL.RequestURI())
		return nil, nil
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--platform", "WATCH_OS", "--latest", "--output", "json")
	if code != rootcmd.ExitUsage || stdout != "" || !strings.Contains(stderr, "platform must be one of") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestVersionsAttachBuildAllowsSameCollectionContinuation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	preReleaseRequests := 0
	requests := []string{}
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			preReleaseRequests++
			if preReleaseRequests == 1 {
				return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/preReleaseVersions?cursor=opaque"}}`)
			}
			if req.URL.Query().Get("cursor") != "opaque" {
				t.Fatalf("continuation query = %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" || query.Get("filter[preReleaseVersion]") != "prv-1" || query.Get("filter[buildAudienceType]") != "APP_STORE_ELIGIBLE" {
				t.Fatalf("build scope = %s", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, buildsCollection("build-46", "46", "VALID"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-46":
			return jsonResponse(http.StatusOK, buildScopeProof("build-46", "IOS"))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code != rootcmd.ExitSuccess || stdout == "" || stderr == "" || len(requests) != 6 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%v", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildRejectsPreReleaseContinuationDriftBeforeWrite(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/preReleaseVersions?filter%5Bapp%5D=987654321&cursor=foreign"}}`)
		default:
			t.Fatalf("drift continuation sent request %d: %s %s", requests, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code == rootcmd.ExitSuccess || stdout != "" || !strings.Contains(stderr, "rejected scoped pagination URL query") || requests != 2 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildRejectsOffCollectionContinuationBeforeWrite(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/builds?cursor=off-collection"}}`)
		default:
			t.Fatalf("off-collection continuation sent request %d: %s %s", requests, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code == rootcmd.ExitSuccess || stdout != "" || !strings.Contains(stderr, "rejected scoped pagination URL path") || requests != 2 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildNumberAllowsSameCollectionBuildContinuation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	buildRequests := 0
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			buildRequests++
			if buildRequests == 1 {
				if req.URL.Query().Get("filter[version]") != "45" || req.URL.Query().Get("filter[preReleaseVersion]") != "prv-1" {
					t.Fatalf("build scope = %s", req.URL.RawQuery)
				}
				return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/builds?cursor=opaque"}}`)
			}
			if req.URL.Query().Get("cursor") != "opaque" {
				t.Fatalf("build continuation query = %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, buildsCollection("build-45", "45", "VALID"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-45":
			return jsonResponse(http.StatusOK, buildScopeProof("build-45", "IOS"))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--build-number", "45", "--output", "json")
	if code != rootcmd.ExitSuccess || stdout == "" || stderr == "" || requests != 6 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildRejectsBuildContinuationTrainDriftBeforeWrite(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
		case 3:
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/builds?filter%5BpreReleaseVersion%5D=prv-foreign&cursor=foreign"}}`)
		default:
			t.Fatalf("build drift continuation sent request %d: %s %s", requests, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--build-number", "45", "--output", "json")
	if code == rootcmd.ExitSuccess || stdout != "" || !strings.Contains(stderr, "rejected scoped pagination URL query") || requests != 3 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildRejectsForeignBuildScopeBeforeWrite(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "foreign app",
			body: buildScopeProofWithScope("build-46", "987654321", "prv-1", "1.2.0", "IOS", "46"),
			want: `belongs to app "987654321", not "123456789"`,
		},
		{
			name: "foreign marketing train",
			body: buildScopeProofWithScope("build-46", "123456789", "prv-foreign", "9.9.9", "IOS", "46"),
			want: `belongs to marketing train "prv-foreign", not version "1.2.0"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			requests := 0
			flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
					return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
					return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
					return jsonResponse(http.StatusOK, buildsCollection("build-46", "46", "VALID"))
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-46":
					return jsonResponse(http.StatusOK, test.body)
				default:
					t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
					return nil, nil
				}
			})
			stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
			if code == rootcmd.ExitSuccess || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want %q", code, stdout, stderr, test.want)
			}
			if requests != 4 {
				t.Fatalf("requests=%d, want version, train, build, and final proof only", requests)
			}
		})
	}
}

func TestVersionsAttachBuildAcceptsEquivalentMarketingVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, buildsCollection("build-46", "46", "VALID"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-46":
			return jsonResponse(http.StatusOK, buildScopeProofWithScope("build-46", "123456789", "prv-1", "1.2", "IOS", "46"))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code != rootcmd.ExitSuccess || stdout == "" || stderr == "" || requests != 5 {
		t.Fatalf("exit=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, requests)
	}
}

func TestVersionsAttachBuildConfiguredBundleUsesStrictEffectiveApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "com.example.owner")
	train := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
		return buildsCollection("build-45", "45", "VALID")
	}}
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps" {
			if req.URL.Query().Get("filter[bundleId]") != "com.example.owner" {
				t.Fatalf("bundle lookup query = %s", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"123456789","attributes":{"bundleId":"com.example.owner"}}]}`)
		}
		return train.RoundTrip(req)
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code != rootcmd.ExitSuccess || !strings.Contains(stdout, `"buildId":"build-45"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestVersionsAttachBuildConfiguredBundleDriftStopsBeforeVersionProof(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "com.example.owner")
	requests := 0
	flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
			t.Fatalf("configured bundle drift sent follow-up request: %s %s", req.Method, req.URL.RequestURI())
		}
		requests++
		return jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"other-app","attributes":{"bundleId":"com.other.app","name":"com.example.owner"}}]}`)
	})
	stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--version-id", "version-1", "--latest", "--output", "json")
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, `no app found for bundle ID "com.example.owner" (exact match required)`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if requests != 1 {
		t.Fatalf("configured bundle drift requests=%d, want one app lookup", requests)
	}
}

func TestVersionsAttachBuildVersionIDScopeRejectsForeignAndMalformedResources(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "foreign app",
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"987654321"}}}}}`,
			want: `belongs to app "987654321", not "123456789"`,
		},
		{
			name: "wrong resource type",
			body: `{"data":{"type":"appInfos","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `expected resource type "appStoreVersions", got "appInfos"`,
		},
		{
			name: "wrong resource ID",
			body: `{"data":{"type":"appStoreVersions","id":"version-other","attributes":{"versionString":"1.2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `expected app store version "version-1", got "version-other"`,
		},
		{
			name: "missing app relationship",
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"IOS"}}}`,
			want: `app relationship missing for app store version "version-1"`,
		},
		{
			name: "invalid platform",
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.0","platform":"WATCH_OS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `has invalid platform`,
		},
		{
			name: "missing version string",
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `has no version string`,
		},
		{
			name: "invalid version string",
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"release-candidate","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `has invalid version string`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			requests := 0
			flow := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1" {
					t.Fatalf("scope refusal sent follow-up request: %s %s", req.Method, req.URL.RequestURI())
				}
				requests++
				return jsonResponse(http.StatusOK, test.body)
			})
			stdout, stderr, code := runWithDeps(t, stubClient(t, flow), "versions", "attach-build", "--app", "123456789", "--version-id", "version-1", "--latest", "--output", "json")
			if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want %q", code, stdout, stderr, test.want)
			}
			if requests != 1 {
				t.Fatalf("version scope requests=%d, want exactly one", requests)
			}
		})
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
		deps    bool
	}{
		{
			name:    "attach-build build id and latest",
			args:    []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1", "--latest"},
			wantErr: "Error: --build-id, --build-number, and --latest are mutually exclusive",
			deps:    true,
		},
		{
			name:    "attach-build version and version id",
			args:    []string{"versions", "attach-build", "--app", "123456789", "--version", "1.2.0", "--version-id", "version-1", "--build-id", "build-1"},
			wantErr: "Error: --version and --version-id are mutually exclusive",
			deps:    true,
		},
		{
			name:    "attach-build version without app",
			args:    []string{"versions", "attach-build", "--version", "1.2.0", "--build-id", "build-1"},
			wantErr: "Error: --app is required with --version, --platform, --build-number, or --latest unless --version-id is provided (or set ASC_APP_ID)",
			deps:    true,
		},
		{
			name:    "attach-build timeout without wait",
			args:    []string{"versions", "attach-build", "--version-id", "version-1", "--build-id", "build-1", "--timeout", "1m"},
			wantErr: "Error: --timeout and --poll-interval require --wait",
			deps:    true,
		},
		{
			name:    "review submit build id and latest",
			deps:    true,
			args:    []string{"review", "submit", "--app", "123456789", "--version", "1.2.0", "--build-id", "build-1", "--latest", "--confirm"},
			wantErr: "Error: --build-id, --build-number, and --latest are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupUsageExitCodeEnv(t)
			failOnRequest := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("validation must fail before HTTP: %s %s", req.Method, req.URL.String())
				return nil, errors.New("unexpected request")
			})

			var code int
			var stdout, stderr string
			if test.deps {
				stdout, stderr, code = runWithDeps(t, stubClient(t, failOnRequest), test.args...)
			} else {
				installDefaultTransport(t, failOnRequest)
				stdout, stderr = captureOutput(t, func() {
					code = rootcmd.Run(test.args, "1.2.3")
				})
			}
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

	stdout, stderr, code := runWithDeps(t, stubClient(t, reviewFlow),
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
