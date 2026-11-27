package reviews

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReviewValidationDiagnosticsPreserveErrorContracts(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	notFoundDir := t.TempDir()
	t.Chdir(notFoundDir)
	notFoundPath := filepath.Join(notFoundDir, "missing.json")

	tests := []struct {
		name       string
		run        func(d *shared.Deps) error
		wantError  string
		wantStderr string
		wantUsage  bool
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "reviews missing app",
			run:        func(d *shared.Deps) error { return NewReviewsCommand(d).ParseAndRun(context.Background(), nil) },
			wantError:  "--app is required (or set ASC_APP_ID)",
			wantStderr: "Error: --app is required (or set ASC_APP_ID)\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--app",
		},
		{
			name: "reviews invalid limit",
			run: func(d *shared.Deps) error {
				return NewReviewsCommand(d).ParseAndRun(context.Background(), []string{"--app", "app-1", "--limit", "201"})
			},
			wantError:  "reviews: --limit must be between 1 and 200",
			wantStderr: "Error: reviews: --limit must be between 1 and 200\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--limit",
		},
		{
			name: "ratings invalid workers",
			run: func(d *shared.Deps) error {
				return newReviewsRatingsCommand(d).ParseAndRun(context.Background(), []string{"--app", "123", "--workers", "0"})
			},
			wantError:  flag.ErrHelp.Error(),
			wantStderr: "Error: --workers must be at least 1\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--workers",
		},
		{
			name: "respond batch missing file",
			run: func(d *shared.Deps) error {
				return newReviewsRespondBatchCommand(d).ParseAndRun(context.Background(), []string{"--app", "app-1", "--dry-run"})
			},
			wantError:  "--file is required",
			wantStderr: "Error: --file is required\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--file",
		},
		{
			name: "respond batch file not found",
			run: func(d *shared.Deps) error {
				return newReviewsRespondBatchCommand(d).ParseAndRun(context.Background(), []string{"--app", "app-1", "--file", notFoundPath, "--dry-run"})
			},
			wantUsage: true,
			wantCode:  shared.DiagnosticFileNotFound,
			wantParam: "--file",
		},
		{
			name: "attachment file not found",
			run: func(d *shared.Deps) error {
				return newReviewDetailsAttachmentsUploadCommand(d).ParseAndRun(context.Background(), []string{"--review-detail", "detail-1", "--file", notFoundPath})
			},
			wantCode:  shared.DiagnosticFileNotFound,
			wantParam: "--file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, _, stderrBuffer := newReviewTestDeps(nil)
			err := test.run(d)
			stderr := stderrBuffer.String()
			if err == nil {
				t.Fatal("expected error")
			}
			if test.wantError != "" && err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if test.wantStderr != "" && !strings.HasPrefix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantStderr)
			}
			if got := errors.Is(err, flag.ErrHelp); got != test.wantUsage {
				t.Fatalf("errors.Is(flag.ErrHelp) = %t, want %t", got, test.wantUsage)
			}
			if !test.wantUsage && !shared.IsValidationError(err) {
				t.Fatalf("error = %v, want non-usage validation marker", err)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

func TestReviewSubmitInputFailuresKeepStructuredDiagnostics(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		args       []string
		wantStderr string
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "missing app",
			args:       nil,
			wantStderr: "Error: --app is required (or set ASC_APP_ID)\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--app",
		},
		{
			name:       "missing build",
			args:       []string{"--app", "123456789"},
			wantStderr: "Error: --build-id, --build-number, or --latest is required\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--build-id",
		},
		{
			name:       "missing version selector",
			args:       []string{"--app", "123456789", "--build-id", "BUILD_ID"},
			wantStderr: "Error: --version or --version-id is required\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "",
		},
		{
			name:       "conflicting version selectors",
			args:       []string{"--app", "123456789", "--build-id", "BUILD_ID", "--version", "1.2.3", "--version-id", "VERSION_ID"},
			wantStderr: "Error: --version and --version-id are mutually exclusive\n",
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "",
		},
		{
			name:       "missing confirm",
			args:       []string{"--app", "123456789", "--build-id", "BUILD_ID", "--version", "1.2.3"},
			wantStderr: "Error: --confirm is required unless --dry-run is set\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--confirm",
		},
		{
			name:       "unsupported platform",
			args:       []string{"--app", "123456789", "--build-id", "BUILD_ID", "--version", "1.2.3", "--dry-run", "--platform", "WATCH_OS"},
			wantStderr: "Error: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--platform",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, _, stderrBuffer := newReviewTestDeps(nil)
			command := newReviewSubmitCommand(d)
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := command.Exec(context.Background(), nil)
			stderr := stderrBuffer.String()
			if err == nil {
				t.Fatal("expected error")
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

func TestReviewAttachmentUnreadableSourceIsDiagnosedBeforeAuth(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "attachment.pdf")
	if err := os.WriteFile(attachmentPath, []byte("review attachment"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	if err := os.Chmod(attachmentPath, 0); err != nil {
		t.Fatalf("make attachment unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(attachmentPath, 0o600) })

	probe, probeErr := os.Open(attachmentPath)
	if probeErr == nil {
		_ = probe.Close()
		t.Skip("current user can open mode-000 files")
	}
	if !errors.Is(probeErr, os.ErrPermission) {
		t.Skipf("cannot reproduce permission-denied open: %v", probeErr)
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	d, _, _ := newReviewTestDeps(nil)
	err := newReviewDetailsAttachmentsUploadCommand(d).ParseAndRun(context.Background(), []string{
		"--review-detail", "detail-1",
		"--file", attachmentPath,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !shared.IsValidationError(err) {
		t.Fatalf("error = %v, want validation marker", err)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("expected structured diagnostic, got %v", err)
	}
	if diagnostic.Code != shared.DiagnosticFilePermissionDenied || diagnostic.Parameter != "--file" {
		t.Fatalf("diagnostic = %+v, want file_permission_denied for --file", diagnostic)
	}
	if !strings.HasPrefix(err.Error(), "review attachments-upload: upload failed: ") {
		t.Fatalf("error = %q, want preserved upload failure prefix", err)
	}
}

func newReviewTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:    client,
		IO:        &shared.IO{Out: &stdout, ErrOut: &stderr},
		Timeout:   shared.ContextWithTimeout,
		Getwd:     os.Getwd,
		LookupEnv: os.LookupEnv,
	}, &stdout, &stderr
}
