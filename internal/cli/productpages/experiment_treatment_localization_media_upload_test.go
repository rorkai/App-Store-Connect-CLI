package productpages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExecuteExperimentTreatmentLocalizationScreenshotUpload_UploadCreatesSetAndOrdersUploads(t *testing.T) {
	dir := t.TempDir()
	fileA := writeCustomPageTestPNG(t, dir, "01-home.png")
	fileB := writeCustomPageTestPNG(t, dir, "02-settings.png")
	sizes := map[string]int64{
		"new-1": customPageFileSize(t, fileA),
		"new-2": customPageFileSize(t, fileB),
	}

	createdSet := false
	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			createdSet = true
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot-set body: %v", err)
			}
			if !strings.Contains(string(body), "appStoreVersionExperimentTreatmentLocalization") {
				t.Fatalf("expected PPO treatment-localization relationship in body, got %s", string(body))
			}
			return customPageJSONResponse(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			id := "new-1"
			if strings.Contains(string(body), "02-settings.png") {
				id = "new-2"
			}
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, sizes[id]))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"new-1", "new-2"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClientWithTimeout(t, 0)
	d := &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Timeout: shared.ContextWithTimeout,
	}

	result, err := executeExperimentTreatmentLocalizationScreenshotUpload(context.Background(), d, "tloc-1", dir, "IPHONE_65", false)
	if err != nil {
		t.Fatalf("executeExperimentTreatmentLocalizationScreenshotUpload() error: %v", err)
	}
	if result.SetID != "set-1" {
		t.Fatalf("expected set ID set-1, got %q", result.SetID)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 uploaded screenshots, got %d", len(result.Results))
	}
	if !createdSet {
		t.Fatal("expected missing screenshot set to be created")
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func TestExecuteExperimentTreatmentLocalizationScreenshotUpload_CanonicalizesAliasDisplayTypeForAPI(t *testing.T) {
	dir := t.TempDir()
	file := writeDisplayTypeTestPNG(t, dir, "01-home.png", "APP_IPHONE_69")
	fileSize := customPageFileSize(t, file)

	createdSetDisplayType := ""
	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot-set body: %v", err)
			}
			var payload asc.AppScreenshotSetCreateRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode create screenshot-set body: %v", err)
			}
			createdSetDisplayType = payload.Data.Attributes.ScreenshotDisplayType
			return customPageJSONResponse(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-69","attributes":{"screenshotDisplayType":"APP_IPHONE_67"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-69/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-69/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileSize))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return customPageJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return customPageJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-69/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClient(t)
	d := &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Timeout: shared.ContextWithTimeout,
	}

	result, err := executeExperimentTreatmentLocalizationScreenshotUpload(context.Background(), d, "tloc-1", dir, "IPHONE_69", false)
	if err != nil {
		t.Fatalf("executeExperimentTreatmentLocalizationScreenshotUpload() error: %v", err)
	}
	if createdSetDisplayType != "APP_IPHONE_67" {
		t.Fatalf("expected canonical create display type APP_IPHONE_67, got %q", createdSetDisplayType)
	}
	if result.DisplayType != "APP_IPHONE_67" {
		t.Fatalf("expected canonical result display type APP_IPHONE_67, got %q", result.DisplayType)
	}
}

func TestExecuteExperimentTreatmentLocalizationScreenshotUpload_UploadPreservesExistingOrderAndAppendsNewUploads(t *testing.T) {
	dir := t.TempDir()
	fileA := writeCustomPageTestPNG(t, dir, "01-home.png")
	fileB := writeCustomPageTestPNG(t, dir, "02-settings.png")
	sizes := map[string]int64{
		"new-1": customPageFileSize(t, fileA),
		"new-2": customPageFileSize(t, fileB),
	}

	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			id := "new-1"
			if strings.Contains(string(body), "02-settings.png") {
				id = "new-2"
			}
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, sizes[id]))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"old-1", "old-2", "new-1", "new-2"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClient(t)
	d := &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Timeout: shared.ContextWithTimeout,
	}

	result, err := executeExperimentTreatmentLocalizationScreenshotUpload(context.Background(), d, "tloc-1", dir, "IPHONE_65", false)
	if err != nil {
		t.Fatalf("executeExperimentTreatmentLocalizationScreenshotUpload() error: %v", err)
	}
	if result.SetID != "set-1" {
		t.Fatalf("expected set ID set-1, got %q", result.SetID)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 uploaded screenshots, got %d", len(result.Results))
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func TestExecuteExperimentTreatmentLocalizationScreenshotUpload_SyncDeletesExistingScreenshotsAndReordersUploads(t *testing.T) {
	dir := t.TempDir()
	fileA := writeCustomPageTestPNG(t, dir, "01-home.png")
	fileB := writeCustomPageTestPNG(t, dir, "02-settings.png")
	sizes := map[string]int64{
		"new-1": customPageFileSize(t, fileA),
		"new-2": customPageFileSize(t, fileB),
	}

	deletedExisting := false
	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"legacy.png"}}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/old-1":
			deletedExisting = true
			return customPageJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			id := "new-1"
			if strings.Contains(string(body), "02-settings.png") {
				id = "new-2"
			}
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, sizes[id]))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"new-1", "new-2"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClient(t)
	d := &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Timeout: shared.ContextWithTimeout,
	}

	result, err := executeExperimentTreatmentLocalizationScreenshotUpload(context.Background(), d, "tloc-1", dir, "IPHONE_65", true)
	if err != nil {
		t.Fatalf("executeExperimentTreatmentLocalizationScreenshotUpload() error: %v", err)
	}
	if result.SetID != "set-1" {
		t.Fatalf("expected set ID set-1, got %q", result.SetID)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 uploaded screenshots, got %d", len(result.Results))
	}
	if !deletedExisting {
		t.Fatal("expected sync to delete existing screenshots before upload")
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func TestExecuteExperimentTreatmentLocalizationScreenshotUpload_UsesRequestTimeoutForMetadataCalls(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "10m")

	dir := t.TempDir()
	file := writeCustomPageTestPNG(t, dir, "01-home.png")
	fileSize := customPageFileSize(t, file)

	var metadataRemaining time.Duration

	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatalf("expected request deadline for %s %s", req.Method, req.URL.Path)
		}

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			metadataRemaining = time.Until(deadline)
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileSize))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return customPageJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return customPageJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClient(t)
	d := &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Timeout: shared.ContextWithTimeout,
	}

	if _, err := executeExperimentTreatmentLocalizationScreenshotUpload(context.Background(), d, "tloc-1", dir, "IPHONE_65", false); err != nil {
		t.Fatalf("executeExperimentTreatmentLocalizationScreenshotUpload() error: %v", err)
	}

	if metadataRemaining <= 0 || metadataRemaining > 5*time.Second {
		t.Fatalf("expected metadata request timeout near ASC_TIMEOUT, got %s remaining", metadataRemaining)
	}
}
