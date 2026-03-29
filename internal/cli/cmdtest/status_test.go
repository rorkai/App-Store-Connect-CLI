package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusRequiresAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestStatusDefaultJSONIncludesAllSections(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/apps/app-1":
			return statusJSONResponse(`{
				"data": {
					"type":"apps",
					"id":"app-1",
					"attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"my-app-sku"}
				}
			}`), nil
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" {
				t.Fatalf("expected filter[app]=app-1, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "50" {
				t.Fatalf("expected limit=50, got %q", query.Get("limit"))
			}
			return statusJSONResponse(`{
				"data": [
					{
						"type":"builds",
						"id":"build-2",
						"attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}
					},
					{
						"type":"builds",
						"id":"build-1",
						"attributes":{"version":"44","uploadedDate":"2026-02-19T00:00:00Z","processingState":"VALID"}
					}
				],
				"links":{"next":""}
			}`), nil
		case "/v1/builds/build-2/preReleaseVersion":
			return statusJSONResponse(`{
				"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
			}`), nil
		case "/v1/buildBetaDetails":
			query := req.URL.Query()
			if query.Get("limit") != "200" {
				t.Fatalf("expected build beta details limit=200, got %q", query.Get("limit"))
			}
			filter := query.Get("filter[build]")
			if !strings.Contains(filter, "build-1") || !strings.Contains(filter, "build-2") {
				t.Fatalf("expected filter[build] to include build-1 and build-2, got %q", filter)
			}
			return statusJSONResponse(`{
				"data": [
					{
						"type":"buildBetaDetails",
						"id":"bbd-2",
						"attributes":{"externalBuildState":"IN_BETA_TESTING"},
						"relationships":{"build":{"data":{"type":"builds","id":"build-2"}}}
					},
					{
						"type":"buildBetaDetails",
						"id":"bbd-1",
						"attributes":{"externalBuildState":"NOT_READY_FOR_TESTING"},
						"relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}
					}
				],
				"links":{"next":""}
			}`), nil
		case "/v1/betaAppReviewSubmissions":
			query := req.URL.Query()
			if query.Get("limit") != "200" {
				t.Fatalf("expected beta app review submissions limit=200, got %q", query.Get("limit"))
			}
			return statusJSONResponse(`{
				"data":[
					{
						"type":"betaAppReviewSubmissions",
						"id":"beta-sub-1",
						"attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2026-02-20T01:00:00Z"},
						"relationships":{"build":{"data":{"type":"builds","id":"build-2"}}}
					}
				],
				"links":{"next":""}
			}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if query.Get("limit") != "200" {
				t.Fatalf("expected app store versions limit=200, got %q", query.Get("limit"))
			}
			return statusJSONResponse(`{
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
			}`), nil
		case "/v1/appStoreVersions/ver-2/appStoreVersionPhasedRelease":
			return statusJSONResponse(`{
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
			}`), nil
		case "/v1/apps/app-1/reviewSubmissions":
			query := req.URL.Query()
			if query.Get("limit") != "200" {
				t.Fatalf("expected review submissions limit=200, got %q", query.Get("limit"))
			}
			return statusJSONResponse(`{
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
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestStatusAppStorePaginatesBeforeChoosingLatestVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	var appStoreCalls lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			switch appStoreCalls.Inc() {
			case 1:
				if req.URL.Query().Get("limit") != "200" {
					t.Fatalf("expected app store versions limit=200, got %q", req.URL.Query().Get("limit"))
				}
				return statusJSONResponse(`{
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
				}`), nil
			case 2:
				if req.URL.Query().Get("cursor") != "page-2" {
					t.Fatalf("expected app store versions cursor=page-2, got %q", req.URL.Query().Get("cursor"))
				}
				return statusJSONResponse(`{
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
				}`), nil
			default:
				t.Fatalf("unexpected extra app store versions request: %s", req.URL.String())
				return nil, nil
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "appstore"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if appStoreCalls.Load() != 2 {
		t.Fatalf("expected 2 app store versions requests, got %d", appStoreCalls.Load())
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

	var reviewCalls lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
					"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
				}`), nil
		case "/v1/apps/app-1/reviewSubmissions":
			switch reviewCalls.Inc() {
			case 1:
				if req.URL.Query().Get("limit") != "200" {
					t.Fatalf("expected review submissions limit=200, got %q", req.URL.Query().Get("limit"))
				}
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-1",
							"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
						}
					],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
				}`), nil
			case 2:
				if req.URL.Query().Get("cursor") != "page-2" {
					t.Fatalf("expected review submissions cursor=page-2, got %q", req.URL.Query().Get("cursor"))
				}
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-2",
							"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
						}
					],
					"links":{"next":""}
				}`), nil
			default:
				t.Fatalf("unexpected extra review submissions request: %s", req.URL.String())
				return nil, nil
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "submission,review"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls.Load() != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls.Load())
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

	var reviewCalls lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
					"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
				}`), nil
		case "/v1/apps/app-1/reviewSubmissions":
			reviewCalls.Inc()
			switch req.URL.Query().Get("cursor") {
			case "":
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-old",
							"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
						}
					],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
				}`), nil
			case "page-2":
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-latest",
							"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
						}
					],
					"links":{"next":""}
				}`), nil
			default:
				t.Fatalf("unexpected review submissions cursor: %q", req.URL.Query().Get("cursor"))
				return nil, nil
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "submission,review"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls.Load() != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls.Load())
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

	var reviewCalls lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
					"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
				}`), nil
		case "/v1/apps/app-1/reviewSubmissions":
			reviewCalls.Inc()
			switch req.URL.Query().Get("cursor") {
			case "":
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-ios",
							"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-10T03:00:00Z"}
						}
					],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
				}`), nil
			case "page-2":
				return statusJSONResponse(`{
					"data":[
						{
							"type":"reviewSubmissions",
							"id":"review-sub-tvos",
							"attributes":{"state":"COMPLETE","platform":"TV_OS","submittedDate":"2026-02-20T03:00:00Z"}
						}
					],
					"links":{"next":""}
				}`), nil
			default:
				t.Fatalf("unexpected review submissions cursor: %q", req.URL.Query().Get("cursor"))
				return nil, nil
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "submission,review"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewCalls.Load() != 2 {
		t.Fatalf("expected 2 review submissions requests, got %d", reviewCalls.Load())
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/builds":
			return statusJSONResponse(`{
				"data":[{"type":"builds","id":"build-2","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
				"links":{"next":""}
			}`), nil
		case "/v1/builds/build-2/preReleaseVersion":
			return statusJSONResponse(`{
				"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "builds"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestStatusRejectsUnknownIncludeSection(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "builds,unknown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "--include contains unsupported section") {
		t.Fatalf("expected include validation error in stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestStatusTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/builds":
			return statusJSONResponse(`{
				"data":[{"type":"builds","id":"build-2","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
				"links":{"next":""}
			}`), nil
		case "/v1/builds/build-2/preReleaseVersion":
			return statusJSONResponse(`{
				"data":{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"IOS"}}
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "builds", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/apps/app-1/reviewSubmissions":
			return statusJSONResponse(`{
				"data":[
					{
						"type":"reviewSubmissions",
						"id":"review-sub-2",
						"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-02-20T03:00:00Z"}
					}
				],
				"links":{"next":""}
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "review", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/apps/app-1":
			return statusJSONResponse(`{
				"data":{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"my-app-sku"}}
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestStatusTestFlightHandlesMissingBuildRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildBetaDetailsCalls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			return statusJSONResponse(`{
				"data": [{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"app-1"}}]
			}`), nil
		case "/v1/apps/app-1":
			return statusJSONResponse(`{
				"data":{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"my-app-sku"}}
			}`), nil
		case "/v1/builds":
			return statusJSONResponse(`{
				"data":[{"type":"builds","id":"build-2","attributes":{"version":"45","uploadedDate":"2026-02-20T00:00:00Z","processingState":"VALID"}}],
				"links":{"next":""}
			}`), nil
		case "/v1/buildBetaDetails":
			buildBetaDetailsCalls++
			if req.URL.Query().Get("filter[build]") != "build-2" {
				t.Fatalf("expected build beta details filter[build]=build-2, got %q", req.URL.Query().Get("filter[build]"))
			}
			return statusJSONResponse(`{
				"data":[{"type":"buildBetaDetails","id":"bbd-2","attributes":{"externalBuildState":"READY_FOR_TESTING"}}],
				"links":{"next":""}
			}`), nil
		case "/v1/betaAppReviewSubmissions":
			return statusJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "app-1", "--include", "testflight"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if buildBetaDetailsCalls < 1 {
		t.Fatal("expected build beta details request")
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
}

func statusJSONResponse(body string) *http.Response {
	return insightsJSONResponse(body)
}
