package distribute

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDistributionRunPathLeaseRejectsRunDirectoryReplacement(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "runs")
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunScaffold(stateDir, runID); err != nil {
		t.Fatal(err)
	}
	lease, err := openDistributionRunPathLease(stateDir, runID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.close(); err != nil {
			t.Errorf("close lease: %v", err)
		}
	})
	original := filepath.Join(stateDir, runID)
	moved := filepath.Join(stateDir, runID+"-moved")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := lease.verify(); err == nil {
		t.Fatal("run-directory replacement was accepted")
	}
}

func TestDistributionRunPathLeaseRejectsStateDirectoryReplacement(t *testing.T) {
	parent := t.TempDir()
	stateDir := filepath.Join(parent, "runs")
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunScaffold(stateDir, runID); err != nil {
		t.Fatal(err)
	}
	lease, err := openDistributionRunPathLease(stateDir, runID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.close(); err != nil {
			t.Errorf("close lease: %v", err)
		}
	})
	moved := filepath.Join(parent, "runs-moved")
	if err := os.Rename(stateDir, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := lease.verify(); err == nil {
		t.Fatal("state-directory replacement was accepted")
	}
}

func TestDistributionRunPathLeaseRejectsEveryScaffoldSubtreeReplacement(t *testing.T) {
	for _, relative := range distributionRunScaffoldDirectories() {
		t.Run(relative, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "runs")
			runID := "drun_0123456789abcdef0123456789abcdef"
			if err := createDistributionRunScaffold(stateDir, runID); err != nil {
				t.Fatal(err)
			}
			verify, closeLease, err := acquireDistributionRunLease(context.Background(), stateDir, runID)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := closeLease(); err != nil {
					t.Errorf("close lease: %v", err)
				}
			})
			original := filepath.Join(stateDir, runID, relative)
			if err := os.Rename(original, original+"-moved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(original, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err == nil || !strings.Contains(err.Error(), "subtree path changed") {
				t.Fatalf("replacement verification error = %v", err)
			}
		})
	}
}

func TestAcquireDistributionRunLeaseRejectsReplacementDuringLockAcquisition(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "runs")
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunScaffold(stateDir, runID); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(stateDir, runID)
	moved := filepath.Join(stateDir, runID+"-moved")
	distributionRunLockAfterOpenForTest = func() {
		if err := os.Rename(original, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(original, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { distributionRunLockAfterOpenForTest = nil })
	_, _, err := acquireDistributionRunLease(context.Background(), stateDir, runID)
	if err == nil {
		t.Fatal("combined lock and lease accepted a replacement run path")
	}
}

func TestDistributionRunLeaseRejectsHeldLockPathReplacement(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "runs")
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunScaffold(stateDir, runID); err != nil {
		t.Fatal(err)
	}
	verify, closeLease, err := acquireDistributionRunLease(context.Background(), stateDir, runID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeLease(); err != nil {
			t.Errorf("close lease: %v", err)
		}
	})
	runDir := filepath.Join(stateDir, runID)
	lockPath := filepath.Join(runDir, distributionRunLockFilename)
	if err := os.Rename(lockPath, filepath.Join(runDir, "lock-moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verify(); err == nil || !strings.Contains(err.Error(), "lock path changed") {
		t.Fatalf("held lock replacement verification error = %v", err)
	}
}

func TestDistributionResumeFailStopsOnMutationPathReplacementAndRecoversOneIntent(t *testing.T) {
	for _, target := range []string{"publish", "secrets", "lock"} {
		t.Run(target, func(t *testing.T) {
			testDistributionResumePathReplacement(t, target)
		})
	}
}

func testDistributionResumePathReplacement(t *testing.T, target string) {
	t.Helper()
	stateDir := filepath.Join(t.TempDir(), "runs")
	plan := validPersistedDistributionPlan(t)
	plan.Paths.StateDir = stateDir
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	run := validCompletedDistributionRun(plan)
	run.Status, run.Stage, run.Recoverable, run.LastFailureCode = "recoverable", "publish", true, "provider_outcome_unknown"
	run.Artifacts.Publication = nil
	if err := createDistributionRunScaffold(stateDir, run.RunID); err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(stateDir, run.RunID)
	replacedPath := filepath.Join(runDir, target)
	movedPath := replacedPath + "-moved"
	intentWrites, remoteWrites, publishCalls, stateWrites := 0, 0, 0, 0
	stateWritesAtReplacement := -1
	firstAttempt := true
	durable := run
	deps := validDistributionOrchestrationDependencies(t)
	deps.acquireLease = acquireDistributionRunLease
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return durable, nil }
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		stateWrites++
		durable = state
		return nil
	}
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.publish = func(_ context.Context, request privatePublishIntentRequest) (publishExecutionResult, error) {
		publishCalls++
		if firstAttempt {
			firstAttempt = false
			if err := os.WriteFile(request.IntentPath, []byte("immutable publish intent\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			intentWrites++
			remoteWrites++
			stateWritesAtReplacement = stateWrites
			if err := os.Rename(replacedPath, movedPath); err != nil {
				t.Fatal(err)
			}
			if target == "lock" {
				if err := os.WriteFile(replacedPath, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(replacedPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return validDistributionPublishResult(), nil
		}
		if _, err := os.Stat(request.IntentPath); err != nil {
			t.Fatalf("resume did not recover the original publish intent: %v", err)
		}
		result := validDistributionPublishResult()
		result.Recovered = true
		return result, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionResume(context.Background(), distributionRunRequest{RunID: run.RunID, StateDir: stateDir})
	if err == nil || result != nil || !strings.Contains(err.Error(), "run_path_changed") {
		t.Fatalf("replacement result=%#v error=%v, want terminal path-change fail-stop", result, err)
	}
	if durable.Artifacts.Publication != nil || durable.Stage != "publish" || durable.LastFailureCode != "" {
		t.Fatalf("replacement received a publication checkpoint: %#v", durable)
	}
	if stateWrites != stateWritesAtReplacement {
		t.Fatalf("replacement received %d checkpoints after path drift", stateWrites-stateWritesAtReplacement)
	}
	if target != "lock" {
		entries, readDirErr := os.ReadDir(replacedPath)
		if readDirErr != nil {
			t.Fatal(readDirErr)
		}
		if len(entries) != 0 {
			t.Fatalf("replacement %s subtree received files: %v", target, entries)
		}
	}
	if intentWrites != 1 || remoteWrites != 1 {
		t.Fatalf("first attempt writes: intent=%d remote=%d", intentWrites, remoteWrites)
	}

	if err := os.Remove(replacedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(movedPath, replacedPath); err != nil {
		t.Fatal(err)
	}
	resumed, err := executeDistributionResume(context.Background(), distributionRunRequest{RunID: run.RunID, StateDir: stateDir})
	if err != nil || resumed == nil || resumed.Status != "complete" {
		t.Fatalf("resume result=%#v error=%v", resumed, err)
	}
	if publishCalls != 2 || intentWrites != 1 || remoteWrites != 1 {
		t.Fatalf("resume duplicated publish work: calls=%d intent=%d remote=%d", publishCalls, intentWrites, remoteWrites)
	}
	intent, err := os.ReadFile(filepath.Join(stateDir, run.RunID, filepath.FromSlash(distributionPublishIntentRel)))
	if err != nil || string(intent) != "immutable publish intent\n" {
		t.Fatalf("recovered intent changed: %q, %v", intent, err)
	}
}

type noopDistributionVerifyLease struct{}

func (noopDistributionVerifyLease) PinSubtrees(...string) error { return nil }
func (noopDistributionVerifyLease) Verify() error               { return nil }
func (noopDistributionVerifyLease) Close() error                { return nil }
