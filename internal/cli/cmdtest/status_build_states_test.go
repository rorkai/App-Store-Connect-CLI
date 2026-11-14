package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// installStatusBuildStateTransport serves a latest build that is expired and
// still processing for internal testers while an older build is the only one
// distributed externally.
func installStatusBuildStateTransport(t *testing.T) *asctest.Registry {
	t.Helper()

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"builds",
				"id":"build-2",
				"attributes":{
					"version":"45",
					"uploadedDate":"2026-02-20T00:00:00Z",
					"processingState":"VALID",
					"expired":true
				},
				"relationships":{
					"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
					"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-2"}}
				}
			},
			{
				"type":"builds",
				"id":"build-1",
				"attributes":{
					"version":"44",
					"uploadedDate":"2026-02-19T00:00:00Z",
					"processingState":"VALID",
					"expired":false
				},
				"relationships":{
					"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
					"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-1"}}
				}
			}
		],
		"included":[
			{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}},
			{"type":"buildBetaDetails","id":"bbd-1","attributes":{"internalBuildState":"IN_BETA_TESTING","externalBuildState":"IN_BETA_TESTING"}},
			{"type":"buildBetaDetails","id":"bbd-2","attributes":{"internalBuildState":"PROCESSING","externalBuildState":"NOT_READY_FOR_TESTING"}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-2,build-1", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	return reg
}

func assertStatusBuildStateRequestCounts(t *testing.T, reg *asctest.Registry) {
	t.Helper()

	if got := countStatusRequests(reg, "/v1/builds"); got != 2 {
		t.Fatalf("expected 2 build requests (snapshot plus active beta review scan), got %d", got)
	}
	if got := countStatusRequests(reg, "/v1/buildBetaDetails"); got != 0 {
		t.Fatalf("expected no build beta details requests (states come from included buildBetaDetail), got %d", got)
	}
	if got := countStatusRequests(reg, "/v1/betaAppReviewSubmissions"); got != 1 {
		t.Fatalf("expected 1 beta app review submissions request, got %d", got)
	}
}

func TestStatusJSONSurfacesLatestBuildInternalStateAndExpiry(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := installStatusBuildStateTransport(t)

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "6748252780", "--include", "builds,testflight", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Builds struct {
			Latest struct {
				ID              string `json:"id"`
				ProcessingState string `json:"processingState"`
				Expired         bool   `json:"expired"`
			} `json:"latest"`
		} `json:"builds"`
		TestFlight struct {
			LatestDistributedBuildID string `json:"latestDistributedBuildId"`
			InternalBuildState       string `json:"internalBuildState"`
			ExternalBuildState       string `json:"externalBuildState"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload.Builds.Latest.ID != "build-2" || payload.Builds.Latest.ProcessingState != "VALID" {
		t.Fatalf("expected latest build-2 reported as VALID, got %+v", payload.Builds.Latest)
	}
	if !payload.Builds.Latest.Expired {
		t.Fatalf("expected builds.latest.expired=true for an expired VALID build, got %+v", payload.Builds.Latest)
	}
	if payload.TestFlight.InternalBuildState != "PROCESSING" {
		t.Fatalf("expected internalBuildState of the latest build, got %+v", payload.TestFlight)
	}
	if payload.TestFlight.LatestDistributedBuildID != "build-1" || payload.TestFlight.ExternalBuildState != "IN_BETA_TESTING" {
		t.Fatalf("expected external distribution fields to keep describing build-1, got %+v", payload.TestFlight)
	}

	assertStatusBuildStateRequestCounts(t, reg)
}

func TestStatusTableSurfacesLatestBuildInternalStateAndExpiry(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := installStatusBuildStateTransport(t)

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "6748252780", "--include", "builds,testflight", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	for _, want := range []string{
		"latest.expired",
		"[x] true",
		"internalBuildState",
		"[~] PROCESSING",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output containing %q:\n%s", want, stdout)
		}
	}

	assertStatusBuildStateRequestCounts(t, reg)
}
