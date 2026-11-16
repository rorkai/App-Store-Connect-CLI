package distribute

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
)

func TestDistributionApplyAndResumeRejectConfigBindingTamperBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*persistedDistributionPlan)
	}{
		{name: "maximum mutations", mutate: func(plan *persistedDistributionPlan) { plan.Reconcile.MaxMutations-- }},
		{name: "publication endpoint", mutate: func(plan *persistedDistributionPlan) { plan.Publication.Endpoint = "https://other.example.com" }},
		{name: "publication lifetime", mutate: func(plan *persistedDistributionPlan) { plan.Publication.URLTTL = "12h" }},
		{name: "published title", mutate: func(plan *persistedDistributionPlan) { plan.Archive.PublishedTitle = "Other title" }},
		{name: "configured certificate", mutate: func(plan *persistedDistributionPlan) { plan.Identity.CertificateSHA256 = strings.Repeat("9", 64) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, operation := range []string{"apply", "resume"} {
				t.Run(operation, func(t *testing.T) {
					plan := validPersistedDistributionPlan(t)
					test.mutate(&plan)
					if err := sealDistributionPlan(&plan); err != nil {
						t.Fatal(err)
					}
					deps := validApplyDistributionOrchestrationDependencies(t, plan)
					deps.createRun = func(string, string) error { t.Fatal("run created before config authorization"); return nil }
					deps.recoverEphemeral = func(context.Context) error { t.Fatal("signing recovery ran before config authorization"); return nil }
					deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
						t.Fatal("publication preflight ran before config authorization")
						return nil
					}
					deps.reconcileApply = func(context.Context, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
						t.Fatal("account reconciliation ran before config authorization")
						return signing.ReconcileReceiptView{}, nil
					}
					if operation == "resume" {
						run := newDistributionRunState(plan, plan.ConfigPath, deterministicDistributionRunID(plan), time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC))
						deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
					}
					installDistributionOrchestrationDependencies(t, deps)

					var err error
					if operation == "apply" {
						_, err = executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: plan.ConfigPath, Confirmation: plan.PlanHash})
					} else {
						_, err = executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: deterministicDistributionRunID(plan), StateDir: plan.Paths.StateDir})
					}
					if err == nil || !strings.Contains(err.Error(), "effect authorization") {
						t.Fatalf("error = %v, want config authorization rejection", err)
					}
				})
			}
		})
	}
}

func TestExecuteDistributionPlanRejectsNestedPolicyEchoMismatchBeforeWriting(t *testing.T) {
	for _, field := range []string{"minimum validity", "maximum mutations"} {
		t.Run(field, func(t *testing.T) {
			now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
			deps := validDistributionOrchestrationDependencies(t)
			deps.reconcilePlan = func(_ context.Context, options signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
				plan := validDistributionReconcilePlan(now)
				plan.MinimumValidityDays = options.MinimumValidityDays
				plan.MaxMutations = options.MaxMutations
				if field == "minimum validity" {
					plan.MinimumValidityDays--
				} else {
					plan.MaxMutations--
				}
				return plan, nil
			}
			deps.writePlan = func(string, persistedDistributionPlan) error {
				t.Fatal("policy-mismatched plan was written")
				return nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			_, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
				ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
				PlanPath: "/plans/plan.json", StateDir: "/state/runs",
			})
			if err == nil || !strings.Contains(err.Error(), "account_policy_mismatch") {
				t.Fatalf("error = %v, want account_policy_mismatch", err)
			}
		})
	}
}

func TestExecuteDistributionPlanWritesLegitimateBlockedNestedPlan(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	deps := validDistributionOrchestrationDependencies(t)
	deps.reconcilePlan = func(_ context.Context, options signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		plan := validDistributionReconcilePlan(now)
		plan.Ready = false
		plan.Blockers = []string{"provider rejected the requested device registration"}
		plan.MinimumValidityDays = options.MinimumValidityDays
		plan.MaxMutations = options.MaxMutations
		return plan, nil
	}
	deps.readReconcilePlan = func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) {
		t.Fatal("apply-only authorization read ran for a blocked plan")
		return signing.ReconcilePlanView{}, nil
	}
	written := false
	deps.writePlan = func(string, persistedDistributionPlan) error { written = true; return nil }
	installDistributionOrchestrationDependencies(t, deps)

	plan, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
		PlanPath: "/plans/plan.json", StateDir: "/state/runs",
	})
	if err != nil || plan == nil || plan.Ready || !written {
		t.Fatalf("blocked plan = %#v, written=%v, error=%v", plan, written, err)
	}
}

func TestDistributionConfigBindingIgnoresOnlyRuntimeDurationCachesAfterPersistence(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "distribution.json")
	config := validDistributionOrchestrationConfig()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	protectedConfig, configSHA, err := readDistributionConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	plan := validPersistedDistributionPlan(t)
	plan.ConfigPath = configPath
	plan.ConfigSHA256 = configSHA
	plan.Publication = protectedConfig.Publication
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "plan.json")
	if err := writePersistedDistributionPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	loaded, err := readPersistedDistributionPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Publication.URLTTLDuration != 0 || protectedConfig.Publication.URLTTLDuration == 0 {
		t.Fatalf("fixture does not exercise persisted/runtime duration cache asymmetry")
	}
	if err := validateDistributionPlanConfigBinding(loaded, protectedConfig); err != nil {
		t.Fatalf("serialized publication policy rejected after persistence: %v", err)
	}
}
