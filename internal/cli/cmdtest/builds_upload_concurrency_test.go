package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsUploadDryRunRejectsExplicitConcurrency(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	asctest.NewRegistry(t).Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--dry-run",
			"--concurrency", "2",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected builds upload to fail, got nil")
	}
	if !strings.Contains(runErr.Error(), "--concurrency is not supported with --dry-run") {
		t.Fatalf("expected dry-run concurrency rejection, got %v", runErr)
	}
}

func TestBuildsUploadReservationFlagsReserveWithoutUploading(t *testing.T) {
	const dryRunWarning = "Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles). In a future release, --dry-run will check the artifact locally without contacting App Store Connect. Use --reserve-only to keep reserving uploads.\n"

	tests := []struct {
		flag        string
		wantWarning bool
	}{
		{flag: "--reserve-only"},
		{flag: "--dry-run", wantWarning: true},
	}
	for _, test := range tests {
		t.Run(test.flag, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			ipaPath := writeBuildUploadIPA(t, "com.example.demo")

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/123456789"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`))
			reg.Register(asctest.POST("/v1/buildUploads").JSONBody(`{"data":{"type":"buildUploads","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`))
			reg.Register(asctest.POST("/v1/buildUploadFiles").JSONBody(buildUploadFileRequestBody(t, ipaPath)),
				asctest.JSON(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"builds", "upload",
					"--app", "123456789",
					"--ipa", ipaPath,
					"--version", "1.0.0",
					"--build-number", "42",
					test.flag,
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr != nil {
				t.Fatalf("expected %s to succeed with default concurrency, got %v", test.flag, runErr)
			}
			if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
				t.Fatalf("expected reserved upload in output, got %q", stdout)
			}
			if strings.Contains(stdout, "https://upload.example.com/part-1") ||
				strings.Contains(stdout, "application/octet-stream") {
				t.Fatalf("default %s output exposed upload capabilities: %q", test.flag, stdout)
			}
			if !strings.Contains(stdout, `"(redacted)"`) {
				t.Fatalf("default %s output did not mark withheld upload capabilities: %q", test.flag, stdout)
			}
			if got := strings.Contains(stderr, dryRunWarning); got != test.wantWarning {
				t.Fatalf("stderr has the --dry-run warning = %t, want %t; stderr %q", got, test.wantWarning, stderr)
			}
			for _, req := range reg.Requests() {
				if req.Method != http.MethodGet && req.Method != http.MethodPost {
					t.Fatalf("expected only the app read and the reservation, got %s %s", req.Method, req.URL)
				}
			}
		})
	}
}

func TestBuildsUploadDryRunRequiresExplicitOptInForUploadCapabilities(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`))
	reg.Register(asctest.POST("/v1/buildUploads").JSONBody(`{"data":{"type":"buildUploads","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`))
	reg.Register(asctest.POST("/v1/buildUploadFiles").JSONBody(buildUploadFileRequestBody(t, ipaPath)),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1?Signature=signed-secret","length":4,"offset":0,"requestHeaders":[{"name":"Authorization","value":"header-secret"}]}]}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--dry-run",
			"--include-sensitive",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	for _, secret := range []string{"signed-secret", "header-secret"} {
		if !strings.Contains(stdout, secret) {
			t.Fatalf("--include-sensitive output omitted %q: %q", secret, stdout)
		}
	}
	if !strings.Contains(stderr, "Warning: --include-sensitive prints secrets") {
		t.Fatalf("expected sensitive-output warning, got %q", stderr)
	}
}
