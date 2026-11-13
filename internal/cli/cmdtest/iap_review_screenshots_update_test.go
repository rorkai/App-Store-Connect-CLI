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

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestIAPReviewScreenshotsUpdateSendsSupportedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Type != asc.ResourceTypeInAppPurchaseAppStoreReviewScreenshots {
			t.Fatalf("expected screenshot resource type, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "shot-1" {
			t.Fatalf("expected screenshot ID shot-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil {
			t.Fatal("expected update attributes")
		}
		if payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded=true, got %#v", payload.Data.Attributes.Uploaded)
		}
		if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != "checksum-1" {
			t.Fatalf("expected sourceFileChecksum checksum-1, got %#v", payload.Data.Attributes.SourceFileChecksum)
		}

		return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true,"sourceFileChecksum":"checksum-1"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--checksum", "checksum-1",
			"--uploaded", "true",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if requestCount != 1 {
		t.Fatalf("expected one PATCH request, got %d", requestCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var output struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				SourceFileChecksum string `json:"sourceFileChecksum"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode output: %v\nstdout=%s", err, stdout)
	}
	if output.Data.Type != string(asc.ResourceTypeInAppPurchaseAppStoreReviewScreenshots) || output.Data.ID != "shot-1" || output.Data.Attributes.SourceFileChecksum != "checksum-1" {
		t.Fatalf("expected returned screenshot resource, got %s", stdout)
	}
}

func TestIAPReviewScreenshotsUpdateSupportsIndividualFields(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantChecksum *string
		wantUploaded *bool
	}{
		{
			name:         "checksum only",
			args:         []string{"--checksum", "checksum-1"},
			wantChecksum: new("checksum-1"),
		},
		{
			name:         "uploaded false",
			args:         []string{"--uploaded", "false"},
			wantUploaded: new(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if payload.Data.Attributes == nil {
					t.Fatal("expected update attributes")
				}
				if test.wantChecksum == nil && payload.Data.Attributes.SourceFileChecksum != nil {
					t.Fatalf("expected checksum to be omitted, got %q", *payload.Data.Attributes.SourceFileChecksum)
				}
				if test.wantChecksum != nil && (payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != *test.wantChecksum) {
					t.Fatalf("expected checksum %q, got %#v", *test.wantChecksum, payload.Data.Attributes.SourceFileChecksum)
				}
				if test.wantUploaded == nil && payload.Data.Attributes.Uploaded != nil {
					t.Fatalf("expected uploaded to be omitted, got %t", *payload.Data.Attributes.Uploaded)
				}
				if test.wantUploaded != nil && (payload.Data.Attributes.Uploaded == nil || *payload.Data.Attributes.Uploaded != *test.wantUploaded) {
					t.Fatalf("expected uploaded %t, got %#v", *test.wantUploaded, payload.Data.Attributes.Uploaded)
				}
				return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"sourceFileChecksum":"checksum-1"}}}`)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1"}, test.args...)
			args = append(args, "--output", "json")
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr != nil {
				t.Fatalf("expected success, got %v", runErr)
			}
			if stdout == "" || stderr != "" {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestIAPReviewScreenshotsUpdateRejectsConfirmWithoutFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "review-screenshots", "update",
		"--screenshot-id", "shot-1",
		"--uploaded", "true",
		"--confirm",
		"--output", "json",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "--confirm can only be used with --file") {
		t.Fatalf("expected confirm usage error, got %v", runErr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no network requests, got %d", requestCount)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm can only be used with --file") {
		t.Fatalf("expected usage diagnostic on stderr, got %q", stderr)
	}
}

func TestIAPReviewScreenshotsUpdateUsesRegisteredTableRenderer(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":42,"assetType":"SCREENSHOT"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--uploaded", "false",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "shot-1") {
		t.Fatalf("expected screenshot table output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestIAPReviewScreenshotsUpdateRequiresAnUpdateFieldBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no update flag"},
		{name: "blank checksum counts as missing", args: []string{"--checksum", "  "}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			requestCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			})

			args := append([]string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1"}, test.args...)
			stdout, stderr, runErr := runRootCommand(t, append(args, "--output", "json"))
			if runErr == nil || !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected reported usage error, got %v", runErr)
			}
			if !strings.Contains(stderr, "Error: at least one update flag is required") {
				t.Fatalf("expected missing update field diagnostic, got %q", stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if requestCount != 0 {
				t.Fatalf("expected no requests, got %d", requestCount)
			}
		})
	}
}

func TestIAPReviewScreenshotsUpdateReuploadsExistingInProgressScreenshot(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	filePath := filepath.Join(t.TempDir(), "review.png")
	writeReviewScreenshotPNG(t, filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	checksum, err := asc.ComputeFileChecksum(filePath, asc.ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatalf("compute fixture checksum: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var requests []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" && len(requests) == 0:
			requests = append(requests, "GET old")
			return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", true, "UPLOADING", info.Size()))
		case req.Method == http.MethodPut && req.URL.Path == "/part":
			requests = append(requests, "PUT")
			return jsonResponse(http.StatusOK, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			requests = append(requests, "PATCH old")
			var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode commit payload: %v", err)
			}
			if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
				t.Fatalf("expected uploaded=true, got %#v", payload.Data.Attributes)
			}
			if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != checksum.Hash {
				t.Fatalf("expected checksum %q, got %#v", checksum.Hash, payload.Data.Attributes.SourceFileChecksum)
			}
			return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", false, "UPLOADING", info.Size()))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" && len(requests) > 0:
			requests = append(requests, "GET final")
			return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", false, "COMPLETE", info.Size()))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "review-screenshots", "update",
		"--screenshot-id", "shot-1",
		"--file", filePath,
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected file update success, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if got, want := strings.Join(requests, ","), "GET old,PUT,PATCH old,GET final"; got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
	if strings.Contains(stdout, "deprecated") {
		t.Fatalf("unexpected deprecation output: %q", stdout)
	}
}

func TestIAPReviewScreenshotsUpdateRejectsInProgressFileSizeMismatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	filePath := filepath.Join(t.TempDir(), "review.png")
	writeReviewScreenshotPNG(t, filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var requests []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requests = append(requests, "GET old")
		return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", true, "UPLOADING", info.Size()+1))
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "review-screenshots", "update",
		"--screenshot-id", "shot-1",
		"--file", filePath,
		"--output", "json",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "does not match") {
		t.Fatalf("expected file-size mismatch, got %v", runErr)
	}
	if diagnostic, ok := shared.DiagnosticFromError(runErr); !shared.IsValidationError(runErr) || !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--file" {
		t.Fatalf("expected an invalid --file validation error, got %v (diagnostic %+v, %t)", runErr, diagnostic, ok)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no command output, stdout=%q stderr=%q", stdout, stderr)
	}
	if got, want := strings.Join(requests, ","), "GET old"; got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestIAPReviewScreenshotsUpdateRejectsCompletedScreenshotWithDeleteThenCreate(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "without confirm"},
		{name: "with confirm", args: []string{"--confirm"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			filePath := filepath.Join(t.TempDir(), "replacement.png")
			writeReviewScreenshotPNG(t, filePath)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			var requests []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/old-shot" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				requests = append(requests, "GET old")
				return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("old-shot", false, "COMPLETE", 1))
			})

			args := append([]string{
				"iap", "review-screenshots", "update",
				"--screenshot-id", "old-shot",
				"--file", filePath,
				"--output", "json",
			}, test.args...)
			code, stdout, stderr := runASCForExitCode(t, args...)
			if code != rootcmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitConflict, stderr)
			}
			quotedPath, ok := shared.ShellQuote(filePath)
			if !ok {
				t.Fatalf("fixture path %q is not shell-quotable", filePath)
			}
			want := "Error: iap review-screenshots update: App Store Connect does not support replacing completed screenshot old-shot through update; run asc iap review-screenshots delete --screenshot-id old-shot --confirm, then asc iap review-screenshots create --iap-id iap-1 --file " + quotedPath + "\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if got, want := strings.Join(requests, ","), "GET old"; got != want {
				t.Fatalf("request sequence = %q, want %q", got, want)
			}
		})
	}
}

func iapReviewScreenshotResponse(id string, hasUploadOperations bool, state string, fileSize int64) string {
	attributes := fmt.Sprintf(`"fileSize":%d,"assetDeliveryState":{"state":%q}`, fileSize, state)
	if hasUploadOperations {
		attributes += fmt.Sprintf(`,"uploadOperations":[{"method":"PUT","url":"https://uploads.example/part","offset":0,"length":%d}]`, fileSize)
	}
	relationships := `,"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}`
	return fmt.Sprintf(`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":%q,"attributes":{%s}%s},"links":{}}`, id, attributes, relationships)
}

func TestIAPReviewScreenshotsUpdateRejectsInvalidUploadedFlag(t *testing.T) {
	setupUsageExitCodeEnv(t)
	assertUsageExit(t, []string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1", "--uploaded", "maybe"}, `invalid value "maybe" for flag -uploaded`)
}

func TestIAPReviewScreenshotsUpdatePreservesAPIValidationError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"INVALID_SOURCE_FILE_CHECKSUM","title":"Invalid Attribute","detail":"checksum does not match uploaded asset"}]}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--checksum", "bad-checksum",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected API validation error")
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) {
		t.Fatalf("expected APIError in chain, got %v", runErr)
	}
	if apiErr.Code != "INVALID_SOURCE_FILE_CHECKSUM" || apiErr.Detail != "checksum does not match uploaded asset" {
		t.Fatalf("expected API code/detail to be preserved, got %#v", apiErr)
	}
	if !strings.Contains(runErr.Error(), "checksum does not match uploaded asset") {
		t.Fatalf("expected API detail in error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no command output before top-level formatting, stdout=%q stderr=%q", stdout, stderr)
	}
}
