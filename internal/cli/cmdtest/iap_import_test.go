package cmdtest

import (
	"context"
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

const iapImportTwoProductsFile = `{
  "products": [
    {
      "type": "CONSUMABLE",
      "referenceName": "Coins",
      "productId": "com.example.coins",
      "localizations": [
        {"locale": "en-US", "name": "Coins", "description": "A pile of coins"}
      ],
      "reviewScreenshot": "shots/coins.png"
    },
    {
      "type": "NON_CONSUMABLE",
      "referenceName": "Pro",
      "productId": "com.example.pro",
      "familySharable": true,
      "localizations": [
        {"locale": "en-US", "name": "Pro"},
        {"locale": "de-DE", "name": "Pro DE", "description": "Keine Limits"}
      ]
    }
  ]
}`

func writeIAPImportFile(t *testing.T, dir, contents string) string {
	t.Helper()

	path := filepath.Join(dir, "iap.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write import file: %v", err)
	}
	return path
}

func writeIAPImportScreenshot(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "shots", "coins.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create screenshot dir: %v", err)
	}
	writePNG(t, path, 40, 40)
	return path
}

func runIAPImport(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func readIAPImportJSONBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()

	if req.Body == nil {
		t.Fatalf("expected request body for %s %s", req.Method, req.URL.Path)
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal body %s: %v", raw, err)
	}
	return payload
}

func iapImportRequestData(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %v", payload)
	}
	return data
}

func iapImportAttributes(t *testing.T, data map[string]any) map[string]any {
	t.Helper()

	attributes, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected attributes object, got %v", data)
	}
	return attributes
}

func TestIAPImportCreatesProductsLocalizationsAndScreenshot(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, iapImportTwoProductsFile)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	screenshotInfo, err := os.Stat(screenshotPath)
	if err != nil {
		t.Fatalf("stat screenshot: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var calls []string
	var createdProducts []map[string]any
	var createdLocalizations []map[string]any

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			data := iapImportRequestData(t, readIAPImportJSONBody(t, req))
			createdProducts = append(createdProducts, data)
			id := fmt.Sprintf("iap-%d", len(createdProducts))
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"inAppPurchases","id":%q,"attributes":{}}}`, id))
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/versions"):
			iapID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v2/inAppPurchases/"), "/versions")
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"inAppPurchaseVersions","id":"version-%s","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`, iapID))
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchaseLocalizations":
			data := iapImportRequestData(t, readIAPImportJSONBody(t, req))
			createdLocalizations = append(createdLocalizations, data)
			id := fmt.Sprintf("loc-%d", len(createdLocalizations))
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"inAppPurchaseLocalizations","id":%q,"attributes":{}}}`, id))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			body := fmt.Sprintf(
				`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`,
				screenshotInfo.Size(), screenshotInfo.Size(),
			)
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	wantCalls := []string{
		"GET /v1/apps/123456789/inAppPurchasesV2",
		"POST /v2/inAppPurchases",
		"GET /v2/inAppPurchases/iap-1/versions",
		"POST /v2/inAppPurchaseLocalizations",
		"POST /v1/inAppPurchaseAppStoreReviewScreenshots",
		"PUT /upload/shot-1",
		"PATCH /v1/inAppPurchaseAppStoreReviewScreenshots/shot-1",
		"GET /v1/inAppPurchaseAppStoreReviewScreenshots/shot-1",
		"POST /v2/inAppPurchases",
		"GET /v2/inAppPurchases/iap-2/versions",
		"POST /v2/inAppPurchaseLocalizations",
		"POST /v2/inAppPurchaseLocalizations",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("unexpected call order:\ngot:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(wantCalls, "\n"))
	}

	if len(createdProducts) != 2 {
		t.Fatalf("expected 2 product creates, got %d", len(createdProducts))
	}
	first := iapImportAttributes(t, createdProducts[0])
	if first["productId"] != "com.example.coins" || first["name"] != "Coins" || first["inAppPurchaseType"] != "CONSUMABLE" {
		t.Fatalf("unexpected first product attributes: %v", first)
	}
	if familySharable, ok := first["familySharable"]; ok && familySharable != false {
		t.Fatalf("expected familySharable to default to false, got %v", familySharable)
	}
	second := iapImportAttributes(t, createdProducts[1])
	if second["productId"] != "com.example.pro" || second["inAppPurchaseType"] != "NON_CONSUMABLE" || second["familySharable"] != true {
		t.Fatalf("unexpected second product attributes: %v", second)
	}

	if len(createdLocalizations) != 3 {
		t.Fatalf("expected 3 localization creates, got %d", len(createdLocalizations))
	}
	localizationAttributes := iapImportAttributes(t, createdLocalizations[0])
	if localizationAttributes["locale"] != "en-US" || localizationAttributes["name"] != "Coins" || localizationAttributes["description"] != "A pile of coins" {
		t.Fatalf("unexpected first localization attributes: %v", localizationAttributes)
	}
	relationships, ok := createdLocalizations[0]["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("expected localization relationships, got %v", createdLocalizations[0])
	}
	version, ok := relationships["version"].(map[string]any)
	if !ok {
		t.Fatalf("expected version relationship, got %v", relationships)
	}
	versionData, ok := version["data"].(map[string]any)
	if !ok || versionData["id"] != "version-iap-1" {
		t.Fatalf("expected version-iap-1 relationship, got %v", version)
	}

	var receipt struct {
		AppID                string `json:"appId"`
		DryRun               bool   `json:"dryRun"`
		Total                int    `json:"total"`
		Created              int    `json:"created"`
		Skipped              int    `json:"skipped"`
		LocalizationsCreated int    `json:"localizationsCreated"`
		ScreenshotsUploaded  int    `json:"screenshotsUploaded"`
		Products             []struct {
			ProductID       string `json:"productId"`
			Status          string `json:"status"`
			InAppPurchaseID string `json:"inAppPurchaseId"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.AppID != "123456789" || receipt.DryRun {
		t.Fatalf("unexpected receipt header: %+v", receipt)
	}
	if receipt.Total != 2 || receipt.Created != 2 || receipt.Skipped != 0 {
		t.Fatalf("unexpected receipt counts: %+v", receipt)
	}
	if receipt.LocalizationsCreated != 3 || receipt.ScreenshotsUploaded != 1 {
		t.Fatalf("unexpected receipt sub-resource counts: %+v", receipt)
	}
	if len(receipt.Products) != 2 || receipt.Products[0].Status != "created" || receipt.Products[1].InAppPurchaseID != "iap-2" {
		t.Fatalf("unexpected receipt products: %+v", receipt.Products)
	}
	if !strings.Contains(stdout, `"createdProductIds":["com.example.coins","com.example.pro"]`) {
		t.Fatalf("expected createdProductIds in receipt, got %s", stdout)
	}
}

func TestIAPImportWithoutSkipExistingFailsBeforeAnyCreate(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, iapImportTwoProductsFile)
	writeIAPImportScreenshot(t, dir)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	mutations := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
			t.Fatalf("unexpected mutation: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Path != "/v1/apps/123456789/inAppPurchasesV2" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"existing-1","attributes":{"productId":"com.example.pro"}}]}`)
	})

	stdout, _, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected an error for an existing product ID without --skip-existing")
	}
	var reported interface{ Reported() bool }
	if errors.As(runErr, &reported) && reported.Reported() {
		t.Fatalf("expected an unreported error so the runner prints it, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "com.example.pro") || !strings.Contains(runErr.Error(), "--skip-existing") {
		t.Fatalf("expected the conflicting product ID and remedy in the error, got %v", runErr)
	}
	if mutations != 0 {
		t.Fatalf("expected zero mutations, got %d", mutations)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestIAPImportSkipExistingSkipsAndReports(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, iapImportTwoProductsFile)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	screenshotInfo, err := os.Stat(screenshotPath)
	if err != nil {
		t.Fatalf("stat screenshot: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	productCreates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"existing-1","attributes":{"productId":"com.example.pro"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			productCreates++
			data := iapImportRequestData(t, readIAPImportJSONBody(t, req))
			if iapImportAttributes(t, data)["productId"] != "com.example.coins" {
				t.Fatalf("expected only the non-existing product to be created, got %v", data)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v2/inAppPurchases/iap-1/versions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchaseLocalizations":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			body := fmt.Sprintf(
				`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`,
				screenshotInfo.Size(), screenshotInfo.Size(),
			)
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--skip-existing",
		"--confirm",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr=%q)", runErr, stderr)
	}
	if productCreates != 1 {
		t.Fatalf("expected 1 product create, got %d", productCreates)
	}

	var receipt struct {
		Created  int `json:"created"`
		Skipped  int `json:"skipped"`
		Products []struct {
			ProductID string `json:"productId"`
			Status    string `json:"status"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.Created != 1 || receipt.Skipped != 1 {
		t.Fatalf("unexpected counts: %+v", receipt)
	}
	if receipt.Products[1].ProductID != "com.example.pro" || receipt.Products[1].Status != "skipped" {
		t.Fatalf("expected the existing product to be reported as skipped, got %+v", receipt.Products)
	}
}

func TestIAPImportDryRunPerformsNoCreates(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, iapImportTwoProductsFile)
	writeIAPImportScreenshot(t, dir)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123456789/inAppPurchasesV2" {
			t.Fatalf("unexpected request during dry run: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"data":[]}`)
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--dry-run",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var receipt struct {
		DryRun               bool `json:"dryRun"`
		Total                int  `json:"total"`
		Created              int  `json:"created"`
		LocalizationsCreated int  `json:"localizationsCreated"`
		Products             []struct {
			Status               string `json:"status"`
			LocalizationsCreated int    `json:"localizationsCreated"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if !receipt.DryRun || receipt.Total != 2 || receipt.Created != 0 || receipt.LocalizationsCreated != 0 {
		t.Fatalf("unexpected dry-run receipt: %+v", receipt)
	}
	for _, product := range receipt.Products {
		if product.Status != "planned" || product.LocalizationsCreated != 0 {
			t.Fatalf("expected planned products with zero created localizations in a dry run, got %+v", receipt.Products)
		}
	}
}

func TestIAPImportRequiresConfirm(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, iapImportTwoProductsFile)
	writeIAPImportScreenshot(t, dir)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request before the apply decision: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--output", "json",
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected the confirm gate on stderr, got %q", stderr)
	}
}

func TestIAPImportRejectsInvalidFilesBeforeAnyRequest(t *testing.T) {
	manyProducts := make([]string, 0, 51)
	for index := 0; index < 51; index++ {
		manyProducts = append(manyProducts, fmt.Sprintf(
			`{"type":"CONSUMABLE","referenceName":"Item %d","productId":"com.example.item%d"}`, index, index,
		))
	}

	tests := []struct {
		name        string
		contents    string
		wantMessage string
	}{
		{
			name:        "unknown top-level field",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro"}],"pricing":{}}`,
			wantMessage: "pricing",
		},
		{
			name:        "unknown product field",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro","price":"3.99"}]}`,
			wantMessage: "price",
		},
		{
			name:        "empty products",
			contents:    `{"products":[]}`,
			wantMessage: "at least one product",
		},
		{
			name:        "unknown type",
			contents:    `{"products":[{"type":"AUTO_RENEWABLE_SUBSCRIPTION","referenceName":"Pro","productId":"com.example.pro"}]}`,
			wantMessage: "type must be one of",
		},
		{
			name:        "missing product id",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro"}]}`,
			wantMessage: "productId is required",
		},
		{
			name:        "missing reference name",
			contents:    `{"products":[{"type":"CONSUMABLE","productId":"com.example.pro"}]}`,
			wantMessage: "referenceName is required",
		},
		{
			name:        "missing locale",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro","localizations":[{"name":"Pro"}]}]}`,
			wantMessage: "locale is required",
		},
		{
			name:        "duplicate product id",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro"},{"type":"CONSUMABLE","referenceName":"Pro 2","productId":"com.example.pro"}]}`,
			wantMessage: "duplicate productId",
		},
		{
			name:        "screenshot is not a regular file",
			contents:    `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro","reviewScreenshot":"shots"}]}`,
			wantMessage: "reviewScreenshot",
		},
		{
			name:        "too many products",
			contents:    `{"products":[` + strings.Join(manyProducts, ",") + `]}`,
			wantMessage: "50",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			dir := t.TempDir()
			filePath := writeIAPImportFile(t, dir, test.contents)
			writeIAPImportScreenshot(t, dir)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request for an invalid file: %s %s", req.Method, req.URL.String())
				return nil, nil
			})

			stdout, stderr, runErr := runIAPImport(t, []string{
				"iap", "import",
				"--app", "123456789",
				"--file", filePath,
				"--confirm",
				"--output", "json",
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected a usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected %q on stderr, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestIAPImportRejectsScreenshotOutsideTheImportRoot(t *testing.T) {
	setupAuth(t)

	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.png")
	writePNG(t, outside, 40, 40)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, fmt.Sprintf(
		`{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro","reviewScreenshot":%q}]}`,
		outside,
	))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request for an out-of-root screenshot: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "must stay inside") {
		t.Fatalf("expected a containment error on stderr, got %q", stderr)
	}
}

func TestIAPImportReportsAcceptedProductWhenCreateResponseHasNoID(t *testing.T) {
	setupAuth(t)
	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins"}]}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	createCalls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			createCalls++
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","attributes":{}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, _, runErr := runIAPImport(t, []string{
		"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "response did not include an id") || createCalls != 1 {
		t.Fatalf("run error = %v, create calls = %d; want one accepted create with missing ID", runErr, createCalls)
	}

	var receipt struct {
		Created           int      `json:"created"`
		CreatedProductIDs []string `json:"createdProductIds"`
		FailedProductID   string   `json:"failedProductId"`
		Products          []struct {
			Status string `json:"status"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.Created != 1 || len(receipt.CreatedProductIDs) != 1 || receipt.CreatedProductIDs[0] != "com.example.coins" || receipt.FailedProductID != "com.example.coins" {
		t.Fatalf("accepted product missing from partial receipt: %+v", receipt)
	}
	if len(receipt.Products) != 1 || receipt.Products[0].Status != "failed" {
		t.Fatalf("expected one failed product in receipt, got %+v", receipt.Products)
	}
}

func TestIAPImportReportsProductCreatedBeforeALocalizationFailure(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{
  "products": [
    {
      "type": "CONSUMABLE",
      "referenceName": "Coins",
      "productId": "com.example.coins",
      "localizations": [{"locale": "en-US", "name": "Coins"}]
    },
    {"type": "CONSUMABLE", "referenceName": "Gems", "productId": "com.example.gems"}
  ]
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	productCreates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			productCreates++
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v2/inAppPurchases/iap-1/versions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchaseLocalizations":
			return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"Conflict","detail":"locale rejected"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, _, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected the localization failure to return an error")
	}
	if productCreates != 1 {
		t.Fatalf("expected the import to stop after the failing product, got %d creates", productCreates)
	}

	var receipt struct {
		Created           int      `json:"created"`
		CreatedProductIDs []string `json:"createdProductIds"`
		FailedProductID   string   `json:"failedProductId"`
		Products          []struct {
			ProductID       string `json:"productId"`
			Status          string `json:"status"`
			InAppPurchaseID string `json:"inAppPurchaseId"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.Created != 1 || len(receipt.CreatedProductIDs) != 1 || receipt.CreatedProductIDs[0] != "com.example.coins" {
		t.Fatalf("expected the created in-app purchase in the receipt, got %+v", receipt)
	}
	if receipt.FailedProductID != "com.example.coins" {
		t.Fatalf("expected the failing product recorded, got %+v", receipt)
	}
	if len(receipt.Products) != 1 || receipt.Products[0].Status != "failed" || receipt.Products[0].InAppPurchaseID != "iap-1" {
		t.Fatalf("expected a failed product carrying its in-app purchase ID, got %+v", receipt.Products)
	}
}

func TestIAPImportStopsOnMidImportFailureAndReportsCreatedProducts(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{
  "products": [
    {"type": "CONSUMABLE", "referenceName": "Coins", "productId": "com.example.coins"},
    {"type": "CONSUMABLE", "referenceName": "Gems", "productId": "com.example.gems"},
    {"type": "CONSUMABLE", "referenceName": "Stars", "productId": "com.example.stars"}
  ]
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	productCreates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			productCreates++
			if productCreates == 2 {
				return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"Conflict","detail":"product rejected"}]}`)
			}
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"inAppPurchases","id":"iap-%d","attributes":{}}}`, productCreates))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, _, runErr := runIAPImport(t, []string{
		"iap", "import",
		"--app", "123456789",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected a mid-import failure to return an error")
	}
	if productCreates != 2 {
		t.Fatalf("expected the import to stop after the failing product, got %d creates", productCreates)
	}

	var receipt struct {
		Created           int      `json:"created"`
		CreatedProductIDs []string `json:"createdProductIds"`
		FailedProductID   string   `json:"failedProductId"`
		Error             string   `json:"error"`
		Products          []struct {
			ProductID string `json:"productId"`
			Status    string `json:"status"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.Created != 1 || len(receipt.CreatedProductIDs) != 1 || receipt.CreatedProductIDs[0] != "com.example.coins" {
		t.Fatalf("expected the already-created product in the receipt, got %+v", receipt)
	}
	if receipt.FailedProductID != "com.example.gems" || receipt.Error == "" {
		t.Fatalf("expected the failing product recorded, got %+v", receipt)
	}
	if len(receipt.Products) != 2 || receipt.Products[1].Status != "failed" {
		t.Fatalf("expected the import to stop before later products, got %+v", receipt.Products)
	}
}
