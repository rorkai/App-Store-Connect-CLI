package distribute

import (
	"context"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestDistributionApplyCheckpointsClosedReconcileFailureClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         func(*testing.T) error
		status      string
		recoverable bool
	}{
		{
			name:        "retryable execution error",
			err:         newRetryableReconcileExecutionError,
			status:      "recoverable",
			recoverable: true,
		},
		{
			name:        "semantic plan drift",
			err:         func(*testing.T) error { return signing.ErrReconcilePlanDrift },
			status:      "blocked",
			recoverable: false,
		},
		{
			name:        "invalid protected plan",
			err:         newInvalidReconcileExecutionError,
			status:      "blocked",
			recoverable: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			var checkpoints []persistedDistributionRunState
			deps.writeRun = func(_ string, state persistedDistributionRunState) error {
				checkpoints = append(checkpoints, state)
				return nil
			}
			deps.reconcileApply = func(context.Context, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
				return signing.ReconcileReceiptView{}, test.err(t)
			}
			deps.writeExportOptions = func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error) {
				t.Fatal("export started after reconcile failure")
				return nil, nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			state, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{
				PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash,
			})
			if err == nil {
				t.Fatal("apply unexpectedly succeeded")
			}
			assertReconcileFailureCheckpoint(t, state, checkpoints, test.status, "account_reconcile", "account_reconcile_failed", test.recoverable)
		})
	}
}

func TestDistributionResumeCheckpointsClosedReconcileVerificationClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         func(*testing.T) error
		status      string
		code        string
		recoverable bool
	}{
		{
			name:        "retryable verification error",
			err:         newRetryableReconcileExecutionError,
			status:      "recoverable",
			code:        "account_verification_failed",
			recoverable: true,
		},
		{
			name:        "semantic account drift",
			err:         func(*testing.T) error { return signing.ErrReconcilePlanDrift },
			status:      "blocked",
			code:        "account_state_changed",
			recoverable: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			run := validCompletedDistributionRun(plan)
			run.Status = "recoverable"
			run.Stage = "publish"
			run.Recoverable = true
			run.LastFailureCode = "provider_outcome_unknown"
			run.Artifacts.Publication = nil
			if err := validatePersistedDistributionRunState(run); err != nil {
				t.Fatalf("resume fixture is invalid: %v", err)
			}

			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			var checkpoints []persistedDistributionRunState
			deps.writeRun = func(_ string, state persistedDistributionRunState) error {
				checkpoints = append(checkpoints, state)
				return nil
			}
			deps.verifyReconcile = func(context.Context, signing.ReconcileApplyOptions, string, signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
				return signing.ReconcileReceiptView{}, test.err(t)
			}
			deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
				t.Fatal("publication resumed after reconcile verification failure")
				return publishExecutionResult{}, nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			state, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{
				RunID: run.RunID, StateDir: plan.Paths.StateDir,
			})
			if err == nil {
				t.Fatal("resume unexpectedly succeeded")
			}
			assertReconcileFailureCheckpoint(t, state, checkpoints, test.status, "publish", test.code, test.recoverable)
		})
	}
}

func newRetryableReconcileExecutionError(t *testing.T) error {
	t.Helper()
	err := context.DeadlineExceeded
	if signing.ClassifyReconcileExecutionError(err) != signing.ReconcileExecutionErrorRetryable {
		t.Fatalf("retryable reconcile error fixture = %v", err)
	}
	return err
}

func newInvalidReconcileExecutionError(t *testing.T) error {
	t.Helper()
	_, err := signing.ExecuteReconcileApply(context.Background(), signing.ReconcileApplyOptions{})
	if err == nil || signing.ClassifyReconcileExecutionError(err) != signing.ReconcileExecutionErrorPlanInvalid {
		t.Fatalf("invalid reconcile error fixture = %v", err)
	}
	return err
}

func assertReconcileFailureCheckpoint(
	t *testing.T,
	state *distributionRunState,
	checkpoints []persistedDistributionRunState,
	status string,
	stage string,
	code string,
	recoverable bool,
) {
	t.Helper()
	if state == nil || state.Status != status || state.Stage != stage || state.LastFailureCode != code || state.Recoverable != recoverable {
		t.Fatalf("reconcile failure state = %#v, want status=%q code=%q recoverable=%t", state, status, code, recoverable)
	}
	if len(checkpoints) == 0 {
		t.Fatal("reconcile failure was not checkpointed")
	}
	last := checkpoints[len(checkpoints)-1]
	if last.Status != status || last.Stage != stage || last.LastFailureCode != code || last.Recoverable != recoverable {
		t.Fatalf("last reconcile checkpoint = %#v, want status=%q code=%q recoverable=%t", last, status, code, recoverable)
	}
}
