package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type screenshotSetUploadProbeResult struct {
	LocalizationID string
	SetID          string
	DisplayType    string
	Results        []asc.AssetUploadResultItem
}

func TestExecuteScreenshotSetUploadCompletesUploadFlow(t *testing.T) {
	dir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, dir, "01-home.png", 1242, 2688)
	fileSizeBytes := fileSize(t, filePath)

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(body), `"id":"new-1"`) {
				t.Fatalf("expected relationship patch to include uploaded screenshot, got %s", string(body))
			}
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	clientFactoryCalled := false
	listCalls := 0
	createCalls := 0

	result, err := ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[screenshotSetUploadProbeResult]{
		LocalizationID: "LOC_123",
		Path:           filePath,
		DeviceType:     "IPHONE_65",
		Replace:        true,
		RequireConfirm: func() error { return nil },
		ClientFactory: func() (*asc.Client, error) {
			clientFactoryCalled = true
			return newAssetsUploadTestClient(t), nil
		},
		Access: ScreenshotSetAccess{
			List: func(_ context.Context, _ *asc.Client, localizationID string, _ asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
				listCalls++
				if localizationID != "LOC_123" {
					t.Fatalf("expected localization ID LOC_123, got %q", localizationID)
				}
				return &asc.AppScreenshotSetsResponse{
					Data: []asc.Resource[asc.AppScreenshotSetAttributes]{
						{
							ID: "set-1",
							Attributes: asc.AppScreenshotSetAttributes{
								ScreenshotDisplayType: "APP_IPHONE_65",
							},
						},
					},
				}, nil
			},
			Create: func(_ context.Context, _ *asc.Client, _, _ string) (*asc.AppScreenshotSetResponse, error) {
				createCalls++
				t.Fatal("expected existing screenshot set to be reused")
				return nil, nil
			},
		},
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], results []asc.AssetUploadResultItem) screenshotSetUploadProbeResult {
			return screenshotSetUploadProbeResult{
				LocalizationID: localizationID,
				SetID:          set.ID,
				DisplayType:    set.Attributes.ScreenshotDisplayType,
				Results:        append([]asc.AssetUploadResultItem(nil), results...),
			}
		},
	})
	if err != nil {
		t.Fatalf("ExecuteScreenshotSetUpload() error: %v", err)
	}
	if !clientFactoryCalled {
		t.Fatal("expected client factory to be called")
	}
	if listCalls != 1 {
		t.Fatalf("expected 1 screenshot-set list call, got %d", listCalls)
	}
	if createCalls != 0 {
		t.Fatalf("expected no screenshot-set create calls, got %d", createCalls)
	}
	if result.LocalizationID != "LOC_123" {
		t.Fatalf("expected localization ID LOC_123, got %#v", result)
	}
	if result.SetID != "set-1" {
		t.Fatalf("expected set ID set-1, got %#v", result)
	}
	if result.DisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected display type APP_IPHONE_65, got %#v", result)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 upload result, got %#v", result.Results)
	}
	if result.Results[0].AssetID != "new-1" || result.Results[0].State != "COMPLETE" {
		t.Fatalf("unexpected upload result: %#v", result.Results[0])
	}
}

func TestExecuteScreenshotSetUploadFullSetUsesOwnerSpecificInspectionCommand(t *testing.T) {
	dir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, dir, "01-home.png", 1242, 2688)

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" {
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"},{"type":"appScreenshots","id":"existing-2"},{"type":"appScreenshots","id":"existing-3"},{"type":"appScreenshots","id":"existing-4"},{"type":"appScreenshots","id":"existing-5"},{"type":"appScreenshots","id":"existing-6"},{"type":"appScreenshots","id":"existing-7"},{"type":"appScreenshots","id":"existing-8"},{"type":"appScreenshots","id":"existing-9"},{"type":"appScreenshots","id":"existing-10"}],"links":{}}`)
			return
		}
		t.Fatalf("full-set validation must not mutate remote assets: %s %s", req.Method, req.URL.String())
	}))

	_, err := ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[screenshotSetUploadProbeResult]{
		LocalizationID: "CUSTOM_LOC_123",
		Path:           filePath,
		DeviceType:     "IPHONE_65",
		InspectCommand: `asc product-pages custom-pages localizations screenshot-sets list --localization-id "CUSTOM_LOC_123" --include-screenshots --paginate --output json`,
		ReplaceCommand: `asc product-pages custom-pages localizations screenshot-sets sync --localization-id "CUSTOM_LOC_123" --path "/tmp/screenshots" --device-type "IPHONE_65" --confirm`,
		ClientFactory: func() (*asc.Client, error) {
			return client, nil
		},
		Access: ScreenshotSetAccess{
			List: func(_ context.Context, _ *asc.Client, localizationID string, _ asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
				if localizationID != "CUSTOM_LOC_123" {
					t.Fatalf("expected localization ID CUSTOM_LOC_123, got %q", localizationID)
				}
				return &asc.AppScreenshotSetsResponse{
					Data: []asc.Resource[asc.AppScreenshotSetAttributes]{
						{ID: "set-1", Attributes: asc.AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_65"}},
					},
				}, nil
			},
			Create: func(_ context.Context, _ *asc.Client, _, _ string) (*asc.AppScreenshotSetResponse, error) {
				t.Fatal("expected existing screenshot set to be reused")
				return nil, nil
			},
		},
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], results []asc.AssetUploadResultItem) screenshotSetUploadProbeResult {
			return screenshotSetUploadProbeResult{LocalizationID: localizationID, SetID: set.ID, Results: results}
		},
	})
	if err == nil {
		t.Fatal("expected full screenshot set error")
	}
	if !strings.Contains(err.Error(), `asc product-pages custom-pages localizations screenshot-sets list --localization-id "CUSTOM_LOC_123" --include-screenshots --paginate --output json`) {
		t.Fatalf("expected owner-specific inspection command, got %v", err)
	}
	if strings.Contains(err.Error(), "asc screenshots list --version-localization") {
		t.Fatalf("must not recommend App Store localization command for a custom page, got %v", err)
	}
	if !strings.Contains(err.Error(), `asc product-pages custom-pages localizations screenshot-sets sync --localization-id "CUSTOM_LOC_123" --path "/tmp/screenshots" --device-type "IPHONE_65" --confirm`) {
		t.Fatalf("expected owner-specific replacement command, got %v", err)
	}
	if strings.Contains(err.Error(), "--replace --confirm") {
		t.Fatalf("must not recommend unavailable replacement flags for a custom page, got %v", err)
	}
}

func TestExecuteScreenshotSetUploadRequiresDependencies(t *testing.T) {
	_, err := ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[struct{}]{
		LocalizationID: "LOC_123",
		Path:           "unused",
		DeviceType:     "IPHONE_65",
	})
	if err == nil || !strings.Contains(err.Error(), "client factory is required") {
		t.Fatalf("expected missing client factory error, got %v", err)
	}

	_, err = ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[struct{}]{
		LocalizationID: "LOC_123",
		Path:           "unused",
		DeviceType:     "IPHONE_65",
		ClientFactory: func() (*asc.Client, error) {
			t.Fatal("client factory should not be called when build result is missing")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "build result function is required") {
		t.Fatalf("expected missing build result function error, got %v", err)
	}
}

func TestExecuteScreenshotSetUploadInvalidDeviceTypeUsageMode(t *testing.T) {
	clientFactoryCalled := false
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		_, runErr = ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[struct{}]{
			LocalizationID:           "LOC_123",
			Path:                     "unused",
			DeviceType:               "ANDROID",
			InvalidDeviceTypeIsUsage: true,
			ClientFactory: func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, nil
			},
			BuildResult: func(string, asc.Resource[asc.AppScreenshotSetAttributes], []asc.AssetUploadResultItem) struct{} {
				return struct{}{}
			},
		})
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, `unsupported screenshot display type "APP_ANDROID"`) {
		t.Fatalf("expected unsupported display type message, got %q", stderr)
	}
	if clientFactoryCalled {
		t.Fatal("expected client factory to be skipped on usage validation failure")
	}
}

func TestExecuteScreenshotSetUploadInvalidDeviceTypeNonUsageMode(t *testing.T) {
	clientFactoryCalled := false
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		_, runErr = ExecuteScreenshotSetUpload(context.Background(), ScreenshotSetUploadOptions[struct{}]{
			LocalizationID: "LOC_123",
			Path:           "unused",
			DeviceType:     "ANDROID",
			ClientFactory: func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, nil
			},
			BuildResult: func(string, asc.Resource[asc.AppScreenshotSetAttributes], []asc.AssetUploadResultItem) struct{} {
				return struct{}{}
			},
		})
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected non-usage error, got %v", runErr)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), `unsupported screenshot display type "APP_ANDROID"`) {
		t.Fatalf("expected unsupported display type error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for non-usage error, got %q", stderr)
	}
	if clientFactoryCalled {
		t.Fatal("expected client factory to be skipped on display type validation failure")
	}
}
