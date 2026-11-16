package distribute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const (
	testDistributionCertificateSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testDistributionProfileSHA256     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testDistributionDeviceSetSHA256   = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	testDistributionDeviceFileSHA256  = "5555555555555555555555555555555555555555555555555555555555555555"
	testDistributionArchiveSHA256     = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	testDistributionIPASHA256         = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	testDistributionReceiptSHA256     = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	testDistributionDescriptorSHA256  = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testDistributionCertificateSHA1   = "0123456789ABCDEF0123456789ABCDEF01234567"
	testDistributionProfileUUID       = "12345678-1234-1234-1234-123456789ABC"
	testDistributionBundleID          = "com.example.demo"
	testDistributionTeamID            = "TEAM123456"
)

func TestExecuteDistributionPlanUsesExactLocalIdentityBeforeAccountPlanning(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "configuration")
	configPath := filepath.Join(configDir, "distribution.json")
	planPath := filepath.Join(t.TempDir(), "plan.json")
	stateDir := filepath.Join(t.TempDir(), "runs")
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	config := validDistributionOrchestrationConfig()
	config.DevicesFile = "inputs/devices.json"
	config.Signing.Identity.Path = "secrets/distribution.p12"
	config.Signing.Identity.PasswordFile = "secrets/password.txt"
	config.Signing.Identity.CertificateSHA256 = testDistributionCertificateSHA256
	config.Metadata.Title = "Release Candidate"

	var calls []string
	deps := validDistributionOrchestrationDependencies(t)
	deps.readConfig = func(got string) (distributionConfig, string, error) {
		calls = append(calls, "read_config")
		if got != configPath {
			t.Fatalf("config path = %q, want %q", got, configPath)
		}
		return config, testDistributionConfigSHA256(), nil
	}
	deps.hashProtectedFile = func(path string) (string, error) {
		calls = append(calls, "hash_devices")
		if path != filepath.Join(configDir, "inputs", "devices.json") {
			t.Fatalf("hashed devices path = %q", path)
		}
		return testDistributionDeviceFileSHA256, nil
	}
	deps.inspectIdentity = func(_ context.Context, _ *shared.Deps, options signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
		calls = append(calls, "inspect_identity")
		wantIdentity := filepath.Join(configDir, "secrets", "distribution.p12")
		wantPassword := filepath.Join(configDir, "secrets", "password.txt")
		if options.IdentityPath != wantIdentity || options.IdentityPasswordPath != wantPassword {
			t.Fatalf("identity options = %#v, want paths %q and %q", options, wantIdentity, wantPassword)
		}
		return validDistributionIdentity(now), nil
	}
	deps.digestArchive = func(_ context.Context, path string) (archiveTreeSnapshot, error) {
		calls = append(calls, "digest_archive")
		if path != "/archives/Demo.xcarchive" {
			t.Fatalf("archive path = %q", path)
		}
		return validDistributionArchiveSnapshot(), nil
	}
	var nestedPlan signing.ReconcilePlanView
	deps.reconcilePlan = func(_ context.Context, _ *shared.Deps, options signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		calls = append(calls, "reconcile_plan")
		if !containsString(calls, "inspect_identity") {
			t.Fatal("account reconciliation planning started before local PKCS#12 inspection")
		}
		if options.DevicesFile != filepath.Join(configDir, "inputs", "devices.json") {
			t.Fatalf("devices path = %q", options.DevicesFile)
		}
		if options.CertificateSHA256 != testDistributionCertificateSHA256 {
			t.Fatalf("certificate pin = %q", options.CertificateSHA256)
		}
		if options.MinimumValidityDays != 7 || options.MaxMutations != 32 {
			t.Fatalf("reconcile limits = validity %d, mutations %d", options.MinimumValidityDays, options.MaxMutations)
		}
		plan := validDistributionReconcilePlan(now)
		plan.MinimumValidityDays = options.MinimumValidityDays
		plan.MaxMutations = options.MaxMutations
		plan.ArchivePath = options.ArchivePath
		plan.DevicesFile = options.DevicesFile
		plan.StateDir = options.StateDir
		plan.PlanPath = filepath.Join(options.StateDir, "plan.json")
		plan.ReceiptPath = filepath.Join(options.StateDir, "receipt.json")
		plan.ProfilesDir = filepath.Join(options.StateDir, "profiles")
		nestedPlan = plan
		return nestedPlan, nil
	}
	deps.readReconcilePlan = func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) {
		return nestedPlan, nil
	}
	var written persistedDistributionPlan
	deps.writePlan = func(gotPath string, plan persistedDistributionPlan) error {
		calls = append(calls, "write_plan")
		if gotPath != planPath {
			t.Fatalf("plan path = %q, want %q", gotPath, planPath)
		}
		written = plan
		return nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive",
		ConfigPath:  configPath,
		PlanPath:    planPath,
		StateDir:    stateDir,
	})
	if err != nil {
		t.Fatalf("executeDistributionPlan() error: %v", err)
	}
	if result == nil || !result.Ready || result.PlanHash == "" {
		t.Fatalf("plan result = %#v", result)
	}
	if written.PlanHash != result.PlanHash {
		t.Fatalf("written hash = %q, result hash = %q", written.PlanHash, result.PlanHash)
	}
	if err := verifyDistributionPlanHash(written); err != nil {
		t.Fatalf("plan hash does not bind the complete plan: %v", err)
	}
	if written.ConfigPath != configPath || written.ConfigSHA256 != testDistributionConfigSHA256() {
		t.Fatalf("config binding = path %q digest %q", written.ConfigPath, written.ConfigSHA256)
	}
	if written.Identity.CertificateSHA256 != testDistributionCertificateSHA256 || written.Identity.TeamID != testDistributionTeamID {
		t.Fatalf("identity binding = %#v", written.Identity)
	}
	if written.Identity.MinimumValidUntil != now.Add(7*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("minimum valid until = %q", written.Identity.MinimumValidUntil)
	}
	if written.Identity.CertificateResourceID != "cert-resource-id" {
		t.Fatalf("certificate resource ID = %q", written.Identity.CertificateResourceID)
	}
	if written.Archive.BundleID != testDistributionBundleID || written.Archive.TeamID != testDistributionTeamID || written.Archive.TargetCount != 1 {
		t.Fatalf("archive binding = %#v", written.Archive)
	}
	if written.Archive.Title != "Demo" || written.Archive.Version != "1.2.3" || written.Archive.BuildNumber != "42" {
		t.Fatalf("archive app identity = %#v", written.Archive)
	}
	if written.Archive.PublishedTitle != "Release Candidate" {
		t.Fatalf("effective published title = %q", written.Archive.PublishedTitle)
	}
	if written.DeviceSet.FileSHA256 != testDistributionDeviceFileSHA256 {
		t.Fatalf("devices-file binding = %q", written.DeviceSet.FileSHA256)
	}
	assertDistributionPlanEffects(t, written.Effects)

	payload, err := json.Marshal(written)
	if err != nil {
		t.Fatalf("json.Marshal(plan): %v", err)
	}
	for _, forbidden := range []string{
		"PKCS12-PASSWORD-CANARY", "S3-SECRET-CANARY", "00008110-RAW-DEVICE-UDID", "Rudrank's iPhone",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("plan leaks forbidden value %q: %s", forbidden, payload)
		}
	}
	if got, want := calls, []string{"read_config", "hash_devices", "inspect_identity", "digest_archive", "reconcile_plan", "read_config", "write_plan"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestDistributionRunPathDriftFailStopsBeforePublication(t *testing.T) {
	for _, stage := range []string{"export", "prepare"} {
		t.Run(stage, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			changed := false
			deps.acquireLease = func(context.Context, *shared.Deps, string, string) (func() error, func() error, error) {
				return func() error {
					if changed {
						return errors.New("run path replaced")
					}
					return nil
				}, func() error { return nil }, nil
			}
			originalExport := deps.exportIPA
			deps.exportIPA = func(ctx context.Context, options localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
				result, err := originalExport(ctx, options)
				if stage == "export" {
					changed = true
				}
				return result, err
			}
			originalPrepare := deps.prepareIPA
			deps.prepareIPA = func(ctx context.Context, request distributionPrepareRequest) (core.PrepareResult, error) {
				result, err := originalPrepare(ctx, request)
				if stage == "prepare" {
					changed = true
				}
				return result, err
			}
			publishCalls := 0
			deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
				publishCalls++
				return validDistributionPublishResult(), nil
			}
			installDistributionOrchestrationDependencies(t, deps)
			result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: plan.ConfigPath, Confirmation: plan.PlanHash})
			if err == nil || result != nil {
				t.Fatalf("path drift result=%#v err=%v", result, err)
			}
			if publishCalls != 0 {
				t.Fatalf("publish calls=%d after path drift", publishCalls)
			}
		})
	}
}

func TestDistributionEffectInventoryExactlyBindsMutationAuthorization(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	for index := range plan.Effects {
		if plan.Effects[index].Kind == "register_device" {
			plan.Effects[index].Count = 2
		}
	}
	plan.Reconcile.MutationCount = 4 // two devices, bundle ID, profile
	if err := validateDistributionEffectInventory(plan); err != nil {
		t.Fatalf("exact inventory rejected: %v", err)
	}

	mutations := []struct {
		name   string
		mutate func(*persistedDistributionPlan)
	}{
		{name: "count mismatch", mutate: func(p *persistedDistributionPlan) { p.Reconcile.MutationCount++ }},
		{name: "missing mandatory effect", mutate: func(p *persistedDistributionPlan) { p.Effects = p.Effects[1:] }},
		{name: "wrong bundle", mutate: func(p *persistedDistributionPlan) {
			for i := range p.Effects {
				if p.Effects[i].Kind == "write_profile" {
					p.Effects[i].BundleID = "com.example.other"
				}
			}
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			candidate := plan
			candidate.Effects = append([]distributionEffect(nil), plan.Effects...)
			test.mutate(&candidate)
			if err := validateDistributionEffectInventory(candidate); err == nil {
				t.Fatal("invalid effect authorization accepted")
			}
		})
	}
}

func TestDistributionRunRelativePathRejectsPortableTraversal(t *testing.T) {
	for _, value := range []string{"..", "../outside", "nested/../../outside"} {
		if err := validateDistributionRunRelativePath("artifact.path", value); err == nil {
			t.Fatalf("portable traversal %q accepted", value)
		}
	}
}

func TestExistingPreparedBundleRequiresExactDescriptorBytesBeforeReuse(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	ipa := []byte("exact exported ipa")
	ipaSHA := digestDistributionBytes(ipa)
	descriptor := validDistributionPrepareResult().Descriptor
	descriptor.Artifact.SHA256 = ipaSHA
	descriptor.Artifact.SizeBytes = int64(len(ipa))
	descriptorBytes, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.json"), descriptorBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "payload", "app.ipa"), ipa, 0o644); err != nil {
		t.Fatal(err)
	}
	plan := validPersistedDistributionPlan(t)
	state := validCompletedDistributionRun(plan)
	state.Artifacts.IPA.SHA256 = ipaSHA
	state.Artifacts.IPA.SizeBytes = int64(len(ipa))
	state.Artifacts.Bundle.DescriptorSHA256 = digestDistributionBytes(descriptorBytes)
	if err := validateExistingDistributionPreparedBundle(newDistributeTestDeps(), bundleDir, plan, state); err != nil {
		t.Fatalf("exact prepared bundle rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.json"), append(descriptorBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateExistingDistributionPreparedBundle(newDistributeTestDeps(), bundleDir, plan, state); err == nil || !strings.Contains(err.Error(), "descriptor bytes") {
		t.Fatalf("semantically equivalent descriptor rewrite accepted: %v", err)
	}
}

func TestDistributionAuthorizationBindsNestedReconcileActionKinds(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*signing.ReconcilePlanView)
	}{
		{name: "action kind", mutate: func(n *signing.ReconcilePlanView) { n.Actions[2].Kind = "createBundleID" }},
		{name: "certificate resource", mutate: func(n *signing.ReconcilePlanView) { n.Certificate.ResourceID = "different" }},
		{name: "certificate digest", mutate: func(n *signing.ReconcilePlanView) { n.Certificate.SHA256 = strings.Repeat("9", 64) }},
		{name: "certificate expiration", mutate: func(n *signing.ReconcilePlanView) { n.Certificate.ExpirationDate = "2028-08-13T08:00:00Z" }},
		{name: "receipt path", mutate: func(n *signing.ReconcilePlanView) { n.ReceiptPath = "/different/receipt.json" }},
		{name: "archive path", mutate: func(n *signing.ReconcilePlanView) { n.ArchivePath = "/different/App.xcarchive" }},
		{name: "devices path", mutate: func(n *signing.ReconcilePlanView) { n.DevicesFile = "/different/devices.json" }},
		{name: "state path", mutate: func(n *signing.ReconcilePlanView) { n.StateDir = "/different" }},
		{name: "profiles path", mutate: func(n *signing.ReconcilePlanView) { n.ProfilesDir = "/different/profiles" }},
		{name: "validity policy", mutate: func(n *signing.ReconcilePlanView) { n.MinimumValidityDays-- }},
		{name: "target kind", mutate: func(n *signing.ReconcilePlanView) { n.Targets[0].Kind = "extension" }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			plan := validPersistedDistributionPlan(t)
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			nested := validDistributionReconcilePlan(time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC))
			nested.PlanHash, nested.MutationCount, nested.MaxMutations = plan.Reconcile.PlanHash, plan.Reconcile.MutationCount, plan.Reconcile.MaxMutations
			nested.DeviceSetSHA256, nested.DeviceCount = plan.DeviceSet.SHA256, plan.DeviceSet.Count
			test.mutate(&nested)
			deps.readReconcilePlan = func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) { return nested, nil }
			deps.createRun = func(string, string) error { t.Fatal("run created for mismatched nested plan"); return nil }
			installDistributionOrchestrationDependencies(t, deps)
			_, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: plan.ConfigPath, Confirmation: plan.PlanHash})
			if err == nil || !strings.Contains(err.Error(), "effect authorization") {
				t.Fatalf("error=%v, want effect authorization mismatch", err)
			}
		})
	}
}

func TestExecuteDistributionPlanRejectsIdentityDriftBeforeAccountPlanning(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		mutateConfig func(*distributionConfig)
		mutateLocal  func(*signing.PKCS12IdentityInfo)
		mutateRemote func(*signing.ReconcilePlanView)
		want         string
		plannerRuns  bool
	}{
		{
			name: "configured certificate differs from PKCS12",
			mutateConfig: func(config *distributionConfig) {
				config.Signing.Identity.CertificateSHA256 = strings.Repeat("9", 64)
			},
			want: "certificate",
		},
		{
			name: "PKCS12 expires inside minimum validity",
			mutateLocal: func(info *signing.PKCS12IdentityInfo) {
				info.NotAfter = now.Add(6*24*time.Hour + time.Hour)
			},
			want: "validity",
		},
		{
			name: "account selected certificate differs",
			mutateRemote: func(plan *signing.ReconcilePlanView) {
				plan.Certificate.SHA256 = strings.Repeat("8", 64)
			},
			want:        "certificate",
			plannerRuns: true,
		},
		{
			name: "account team differs",
			mutateRemote: func(plan *signing.ReconcilePlanView) {
				plan.TeamID = "OTHERTEAM"
				plan.Certificate.TeamID = "OTHERTEAM"
			},
			want:        "team",
			plannerRuns: true,
		},
		{
			name: "account expiration differs",
			mutateRemote: func(plan *signing.ReconcilePlanView) {
				plan.Certificate.ExpirationDate = now.Add(300 * 24 * time.Hour).Format(time.RFC3339)
			},
			want:        "expiration",
			plannerRuns: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validDistributionOrchestrationConfig()
			if test.mutateConfig != nil {
				test.mutateConfig(&config)
			}
			local := validDistributionIdentity(now)
			if test.mutateLocal != nil {
				test.mutateLocal(&local)
			}
			remote := validDistributionReconcilePlan(now)
			if test.mutateRemote != nil {
				test.mutateRemote(&remote)
			}
			plannerCalls := 0
			deps := validDistributionOrchestrationDependencies(t)
			deps.readConfig = func(string) (distributionConfig, string, error) { return config, testDistributionConfigSHA256(), nil }
			deps.inspectIdentity = func(context.Context, *shared.Deps, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
				return local, nil
			}
			deps.reconcilePlan = func(context.Context, *shared.Deps, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
				plannerCalls++
				return remote, nil
			}
			deps.writePlan = func(string, persistedDistributionPlan) error {
				t.Fatal("invalid identity evidence must not publish a plan")
				return nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			_, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
				ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
				PlanPath: "/plans/plan.json", StateDir: "/state/runs",
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			wantPlannerCalls := 0
			if test.plannerRuns {
				wantPlannerCalls = 1
			}
			if plannerCalls != wantPlannerCalls {
				t.Fatalf("planner calls = %d, want %d", plannerCalls, wantPlannerCalls)
			}
		})
	}
}

func TestExecuteDistributionPlanReturnsTypedBlockerForMultipleTargets(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	deps := validDistributionOrchestrationDependencies(t)
	deps.reconcilePlan = func(context.Context, *shared.Deps, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		plan := validDistributionReconcilePlan(now)
		plan.Targets = append(plan.Targets, signing.ReconcileTargetView{Kind: "extension", BundleID: "com.example.demo.widget"})
		return plan, nil
	}
	var written persistedDistributionPlan
	deps.writePlan = func(_ string, plan persistedDistributionPlan) error { written = plan; return nil }
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
		PlanPath: "/plans/plan.json", StateDir: "/state/runs",
	})
	if err != nil {
		t.Fatalf("executeDistributionPlan() error: %v", err)
	}
	if result.Ready || written.Ready {
		t.Fatalf("multi-target plan reported ready: %#v", written)
	}
	if len(written.Blockers) != 1 || written.Blockers[0].Code != "embedded_targets_unsupported" || written.Blockers[0].Stage != "preflight" {
		t.Fatalf("blockers = %#v", written.Blockers)
	}
	if written.Archive.TargetCount != 2 {
		t.Fatalf("target count = %d, want 2", written.Archive.TargetCount)
	}
	if err := verifyDistributionPlanHash(written); err != nil {
		t.Fatalf("blocked plan hash is invalid: %v", err)
	}
}

func TestExecuteDistributionApplyRejectsConfirmationMismatchBeforeSideEffects(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	readPlanCalls := 0
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.readPlan = func(string) (persistedDistributionPlan, error) {
		readPlanCalls++
		return plan, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	_, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{
		PlanPath: "/plans/plan.json", Confirmation: strings.Repeat("9", 64),
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "confirm") {
		t.Fatalf("error = %v, want exact confirmation mismatch", err)
	}
	if got := shared.ClassifyUsageError(err); got != shared.UsageErrorInvalidValue {
		t.Fatalf("confirmation mismatch usage classification = %q, want %q", got, shared.UsageErrorInvalidValue)
	}
	if readPlanCalls != 1 {
		t.Fatalf("read plan calls = %d, want 1", readPlanCalls)
	}
}

func TestExecuteDistributionApplyOrdersPreflightAndBindsEverySigningStage(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	plan := validPersistedDistributionPlan(t)
	config := validDistributionOrchestrationConfig()
	var calls []string
	var states []persistedDistributionRunState
	deps := validDistributionOrchestrationDependencies(t)
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readConfig = func(string) (distributionConfig, string, error) { return config, plan.ConfigSHA256, nil }
	deps.createRun = func(_, _ string) error { calls = append(calls, "create_run"); return nil }
	deps.readRun = func(string, string) (persistedDistributionRunState, error) {
		return persistedDistributionRunState{}, os.ErrNotExist
	}
	deps.acquireLock = func(_ context.Context, _ *shared.Deps, _, _ string) (func() error, error) {
		calls = append(calls, "lock")
		return func() error { calls = append(calls, "unlock"); return nil }, nil
	}
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		states = append(states, state)
		return nil
	}
	deps.inspectIdentity = func(context.Context, *shared.Deps, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
		calls = append(calls, "inspect_identity")
		return validDistributionIdentity(now), nil
	}
	deps.digestArchive = func(context.Context, string) (archiveTreeSnapshot, error) {
		calls = append(calls, "digest_archive")
		return validDistributionArchiveSnapshot(), nil
	}
	deps.snapshotArchive = func(_ context.Context, archivePath, stateDir, runID string) (archiveTreeSnapshot, error) {
		calls = append(calls, "snapshot_archive")
		if archivePath != plan.Archive.Path || stateDir != plan.Paths.StateDir || runID == "" {
			t.Fatalf("snapshot args = %q %q %q", archivePath, stateDir, runID)
		}
		return archiveTreeSnapshot{RelativePath: "inputs/Demo.xcarchive", TreeSHA256: plan.Archive.TreeSHA256, SizeBytes: plan.Archive.SizeBytes, EntryCount: plan.Archive.FileCount, App: archiveAppIdentity{BundleID: plan.Archive.BundleID, Title: plan.Archive.Title, Version: plan.Archive.Version, BuildNumber: plan.Archive.BuildNumber, MinimumOSVersion: plan.Archive.MinimumOSVersion}}, nil
	}
	deps.hashProtectedFile = func(path string) (string, error) {
		calls = append(calls, "hash_devices")
		if path != config.DevicesFile {
			t.Fatalf("devices path = %q, want %q", path, config.DevicesFile)
		}
		return plan.DeviceSet.FileSHA256, nil
	}
	deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
		calls = append(calls, "preflight_publish")
		return nil
	}
	deps.reconcileApply = func(_ context.Context, _ *shared.Deps, options signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		calls = append(calls, "reconcile_apply")
		if !containsString(calls, "snapshot_archive") || !containsString(calls, "preflight_publish") {
			t.Fatal("account mutation started before archive snapshot and S3 credential preflight")
		}
		if options.PlanPath != plan.Reconcile.PlanPath || options.ExpectedPlanHash != plan.Reconcile.PlanHash || !options.Confirm {
			t.Fatalf("reconcile apply options = %#v", options)
		}
		return validDistributionReconcileReceipt(), nil
	}
	deps.writeExportOptions = func(_ context.Context, options localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error) {
		calls = append(calls, "write_export_options")
		if options.TeamID != testDistributionTeamID || options.SigningCertificate != testDistributionCertificateSHA1 {
			t.Fatalf("export signing selection = team %q certificate %q", options.TeamID, options.SigningCertificate)
		}
		if got := options.ProvisioningProfiles[testDistributionBundleID]; got != testDistributionProfileUUID {
			t.Fatalf("export profile mapping = %q", got)
		}
		return &localxcode.ManualReleaseTestingExportOptionsResult{Path: "/state/run/export/ExportOptions.plist", SHA256: testDistributionDescriptorSHA256}, nil
	}
	t.Setenv("AWS_SECRET_ACCESS_KEY", "S3-SECRET-CANARY")
	t.Setenv("ASC_PRIVATE_KEY", "ASC-SECRET-CANARY")
	deps.runSigning = func(ctx context.Context, _ *shared.Deps, options signing.EphemeralRunOptions, callback func(context.Context) error) error {
		calls = append(calls, "signing_start")
		if options.ExpectedCertificateSHA256 != testDistributionCertificateSHA256 || options.ExpectedProfileSHA256 != testDistributionProfileSHA256 {
			t.Fatalf("ephemeral signing pins = cert %q profile %q", options.ExpectedCertificateSHA256, options.ExpectedProfileSHA256)
		}
		if err := callback(ctx); err != nil {
			return err
		}
		calls = append(calls, "signing_cleanup")
		return nil
	}
	deps.exportIPA = func(_ context.Context, options localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
		calls = append(calls, "export")
		for _, entry := range options.Environment {
			if strings.HasPrefix(entry, "AWS_") || strings.HasPrefix(entry, "ASC_") {
				t.Fatalf("secret environment leaked to Xcode: %q", entry)
			}
		}
		return &localxcode.ExportResult{IPAPath: "/state/run/export/Demo.ipa", BundleID: testDistributionBundleID, Version: "1.2.3", BuildNumber: "42"}, nil
	}
	deps.hashFile = func(path string) (distributionSizedFileArtifact, error) {
		artifact, err := fakeDistributionSizedArtifact(path)
		if strings.HasSuffix(path, ".ipa") {
			calls = append(calls, "hash_ipa")
		}
		return artifact, err
	}
	deps.prepareIPA = func(_ context.Context, request distributionPrepareRequest) (core.PrepareResult, error) {
		calls = append(calls, "prepare")
		if !containsString(calls, "signing_cleanup") {
			t.Fatal("preparation began before ephemeral signing cleanup completed")
		}
		if request.ExpectedSHA256 != testDistributionIPASHA256 || request.ExpectedSize != 1234 {
			t.Fatalf("prepare expected IPA = %q/%d", request.ExpectedSHA256, request.ExpectedSize)
		}
		return validDistributionPrepareResult(), nil
	}
	deps.publish = func(_ context.Context, _ *shared.Deps, request privatePublishIntentRequest) (publishExecutionResult, error) {
		calls = append(calls, "publish")
		if !containsString(calls, "signing_cleanup") || !containsString(calls, "prepare") {
			t.Fatal("publication began before cleanup and preparation")
		}
		if request.BundleDir == "" || request.ReceiptPath == "" || request.IntentPath == "" {
			t.Fatalf("private publication request is incomplete: %#v", request)
		}
		want := privatePublishBundleAuthorization{
			DescriptorSHA256: testDistributionDescriptorSHA256, DescriptorSize: 2048,
			IPASHA256: testDistributionIPASHA256, IPASize: 1234,
			ProfileUUID: testDistributionProfileUUID, ProfileSHA256: testDistributionProfileSHA256,
			TeamID: testDistributionTeamID, DeviceSetSHA256: plan.DeviceSet.SHA256, DeviceCount: plan.DeviceSet.Count,
			CertificateSHA256: testDistributionCertificateSHA256,
		}
		if request.ExpectedBundle != want {
			t.Fatalf("private publication authorization = %#v, want %#v", request.ExpectedBundle, want)
		}
		return validDistributionPublishResult(), nil
	}
	deps.writeReceipt = func(_ string, receipt persistedDistributionReceipt) error {
		calls = append(calls, "write_receipt")
		if receipt.Status != "published_and_fetch_verified" || receipt.PlanHash != plan.PlanHash {
			t.Fatalf("completion receipt = %#v", receipt)
		}
		return nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(now), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err != nil {
		t.Fatalf("executeDistributionApply() error: %v", err)
	}
	if result == nil || result.Status != "complete" || result.Stage != "complete" {
		t.Fatalf("apply result = %#v", result)
	}
	if len(states) == 0 || states[len(states)-1].Status != "complete" {
		t.Fatalf("checkpointed states = %#v", states)
	}
	assertBefore(t, calls, "preflight_publish", "snapshot_archive", "hash_devices", "reconcile_apply", "signing_start", "export", "signing_cleanup", "prepare", "publish", "write_receipt", "unlock")
}

func TestExecuteDistributionApplyBoundsCredentialPreflightBeforeSideEffects(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.preflightPublish = preflightDistributionPublication
	deps.snapshotArchive = func(context.Context, string, string, string) (archiveTreeSnapshot, error) {
		t.Fatal("archive snapshot ran after credential preflight failed")
		return archiveTreeSnapshot{}, nil
	}
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		t.Fatal("ASC reconciliation ran after credential preflight failed")
		return signing.ReconcileReceiptView{}, nil
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		t.Fatal("storage publication ran after credential preflight failed")
		return publishExecutionResult{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	originalStore := newObjectStore
	t.Cleanup(func() { newObjectStore = originalStore })
	sawDeadline := false
	newObjectStore = func(ctx context.Context, _ core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		if _, ok := ctx.Deadline(); !ok {
			return nil, time.Time{}, errors.New("credential lookup context has no deadline")
		}
		sawDeadline = true
		return nil, time.Time{}, context.DeadlineExceeded
	}

	state, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{
		PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash,
	})
	if err == nil || !strings.Contains(err.Error(), "storage_preflight_failed") {
		t.Fatalf("apply error = %v, want safe credential preflight failure", err)
	}
	if !sawDeadline {
		t.Fatal("credential retrieval did not receive a deadline")
	}
	if state == nil || state.Stage != "preflight" || state.Status != "recoverable" || !state.Recoverable || state.LastFailureCode != "storage_preflight_failed" {
		t.Fatalf("credential preflight failure state = %#v", state)
	}
}

func TestPreflightDistributionPublicationPropagatesCancellation(t *testing.T) {
	originalStore := newObjectStore
	t.Cleanup(func() { newObjectStore = originalStore })
	newObjectStore = func(ctx context.Context, _ core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		<-ctx.Done()
		return nil, time.Time{}, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := preflightDistributionPublication(ctx, newDistributeTestDeps(), validDistributionOrchestrationConfig().Publication)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("preflight error = %v, want context cancellation", err)
	}
}

func TestExecuteDistributionApplyStopsBeforePublishOnPreparedEvidenceMismatch(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.prepareIPA = func(context.Context, distributionPrepareRequest) (core.PrepareResult, error) {
		result := validDistributionPrepareResult()
		result.Descriptor.Signing.EmbeddedProfileSHA256 = strings.Repeat("9", 64)
		return result, nil
	}
	publishCalls := 0
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		publishCalls++
		return publishExecutionResult{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	state, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "profile") {
		t.Fatalf("error = %v, want prepared profile binding mismatch", err)
	}
	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
	}
	if state == nil || state.Stage != "prepare" || state.Status != "blocked" || state.Recoverable {
		t.Fatalf("failure state = %#v", state)
	}
}

func TestValidatePreparedDistributionBindingRejectsPublishedTitleTamper(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	plan.Archive.PublishedTitle = "Release Candidate"
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	state := validCompletedDistributionRun(plan)
	descriptor := validDistributionPrepareResult().Descriptor
	descriptor.App.Title = plan.Archive.PublishedTitle
	if err := validatePreparedDistributionBinding(plan, state, descriptor); err != nil {
		t.Fatalf("effective published title rejected: %v", err)
	}
	descriptor.App.Title = plan.Archive.Title
	if err := validatePreparedDistributionBinding(plan, state, descriptor); err == nil {
		t.Fatal("prepared descriptor title tamper was accepted")
	}
}

func TestDistributionApplyBlocksDeterministicallyIneligibleIPA(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.prepareIPA = func(context.Context, distributionPrepareRequest) (core.PrepareResult, error) {
		return core.PrepareResult{}, core.ErrNotEligible
	}
	installDistributionOrchestrationDependencies(t, deps)
	state, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || state == nil || state.Status != "blocked" || state.Recoverable {
		t.Fatalf("ineligible IPA state=%#v error=%v", state, err)
	}
}

func TestDistributionResumeReusesCheckpointedArtifactsAfterRecoverablePublishFailure(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	states := make(map[string]persistedDistributionRunState)
	var runID string
	reconcileCalls, reconcileVerifyCalls, exportCalls, prepareCalls, publishCalls, revalidateCalls := 0, 0, 0, 0, 0, 0
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.createRun = func(_, id string) error { runID = id; return nil }
	deps.writeRun = func(_ string, state persistedDistributionRunState) error { states[state.RunID] = state; return nil }
	deps.readRun = func(_, id string) (persistedDistributionRunState, error) {
		state, ok := states[id]
		if !ok {
			return persistedDistributionRunState{}, os.ErrNotExist
		}
		return state, nil
	}
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		reconcileCalls++
		return validDistributionReconcileReceipt(), nil
	}
	deps.verifyReconcile = func(_ context.Context, _ *shared.Deps, options signing.ReconcileApplyOptions, archivePath string, evidence signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
		reconcileVerifyCalls++
		if options.PlanPath != plan.Reconcile.PlanPath || options.ExpectedPlanHash != plan.Reconcile.PlanHash || !options.Confirm {
			t.Fatalf("reconcile verification options = %#v", options)
		}
		if !strings.HasSuffix(filepath.ToSlash(archivePath), "/inputs/Demo.xcarchive") {
			t.Fatalf("reconcile verification archive = %q", archivePath)
		}
		if !strings.HasSuffix(filepath.ToSlash(evidence.ReceiptPath), "/reconcile/receipt.json") || evidence.ReceiptSHA256 != strings.Repeat("5", 64) || len(evidence.Profiles) != 1 || evidence.Profiles[0].ResourceID != "profile-resource-id" || evidence.Profiles[0].SHA256 != testDistributionProfileSHA256 {
			t.Fatalf("reconcile completion evidence = %#v", evidence)
		}
		return validDistributionReconcileReceipt(), nil
	}
	deps.exportIPA = func(context.Context, localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
		exportCalls++
		return &localxcode.ExportResult{IPAPath: "/state/run/export/Demo.ipa", BundleID: testDistributionBundleID, Version: "1.2.3", BuildNumber: "42"}, nil
	}
	deps.prepareIPA = func(context.Context, distributionPrepareRequest) (core.PrepareResult, error) {
		prepareCalls++
		result := validDistributionPrepareResult()
		result.Reused = prepareCalls > 1
		return result, nil
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		publishCalls++
		if publishCalls == 1 {
			return publishExecutionResult{}, errors.New("object store response lost after request")
		}
		result := validDistributionPublishResult()
		result.Recovered = true
		return result, nil
	}
	deps.revalidateArchive = func(_ context.Context, stateDir, id string, snapshot distributionArchiveSnapshot) error {
		revalidateCalls++
		if stateDir != plan.Paths.StateDir || id != runID || snapshot.TreeSHA256 != plan.Archive.TreeSHA256 {
			t.Fatalf("archive revalidation = state %q run %q snapshot %#v", stateDir, id, snapshot)
		}
		return nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	failed, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil {
		t.Fatal("apply unexpectedly succeeded")
	}
	if failed == nil || failed.Status != "recoverable" || failed.Stage != "publish" || !failed.Recoverable {
		t.Fatalf("apply failure state = %#v", failed)
	}
	if runID == "" {
		t.Fatal("apply did not allocate a run ID")
	}

	resumed, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: runID, StateDir: plan.Paths.StateDir})
	if err != nil {
		t.Fatalf("executeDistributionResume() error: %v", err)
	}
	if resumed.Status != "complete" {
		t.Fatalf("resume state = %#v", resumed)
	}
	if reconcileCalls != 1 || exportCalls != 1 {
		t.Fatalf("resume repeated irreversible/upstream work: reconcile=%d export=%d", reconcileCalls, exportCalls)
	}
	if reconcileVerifyCalls != 1 {
		t.Fatalf("resume reconcile read-only verification calls = %d, want 1", reconcileVerifyCalls)
	}
	if prepareCalls < 1 || prepareCalls > 2 {
		t.Fatalf("prepare calls = %d, want safe exact reuse only", prepareCalls)
	}
	if publishCalls != 2 {
		t.Fatalf("publish calls = %d, want initial plus recovery", publishCalls)
	}
	if revalidateCalls != 1 {
		t.Fatalf("archive revalidation calls = %d, want 1 before resume reuse", revalidateCalls)
	}
}

func TestDistributionResumeVerifiesFromRunLocalReconcileEvidenceAfterNestedOutputsAreGone(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	plan.Paths.StateDir = filepath.Join(t.TempDir(), "runs")
	nestedDir := filepath.Join(t.TempDir(), "nested-reconcile")
	plan.Reconcile.PlanPath = filepath.Join(nestedDir, "plan.json")
	plan.Reconcile.ReceiptPath = filepath.Join(nestedDir, "receipt.json")
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	run := validCompletedDistributionRun(plan)
	run.Status, run.Stage, run.Recoverable, run.LastFailureCode = "recoverable", "publish", true, "provider_outcome_unknown"
	run.Artifacts.Publication = nil
	runRoot := filepath.Join(plan.Paths.StateDir, run.RunID)
	nestedProfilePath := filepath.Join(nestedDir, "profiles", run.Artifacts.Profile.UUID+".mobileprovision")
	receiptData, err := json.Marshal(distributionReconcileReceiptEvidence{
		SchemaVersion: 1, PlanHash: plan.Reconcile.PlanHash, StartedAt: "2026-08-13T08:00:00Z", UpdatedAt: "2026-08-13T08:01:00Z",
		Complete: true, StateDir: nestedDir, ReceiptPath: plan.Reconcile.ReceiptPath,
		Actions: []distributionReconcileActionReceiptEvidence{{
			ID: "profile:" + plan.Archive.BundleID, Kind: "downloadProfile", Status: "completed",
			ResourceID: run.Artifacts.Profile.ResourceID, OutputPath: nestedProfilePath,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	profileData := []byte("run-local-profile")
	for relative, data := range map[string][]byte{
		run.Artifacts.ReconcileReceipt.Path: receiptData,
		run.Artifacts.Profile.Path:          profileData,
	} {
		path := filepath.Join(runRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run.Artifacts.ReconcileReceipt.SHA256 = digestDistributionBytes(receiptData)
	run.Artifacts.Profile.SHA256 = digestDistributionBytes(profileData)
	for path, data := range map[string][]byte{plan.Reconcile.ReceiptPath: receiptData, nestedProfilePath: profileData} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
	deps.writeRun = func(_ string, state persistedDistributionRunState) error { run = state; return nil }
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readReconcilePlan = func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) {
		nested := validDistributionReconcilePlan(time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC))
		nested.ArchivePath, nested.DevicesFile = plan.Archive.Path, "/config/devices.json"
		nested.StateDir, nested.PlanPath, nested.ReceiptPath, nested.ProfilesDir = nestedDir, plan.Reconcile.PlanPath, plan.Reconcile.ReceiptPath, filepath.Join(nestedDir, "profiles")
		return nested, nil
	}
	verifyCalls := 0
	deps.verifyReconcile = func(_ context.Context, _ *shared.Deps, options signing.ReconcileApplyOptions, archivePath string, evidence signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
		verifyCalls++
		if _, err := os.Stat(plan.Reconcile.ReceiptPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("nested receipt unexpectedly remains available: %v", err)
		}
		if options.PlanPath != plan.Reconcile.PlanPath || options.ExpectedPlanHash != plan.Reconcile.PlanHash || !options.Confirm {
			t.Fatalf("verification options = %#v", options)
		}
		if !strings.HasSuffix(filepath.ToSlash(archivePath), "/inputs/Demo.xcarchive") {
			t.Fatalf("archive snapshot path = %q", archivePath)
		}
		gotReceipt, err := os.ReadFile(evidence.ReceiptPath)
		if err != nil || string(gotReceipt) != string(receiptData) || evidence.ReceiptSHA256 != digestDistributionBytes(receiptData) {
			t.Fatalf("run-local receipt evidence = %#v data=%q error=%v", evidence, gotReceipt, err)
		}
		if len(evidence.Profiles) != 1 {
			t.Fatalf("profile evidence = %#v", evidence.Profiles)
		}
		gotProfile, err := os.ReadFile(evidence.Profiles[0].Path)
		if err != nil || string(gotProfile) != string(profileData) || evidence.Profiles[0].SHA256 != digestDistributionBytes(profileData) || evidence.Profiles[0].ResourceID != run.Artifacts.Profile.ResourceID {
			t.Fatalf("run-local profile evidence = %#v data=%q error=%v", evidence.Profiles[0], gotProfile, err)
		}
		result := validDistributionReconcileReceipt()
		result.PlanHash = plan.Reconcile.PlanHash
		result.ReceiptPath = plan.Reconcile.ReceiptPath
		result.MainProfile.Path = nestedProfilePath
		result.MainProfile.SHA256 = evidence.Profiles[0].SHA256
		result.Profiles = []signing.ReconcileProfileView{*result.MainProfile}
		return result, nil
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		result := validDistributionPublishResult()
		result.Receipt.Signing.EmbeddedProfileSHA256 = run.Artifacts.Profile.SHA256
		return result, nil
	}
	deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
		result := validDistributionPublishResult()
		result.Receipt.Signing.EmbeddedProfileSHA256 = run.Artifacts.Profile.SHA256
		return result, nil
	}
	var completion persistedDistributionReceipt
	deps.writeReceipt = func(_ string, receipt persistedDistributionReceipt) error { completion = receipt; return nil }
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
		if completion.RunID == "" {
			return persistedDistributionReceipt{}, os.ErrNotExist
		}
		return completion, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
	if err != nil {
		t.Fatalf("executeDistributionResume() error: %v", err)
	}
	if result.Status != "complete" || verifyCalls != 1 {
		t.Fatalf("resume result=%#v verify calls=%d", result, verifyCalls)
	}
}

func TestExecuteDistributionStatusIsStrictlyLocal(t *testing.T) {
	plan := persistedDistributionPlan{PlanID: "dplan_0123456789abcdef0123456789abcdef", PlanHash: strings.Repeat("1", 64), Paths: distributionPlanPaths{StateDir: "/state/runs"}}
	want := persistedDistributionRunState{
		SchemaVersion: 1, RunID: deterministicDistributionRunID(plan), PlanID: "dplan_0123456789abcdef0123456789abcdef",
		PlanPath: "/plans/plan.json", PlanHash: strings.Repeat("1", 64), Status: "recoverable", Stage: "publish",
		UpdatedAt: "2026-08-13T08:00:00Z", Attempt: 2, Recoverable: true, LastFailureCode: "provider_outcome_unknown",
	}
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.readRun = func(stateDir, runID string) (persistedDistributionRunState, error) {
		if stateDir != "/state/runs" || runID != want.RunID {
			t.Fatalf("status read = %q %q", stateDir, runID)
		}
		return want, nil
	}
	deps.readPlan = func(path string) (persistedDistributionPlan, error) {
		if path != want.PlanPath {
			t.Fatalf("status plan path = %q", path)
		}
		return plan, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	got, err := executeDistributionStatus(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: want.RunID, StateDir: "/state/runs"})
	if err != nil {
		t.Fatalf("executeDistributionStatus() error: %v", err)
	}
	if got == nil || got.RunID != want.RunID || got.Status != want.Status || got.Stage != want.Stage {
		t.Fatalf("status = %#v, want %#v", got, want)
	}
}

func TestExecuteDistributionVerifyRevalidatesExactPublicationAndOptionallyObservesDevice(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	for _, device := range []string{"", "my-connected-phone"} {
		t.Run(map[bool]string{false: "publication only", true: "with device observation"}[device != ""], func(t *testing.T) {
			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.hashFile = fakeDistributionSizedArtifact
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
			receiptReads := 0
			deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
				receiptReads++
				return receipt, nil
			}
			verifyCalls := 0
			deps.reverifyPublish = func(_ context.Context, _ *shared.Deps, request privatePublishVerificationRequest) (publishExecutionResult, error) {
				verifyCalls++
				runRoot := filepath.Join(plan.Paths.StateDir, run.RunID)
				if request.BundleDir != filepath.Join(runRoot, run.Artifacts.Bundle.Path) ||
					request.ReceiptPath != filepath.Join(runRoot, run.Artifacts.Publication.ReceiptPath) ||
					request.LinkPath != filepath.Join(runRoot, run.Artifacts.Publication.LinkPath) {
					t.Fatalf("verification request = %#v", request)
				}
				if request.VerifyTimeout != plan.Publication.VerifyTimeoutDuration {
					t.Fatalf("verify timeout = %s", request.VerifyTimeout)
				}
				return validDistributionPublishResultForCompletion(plan, run), nil
			}
			observeCalls := 0
			deps.observeDevice = func(_ context.Context, request distributionDeviceObservationRequest) (deviceObservation, error) {
				observeCalls++
				if request.Device != device || request.BundleID != testDistributionBundleID || request.Version != "1.2.3" || request.Build != "42" || request.Timeout <= 0 || request.Timeout > 45*time.Second {
					t.Fatalf("device observation request = %#v", request)
				}
				return deviceObservation{Requested: true, DeviceFound: true, AppInstalled: true, BundleID: request.BundleID, Version: request.Version, Build: request.Build}, nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{
				RunID: run.RunID, StateDir: plan.Paths.StateDir, Device: device, Timeout: 45 * time.Second,
			})
			if err != nil {
				t.Fatalf("executeDistributionVerify() error: %v", err)
			}
			if verifyCalls != 1 {
				t.Fatalf("publication verification calls = %d, want 1", verifyCalls)
			}
			if receiptReads != 1 {
				t.Fatalf("immutable completion receipt reads = %d, want 1", receiptReads)
			}
			if result == nil || !result.PublicationVerified || result.RunID != run.RunID {
				t.Fatalf("verification result = %#v", result)
			}
			wantObserve := 0
			if device != "" {
				wantObserve = 1
				if result.DeviceObservation == nil || !result.DeviceObservation.DeviceFound || !result.DeviceObservation.AppInstalled || result.DeviceObservation.BundleID != testDistributionBundleID {
					t.Fatalf("device observation result = %#v", result.DeviceObservation)
				}
			} else if result.DeviceObservation != nil {
				t.Fatalf("publication-only result unexpectedly contains device observation: %#v", result.DeviceObservation)
			}
			if observeCalls != wantObserve {
				t.Fatalf("device observation calls = %d, want %d", observeCalls, wantObserve)
			}
		})
	}
}

func TestExecuteDistributionVerifyRejectsPublicationDifferentFromCompletionReceipt(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.hashFile = fakeDistributionSizedArtifact
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
	deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
		result := validDistributionPublishResultForCompletion(plan, run)
		result.Receipt.Artifact.Key = "team/app/objects/sha256/different.ipa"
		return result, nil
	}
	deps.observeDevice = func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error) {
		t.Fatal("device observation must not run after publication identity drift")
		return deviceObservation{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	_, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{
		RunID: run.RunID, StateDir: plan.Paths.StateDir, Device: "my-connected-phone", Timeout: 45 * time.Second,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "publication") {
		t.Fatalf("error = %v, want exact publication mismatch", err)
	}
}

func TestPublishMatchesCompletionRejectsEveryUnboundPublicationClass(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	if got := validDistributionPublishResultForCompletion(plan, run).Receipt; !publishMatchesCompletion(got, receipt, plan, run) {
		t.Fatal("exact publication fixture does not match completion evidence")
	}

	tests := map[string]func(*core.PublishReceipt){
		"schema":            func(got *core.PublishReceipt) { got.SchemaVersion = "2" },
		"endpoint":          func(got *core.PublishReceipt) { got.Endpoint = "https://other.example.com" },
		"download endpoint": func(got *core.PublishReceipt) { got.DownloadEndpoint = "https://other.example.com" },
		"region":            func(got *core.PublishReceipt) { got.Region = "elsewhere" },
		"addressing":        func(got *core.PublishReceipt) { got.AddressingStyle = "virtual" },
		"access":            func(got *core.PublishReceipt) { got.Access = core.AccessPublic },
		"public base":       func(got *core.PublishReceipt) { got.PublicBaseURL = "https://public.example.com" },
		"bucket":            func(got *core.PublishReceipt) { got.Bucket = "other-builds" },
		"prefix":            func(got *core.PublishReceipt) { got.Prefix = "other/app" },
		"url ttl":           func(got *core.PublishReceipt) { got.URLTTL = "23h" },
		"download grace":    func(got *core.PublishReceipt) { got.DownloadGrace = "2h" },
		"artifact content":  func(got *core.PublishReceipt) { got.Artifact.ContentType = core.ContentTypeHTML },
		"artifact status":   func(got *core.PublishReceipt) { got.Artifact.Status = "unknown" },
		"manifest size":     func(got *core.PublishReceipt) { got.Manifest.SizeBytes++ },
		"page digest":       func(got *core.PublishReceipt) { got.Page.SHA256 = strings.Repeat("9", 64) },
		"install url":       func(got *core.PublishReceipt) { got.InstallURL = "https://downloads.example.com/other/REDACTED" },
		"direct install":    func(got *core.PublishReceipt) { got.DirectInstallURL = "https://downloads.example.com/REDACTED" },
		"expiry": func(got *core.PublishReceipt) {
			expired := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
			got.ExpiresAt = &expired
		},
		"app title":      func(got *core.PublishReceipt) { got.App.Title = "Replacement" },
		"minimum os":     func(got *core.PublishReceipt) { got.App.MinimumOSVersion = "99.0" },
		"profile class":  func(got *core.PublishReceipt) { got.Signing.ProfileClass = "development" },
		"profile expiry": func(got *core.PublishReceipt) { got.Signing.ProfileExpiresAt = "2028-01-01T00:00:00Z" },
		"device count":   func(got *core.PublishReceipt) { got.Signing.DeviceCount++ },
		"profile certs": func(got *core.PublishReceipt) {
			got.Signing.ProfileCertificateFingerprints = []string{strings.Repeat("9", 64)}
		},
		"signature scope": func(got *core.PublishReceipt) { got.Signing.CodeSignatureVerification.Scope = "partial" },
		"signer certs": func(got *core.PublishReceipt) {
			got.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = []string{strings.Repeat("9", 64)}
		},
		"receipt path":      func(got *core.PublishReceipt) { got.ReceiptPath += ".replacement" },
		"private link path": func(got *core.PublishReceipt) { got.LinkPath += ".replacement" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			got := validDistributionPublishResultForCompletion(plan, run).Receipt
			mutate(&got)
			if publishMatchesCompletion(got, receipt, plan, run) {
				t.Fatalf("publication with tampered %s matched completion evidence", name)
			}
		})
	}
}

func TestExecuteDistributionVerifyNeverAcceptsRunOrArtifactSubtreeReplacement(t *testing.T) {
	for _, target := range []string{"run", "prepared bundle"} {
		t.Run(target, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "runs")
			plan := validPersistedDistributionPlan(t)
			plan.Paths.StateDir = stateDir
			if err := sealDistributionPlan(&plan); err != nil {
				t.Fatal(err)
			}
			run := validCompletedDistributionRun(plan)
			run.PlanHash = plan.PlanHash
			receipt := validPersistedDistributionReceipt(run)
			if err := createDistributionRunScaffold(stateDir, run.RunID); err != nil {
				t.Fatal(err)
			}
			for _, relative := range []string{run.Artifacts.ArchiveSnapshot.RelativePath, run.Artifacts.Bundle.Path} {
				if err := os.MkdirAll(filepath.Join(stateDir, run.RunID, filepath.FromSlash(relative)), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.acquireVerifyLease = acquireDistributionVerifyPathLease
			deps.hashFile = fakeDistributionSizedArtifact
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
			deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
			deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
				runRoot := filepath.Join(stateDir, run.RunID)
				original := runRoot
				if target == "prepared bundle" {
					original = filepath.Join(runRoot, filepath.FromSlash(run.Artifacts.Bundle.Path))
				}
				moved := original + "-moved"
				if err := os.Rename(original, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(original, 0o700); err != nil {
					t.Fatal(err)
				}
				return validDistributionPublishResultForCompletion(plan, run), nil
			}
			installDistributionOrchestrationDependencies(t, deps)

			result, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{
				RunID: run.RunID, StateDir: stateDir, Timeout: 45 * time.Second,
			})
			if err == nil || (result != nil && result.PublicationVerified) {
				t.Fatalf("replacement target=%q result=%#v err=%v", target, result, err)
			}
			if !strings.Contains(err.Error(), "run_path_changed") {
				t.Fatalf("replacement error = %v, want terminal path-change failure", err)
			}
		})
	}
}

func TestExecuteDistributionVerifyMissingRunIsReadOnly(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "runs")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runID := "drun_0123456789abcdef0123456789abcdef"
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.acquireVerifyLease = acquireDistributionVerifyPathLease
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{RunID: runID, StateDir: stateDir, Timeout: time.Second})
	if err == nil || result != nil {
		t.Fatalf("missing-run verify result=%#v err=%v", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, runID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("read-only verify created or changed missing run path: %v", statErr)
	}
}

func TestDistributionVerifyUsesOneTotalTimeoutBudget(t *testing.T) {
	synctest.Test(t, testDistributionVerifyUsesOneTotalTimeoutBudget)
}

func testDistributionVerifyUsesOneTotalTimeoutBudget(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.hashFile = fakeDistributionSizedArtifact
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
	deps.reverifyPublish = func(ctx context.Context, _ *shared.Deps, request privatePublishVerificationRequest) (publishExecutionResult, error) {
		if request.VerifyTimeout <= 0 || request.VerifyTimeout > 80*time.Second {
			t.Fatalf("publication timeout = %s", request.VerifyTimeout)
		}
		select {
		case <-time.After(50 * time.Second):
		case <-ctx.Done():
			return publishExecutionResult{}, ctx.Err()
		}
		return validDistributionPublishResultForCompletion(plan, run), nil
	}
	deps.observeDevice = func(ctx context.Context, request distributionDeviceObservationRequest) (deviceObservation, error) {
		if request.Timeout <= 0 || request.Timeout >= 50*time.Second {
			t.Fatalf("device received fresh rather than remaining budget: %s", request.Timeout)
		}
		<-ctx.Done()
		return deviceObservation{}, ctx.Err()
	}
	installDistributionOrchestrationDependencies(t, deps)
	started := time.Now()
	_, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir, Device: "phone", Timeout: 80 * time.Second})
	if err == nil || time.Since(started) > 80*time.Second {
		t.Fatalf("total deadline error=%v elapsed=%s", err, time.Since(started))
	}
}

func TestDistributionVerifyTreatsInsufficientRemainingDeviceBudgetAsTimeout(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	receipt := validPersistedDistributionReceipt(run)
	deps := panicOnDistributionOrchestrationSideEffects(t)
	deps.hashFile = fakeDistributionSizedArtifact
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
	deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
		return validDistributionPublishResultForCompletion(plan, run), nil
	}
	deps.observeDevice = func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error) {
		t.Fatal("device observation must not receive less than its minimum timeout")
		return deviceObservation{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionVerify(context.Background(), distributionTestDepsAt(distributionTestNow), distributionVerifyRequest{
		RunID: run.RunID, StateDir: plan.Paths.StateDir, Device: "phone", Timeout: deviceObservationMinimumTimeout,
	})
	if err == nil || result != nil || !strings.Contains(err.Error(), "verification_timeout") {
		t.Fatalf("result=%#v error=%v, want device verification timeout", result, err)
	}
}

func TestExecuteDistributionApplySamePlanReturnsSameRunWithoutRepeatingStages(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	var mutex sync.Mutex
	states := make(map[string]persistedDistributionRunState)
	created := make(map[string]bool)
	var firstRunID string
	createCalls, reconcileCalls, exportCalls, publishCalls, receiptWrites := 0, 0, 0, 0, 0
	deps.createRun = func(_ string, runID string) error {
		mutex.Lock()
		defer mutex.Unlock()
		createCalls++
		if created[runID] {
			return os.ErrExist
		}
		created[runID] = true
		firstRunID = runID
		return nil
	}
	deps.readRun = func(_ string, runID string) (persistedDistributionRunState, error) {
		mutex.Lock()
		defer mutex.Unlock()
		state, ok := states[runID]
		if !ok {
			return persistedDistributionRunState{}, os.ErrNotExist
		}
		return state, nil
	}
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		mutex.Lock()
		defer mutex.Unlock()
		states[state.RunID] = state
		return nil
	}
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		reconcileCalls++
		return validDistributionReconcileReceipt(), nil
	}
	deps.exportIPA = func(context.Context, localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
		exportCalls++
		return &localxcode.ExportResult{IPAPath: "/state/run/export/Demo.ipa", BundleID: testDistributionBundleID, Version: "1.2.3", BuildNumber: "42"}, nil
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		publishCalls++
		return validDistributionPublishResult(), nil
	}
	var completion persistedDistributionReceipt
	deps.writeReceipt = func(_ string, receipt persistedDistributionReceipt) error {
		receiptWrites++
		completion = receipt
		return nil
	}
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
		if completion.SchemaVersion == 0 {
			return persistedDistributionReceipt{}, os.ErrNotExist
		}
		return completion, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	first, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err != nil {
		t.Fatalf("first apply error: %v", err)
	}
	second, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err != nil {
		t.Fatalf("second apply error: %v", err)
	}
	if first.RunID == "" || first.RunID != second.RunID || first.RunID != firstRunID {
		t.Fatalf("run IDs = first %q second %q allocated %q", first.RunID, second.RunID, firstRunID)
	}
	if createCalls != 2 {
		t.Fatalf("create calls = %d, want one deterministic create attempt per apply", createCalls)
	}
	if reconcileCalls != 1 || exportCalls != 1 || publishCalls != 1 || receiptWrites != 1 {
		t.Fatalf("duplicate stages: reconcile=%d export=%d publish=%d receipts=%d", reconcileCalls, exportCalls, publishCalls, receiptWrites)
	}
}

func TestConcurrentApplyInitializesOnlyAfterReadingStateUnderRunLock(t *testing.T) {
	// executeDistributionApply runs on a worker goroutine below, so these
	// dependency stubs must report and return instead of calling t.Fatal:
	// terminating the worker would leave the test blocked on done forever.
	fixture := handlertest.New(t)
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	runID := deterministicDistributionRunID(plan)
	completed := validCompletedDistributionRun(plan)
	completed.RunID = runID
	completed.PlanPath = "/plans/plan.json"
	createReturned := make(chan struct{})
	allowFirstLock := make(chan struct{})
	var createOnce sync.Once
	deps.createRun = func(string, string) error {
		createOnce.Do(func() { close(createReturned) })
		return nil
	}
	deps.acquireLock = func(context.Context, *shared.Deps, string, string) (func() error, error) {
		<-allowFirstLock
		return func() error { return nil }, nil
	}
	deps.readRun = func(string, string) (persistedDistributionRunState, error) { return completed, nil }
	deps.writeRun = func(string, persistedDistributionRunState) error {
		return fixture.Errorf("stale creator overwrote completed state")
	}
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		return signing.ReconcileReceiptView{}, fixture.Errorf("stale creator repeated reconciliation")
	}
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
		return validPersistedDistributionReceipt(completed), nil
	}
	installDistributionOrchestrationDependencies(t, deps)
	done := make(chan error, 1)
	go func() {
		result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
		if err == nil && (result == nil || result.Status != "complete") {
			err = fmt.Errorf("unexpected result %#v", result)
		}
		done <- err
	}()
	<-createReturned
	// Models another process completing the deterministic run before this
	// creator acquires the run lock.
	close(allowFirstLock)
	if err := <-done; err != nil {
		t.Fatalf("stale creator did not adopt completed state: %v", err)
	}
}

func TestConcurrentDistributionResumeSerializesSameRunAndRechecksStateInsideLock(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	run.Status = "recoverable"
	run.Stage = "publish"
	run.Recoverable = true
	run.LastFailureCode = "provider_outcome_unknown"
	run.Artifacts.Publication = nil
	var stateMutex sync.Mutex
	current := run
	var runLock sync.Mutex
	lockAttempts := make(chan struct{}, 2)
	publishEntered := make(chan struct{})
	allowPublish := make(chan struct{})
	publishCalls := 0
	deps := validDistributionOrchestrationDependencies(t)
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readRun = func(string, string) (persistedDistributionRunState, error) {
		stateMutex.Lock()
		defer stateMutex.Unlock()
		return current, nil
	}
	deps.writeRun = func(_ string, state persistedDistributionRunState) error {
		stateMutex.Lock()
		defer stateMutex.Unlock()
		current = state
		return nil
	}
	deps.acquireLock = func(context.Context, *shared.Deps, string, string) (func() error, error) {
		lockAttempts <- struct{}{}
		runLock.Lock()
		return func() error { runLock.Unlock(); return nil }, nil
	}
	deps.revalidateArchive = func(context.Context, string, string, distributionArchiveSnapshot) error { return nil }
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		stateMutex.Lock()
		publishCalls++
		call := publishCalls
		stateMutex.Unlock()
		if call == 1 {
			close(publishEntered)
			<-allowPublish
		}
		return validDistributionPublishResult(), nil
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

	result := make(chan error, 2)
	go func() {
		_, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
		result <- err
	}()
	<-lockAttempts
	<-publishEntered
	go func() {
		_, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
		result <- err
	}()
	<-lockAttempts
	close(allowPublish)
	for range 2 {
		if err := <-result; err != nil {
			t.Fatalf("concurrent resume error: %v", err)
		}
	}
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if publishCalls != 1 {
		t.Fatalf("serialized resumes published %d times, want 1", publishCalls)
	}
	if current.Status != "complete" {
		t.Fatalf("final state = %#v", current)
	}
}

func TestDistributionApplyRecoversEphemeralSigningAfterRunLockBeforeOtherWork(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	var calls []string
	deps.acquireLock = func(context.Context, *shared.Deps, string, string) (func() error, error) {
		calls = append(calls, "lock")
		return func() error { calls = append(calls, "unlock"); return nil }, nil
	}
	deps.recoverEphemeral = func(context.Context, *shared.Deps) error {
		calls = append(calls, "recover_ephemeral")
		return errors.New("validated signing journal recovery failed")
	}
	deps.snapshotArchive = func(context.Context, string, string, string) (archiveTreeSnapshot, error) {
		calls = append(calls, "snapshot")
		return archiveTreeSnapshot{}, nil
	}
	deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
		calls = append(calls, "storage_preflight")
		return nil
	}
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		calls = append(calls, "reconcile_mutation")
		return signing.ReconcileReceiptView{}, nil
	}
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "recover") {
		t.Fatalf("error = %v, want signing recovery failure", err)
	}
	if result == nil || result.Status != "recoverable" || !result.Recoverable {
		t.Fatalf("transient recovery failure = %#v, want recoverable", result)
	}
	if got, want := calls, []string{"lock", "recover_ephemeral", "unlock"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestDistributionApplyBlocksInvalidSigningRecoveryJournal(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	deps.recoverEphemeral = func(context.Context, *shared.Deps) error { return signing.ErrEphemeralRecoveryJournalInvalid }
	installDistributionOrchestrationDependencies(t, deps)
	result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || result == nil || result.Status != "blocked" || result.Recoverable {
		t.Fatalf("invalid journal result=%#v error=%v", result, err)
	}
}

func TestValidatePersistedDistributionRunStateRejectsImpossibleTransitions(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	tests := []struct {
		name   string
		mutate func(*persistedDistributionRunState)
	}{
		{name: "complete status before complete stage", mutate: func(state *persistedDistributionRunState) { state.Stage = "publish" }},
		{name: "running at complete stage", mutate: func(state *persistedDistributionRunState) { state.Status = "running" }},
		{name: "complete is recoverable", mutate: func(state *persistedDistributionRunState) { state.Recoverable = true }},
		{name: "complete carries failure", mutate: func(state *persistedDistributionRunState) { state.LastFailureCode = "provider_outcome_unknown" }},
		{name: "complete misses publication", mutate: func(state *persistedDistributionRunState) { state.Artifacts.Publication = nil }},
		{name: "publish misses prepared bundle", mutate: func(state *persistedDistributionRunState) {
			state.Status, state.Stage = "running", "publish"
			state.Artifacts.Bundle = nil
			state.Artifacts.Publication = nil
		}},
		{name: "prepare contains future publication", mutate: func(state *persistedDistributionRunState) { state.Status, state.Stage = "running", "prepare" }},
		{name: "planned state contains artifacts", mutate: func(state *persistedDistributionRunState) { state.Status, state.Stage = "planned", "preflight" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := validCompletedDistributionRun(plan)
			test.mutate(&state)
			if err := validatePersistedDistributionRunState(state); err == nil {
				t.Fatalf("invalid state accepted: %#v", state)
			}
		})
	}
}

func TestValidatePersistedDistributionRunStateRejectsArtifactsOutsideRunRoot(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	state := validCompletedDistributionRun(plan)
	if err := validatePersistedDistributionRunState(state); err != nil {
		t.Fatalf("valid contained state rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*persistedDistributionRunState)
	}{
		{name: "profile", mutate: func(state *persistedDistributionRunState) {
			state.Artifacts.Profile.Path = "/tmp/profile.mobileprovision"
		}},
		{name: "IPA traversal", mutate: func(state *persistedDistributionRunState) { state.Artifacts.IPA.Path = "../Demo.ipa" }},
		{name: "bundle", mutate: func(state *persistedDistributionRunState) { state.Artifacts.Bundle.Path = "/tmp/bundle" }},
		{name: "publication receipt", mutate: func(state *persistedDistributionRunState) {
			state.Artifacts.Publication.ReceiptPath = "/tmp/receipt.json"
		}},
		{name: "sensitive links", mutate: func(state *persistedDistributionRunState) { state.Artifacts.Publication.LinkPath = "/tmp/links.json" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := validCompletedDistributionRun(plan)
			test.mutate(&candidate)
			if err := validatePersistedDistributionRunState(candidate); err == nil {
				t.Fatalf("outside-run artifact accepted: %#v", candidate.Artifacts)
			}
		})
	}
}

func TestDistributionApplyCheckpointsEveryPublicationCrashBoundary(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	providerPhases := []string{
		"after saved publish intent", "after IPA PUT", "after manifest PUT", "after install page PUT", "after publication receipt",
	}
	for _, phase := range providerPhases {
		t.Run(phase, func(t *testing.T) {
			deps := validApplyDistributionOrchestrationDependencies(t, plan)
			var states []persistedDistributionRunState
			deps.writeRun = func(_ string, state persistedDistributionRunState) error { states = append(states, state); return nil }
			deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
				return publishExecutionResult{}, errors.New(phase)
			}
			installDistributionOrchestrationDependencies(t, deps)
			result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
			if err == nil || result == nil || result.Status != "recoverable" || result.Stage != "publish" || !result.Recoverable {
				t.Fatalf("result=%#v error=%v", result, err)
			}
			if len(states) == 0 || states[len(states)-1].Status != "recoverable" {
				t.Fatalf("checkpointed states = %#v", states)
			}
		})
	}
}

func TestDistributionApplyRecoversAcrossFinalReceiptAndFinalStateCrashBoundaries(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	t.Run("final receipt write", func(t *testing.T) {
		deps := validApplyDistributionOrchestrationDependencies(t, plan)
		var states []persistedDistributionRunState
		deps.writeRun = func(_ string, state persistedDistributionRunState) error { states = append(states, state); return nil }
		deps.writeReceipt = func(string, persistedDistributionReceipt) error { return errors.New("final receipt fsync failed") }
		installDistributionOrchestrationDependencies(t, deps)
		result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
		if err == nil || result == nil || !result.Recoverable || result.Status != "recoverable" {
			t.Fatalf("result=%#v error=%v", result, err)
		}
		if len(states) == 0 || states[len(states)-1].Status != "recoverable" {
			t.Fatalf("states = %#v", states)
		}
	})

	t.Run("after final receipt before final state", func(t *testing.T) {
		deps := validApplyDistributionOrchestrationDependencies(t, plan)
		var mutex sync.Mutex
		var state persistedDistributionRunState
		var completion persistedDistributionReceipt
		publishCalls := 0
		failedFinalState := false
		deps.createRun = func(string, string) error { return nil }
		deps.writeRun = func(_ string, candidate persistedDistributionRunState) error {
			mutex.Lock()
			defer mutex.Unlock()
			if candidate.Status == "complete" && !failedFinalState {
				failedFinalState = true
				return errors.New("final state rename failed")
			}
			state = candidate
			return nil
		}
		deps.readRun = func(string, string) (persistedDistributionRunState, error) {
			mutex.Lock()
			defer mutex.Unlock()
			if state.SchemaVersion == 0 {
				return persistedDistributionRunState{}, os.ErrNotExist
			}
			return state, nil
		}
		deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
			publishCalls++
			return validDistributionPublishResult(), nil
		}
		deps.writeReceipt = func(_ string, receipt persistedDistributionReceipt) error { completion = receipt; return nil }
		deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
			if completion.SchemaVersion == 0 {
				return persistedDistributionReceipt{}, os.ErrNotExist
			}
			return completion, nil
		}
		deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
		installDistributionOrchestrationDependencies(t, deps)
		failed, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
		if err == nil || failed == nil || !failed.Recoverable {
			t.Fatalf("failed result=%#v error=%v", failed, err)
		}
		resumed, err := executeDistributionResume(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: failed.RunID, StateDir: plan.Paths.StateDir})
		if err != nil || resumed.Status != "complete" {
			t.Fatalf("resumed=%#v error=%v", resumed, err)
		}
		if publishCalls != 1 {
			t.Fatalf("resume repeated publication after immutable completion receipt: %d calls", publishCalls)
		}
	})
}

func TestCompleteDistributionStatusRequiresExactImmutableReceiptMatch(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	validReceipt := validPersistedDistributionReceipt(run)
	if err := validatePersistedDistributionReceipt(validReceipt); err != nil {
		t.Fatalf("valid completion receipt fixture rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*persistedDistributionReceipt)
		valid  bool
	}{
		{name: "exact", valid: true},
		{name: "plan hash", mutate: func(receipt *persistedDistributionReceipt) { receipt.PlanHash = strings.Repeat("9", 64) }},
		{name: "run ID", mutate: func(receipt *persistedDistributionReceipt) { receipt.RunID = "drun_99999999999999999999999999999999" }},
		{name: "IPA digest", mutate: func(receipt *persistedDistributionReceipt) { receipt.ArtifactSHA256 = strings.Repeat("9", 64) }},
		{name: "descriptor digest", mutate: func(receipt *persistedDistributionReceipt) { receipt.BundleDescriptorSHA256 = strings.Repeat("9", 64) }},
		{name: "publication receipt path", mutate: func(receipt *persistedDistributionReceipt) {
			receipt.PublicationReceiptPath = "publish/elsewhere.json"
		}},
		{name: "publication receipt digest", mutate: func(receipt *persistedDistributionReceipt) {
			receipt.PublicationReceiptSHA256 = strings.Repeat("9", 64)
		}},
		{name: "sensitive link digest", mutate: func(receipt *persistedDistributionReceipt) { receipt.LinkSHA256 = strings.Repeat("9", 64) }},
		{name: "artifact key", mutate: func(receipt *persistedDistributionReceipt) { receipt.ArtifactKey += ".different" }},
		{name: "profile digest", mutate: func(receipt *persistedDistributionReceipt) { receipt.ProfileSHA256 = strings.Repeat("9", 64) }},
		{name: "device set", mutate: func(receipt *persistedDistributionReceipt) { receipt.DeviceSetSHA256 = strings.Repeat("9", 64) }},
		{name: "certificate", mutate: func(receipt *persistedDistributionReceipt) { receipt.CertificateSHA256 = strings.Repeat("9", 64) }},
		{name: "fetch proof", mutate: func(receipt *persistedDistributionReceipt) { receipt.FetchVerified = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt := validReceipt
			if test.mutate != nil {
				test.mutate(&receipt)
			}
			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return run, nil }
			deps.readPlan = func(path string) (persistedDistributionPlan, error) {
				if path != run.PlanPath {
					t.Fatalf("status plan path = %q, want %q", path, run.PlanPath)
				}
				return plan, nil
			}
			deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) { return receipt, nil }
			installDistributionOrchestrationDependencies(t, deps)
			_, err := executeDistributionStatus(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir})
			if test.valid && err != nil {
				t.Fatalf("exact completion rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatalf("mismatched completion receipt accepted: %#v", receipt)
			}
		})
	}
}

func TestIncompleteDistributionStatusStillRequiresExactPlanBinding(t *testing.T) {
	plan := validPersistedDistributionPlan(t)
	run := validCompletedDistributionRun(plan)
	run.Status, run.Stage, run.Recoverable = "recoverable", "publish", true
	run.LastFailureCode = "provider_outcome_unknown"
	for _, test := range []struct {
		name   string
		mutate func(*persistedDistributionRunState, *persistedDistributionPlan)
	}{
		{name: "plan hash", mutate: func(state *persistedDistributionRunState, _ *persistedDistributionPlan) {
			state.PlanHash = strings.Repeat("9", 64)
		}},
		{name: "plan id", mutate: func(state *persistedDistributionRunState, _ *persistedDistributionPlan) {
			state.PlanID = "dplan_ffffffffffffffffffffffffffffffff"
		}},
		{name: "state root", mutate: func(_ *persistedDistributionRunState, plan *persistedDistributionPlan) {
			plan.Paths.StateDir = "/other/root"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidateState, candidatePlan := run, plan
			test.mutate(&candidateState, &candidatePlan)
			deps := panicOnDistributionOrchestrationSideEffects(t)
			deps.readRun = func(string, string) (persistedDistributionRunState, error) { return candidateState, nil }
			deps.readPlan = func(string) (persistedDistributionPlan, error) { return candidatePlan, nil }
			installDistributionOrchestrationDependencies(t, deps)
			if _, err := executeDistributionStatus(context.Background(), distributionTestDepsAt(distributionTestNow), distributionRunRequest{RunID: run.RunID, StateDir: plan.Paths.StateDir}); err == nil {
				t.Fatal("incomplete state with mismatched plan binding was accepted")
			}
		})
	}
}

func TestDistributionProviderErrorsNeverLeakURLUDIDOrSecretCanaries(t *testing.T) {
	canaries := []string{
		"X-Amz-Credential=SHOULD_NOT_RENDER",
		"00008110-RAW-DEVICE-UDID",
		"ASC_PRIVATE_KEY=SHOULD_NOT_RENDER",
	}
	providerError := errors.New("provider failed at https://downloads.example.com/app?" + strings.Join(canaries, "&"))
	plan := validPersistedDistributionPlan(t)
	deps := validApplyDistributionOrchestrationDependencies(t, plan)
	var states []persistedDistributionRunState
	deps.writeRun = func(_ string, state persistedDistributionRunState) error { states = append(states, state); return nil }
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		return publishExecutionResult{}, providerError
	}
	installDistributionOrchestrationDependencies(t, deps)
	result, err := executeDistributionApply(context.Background(), distributionTestDepsAt(distributionTestNow), distributionApplyRequest{PlanPath: "/plans/plan.json", Confirmation: plan.PlanHash})
	if err == nil || result == nil {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	persisted, marshalErr := json.Marshal(states)
	if marshalErr != nil {
		t.Fatalf("json.Marshal(states): %v", marshalErr)
	}
	for _, canary := range canaries {
		if strings.Contains(err.Error(), canary) {
			t.Errorf("returned diagnostic leaks %q: %v", canary, err)
		}
		if strings.Contains(string(persisted), canary) {
			t.Errorf("checkpoint leaks %q: %s", canary, persisted)
		}
	}
}

func TestDistributionPlanNeverPersistsProviderBlockerCanaries(t *testing.T) {
	canaries := []string{
		"X-Amz-Signature=SHOULD_NOT_PERSIST",
		"00008110-RAW-DEVICE-UDID",
		"ASC_S3_SECRET_ACCESS_KEY=SHOULD_NOT_PERSIST",
	}
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	deps := validDistributionOrchestrationDependencies(t)
	deps.reconcilePlan = func(context.Context, *shared.Deps, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		plan := validDistributionReconcilePlan(now)
		plan.Ready = false
		plan.Blockers = []string{"provider blocker at https://example.com/?" + strings.Join(canaries, "&")}
		return plan, nil
	}
	var written persistedDistributionPlan
	deps.writePlan = func(_ string, plan persistedDistributionPlan) error { written = plan; return nil }
	installDistributionOrchestrationDependencies(t, deps)

	result, err := executeDistributionPlan(context.Background(), distributionTestDepsAt(now), distributionPlanRequest{
		ArchivePath: "/archives/Demo.xcarchive", ConfigPath: "/config/distribution.json",
		PlanPath: "/plans/plan.json", StateDir: "/state/runs",
	})
	if err != nil {
		t.Fatalf("executeDistributionPlan() error: %v", err)
	}
	if result.Ready || len(written.Blockers) == 0 {
		t.Fatalf("blocked result = %#v written=%#v", result, written)
	}
	payload, marshalErr := json.Marshal(written)
	if marshalErr != nil {
		t.Fatalf("json.Marshal(plan): %v", marshalErr)
	}
	for _, canary := range canaries {
		if strings.Contains(string(payload), canary) {
			t.Errorf("plan blocker leaks %q: %s", canary, payload)
		}
	}
}

func validDistributionOrchestrationDependencies(t *testing.T) distributionOrchestrationDependencies {
	t.Helper()
	now := distributionTestNow
	return distributionOrchestrationDependencies{
		readConfig: func(string) (distributionConfig, string, error) {
			return validDistributionOrchestrationConfig(), testDistributionConfigSHA256(), nil
		},
		hashProtectedFile: func(string) (string, error) { return testDistributionDeviceFileSHA256, nil },
		inspectIdentity: func(context.Context, *shared.Deps, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
			return validDistributionIdentity(now), nil
		},
		digestArchive: func(context.Context, string) (archiveTreeSnapshot, error) {
			return validDistributionArchiveSnapshot(), nil
		},
		reconcilePlan: func(context.Context, *shared.Deps, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
			return validDistributionReconcilePlan(now), nil
		},
		writePlan: func(string, persistedDistributionPlan) error { return nil },
		readPlan: func(string) (persistedDistributionPlan, error) {
			return persistedDistributionPlan{}, errors.New("unexpected plan read")
		},
		createRun: func(string, string) error { return nil },
		acquireLock: func(context.Context, *shared.Deps, string, string) (func() error, error) {
			return func() error { return nil }, nil
		},
		acquireVerifyLease: func(string, string) (distributionVerifyLease, error) {
			return noopDistributionVerifyLease{}, nil
		},
		recoverEphemeral: func(context.Context, *shared.Deps) error { return nil },
		readRun: func(string, string) (persistedDistributionRunState, error) {
			return persistedDistributionRunState{}, errors.New("unexpected run read")
		},
		writeRun: func(string, persistedDistributionRunState) error { return nil },
		snapshotArchive: func(context.Context, string, string, string) (archiveTreeSnapshot, error) {
			return archiveTreeSnapshot{RelativePath: "inputs/Demo.xcarchive", TreeSHA256: testDistributionArchiveSHA256, SizeBytes: 2048, EntryCount: 10, App: archiveAppIdentity{BundleID: testDistributionBundleID, Title: "Demo", Version: "1.2.3", BuildNumber: "42"}}, nil
		},
		copyArtifact: func(source, _, _, relative string, _ int64) (distributionSizedFileArtifact, error) {
			digest := strings.Repeat("5", 64)
			if strings.Contains(source, "profile") || strings.Contains(relative, "profile") {
				digest = testDistributionProfileSHA256
			} else if strings.Contains(source, "signing") || strings.Contains(relative, "signing") {
				digest = strings.Repeat("6", 64)
			}
			return distributionSizedFileArtifact{Path: relative, SHA256: digest, SizeBytes: 512}, nil
		},
		revalidateArchive: func(context.Context, string, string, distributionArchiveSnapshot) error { return nil },
		preflightPublish:  func(context.Context, *shared.Deps, distributionPublicationConfig) error { return nil },
		reconcileApply: func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
			return validDistributionReconcileReceipt(), nil
		},
		verifyReconcile: func(context.Context, *shared.Deps, signing.ReconcileApplyOptions, string, signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
			return validDistributionReconcileReceipt(), nil
		},
		readReconcilePlan: func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error) {
			return validDistributionReconcilePlan(now), nil
		},
		validateReconcileEvidence: func(string, persistedDistributionPlan, persistedDistributionRunState, signing.ReconcileReceiptView, distributionFileArtifact, distributionProfileArtifact) error {
			return nil
		},
		writeExportOptions: func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error) {
			return &localxcode.ManualReleaseTestingExportOptionsResult{Path: "/state/run/export/ExportOptions.plist", SHA256: testDistributionDescriptorSHA256}, nil
		},
		validateExportOptions: func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error) {
			return &localxcode.ManualReleaseTestingExportOptionsResult{Path: "/state/run/export/ExportOptions.plist", SHA256: testDistributionDescriptorSHA256}, nil
		},
		runSigning: func(ctx context.Context, _ *shared.Deps, _ signing.EphemeralRunOptions, callback func(context.Context) error) error {
			return callback(ctx)
		},
		validateSigningEvidence: func(string, persistedDistributionPlan, persistedDistributionRunState, distributionFileArtifact) error {
			return nil
		},
		validatePreparedBundle: func(*shared.Deps, string, persistedDistributionPlan, persistedDistributionRunState) error { return nil },
		exportIPA: func(context.Context, localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
			return &localxcode.ExportResult{IPAPath: "/state/run/export/Demo.ipa", BundleID: testDistributionBundleID, Version: "1.2.3", BuildNumber: "42"}, nil
		},
		hashFile: fakeDistributionSizedArtifact,
		prepareIPA: func(context.Context, distributionPrepareRequest) (core.PrepareResult, error) {
			return validDistributionPrepareResult(), nil
		},
		publish: func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
			return validDistributionPublishResult(), nil
		},
		reverifyPublish: func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
			return validDistributionPublishResult(), nil
		},
		observeDevice: func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error) {
			return deviceObservation{}, nil
		},
		readReceipt: func(string, string) (persistedDistributionReceipt, error) {
			return persistedDistributionReceipt{}, os.ErrNotExist
		},
		writeReceipt: func(string, persistedDistributionReceipt) error { return nil },
		validateCompletion: func(plan persistedDistributionPlan, state persistedDistributionRunState, receipt persistedDistributionReceipt) error {
			if !completionMatchesRunPlan(receipt, state, plan) {
				return errors.New("completion mismatch")
			}
			return nil
		},
	}
}

func validApplyDistributionOrchestrationDependencies(t *testing.T, plan persistedDistributionPlan) distributionOrchestrationDependencies {
	t.Helper()
	deps := validDistributionOrchestrationDependencies(t)
	deps.readPlan = func(string) (persistedDistributionPlan, error) { return plan, nil }
	deps.readConfig = func(string) (distributionConfig, string, error) {
		return validDistributionOrchestrationConfig(), plan.ConfigSHA256, nil
	}
	deps.readRun = func(string, string) (persistedDistributionRunState, error) {
		return persistedDistributionRunState{}, os.ErrNotExist
	}
	return deps
}

func panicOnDistributionOrchestrationSideEffects(t *testing.T) distributionOrchestrationDependencies {
	t.Helper()
	panicCall := func(name string) { t.Fatalf("unexpected non-local or mutating dependency call: %s", name) }
	deps := validDistributionOrchestrationDependencies(t)
	deps.readConfig = func(string) (distributionConfig, string, error) {
		panicCall("readConfig")
		return distributionConfig{}, "", nil
	}
	deps.hashProtectedFile = func(string) (string, error) { panicCall("hashProtectedFile"); return "", nil }
	deps.inspectIdentity = func(context.Context, *shared.Deps, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error) {
		panicCall("inspectIdentity")
		return signing.PKCS12IdentityInfo{}, nil
	}
	deps.digestArchive = func(context.Context, string) (archiveTreeSnapshot, error) {
		panicCall("digestArchive")
		return archiveTreeSnapshot{}, nil
	}
	deps.reconcilePlan = func(context.Context, *shared.Deps, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error) {
		panicCall("reconcilePlan")
		return signing.ReconcilePlanView{}, nil
	}
	deps.writePlan = func(string, persistedDistributionPlan) error { panicCall("writePlan"); return nil }
	deps.createRun = func(string, string) error { panicCall("createRun"); return nil }
	deps.acquireLock = func(context.Context, *shared.Deps, string, string) (func() error, error) {
		panicCall("acquireLock")
		return func() error { return nil }, nil
	}
	deps.recoverEphemeral = func(context.Context, *shared.Deps) error { panicCall("recoverEphemeral"); return nil }
	deps.writeRun = func(string, persistedDistributionRunState) error { panicCall("writeRun"); return nil }
	deps.snapshotArchive = func(context.Context, string, string, string) (archiveTreeSnapshot, error) {
		panicCall("snapshotArchive")
		return archiveTreeSnapshot{}, nil
	}
	deps.copyArtifact = func(string, string, string, string, int64) (distributionSizedFileArtifact, error) {
		panicCall("copyArtifact")
		return distributionSizedFileArtifact{}, nil
	}
	deps.revalidateArchive = func(context.Context, string, string, distributionArchiveSnapshot) error {
		panicCall("revalidateArchive")
		return nil
	}
	deps.preflightPublish = func(context.Context, *shared.Deps, distributionPublicationConfig) error {
		panicCall("preflightPublish")
		return nil
	}
	deps.reconcileApply = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error) {
		panicCall("reconcileApply")
		return signing.ReconcileReceiptView{}, nil
	}
	deps.verifyReconcile = func(context.Context, *shared.Deps, signing.ReconcileApplyOptions, string, signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error) {
		panicCall("verifyReconcile")
		return signing.ReconcileReceiptView{}, nil
	}
	deps.writeExportOptions = func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error) {
		panicCall("writeExportOptions")
		return nil, nil
	}
	deps.runSigning = func(context.Context, *shared.Deps, signing.EphemeralRunOptions, func(context.Context) error) error {
		panicCall("runSigning")
		return nil
	}
	deps.exportIPA = func(context.Context, localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error) {
		panicCall("exportIPA")
		return nil, nil
	}
	deps.hashFile = func(string) (distributionSizedFileArtifact, error) {
		panicCall("hashFile")
		return distributionSizedFileArtifact{}, nil
	}
	deps.prepareIPA = func(context.Context, distributionPrepareRequest) (core.PrepareResult, error) {
		panicCall("prepareIPA")
		return core.PrepareResult{}, nil
	}
	deps.publish = func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error) {
		panicCall("publish")
		return publishExecutionResult{}, nil
	}
	deps.reverifyPublish = func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error) {
		panicCall("reverifyPublish")
		return publishExecutionResult{}, nil
	}
	deps.observeDevice = func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error) {
		panicCall("observeDevice")
		return deviceObservation{}, nil
	}
	deps.readReceipt = func(string, string) (persistedDistributionReceipt, error) {
		panicCall("readReceipt")
		return persistedDistributionReceipt{}, nil
	}
	deps.writeReceipt = func(string, persistedDistributionReceipt) error { panicCall("writeReceipt"); return nil }
	return deps
}

func installDistributionOrchestrationDependencies(t *testing.T, deps distributionOrchestrationDependencies) {
	t.Helper()
	original := distributionOrchestrationDeps
	distributionOrchestrationDeps = deps
	t.Cleanup(func() { distributionOrchestrationDeps = original })
}

func fakeDistributionSizedArtifact(path string) (distributionSizedFileArtifact, error) {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	switch {
	case strings.HasSuffix(cleaned, ".ipa"):
		return distributionSizedFileArtifact{Path: path, SHA256: testDistributionIPASHA256, SizeBytes: 1234}, nil
	case strings.HasSuffix(cleaned, "/bundle.json"):
		return distributionSizedFileArtifact{Path: path, SHA256: testDistributionDescriptorSHA256, SizeBytes: 2048}, nil
	case strings.HasSuffix(cleaned, "/publish/receipt.json"):
		return distributionSizedFileArtifact{Path: path, SHA256: testDistributionReceiptSHA256, SizeBytes: 1024}, nil
	case strings.HasSuffix(cleaned, "/secrets/links.json"), strings.HasSuffix(cleaned, "/intent.json"), strings.HasSuffix(cleaned, "/publication-intent.json"):
		return distributionSizedFileArtifact{Path: path, SHA256: strings.Repeat("7", 64), SizeBytes: 1024}, nil
	case strings.HasSuffix(cleaned, "/signing/receipt.json"):
		return distributionSizedFileArtifact{Path: path, SHA256: strings.Repeat("6", 64), SizeBytes: 512}, nil
	default:
		return distributionSizedFileArtifact{}, errors.New("unexpected orchestration file hash path")
	}
}

func validDistributionOrchestrationConfig() distributionConfig {
	return distributionConfig{
		SchemaVersion: 1,
		DevicesFile:   "/config/devices.json",
		Signing: distributionSigningConfig{
			Identity: distributionIdentityConfig{
				Format: "pkcs12", Path: "/config/distribution.p12", PasswordFile: "/config/password.txt",
				CertificateSHA256: testDistributionCertificateSHA256,
			},
			MinimumValidityDays: 7,
			MaxMutations:        32,
		},
		Publication: distributionPublicationConfig{
			Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com",
			Region: "auto", Bucket: "ios-builds", Prefix: "team/app", AddressingStyle: "path",
			URLTTL: "24h", DownloadGrace: "1h", VerifyTimeout: "30s",
			URLTTLDuration: 24 * time.Hour, DownloadGraceDuration: time.Hour, VerifyTimeoutDuration: 30 * time.Second,
		},
		Metadata: distributionMetadataConfig{Title: "Demo", Channel: "pull-request-42", SourceRevision: "abc123", SourceURL: "https://example.com/team/app/commit/abc123"},
	}
}

func validDistributionIdentity(now time.Time) signing.PKCS12IdentityInfo {
	return signing.PKCS12IdentityInfo{
		CertificateSHA256: strings.ToUpper(testDistributionCertificateSHA256),
		CertificateSHA1:   testDistributionCertificateSHA1,
		TeamID:            testDistributionTeamID,
		NotBefore:         now.Add(-24 * time.Hour),
		NotAfter:          now.Add(365 * 24 * time.Hour),
	}
}

func validDistributionArchiveSnapshot() archiveTreeSnapshot {
	return archiveTreeSnapshot{TreeSHA256: testDistributionArchiveSHA256, SizeBytes: 2048, EntryCount: 10, App: archiveAppIdentity{
		BundleID: testDistributionBundleID, Title: "Demo", Version: "1.2.3", BuildNumber: "42",
	}}
}

func validDistributionReconcilePlan(now time.Time) signing.ReconcilePlanView {
	return signing.ReconcilePlanView{
		SchemaVersion: 1, PlanHash: strings.Repeat("1", 64), Ready: true, MutationCount: 3, MaxMutations: 32,
		DeviceCount: 2, DeviceSetSHA256: testDistributionDeviceSetSHA256, TeamID: testDistributionTeamID,
		ArchivePath: "/archives/Demo.xcarchive", DevicesFile: "/config/devices.json",
		StateDir: "/plans/reconcile", PlanPath: "/plans/reconcile/plan.json", ReceiptPath: "/plans/reconcile/receipt.json", ProfilesDir: "/plans/reconcile/profiles",
		MinimumValidityDays: 7,
		Certificate: &signing.ReconcileCertificateView{
			ResourceID: "cert-resource-id", SHA256: testDistributionCertificateSHA256, TeamID: testDistributionTeamID,
			ExpirationDate: now.Add(365 * 24 * time.Hour).Format(time.RFC3339),
		},
		Targets: []signing.ReconcileTargetView{{Kind: "application", BundleID: testDistributionBundleID}},
		Actions: []signing.ReconcileActionView{
			{Kind: "registerDevice"},
			{Kind: "createBundleID", BundleID: testDistributionBundleID},
			{Kind: "createProfile", BundleID: testDistributionBundleID},
		},
	}
}

func validPersistedDistributionPlan(t *testing.T) persistedDistributionPlan {
	t.Helper()
	config := validDistributionOrchestrationConfig()
	plan := persistedDistributionPlan{
		SchemaVersion: 1,
		PlanID:        "dplan_0123456789abcdef0123456789abcdef",
		CreatedAt:     "2026-08-13T08:00:00Z",
		Ready:         true,
		ConfigPath:    "/config/distribution.json",
		ConfigSHA256:  testDistributionConfigSHA256(),
		Archive: distributionArchiveBinding{
			Path: "/archives/Demo.xcarchive", TreeSHA256: testDistributionArchiveSHA256,
			SizeBytes: 2048, FileCount: 10, BundleID: testDistributionBundleID, Title: "Demo", PublishedTitle: "Demo", Version: "1.2.3", BuildNumber: "42",
			TeamID: testDistributionTeamID, TargetCount: 1,
		},
		DeviceSet: distributionDeviceSetBinding{SHA256: testDistributionDeviceSetSHA256, FileSHA256: testDistributionDeviceFileSHA256, Count: 2},
		Identity: distributionIdentityBinding{
			CertificateResourceID: "cert-resource-id", CertificateSHA256: testDistributionCertificateSHA256,
			TeamID: testDistributionTeamID, ExpirationDate: "2027-08-13T08:00:00Z", MinimumValidUntil: "2026-08-20T08:00:00Z",
		},
		Publication: config.Publication,
		Reconcile: distributionReconcileBinding{
			PlanPath: "/plans/reconcile/plan.json", PlanHash: strings.Repeat("1", 64), ReceiptPath: "/plans/reconcile/receipt.json",
			MinimumValidityDays: 7, MutationCount: 3, MaxMutations: 32,
		},
		Effects: []distributionEffect{
			{Stage: "account_reconcile", Kind: "register_device", Count: 1},
			{Stage: "account_reconcile", Kind: "create_bundle_id", BundleID: testDistributionBundleID},
			{Stage: "account_reconcile", Kind: "create_profile", BundleID: testDistributionBundleID},
			{Stage: "account_reconcile", Kind: "write_profile", BundleID: testDistributionBundleID},
			{Stage: "export", Kind: "write_export_options", BundleID: testDistributionBundleID},
			{Stage: "export", Kind: "write_ipa", BundleID: testDistributionBundleID},
			{Stage: "prepare", Kind: "write_bundle", BundleID: testDistributionBundleID},
			{Stage: "publish", Kind: "ensure_ipa"},
			{Stage: "publish", Kind: "ensure_manifest"},
			{Stage: "publish", Kind: "ensure_install_page"},
		},
		Paths: distributionPlanPaths{StateDir: "/state/runs"},
	}
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatalf("sealDistributionPlan(): %v", err)
	}
	if err := validatePersistedDistributionPlan(plan); err != nil {
		t.Fatalf("valid plan fixture rejected: %v", err)
	}
	return plan
}

func validDistributionReconcileReceipt() signing.ReconcileReceiptView {
	profile := signing.ReconcileProfileView{
		TargetKind: "application", BundleID: testDistributionBundleID, ResourceID: "profile-resource-id",
		UUID: testDistributionProfileUUID, Path: "/state/reconcile/profile.mobileprovision", SHA256: testDistributionProfileSHA256,
	}
	return signing.ReconcileReceiptView{
		SchemaVersion: 1, PlanHash: strings.Repeat("1", 64), Complete: true,
		ReceiptPath: "/plans/reconcile/receipt.json", MainProfile: &profile, Profiles: []signing.ReconcileProfileView{profile},
	}
}

func validDistributionPrepareResult() core.PrepareResult {
	verified := core.CodeSignatureVerification{Status: core.CodeSignatureVerified}
	return core.PrepareResult{
		BundlePath: "/state/run/prepared/bundle",
		Descriptor: core.Descriptor{
			SchemaVersion: "1", Platform: "IOS", DistributionMethod: "release-testing",
			App:      core.App{BundleID: testDistributionBundleID, Title: "Demo", Version: "1.2.3", BuildNumber: "42"},
			Artifact: core.Artifact{RelativePath: "payload/app.ipa", SHA256: testDistributionIPASHA256, SizeBytes: 1234},
			Signing: core.Signing{
				ProfileClass: core.ProfileClassAdHoc, ProfileUUID: testDistributionProfileUUID, TeamID: testDistributionTeamID,
				ExpiresAt: "2027-08-13T08:00:00Z", DeviceCount: 2, DeviceSetSHA256: testDistributionDeviceSetSHA256,
				EmbeddedProfileSHA256:                testDistributionProfileSHA256,
				ProfileCertificateSHA256Fingerprints: []string{testDistributionCertificateSHA256},
				ProfileIntegrityVerification:         verified, ProfileTrustVerification: verified,
				CodeSignatureVerification: core.CodeSignatureVerification{
					Status: core.CodeSignatureVerified, Scope: core.CodeSignatureScopeCompleteMainApp,
					SignerCertificateSHA256Fingerprints: []string{testDistributionCertificateSHA256},
				},
			},
		},
	}
}

func validDistributionPublishResult() publishExecutionResult {
	descriptor := validDistributionPrepareResult().Descriptor
	expiresAt := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	verified := core.PreparedCodeSignatureVerification{Status: string(core.CodeSignatureVerified)}
	return publishExecutionResult{Receipt: core.PublishReceipt{
		SchemaVersion: "1", Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com",
		Region: "auto", AddressingStyle: "path", Access: core.AccessPrivate, Bucket: "ios-builds", Prefix: "team/app",
		URLTTL: "24h0m0s", DownloadGrace: "1h0m0s", Verified: true, ExpiresAt: &expiresAt,
		Artifact:   core.StoredObject{Key: "team/app/objects/sha256/" + testDistributionIPASHA256 + ".ipa", SHA256: testDistributionIPASHA256, SizeBytes: 1234, ContentType: core.ContentTypeIPA, Status: "uploaded"},
		Manifest:   core.StoredObject{Key: "team/app/links/random/manifest.plist", SHA256: strings.Repeat("2", 64), SizeBytes: 512, ContentType: core.ContentTypeManifest, Status: "reused"},
		Page:       core.StoredObject{Key: "team/app/links/random/index.html", SHA256: strings.Repeat("3", 64), SizeBytes: 512, ContentType: core.ContentTypeHTML, Status: "uploaded"},
		InstallURL: "https://downloads.example.com/REDACTED", DirectInstallURL: "itms-services://?action=download-manifest&url=REDACTED", App: core.PreparedApp{
			BundleID: descriptor.App.BundleID, Title: descriptor.App.Title, Version: descriptor.App.Version, BuildNumber: descriptor.App.BuildNumber,
		},
		Signing: core.ReceiptSigning{
			ProfileClass: string(descriptor.Signing.ProfileClass), ProfileUUID: descriptor.Signing.ProfileUUID,
			EmbeddedProfileSHA256: descriptor.Signing.EmbeddedProfileSHA256, TeamID: descriptor.Signing.TeamID,
			ProfileExpiresAt: descriptor.Signing.ExpiresAt, DeviceCount: descriptor.Signing.DeviceCount,
			DeviceSetSHA256:                descriptor.Signing.DeviceSetSHA256,
			ProfileCertificateFingerprints: descriptor.Signing.ProfileCertificateSHA256Fingerprints,
			ProfileIntegrityVerification:   verified,
			ProfileTrustVerification:       verified,
			CodeSignatureVerification: core.PreparedCodeSignatureVerification{
				Status: string(core.CodeSignatureVerified), Scope: core.CodeSignatureScopeCompleteMainApp,
				SignerCertificateSHA256Fingerprints: []string{testDistributionCertificateSHA256},
			},
		},
		ReceiptPath: "/state/run/publish/receipt.json", LinkPath: "/state/run/secrets/links.json",
	}}
}

func validDistributionPublishResultForCompletion(plan persistedDistributionPlan, run persistedDistributionRunState) publishExecutionResult {
	result := validDistributionPublishResult()
	runRoot := filepath.Join(plan.Paths.StateDir, run.RunID)
	result.Receipt.ReceiptPath = filepath.Join(runRoot, filepath.FromSlash(run.Artifacts.Publication.ReceiptPath))
	result.Receipt.LinkPath = filepath.Join(runRoot, filepath.FromSlash(run.Artifacts.Publication.LinkPath))
	return result
}

func validCompletedDistributionRun(plan persistedDistributionPlan) persistedDistributionRunState {
	runID := deterministicDistributionRunID(plan)
	return persistedDistributionRunState{
		SchemaVersion: 1, RunID: runID, PlanID: plan.PlanID,
		PlanPath: "/plans/plan.json", PlanHash: plan.PlanHash, Status: "complete", Stage: "complete",
		UpdatedAt: "2026-08-13T08:15:00Z", Attempt: 1,
		Artifacts: distributionRunArtifacts{
			ArchiveSnapshot: &distributionArchiveSnapshot{
				RelativePath: "inputs/Demo.xcarchive", TreeSHA256: testDistributionArchiveSHA256, SizeBytes: 2048, EntryCount: 10,
				App: archiveAppIdentity{BundleID: testDistributionBundleID, Title: "Demo", Version: "1.2.3", BuildNumber: "42"},
			},
			ReconcileReceipt: &distributionFileArtifact{Path: "reconcile/receipt.json", SHA256: strings.Repeat("5", 64)},
			SigningReceipt:   &distributionFileArtifact{Path: "signing/receipt.json", SHA256: strings.Repeat("6", 64)},
			ExportOptions:    &distributionFileArtifact{Path: "export/ExportOptions.plist", SHA256: testDistributionDescriptorSHA256},
			Profile:          &distributionProfileArtifact{ResourceID: "profile-resource-id", UUID: testDistributionProfileUUID, Path: "reconcile/profile.mobileprovision", SHA256: testDistributionProfileSHA256, BundleID: testDistributionBundleID},
			IPA:              &distributionSizedFileArtifact{Path: "export/Demo.ipa", SHA256: testDistributionIPASHA256, SizeBytes: 1234},
			Bundle:           &distributionBundleArtifact{Path: "prepared/bundle", DescriptorSHA256: testDistributionDescriptorSHA256},
			Publication: &distributionPublicationArtifact{
				ReceiptPath: "publish/receipt.json", ReceiptSHA256: testDistributionReceiptSHA256,
				LinkPath: "secrets/links.json", LinkSHA256: strings.Repeat("7", 64), ArtifactKey: "team/app/objects/sha256/" + testDistributionIPASHA256 + ".ipa",
				ManifestKey: "team/app/links/random/manifest.plist", PageKey: "team/app/links/random/index.html",
				InstallURLRedacted: "https://downloads.example.com/REDACTED",
			},
		},
	}
}

func validPersistedDistributionReceipt(run persistedDistributionRunState) persistedDistributionReceipt {
	return persistedDistributionReceipt{
		SchemaVersion: 1, RunID: run.RunID, PlanID: run.PlanID, PlanHash: run.PlanHash, Status: "published_and_fetch_verified",
		CompletedAt: "2026-08-13T08:15:00Z", PublicationReceiptPath: run.Artifacts.Publication.ReceiptPath,
		PublicationReceiptSHA256: run.Artifacts.Publication.ReceiptSHA256, LinkPath: run.Artifacts.Publication.LinkPath, LinkSHA256: run.Artifacts.Publication.LinkSHA256,
		ArtifactSHA256: testDistributionIPASHA256, BundleDescriptorSHA256: testDistributionDescriptorSHA256,
		ArtifactKey: run.Artifacts.Publication.ArtifactKey, ManifestKey: run.Artifacts.Publication.ManifestKey,
		PageKey: run.Artifacts.Publication.PageKey, InstallURLRedacted: run.Artifacts.Publication.InstallURLRedacted,
		AppBundleID: testDistributionBundleID, AppVersion: "1.2.3", AppBuildNumber: "42",
		TeamID: testDistributionTeamID, ProfileResourceID: "profile-resource-id", ProfileClass: string(core.ProfileClassAdHoc),
		ProfileUUID: testDistributionProfileUUID, ProfileExpiresAt: "2027-08-13T08:00:00Z", ProfileSHA256: testDistributionProfileSHA256,
		DeviceSetSHA256: testDistributionDeviceSetSHA256, DeviceCount: 2, CertificateSHA256: testDistributionCertificateSHA256,
		ProfileCertificateSHA256Fingerprints: []string{testDistributionCertificateSHA256},
		SignerCertificateSHA256Fingerprints:  []string{testDistributionCertificateSHA256},
		ProfileIntegrityStatus:               string(core.CodeSignatureVerified), ProfileTrustStatus: string(core.CodeSignatureVerified),
		CodeSignatureStatus: string(core.CodeSignatureVerified), CodeSignatureScope: core.CodeSignatureScopeCompleteMainApp,
		ArtifactSizeBytes: 1234, BundleDescriptorSizeBytes: 2048,
		ManifestSHA256: strings.Repeat("2", 64), ManifestSizeBytes: 512,
		PageSHA256: strings.Repeat("3", 64), PageSizeBytes: 512,
		FetchVerified: true, FetchVerifiedAt: "2026-08-13T08:14:00Z",
	}
}

func testDistributionConfigSHA256() string { return strings.Repeat("4", 64) }

func assertDistributionPlanEffects(t *testing.T, effects []distributionEffect) {
	t.Helper()
	want := map[string]bool{
		"account_reconcile/register_device":  false,
		"account_reconcile/create_bundle_id": false,
		"account_reconcile/create_profile":   false,
		"account_reconcile/write_profile":    false,
		"export/write_export_options":        false,
		"export/write_ipa":                   false,
		"prepare/write_bundle":               false,
		"publish/ensure_ipa":                 false,
		"publish/ensure_manifest":            false,
		"publish/ensure_install_page":        false,
	}
	for _, effect := range effects {
		key := effect.Stage + "/" + effect.Kind
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected effect %q", key)
		}
		want[key] = true
	}
	for effect, found := range want {
		if !found {
			t.Errorf("missing effect %q", effect)
		}
	}
}

func assertBefore(t *testing.T, calls []string, ordered ...string) {
	t.Helper()
	position := -1
	for _, want := range ordered {
		found := -1
		for index := position + 1; index < len(calls); index++ {
			if calls[index] == want {
				found = index
				break
			}
		}
		if found < 0 {
			t.Fatalf("call %q missing or out of order in %v", want, calls)
		}
		position = found
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
