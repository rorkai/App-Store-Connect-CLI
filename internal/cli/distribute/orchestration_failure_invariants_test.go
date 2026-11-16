package distribute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

func TestDistributionFirstRunPreSnapshotFailuresCheckpointDurably(t *testing.T) {
	tests := []struct {
		name        string
		configure   func(*distributionOrchestrationDependencies)
		status      string
		code        string
		recoverable bool
	}{
		{
			name: "identity inspection or binding mismatch is terminal",
			configure: func(deps *distributionOrchestrationDependencies) {
				deps.inspectIdentity = func(context.Context, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
					return signing.PKCS12IdentityInfo{}, errors.New("identity changed")
				}
			},
			status: "blocked", code: "identity_changed",
		},
		{
			name: "archive snapshot I/O failure is retryable",
			configure: func(deps *distributionOrchestrationDependencies) {
				deps.snapshotArchive = func(context.Context, string, string, string) (archiveTreeSnapshot, error) {
					return archiveTreeSnapshot{}, errors.New("temporary snapshot I/O failure")
				}
			},
			status: "recoverable", code: "archive_snapshot_failed", recoverable: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			plan.Paths.StateDir = filepath.Join(t.TempDir(), "runs")
			if err := sealDistributionPlan(&plan); err != nil {
				t.Fatal(err)
			}
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			deps.createRun = createDistributionRunScaffold
			deps.readRun = readDistributionRunState
			deps.writeRun = writeDistributionRunState
			test.configure(&deps)
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{
				PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash,
			})
			if err == nil {
				t.Fatal("first-run failure unexpectedly succeeded")
			}
			if result == nil || result.Status != test.status || result.Stage != "identity_validate" || result.LastFailureCode != test.code || result.Recoverable != test.recoverable {
				t.Fatalf("result = %#v, want %s identity_validate/%s recoverable=%t", result, test.status, test.code, test.recoverable)
			}
			durable, readErr := readDistributionRunState(plan.Paths.StateDir, result.RunID)
			if readErr != nil {
				t.Fatalf("read durable failure: %v", readErr)
			}
			if durable.RunID != result.RunID || durable.Artifacts.ArchiveSnapshot != nil {
				t.Fatalf("durable pre-snapshot state = %#v", durable)
			}
		})
	}
}

func TestDistributionLateResumeUsesProductionStateWriterWithoutStageRewind(t *testing.T) {
	for _, initialStage := range []string{"publish", "fetch_verify"} {
		t.Run(initialStage, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			plan.Paths.StateDir = filepath.Join(t.TempDir(), "runs")
			if err := sealDistributionPlan(&plan); err != nil {
				t.Fatal(err)
			}
			run := validCompletedDistributionRun(plan)
			run.Status, run.Stage, run.Recoverable = "recoverable", initialStage, true
			if initialStage == "publish" {
				run.LastFailureCode = "provider_outcome_unknown"
			} else {
				run.LastFailureCode = "publication_verification_failed"
			}
			if err := createDistributionRunScaffold(plan.Paths.StateDir, run.RunID); err != nil {
				t.Fatal(err)
			}
			if err := writeDistributionRunState(plan.Paths.StateDir, run); err != nil {
				t.Fatalf("write initial late-stage state: %v", err)
			}

			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			deps.readRun = readDistributionRunState
			deps.writeRun = writeDistributionRunState
			deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
				return publishExecutionResult{}, fmtWrapped(core.ErrPrivatePublishConflict)
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
			if err == nil || result == nil || result.Status != "blocked" || result.Stage != "fetch_verify" || result.LastFailureCode != "publication_intent_conflict" {
				t.Fatalf("late resume result = %#v, %v", result, err)
			}
			durable, readErr := readDistributionRunState(plan.Paths.StateDir, run.RunID)
			if readErr != nil || durable.Status != "blocked" || durable.Stage != "fetch_verify" || durable.LastFailureCode != "publication_intent_conflict" {
				t.Fatalf("durable late resume = %#v, %v", durable, readErr)
			}
		})
	}
}

func TestDistributionFinalStateWriteFailureFallsBackToDurableFetchVerify(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	plan.Paths.StateDir = filepath.Join(t.TempDir(), "runs")
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	run := validCompletedDistributionRun(plan)
	run.Status, run.Stage, run.Recoverable, run.LastFailureCode = "recoverable", "fetch_verify", true, "publication_verification_failed"
	if err := createDistributionRunScaffold(plan.Paths.StateDir, run.RunID); err != nil {
		t.Fatal(err)
	}
	if err := writeDistributionRunState(plan.Paths.StateDir, run); err != nil {
		t.Fatal(err)
	}

	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.readRun = readDistributionRunState
	completeWrites := 0
	deps.writeRun = func(stateDir string, state persistedDistributionRunState) error {
		if state.Status == "complete" {
			completeWrites++
			return errors.New("temporary final state write failure")
		}
		return writeDistributionRunState(stateDir, state)
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
	if err == nil || result == nil || result.Status != "recoverable" || result.Stage != "fetch_verify" || result.LastFailureCode != "completion_state_failed" {
		t.Fatalf("final write failure = %#v, %v", result, err)
	}
	if completeWrites != 1 {
		t.Fatalf("complete writes = %d, want 1", completeWrites)
	}
	durable, readErr := readDistributionRunState(plan.Paths.StateDir, run.RunID)
	if readErr != nil || durable.Status != "recoverable" || durable.Stage != "fetch_verify" || durable.LastFailureCode != "completion_state_failed" {
		t.Fatalf("durable fallback state = %#v, %v", durable, readErr)
	}
}

func TestDistributionAccountReconcileStillRequiresArchiveSnapshot(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	state := newDistributionRunState(plan, "/plans/plan.json", deterministicDistributionRunID(plan), distributionTestNow)
	state.Attempt = 1
	state.Status, state.Stage, state.LastFailureCode = "blocked", "account_reconcile", "account_reconcile_failed"
	if err := validatePersistedDistributionRunState(state); err == nil || !strings.Contains(err.Error(), "archive snapshot") {
		t.Fatalf("account_reconcile without archive snapshot error = %v", err)
	}
}

func TestDistributionResumeClassifiesPublicationReverificationFailures(t *testing.T) {
	tests := []struct {
		name        string
		failure     error
		status      string
		code        string
		recoverable bool
		noLoop      bool
	}{
		{name: "saved intent conflict", failure: fmtWrapped(errPrivatePublishIntentConflict), status: "blocked", code: "publication_intent_conflict", noLoop: true},
		{name: "immutable provider evidence conflict", failure: fmtWrapped(core.ErrPrivatePublishConflict), status: "blocked", code: "publication_intent_conflict", noLoop: true},
		{name: "post-200 fetched bytes conflict", failure: fmt.Errorf("%w: %w", core.ErrPrivatePublishConflict, core.ErrVerificationContentConflict), status: "blocked", code: "publication_intent_conflict", noLoop: true},
		{name: "install link expired", failure: fmtWrapped(core.ErrPrivatePublishLinkExpired), status: "blocked", code: "publication_expired", noLoop: true},
		{name: "profile expired", failure: fmtWrapped(core.ErrPrivatePublishProfileExpired), status: "blocked", code: "publication_expired", noLoop: true},
		{name: "transient verifier failure", failure: errors.New("temporary verifier transport failure"), status: "recoverable", code: "publication_verification_failed", recoverable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			run := validCompletedDistributionRun(plan)
			run.Status, run.Stage, run.Recoverable, run.LastFailureCode = "recoverable", "fetch_verify", true, "publication_verification_failed"
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.writeRun = func(_ string, state persistedDistributionRunState) error {
				if err := validatePersistedDistributionRunState(state); err != nil {
					return err
				}
				run = state
				return nil
			}
			verifyCalls := 0
			deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
				verifyCalls++
				return publishExecutionResult{}, test.failure
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
			if err == nil {
				t.Fatal("publication reverification failure unexpectedly succeeded")
			}
			if result == nil || result.Status != test.status || result.Stage != "fetch_verify" || result.LastFailureCode != test.code || result.Recoverable != test.recoverable {
				t.Fatalf("result = %#v error=%v, want %s fetch_verify/%s recoverable=%t", result, err, test.status, test.code, test.recoverable)
			}
			if verifyCalls != 1 {
				t.Fatalf("reverify calls = %d, want 1", verifyCalls)
			}
			if test.noLoop {
				second, secondErr := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
				if secondErr == nil || second == nil || second.Status != "blocked" {
					t.Fatalf("second resume = %#v, %v; want blocked new-plan result", second, secondErr)
				}
				if verifyCalls != 1 {
					t.Fatalf("terminal failure reverified %d times, want no retry", verifyCalls)
				}
			}
		})
	}
}

func TestDistributionProfileCopyFailureClassification(t *testing.T) {
	tests := []struct {
		name        string
		copyResult  distributionSizedFileArtifact
		copyErr     error
		status      string
		recoverable bool
	}{
		{name: "transient read or write failure", copyErr: errors.New("temporary profile copy failure"), status: "recoverable", recoverable: true},
		{name: "existing artifact conflict", copyErr: fmtWrapped(errDistributionArtifactConflict), status: "blocked"},
		{name: "successful copy digest mismatch", copyResult: distributionSizedFileArtifact{Path: distributionProfileRelative, SHA256: strings.Repeat("9", 64), SizeBytes: 512}, status: "blocked"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			var durable persistedDistributionRunState
			deps.readRun = func(string, string) (persistedDistributionRunState, error) {
				if durable.SchemaVersion == 0 {
					return persistedDistributionRunState{}, os.ErrNotExist
				}
				return durable, nil
			}
			deps.writeRun = func(_ string, state persistedDistributionRunState) error {
				if err := validatePersistedDistributionRunState(state); err != nil {
					return err
				}
				durable = state
				return nil
			}
			deps.copyArtifact = func(_, _, _, relative string, _ int64) (distributionSizedFileArtifact, error) {
				if relative == distributionReconcileRelative {
					return distributionSizedFileArtifact{Path: relative, SHA256: strings.Repeat("5", 64), SizeBytes: 512}, nil
				}
				if relative != distributionProfileRelative {
					t.Fatalf("unexpected artifact copy %q", relative)
				}
				return test.copyResult, test.copyErr
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
			if err == nil {
				t.Fatal("profile copy failure unexpectedly succeeded")
			}
			if result == nil || result.Status != test.status || result.Stage != "account_reconcile" || result.LastFailureCode != "profile_copy_failed" || result.Recoverable != test.recoverable {
				t.Fatalf("result = %#v, want %s account_reconcile/profile_copy_failed recoverable=%t", result, test.status, test.recoverable)
			}
			if result.Artifacts.ReconcileReceipt != nil || result.Artifacts.Profile != nil {
				t.Fatalf("partial reconcile pair was adopted after profile failure: %#v", result.Artifacts)
			}
		})
	}
}

func TestDistributionResumeReappliesAfterTransientProfileCopyWithoutHalfAdoptedPair(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	var durable persistedDistributionRunState
	deps.readRun = func(string, string) (persistedDistributionRunState, error) {
		if durable.SchemaVersion == 0 {
			return persistedDistributionRunState{}, os.ErrNotExist
		}
		return durable, nil
	}
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		if err := validatePersistedDistributionRunState(state); err != nil {
			return err
		}
		durable = state
		return nil
	}
	reconcileApplyCalls := 0
	deps.reconcileApply = func(context.Context, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		reconcileApplyCalls++
		return validDistributionReconcileReceipt(), nil
	}
	verifyCalls := 0
	deps.verifyReconcile = func(context.Context, signing.ReconcileApplyOptions, string, signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
		verifyCalls++
		return signing.ReconcileReceiptView{}, errors.New("half-adopted reconcile state must not use verification")
	}
	profileCopyCalls := 0
	deps.copyArtifact = func(_, _, _, relative string, _ int64) (distributionSizedFileArtifact, error) {
		if relative == distributionReconcileRelative {
			return distributionSizedFileArtifact{Path: relative, SHA256: strings.Repeat("5", 64), SizeBytes: 512}, nil
		}
		if relative == distributionProfileRelative {
			profileCopyCalls++
			if profileCopyCalls == 1 {
				return distributionSizedFileArtifact{}, errors.New("temporary profile copy failure")
			}
			return distributionSizedFileArtifact{Path: relative, SHA256: testDistributionProfileSHA256, SizeBytes: 512}, nil
		}
		digest := strings.Repeat("6", 64)
		return distributionSizedFileArtifact{Path: relative, SHA256: digest, SizeBytes: 512}, nil
	}
	var completion persistedDistributionReceipt
	deps.writeReceipt = func(_ string, receipt persistedDistributionReceipt) error { completion = receipt; return nil }
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
		if completion.SchemaVersion == 0 {
			return persistedDistributionReceipt{}, os.ErrNotExist
		}
		return completion, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	failed, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || failed == nil || failed.Status != "recoverable" || failed.Artifacts.ReconcileReceipt != nil || failed.Artifacts.Profile != nil {
		t.Fatalf("first profile failure = %#v, %v", failed, err)
	}
	resumed, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: failed.RunID, StateDir: plan.Paths.StateDir})
	if err != nil || resumed == nil || resumed.Status != "complete" {
		t.Fatalf("resume after profile copy failure = %#v, %v", resumed, err)
	}
	if reconcileApplyCalls != 2 || verifyCalls != 0 || profileCopyCalls != 2 {
		t.Fatalf("recovery calls: apply=%d verify=%d profileCopy=%d", reconcileApplyCalls, verifyCalls, profileCopyCalls)
	}
}

func fmtWrapped(err error) error {
	return errors.Join(errors.New("operation failed"), err)
}
