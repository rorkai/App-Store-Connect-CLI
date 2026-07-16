package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIAPVersionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"create requires iap", []string{"iap", "versions", "create"}, "--iap-id is required"},
		{"list requires iap", []string{"iap", "versions", "list"}, "--iap-id is required"},
		{"list validates state", []string{"iap", "versions", "list", "--iap-id", "iap-1", "--state", "NOPE"}, "--state must be one of"},
		{"view requires version", []string{"iap", "versions", "view"}, "--version-id is required"},
		{"image requires version", []string{"iap", "versions", "image"}, "--version-id is required"},
		{"localization create requires version", []string{"iap", "versions", "localizations", "create", "--name", "Name", "--locale", "en-US"}, "--version-id is required"},
		{"localization view requires id", []string{"iap", "versions", "localizations", "view"}, "--localization-id is required"},
		{"localization delete requires confirm", []string{"iap", "versions", "localizations", "delete", "--localization-id", "loc-1"}, "--confirm is required"},
		{"image create requires version", []string{"iap", "versions", "images", "create", "--file", "image.png"}, "--version-id is required"},
		{"image delete requires confirm", []string{"iap", "versions", "images", "delete", "--image-id", "img-1"}, "--confirm is required"},
		{"submit requires submission", []string{"iap", "versions", "submit", "--version-id", "version-1", "--confirm"}, "--submission is required"},
		{"submit requires confirm", []string{"iap", "versions", "submit", "--version-id", "version-1", "--submission", "submission-1"}, "--confirm is required"},
		{"links version requires iap", []string{"iap", "versions", "links", "versions"}, "--iap-id is required"},
		{"links image requires version", []string{"iap", "versions", "links", "image"}, "--version-id is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPVersionImagesCreateRunsV2UploadLifecycle(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "review.png")
	if err := os.WriteFile(imagePath, pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		body := ""
		status := http.StatusOK
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchaseImages" {
				t.Fatalf("unexpected reservation request: %s %s", req.Method, req.URL)
			}
			requestBody, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.Contains(string(requestBody), `"version":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}`) {
				t.Fatalf("missing version relationship: %s", requestBody)
			}
			status = http.StatusCreated
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"fileName":"review.png","fileSize":` + stringValue(len(pngBytes)) + `,"uploadOperations":[{"method":"PUT","url":"https://upload.example/image-1","offset":0,"length":` + stringValue(len(pngBytes)) + `}]}}}`
		case 2:
			if req.Method != http.MethodPut || req.URL.String() != "https://upload.example/image-1" {
				t.Fatalf("unexpected upload request: %s %s", req.Method, req.URL)
			}
			uploadedBody, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(uploadedBody) != string(pngBytes) {
				t.Fatalf("uploaded bytes differ")
			}
			status = http.StatusOK
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v2/inAppPurchaseImages/image-1" {
				t.Fatalf("unexpected commit request: %s %s", req.Method, req.URL)
			}
			commitBody, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.Contains(string(commitBody), `"uploaded":true`) {
				t.Fatalf("missing uploaded commit: %s", commitBody)
			}
			if strings.Contains(string(commitBody), "sourceFileChecksum") {
				t.Fatalf("v2 commit must not send the removed checksum field: %s", commitBody)
			}
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"assetDeliveryState":{"state":"PROCESSING"}}}}`
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchaseImages/image-1" {
				t.Fatalf("unexpected final fetch: %s %s", req.Method, req.URL)
			}
			body = `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"fileName":"review.png","fileSize":` + stringValue(len(pngBytes)) + `,"assetDeliveryState":{"state":"COMPLETE"}}}}`
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "versions", "images", "create", "--version-id", "version-1", "--file", imagePath, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	data, _ := response["data"].(map[string]any)
	if data["id"] != "image-1" {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
}

func TestIAPVersionsSubmitUsesExplicitReviewSubmissionRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		bodyString := string(body)
		if !strings.Contains(bodyString, `"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"submission-1"}}`) {
			t.Fatalf("missing review submission relationship: %s", body)
		}
		if !strings.Contains(bodyString, `"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}`) {
			t.Fatalf("missing IAP version relationship: %s", body)
		}
		if strings.Contains(bodyString, "inAppPurchaseSubmission") {
			t.Fatalf("version submission must not use legacy IAP submission semantics: %s", body)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"data":{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"}}}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "versions", "submit", "--version-id", "version-1", "--submission", "submission-1", "--confirm", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, `"id":"item-1"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func stringValue(value int) string {
	return fmt.Sprintf("%d", value)
}
