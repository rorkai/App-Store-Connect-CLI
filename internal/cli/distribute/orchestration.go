package distribute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const (
	distributionPlanStagePreflight  = "preflight"
	distributionMaximumValidityDays = 3650
	distributionRunReceiptName      = "receipt.json"
	distributionArchiveRelative     = "inputs/App.xcarchive"
	distributionProfileRelative     = "reconcile/profile.mobileprovision"
	distributionReconcileRelative   = "reconcile/receipt.json"
	distributionExportOptionsRel    = "export/ExportOptions.plist"
	distributionBundleRelative      = "prepared/bundle"
	distributionPublishReceiptRel   = "publish/receipt.json"
	distributionPublishIntentRel    = "secrets/publication-intent.json"
)

var (
	errDistributionArtifactConflict                    = errors.New("distribution run artifact conflict")
	errDistributionRunPathChanged                      = errors.New("distribution run path changed")
	errDistributionPublicationCredentialsExpireTooSoon = errors.New("distribution publication credentials expire too soon")
)

const distributionPublicationValiditySafetyMargin = time.Minute

type distributionPlanRequest struct {
	ArchivePath string
	ConfigPath  string
	PlanPath    string
	StateDir    string
}

type distributionApplyRequest struct {
	PlanPath     string
	Confirmation string
}

type distributionRunRequest struct {
	RunID    string
	StateDir string
}

type distributionVerifyRequest struct {
	RunID    string
	StateDir string
	Device   string
	Timeout  time.Duration
}

type (
	distributionPlan     = persistedDistributionPlan
	distributionRunState = persistedDistributionRunState
)

type distributionVerificationResult struct {
	SchemaVersion       int                `json:"schemaVersion"`
	RunID               string             `json:"runId"`
	PlanHash            string             `json:"planHash"`
	PublicationVerified bool               `json:"publicationVerified"`
	VerifiedAt          string             `json:"verifiedAt"`
	ArtifactSHA256      string             `json:"artifactSha256"`
	AppBundleID         string             `json:"appBundleId"`
	AppVersion          string             `json:"appVersion"`
	AppBuildNumber      string             `json:"appBuildNumber"`
	DeviceObservation   *deviceObservation `json:"deviceObservation,omitempty"`
}

type distributionPrepareRequest struct {
	StateDir        string
	RunID           string
	IPARelativePath string
	ExpectedSHA256  string
	ExpectedSize    int64
	Metadata        distributionMetadataConfig
}

type distributionDeviceObservationRequest struct {
	Device   string
	BundleID string
	Version  string
	Build    string
	Timeout  time.Duration
}

type distributionOrchestrationDependencies struct {
	readConfig                func(string) (distributionConfig, string, error)
	hashProtectedFile         func(string) (string, error)
	inspectIdentity           func(context.Context, signing.PKCS12IdentityOptions) (signing.PKCS12IdentityInfo, error)
	digestArchive             func(context.Context, string) (archiveTreeSnapshot, error)
	reconcilePlan             func(context.Context, signing.ReconcilePlanOptions) (signing.ReconcilePlanView, error)
	writePlan                 func(string, persistedDistributionPlan) error
	readPlan                  func(string) (persistedDistributionPlan, error)
	createRun                 func(string, string) error
	ensureRun                 func(string, string) error
	acquireLock               func(context.Context, *shared.Deps, string, string) (func() error, error)
	acquireLease              func(context.Context, *shared.Deps, string, string) (func() error, func() error, error)
	acquireVerifyLease        func(string, string) (distributionVerifyLease, error)
	recoverEphemeral          func(context.Context) error
	readRun                   func(string, string) (persistedDistributionRunState, error)
	writeRun                  func(string, persistedDistributionRunState) error
	snapshotArchive           func(context.Context, string, string, string) (archiveTreeSnapshot, error)
	copyArtifact              func(string, string, string, string, int64) (distributionSizedFileArtifact, error)
	revalidateArchive         func(context.Context, string, string, distributionArchiveSnapshot) error
	preflightPublish          func(context.Context, *shared.Deps, distributionPublicationConfig) error
	reconcileApply            func(context.Context, signing.ReconcileApplyOptions) (signing.ReconcileReceiptView, error)
	verifyReconcile           func(context.Context, signing.ReconcileApplyOptions, string, signing.ReconcileCompletionEvidence) (signing.ReconcileReceiptView, error)
	readReconcilePlan         func(signing.ReconcileApplyOptions) (signing.ReconcilePlanView, error)
	validateReconcileEvidence func(string, persistedDistributionPlan, persistedDistributionRunState, signing.ReconcileReceiptView, distributionFileArtifact, distributionProfileArtifact) error
	writeExportOptions        func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error)
	validateExportOptions     func(context.Context, localxcode.ManualReleaseTestingExportOptions) (*localxcode.ManualReleaseTestingExportOptionsResult, error)
	runSigning                func(context.Context, signing.EphemeralRunOptions, func(context.Context) error) error
	validateSigningEvidence   func(string, persistedDistributionPlan, persistedDistributionRunState, distributionFileArtifact) error
	validatePreparedBundle    func(string, persistedDistributionPlan, persistedDistributionRunState) error
	exportIPA                 func(context.Context, localxcode.ReleaseTestingExportOptions) (*localxcode.ExportResult, error)
	hashFile                  func(string) (distributionSizedFileArtifact, error)
	pathExists                func(string) (bool, error)
	prepareIPA                func(context.Context, distributionPrepareRequest) (core.PrepareResult, error)
	publish                   func(context.Context, *shared.Deps, privatePublishIntentRequest) (publishExecutionResult, error)
	reverifyPublish           func(context.Context, *shared.Deps, privatePublishVerificationRequest) (publishExecutionResult, error)
	observeDevice             func(context.Context, distributionDeviceObservationRequest) (deviceObservation, error)
	readReceipt               func(string, string) (persistedDistributionReceipt, error)
	writeReceipt              func(string, persistedDistributionReceipt) error
	validateCompletion        func(persistedDistributionPlan, persistedDistributionRunState, persistedDistributionReceipt) error
}

var distributionOrchestrationDeps = productionDistributionOrchestrationDependencies()

func productionDistributionOrchestrationDependencies() distributionOrchestrationDependencies {
	return distributionOrchestrationDependencies{
		readConfig:                readDistributionConfig,
		hashProtectedFile:         hashProtectedDistributionFile,
		inspectIdentity:           signing.InspectPKCS12Identity,
		digestArchive:             digestXCArchive,
		reconcilePlan:             signing.ExecuteReconcilePlan,
		writePlan:                 writePersistedDistributionPlan,
		readPlan:                  readPersistedDistributionPlan,
		createRun:                 createDistributionRunScaffold,
		ensureRun:                 ensureDistributionRunScaffold,
		acquireLock:               acquireDistributionRunLock,
		acquireLease:              acquireDistributionRunLease,
		acquireVerifyLease:        acquireDistributionVerifyPathLease,
		recoverEphemeral:          signing.RecoverEphemeral,
		readRun:                   readDistributionRunState,
		writeRun:                  writeDistributionRunState,
		snapshotArchive:           snapshotDistributionArchive,
		copyArtifact:              copyDistributionArtifact,
		revalidateArchive:         revalidateDistributionArchive,
		preflightPublish:          preflightDistributionPublication,
		reconcileApply:            signing.ExecuteReconcileApply,
		verifyReconcile:           signing.VerifyReconcileCompletionFromEvidence,
		readReconcilePlan:         signing.ReadReconcilePlan,
		validateReconcileEvidence: validateDistributionReconcileEvidence,
		writeExportOptions:        localxcode.WriteManualReleaseTestingExportOptions,
		validateExportOptions:     localxcode.ValidateManualReleaseTestingExportOptions,
		runSigning:                signing.RunEphemeral,
		validateSigningEvidence:   validateDistributionSigningEvidence,
		validatePreparedBundle:    validateExistingDistributionPreparedBundle,
		exportIPA:                 localxcode.ExportReleaseTesting,
		hashFile:                  hashDistributionFile,
		pathExists:                distributionPathExists,
		prepareIPA:                prepareDistributionIPA,
		publish:                   executePrivatePublishIntent,
		reverifyPublish:           reverifyPrivatePublishIntent,
		observeDevice:             observeDistributionDevice,
		readReceipt:               readDistributionReceipt,
		writeReceipt:              writeDistributionReceipt,
		validateCompletion:        validateExactDistributionCompletion,
	}
}

func executeDistributionPlan(ctx context.Context, d *shared.Deps, request distributionPlanRequest) (*distributionPlan, error) {
	archivePath, err := canonicalAbsolutePath(request.ArchivePath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("archive_path_invalid")
	}
	configPath, err := canonicalAbsolutePath(request.ConfigPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("config_path_invalid")
	}
	planPath, err := canonicalAbsolutePath(request.PlanPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("plan_path_invalid")
	}
	stateDir, err := canonicalAbsolutePath(request.StateDir)
	if err != nil {
		return nil, safeDistributionPlanningFailure("state_path_invalid")
	}
	config, configSHA, err := distributionOrchestrationDeps.readConfig(configPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("config_read_failed")
	}
	devicesPath, err := resolveDistributionConfigPath(configPath, config.DevicesFile)
	if err != nil {
		return nil, safeDistributionPlanningFailure("devices_path_invalid")
	}
	identityPath, err := resolveDistributionConfigPath(configPath, config.Signing.Identity.Path)
	if err != nil {
		return nil, safeDistributionPlanningFailure("identity_path_invalid")
	}
	passwordPath, err := resolveOptionalDistributionConfigPath(configPath, config.Signing.Identity.PasswordFile)
	if err != nil {
		return nil, safeDistributionPlanningFailure("identity_password_path_invalid")
	}
	deviceFileSHA, err := distributionOrchestrationDeps.hashProtectedFile(devicesPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("devices_read_failed")
	}
	identity, err := distributionOrchestrationDeps.inspectIdentity(ctx, signing.PKCS12IdentityOptions{
		IdentityPath: identityPath, IdentityPasswordPath: passwordPath,
	})
	if err != nil {
		return nil, safeDistributionPlanningFailure("identity_inspection_failed")
	}
	identity.CertificateSHA256 = strings.ToLower(identity.CertificateSHA256)
	configuredFingerprint := strings.ToLower(strings.TrimSpace(config.Signing.Identity.CertificateSHA256))
	if configuredFingerprint != "" && configuredFingerprint != identity.CertificateSHA256 {
		return nil, safeDistributionPlanningFailure("identity_certificate_mismatch")
	}
	now := d.Now().UTC()
	effectiveMinimumValidityDays, err := effectiveDistributionMinimumValidityDays(config.Signing.MinimumValidityDays, config.Publication.URLTTLDuration, config.Publication.DownloadGraceDuration)
	if err != nil {
		return nil, safeDistributionPlanningFailure("validity_policy_invalid")
	}
	minimumValidUntil := now.Add(time.Duration(effectiveMinimumValidityDays) * 24 * time.Hour)
	if !identity.NotAfter.After(minimumValidUntil) {
		return nil, safeDistributionPlanningFailure("identity_validity_failed")
	}
	archive, err := distributionOrchestrationDeps.digestArchive(ctx, archivePath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("archive_inspection_failed")
	}
	planID, err := newDistributionPlanID()
	if err != nil {
		return nil, safeDistributionPlanningFailure("plan_id_failed")
	}
	reconcileDir := filepath.Join(filepath.Dir(planPath), planID+"-reconcile")
	reconcile, err := distributionOrchestrationDeps.reconcilePlan(ctx, signing.ReconcilePlanOptions{
		ArchivePath: archivePath, DevicesFile: devicesPath,
		CertificateSHA256:   identity.CertificateSHA256,
		MinimumValidityDays: effectiveMinimumValidityDays, MaxMutations: config.Signing.MaxMutations,
		StateDir: reconcileDir,
	})
	if err != nil {
		return nil, safeDistributionPlanningFailure("account_reconcile_failed")
	}
	if reconcile.Certificate == nil {
		return nil, safeDistributionPlanningFailure("account_certificate_missing")
	}
	if reconcile.MinimumValidityDays != effectiveMinimumValidityDays || reconcile.MaxMutations != config.Signing.MaxMutations {
		return nil, safeDistributionPlanningFailure("account_policy_mismatch")
	}
	if !strings.EqualFold(reconcile.Certificate.SHA256, identity.CertificateSHA256) {
		return nil, safeDistributionPlanningFailure("account_certificate_mismatch")
	}
	if reconcile.TeamID != identity.TeamID || reconcile.Certificate.TeamID != identity.TeamID {
		return nil, safeDistributionPlanningFailure("account_team_mismatch")
	}
	remoteExpiration, err := time.Parse(time.RFC3339, reconcile.Certificate.ExpirationDate)
	if err != nil || !remoteExpiration.Equal(identity.NotAfter) {
		return nil, safeDistributionPlanningFailure("account_certificate_expiration_mismatch")
	}
	if len(reconcile.Targets) == 0 {
		return nil, safeDistributionPlanningFailure("account_target_missing")
	}
	if err := validateArchiveAppIdentity(archive.App); err != nil {
		return nil, safeDistributionPlanningFailure("archive_identity_invalid")
	}
	if reconcile.Targets[0].BundleID != archive.App.BundleID {
		return nil, safeDistributionPlanningFailure("account_target_mismatch")
	}
	publishedTitle := strings.TrimSpace(config.Metadata.Title)
	if publishedTitle == "" {
		publishedTitle = archive.App.Title
	}
	blockers := safeDistributionBlockers(reconcile.Blockers)
	if len(reconcile.Targets) != 1 {
		blockers = []distributionBlocker{{Code: "embedded_targets_unsupported", Stage: distributionPlanStagePreflight, Message: "v1 distribution supports exactly one main application target"}}
	}
	ready := reconcile.Ready && len(blockers) == 0 && len(reconcile.Targets) == 1
	reconcilePlanPath, err := canonicalAbsolutePath(reconcile.PlanPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("reconcile_plan_path_invalid")
	}
	reconcileReceiptPath, err := canonicalAbsolutePath(reconcile.ReceiptPath)
	if err != nil {
		return nil, safeDistributionPlanningFailure("reconcile_receipt_path_invalid")
	}
	plan := persistedDistributionPlan{
		SchemaVersion: distributionStateSchemaVersion, PlanID: planID, CreatedAt: now.Format(time.RFC3339), Ready: ready,
		ConfigPath: configPath, ConfigSHA256: strings.ToLower(configSHA),
		Archive: distributionArchiveBinding{
			Path: archivePath, TreeSHA256: archive.TreeSHA256, SizeBytes: archive.SizeBytes, FileCount: archive.EntryCount,
			BundleID: archive.App.BundleID, Title: archive.App.Title, PublishedTitle: publishedTitle, Version: archive.App.Version,
			BuildNumber: archive.App.BuildNumber, MinimumOSVersion: archive.App.MinimumOSVersion,
			TeamID: identity.TeamID, TargetCount: len(reconcile.Targets),
		},
		DeviceSet: distributionDeviceSetBinding{SHA256: reconcile.DeviceSetSHA256, FileSHA256: deviceFileSHA, Count: reconcile.DeviceCount},
		Identity: distributionIdentityBinding{
			CertificateResourceID: reconcile.Certificate.ResourceID, CertificateSHA256: identity.CertificateSHA256,
			TeamID: identity.TeamID, ExpirationDate: identity.NotAfter.UTC().Format(time.RFC3339), MinimumValidUntil: minimumValidUntil.Format(time.RFC3339),
		},
		Publication: config.Publication,
		Reconcile: distributionReconcileBinding{
			PlanPath: reconcilePlanPath, PlanHash: reconcile.PlanHash,
			ReceiptPath: reconcileReceiptPath, MinimumValidityDays: effectiveMinimumValidityDays,
			MutationCount: reconcile.MutationCount, MaxMutations: reconcile.MaxMutations,
		},
		Effects: distributionEffects(reconcile), Blockers: blockers, Paths: distributionPlanPaths{StateDir: stateDir},
	}
	if err := sealDistributionPlan(&plan); err != nil {
		return nil, safeDistributionPlanningFailure("plan_seal_failed")
	}
	if plan.Ready {
		if err := validateDistributionAuthorizedReconcilePlan(plan); err != nil {
			return nil, safeDistributionPlanningFailure("plan_authorization_mismatch")
		}
	}
	if err := distributionOrchestrationDeps.writePlan(planPath, plan); err != nil {
		return nil, safeDistributionPlanningFailure("plan_write_failed")
	}
	return &plan, nil
}

func executeDistributionApply(ctx context.Context, d *shared.Deps, request distributionApplyRequest) (*distributionRunState, error) {
	planPath, err := canonicalAbsolutePath(request.PlanPath)
	if err != nil {
		return nil, fmt.Errorf("plan path: %w", err)
	}
	plan, err := distributionOrchestrationDeps.readPlan(planPath)
	if err != nil {
		return nil, fmt.Errorf("read distribution plan: %w", err)
	}
	if request.Confirmation != plan.PlanHash {
		return nil, d.UsageError("--confirm must be the exact planHash")
	}
	if !plan.Ready {
		return nil, fmt.Errorf("distribution plan is blocked")
	}
	if err := validateDistributionAuthorizedReconcilePlan(plan); err != nil {
		return nil, fmt.Errorf("distribution plan effect authorization is invalid: %w", err)
	}
	runID := deterministicDistributionRunID(plan)
	if err := distributionOrchestrationDeps.createRun(plan.Paths.StateDir, runID); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create distribution run: %w", err)
		}
	}
	if distributionOrchestrationDeps.acquireLease != nil {
		verifyLease, closeLease, leaseErr := distributionOrchestrationDeps.acquireLease(ctx, d, plan.Paths.StateDir, runID)
		if leaseErr != nil {
			return nil, fmt.Errorf("lock and pin distribution run: %w", leaseErr)
		}
		defer func() { _ = closeLease() }()
		if err := verifyLease(); err != nil {
			return nil, safeDistributionFailure("preflight", "run_path_changed", err)
		}
		ctx = context.WithValue(ctx, distributionRunLeaseContextKey{}, verifyLease)
	} else {
		release, err := distributionOrchestrationDeps.acquireLock(ctx, d, plan.Paths.StateDir, runID)
		if err != nil {
			return nil, fmt.Errorf("lock distribution run: %w", err)
		}
		defer func() { _ = release() }()
	}
	if distributionOrchestrationDeps.ensureRun != nil {
		if err := distributionOrchestrationDeps.ensureRun(plan.Paths.StateDir, runID); err != nil {
			return nil, fmt.Errorf("validate distribution run scaffold: %w", err)
		}
	}
	state, err := distributionOrchestrationDeps.readRun(plan.Paths.StateDir, runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state = newDistributionRunState(plan, planPath, runID, d.Now().UTC())
			return continueDistributionRun(ctx, d, plan, state)
		}
		return nil, fmt.Errorf("read existing distribution run: %w", err)
	}
	if state.PlanID != plan.PlanID || state.PlanHash != plan.PlanHash || state.PlanPath != planPath {
		return nil, fmt.Errorf("existing distribution run does not match the confirmed plan")
	}
	if state.Status == "complete" {
		if err := verifyDistributionRunPathLease(ctx); err != nil {
			return nil, safeDistributionFailure("complete", "run_path_changed", err)
		}
		if _, err := loadExactDistributionCompletion(plan, state); err != nil {
			return nil, err
		}
		if err := verifyDistributionRunPathLease(ctx); err != nil {
			return nil, safeDistributionFailure("complete", "run_path_changed", err)
		}
		return &state, nil
	}
	if state.Status == "blocked" {
		return &state, fmt.Errorf("distribution run is blocked; create and confirm a new plan")
	}
	return continueDistributionRun(ctx, d, plan, state)
}

func executeDistributionResume(ctx context.Context, d *shared.Deps, request distributionRunRequest) (*distributionRunState, error) {
	stateDir, err := canonicalAbsolutePath(request.StateDir)
	if err != nil {
		return nil, fmt.Errorf("state directory: %w", err)
	}
	if distributionOrchestrationDeps.acquireLease != nil {
		verifyLease, closeLease, leaseErr := distributionOrchestrationDeps.acquireLease(ctx, d, stateDir, request.RunID)
		if leaseErr != nil {
			return nil, fmt.Errorf("lock and pin distribution run: %w", leaseErr)
		}
		defer func() { _ = closeLease() }()
		if err := verifyLease(); err != nil {
			return nil, safeDistributionFailure("preflight", "run_path_changed", err)
		}
		ctx = context.WithValue(ctx, distributionRunLeaseContextKey{}, verifyLease)
	} else {
		release, err := distributionOrchestrationDeps.acquireLock(ctx, d, stateDir, request.RunID)
		if err != nil {
			return nil, fmt.Errorf("lock distribution run: %w", err)
		}
		defer func() { _ = release() }()
	}
	state, err := distributionOrchestrationDeps.readRun(stateDir, request.RunID)
	if err != nil {
		return nil, fmt.Errorf("read distribution run: %w", err)
	}
	plan, err := distributionOrchestrationDeps.readPlan(state.PlanPath)
	if err != nil {
		return nil, fmt.Errorf("read distribution plan: %w", err)
	}
	if err := validateDistributionRunPlanBinding(stateDir, state, plan); err != nil {
		return nil, fmt.Errorf("distribution run does not match its plan")
	}
	if err := validateDistributionAuthorizedReconcilePlan(plan); err != nil {
		return nil, fmt.Errorf("distribution plan effect authorization is invalid: %w", err)
	}
	if state.Status == "complete" {
		if err := verifyDistributionRunPathLease(ctx); err != nil {
			return nil, safeDistributionFailure("complete", "run_path_changed", err)
		}
		if _, err := loadExactDistributionCompletion(plan, state); err != nil {
			return nil, err
		}
		if err := verifyDistributionRunPathLease(ctx); err != nil {
			return nil, safeDistributionFailure("complete", "run_path_changed", err)
		}
		return &state, nil
	}
	if state.Status == "blocked" {
		return &state, fmt.Errorf("distribution run is blocked; create and confirm a new plan")
	}
	return continueDistributionRun(ctx, d, plan, state)
}

func validateDistributionAuthorizedReconcilePlan(plan persistedDistributionPlan) error {
	if err := validateDistributionEffectInventory(plan); err != nil {
		return err
	}
	if distributionOrchestrationDeps.readReconcilePlan == nil {
		return nil
	}
	nested, err := distributionOrchestrationDeps.readReconcilePlan(signing.ReconcileApplyOptions{PlanPath: plan.Reconcile.PlanPath, ExpectedPlanHash: plan.Reconcile.PlanHash})
	if err != nil {
		return fmt.Errorf("read protected reconcile plan: %w", err)
	}
	config, configSHA, err := distributionOrchestrationDeps.readConfig(plan.ConfigPath)
	if err != nil || !strings.EqualFold(configSHA, plan.ConfigSHA256) {
		return fmt.Errorf("read exact distribution config")
	}
	if err := validateDistributionPlanConfigBinding(plan, config); err != nil {
		return err
	}
	devicesPath, err := resolveDistributionConfigPath(plan.ConfigPath, config.DevicesFile)
	if err != nil {
		return fmt.Errorf("resolve protected devices path: %w", err)
	}
	nestedStateDir := filepath.Dir(plan.Reconcile.PlanPath)
	certificateMatches := nested.Certificate != nil && nested.Certificate.ResourceID == plan.Identity.CertificateResourceID && strings.EqualFold(nested.Certificate.SHA256, plan.Identity.CertificateSHA256) && nested.Certificate.TeamID == plan.Identity.TeamID && nested.Certificate.ExpirationDate == plan.Identity.ExpirationDate
	pathsMatch := nested.ArchivePath == plan.Archive.Path && nested.DevicesFile == devicesPath && nested.StateDir == nestedStateDir && nested.PlanPath == plan.Reconcile.PlanPath && nested.ReceiptPath == filepath.Join(nestedStateDir, "receipt.json") && nested.ReceiptPath == plan.Reconcile.ReceiptPath && nested.ProfilesDir == filepath.Join(nestedStateDir, "profiles")
	effectiveMinimumValidityDays, validityErr := effectiveDistributionMinimumValidityDays(config.Signing.MinimumValidityDays, config.Publication.URLTTLDuration, config.Publication.DownloadGraceDuration)
	if !nested.Ready || nested.PlanHash != plan.Reconcile.PlanHash || !pathsMatch || validityErr != nil || plan.Reconcile.MinimumValidityDays != effectiveMinimumValidityDays || nested.MinimumValidityDays != plan.Reconcile.MinimumValidityDays || nested.MutationCount != plan.Reconcile.MutationCount || nested.MaxMutations != plan.Reconcile.MaxMutations || nested.DeviceSetSHA256 != plan.DeviceSet.SHA256 || nested.DeviceCount != plan.DeviceSet.Count || nested.TeamID != plan.Identity.TeamID || !certificateMatches || len(nested.Targets) != 1 || nested.Targets[0].Kind != "application" || nested.Targets[0].BundleID != plan.Archive.BundleID || !reflect.DeepEqual(distributionEffects(nested), plan.Effects) {
		return fmt.Errorf("protected reconcile plan does not match the authorized distribution effects")
	}
	return nil
}

func validateDistributionPlanConfigBinding(plan persistedDistributionPlan, config distributionConfig) error {
	if plan.Reconcile.MaxMutations != config.Signing.MaxMutations || !distributionPublicationPolicyEqual(plan.Publication, config.Publication) {
		return fmt.Errorf("distribution plan does not match its protected configuration policy")
	}
	publishedTitle := strings.TrimSpace(config.Metadata.Title)
	if publishedTitle == "" {
		publishedTitle = plan.Archive.Title
	}
	configuredCertificate := strings.ToLower(strings.TrimSpace(config.Signing.Identity.CertificateSHA256))
	if plan.Archive.PublishedTitle != publishedTitle || (configuredCertificate != "" && !strings.EqualFold(configuredCertificate, plan.Identity.CertificateSHA256)) {
		return fmt.Errorf("distribution plan does not match its protected configuration identity")
	}
	return nil
}

func distributionPublicationPolicyEqual(plan, config distributionPublicationConfig) bool {
	// The parsed durations are runtime caches and are deliberately excluded from
	// JSON. Bind the exact serialized policy fields so a persisted plan loaded
	// from disk compares identically to the protected configuration that produced
	// it.
	return plan.Endpoint == config.Endpoint &&
		plan.DownloadEndpoint == config.DownloadEndpoint &&
		plan.Region == config.Region &&
		plan.Bucket == config.Bucket &&
		plan.Prefix == config.Prefix &&
		plan.AddressingStyle == config.AddressingStyle &&
		plan.URLTTL == config.URLTTL &&
		plan.DownloadGrace == config.DownloadGrace &&
		plan.VerifyTimeout == config.VerifyTimeout
}

func executeDistributionStatus(_ context.Context, _ *shared.Deps, request distributionRunRequest) (*distributionRunState, error) {
	stateDir, err := canonicalAbsolutePath(request.StateDir)
	if err != nil {
		return nil, fmt.Errorf("state directory: %w", err)
	}
	state, err := distributionOrchestrationDeps.readRun(stateDir, request.RunID)
	if err != nil {
		return nil, fmt.Errorf("read distribution run: %w", err)
	}
	plan, err := distributionOrchestrationDeps.readPlan(state.PlanPath)
	if err != nil {
		return nil, fmt.Errorf("read distribution plan: %w", err)
	}
	if err := validateDistributionRunPlanBinding(stateDir, state, plan); err != nil {
		return nil, err
	}
	if state.Status == "complete" {
		if _, err := loadExactDistributionCompletion(plan, state); err != nil {
			return nil, err
		}
	}
	return &state, nil
}

func executeDistributionVerify(ctx context.Context, d *shared.Deps, request distributionVerifyRequest) (*distributionVerificationResult, error) {
	stateDir, err := canonicalAbsolutePath(request.StateDir)
	if err != nil {
		return nil, err
	}
	acquireLease := distributionOrchestrationDeps.acquireVerifyLease
	if acquireLease == nil {
		return nil, safeDistributionFailure("verification", "run_path_unavailable", nil)
	}
	lease, err := acquireLease(stateDir, request.RunID)
	if err != nil {
		return nil, safeDistributionFailure("verification", "run_path_unavailable", err)
	}
	defer func() { _ = lease.Close() }()
	guard := func() error {
		if err := lease.Verify(); err != nil {
			return safeDistributionFailure("verification", "run_path_changed", err)
		}
		return nil
	}
	if err := guard(); err != nil {
		return nil, err
	}
	stateValue, err := distributionOrchestrationDeps.readRun(stateDir, request.RunID)
	if err != nil {
		return nil, fmt.Errorf("read distribution run: %w", err)
	}
	if err := guard(); err != nil {
		return nil, err
	}
	state := &stateValue
	if state.Status != "complete" {
		return nil, fmt.Errorf("distribution run is not complete")
	}
	if err := guard(); err != nil {
		return nil, err
	}
	plan, err := distributionOrchestrationDeps.readPlan(state.PlanPath)
	if err != nil {
		return nil, fmt.Errorf("read distribution plan: %w", err)
	}
	if err := guard(); err != nil {
		return nil, err
	}
	if err := validateDistributionRunPlanBinding(stateDir, *state, plan); err != nil {
		return nil, err
	}
	if err := lease.PinSubtrees(distributionVerificationSubtrees(*state)...); err != nil {
		return nil, safeDistributionFailure("verification", "run_subtree_changed", err)
	}
	if err := guard(); err != nil {
		return nil, err
	}
	receipt, err := loadExactDistributionCompletion(plan, *state)
	if err != nil {
		return nil, err
	}
	if err := guard(); err != nil {
		return nil, err
	}
	runDir := filepath.Join(plan.Paths.StateDir, state.RunID)
	if err := guard(); err != nil {
		return nil, err
	}
	localIPA, err := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, state.Artifacts.IPA.Path))
	if err != nil || localIPA.SHA256 != state.Artifacts.IPA.SHA256 || localIPA.SizeBytes != state.Artifacts.IPA.SizeBytes {
		return nil, safeDistributionFailure("verification", "local_ipa_changed", err)
	}
	if err := guard(); err != nil {
		return nil, err
	}
	totalTimeout := request.Timeout
	if totalTimeout <= 0 {
		totalTimeout = plan.Publication.VerifyTimeoutDuration
		if totalTimeout <= 0 {
			totalTimeout, _ = time.ParseDuration(plan.Publication.VerifyTimeout)
		}
	}
	verifyCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()
	verifyTimeout := plan.Publication.VerifyTimeoutDuration
	if verifyTimeout <= 0 {
		verifyTimeout, _ = time.ParseDuration(plan.Publication.VerifyTimeout)
	}
	if remaining := remainingDistributionTimeout(verifyCtx, totalTimeout); verifyTimeout <= 0 || remaining < verifyTimeout {
		verifyTimeout = remaining
	}
	if err := guard(); err != nil {
		return nil, err
	}
	published, err := distributionOrchestrationDeps.reverifyPublish(verifyCtx, d, privatePublishVerificationRequest{
		BundleDir:   filepath.Join(runDir, state.Artifacts.Bundle.Path),
		ReceiptPath: filepath.Join(runDir, state.Artifacts.Publication.ReceiptPath),
		LinkPath:    filepath.Join(runDir, state.Artifacts.Publication.LinkPath), VerifyTimeout: verifyTimeout,
	})
	if err != nil {
		return nil, safeDistributionFailure("verification", "publication_verification_failed", err)
	}
	if err := guard(); err != nil {
		return nil, err
	}
	if !publishMatchesCompletion(published.Receipt, receipt, plan, *state) {
		return nil, fmt.Errorf("publication verification does not match the immutable completion receipt")
	}
	result := &distributionVerificationResult{
		SchemaVersion: distributionStateSchemaVersion, RunID: state.RunID, PlanHash: state.PlanHash,
		PublicationVerified: true, VerifiedAt: d.Now().UTC().Format(time.RFC3339),
		ArtifactSHA256: receipt.ArtifactSHA256, AppBundleID: receipt.AppBundleID, AppVersion: receipt.AppVersion, AppBuildNumber: receipt.AppBuildNumber,
	}
	if strings.TrimSpace(request.Device) != "" {
		remaining := remainingDistributionTimeout(verifyCtx, totalTimeout)
		if remaining < deviceObservationMinimumTimeout {
			timeoutErr := verifyCtx.Err()
			if timeoutErr == nil {
				timeoutErr = context.DeadlineExceeded
			}
			return nil, safeDistributionFailure("device verification", "verification_timeout", timeoutErr)
		}
		if remaining > deviceObservationMaximumTimeout {
			remaining = deviceObservationMaximumTimeout
		}
		observation, err := distributionOrchestrationDeps.observeDevice(verifyCtx, distributionDeviceObservationRequest{
			Device: request.Device, BundleID: receipt.AppBundleID, Version: receipt.AppVersion, Build: receipt.AppBuildNumber, Timeout: remaining,
		})
		if err != nil {
			return nil, safeDistributionFailure("device verification", "device_observation_failed", err)
		}
		if !deviceObservationMatchesCompletion(observation, receipt) {
			return nil, safeDistributionFailure("device verification", "device_observation_mismatch", nil)
		}
		if err := guard(); err != nil {
			return nil, err
		}
		result.DeviceObservation = &observation
	}
	if err := guard(); err != nil {
		return nil, err
	}
	return result, nil
}

func deviceObservationMatchesCompletion(observation deviceObservation, receipt persistedDistributionReceipt) bool {
	return observation.Requested && observation.DeviceFound && observation.AppInstalled &&
		observation.BundleID == receipt.AppBundleID && observation.Version == receipt.AppVersion && observation.Build == receipt.AppBuildNumber
}

func distributionVerificationSubtrees(state persistedDistributionRunState) []string {
	var subtrees []string
	appendParent := func(relative string) {
		if parent := path.Dir(relative); parent != "." && parent != "" {
			subtrees = append(subtrees, parent)
		}
	}
	if snapshot := state.Artifacts.ArchiveSnapshot; snapshot != nil {
		subtrees = append(subtrees, snapshot.RelativePath)
	}
	if receipt := state.Artifacts.ReconcileReceipt; receipt != nil {
		appendParent(receipt.Path)
	}
	if receipt := state.Artifacts.SigningReceipt; receipt != nil {
		appendParent(receipt.Path)
	}
	if options := state.Artifacts.ExportOptions; options != nil {
		appendParent(options.Path)
	}
	if profile := state.Artifacts.Profile; profile != nil {
		appendParent(profile.Path)
	}
	if ipa := state.Artifacts.IPA; ipa != nil {
		appendParent(ipa.Path)
	}
	if bundle := state.Artifacts.Bundle; bundle != nil {
		subtrees = append(subtrees, bundle.Path)
	}
	if publication := state.Artifacts.Publication; publication != nil {
		appendParent(publication.ReceiptPath)
		appendParent(publication.LinkPath)
	}
	return subtrees
}

func continueDistributionRun(ctx context.Context, d *shared.Deps, plan persistedDistributionPlan, state persistedDistributionRunState) (*distributionRunState, error) {
	state.Attempt++
	state.Status, state.Recoverable, state.LastFailureCode = "running", false, ""
	guard := func(stage string) error {
		if err := verifyDistributionRunPathLease(ctx); err != nil {
			return safeDistributionFailure(stage, "run_path_changed", err)
		}
		return nil
	}
	if err := guard("preflight"); err != nil {
		return nil, err
	}
	if err := distributionOrchestrationDeps.recoverEphemeral(ctx); err != nil {
		terminal := errors.Is(err, signing.ErrEphemeralRecoveryJournalInvalid) || shared.IsValidationError(err)
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "signing_recovery_failed", !terminal, err)
	}
	config, configSHA, err := distributionOrchestrationDeps.readConfig(plan.ConfigPath)
	if err != nil || configSHA != plan.ConfigSHA256 {
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "config_changed", false, err)
	}
	devicesPath, err := resolveDistributionConfigPath(plan.ConfigPath, config.DevicesFile)
	if err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "devices_path_invalid", false, err)
	}
	identityPath, err := resolveDistributionConfigPath(plan.ConfigPath, config.Signing.Identity.Path)
	if err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "identity_path_invalid", false, err)
	}
	passwordPath, err := resolveOptionalDistributionConfigPath(plan.ConfigPath, config.Signing.Identity.PasswordFile)
	if err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "password_path_invalid", false, err)
	}
	identity, err := distributionOrchestrationDeps.inspectIdentity(ctx, signing.PKCS12IdentityOptions{IdentityPath: identityPath, IdentityPasswordPath: passwordPath})
	plannedExpiration, expirationErr := time.Parse(time.RFC3339, plan.Identity.ExpirationDate)
	minimumValidUntil, minimumErr := time.Parse(time.RFC3339, plan.Identity.MinimumValidUntil)
	if err != nil || expirationErr != nil || minimumErr != nil ||
		!strings.EqualFold(identity.CertificateSHA256, plan.Identity.CertificateSHA256) || identity.TeamID != plan.Identity.TeamID ||
		!identity.NotAfter.Equal(plannedExpiration) || !identity.NotAfter.After(minimumValidUntil) {
		return checkpointDistributionFailure(ctx, d, plan, state, "identity_validate", "identity_changed", false, err)
	}
	if state.Artifacts.ArchiveSnapshot == nil {
		archive, archiveErr := distributionOrchestrationDeps.digestArchive(ctx, plan.Archive.Path)
		if archiveErr != nil || !archiveSnapshotMatchesPlan(archive, plan.Archive) {
			return checkpointDistributionFailure(ctx, d, plan, state, "identity_validate", "archive_changed", false, archiveErr)
		}
	}
	if state.Artifacts.Publication == nil {
		if err := distributionOrchestrationDeps.preflightPublish(ctx, d, plan.Publication); err != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "storage_preflight_failed", true, err)
		}
	}
	if err := guard("identity_validate"); err != nil {
		return nil, err
	}
	if state.Artifacts.ArchiveSnapshot == nil {
		snapshot, err := distributionOrchestrationDeps.snapshotArchive(ctx, plan.Archive.Path, plan.Paths.StateDir, state.RunID)
		if err != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "identity_validate", "archive_snapshot_failed", true, err)
		}
		if !archiveSnapshotMatchesPlan(snapshot, plan.Archive) {
			return checkpointDistributionFailure(ctx, d, plan, state, "identity_validate", "archive_snapshot_mismatch", false, nil)
		}
		state.Artifacts.ArchiveSnapshot = &distributionArchiveSnapshot{RelativePath: snapshot.RelativePath, TreeSHA256: snapshot.TreeSHA256, SizeBytes: snapshot.SizeBytes, EntryCount: snapshot.EntryCount, App: snapshot.App}
	} else if err := distributionOrchestrationDeps.revalidateArchive(ctx, plan.Paths.StateDir, state.RunID, *state.Artifacts.ArchiveSnapshot); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "identity_validate", "archive_snapshot_changed", false, err)
	}
	deviceFileSHA, err := distributionOrchestrationDeps.hashProtectedFile(devicesPath)
	if err != nil || deviceFileSHA != plan.DeviceSet.FileSHA256 {
		return checkpointDistributionFailure(ctx, d, plan, state, "preflight", "devices_changed", false, err)
	}
	if err := checkpointDistributionState(ctx, d, plan, &state, "account_reconcile"); err != nil {
		return checkpointDistributionWriteFailure(plan, state, err)
	}
	reconcileOptions := signing.ReconcileApplyOptions{PlanPath: plan.Reconcile.PlanPath, ExpectedPlanHash: plan.Reconcile.PlanHash, Confirm: true}
	var reconcile signing.ReconcileReceiptView
	if state.Artifacts.ReconcileReceipt == nil {
		if err := guard("account_reconcile"); err != nil {
			return nil, err
		}
		reconcile, err = distributionOrchestrationDeps.reconcileApply(ctx, reconcileOptions)
		if guardErr := guard("account_reconcile"); guardErr != nil {
			return nil, guardErr
		}
		if err != nil {
			recoverable := signing.ClassifyReconcileExecutionError(err) == signing.ReconcileExecutionErrorRetryable
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "account_reconcile_failed", recoverable, err)
		}
	} else {
		archiveSnapshotPath := filepath.Join(plan.Paths.StateDir, state.RunID, state.Artifacts.ArchiveSnapshot.RelativePath)
		runRoot := filepath.Join(plan.Paths.StateDir, state.RunID)
		evidence := signing.ReconcileCompletionEvidence{
			ReceiptPath: filepath.Join(runRoot, state.Artifacts.ReconcileReceipt.Path), ReceiptSHA256: state.Artifacts.ReconcileReceipt.SHA256,
		}
		if state.Artifacts.Profile != nil {
			evidence.Profiles = []signing.ReconcileProfileEvidence{{
				ResourceID: state.Artifacts.Profile.ResourceID, Path: filepath.Join(runRoot, state.Artifacts.Profile.Path), SHA256: state.Artifacts.Profile.SHA256,
			}}
		}
		reconcile, err = distributionOrchestrationDeps.verifyReconcile(ctx, reconcileOptions, archiveSnapshotPath, evidence)
		if err != nil {
			if signing.ClassifyReconcileExecutionError(err) == signing.ReconcileExecutionErrorRetryable {
				return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "account_verification_failed", true, err)
			}
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "account_state_changed", false, err)
		}
	}
	if !reconcile.Complete || reconcile.MainProfile == nil || reconcile.MainProfile.BundleID != plan.Archive.BundleID {
		return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "profile_binding_failed", false, nil)
	}
	profile := reconcile.MainProfile
	if state.Artifacts.ReconcileReceipt != nil && state.Artifacts.Profile != nil {
		if err := distributionOrchestrationDeps.validateReconcileEvidence(plan.Paths.StateDir, plan, state, reconcile, *state.Artifacts.ReconcileReceipt, *state.Artifacts.Profile); err != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "reconcile_evidence_changed", false, err)
		}
	}
	reconcileReceiptArtifact := state.Artifacts.ReconcileReceipt
	profileArtifact := state.Artifacts.Profile
	if reconcileReceiptArtifact == nil {
		artifact, copyErr := distributionOrchestrationDeps.copyArtifact(reconcile.ReceiptPath, plan.Paths.StateDir, state.RunID, distributionReconcileRelative, distributionStateMaxBytes)
		if copyErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "reconcile_receipt_copy_failed", !errors.Is(copyErr, errDistributionArtifactConflict), copyErr)
		}
		reconcileReceiptArtifact = &distributionFileArtifact{Path: artifact.Path, SHA256: artifact.SHA256}
	}
	if profileArtifact == nil {
		artifact, copyErr := distributionOrchestrationDeps.copyArtifact(profile.Path, plan.Paths.StateDir, state.RunID, distributionProfileRelative, 16<<20)
		if copyErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "profile_copy_failed", !errors.Is(copyErr, errDistributionArtifactConflict), copyErr)
		}
		if artifact.SHA256 != profile.SHA256 {
			return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "profile_copy_failed", false, nil)
		}
		profileArtifact = &distributionProfileArtifact{ResourceID: profile.ResourceID, UUID: profile.UUID, Path: artifact.Path, SHA256: artifact.SHA256, BundleID: profile.BundleID}
	}
	// Adopt the reconcile receipt and its selected profile as one state-level
	// unit. If either copy fails, a retry can re-run exact reconciliation and
	// safely adopt any identical unreferenced file left in the run directory.
	state.Artifacts.ReconcileReceipt = reconcileReceiptArtifact
	state.Artifacts.Profile = profileArtifact
	if err := distributionOrchestrationDeps.validateReconcileEvidence(plan.Paths.StateDir, plan, state, reconcile, *state.Artifacts.ReconcileReceipt, *state.Artifacts.Profile); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "account_reconcile", "reconcile_evidence_invalid", false, err)
	}
	if err := checkpointDistributionState(ctx, d, plan, &state, "export"); err != nil {
		return checkpointDistributionWriteFailure(plan, state, err)
	}
	runDir := filepath.Join(plan.Paths.StateDir, state.RunID)
	expectedOptions := localxcode.ManualReleaseTestingExportOptions{
		OutputPath: filepath.Join(runDir, distributionExportOptionsRel), TeamID: plan.Identity.TeamID,
		SigningCertificate: identity.CertificateSHA1, ProvisioningProfiles: map[string]string{plan.Archive.BundleID: profile.UUID},
	}
	if state.Artifacts.ExportOptions == nil {
		if err := guard("export"); err != nil {
			return nil, err
		}
		exportOptionsPath := filepath.Join(runDir, distributionExportOptionsRel)
		exists, existsErr := orchestrationPathExists(exportOptionsPath)
		if existsErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "export", "export_options_recovery_failed", false, existsErr)
		}
		if exists {
			result, validateErr := distributionOrchestrationDeps.validateExportOptions(ctx, expectedOptions)
			if validateErr != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "export_options_recovery_failed", false, validateErr)
			}
			state.Artifacts.ExportOptions = &distributionFileArtifact{Path: distributionExportOptionsRel, SHA256: result.SHA256}
		} else {
			result, err := distributionOrchestrationDeps.writeExportOptions(ctx, expectedOptions)
			if err != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "export_options_failed", true, err)
			}
			state.Artifacts.ExportOptions = &distributionFileArtifact{Path: distributionExportOptionsRel, SHA256: result.SHA256}
		}
	}
	validatedOptions, validateOptionsErr := distributionOrchestrationDeps.validateExportOptions(ctx, expectedOptions)
	if validateOptionsErr != nil || !strings.EqualFold(validatedOptions.SHA256, state.Artifacts.ExportOptions.SHA256) {
		return checkpointDistributionFailure(ctx, d, plan, state, "export", "export_options_changed", false, validateOptionsErr)
	}
	if state.Artifacts.IPA == nil || state.Artifacts.SigningReceipt == nil {
		if err := guard("export"); err != nil {
			return nil, err
		}
		currentIPARel := fmt.Sprintf("export/App-%06d.ipa", state.Attempt)
		ipaPath := filepath.Join(runDir, currentIPARel)
		currentSigningReceiptRel := fmt.Sprintf("signing/receipt-%06d.json", state.Attempt)
		{
			signingReceiptPath := filepath.Join(runDir, currentSigningReceiptRel)
			var exportResult *localxcode.ExportResult
			err := distributionOrchestrationDeps.runSigning(ctx, signing.EphemeralRunOptions{
				IdentityPath: identityPath, IdentityPasswordPath: passwordPath,
				ProfilePath: filepath.Join(runDir, state.Artifacts.Profile.Path), ReceiptPath: signingReceiptPath,
				ExpectedCertificateSHA256: plan.Identity.CertificateSHA256, ExpectedProfileSHA256: state.Artifacts.Profile.SHA256,
			}, func(runCtx context.Context) error {
				var callbackErr error
				exportResult, callbackErr = distributionOrchestrationDeps.exportIPA(runCtx, localxcode.ReleaseTestingExportOptions{
					ArchivePath:       filepath.Join(runDir, state.Artifacts.ArchiveSnapshot.RelativePath),
					ExportOptionsPath: filepath.Join(runDir, state.Artifacts.ExportOptions.Path), ExportOptionsSHA256: state.Artifacts.ExportOptions.SHA256,
					IPAPath: ipaPath, Environment: signing.SanitizedChildEnvironment(os.Environ()),
				})
				return callbackErr
			})
			if err != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "xcode_export_failed", true, err)
			}
			if exportResult == nil || exportResult.BundleID != plan.Archive.BundleID {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "export_identity_mismatch", false, nil)
			}
			ipa, hashErr := distributionOrchestrationDeps.hashFile(exportResult.IPAPath)
			if hashErr != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "ipa_hash_failed", true, hashErr)
			}
			ipa.Path = currentIPARel
			state.Artifacts.IPA = &ipa
			signingReceipt, copyErr := distributionOrchestrationDeps.copyArtifact(signingReceiptPath, plan.Paths.StateDir, state.RunID, currentSigningReceiptRel, distributionStateMaxBytes)
			if copyErr != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "signing_receipt_hash_failed", true, copyErr)
			}
			candidate := distributionFileArtifact{Path: currentSigningReceiptRel, SHA256: signingReceipt.SHA256}
			if err := distributionOrchestrationDeps.validateSigningEvidence(plan.Paths.StateDir, plan, state, candidate); err != nil {
				return checkpointDistributionFailure(ctx, d, plan, state, "export", "signing_evidence_invalid", false, err)
			}
			state.Artifacts.SigningReceipt = &candidate
		}
		if err := guard("export"); err != nil {
			return nil, err
		}
	}
	if err := distributionOrchestrationDeps.validateSigningEvidence(plan.Paths.StateDir, plan, state, *state.Artifacts.SigningReceipt); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "export", "signing_evidence_changed", false, err)
	}
	currentIPA, currentIPAErr := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, state.Artifacts.IPA.Path))
	if currentIPAErr != nil || currentIPA.SHA256 != state.Artifacts.IPA.SHA256 || currentIPA.SizeBytes != state.Artifacts.IPA.SizeBytes {
		return checkpointDistributionFailure(ctx, d, plan, state, "export", "ipa_changed", false, currentIPAErr)
	}
	if err := checkpointDistributionState(ctx, d, plan, &state, "prepare"); err != nil {
		return checkpointDistributionWriteFailure(plan, state, err)
	}
	var prepared core.PrepareResult
	if state.Artifacts.Bundle == nil {
		if err := guard("prepare"); err != nil {
			return nil, err
		}
		prepared, err = distributionOrchestrationDeps.prepareIPA(ctx, distributionPrepareRequest{
			StateDir: plan.Paths.StateDir, RunID: state.RunID, IPARelativePath: state.Artifacts.IPA.Path,
			ExpectedSHA256: state.Artifacts.IPA.SHA256, ExpectedSize: state.Artifacts.IPA.SizeBytes, Metadata: config.Metadata,
		})
		if err != nil {
			terminal := errors.Is(err, core.ErrBundleConflict) || errors.Is(err, core.ErrIPAIdentityMismatch) || errors.Is(err, core.ErrNotEligible)
			return checkpointDistributionFailure(ctx, d, plan, state, "prepare", "prepare_failed", !terminal, err)
		}
		if err := validatePreparedDistributionBinding(plan, state, prepared.Descriptor); err != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "prepare", "prepared_profile_or_identity_mismatch", false, err)
		}
		descriptor, hashErr := distributionOrchestrationDeps.hashFile(filepath.Join(prepared.BundlePath, "bundle.json"))
		if hashErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "prepare", "descriptor_hash_failed", true, hashErr)
		}
		state.Artifacts.Bundle = &distributionBundleArtifact{Path: distributionBundleRelative, DescriptorSHA256: descriptor.SHA256}
		if err := guard("prepare"); err != nil {
			return nil, err
		}
	}
	if err := distributionOrchestrationDeps.validatePreparedBundle(filepath.Join(runDir, state.Artifacts.Bundle.Path), plan, state); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "prepare", "prepared_bundle_changed", false, err)
	}
	authorizedDescriptor, descriptorErr := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, state.Artifacts.Bundle.Path, "bundle.json"))
	if descriptorErr != nil || authorizedDescriptor.SHA256 != state.Artifacts.Bundle.DescriptorSHA256 {
		return checkpointDistributionFailure(ctx, d, plan, state, "prepare", "prepared_bundle_changed", false, descriptorErr)
	}
	if err := checkpointDistributionState(ctx, d, plan, &state, "publish"); err != nil {
		return checkpointDistributionWriteFailure(plan, state, err)
	}
	var published publishExecutionResult
	if state.Artifacts.Publication == nil {
		if err := guard("publish"); err != nil {
			return nil, err
		}
		urlTTL, downloadGrace, verifyTimeout, durationErr := distributionPublicationDurations(plan.Publication)
		if durationErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "publish", "publication_policy_invalid", false, durationErr)
		}
		published, err = distributionOrchestrationDeps.publish(ctx, d, privatePublishIntentRequest{
			BundleDir: filepath.Join(runDir, state.Artifacts.Bundle.Path),
			ExpectedBundle: privatePublishBundleAuthorization{
				DescriptorSHA256: authorizedDescriptor.SHA256, DescriptorSize: authorizedDescriptor.SizeBytes,
				IPASHA256: state.Artifacts.IPA.SHA256, IPASize: state.Artifacts.IPA.SizeBytes,
				ProfileUUID: state.Artifacts.Profile.UUID, ProfileSHA256: state.Artifacts.Profile.SHA256,
				TeamID: plan.Identity.TeamID, DeviceSetSHA256: plan.DeviceSet.SHA256, DeviceCount: plan.DeviceSet.Count,
				CertificateSHA256: plan.Identity.CertificateSHA256,
			},
			Endpoint:         plan.Publication.Endpoint,
			DownloadEndpoint: plan.Publication.DownloadEndpoint, Region: plan.Publication.Region, Bucket: plan.Publication.Bucket,
			Prefix: plan.Publication.Prefix, AddressingStyle: plan.Publication.AddressingStyle,
			URLTTL: urlTTL, DownloadGrace: downloadGrace, VerifyTimeout: verifyTimeout,
			ReceiptPath: filepath.Join(runDir, distributionPublishReceiptRel), IntentPath: filepath.Join(runDir, distributionPublishIntentRel), DiagnosticWriter: io.Discard,
		})
		if guardErr := guard("publish"); guardErr != nil {
			return nil, guardErr
		}
		if err != nil {
			terminal := errors.Is(err, errPrivatePublishIntentConflict) || errors.Is(err, core.ErrPrivatePublishConflict) || errors.Is(err, core.ErrPrivatePublishLinkExpired) || errors.Is(err, core.ErrPrivatePublishProfileExpired)
			return checkpointDistributionFailure(ctx, d, plan, state, "publish", "provider_outcome_unknown", !terminal, err)
		}
		receiptFile, hashErr := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, distributionPublishReceiptRel))
		if hashErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "publish", "publication_receipt_hash_failed", true, hashErr)
		}
		intentFile, hashErr := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, distributionPublishIntentRel))
		if hashErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "publish", "publication_intent_hash_failed", true, hashErr)
		}
		state.Artifacts.Publication = &distributionPublicationArtifact{
			ReceiptPath: distributionPublishReceiptRel, ReceiptSHA256: receiptFile.SHA256,
			LinkPath: distributionPublishIntentRel, LinkSHA256: intentFile.SHA256,
			ArtifactKey: published.Receipt.Artifact.Key, ManifestKey: published.Receipt.Manifest.Key, PageKey: published.Receipt.Page.Key,
			InstallURLRedacted: published.Receipt.InstallURL,
		}
	}
	if err := checkpointDistributionState(ctx, d, plan, &state, "fetch_verify"); err != nil {
		return checkpointDistributionWriteFailure(plan, state, err)
	}
	if published.Receipt.SchemaVersion == "" {
		if err := guard("fetch_verify"); err != nil {
			return nil, err
		}
		_, _, verifyTimeout, durationErr := distributionPublicationDurations(plan.Publication)
		if durationErr != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "publication_policy_invalid", false, durationErr)
		}
		published, err = distributionOrchestrationDeps.reverifyPublish(ctx, d, privatePublishVerificationRequest{
			BundleDir: filepath.Join(runDir, state.Artifacts.Bundle.Path), ReceiptPath: filepath.Join(runDir, state.Artifacts.Publication.ReceiptPath),
			LinkPath: filepath.Join(runDir, state.Artifacts.Publication.LinkPath), VerifyTimeout: verifyTimeout,
		})
		if guardErr := guard("fetch_verify"); guardErr != nil {
			return nil, guardErr
		}
		if err != nil {
			code, recoverable := classifyDistributionPublicationReverificationFailure(err)
			return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", code, recoverable, err)
		}
	}
	if prepared.Descriptor.SchemaVersion == "" {
		// Final production binding is performed by the state layer against the
		// immutable descriptor. Tests inject the same facts through the receipt.
		prepared = validPreparedResultFromPublishForOrchestration(published.Receipt, state)
	}
	if err := guard("fetch_verify"); err != nil {
		return nil, err
	}
	descriptorFile, descriptorErr := distributionOrchestrationDeps.hashFile(filepath.Join(runDir, state.Artifacts.Bundle.Path, "bundle.json"))
	if descriptorErr != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "descriptor_recheck_failed", true, descriptorErr)
	}
	completion := newPersistedDistributionReceipt(plan, state, prepared.Descriptor, published.Receipt, descriptorFile.SizeBytes, d.Now().UTC())
	existing, readErr := distributionOrchestrationDeps.readReceipt(plan.Paths.StateDir, state.RunID)
	if readErr == nil {
		validate := distributionOrchestrationDeps.validateCompletion
		if validate == nil {
			validate = validateExactDistributionCompletion
		}
		if err := validate(plan, state, existing); err != nil {
			return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "completion_receipt_conflict", false, err)
		}
		completion = existing
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "completion_receipt_conflict", false, readErr)
	} else if err := distributionOrchestrationDeps.writeReceipt(plan.Paths.StateDir, completion); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "completion_receipt_failed", true, err)
	}
	validate := distributionOrchestrationDeps.validateCompletion
	if validate == nil {
		validate = validateExactDistributionCompletion
	}
	if err := validate(plan, state, completion); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "completion_evidence_changed", false, err)
	}
	state.Status, state.Stage, state.Recoverable, state.LastFailureCode = "complete", "complete", false, ""
	state.UpdatedAt = d.Now().UTC().Format(time.RFC3339)
	if err := distributionOrchestrationDeps.writeRun(plan.Paths.StateDir, state); err != nil {
		return checkpointDistributionFailure(ctx, d, plan, state, "fetch_verify", "completion_state_failed", true, err)
	}
	if err := guard("complete"); err != nil {
		return nil, err
	}
	return &state, nil
}

func classifyDistributionPublicationReverificationFailure(err error) (code string, recoverable bool) {
	if errors.Is(err, errPrivatePublishIntentConflict) || errors.Is(err, core.ErrPrivatePublishConflict) {
		return "publication_intent_conflict", false
	}
	if errors.Is(err, core.ErrPrivatePublishLinkExpired) || errors.Is(err, core.ErrPrivatePublishProfileExpired) {
		return "publication_expired", false
	}
	return "publication_verification_failed", true
}

func checkpointDistributionWriteFailure(plan persistedDistributionPlan, attempted persistedDistributionRunState, cause error) (*distributionRunState, error) {
	if errors.Is(cause, errDistributionRunPathChanged) {
		return nil, cause
	}
	durable, err := distributionOrchestrationDeps.readRun(plan.Paths.StateDir, attempted.RunID)
	if err != nil {
		return nil, cause
	}
	return &durable, cause
}

func newDistributionRunState(plan persistedDistributionPlan, planPath, runID string, now time.Time) persistedDistributionRunState {
	return persistedDistributionRunState{
		SchemaVersion: distributionStateSchemaVersion, RunID: runID, PlanID: plan.PlanID, PlanPath: planPath, PlanHash: plan.PlanHash,
		Status: "running", Stage: "preflight", UpdatedAt: now.UTC().Format(time.RFC3339), Attempt: 0,
	}
}

func checkpointDistributionState(ctx context.Context, d *shared.Deps, plan persistedDistributionPlan, state *persistedDistributionRunState, stage string) error {
	if err := verifyDistributionRunPathLease(ctx); err != nil {
		return fmt.Errorf("%w: %w", errDistributionRunPathChanged, safeDistributionFailure(stage, "run_path_changed", err))
	}
	stage = monotonicDistributionStage(state.Stage, stage)
	state.Status, state.Stage, state.Recoverable, state.LastFailureCode = "running", stage, false, ""
	state.UpdatedAt = d.Now().UTC().Format(time.RFC3339)
	if err := distributionOrchestrationDeps.writeRun(plan.Paths.StateDir, *state); err != nil {
		return safeDistributionFailure(stage, "checkpoint_failed", err)
	}
	if err := verifyDistributionRunPathLease(ctx); err != nil {
		return fmt.Errorf("%w: %w", errDistributionRunPathChanged, safeDistributionFailure(stage, "run_path_changed", err))
	}
	return nil
}

func checkpointDistributionFailure(ctx context.Context, d *shared.Deps, plan persistedDistributionPlan, state persistedDistributionRunState, stage, code string, recoverable bool, cause error) (*distributionRunState, error) {
	if err := verifyDistributionRunPathLease(ctx); err != nil {
		return nil, safeDistributionFailure(stage, "run_path_changed", err)
	}
	stage = monotonicDistributionStage(state.Stage, stage)
	state.Stage, state.Recoverable, state.LastFailureCode = stage, recoverable, code
	state.Status = "blocked"
	if recoverable {
		state.Status = "recoverable"
	}
	state.UpdatedAt = d.Now().UTC().Format(time.RFC3339)
	if err := distributionOrchestrationDeps.writeRun(plan.Paths.StateDir, state); err != nil {
		durable, readErr := distributionOrchestrationDeps.readRun(plan.Paths.StateDir, state.RunID)
		if readErr == nil {
			return &durable, safeDistributionFailure(stage, "checkpoint_failed", err)
		}
		return nil, safeDistributionFailure(stage, "checkpoint_failed", errors.Join(err, cause))
	}
	if err := verifyDistributionRunPathLease(ctx); err != nil {
		return nil, safeDistributionFailure(stage, "run_path_changed", err)
	}
	return &state, safeDistributionFailure(stage, code, cause)
}

func monotonicDistributionStage(current, requested string) string {
	// A failed attempt to persist the terminal complete state leaves only the
	// in-memory value at complete. Recovery must fall back to the last durable
	// fetch-verification stage; a non-complete status may never use complete.
	if current == "complete" && requested != "complete" {
		return requested
	}
	order := map[string]int{
		"preflight": 0, "identity_validate": 1, "account_reconcile": 2,
		"export": 3, "prepare": 4, "publish": 5, "fetch_verify": 6,
		"complete": 7,
	}
	if order[current] > order[requested] {
		return current
	}
	return requested
}

func safeDistributionFailure(stage, code string, _ error) error {
	return fmt.Errorf("distribution %s failed (%s); inspect the protected run state for details", stage, code)
}

func safeDistributionPlanningFailure(code string) error {
	return fmt.Errorf("distribution planning failed (%s); inspect protected inputs and rerun", code)
}

func deterministicDistributionRunID(plan persistedDistributionPlan) string {
	digest := sha256.Sum256([]byte("asc-distribution-run-v1\x00" + plan.PlanID + "\x00" + plan.PlanHash))
	return "drun_" + hex.EncodeToString(digest[:16])
}

func distributionEffects(reconcile signing.ReconcilePlanView) []distributionEffect {
	effects := make([]distributionEffect, 0, len(reconcile.Actions)+7)
	registerCount := 0
	for _, action := range reconcile.Actions {
		kind := ""
		switch action.Kind {
		case "registerDevice":
			registerCount++
			continue
		case "createBundleID":
			kind = "create_bundle_id"
		case "createProfile":
			kind = "create_profile"
		}
		if kind != "" {
			effect := distributionEffect{Stage: "account_reconcile", Kind: kind, BundleID: action.BundleID}
			effects = append(effects, effect)
		}
	}
	if registerCount > 0 {
		effects = append([]distributionEffect{{Stage: "account_reconcile", Kind: "register_device", Count: registerCount}}, effects...)
	}
	if len(reconcile.Targets) > 0 {
		bundleID := reconcile.Targets[0].BundleID
		effects = append(
			effects,
			distributionEffect{Stage: "account_reconcile", Kind: "write_profile", BundleID: bundleID},
			distributionEffect{Stage: "export", Kind: "write_export_options", BundleID: bundleID},
			distributionEffect{Stage: "export", Kind: "write_ipa", BundleID: bundleID},
			distributionEffect{Stage: "prepare", Kind: "write_bundle", BundleID: bundleID},
		)
	}
	effects = append(
		effects,
		distributionEffect{Stage: "publish", Kind: "ensure_ipa"},
		distributionEffect{Stage: "publish", Kind: "ensure_manifest"},
		distributionEffect{Stage: "publish", Kind: "ensure_install_page"},
	)
	return effects
}

func safeDistributionBlockers(values []string) []distributionBlocker {
	if len(values) == 0 {
		return nil
	}
	return []distributionBlocker{{Code: "signing_reconcile_blocked", Stage: "account_reconcile", Message: "signing reconciliation reported one or more blockers; inspect the protected reconcile plan"}}
}

func archiveSnapshotMatchesPlan(snapshot archiveTreeSnapshot, binding distributionArchiveBinding) bool {
	return snapshot.TreeSHA256 == binding.TreeSHA256 && snapshot.SizeBytes == binding.SizeBytes && snapshot.EntryCount == binding.FileCount &&
		snapshot.App.BundleID == binding.BundleID && snapshot.App.Title == binding.Title && snapshot.App.Version == binding.Version &&
		snapshot.App.BuildNumber == binding.BuildNumber && snapshot.App.MinimumOSVersion == binding.MinimumOSVersion
}

func descriptorAppMatchesArchive(app core.App, binding distributionArchiveBinding) bool {
	return app.BundleID == binding.BundleID && app.Title == binding.PublishedTitle && app.Version == binding.Version &&
		app.BuildNumber == binding.BuildNumber && app.MinimumOSVersion == binding.MinimumOSVersion
}

func preparedAppMatchesArchive(app core.PreparedApp, binding distributionArchiveBinding) bool {
	return app.BundleID == binding.BundleID && app.Title == binding.PublishedTitle && app.Version == binding.Version &&
		app.BuildNumber == binding.BuildNumber && app.MinimumOSVersion == binding.MinimumOSVersion
}

func validatePreparedDistributionBinding(plan persistedDistributionPlan, state persistedDistributionRunState, descriptor core.Descriptor) error {
	if err := core.ValidateDescriptorForPublish(descriptor); err != nil {
		return err
	}
	if !descriptorAppMatchesArchive(descriptor.App, plan.Archive) || descriptor.Signing.TeamID != plan.Identity.TeamID {
		return fmt.Errorf("prepared app identity does not match the plan")
	}
	if descriptor.Artifact.SHA256 != state.Artifacts.IPA.SHA256 || descriptor.Artifact.SizeBytes != state.Artifacts.IPA.SizeBytes {
		return fmt.Errorf("prepared IPA does not match the exported IPA")
	}
	if descriptor.Signing.ProfileUUID != state.Artifacts.Profile.UUID || descriptor.Signing.EmbeddedProfileSHA256 != state.Artifacts.Profile.SHA256 {
		return fmt.Errorf("prepared profile does not match the reconciled profile")
	}
	if descriptor.Signing.DeviceSetSHA256 != plan.DeviceSet.SHA256 || descriptor.Signing.DeviceCount != plan.DeviceSet.Count {
		return fmt.Errorf("prepared device set does not match the plan")
	}
	if !containsFold(descriptor.Signing.ProfileCertificateSHA256Fingerprints, plan.Identity.CertificateSHA256) || !containsFold(descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints, plan.Identity.CertificateSHA256) {
		return fmt.Errorf("prepared certificate evidence does not match the planned identity")
	}
	return nil
}

func validateExistingDistributionPreparedBundle(bundleDir string, plan persistedDistributionPlan, state persistedDistributionRunState) error {
	descriptorFile, err := hashDistributionFile(filepath.Join(bundleDir, "bundle.json"))
	if err != nil {
		return fmt.Errorf("hash prepared descriptor: %w", err)
	}
	if state.Artifacts.Bundle == nil || descriptorFile.SHA256 != state.Artifacts.Bundle.DescriptorSHA256 {
		return fmt.Errorf("prepared descriptor bytes differ from the exact run evidence")
	}
	bundle, err := core.LoadPreparedBundle(bundleDir)
	if err != nil {
		return err
	}
	defer bundle.IPA.Close()
	descriptor := bundle.Descriptor
	if !preparedAppMatchesArchive(descriptor.App, plan.Archive) || descriptor.Signing.TeamID != plan.Identity.TeamID || descriptor.Artifact.SHA256 != state.Artifacts.IPA.SHA256 || descriptor.Artifact.SizeBytes != state.Artifacts.IPA.SizeBytes || descriptor.Signing.ProfileUUID != state.Artifacts.Profile.UUID || descriptor.Signing.EmbeddedProfileSHA256 != state.Artifacts.Profile.SHA256 || descriptor.Signing.DeviceSetSHA256 != plan.DeviceSet.SHA256 || descriptor.Signing.DeviceCount != plan.DeviceSet.Count || !containsFold(descriptor.Signing.ProfileCertificateSHA256Fingerprints, plan.Identity.CertificateSHA256) || !containsFold(descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints, plan.Identity.CertificateSHA256) {
		return fmt.Errorf("prepared bundle does not match the exact run evidence")
	}
	if bundle.IPASHA256 != state.Artifacts.IPA.SHA256 || bundle.IPASize != state.Artifacts.IPA.SizeBytes {
		return fmt.Errorf("prepared payload differs from the exact exported IPA")
	}
	return nil
}

func newPersistedDistributionReceipt(plan persistedDistributionPlan, state persistedDistributionRunState, descriptor core.Descriptor, published core.PublishReceipt, descriptorSize int64, now time.Time) persistedDistributionReceipt {
	publication := state.Artifacts.Publication
	return persistedDistributionReceipt{
		SchemaVersion: 1, RunID: state.RunID, PlanID: plan.PlanID, PlanHash: plan.PlanHash, Status: "published_and_fetch_verified", CompletedAt: now.UTC().Format(time.RFC3339),
		PublicationReceiptPath: publication.ReceiptPath, PublicationReceiptSHA256: publication.ReceiptSHA256, LinkPath: publication.LinkPath, LinkSHA256: publication.LinkSHA256,
		ArtifactSHA256: state.Artifacts.IPA.SHA256, BundleDescriptorSHA256: state.Artifacts.Bundle.DescriptorSHA256,
		ArtifactKey: publication.ArtifactKey, ManifestKey: publication.ManifestKey, PageKey: publication.PageKey, InstallURLRedacted: publication.InstallURLRedacted,
		AppBundleID: descriptor.App.BundleID, AppVersion: descriptor.App.Version, AppBuildNumber: descriptor.App.BuildNumber,
		TeamID: descriptor.Signing.TeamID, ProfileResourceID: state.Artifacts.Profile.ResourceID, ProfileClass: string(descriptor.Signing.ProfileClass),
		ProfileUUID: descriptor.Signing.ProfileUUID, ProfileExpiresAt: descriptor.Signing.ExpiresAt, ProfileSHA256: descriptor.Signing.EmbeddedProfileSHA256,
		DeviceSetSHA256: descriptor.Signing.DeviceSetSHA256, DeviceCount: descriptor.Signing.DeviceCount, CertificateSHA256: plan.Identity.CertificateSHA256,
		ProfileCertificateSHA256Fingerprints: append([]string(nil), descriptor.Signing.ProfileCertificateSHA256Fingerprints...),
		SignerCertificateSHA256Fingerprints:  append([]string(nil), descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints...),
		ProfileIntegrityStatus:               string(descriptor.Signing.ProfileIntegrityVerification.Status), ProfileTrustStatus: string(descriptor.Signing.ProfileTrustVerification.Status),
		CodeSignatureStatus: string(descriptor.Signing.CodeSignatureVerification.Status), CodeSignatureScope: descriptor.Signing.CodeSignatureVerification.Scope,
		ArtifactSizeBytes: state.Artifacts.IPA.SizeBytes, BundleDescriptorSizeBytes: descriptorSize,
		ManifestSHA256: published.Manifest.SHA256, ManifestSizeBytes: published.Manifest.SizeBytes, PageSHA256: published.Page.SHA256, PageSizeBytes: published.Page.SizeBytes,
		FetchVerified: published.Verified, FetchVerifiedAt: now.UTC().Format(time.RFC3339),
	}
}

func loadExactDistributionCompletion(plan persistedDistributionPlan, state persistedDistributionRunState) (persistedDistributionReceipt, error) {
	receipt, err := distributionOrchestrationDeps.readReceipt(plan.Paths.StateDir, state.RunID)
	if err != nil {
		return receipt, fmt.Errorf("read immutable completion receipt: %w", err)
	}
	validate := distributionOrchestrationDeps.validateCompletion
	if validate == nil {
		validate = func(plan persistedDistributionPlan, state persistedDistributionRunState, receipt persistedDistributionReceipt) error {
			if !completionMatchesRunPlan(receipt, state, plan) {
				return fmt.Errorf("completion mismatch")
			}
			return nil
		}
	}
	if err := validate(plan, state, receipt); err != nil {
		return receipt, fmt.Errorf("immutable completion receipt does not match the distribution run and plan: %w", err)
	}
	return receipt, nil
}

func validateExactDistributionCompletion(plan persistedDistributionPlan, state persistedDistributionRunState, receipt persistedDistributionReceipt) error {
	if err := validateAdoptedReconcileEvidenceLocal(plan.Paths.StateDir, plan, state); err != nil {
		return err
	}
	if err := validateAdoptedSigningEvidence(plan.Paths.StateDir, plan, state); err != nil {
		return err
	}
	runRoot, err := protectedDistributionRunRoot(plan.Paths.StateDir, state.RunID)
	if err != nil {
		return err
	}
	defer runRoot.Close()
	return distributionReceiptMatchesRunAndPlan(runRoot, receipt, state, plan)
}

func validateDistributionRunPlanBinding(stateDir string, state persistedDistributionRunState, plan persistedDistributionPlan) error {
	if state.RunID != deterministicDistributionRunID(plan) || state.PlanID != plan.PlanID || state.PlanHash != plan.PlanHash || plan.Paths.StateDir != stateDir {
		return fmt.Errorf("distribution run does not match its requested state root and plan")
	}
	return nil
}

func remainingDistributionTimeout(ctx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			return remaining
		}
		return 0
	}
	return fallback
}

func completionMatchesRunPlan(receipt persistedDistributionReceipt, state persistedDistributionRunState, plan persistedDistributionPlan) bool {
	if receipt.RunID != state.RunID || receipt.PlanID != plan.PlanID || receipt.PlanHash != plan.PlanHash || receipt.Status != "published_and_fetch_verified" || !receipt.FetchVerified {
		return false
	}
	if state.Artifacts.IPA == nil || state.Artifacts.Bundle == nil || state.Artifacts.Profile == nil || state.Artifacts.Publication == nil {
		return false
	}
	if !distributionArchiveSnapshotMatchesPlan(state.Artifacts.ArchiveSnapshot, plan.Archive) {
		return false
	}
	p := state.Artifacts.Publication
	return receipt.ArtifactSHA256 == state.Artifacts.IPA.SHA256 && receipt.BundleDescriptorSHA256 == state.Artifacts.Bundle.DescriptorSHA256 && receipt.ProfileSHA256 == state.Artifacts.Profile.SHA256 && receipt.ProfileResourceID == state.Artifacts.Profile.ResourceID && receipt.ProfileUUID == state.Artifacts.Profile.UUID && receipt.AppBundleID == plan.Archive.BundleID && receipt.AppVersion == plan.Archive.Version && receipt.AppBuildNumber == plan.Archive.BuildNumber && receipt.TeamID == plan.Identity.TeamID && receipt.DeviceSetSHA256 == plan.DeviceSet.SHA256 && receipt.CertificateSHA256 == plan.Identity.CertificateSHA256 && receipt.PublicationReceiptPath == p.ReceiptPath && receipt.PublicationReceiptSHA256 == p.ReceiptSHA256 && receipt.LinkPath == p.LinkPath && receipt.LinkSHA256 == p.LinkSHA256 && receipt.ArtifactKey == p.ArtifactKey && receipt.ManifestKey == p.ManifestKey && receipt.PageKey == p.PageKey && receipt.InstallURLRedacted == p.InstallURLRedacted
}

func publishMatchesCompletion(published core.PublishReceipt, receipt persistedDistributionReceipt, plan persistedDistributionPlan, state persistedDistributionRunState) bool {
	if state.Artifacts.Publication == nil || published.SchemaVersion != "1" || !published.Verified || published.Access != core.AccessPrivate || published.PublicBaseURL != "" {
		return false
	}
	wantEndpoint, endpointErr := normalizedDistributionEndpoint(plan.Publication.Endpoint)
	wantDownloadEndpoint, downloadErr := normalizedDistributionEndpoint(plan.Publication.DownloadEndpoint)
	if plan.Publication.DownloadEndpoint == "" {
		wantDownloadEndpoint, downloadErr = wantEndpoint, endpointErr
	}
	if endpointErr != nil || downloadErr != nil || published.Endpoint != wantEndpoint || published.DownloadEndpoint != wantDownloadEndpoint || published.Region != plan.Publication.Region || published.AddressingStyle != plan.Publication.AddressingStyle || published.Bucket != plan.Publication.Bucket || published.Prefix != plan.Publication.Prefix {
		return false
	}
	publishedTTL, publishedTTLErr := time.ParseDuration(published.URLTTL)
	plannedTTL, plannedTTLErr := time.ParseDuration(plan.Publication.URLTTL)
	publishedGrace, publishedGraceErr := time.ParseDuration(published.DownloadGrace)
	plannedGrace, plannedGraceErr := time.ParseDuration(plan.Publication.DownloadGrace)
	if publishedTTLErr != nil || plannedTTLErr != nil || publishedGraceErr != nil || plannedGraceErr != nil || publishedTTL != plannedTTL || publishedGrace != plannedGrace {
		return false
	}
	validObject := func(object core.StoredObject, key, sha256 string, size int64, contentType string) bool {
		return object.Key == key && object.SHA256 == sha256 && object.SizeBytes == size && object.ContentType == contentType && validDistributionStoredObjectStatus(object.Status)
	}
	if !validObject(published.Artifact, receipt.ArtifactKey, receipt.ArtifactSHA256, receipt.ArtifactSizeBytes, core.ContentTypeIPA) ||
		!validObject(published.Manifest, receipt.ManifestKey, receipt.ManifestSHA256, receipt.ManifestSizeBytes, core.ContentTypeManifest) ||
		!validObject(published.Page, receipt.PageKey, receipt.PageSHA256, receipt.PageSizeBytes, core.ContentTypeHTML) {
		return false
	}
	if published.InstallURL != receipt.InstallURLRedacted || published.DirectInstallURL != "itms-services://?action=download-manifest&url=REDACTED" || published.ExpiresAt == nil {
		return false
	}
	completedAt, completedErr := time.Parse(time.RFC3339, receipt.CompletedAt)
	verifiedAt, verifiedErr := time.Parse(time.RFC3339, receipt.FetchVerifiedAt)
	if completedErr != nil || verifiedErr != nil || !published.ExpiresAt.After(completedAt) || !published.ExpiresAt.After(verifiedAt) {
		return false
	}
	if published.App.BundleID != receipt.AppBundleID || published.App.Title != plan.Archive.PublishedTitle || published.App.Version != receipt.AppVersion || published.App.BuildNumber != receipt.AppBuildNumber || published.App.MinimumOSVersion != plan.Archive.MinimumOSVersion {
		return false
	}
	signing := published.Signing
	if signing.ProfileClass != receipt.ProfileClass || signing.ProfileUUID != receipt.ProfileUUID || signing.EmbeddedProfileSHA256 != receipt.ProfileSHA256 || signing.TeamID != receipt.TeamID || signing.ProfileExpiresAt != receipt.ProfileExpiresAt || signing.DeviceCount != receipt.DeviceCount || signing.DeviceSetSHA256 != receipt.DeviceSetSHA256 || !reflect.DeepEqual(signing.ProfileCertificateFingerprints, receipt.ProfileCertificateSHA256Fingerprints) {
		return false
	}
	if signing.ProfileIntegrityVerification.Status != receipt.ProfileIntegrityStatus ||
		signing.ProfileTrustVerification.Status != receipt.ProfileTrustStatus ||
		!distributionVerifiedPublicationCheck(signing.CodeSignatureVerification, receipt.CodeSignatureStatus, receipt.CodeSignatureScope, receipt.SignerCertificateSHA256Fingerprints) {
		return false
	}
	wantRunRoot := filepath.Join(plan.Paths.StateDir, state.RunID)
	wantReceiptPath := filepath.Clean(filepath.Join(wantRunRoot, filepath.FromSlash(state.Artifacts.Publication.ReceiptPath)))
	wantLinkPath := filepath.Clean(filepath.Join(wantRunRoot, filepath.FromSlash(state.Artifacts.Publication.LinkPath)))
	return filepath.IsAbs(published.ReceiptPath) && filepath.IsAbs(published.LinkPath) && filepath.Clean(published.ReceiptPath) == wantReceiptPath && filepath.Clean(published.LinkPath) == wantLinkPath
}

func distributionVerifiedPublicationCheck(actual core.PreparedCodeSignatureVerification, status, scope string, fingerprints []string) bool {
	return actual.Status == status && actual.Scope == scope && reflect.DeepEqual(actual.SignerCertificateSHA256Fingerprints, fingerprints)
}

func validPreparedResultFromPublishForOrchestration(receipt core.PublishReceipt, state persistedDistributionRunState) core.PrepareResult {
	toVerification := func(value core.PreparedCodeSignatureVerification) core.CodeSignatureVerification {
		return core.CodeSignatureVerification{
			Status: core.CodeSignatureVerificationStatus(value.Status), Reason: value.Reason, Scope: value.Scope,
			SignerCertificateSHA256Fingerprints: append([]string(nil), value.SignerCertificateSHA256Fingerprints...),
		}
	}
	return core.PrepareResult{Descriptor: core.Descriptor{
		SchemaVersion: "1", Platform: "IOS", DistributionMethod: "release-testing",
		App:      core.App{BundleID: receipt.App.BundleID, Title: receipt.App.Title, Version: receipt.App.Version, BuildNumber: receipt.App.BuildNumber, MinimumOSVersion: receipt.App.MinimumOSVersion},
		Artifact: core.Artifact{SHA256: state.Artifacts.IPA.SHA256, SizeBytes: state.Artifacts.IPA.SizeBytes},
		Signing: core.Signing{
			ProfileClass: core.ProfileClass(receipt.Signing.ProfileClass), ProfileUUID: receipt.Signing.ProfileUUID,
			TeamID: receipt.Signing.TeamID, ExpiresAt: receipt.Signing.ProfileExpiresAt,
			DeviceCount: receipt.Signing.DeviceCount, DeviceSetSHA256: receipt.Signing.DeviceSetSHA256,
			EmbeddedProfileSHA256:                receipt.Signing.EmbeddedProfileSHA256,
			ProfileCertificateSHA256Fingerprints: append([]string(nil), receipt.Signing.ProfileCertificateFingerprints...),
			ProfileIntegrityVerification:         toVerification(receipt.Signing.ProfileIntegrityVerification),
			ProfileTrustVerification:             toVerification(receipt.Signing.ProfileTrustVerification),
			CodeSignatureVerification:            toVerification(receipt.Signing.CodeSignatureVerification),
		},
	}}
}

func canonicalAbsolutePath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func resolveDistributionConfigPath(configPath, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(value) {
		return canonicalAbsolutePath(value)
	}
	return canonicalAbsolutePath(filepath.Join(filepath.Dir(configPath), value))
}

func resolveOptionalDistributionConfigPath(configPath, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return resolveDistributionConfigPath(configPath, value)
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func hashProtectedDistributionFile(path string) (string, error) {
	data, err := readProtectedDistributionFile(path, 8<<20)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func snapshotDistributionArchive(ctx context.Context, archivePath, stateDir, runID string) (archiveTreeSnapshot, error) {
	runDir := filepath.Join(stateDir, runID)
	root, err := rootfs.New(runDir)
	if err != nil {
		return archiveTreeSnapshot{}, err
	}
	defer root.Close()
	if info, statErr := os.Lstat(filepath.Join(runDir, distributionArchiveRelative)); statErr == nil && info.IsDir() {
		actual, digestErr := digestXCArchive(ctx, filepath.Join(runDir, distributionArchiveRelative))
		if digestErr != nil {
			return archiveTreeSnapshot{}, digestErr
		}
		actual.RelativePath = distributionArchiveRelative
		if err := revalidateXCArchiveSnapshot(ctx, root, actual); err != nil {
			return archiveTreeSnapshot{}, err
		}
		return actual, nil
	}
	return snapshotXCArchive(ctx, archivePath, root, distributionArchiveRelative)
}

func createDistributionRunScaffold(stateDir, runID string) error {
	if err := createDistributionRunDirectory(stateDir, runID); err != nil {
		return err
	}
	runRoot, err := rootfs.New(filepath.Join(stateDir, runID))
	if err != nil {
		return err
	}
	defer runRoot.Close()
	for _, relative := range distributionRunScaffoldDirectories() {
		if err := runRoot.MkdirAll(relative, 0o700); err != nil {
			return fmt.Errorf("create distribution run directory %s: %w", relative, err)
		}
	}
	return nil
}

func ensureDistributionRunScaffold(stateDir, runID string) error {
	runRoot, err := protectedDistributionRunRoot(stateDir, runID)
	if err != nil {
		return err
	}
	defer runRoot.Close()
	for _, relative := range distributionRunScaffoldDirectories() {
		if _, err := runRoot.Lstat(relative); errors.Is(err, os.ErrNotExist) {
			if err := runRoot.Mkdir(relative, 0o700); err != nil {
				return fmt.Errorf("create distribution run directory %s: %w", relative, err)
			}
			if err := distributionSyncDirectoryForTest(runRoot); err != nil {
				return fmt.Errorf("sync distribution run directory: %w", err)
			}
		} else if err != nil {
			return err
		}
		child, err := openPinnedDistributionChild(runRoot, relative)
		if err != nil {
			return err
		}
		err = validatePrivateDistributionDirectoryRoot(child)
		_ = child.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func distributionRunScaffoldDirectories() []string {
	return []string{"inputs", "reconcile", "export", "signing", "prepared", "publish", "secrets"}
}

func revalidateDistributionArchive(ctx context.Context, stateDir, runID string, snapshot distributionArchiveSnapshot) error {
	root, err := rootfs.New(filepath.Join(stateDir, runID))
	if err != nil {
		return err
	}
	defer root.Close()
	return revalidateXCArchiveSnapshot(ctx, root, archiveTreeSnapshot(snapshot))
}

func copyDistributionArtifact(source, stateDir, runID, relative string, limit int64) (distributionSizedFileArtifact, error) {
	data, err := readProtectedDistributionFile(source, limit)
	if err != nil {
		return distributionSizedFileArtifact{}, err
	}
	runRoot, err := rootfs.New(filepath.Join(stateDir, runID))
	if err != nil {
		return distributionSizedFileArtifact{}, err
	}
	defer runRoot.Close()
	if parent := filepath.Dir(relative); parent != "." {
		if err := runRoot.MkdirAll(parent, 0o700); err != nil {
			return distributionSizedFileArtifact{}, err
		}
	}
	if err := runRoot.CreateNewFileAtomic(relative, data, 0o600); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return distributionSizedFileArtifact{}, err
		}
		existing, readErr := runRoot.ReadFileLimited(relative, limit)
		if readErr != nil || !reflect.DeepEqual(existing, data) {
			return distributionSizedFileArtifact{}, fmt.Errorf("%w: existing artifact differs from requested copy", errDistributionArtifactConflict)
		}
	}
	digest := sha256.Sum256(data)
	return distributionSizedFileArtifact{Path: relative, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(data))}, nil
}

func preflightDistributionPublication(ctx context.Context, d *shared.Deps, config distributionPublicationConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	credentialCtx, cancel := d.Timeout(ctx)
	defer cancel()
	_, credentialLimit, err := newObjectStore(credentialCtx, core.S3StoreConfig{Endpoint: config.Endpoint, DownloadEndpoint: config.DownloadEndpoint, Region: config.Region, Bucket: config.Bucket, AddressingStyle: config.AddressingStyle})
	if err != nil {
		return err
	}
	urlTTL, downloadGrace, _, err := distributionPublicationDurations(config)
	if err != nil {
		return err
	}
	if !credentialLimitCoversDistributionPublication(credentialLimit, time.Now().UTC(), urlTTL, downloadGrace) {
		return errDistributionPublicationCredentialsExpireTooSoon
	}
	return nil
}

func credentialLimitCoversDistributionPublication(credentialLimit, now time.Time, urlTTL, downloadGrace time.Duration) bool {
	return credentialLimit.IsZero() || credentialLimit.After(now.Add(urlTTL+downloadGrace+distributionPublicationValiditySafetyMargin))
}

func effectiveDistributionMinimumValidityDays(configured int, urlTTL, downloadGrace time.Duration) (int, error) {
	if configured < 0 || configured > distributionMaximumValidityDays || urlTTL <= 0 || downloadGrace < 0 {
		return 0, fmt.Errorf("invalid distribution validity policy")
	}
	required := urlTTL + downloadGrace + distributionPublicationValiditySafetyMargin
	day := 24 * time.Hour
	// Signing reconciliation expresses validity in whole days and evaluates it
	// against its own current time. Always choose a whole-day policy strictly
	// longer than the publication window so planning-to-apply delay cannot turn
	// an equality boundary into an under-authorized profile.
	requiredDays := int(required/day) + 1
	if configured > requiredDays {
		requiredDays = configured
	}
	if requiredDays > distributionMaximumValidityDays {
		return 0, fmt.Errorf("distribution validity policy exceeds maximum")
	}
	return requiredDays, nil
}

func hashDistributionFile(path string) (distributionSizedFileArtifact, error) {
	anchored, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return distributionSizedFileArtifact{}, err
	}
	defer anchored.Close()
	file, err := anchored.OpenFile(filepath.Base(path))
	if err != nil {
		return distributionSizedFileArtifact{}, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return distributionSizedFileArtifact{}, fmt.Errorf("distribution artifact must be a regular file")
	}
	uid, nlink, ok := distributionStatIdentity(before)
	if (runtime.GOOS != "windows" && (!ok || uid != uint64(os.Geteuid()))) || (ok && nlink != 1) {
		return distributionSizedFileArtifact{}, fmt.Errorf("distribution artifact must be owned by the current user and single-linked")
	}
	if runtime.GOOS != "windows" && before.Mode().Perm() != 0o600 && before.Mode().Perm() != 0o644 {
		return distributionSizedFileArtifact{}, fmt.Errorf("distribution artifact permissions must be 0600 or 0644")
	}
	h := sha256.New()
	size, err := io.Copy(h, io.LimitReader(file, (8<<30)+1))
	if err != nil {
		return distributionSizedFileArtifact{}, err
	}
	if size > 8<<30 {
		return distributionSizedFileArtifact{}, fmt.Errorf("distribution artifact exceeds size limit")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return distributionSizedFileArtifact{}, fmt.Errorf("distribution artifact changed while hashing")
	}
	return distributionSizedFileArtifact{Path: path, SHA256: hex.EncodeToString(h.Sum(nil)), SizeBytes: size}, nil
}

func distributionPathExists(path string) (bool, error) {
	anchored, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	defer anchored.Close()
	file, err := anchored.OpenFile(filepath.Base(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, file.Close()
}

func orchestrationPathExists(path string) (bool, error) {
	if distributionOrchestrationDeps.pathExists == nil {
		return false, nil
	}
	return distributionOrchestrationDeps.pathExists(path)
}

func distributionPublicationDurations(config distributionPublicationConfig) (time.Duration, time.Duration, time.Duration, error) {
	urlTTL, err := time.ParseDuration(config.URLTTL)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid publication URL TTL")
	}
	downloadGrace, err := time.ParseDuration(config.DownloadGrace)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid publication download grace")
	}
	verifyTimeout, err := time.ParseDuration(config.VerifyTimeout)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid publication verify timeout")
	}
	return urlTTL, downloadGrace, verifyTimeout, nil
}

func prepareDistributionIPA(ctx context.Context, request distributionPrepareRequest) (core.PrepareResult, error) {
	runDir := filepath.Join(request.StateDir, request.RunID)
	root, err := rootfs.New(runDir)
	if err != nil {
		return core.PrepareResult{}, err
	}
	defer root.Close()
	return core.PrepareIPAPathExact(ctx, root, request.IPARelativePath, core.ExpectedIPA{SHA256: request.ExpectedSHA256, SizeBytes: request.ExpectedSize}, core.PrepareOptions{Root: runDir, OutputDir: distributionBundleRelative, Title: request.Metadata.Title, Channel: request.Metadata.Channel, SourceRevision: request.Metadata.SourceRevision, SourceURL: request.Metadata.SourceURL})
}

func observeDistributionDevice(ctx context.Context, request distributionDeviceObservationRequest) (deviceObservation, error) {
	return observeInstalledAppOnDevice(ctx, deviceObservationRequest{DeviceSelector: request.Device, BundleID: request.BundleID, Version: request.Version, Build: request.Build, Timeout: request.Timeout, Environment: signing.SanitizedChildEnvironment(os.Environ())})
}
