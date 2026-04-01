package cmdtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type submitCreateRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn submitCreateRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := fn(req)
	if err == nil {
		return resp, nil
	}
	if !strings.HasPrefix(err.Error(), "unexpected request:") {
		return nil, err
	}
	if fallback, ok := submitCreateDefaultReadinessResponse(req); ok {
		return fallback, nil
	}
	return nil, err
}

func setupSubmitCreateAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func submitCreateJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func mustSubmitCreateOKJSONResponse(body string) *http.Response {
	resp, err := submitCreateJSONResponse(http.StatusOK, body)
	if err != nil {
		panic(err)
	}
	return resp
}

func submitCreateDefaultReadinessResponse(req *http.Request) (*http.Response, bool) {
	path := req.URL.Path

	switch {
	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/builds/") && !strings.Contains(strings.TrimPrefix(path, "/v1/builds/"), "/"):
		buildID := strings.TrimSpace(strings.TrimPrefix(path, "/v1/builds/"))
		if buildID == "" {
			buildID = "build-1"
		}
		return mustSubmitCreateOKJSONResponse(fmt.Sprintf(`{"data":{"type":"builds","id":"%s","attributes":{"version":"1.0","processingState":"VALID","expired":false,"usesNonExemptEncryption":false}}}`, buildID)), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && strings.HasSuffix(path, "/appInfos"):
		return mustSubmitCreateOKJSONResponse(`{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && strings.HasSuffix(path, "/appPriceSchedule"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && strings.HasSuffix(path, "/appAvailabilityV2"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"appAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && strings.HasSuffix(path, "/subscriptionGroups"):
		return mustSubmitCreateOKJSONResponse(`{"data":[],"links":{}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && strings.HasSuffix(path, "/inAppPurchasesV2"):
		return mustSubmitCreateOKJSONResponse(`{"data":[],"links":{}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/apps/") && !strings.Contains(strings.TrimPrefix(path, "/v1/apps/"), "/"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appInfos/") && strings.HasSuffix(path, "/appInfoLocalizations"):
		return mustSubmitCreateOKJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App","subtitle":"Subtitle","privacyPolicyUrl":"https://example.com/privacy"}}]}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appInfos/") && strings.HasSuffix(path, "/relationships/primaryCategory"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"appCategories","id":"cat-1"}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appInfos/") && strings.HasSuffix(path, "/ageRatingDeclaration"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"advertising":false,"gambling":false,"healthOrWellnessTopics":false,"lootBox":false,"messagingAndChat":true,"parentalControls":true,"ageAssurance":false,"unrestrictedWebAccess":false,"userGeneratedContent":true,"alcoholTobaccoOrDrugUseOrReferences":"NONE","contests":"NONE","gamblingSimulated":"NONE","gunsOrOtherWeapons":"NONE","medicalOrTreatmentInformation":"NONE","profanityOrCrudeHumor":"NONE","sexualContentGraphicAndNudity":"NONE","sexualContentOrNudity":"NONE","horrorOrFearThemes":"NONE","matureOrSuggestiveThemes":"NONE","violenceCartoonOrFantasy":"NONE","violenceRealistic":"NONE","violenceRealisticProlongedGraphicOrSadistic":"NONE"}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appStoreVersions/") && strings.HasSuffix(path, "/appStoreReviewDetail"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"appStoreReviewDetails","id":"review-detail-1","attributes":{"contactFirstName":"A","contactLastName":"B","contactEmail":"a@example.com","contactPhone":"123","demoAccountName":"","demoAccountPassword":"","demoAccountRequired":false,"notes":"Review notes"}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appStoreVersions/") && strings.HasSuffix(path, "/build"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"VALID","expired":false,"usesNonExemptEncryption":false}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appStoreVersions/") && !strings.Contains(strings.TrimPrefix(path, "/v1/appStoreVersions/"), "/"):
		return mustSubmitCreateOKJSONResponse(`{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","copyright":"2026 Test Company"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appStoreVersionLocalizations/") && strings.HasSuffix(path, "/appScreenshotSets"):
		return mustSubmitCreateOKJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/appScreenshotSets/") && strings.HasSuffix(path, "/appScreenshots"):
		return mustSubmitCreateOKJSONResponse(`{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":1024,"imageAsset":{"width":1242,"height":2688}}}]}`), true

	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v2/appAvailabilities/") && strings.HasSuffix(path, "/territoryAvailabilities"):
		return mustSubmitCreateOKJSONResponse(`{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}}]}`), true
	}

	return nil, false
}

func TestSubmitCreateCancelsStaleSubmissions(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 8)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		// Stale submissions query — returns one stale submission
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		// Cancel stale submission
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// Add version as submission item
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		// Submit for review
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify stale submission was canceled (logged to stderr)
	if !strings.Contains(stderr, "Canceled stale review submission stale-1") {
		t.Errorf("expected stale submission cancel message in stderr, got: %q", stderr)
	}

	// Verify the cancel happened before creating the new submission
	cancelIdx := -1
	createIdx := -1
	for i, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/stale-1" {
			cancelIdx = i
		}
		if req == "POST /v1/reviewSubmissions" {
			createIdx = i
		}
	}
	if cancelIdx == -1 {
		t.Fatal("expected stale submission cancel request")
	}
	if createIdx == -1 {
		t.Fatal("expected new submission create request")
	}
	if cancelIdx >= createIdx {
		t.Fatalf("stale cancel (idx=%d) should happen before new create (idx=%d)", cancelIdx, createIdx)
	}

	// Verify stdout has valid JSON result
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateNoStaleSubmissions(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 8)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		// No stale submissions
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// No stale cancel messages
	if strings.Contains(stderr, "stale") {
		t.Errorf("expected no stale messages, got: %q", stderr)
	}

	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSkipsNonStaleSubmissionsFromCleanupResults(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 10)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		// Return mixed records defensively; cleanup should only cancel READY_FOR_REVIEW + IOS.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}},{"type":"reviewSubmissions","id":"active-1","attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS"}},{"type":"reviewSubmissions","id":"other-platform-1","attributes":{"state":"READY_FOR_REVIEW","platform":"MAC_OS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Canceled stale review submission stale-1") {
		t.Fatalf("expected stale cancel message, got: %q", stderr)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/reviewSubmissions/active-1") {
		t.Fatalf("did not expect cancel request for non-stale submission, requests: %v", requests)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/reviewSubmissions/other-platform-1") {
		t.Fatalf("did not expect cancel request for other platform submission, requests: %v", requests)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateWarnsWhenStaleSubmissionQueryFails(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Server Error"}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: failed to query stale review submissions") {
		t.Fatalf("expected stale query warning in stderr, got: %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateWarnsWhenStaleSubmissionCancelFails(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 9)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		// Cancel fails, but submit create should continue.
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusBadGateway, `{"errors":[{"status":"502","code":"BAD_GATEWAY","title":"Bad Gateway"}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: failed to cancel stale submission stale-1") {
		t.Fatalf("expected cancel warning in stderr, got: %q", stderr)
	}

	cancelIdx := -1
	createIdx := -1
	for i, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/stale-1" {
			cancelIdx = i
		}
		if req == "POST /v1/reviewSubmissions" {
			createIdx = i
		}
	}
	if cancelIdx == -1 {
		t.Fatal("expected stale submission cancel attempt")
	}
	if createIdx == -1 {
		t.Fatal("expected new submission create request")
	}
	if cancelIdx >= createIdx {
		t.Fatalf("stale cancel attempt (idx=%d) should happen before new create (idx=%d)", cancelIdx, createIdx)
	}

	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSkipsNonCancellableSubmissionWithoutVersionItems(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	cancelAttempted := false
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		// Cancel returns 409 Conflict (submission already transitioned to non-cancellable state)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			if !cancelAttempted {
				cancelAttempted = true
				return submitCreateJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/stale-1/items":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// 409 Conflict should produce a clear info message, not the scary "Warning: failed to cancel" dump
	if strings.Contains(stderr, "Warning: failed to cancel stale submission") {
		t.Fatalf("expected no warning for 409 conflict on stale cancel, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Skipped stale submission stale-1: already transitioned to a non-cancellable state") {
		t.Fatalf("expected skip message for non-cancellable submission without version items, got: %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
	if !strings.Contains(stdout, "new-sub-1") {
		t.Fatalf("expected a fresh submission ID in stdout, got %q", stdout)
	}
}

func TestSubmitCreatePreflightBlocksWhenRequiredLocalizationFieldsAreMissing(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Submit preflight localizations check.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","whatsNew":"Nouveautes"}}]}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected preflight error for submit-incomplete localizations")
	}
	if !strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected preflight error, got: %v", runErr)
	}
	if !strings.Contains(stderr, "fr-FR") || !strings.Contains(stderr, "description") || !strings.Contains(stderr, "keywords") || !strings.Contains(stderr, "supportUrl") {
		t.Fatalf("expected per-locale missing fields summary in stderr, got: %q", stderr)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/appStoreVersions/version-1/relationships/build") {
		t.Fatalf("did not expect build attach request after preflight failure, requests: %v", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on preflight failure, got: %q", stdout)
	}
}

func TestSubmitCreateWarnsForSubscriptionPreflightStates(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-ready","attributes":{"name":"Monthly Ready","productId":"com.example.ready","state":"READY_TO_SUBMIT"}},{"type":"subscriptions","id":"sub-missing","attributes":{"name":"Monthly Missing","productId":"com.example.missing","state":"MISSING_METADATA"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: the following subscriptions are MISSING_METADATA") {
		t.Fatalf("expected missing metadata warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Monthly Missing") {
		t.Fatalf("expected missing metadata subscription name, got %q", stderr)
	}
	if !strings.Contains(stderr, "Run `asc validate subscriptions` for details on what's missing.") {
		t.Fatalf("expected validate subscriptions guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "Warning: the following subscriptions are READY_TO_SUBMIT") {
		t.Fatalf("expected ready-to-submit warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Monthly Ready") {
		t.Fatalf("expected ready-to-submit subscription name, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc web review subscriptions attach-group --app \"APP_ID\" --group-id \"GROUP_ID\" --confirm") {
		t.Fatalf("expected experimental web group attach guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc subscriptions review submit --subscription-id \"SUB_ID\" --confirm") {
		t.Fatalf("expected corrected submit command guidance, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateFailsReadinessPreflightBeforeCreatingReviewSubmission(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 24)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return submitCreateJSONResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected submit create to fail when full readiness preflight finds blocking issues")
	}
	if !strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected submit preflight failure, got %v", runErr)
	}
	if !strings.Contains(stderr, "App Store review details") {
		t.Fatalf("expected readiness preflight output to mention review details, got %q", stderr)
	}
	for _, req := range requests {
		if req == "PATCH /v1/appStoreVersions/version-1/relationships/build" {
			t.Fatalf("did not expect build attachment before readiness preflight failure, requests: %v", requests)
		}
		if req == "POST /v1/reviewSubmissions" {
			t.Fatalf("did not expect review submission creation after readiness preflight failure, requests: %v", requests)
		}
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on readiness preflight failure, got %q", stdout)
	}
}

func TestSubmitCreateSubscriptionPreflightPaginatesAndReportsSkippedGroups(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.RawQuery == "limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=groups-2&limit=200"}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.RawQuery == "cursor=groups-2&limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-2","attributes":{"referenceName":"Family"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.RawQuery == "limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-ready","attributes":{"name":"Monthly Ready","productId":"com.example.ready","state":"READY_TO_SUBMIT"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=subs-2&limit=200"}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.RawQuery == "cursor=subs-2&limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-missing","attributes":{"name":"Monthly Missing","productId":"com.example.missing","state":"MISSING_METADATA"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-2/subscriptions":
			return submitCreateJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Monthly Ready") || !strings.Contains(stderr, "Monthly Missing") {
		t.Fatalf("expected paginated subscription states in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Family") || !strings.Contains(stderr, "could not be fully checked") {
		t.Fatalf("expected skipped group warning in stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSubscriptionPreflightDoesNotConsumeSubmitTimeoutBudget(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_TIMEOUT", "200ms")

	const longDelay = 120 * time.Millisecond
	stopErr := errors.New("stop after submit timeout budget capture")
	var reviewSubmissionBudget time.Duration

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			if err := sleepWithContextDuration(req.Context(), longDelay); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if err := sleepWithContextDuration(req.Context(), longDelay); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected review submission request to have a deadline")
			}
			reviewSubmissionBudget = time.Until(deadline)
			return nil, stopErr

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"submit", "create",
		"--app", "app-1",
		"--version", "1.0",
		"--build", "build-1",
		"--platform", "IOS",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), stopErr.Error()) {
		t.Fatalf("expected submit create to stop after capturing fresh timeout budget, got %v", err)
	}
	if reviewSubmissionBudget < 100*time.Millisecond {
		t.Fatalf("expected fresh submit timeout budget after subscription preflight, got %v", reviewSubmissionBudget)
	}
}

func TestSubmitCreateLocalizationPreflightDoesNotConsumeSubmitTimeoutBudget(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_TIMEOUT", "200ms")

	const longDelay = 120 * time.Millisecond
	stopErr := errors.New("stop after localization timeout budget capture")
	var reviewSubmissionBudget time.Duration
	localizationRequests := 0

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				if err := sleepWithContextDuration(req.Context(), longDelay); err != nil {
					return nil, err
				}
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			localizationRequests++
			if localizationRequests == 1 {
				if err := sleepWithContextDuration(req.Context(), longDelay); err != nil {
					return nil, err
				}
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":""}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected review submission request to have a deadline")
			}
			reviewSubmissionBudget = time.Until(deadline)
			return nil, stopErr

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"submit", "create",
		"--app", "app-1",
		"--version", "1.0",
		"--build", "build-1",
		"--platform", "IOS",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), stopErr.Error()) {
		t.Fatalf("expected submit create to stop after capturing fresh localization timeout budget, got %v", err)
	}
	if localizationRequests < 2 {
		t.Fatalf("expected submit create to fetch localizations during both localization and readiness preflight, got %d request(s)", localizationRequests)
	}
	if reviewSubmissionBudget < 100*time.Millisecond {
		t.Fatalf("expected fresh submit timeout budget after localization preflight, got %v", reviewSubmissionBudget)
	}
}

func TestSubmitCreatePreparationDoesNotConsumeSubmitTimeoutBudget(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_TIMEOUT", "200ms")

	const longDelay = 120 * time.Millisecond
	stopErr := errors.New("stop after preparation timeout budget capture")
	var reviewSubmissionBudget time.Duration

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if err := sleepWithContextDuration(req.Context(), longDelay); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected review submission request to have a deadline")
			}
			reviewSubmissionBudget = time.Until(deadline)
			return nil, stopErr

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"submit", "create",
		"--app", "app-1",
		"--version", "1.0",
		"--build", "build-1",
		"--platform", "IOS",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), stopErr.Error()) {
		t.Fatalf("expected submit create to stop after capturing fresh preparation timeout budget, got %v", err)
	}
	if reviewSubmissionBudget < 100*time.Millisecond {
		t.Fatalf("expected fresh submit timeout budget after preparation checks, got %v", reviewSubmissionBudget)
	}
}

func TestSubmitCreateRecoversFromAlreadyAddedError(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 10)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution + isAppUpdate check
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":"Bug fixes"}}]}`)

		// Subscription preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// No stale submissions
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// Add version fails with "already added" error
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "ENTITY_ERROR",
					"title": "The request entity is not valid.",
					"detail": "An attribute value is not valid.",
					"meta": {
						"associatedErrors": {
							"/v1/reviewSubmissionItems": [{
								"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
								"detail": "appStoreVersions with id 883340862 was already added to another reviewSubmission with id fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a"
							}]
						}
					}
				}]
			}`)

		// Cancel the empty new submission we created
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"CANCELING"}}}`)

		// Submit the existing submission for review
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-13T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify recovery message was logged
	if !strings.Contains(stderr, "Version already in review submission") {
		t.Errorf("expected recovery message in stderr, got: %q", stderr)
	}

	// Verify the existing submission was submitted (not the new one)
	foundExistingSubmit := false
	for _, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a" {
			foundExistingSubmit = true
		}
	}
	if !foundExistingSubmit {
		t.Fatal("expected existing submission to be submitted for review")
	}

	// Verify stdout has valid JSON result
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
	if !strings.Contains(stdout, "fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a") {
		t.Errorf("expected output to reference existing submission ID, got: %q", stdout)
	}
}

func TestSubmitCreateReusesExistingEmptySubmissionWithoutVersionItemsAfterConflictRefresh(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 16)
	cancelAttempted := false
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"existing-empty","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/existing-empty":
			if !cancelAttempted {
				cancelAttempted = true
				return submitCreateJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-empty","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-empty":
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "existing-empty",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
				}
			}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-empty/items":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	foundNewSubmissionCreate := false
	for _, req := range requests {
		if req == "POST /v1/reviewSubmissions" {
			foundNewSubmissionCreate = true
		}
	}
	if foundNewSubmissionCreate {
		t.Fatalf("did not expect a fresh review submission when the refreshed submission is still empty, requests: %v", requests)
	}
	if !strings.Contains(strings.Join(requests, "\n"), "GET /v1/reviewSubmissions/existing-empty/items") {
		t.Fatalf("expected existing submission items lookup before reusing the empty submission, requests: %v", requests)
	}
	if !strings.Contains(stdout, "existing-empty") {
		t.Fatalf("expected output to reference the reused submission ID, got %q", stdout)
	}
	if !strings.Contains(stderr, "Reusing existing empty review submission existing-empty because App Store Connect would not cancel it.") {
		t.Fatalf("expected stderr to explain why the existing empty submission was reused, got %q", stderr)
	}
}

func TestSubmitCreatePrintsHintsWhenAnotherSubmissionIsStillInProgress(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 16)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "ENTITY_ERROR",
					"title": "The request entity is not valid.",
					"detail": "This resource cannot be reviewed, please check associated errors to see why.",
					"meta": {
						"associatedErrors": {
							"/v1/reviewSubmissionItems": [{
								"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
								"detail": "appStoreVersions with id version-1 is already in another reviewSubmission with id active-submission-1 still in progress"
							}],
							"/v1/appStoreVersions/version-1": [{
								"code": "STATE_ERROR.ENTITY_INVALID",
								"detail": "appStoreVersions with id version-1 is not ready to be submitted for review"
							}]
						}
					}
				}]
			}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"CANCELING"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected submit create to fail, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	for _, want := range []string{
		"Hint: Check the active submission: asc submit status --id active-submission-1",
		"Hint: Inspect the active submission payload: asc review submissions-get --id active-submission-1",
		"Hint: Re-run readiness validation: asc validate --app app-1 --version-id version-1",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "Hint: Re-run readiness validation: asc validate --app app-1 --version 1.0 --platform IOS") {
		t.Fatalf("did not expect duplicate version-string readiness hint, got %q", stderr)
	}
	foundCleanup := false
	for _, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/new-sub-1" {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Fatal("expected empty created submission cleanup request")
	}
}

func TestSubmitCreateDoesNotReuseSubmissionContainingNonVersionItemsAfterConflictRefresh(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 20)
	cancelAttempted := false
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"existing-items","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/existing-items":
			if !cancelAttempted {
				cancelAttempted = true
				return submitCreateJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-items","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-items":
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "existing-items",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
				}
			}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-items/items":
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "asset-item-1",
					"relationships": {
						"backgroundAssetVersion": {
							"data": {"type": "backgroundAssetVersions", "id": "asset-1"}
						}
					}
				}],
				"links": {}
			}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	foundNewSubmissionCreate := false
	for _, req := range requests {
		if req == "POST /v1/reviewSubmissions" {
			foundNewSubmissionCreate = true
		}
	}
	if !foundNewSubmissionCreate {
		t.Fatalf("expected a fresh review submission when the conflicting submission contains non-version items, requests: %v", requests)
	}
	if !strings.Contains(strings.Join(requests, "\n"), "GET /v1/reviewSubmissions/existing-items/items") {
		t.Fatalf("expected existing submission items lookup before deciding whether to reuse, requests: %v", requests)
	}
	if !strings.Contains(stdout, "new-sub-1") {
		t.Fatalf("expected output to reference the new submission ID, got %q", stdout)
	}
	if !strings.Contains(stderr, "Skipped stale submission existing-items: already transitioned to a non-cancellable state") {
		t.Fatalf("expected stderr to explain why the conflicting submission was not reused, got %q", stderr)
	}
}

func TestSubmitCreatePaginatesReadyForReviewSubmissionsBeforeCreatingNewOne(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 20)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.RequestURI()
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions" && req.URL.Query().Get("cursor") == "":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": [],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"
				}
			}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions" && req.URL.Query().Get("cursor") == "page-2":
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "existing-empty-page-2",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {}
			}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-empty-page-2/items":
			return submitCreateJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "version-item",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}]
			}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/existing-empty-page-2":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-empty-page-2","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-20T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	for _, req := range requests {
		if req == "POST /v1/reviewSubmissions" {
			t.Fatalf("did not expect a new review submission when a reusable READY_FOR_REVIEW submission exists on a later page, requests: %v", requests)
		}
	}
	if !strings.Contains(strings.Join(requests, "\n"), "GET /v1/apps/app-1/reviewSubmissions?cursor=page-2") {
		t.Fatalf("expected paginated review submissions lookup, requests: %v", requests)
	}
	if !strings.Contains(stdout, "existing-empty-page-2") {
		t.Fatalf("expected output to reference reused submission ID, got %q", stdout)
	}
	if !strings.Contains(stderr, "existing-empty-page-2") {
		t.Fatalf("expected stderr to mention the reused submission, got %q", stderr)
	}
}

func TestSubmitCreateRetriesWhenConflictPointsToRecentlyCanceledStaleSubmission(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 12)
	addItemAttempts := 0
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution + isAppUpdate check
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":"Bug fixes"}}]}`)

		// Subscription preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// One stale submission gets canceled before the new submission is created.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// The first add still points at the stale submission we just canceled.
		// The retry succeeds once App Store Connect catches up.
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			addItemAttempts++
			if addItemAttempts == 1 {
				return submitCreateJSONResponse(http.StatusConflict, `{
					"errors": [{
						"status": "409",
						"code": "ENTITY_ERROR",
						"title": "The request entity is not valid.",
						"detail": "An attribute value is not valid.",
						"meta": {
							"associatedErrors": {
								"/v1/reviewSubmissionItems": [{
									"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
									"detail": "appStoreVersions with id version-1 was already added to another reviewSubmission with id stale-1"
								}]
							}
						}
					}]
				}`)
			}
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		// The newly created submission is the one that gets submitted.
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-14T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if addItemAttempts != 2 {
		t.Fatalf("expected 2 add-item attempts, got %d", addItemAttempts)
	}
	if !strings.Contains(stderr, "Version is still detaching from recently canceled review submission stale-1") {
		t.Fatalf("expected stale-detaching retry message in stderr, got: %q", stderr)
	}

	stalePatchCount := 0
	newSubmissionPatchCount := 0
	addItemCount := 0
	for _, req := range requests {
		switch req {
		case "PATCH /v1/reviewSubmissions/stale-1":
			stalePatchCount++
		case "PATCH /v1/reviewSubmissions/new-sub-1":
			newSubmissionPatchCount++
		case "POST /v1/reviewSubmissionItems":
			addItemCount++
		}
	}
	if stalePatchCount != 1 {
		t.Fatalf("expected exactly one PATCH to stale submission for cancel, got %d in %v", stalePatchCount, requests)
	}
	if newSubmissionPatchCount != 1 {
		t.Fatalf("expected exactly one PATCH to new submission for submit, got %d in %v", newSubmissionPatchCount, requests)
	}
	if addItemCount != 2 {
		t.Fatalf("expected exactly two add-item requests, got %d in %v", addItemCount, requests)
	}

	if !strings.Contains(stdout, "new-sub-1") {
		t.Fatalf("expected output to reference new submission ID, got: %q", stdout)
	}
}

func TestSubmitCreateAcceptsApprovedEncryptionDeclaration(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"VALID","expired":false,"usesNonExemptEncryption":true}}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appEncryptionDeclaration":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"APPROVED"}}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected approved encryption declaration to pass readiness preflight, got %v (stderr: %q)", runErr, stderr)
	}
	if stdout == "" {
		t.Fatal("expected submit create JSON output")
	}
	if strings.Contains(stderr, "Attached build: build uses non-exempt encryption but has no linked encryption declaration") {
		t.Fatalf("did not expect false missing declaration error, got %q", stderr)
	}
}

func sleepWithContext(ctx context.Context) error {
	return sleepWithContextDuration(ctx, 70*time.Millisecond)
}

func sleepWithContextDuration(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
