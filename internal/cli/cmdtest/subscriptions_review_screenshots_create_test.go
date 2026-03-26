package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsReviewScreenshotsCreatePrintsVerifiedScreenshot(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	imagePath := filepath.Join(t.TempDir(), "review.png")
	writePNG(t, imagePath, 1242, 2688)
	imageInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review screenshot fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	getRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots":
			body := fmt.Sprintf(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, imageInfo.Size(), imageInfo.Size())
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			getRequests++
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "review", "screenshots", "create",
			"--subscription-id", "sub-1",
			"--file", imagePath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if getRequests != 1 {
		t.Fatalf("expected 1 GET request from polling, got %d", getRequests)
	}

	var output struct {
		Data struct {
			Attributes struct {
				AssetDeliveryState struct {
					State string `json:"state"`
				} `json:"assetDeliveryState"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if output.Data.Attributes.AssetDeliveryState.State != "COMPLETE" {
		t.Fatalf("expected output assetDeliveryState COMPLETE, got %q", output.Data.Attributes.AssetDeliveryState.State)
	}
}
