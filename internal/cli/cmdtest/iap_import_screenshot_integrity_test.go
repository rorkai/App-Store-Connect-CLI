package cmdtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIAPImportPreservesScreenshotIDInPartialReceipt(t *testing.T) {
	for _, test := range []struct {
		name          string
		withUploadOps bool
	}{
		{name: "upload failure", withUploadOps: true},
		{name: "missing upload operations"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			dir := t.TempDir()
			filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
			screenshotPath := writeIAPImportScreenshot(t, dir)
			imageInfo, err := os.Stat(screenshotPath)
			if err != nil {
				t.Fatalf("stat screenshot: %v", err)
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
					return jsonResponse(http.StatusOK, `{"data":[]}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
					return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
					attributes := `"fileName":"coins.png","fileSize":` + strconv.FormatInt(imageInfo.Size(), 10)
					if test.withUploadOps {
						attributes += `,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","length":` + strconv.FormatInt(imageInfo.Size(), 10) + `,"offset":0}]`
					}
					body := `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-partial","attributes":{` + attributes + `}}}`
					return jsonResponse(http.StatusCreated, body)
				case req.Method == http.MethodPut && req.URL.Path == "/part" && test.withUploadOps:
					return jsonResponse(http.StatusInternalServerError, `upload failed`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			stdout, _, runErr := runIAPImport(t, []string{
				"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
			})
			if runErr == nil {
				t.Fatal("expected screenshot setup or upload to fail")
			}

			var receipt struct {
				Products []struct {
					Status             string `json:"status"`
					ReviewScreenshotID string `json:"reviewScreenshotId"`
				} `json:"products"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
			}
			if len(receipt.Products) != 1 || receipt.Products[0].Status != "failed" {
				t.Fatalf("expected one failed product in receipt, got %+v", receipt.Products)
			}
			if receipt.Products[0].ReviewScreenshotID != "shot-partial" {
				t.Fatalf("review screenshot ID = %q, want shot-partial", receipt.Products[0].ReviewScreenshotID)
			}
		})
	}
}

func TestIAPImportRejectsScreenshotGrownAfterPlanValidation(t *testing.T) {
	setupAuth(t)
	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)

	createdProduct := false
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			if err := os.Truncate(screenshotPath, 1<<30+1); err != nil {
				t.Fatalf("grow screenshot after planning: %v", err)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			createdProduct = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		default:
			t.Fatalf("unexpected request after screenshot growth: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr == nil || !createdProduct || !bytes.Contains([]byte(runErr.Error()), []byte("file size exceeds")) {
		t.Fatalf("run error = %v, product created = %t, stderr = %q; want size rejection after product creation", runErr, createdProduct, stderr)
	}
	if !bytes.Contains([]byte(stdout), []byte(`"status":"failed"`)) {
		t.Fatalf("partial receipt = %q, want failed product", stdout)
	}
}

func TestIAPImportKeepsScreenshotChecksumBoundToUploadedBytes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	original, err := os.ReadFile(screenshotPath)
	if err != nil {
		t.Fatalf("read screenshot fixture: %v", err)
	}
	expectedChecksum, err := asc.ComputeChecksumFromReader(bytes.NewReader(original), asc.ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatalf("compute fixture checksum: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	mutated := false
	var uploaded []byte
	var committedChecksum string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			if err := rewriteIAPImportFileInPlace(screenshotPath, bytes.Repeat([]byte{'B'}, len(original))); err != nil {
				t.Fatalf("rewrite screenshot fixture: %v", err)
			}
			mutated = true
			body := `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","fileSize":` + strconv.Itoa(len(original)) + `,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","length":` + strconv.Itoa(len(original)) + `,"offset":0}]}}}`
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Path == "/part":
			uploaded, err = io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read uploaded bytes: %v", err)
			}
			return jsonResponse(http.StatusOK, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode commit payload: %v", err)
			}
			if payload.Data.Attributes == nil || payload.Data.Attributes.SourceFileChecksum == nil {
				t.Fatal("expected sourceFileChecksum in commit payload")
			}
			committedChecksum = *payload.Data.Attributes.SourceFileChecksum
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected import success, got %v (stderr=%q)", runErr, stderr)
	}
	if !mutated {
		t.Fatal("transport hook did not mutate screenshot fixture")
	}
	if !bytes.Equal(uploaded, original) {
		t.Fatalf("uploaded bytes differ from pre-mutation source: got %d bytes", len(uploaded))
	}
	if committedChecksum != expectedChecksum.Hash {
		t.Fatalf("committed checksum = %q, want %q", committedChecksum, expectedChecksum.Hash)
	}
}

func TestIAPImportUsesUploadTimeoutForPresignedPart(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "3s")

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	imageInfo, err := os.Stat(screenshotPath)
	if err != nil {
		t.Fatalf("stat screenshot: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	uploadParts := 0
	minimumTimeout, maximumTimeout := time.Second, 3*time.Second
	checkDefault := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut && req.URL.Path == "/part" {
			deadline, ok := req.Context().Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining < minimumTimeout || remaining > maximumTimeout {
				t.Fatalf("presigned upload deadline = %v (remaining %s), want between %s and %s", deadline, remaining, minimumTimeout, maximumTimeout)
			}
			uploadParts++
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			if checkDefault {
				deadline, ok := req.Context().Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining < 9*time.Minute || remaining > 10*time.Minute {
					t.Fatalf("screenshot reservation deadline = %v (remaining %s), want ten-minute IAP default", deadline, remaining)
				}
			}
			body := `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","fileSize":` + strconv.FormatInt(imageInfo.Size(), 10) + `,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","length":` + strconv.FormatInt(imageInfo.Size(), 10) + `,"offset":0}]}}}`
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Path == "/part":
			return jsonResponse(http.StatusOK, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr != nil || uploadParts != 1 {
		t.Fatalf("run error = %v, upload parts = %d, stderr = %q; want one upload-timeout part", runErr, uploadParts, stderr)
	}

	t.Setenv("ASC_UPLOAD_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT", "20m")
	minimumTimeout, maximumTimeout = 4*time.Minute, 5*time.Minute
	checkDefault = true
	_, stderr, runErr = runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr != nil || uploadParts != 2 {
		t.Fatalf("run error = %v, upload parts = %d, stderr = %q; want ten-minute default upload timeout", runErr, uploadParts, stderr)
	}
}

func TestIAPImportGivesScreenshotDeliveryVerificationItsOwnTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "3s")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "1s")

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	imageInfo, err := os.Stat(screenshotPath)
	if err != nil {
		t.Fatalf("stat screenshot: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			time.Sleep(600 * time.Millisecond)
			body := `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","fileSize":` + strconv.FormatInt(imageInfo.Size(), 10) + `,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","length":` + strconv.FormatInt(imageInfo.Size(), 10) + `,"offset":0}]}}}`
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Path == "/part":
			return jsonResponse(http.StatusOK, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(600 * time.Millisecond):
				return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected import to verify delivery with a fresh timeout, got %v (stderr=%q)", runErr, stderr)
	}
}

func rewriteIAPImportFileInPlace(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteAt(contents, 0); err != nil {
		return err
	}
	return file.Sync()
}
