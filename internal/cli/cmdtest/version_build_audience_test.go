package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// API fixture contains two builds in the version's train. Apple performs
// sorting/filtering before limit=1; without an audience filter build 46 wins.
func TestAppStoreLatestSelectionAudience(t *testing.T) {
	for _, mode := range []string{"attach", "attach-wait", "review-dry-run", "review-dry-run-wait"} {
		t.Run(mode, func(t *testing.T) {
			query := ""
			transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
				query = req.URL.Query().Get("filter[buildAudienceType]")
				if query == "APP_STORE_ELIGIBLE" {
					return `{"data":[{"type":"builds","id":"build-45","attributes":{"version":"45","processingState":"VALID","buildAudienceType":"APP_STORE_ELIGIBLE","uploadedDate":"2026-10-01T10:00:00Z"}}],"links":{}}`
				}
				return `{"data":[{"type":"builds","id":"build-46","attributes":{"version":"46","processingState":"VALID","buildAudienceType":"INTERNAL_ONLY","uploadedDate":"2026-10-01T11:00:00Z"}}],"links":{}}`
			}}
			transport.build = func(id string) string {
				return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"45","processingState":"VALID","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`
			}
			var flow http.RoundTripper = transport
			args := []string{"versions", "attach-build", "--app", "123456789", "--version-id", "version-1", "--latest", "--output", "json"}
			if strings.HasPrefix(mode, "review-dry-run") {
				args = []string{"review", "submit", "--app", "123456789", "--version-id", "version-1", "--latest", "--dry-run", "--output", "json"}
				flow = roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			}
			if strings.HasSuffix(mode, "-wait") {
				args = append(args, "--wait", "--timeout", "1s", "--poll-interval", "1ms")
			}
			code, stdout, stderr := runVersionBuildSelector(t, flow, args...)
			if code != rootcmd.ExitSuccess {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			// Verify filtering before limit=1 and the actual attachment target.
			if query != "APP_STORE_ELIGIBLE" {
				t.Errorf("App Store latest lookup audience=%q; want APP_STORE_ELIGIBLE before selecting limit=1", query)
			}
			var result struct {
				BuildID         string `json:"buildId"`
				BuildAttachment struct {
					BuildID     string `json:"buildId"`
					WouldAttach bool   `json:"wouldAttach"`
				} `json:"buildAttachment"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode: %v stdout=%q", err, stdout)
			}
			if result.BuildID != "build-45" {
				t.Errorf("selected %q; newer build-46 is INTERNAL_ONLY, want eligible build-45", result.BuildID)
			}
			if strings.HasPrefix(mode, "attach") {
				if len(transport.attached) != 1 || transport.attached[0] != "build-45" {
					t.Errorf("PATCH target=%v; want [build-45]", transport.attached)
				}
			} else {
				if result.BuildAttachment.BuildID != "build-45" || !result.BuildAttachment.WouldAttach {
					t.Errorf("dry-run attachment=%+v; want eligible build-45", result.BuildAttachment)
				}
				if len(transport.attached) != 0 {
					t.Errorf("dry-run mutated: %v", transport.attached)
				}
			}
		})
	}
}

// An exact numbered selector has no fallback: report the audience problem
// rather than silently attaching another number or sending a known-ineligible ID.
func TestAppStoreNumberInternalOnly(t *testing.T) {
	for _, mode := range []string{"number", "number-wait", "id-wait"} {
		t.Run(mode, func(t *testing.T) {
			transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
				if req.URL.Query().Get("filter[version]") != "46" {
					t.Fatalf("wrong build-number query: %s", req.URL.RawQuery)
				}
				return `{"data":[{"type":"builds","id":"build-46","attributes":{"version":"46","processingState":"VALID","buildAudienceType":"INTERNAL_ONLY"}}],"links":{}}`
			}, build: func(id string) string {
				return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"46","processingState":"VALID","buildAudienceType":"INTERNAL_ONLY"}}}`
			}}
			args := []string{"versions", "attach-build", "--app", "123456789", "--version-id", "version-1", "--output", "json"}
			if mode == "id-wait" {
				args = append(args, "--build-id", "build-46")
			} else {
				args = append(args, "--build-number", "46")
			}
			if strings.HasSuffix(mode, "-wait") {
				args = append(args, "--wait", "--timeout", "1s", "--poll-interval", "1ms")
			}
			code, _, stderr := runVersionBuildSelector(t, transport, args...)
			if code != rootcmd.ExitError {
				t.Errorf("known INTERNAL_ONLY build exit=%d; stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "INTERNAL_ONLY") {
				t.Errorf("missing explicit audience diagnostic: %q", stderr)
			}
			if len(transport.attached) != 0 {
				t.Errorf("known-ineligible PATCH target=%v", transport.attached)
			}
		})
	}
}

func TestGenericBuildWaitKeepsInternalOnlyBuilds(t *testing.T) {
	transport := &versionTrainTransport{t: t, platform: "IOS", builds: func(req *http.Request) string {
		if audience := req.URL.Query().Get("filter[buildAudienceType]"); audience != "" {
			t.Fatalf("generic wait constrained audience: %q", audience)
		}
		return `{"data":[{"type":"builds","id":"build-46","attributes":{"version":"46","processingState":"VALID","buildAudienceType":"INTERNAL_ONLY"}}],"links":{}}`
	}, build: func(id string) string {
		return `{"data":{"type":"builds","id":"` + id + `","attributes":{"version":"46","processingState":"VALID","buildAudienceType":"INTERNAL_ONLY"}}}`
	}}
	code, stdout, stderr := runVersionBuildSelector(t, transport, "builds", "wait", "--app", "123456789", "--version", "1.2.0", "--platform", "IOS", "--latest", "--timeout", "1s", "--poll-interval", "1ms", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("generic wait exit=%d stderr=%q", code, stderr)
	}
	var result struct {
		BuildID string `json:"buildId"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.BuildID != "build-46" || len(transport.attached) != 0 {
		t.Fatalf("generic wait result=%+v attached=%v", result, transport.attached)
	}
}
