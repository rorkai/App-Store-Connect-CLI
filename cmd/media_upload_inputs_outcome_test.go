package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// TestRunMediaUploadInputFailuresAreClassified covers local --path and
// --device-type failures of the media upload commands. They fail before any
// client is created, keep their message and exit code, and must be reported
// as expected negatives instead of internal errors.
func TestRunMediaUploadInputFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")
	junk := filepath.Join(dir, "junk.png")
	if err := os.WriteFile(junk, []byte("junk\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", junk, err)
	}
	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o700); err != nil {
		t.Fatalf("Mkdir(%q) error: %v", emptyDir, err)
	}
	_, lstatErr := os.Lstat(missing)
	if lstatErr == nil {
		t.Fatalf("os.Lstat(%q) unexpectedly succeeded", missing)
	}

	customScreenshots := func(path string, extra ...string) []string {
		return append([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC", "--path", path, "--device-type", "IPHONE_65"}, extra...)
	}
	customPreviews := func(path string) []string {
		return []string{"product-pages", "custom-pages", "localizations", "preview-sets", "upload", "--localization-id", "LOC", "--path", path, "--device-type", "IPHONE_65"}
	}
	treatmentScreenshots := func(path string) []string {
		return []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC", "--path", path, "--device-type", "IPHONE_65"}
	}
	videoPreviews := func(path, deviceType string) []string {
		return []string{"video-previews", "upload", "--version-localization", "LOC", "--path", path, "--device-type", deviceType}
	}
	screenshots := func(path string) []string {
		return []string{"screenshots", "upload", "--version-localization", "LOC", "--path", path, "--device-type", "IPHONE_65"}
	}

	tests := []struct {
		name             string
		args             []string
		wantStderrPrefix string
		wantDiagnosis    shared.DiagnosticCode
		wantParameter    string
	}{
		{
			name:             "custom page screenshots missing path",
			args:             customScreenshots(missing),
			wantStderrPrefix: "Error: custom-pages localizations screenshot-sets upload: " + lstatErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "custom page screenshots undecodable image",
			args:             customScreenshots(junk),
			wantStderrPrefix: "Error: custom-pages localizations screenshot-sets upload: \"" + junk + "\": decode image format: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
		{
			name:             "custom page screenshots empty directory",
			args:             customScreenshots(emptyDir),
			wantStderrPrefix: "Error: custom-pages localizations screenshot-sets upload: no files found in \"" + emptyDir + "\"\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "custom page screenshots unknown device type",
			args:             []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC", "--path", junk, "--device-type", "NOT_A_DEVICE"},
			wantStderrPrefix: "Error: custom-pages localizations screenshot-sets upload: ",
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--device-type",
		},
		{
			name:             "treatment screenshots missing path",
			args:             treatmentScreenshots(missing),
			wantStderrPrefix: "Error: experiments treatments localizations screenshot-sets upload: " + lstatErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "custom page previews unsupported extension",
			args:             customPreviews(junk),
			wantStderrPrefix: "Error: custom-pages localizations preview-sets upload: unsupported preview file extension \".png\"; supported extensions are .mov, .m4v, and .mp4\n",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
		{
			name:             "custom page previews unknown device type",
			args:             []string{"product-pages", "custom-pages", "localizations", "preview-sets", "upload", "--localization-id", "LOC", "--path", junk, "--device-type", "NOT_A_DEVICE"},
			wantStderrPrefix: "Error: custom-pages localizations preview-sets upload: ",
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--device-type",
		},
		{
			name:             "video previews unsupported extension",
			args:             videoPreviews(junk, "IPHONE_65"),
			wantStderrPrefix: "Error: video-previews upload: unsupported preview file extension \".png\"; supported extensions are .mov, .m4v, and .mp4\n",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
		{
			name:             "video previews missing path",
			args:             videoPreviews(missing, "IPHONE_65"),
			wantStderrPrefix: "Error: video-previews upload: " + lstatErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "video previews unknown device type",
			args:             videoPreviews(junk, "NOT_A_DEVICE"),
			wantStderrPrefix: "Error: video-previews upload: ",
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--device-type",
		},
		{
			name:             "screenshots undecodable image",
			args:             screenshots(junk),
			wantStderrPrefix: "Error: screenshots upload: \"" + junk + "\": decode image format: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
		{
			name:             "screenshots missing path",
			args:             screenshots(missing),
			wantStderrPrefix: "Error: screenshots upload: " + lstatErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			tempDir := t.TempDir()
			t.Setenv("HOME", filepath.Join(tempDir, "home"))
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				t.Fatal("unexpected client creation for a local input failure")
				return nil, nil
			}))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			var code int
			stdout, stderr := captureCommandOutput(t, func() {
				code = Run(test.args, "5.15.0")
			})

			if code != ExitError {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, ExitError, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderrPrefix) || strings.Count(stderr, "Error:") != 1 {
				t.Fatalf("stderr = %q, want one error line with prefix %q", stderr, test.wantStderrPrefix)
			}
			if gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry context = %+v, want outcome %q, diagnostic %q, parameter %q",
					gotContext, telemetry.OutcomeExpectedNegative, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
