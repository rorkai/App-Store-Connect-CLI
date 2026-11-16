package distribute

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestDistributionHumanOutputUsesTypedPlanRunAndVerificationRows(t *testing.T) {
	digest := strings.Repeat("1", 64)
	plan := &distributionPlan{
		SchemaVersion: 1,
		PlanID:        "dplan_11111111111111111111111111111111",
		PlanHash:      digest,
		CreatedAt:     "2026-08-13T08:00:00Z",
		Ready:         true,
		ConfigPath:    "/private/config/distribution.json",
		ConfigSHA256:  digest,
		Archive: distributionArchiveBinding{
			BundleID: "com.example.agent", Title: "Agent Preview", PublishedTitle: "Agent Preview", Version: "1.2.3", BuildNumber: "42", MinimumOSVersion: "17.0",
			TeamID: "TEAM123", TreeSHA256: digest, Path: "/private/archive/App.xcarchive", SizeBytes: 1234, FileCount: 12, TargetCount: 1,
		},
		DeviceSet: distributionDeviceSetBinding{SHA256: digest, FileSHA256: digest, Count: 2},
		Identity: distributionIdentityBinding{
			CertificateResourceID: "CERT123", CertificateSHA256: digest, TeamID: "TEAM123",
		},
		Reconcile: distributionReconcileBinding{PlanPath: "/private/reconcile/plan.json", PlanHash: digest, ReceiptPath: "/private/reconcile/receipt.json", MutationCount: 1},
		Effects:   []distributionEffect{{Stage: "account_reconcile", Kind: "register_device", Count: 1}},
		Paths:     distributionPlanPaths{StateDir: "/private/state"},
	}
	run := &distributionRunState{
		SchemaVersion:   1,
		RunID:           "drun_11111111111111111111111111111111",
		PlanID:          plan.PlanID,
		PlanHash:        digest,
		Status:          "recoverable",
		Stage:           "publish",
		Attempt:         2,
		Recoverable:     true,
		UpdatedAt:       "2026-08-13T08:01:00Z",
		LastFailureCode: "provider_outcome_unknown",
		Artifacts: distributionRunArtifacts{
			IPA:         &distributionSizedFileArtifact{SHA256: digest, SizeBytes: 42},
			Publication: &distributionPublicationArtifact{ReceiptPath: "publish/receipt.json", ReceiptSHA256: digest, LinkPath: "secrets/publication-intent.json", LinkSHA256: digest},
		},
	}
	verification := &distributionVerificationResult{
		SchemaVersion:       1,
		RunID:               run.RunID,
		PlanHash:            digest,
		PublicationVerified: true,
		VerifiedAt:          "2026-08-13T08:02:00Z",
		ArtifactSHA256:      digest,
		AppBundleID:         "com.example.agent",
		AppVersion:          "1.2.3",
		AppBuildNumber:      "42",
		DeviceObservation: &deviceObservation{
			Requested: true, DeviceFound: true, AppInstalled: true,
			BundleID: "com.example.agent", Version: "1.2.3", Build: "42",
		},
	}

	tests := []struct {
		name   string
		value  any
		format string
		want   []string
	}{
		{name: "plan table", value: plan, format: "table", want: []string{"Field", "Value", "Plan Hash", "Config Path", "/private/config/distribution.json", "Archive Path", "Archive Size", "Archive Files", "Archive Targets", "Identity Team ID", "Reconcile Plan Path", "Reconcile Receipt Path", "Storage Endpoint", "Run State Directory", "Effect 1", "register_device"}},
		{name: "run markdown", value: run, format: "markdown", want: []string{"| Field", "| Value", "Run ID", "Last Failure Code", "provider_outcome_unknown", "Publication Receipt Path", "Private Link Artifact", "Install URL (Redacted)"}},
		{name: "verification table", value: verification, format: "table", want: []string{"Publication Verified", "Device Found", "App Installed", "com.example.agent"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := newDistributeTestDeps()
			stdout, stderr, err := captureDistributionCommandOutput(t, d, func() error {
				return printDistributionValue(d, test.value, test.format, false)
			})
			if err != nil {
				t.Fatalf("printDistributionValue() error = %v", err)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			for _, fragment := range test.want {
				if !strings.Contains(stdout, fragment) {
					t.Fatalf("%s output missing %q:\n%s", test.format, fragment, stdout)
				}
			}
			if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
				t.Fatalf("%s output fell back to raw JSON: %s", test.format, stdout)
			}
		})
	}
}

func TestDistributionApplyWellFormedConfirmationMismatchRemainsUsageError(t *testing.T) {
	confirmation := strings.Repeat("1", 64)
	d := newDistributeTestDeps()
	cmd := distributionApplyCommandWithExecutor(d, func(_ context.Context, executorDeps *shared.Deps, request distributionApplyRequest) (*distributionRunState, error) {
		if request.Confirmation != confirmation {
			t.Fatalf("confirmation = %q", request.Confirmation)
		}
		return nil, executorDeps.UsageError("--confirm must be the exact planHash")
	})
	if err := cmd.FlagSet.Parse([]string{"--plan", "/private/plan.json", "--confirm", confirmation, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := captureDistributionCommandOutput(t, d, func() error {
		return cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})
	if got := shared.ClassifyUsageError(err); got != shared.UsageErrorInvalidValue {
		t.Fatalf("usage classification = %q, want %q (error=%v)", got, shared.UsageErrorInvalidValue, err)
	}
	if stdout != "" || stderr != "Error: --confirm must be the exact planHash\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestDistributionApplyPrintsJSONResultBeforeReturningOperationalError(t *testing.T) {
	runID := "drun_11111111111111111111111111111111"
	wantErr := errors.New("provider result remains unknown")
	d := newDistributeTestDeps()
	cmd := distributionApplyCommandWithExecutor(d, func(context.Context, *shared.Deps, distributionApplyRequest) (*distributionRunState, error) {
		return &distributionRunState{SchemaVersion: 1, RunID: runID, Status: "recoverable"}, wantErr
	})
	if err := cmd.FlagSet.Parse([]string{
		"--plan", "/private/plan.json",
		"--confirm", strings.Repeat("1", 64),
		"--output", "json",
	}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := captureDistributionCommandOutput(t, d, func() error {
		return cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped operational error", err)
	}
	if stderr != "" {
		t.Fatalf("command wrote operational error to stderr directly: %q", stderr)
	}
	if strings.Contains(stdout, wantErr.Error()) {
		t.Fatalf("operational diagnostic leaked into JSON stdout: %q", stdout)
	}
	var result distributionRunState
	if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", decodeErr, stdout)
	}
	if result.RunID != runID || result.Status != "recoverable" {
		t.Fatalf("JSON result = %#v", result)
	}
}

func TestDistributionVerifyRejectsInvalidDeviceTimeoutBeforeExecution(t *testing.T) {
	for _, timeout := range []string{"4s", "6m"} {
		t.Run(timeout, func(t *testing.T) {
			executeCalls := 0
			cmd := distributionVerifyCommandWithExecutor(newDistributeTestDeps(), func(context.Context, *shared.Deps, distributionVerifyRequest) (*distributionVerificationResult, error) {
				executeCalls++
				return nil, nil
			})
			if err := cmd.FlagSet.Parse([]string{
				"--run", "drun_11111111111111111111111111111111",
				"--device", "phone",
				"--timeout", timeout,
			}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Exec(context.Background(), cmd.FlagSet.Args())
			if got := shared.ClassifyUsageError(err); got != shared.UsageErrorInvalidValue {
				t.Fatalf("usage classification = %q, want %q (error=%v)", got, shared.UsageErrorInvalidValue, err)
			}
			if executeCalls != 0 {
				t.Fatalf("executor calls = %d, want none", executeCalls)
			}
		})
	}
}

// captureDistributionCommandOutput runs run and returns what it wrote to d's
// stdout and stderr. It fails the test if run wrote to the process stdout or
// stderr.
func captureDistributionCommandOutput(t *testing.T, d *shared.Deps, run func() error) (string, string, error) {
	t.Helper()
	processStdout, processStderr, err := captureProcessOutput(t, run)
	if processStdout != "" || processStderr != "" {
		t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
	}
	return d.IO.Out.(*bytes.Buffer).String(), d.IO.ErrOut.(*bytes.Buffer).String(), err
}

func captureProcessOutput(t *testing.T, run func() error) (string, string, error) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatal(err)
	}
	restored := false
	defer func() {
		if restored {
			return
		}
		os.Stdout, os.Stderr = oldStdout, oldStderr
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	runErr := run()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	restored = true
	stdout, stdoutErr := io.ReadAll(stdoutReader)
	stderr, stderrErr := io.ReadAll(stderrReader)
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	if stdoutErr != nil {
		t.Fatal(stdoutErr)
	}
	if stderrErr != nil {
		t.Fatal(stderrErr)
	}
	return string(stdout), string(stderr), runErr
}
