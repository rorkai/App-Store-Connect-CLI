package distribute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

func TestEffectiveDistributionMinimumValidityDaysRoundsPastPublicationWindow(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		urlTTL     time.Duration
		grace      time.Duration
		want       int
	}{
		{name: "sub-day window", urlTTL: time.Hour, want: 1},
		{name: "exact day including safety margin", urlTTL: 23*time.Hour + 59*time.Minute, want: 2},
		{name: "seven-day link", urlTTL: 7 * 24 * time.Hour, want: 8},
		{name: "configured policy is stronger", configured: 30, urlTTL: time.Hour, want: 30},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := effectiveDistributionMinimumValidityDays(test.configured, test.urlTTL, test.grace)
			if err != nil {
				t.Fatalf("effectiveDistributionMinimumValidityDays() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("effective validity days = %d, want %d", got, test.want)
			}
		})
	}
}

func TestExecuteDistributionPlanRejectsIdentityTooShortForPrivatePublicationBeforeAccountPlanning(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	config := validDistributionOrchestrationConfig()
	config.Signing.MinimumValidityDays = 0
	config.Publication.URLTTL = "168h"
	config.Publication.DownloadGrace = "0s"
	config.Publication.URLTTLDuration = 7 * 24 * time.Hour
	config.Publication.DownloadGraceDuration = 0

	deps := validDistributionOrchestrationDependencies(t)
	deps.readConfig = func(string) (distributionConfig, string, error) {
		return config, testDistributionConfigSHA256(), nil
	}
	deps.inspectIdentity = func(context.Context, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
		identity := validDistributionIdentity(now)
		identity.NotAfter = now.Add(48 * time.Hour)
		return identity, nil
	}
	deps.reconcilePlan = func(context.Context, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		t.Fatal("account reconciliation planning ran for an identity that cannot cover the publication lifetime")
		return signing.ReconcilePlanView{}, nil
	}
	deps.writePlan = func(string, persistedDistributionPlan) error {
		t.Fatal("blocked lifetime authorization wrote a plan")
		return nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	_, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
		PlanPath: "/plans/plan.json", StateDir: "/state/runs",
	})
	if err == nil || !strings.Contains(err.Error(), "identity_validity_failed") {
		t.Fatalf("error = %v, want identity_validity_failed", err)
	}
}

func TestExecuteDistributionPlanBindsEffectivePublicationValidityPolicy(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	config := validDistributionOrchestrationConfig()
	config.Signing.MinimumValidityDays = 0
	config.Publication.URLTTL = "168h"
	config.Publication.DownloadGrace = "0s"
	config.Publication.URLTTLDuration = 7 * 24 * time.Hour
	config.Publication.DownloadGraceDuration = 0
	identity := validDistributionIdentity(now)
	identity.NotAfter = now.Add(9 * 24 * time.Hour)

	deps := validDistributionOrchestrationDependencies(t)
	deps.readConfig = func(string) (distributionConfig, string, error) {
		return config, testDistributionConfigSHA256(), nil
	}
	deps.inspectIdentity = func(context.Context, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
		return identity, nil
	}
	var nestedPlan signing.ReconcilePlanView
	deps.reconcilePlan = func(_ context.Context, options signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		if options.MinimumValidityDays != 8 {
			t.Fatalf("nested minimum validity = %d, want 8", options.MinimumValidityDays)
		}
		plan := validDistributionReconcilePlan(now)
		plan.MinimumValidityDays = options.MinimumValidityDays
		plan.MaxMutations = options.MaxMutations
		plan.Certificate.ExpirationDate = identity.NotAfter.Format(time.RFC3339)
		nestedPlan = plan
		return nestedPlan, nil
	}
	deps.readReconcilePlan = func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) { return nestedPlan, nil }
	var written persistedDistributionPlan
	deps.writePlan = func(_ string, plan persistedDistributionPlan) error {
		written = plan
		return nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
		PlanPath: "/plans/plan.json", StateDir: "/state/runs",
	})
	if err != nil {
		t.Fatalf("executeDistributionPlan() error = %v", err)
	}
	if result == nil || !result.Ready {
		t.Fatalf("plan = %#v, want ready", result)
	}
	if written.Reconcile.MinimumValidityDays != 8 {
		t.Fatalf("bound nested minimum validity = %d, want 8", written.Reconcile.MinimumValidityDays)
	}
	if got, want := written.Identity.MinimumValidUntil, now.Add(8*24*time.Hour).Format(time.RFC3339); got != want {
		t.Fatalf("minimum valid until = %q, want %q", got, want)
	}
}

func TestPreflightDistributionPublicationEnforcesKnownCredentialLifetime(t *testing.T) {
	config := validDistributionOrchestrationConfig().Publication
	config.URLTTL = "168h"
	config.DownloadGrace = "0s"

	original := newObjectStore
	t.Cleanup(func() { newObjectStore = original })

	tests := []struct {
		name       string
		expiration func() time.Time
		wantError  bool
	}{
		{name: "short temporary credentials", expiration: func() time.Time { return time.Now().UTC().Add(48 * time.Hour) }, wantError: true},
		{name: "sufficient temporary credentials", expiration: func() time.Time { return time.Now().UTC().Add(9 * 24 * time.Hour) }},
		{name: "non-expiring credentials", expiration: func() time.Time { return time.Time{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newObjectStore = func(context.Context, core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
				return nil, test.expiration(), nil
			}
			err := preflightDistributionPublication(context.Background(), newDistributeTestDeps(), config)
			if (err != nil) != test.wantError {
				t.Fatalf("preflightDistributionPublication() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestPreflightDistributionPublicationBoundsCredentialResolution(t *testing.T) {
	original := newObjectStore
	t.Cleanup(func() { newObjectStore = original })
	newObjectStore = func(ctx context.Context, _ core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("credential preflight context has no deadline")
		}
		return nil, time.Time{}, nil
	}

	if err := preflightDistributionPublication(context.Background(), newDistributeTestDeps(), validDistributionOrchestrationConfig().Publication); err != nil {
		t.Fatal(err)
	}
}

func TestDistributionShortCredentialPreflightStopsBeforeAccountMutation(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
		return errDistributionPublicationCredentialsExpireTooSoon
	}
	deps.reconcileApply = func(context.Context, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		t.Fatal("account reconciliation mutated after credential lifetime preflight failed")
		return signing.ReconcileReceiptView{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	state, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || state == nil || state.LastFailureCode != "storage_preflight_failed" {
		t.Fatalf("state = %#v, error = %v, want storage_preflight_failed", state, err)
	}
}

func TestDistributionLateResumeUsesPersistedPublicationWithoutCredentialPreflight(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	run.Status = "recoverable"
	run.Stage = "fetch_verify"
	run.Recoverable = true
	run.LastFailureCode = "publication_verification_failed"

	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
	deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
		t.Fatal("late resume required storage credentials despite a persisted publication")
		return errDistributionPublicationCredentialsExpireTooSoon
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		t.Fatal("late resume attempted a provider write")
		return publishExecutionResult{}, nil
	}
	verifyCalls := 0
	deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
		verifyCalls++
		return validDistributionPublishResultForCompletion(plan, run), nil
	}
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		run = state
		return nil
	}
	deps.writeReceipt = func(string, persistedDistributionReceipt) error { return nil }
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
	if err != nil {
		t.Fatalf("executeDistributionResume() error = %v", err)
	}
	if result == nil || result.Status != "complete" || verifyCalls != 1 {
		t.Fatalf("result = %#v, reverify calls = %d, want completed read-only reverify", result, verifyCalls)
	}
}

func TestDistributionOperationsRejectCopiedStateUnderNondeterministicRunID(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	run.RunID = "drun_99999999999999999999999999999999"

	tests := []struct {
		name string
		run  func(context.Context, distributionOrchestrationDependencies) error
	}{
		{name: "resume", run: func(ctx context.Context, deps distributionOrchestrationDependencies) error {
			installDistributionOrchestrationDependencies(t, deps)
			_, err := executeDistributionResume(ctx, distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
			return err
		}},
		{name: "status", run: func(ctx context.Context, deps distributionOrchestrationDependencies) error {
			installDistributionOrchestrationDependencies(t, deps)
			_, err := executeDistributionStatus(ctx, distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
			return err
		}},
		{name: "verify", run: func(ctx context.Context, deps distributionOrchestrationDependencies) error {
			installDistributionOrchestrationDependencies(t, deps)
			_, err := executeDistributionVerify(ctx, distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir, Timeout: time.Second})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
			deps.acquireLock = func(context.Context, *shared.Deps, string, string) (func() error, error) {
				return func() error { return nil }, nil
			}
			deps.acquireVerifyLease = func(string, string) (distributionVerifyLease, error) {
				return noopDistributionVerifyLease{}, nil
			}
			err := test.run(context.Background(), deps)
			if err == nil || !strings.Contains(err.Error(), "does not match") {
				t.Fatalf("error = %v, want deterministic run binding rejection", err)
			}
		})
	}
}

func TestDistributionVerifyRejectsNilErrorDeviceObservationWithoutExactCompletion(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	tests := []struct {
		name        string
		observation deviceObservation
	}{
		{name: "false outcome", observation: deviceObservation{}},
		{name: "wrong bundle", observation: deviceObservation{Requested: true, DeviceFound: true, AppInstalled: true, BundleID: "com.secret.wrong", Version: receipt.AppVersion, Build: receipt.AppBuildNumber}},
		{name: "wrong version", observation: deviceObservation{Requested: true, DeviceFound: true, AppInstalled: true, BundleID: receipt.AppBundleID, Version: "PRIVATE-VERSION-CANARY", Build: receipt.AppBuildNumber}},
		{name: "wrong build", observation: deviceObservation{Requested: true, DeviceFound: true, AppInstalled: true, BundleID: receipt.AppBundleID, Version: receipt.AppVersion, Build: "PRIVATE-BUILD-CANARY"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.hashFile = fakeDistributionSizedArtifact
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
			deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
			deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
				return validDistributionPublishResultForCompletion(plan, run), nil
			}
			deps.observeDevice = func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error) {
				return test.observation, nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{
				RunID: run.RunID, StateDir: plan.Paths.StateDir, Device: "PRIVATE-DEVICE-SELECTOR", Timeout: 45 * time.Second,
			})
			if err == nil || result != nil || !strings.Contains(err.Error(), "device_observation_mismatch") {
				t.Fatalf("result = %#v, error = %v, want closed device observation mismatch", result, err)
			}
			for _, secret := range []string{"PRIVATE-DEVICE-SELECTOR", "com.secret.wrong", "PRIVATE-VERSION-CANARY", "PRIVATE-BUILD-CANARY"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("closed error leaked %q: %v", secret, err)
				}
			}
		})
	}
}
