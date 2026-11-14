package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestStatusUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "RequiresAppID",
			args:    []string{"status"},
			wantErr: "Error: --app is required (or set ASC_APP_ID)",
		},
		{
			name:    "RejectsUnknownPlatform",
			args:    []string{"status", "--app", "app-1", "--platform", "IPAD_OS"},
			wantErr: "--platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS",
		},
		{
			name:    "RejectsUnknownIncludeSection",
			args:    []string{"status", "--app", "app-1", "--include", "builds,unknown"},
			wantErr: "--include contains unsupported section",
		},
	})
}

func TestStatusDefaultJSONIncludesAllSections(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type":"apps",
			"id":"app-1",
			"attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"my-app-sku"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-2",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
			},
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"IN_REVIEW","platform":"IOS","submittedDate":"2026-02-19T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data": [
			{
				"type":"builds",
				"id":"build-2",
				"attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"},
				"relationships":{"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-2"}}}
			},
			{
				"type":"builds",
				"id":"build-1",
				"attributes":{"version":"44","uploadedDate":"2026-02-19T00:00:00Z","processingState":"VALID"},
				"relationships":{"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-1"}}}
			}
		],
		"included": [
			{"type":"buildBetaDetails","id":"bbd-2","attributes":{"externalBuildState":"IN_BETA_TESTING"}},
			{"type":"buildBetaDetails","id":"bbd-1","attributes":{"externalBuildState":"NOT_READY_FOR_TESTING"}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-2/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-2,build-1", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"betaAppReviewSubmissions",
				"id":"beta-sub-1",
				"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-02-20T01:00:00Z"},
				"relationships":{"build":{"data":{"type":"builds","id":"build-2"}}}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-2",
			"attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-2"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-2",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"READY_FOR_SALE",
					"createdDate":"2026-02-20T02:00:00Z"
				}
			},
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.2",
					"appVersionState":"WAITING_FOR_REVIEW",
					"createdDate":"2026-02-10T02:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2/appStoreVersionPhasedRelease"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersionPhasedReleases",
			"id":"phase-1",
			"attributes":{
				"phasedReleaseState":"ACTIVE",
				"startDate":"2026-02-20",
				"totalPauseDuration":0,
				"currentDayNumber":3
			}
		}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if _, ok := payload["app"]; !ok {
		t.Fatalf("expected app section, got %v", payload)
	}
	summary, ok := payload["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary object, got %T", payload["summary"])
	}
	if summary["health"] == "" {
		t.Fatalf("expected summary.health, got %v", summary)
	}
	if summary["nextAction"] == "" {
		t.Fatalf("expected summary.nextAction, got %v", summary)
	}
	for _, key := range []string{"builds", "testflight", "appstore", "submission", "review", "phasedRelease", "links"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected %s section in payload, got %v", key, payload)
		}
	}
}

func TestStatusNullPhasedReleaseIsUnconfigured(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"READY_FOR_SALE"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionPhasedRelease"), asctest.JSON(http.StatusOK, `{"data":null,"links":{}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "phased-release", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if phasedReads := countStatusRequests(reg, "/v1/appStoreVersions/ver-1/appStoreVersionPhasedRelease"); stderr != "" || phasedReads != 1 {
		t.Fatalf("stderr = %q, phased release reads = %d; want empty stderr and one read", stderr, phasedReads)
	}
	var payload struct {
		PhasedRelease map[string]any `json:"phasedRelease"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload.PhasedRelease) != 1 || payload.PhasedRelease["configured"] != false {
		t.Fatalf("phasedRelease = %v, want only configured:false", payload.PhasedRelease)
	}
}

func TestStatusCorrelatesOlderBetaReviewSubmissionWithItsActualBuild(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"builds",
				"id":"build-326",
				"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			},
			{
				"type":"builds",
				"id":"build-325",
				"attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			}
		],
		"included":[
			{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-326,build-325", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"betaAppReviewSubmissions",
			"id":"review-325",
			"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-08-09T03:00:00Z"},
			"relationships":{"build":{"data":{"type":"builds","id":"build-325"}}}
		}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"builds",
				"id":"build-326",
				"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			},
			{
				"type":"builds",
				"id":"build-325",
				"attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			}
		],
		"included":[
			{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "6748252780", "--platform", "IOS", "--include", "builds,testflight", "--output", "json", "--pretty"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Summary struct {
			Health     string   `json:"health"`
			NextAction string   `json:"nextAction"`
			Blockers   []string `json:"blockers"`
		} `json:"summary"`
		Builds struct {
			Latest struct {
				ID          string `json:"id"`
				BuildNumber string `json:"buildNumber"`
			} `json:"latest"`
		} `json:"builds"`
		TestFlight struct {
			BetaReviewState      string `json:"betaReviewState"`
			SubmittedDate        string `json:"submittedDate"`
			BetaReviewSubmission struct {
				ID                    string `json:"id"`
				State                 string `json:"state"`
				RelationToLatestBuild string `json:"relationToLatestBuild"`
				Build                 struct {
					ID          string `json:"id"`
					Version     string `json:"version"`
					BuildNumber string `json:"buildNumber"`
					Platform    string `json:"platform"`
				} `json:"build"`
			} `json:"betaReviewSubmission"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload.Builds.Latest.ID != "build-326" || payload.Builds.Latest.BuildNumber != "326" {
		t.Fatalf("expected latest uploaded build 326, got %+v", payload.Builds.Latest)
	}
	review := payload.TestFlight.BetaReviewSubmission
	if payload.TestFlight.BetaReviewState != "WAITING_FOR_REVIEW" || payload.TestFlight.SubmittedDate != "2026-08-09T03:00:00Z" {
		t.Fatalf("expected additive compatibility fields to remain populated, got %+v", payload.TestFlight)
	}
	if review.ID != "review-325" || review.State != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected review submission 325 identity, got %+v", review)
	}
	if review.Build.ID != "build-325" || review.Build.BuildNumber != "325" || review.Build.Version != "1.2.3" || review.Build.Platform != "IOS" {
		t.Fatalf("expected actual review build context for 325, got %+v", review.Build)
	}
	if review.RelationToLatestBuild != "sameVersionTrain" {
		t.Fatalf("expected sameVersionTrain relation, got %q", review.RelationToLatestBuild)
	}
	if payload.Summary.Health != "red" {
		t.Fatalf("expected blocked health=red, got %q", payload.Summary.Health)
	}
	if len(payload.Summary.Blockers) != 1 || !strings.Contains(payload.Summary.Blockers[0], "build 325") || !strings.Contains(payload.Summary.Blockers[0], "build 326") {
		t.Fatalf("expected blocker naming review build 325 and latest build 326, got %v", payload.Summary.Blockers)
	}
	if !strings.Contains(payload.Summary.NextAction, "build 325") || !strings.Contains(payload.Summary.NextAction, "build 326") {
		t.Fatalf("expected actionable next step naming both builds, got %q", payload.Summary.NextAction)
	}
}

func TestStatusResolvesMissingActiveBetaReviewBuildBeforeSelection(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"builds",
				"id":"build-326",
				"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			},
			{
				"type":"builds",
				"id":"build-325",
				"attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			}
		],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-326,build-325", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"betaAppReviewSubmissions",
				"id":"approved-326",
				"attributes":{"betaReviewState":"APPROVED","submittedDate":"2026-08-10T05:00:00Z"},
				"relationships":{"build":{"data":{"type":"builds","id":"build-326"}}}
			},
			{
				"type":"betaAppReviewSubmissions",
				"id":"waiting-325",
				"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-08-09T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"builds",
				"id":"build-326",
				"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			},
			{
				"type":"builds",
				"id":"build-325",
				"attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"},
				"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
			}
		],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions/waiting-325/build"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"builds","id":"build-325","attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"}}
	}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--platform", "IOS", "--include", "builds,testflight", "--output", "json"}); err != nil {
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
		Summary struct {
			Health   string   `json:"health"`
			Blockers []string `json:"blockers"`
		} `json:"summary"`
		TestFlight struct {
			BetaReviewSubmission struct {
				ID                    string `json:"id"`
				RelationToLatestBuild string `json:"relationToLatestBuild"`
				Build                 struct {
					ID          string `json:"id"`
					Version     string `json:"version"`
					BuildNumber string `json:"buildNumber"`
					Platform    string `json:"platform"`
				} `json:"build"`
			} `json:"betaReviewSubmission"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review := payload.TestFlight.BetaReviewSubmission
	if relatedBuildCalls := countStatusRequests(reg, "/v1/betaAppReviewSubmissions/waiting-325/build"); relatedBuildCalls != 1 {
		t.Fatalf("expected one bounded related-build fallback, got %d", relatedBuildCalls)
	}
	if review.ID != "waiting-325" || review.RelationToLatestBuild != "sameVersionTrain" {
		t.Fatalf("expected active same-train review to win selection, got %+v", review)
	}
	if review.Build.ID != "build-325" || review.Build.BuildNumber != "325" || review.Build.Version != "1.2.3" || review.Build.Platform != "IOS" {
		t.Fatalf("expected resolved review build context, got %+v", review.Build)
	}
	if payload.Summary.Health != "red" || len(payload.Summary.Blockers) != 1 {
		t.Fatalf("expected older active review to block latest build, got %+v", payload.Summary)
	}
}

func TestStatusResolvesSelectedActiveBetaReviewBeyondPrefetchCap(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewSubmissions := make([]map[string]any, 0, 6)
	for index := 1; index <= 6; index++ {
		reviewSubmissions = append(reviewSubmissions, map[string]any{
			"type": "betaAppReviewSubmissions",
			"id":   fmt.Sprintf("waiting-%d", index),
			"attributes": map[string]any{
				"betaReviewState": "WAITING_FOR_REVIEW",
				"submittedDate":   fmt.Sprintf("2026-08-09T%02d:00:00Z", 7-index),
			},
		})
	}
	reviewResponse, err := json.Marshal(map[string]any{
		"data":  reviewSubmissions,
		"links": map[string]any{"next": ""},
	})
	if err != nil {
		t.Fatalf("marshal review response: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-326",
			"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-326", "include=build", "limit=200"), asctest.JSON(http.StatusOK, string(reviewResponse)))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-326",
			"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	for index := 1; index <= 6; index++ {
		submissionID := fmt.Sprintf("waiting-%d", index)
		buildNumber, version := "400", "2.0.0"
		if submissionID == "waiting-6" {
			buildNumber, version = "325", "1.2.3"
		}
		reg.Register(asctest.GET("/v1/betaAppReviewSubmissions/"+submissionID+"/build"), asctest.JSON(http.StatusOK, fmt.Sprintf(`{
			"data":{"type":"builds","id":"review-build-%s","attributes":{"version":"%s","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"}}
		}`, submissionID, buildNumber)))
		reg.Register(asctest.GET("/v1/builds/review-build-"+submissionID+"/preReleaseVersion"), asctest.JSON(http.StatusOK, fmt.Sprintf(`{
			"data":{"type":"preReleaseVersions","id":"train-%s-ios","attributes":{"version":"%s","platform":"IOS"}}
		}`, version, version)))
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--platform", "IOS", "--include", "builds,testflight", "--output", "json"}); err != nil {
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
		Summary struct {
			Health     string   `json:"health"`
			Blockers   []string `json:"blockers"`
			NextAction string   `json:"nextAction"`
		} `json:"summary"`
		TestFlight struct {
			BetaReviewSubmission struct {
				ID                    string                 `json:"id"`
				RelationToLatestBuild string                 `json:"relationToLatestBuild"`
				Build                 betaReviewBuildPayload `json:"build"`
			} `json:"betaReviewSubmission"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review := payload.TestFlight.BetaReviewSubmission
	relatedBuildCalls, preReleaseCalls := 0, 0
	for _, req := range reg.Requests() {
		switch {
		case strings.HasPrefix(req.URL.Path, "/v1/betaAppReviewSubmissions/waiting-") && strings.HasSuffix(req.URL.Path, "/build"):
			relatedBuildCalls++
		case strings.HasPrefix(req.URL.Path, "/v1/builds/review-build-waiting-") && strings.HasSuffix(req.URL.Path, "/preReleaseVersion"):
			preReleaseCalls++
		}
	}
	if relatedBuildCalls != 6 || preReleaseCalls != 6 {
		t.Fatalf("expected exactly six bounded fallback contexts (12 requests), got related=%d preRelease=%d", relatedBuildCalls, preReleaseCalls)
	}
	if review.ID != "waiting-6" || review.RelationToLatestBuild != "sameVersionTrain" {
		t.Fatalf("expected selected sixth active review to resolve as same train, got %+v", review)
	}
	if review.Build.ID != "review-build-waiting-6" || review.Build.BuildNumber != "325" || review.Build.Version != "1.2.3" || review.Build.Platform != "IOS" {
		t.Fatalf("expected resolved selected review build, got %+v", review.Build)
	}
	if payload.Summary.Health != "red" || len(payload.Summary.Blockers) != 1 {
		t.Fatalf("expected selected older active review to block latest build, got %+v", payload.Summary)
	}
	if !strings.Contains(payload.Summary.NextAction, "build 325") || !strings.Contains(payload.Summary.NextAction, "build 326") {
		t.Fatalf("expected actionable next step naming review and latest builds, got %q", payload.Summary.NextAction)
	}
}

func TestStatusEnrichesLinkedActiveReviewBuildOutsideSnapshot(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-326",
			"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-326", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"betaAppReviewSubmissions",
			"id":"waiting-325",
			"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-08-09T03:00:00Z"},
			"relationships":{"build":{"data":{"type":"builds","id":"build-325"}}}
		}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-326",
			"attributes":{"version":"326","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions/waiting-325/build"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"builds","id":"build-325","attributes":{"version":"325","uploadedDate":"2026-08-09T02:00:00Z","processingState":"VALID"}}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-325/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}
	}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--platform", "IOS", "--include", "builds,testflight", "--output", "json"}); err != nil {
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
		Summary struct {
			Health   string   `json:"health"`
			Blockers []string `json:"blockers"`
		} `json:"summary"`
		TestFlight struct {
			BetaReviewSubmission struct {
				RelationToLatestBuild string                 `json:"relationToLatestBuild"`
				Build                 betaReviewBuildPayload `json:"build"`
			} `json:"betaReviewSubmission"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review := payload.TestFlight.BetaReviewSubmission
	if review.RelationToLatestBuild != "sameVersionTrain" {
		t.Fatalf("expected enriched sameVersionTrain relation, got %+v", review)
	}
	if review.Build.ID != "build-325" || review.Build.BuildNumber != "325" || review.Build.Version != "1.2.3" || review.Build.Platform != "IOS" {
		t.Fatalf("expected full review build identity outside snapshot, got %+v", review.Build)
	}
	if payload.Summary.Health != "red" || len(payload.Summary.Blockers) != 1 {
		t.Fatalf("expected enriched older active review to block latest build, got %+v", payload.Summary)
	}
}

func TestStatusFindsActiveBetaReviewBeyondFiftyBuildSnapshot(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	snapshotBuildIDs := make([]string, 0, 50)
	builds := make([]map[string]any, 0, 50)
	for buildNumber := 376; buildNumber >= 327; buildNumber-- {
		snapshotBuildIDs = append(snapshotBuildIDs, fmt.Sprintf("build-%d", buildNumber))
		builds = append(builds, map[string]any{
			"type": "builds",
			"id":   fmt.Sprintf("build-%d", buildNumber),
			"attributes": map[string]any{
				"version":         fmt.Sprintf("%d", buildNumber),
				"uploadedDate":    "2026-08-10T02:00:00Z",
				"processingState": "VALID",
			},
			"relationships": map[string]any{
				"preReleaseVersion": map[string]any{
					"data": map[string]any{"type": "preReleaseVersions", "id": "train-1.2.3-ios"},
				},
			},
		})
	}
	body, err := json.Marshal(map[string]any{
		"data": builds,
		"included": []map[string]any{{
			"type": "preReleaseVersions", "id": "train-1.2.3-ios",
			"attributes": map[string]any{"version": "1.2.3", "platform": "IOS"},
		}},
		"links": map[string]any{"next": "https://api.appstoreconnect.apple.com/v1/builds?cursor=older"},
	})
	if err != nil {
		t.Fatalf("marshal builds response: %v", err)
	}
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, string(body)))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]="+strings.Join(snapshotBuildIDs, ","), "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-376",
			"attributes":{"version":"376","uploadedDate":"2026-08-10T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/builds?cursor=active-older&filter%5Bapp%5D=6748252780&filter%5BbetaAppReviewSubmission.betaReviewState%5D=WAITING_FOR_REVIEW%2CIN_REVIEW&filter%5BpreReleaseVersion.platform%5D=IOS&include=preReleaseVersion&limit=50"}
	}`))
	reg.Register(asctest.GET("/v1/builds", "cursor=active-older", "filter[app]=6748252780", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "filter[preReleaseVersion.platform]=IOS", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-325",
			"attributes":{"version":"325","uploadedDate":"2026-08-08T02:00:00Z","processingState":"VALID"},
			"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}}}
		}],
		"included":[{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-325", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"betaAppReviewSubmissions",
			"id":"waiting-325",
			"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-08-09T03:00:00Z"},
			"relationships":{"build":{"data":{"type":"builds","id":"build-325"}}}
		}],
		"links":{"next":""}
	}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--platform", "IOS", "--include", "builds,testflight", "--output", "json"}); err != nil {
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
		Summary struct {
			Health     string   `json:"health"`
			Blockers   []string `json:"blockers"`
			NextAction string   `json:"nextAction"`
		} `json:"summary"`
		TestFlight struct {
			BetaReviewSubmission struct {
				ID                    string                 `json:"id"`
				RelationToLatestBuild string                 `json:"relationToLatestBuild"`
				Build                 betaReviewBuildPayload `json:"build"`
			} `json:"betaReviewSubmission"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review := payload.TestFlight.BetaReviewSubmission
	buildPageCalls, reviewSubmissionCalls := countStatusRequests(reg, "/v1/builds"), countStatusRequests(reg, "/v1/betaAppReviewSubmissions")
	if buildPageCalls != 3 || reviewSubmissionCalls != 2 {
		t.Fatalf("expected one snapshot plus two active-review build pages and two batched review queries, got builds=%d reviews=%d", buildPageCalls, reviewSubmissionCalls)
	}
	if review.ID != "waiting-325" || review.RelationToLatestBuild != "sameVersionTrain" {
		t.Fatalf("expected active review beyond snapshot to match the latest train, got %+v", review)
	}
	if review.Build.ID != "build-325" || review.Build.BuildNumber != "325" || review.Build.Version != "1.2.3" || review.Build.Platform != "IOS" {
		t.Fatalf("expected full older review build identity, got %+v", review.Build)
	}
	if payload.Summary.Health != "red" || len(payload.Summary.Blockers) != 1 {
		t.Fatalf("expected older active review to block instead of reporting green, got %+v", payload.Summary)
	}
	if !strings.Contains(payload.Summary.NextAction, "build 325") || !strings.Contains(payload.Summary.NextAction, "build 376") {
		t.Fatalf("expected action naming review build 325 and latest build 376, got %q", payload.Summary.NextAction)
	}
}

type betaReviewBuildPayload struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	BuildNumber string `json:"buildNumber"`
	Platform    string `json:"platform"`
}

func TestStatusPlatformFiltersPlatformScopedSections(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=MAC_OS", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-mac",
				"attributes":{"state":"COMPLETE","platform":"MAC_OS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "filter[preReleaseVersion.platform]=MAC_OS", "include=preReleaseVersion", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"builds","id":"build-mac","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-mac/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"prv-mac","attributes":{"version":"1.2.3","platform":"MAC_OS"}}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=MAC_OS", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-mac",
				"attributes":{
					"platform":"MAC_OS",
					"versionString":"1.2.3",
					"appVersionState":"READY_FOR_SALE",
					"createdDate":"2026-02-20T02:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=MAC_OS", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-mac",
				"attributes":{"state":"COMPLETE","platform":"MAC_OS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "filter[preReleaseVersion.platform]=MAC_OS", "include=preReleaseVersion", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"builds","id":"build-mac","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-mac/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"prv-mac","attributes":{"version":"1.2.3","platform":"MAC_OS"}}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=MAC_OS", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-mac",
				"attributes":{
					"platform":"MAC_OS",
					"versionString":"1.2.3",
					"appVersionState":"READY_FOR_SALE",
					"createdDate":"2026-02-20T02:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))

	for _, tt := range []struct {
		name string
		args []string
	}{
		{
			name: "canonical order",
			args: []string{"status", "--app", "app-1", "--platform", "mac_os", "--include", "builds,appstore,review"},
		},
		{
			name: "platform before app",
			args: []string{"status", "--platform", "mac_os", "--app", "app-1", "--include", "builds,appstore,review"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runCommand(t, tt.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
			}

			summary := payload["summary"].(map[string]any)
			if summary["platform"] != "MAC_OS" {
				t.Fatalf("expected summary.platform MAC_OS, got %v", summary["platform"])
			}

			builds := payload["builds"].(map[string]any)
			latestBuild := builds["latest"].(map[string]any)
			if latestBuild["platform"] != "MAC_OS" {
				t.Fatalf("expected builds.latest.platform MAC_OS, got %v", latestBuild["platform"])
			}

			appStore := payload["appstore"].(map[string]any)
			if appStore["platform"] != "MAC_OS" || appStore["versionId"] != "ver-mac" {
				t.Fatalf("expected macOS appstore version, got %v", appStore)
			}

			review := payload["review"].(map[string]any)
			if review["platform"] != "MAC_OS" || review["latestSubmissionId"] != "review-sub-mac" {
				t.Fatalf("expected macOS review submission, got %v", review)
			}
		})
	}
}

func TestStatusAppStorePaginatesBeforeChoosingLatestVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.2",
					"appVersionState":"WAITING_FOR_REVIEW",
					"createdDate":"2026-02-10T02:00:00Z"
				}
			}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page-2"}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "cursor=page-2"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-2",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"READY_FOR_SALE",
					"createdDate":"2026-02-20T02:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "appstore"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if appStoreCalls := countStatusRequests(reg, "/v1/apps/app-1/appStoreVersions"); appStoreCalls != 2 {
		t.Fatalf("expected 2 app store versions requests, got %d", appStoreCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	appStore, ok := payload["appstore"].(map[string]any)
	if !ok {
		t.Fatalf("expected appstore section, got %T", payload["appstore"])
	}
	if appStore["versionId"] != "ver-2" {
		t.Fatalf("expected paginated latest version ver-2, got %v", appStore["versionId"])
	}
	if appStore["version"] != "1.2.3" {
		t.Fatalf("expected paginated latest version string 1.2.3, got %v", appStore["version"])
	}
}

func TestStatusSubmissionAndReviewPaginateBeforeDerivingState(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
			"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
		}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
			}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "cursor=page-2"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-2",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "submission,review"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls := countStatusRequests(reg, "/v1/apps/app-1/reviewSubmissions"); reviewCalls != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review, ok := payload["review"].(map[string]any)
	if !ok {
		t.Fatalf("expected review section, got %T", payload["review"])
	}
	if review["latestSubmissionId"] != "review-sub-2" {
		t.Fatalf("expected latest paginated submission review-sub-2, got %v", review["latestSubmissionId"])
	}
	if review["state"] != "UNRESOLVED_ISSUES" {
		t.Fatalf("expected latest paginated review state UNRESOLVED_ISSUES, got %v", review["state"])
	}

	submission, ok := payload["submission"].(map[string]any)
	if !ok {
		t.Fatalf("expected submission section, got %T", payload["submission"])
	}
	blockingIssues, ok := submission["blockingIssues"].([]any)
	if !ok {
		t.Fatalf("expected blockingIssues slice, got %T", submission["blockingIssues"])
	}
	if len(blockingIssues) != 1 || blockingIssues[0] != "submission review-sub-2 has unresolved issues" {
		t.Fatalf("expected paginated blocking issue for review-sub-2, got %#v", blockingIssues)
	}
}

func TestStatusSubmissionIgnoresHistoricUnresolvedIssuesWhenLatestSubmissionMovedOn(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
			"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
		}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-old",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
			}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "cursor=page-2"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-latest",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "submission,review"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls := countStatusRequests(reg, "/v1/apps/app-1/reviewSubmissions"); reviewCalls != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review, ok := payload["review"].(map[string]any)
	if !ok {
		t.Fatalf("expected review section, got %T", payload["review"])
	}
	if review["latestSubmissionId"] != "review-sub-latest" {
		t.Fatalf("expected latest submission review-sub-latest, got %v", review["latestSubmissionId"])
	}
	if review["state"] != "COMPLETE" {
		t.Fatalf("expected latest review state COMPLETE, got %v", review["state"])
	}

	submission, ok := payload["submission"].(map[string]any)
	if !ok {
		t.Fatalf("expected submission section, got %T", payload["submission"])
	}
	blockingIssues, ok := submission["blockingIssues"].([]any)
	if !ok {
		t.Fatalf("expected blockingIssues slice, got %T", submission["blockingIssues"])
	}
	if len(blockingIssues) != 0 {
		t.Fatalf("expected no blocking issues from stale unresolved submissions, got %#v", blockingIssues)
	}
}

func TestStatusSubmissionTracksLatestSubmissionPerPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
			"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
		}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-ios",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
			}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "cursor=page-2"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-tvos",
				"attributes":{"state":"COMPLETE","platform":"TV_OS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "submission,review"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls := countStatusRequests(reg, "/v1/apps/app-1/reviewSubmissions"); reviewCalls != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	review, ok := payload["review"].(map[string]any)
	if !ok {
		t.Fatalf("expected review section, got %T", payload["review"])
	}
	if review["latestSubmissionId"] != "review-sub-tvos" {
		t.Fatalf("expected latest submission review-sub-tvos, got %v", review["latestSubmissionId"])
	}

	submission, ok := payload["submission"].(map[string]any)
	if !ok {
		t.Fatalf("expected submission section, got %T", payload["submission"])
	}
	inFlight, ok := submission["inFlight"].(bool)
	if !ok {
		t.Fatalf("expected inFlight bool, got %T", submission["inFlight"])
	}
	if !inFlight {
		t.Fatalf("expected submission summary to remain in flight when another platform has unresolved issues")
	}
	blockingIssues, ok := submission["blockingIssues"].([]any)
	if !ok {
		t.Fatalf("expected blockingIssues slice, got %T", submission["blockingIssues"])
	}
	if len(blockingIssues) != 1 || blockingIssues[0] != "submission review-sub-ios has unresolved issues" {
		t.Fatalf("expected blocking issues for the latest IOS submission, got %#v", blockingIssues)
	}
}

func TestStatusIncludeBuildsOnlyFiltersSections(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "include=preReleaseVersion", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"builds","id":"build-2","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-2/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "builds"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if _, ok := payload["app"]; ok {
		t.Fatalf("did not expect app section when not included, got %v", payload)
	}
	if _, ok := payload["builds"]; !ok {
		t.Fatalf("expected builds section, got %v", payload)
	}
	for _, key := range []string{"testflight", "appstore", "submission", "review", "phasedRelease", "links"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("did not expect %s section in filtered output: %v", key, payload)
		}
	}
}

func TestStatusTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "include=preReleaseVersion", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"builds","id":"build-2","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/builds/build-2/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "builds", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "SUMMARY") || !strings.Contains(stdout, "BUILDS") {
		t.Fatalf("expected section-driven status headings in table output, got %q", stdout)
	}
	if strings.Contains(stdout, "NEEDS ATTENTION") {
		t.Fatalf("did not expect NEEDS ATTENTION section without blockers, got %q", stdout)
	}
	if !strings.Contains(stdout, "[+") || !strings.Contains(stdout, "ago") {
		t.Fatalf("expected symbol-prefixed states and relative time in table output, got %q", stdout)
	}
}

func TestStatusTableOutputShowsNeedsAttentionWhenBlocked(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-2",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "review", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "NEEDS ATTENTION") {
		t.Fatalf("expected NEEDS ATTENTION section when blockers exist, got %q", stdout)
	}
	if !strings.Contains(stdout, "[x] blocker_1") {
		t.Fatalf("expected blocker row with failure symbol, got %q", stdout)
	}
}

func TestStatusIncludeAppOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"my-app-sku"}}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "app"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if _, ok := payload["app"]; !ok {
		t.Fatalf("expected app section, got %v", payload)
	}
	if _, ok := payload["summary"]; !ok {
		t.Fatalf("expected summary section, got %v", payload)
	}
	for _, key := range []string{"builds", "testflight", "appstore", "submission", "review", "phasedRelease", "links"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("did not expect %s section in app-only output: %v", key, payload)
		}
	}
}

func TestStatusTestFlightReadsStatesFromIncludedBuildBetaDetail(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=app-1", "limit=2"), asctest.JSON(http.StatusOK, `{
		"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
	}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "include=preReleaseVersion,buildBetaDetail", "limit=50", "sort=-uploadedDate"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-2",
			"attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"},
			"relationships":{
				"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
				"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-2"}}
			}
		}],
		"included":[
			{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}},
			{"type":"buildBetaDetails","id":"bbd-2","attributes":{"internalBuildState":"IN_BETA_TESTING","externalBuildState":"READY_FOR_TESTING"}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-2", "include=build", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=app-1", "filter[betaAppReviewSubmission.betaReviewState]=WAITING_FOR_REVIEW,IN_REVIEW", "include=preReleaseVersion", "limit=50"), asctest.JSON(http.StatusOK, `{
		"data":[{
			"type":"builds",
			"id":"build-2",
			"attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"},
			"relationships":{
				"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
				"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-2"}}
			}
		}],
		"included":[
			{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}},
			{"type":"buildBetaDetails","id":"bbd-2","attributes":{"internalBuildState":"IN_BETA_TESTING","externalBuildState":"READY_FOR_TESTING"}}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runCommand(t, []string{"status", "--app", "app-1", "--include", "testflight"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if buildBetaDetailsCalls := countStatusRequests(reg, "/v1/buildBetaDetails"); buildBetaDetailsCalls != 0 {
		t.Fatalf("expected no /v1/buildBetaDetails requests, got %d", buildBetaDetailsCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	testflight, ok := payload["testflight"].(map[string]any)
	if !ok {
		t.Fatalf("expected testflight object, got %T", payload["testflight"])
	}
	if testflight["latestDistributedBuildId"] != "build-2" {
		t.Fatalf("expected latestDistributedBuildId=build-2, got %v", testflight["latestDistributedBuildId"])
	}
	if testflight["externalBuildState"] != "READY_FOR_TESTING" {
		t.Fatalf("expected externalBuildState=READY_FOR_TESTING, got %v", testflight["externalBuildState"])
	}
	if testflight["internalBuildState"] != "IN_BETA_TESTING" {
		t.Fatalf("expected internalBuildState=IN_BETA_TESTING, got %v", testflight["internalBuildState"])
	}
}

func statusJSONResponse(body string) *http.Response {
	return insightsJSONResponse(body)
}
