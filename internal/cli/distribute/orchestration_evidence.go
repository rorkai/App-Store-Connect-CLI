package distribute

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/signing"
)

const distributionReconcileProfileMaxBytes = 16 << 20

// distributionReconcileReceiptEvidence mirrors only the protected receipt
// fields needed to adopt a completed signing reconciliation. Keeping this
// projection local avoids treating a file hash as semantic proof.
type distributionReconcileReceiptEvidence struct {
	SchemaVersion int                                          `json:"schemaVersion"`
	PlanHash      string                                       `json:"planHash"`
	StartedAt     string                                       `json:"startedAt"`
	UpdatedAt     string                                       `json:"updatedAt"`
	Complete      bool                                         `json:"complete"`
	StateDir      string                                       `json:"stateDir"`
	ReceiptPath   string                                       `json:"receiptPath"`
	Actions       []distributionReconcileActionReceiptEvidence `json:"actions"`
}

type distributionReconcileActionReceiptEvidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	ResourceID string `json:"resourceId,omitempty"`
	OutputPath string `json:"outputPath,omitempty"`
	Error      string `json:"error,omitempty"`
}

type distributionSigningReceiptEvidence struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Purpose              string `json:"purpose"`
	Outcome              string `json:"outcome"`
	ChildExitCode        int    `json:"childExitCode"`
	CertificateSHA256    string `json:"certificateSha256"`
	ProfileSHA256        string `json:"profileSha256"`
	ProfileUUID          string `json:"profileUuid"`
	TeamID               string `json:"teamId"`
	BundleID             string `json:"bundleId"`
	ProfileCleanupState  string `json:"profileCleanupState"`
	KeychainCleanupState string `json:"keychainCleanupState"`
}

// validateDistributionReconcileEvidence validates candidate run-local
// evidence before it is adopted into state. verified must come from the
// read-only signing.VerifyReconcileCompletionFromEvidence seam for the exact nested plan.
func validateDistributionReconcileEvidence(
	stateDir string,
	plan persistedDistributionPlan,
	state persistedDistributionRunState,
	verified signing.ReconcileReceiptView,
	receipt distributionFileArtifact,
	profile distributionProfileArtifact,
) error {
	if filepath.Clean(stateDir) != filepath.Clean(plan.Paths.StateDir) || state.PlanID != plan.PlanID || state.PlanHash != plan.PlanHash {
		return fmt.Errorf("run identity does not match the exact distribution plan")
	}
	if verified.SchemaVersion != 1 || !verified.Complete || verified.PlanHash != plan.Reconcile.PlanHash ||
		filepath.Clean(verified.ReceiptPath) != filepath.Clean(plan.Reconcile.ReceiptPath) || verified.MainProfile == nil {
		return fmt.Errorf("verified reconcile receipt does not match the exact nested plan")
	}
	main := verified.MainProfile
	if main.TargetKind != "application" || main.BundleID != plan.Archive.BundleID ||
		main.ResourceID != profile.ResourceID || main.UUID != profile.UUID || main.SHA256 != profile.SHA256 ||
		!filepath.IsAbs(main.Path) || filepath.Clean(main.Path) != main.Path {
		return fmt.Errorf("verified main profile does not match the adopted profile evidence")
	}
	profileMatches := 0
	for _, item := range verified.Profiles {
		if item.TargetKind == main.TargetKind && item.BundleID == main.BundleID && item.ResourceID == main.ResourceID &&
			item.UUID == main.UUID && item.Path == main.Path && item.SHA256 == main.SHA256 {
			profileMatches++
		}
	}
	if profileMatches != 1 {
		return fmt.Errorf("verified reconcile receipt does not uniquely identify the main profile")
	}
	if profile.BundleID != plan.Archive.BundleID || profile.ResourceID == "" || profile.UUID == "" ||
		!distributionDigestPattern.MatchString(profile.SHA256) {
		return fmt.Errorf("adopted profile identity is incomplete")
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, state.RunID)
	if err != nil {
		return fmt.Errorf("open protected distribution run evidence")
	}
	defer runRoot.Close()
	receiptData, err := readDistributionRunEvidenceFileExact(runRoot, receipt.Path, distributionStateMaxBytes, receipt.SHA256)
	if err != nil {
		return fmt.Errorf("reconcile receipt does not match its protected exact hash")
	}
	if _, err := readDistributionRunEvidenceFileExact(runRoot, profile.Path, distributionReconcileProfileMaxBytes, profile.SHA256); err != nil {
		return fmt.Errorf("reconciled profile does not match its protected exact hash")
	}
	var evidence distributionReconcileReceiptEvidence
	if err := decodeStrictDistributionJSON(receiptData, &evidence); err != nil {
		return fmt.Errorf("reconcile receipt is not strict valid JSON")
	}
	startedAt, startedErr := time.Parse(time.RFC3339, evidence.StartedAt)
	updatedAt, updatedErr := time.Parse(time.RFC3339, evidence.UpdatedAt)
	if evidence.SchemaVersion != 1 || evidence.PlanHash != plan.Reconcile.PlanHash || !evidence.Complete ||
		startedErr != nil || updatedErr != nil || updatedAt.Before(startedAt) ||
		filepath.Clean(evidence.StateDir) != filepath.Clean(filepath.Dir(plan.Reconcile.PlanPath)) ||
		filepath.Clean(evidence.ReceiptPath) != filepath.Clean(plan.Reconcile.ReceiptPath) {
		return fmt.Errorf("reconcile receipt semantics do not match the exact nested plan")
	}
	seen := make(map[string]struct{}, len(evidence.Actions))
	receiptProfileMatches := 0
	for _, action := range evidence.Actions {
		if strings.TrimSpace(action.ID) == "" || strings.TrimSpace(action.Kind) == "" || action.Status != "completed" ||
			strings.TrimSpace(action.ResourceID) == "" || action.Error != "" {
			return fmt.Errorf("reconcile receipt contains an incomplete action")
		}
		if _, duplicate := seen[action.ID]; duplicate {
			return fmt.Errorf("reconcile receipt contains duplicate actions")
		}
		seen[action.ID] = struct{}{}
		if action.Kind != "createProfile" && action.Kind != "downloadProfile" {
			continue
		}
		if action.ResourceID == main.ResourceID && filepath.Clean(action.OutputPath) == filepath.Clean(main.Path) {
			receiptProfileMatches++
		}
	}
	if receiptProfileMatches != 1 {
		return fmt.Errorf("reconcile receipt does not uniquely bind the verified main profile")
	}
	return nil
}

func validateAdoptedReconcileEvidence(stateDir string, plan persistedDistributionPlan, state persistedDistributionRunState, verified signing.ReconcileReceiptView) error {
	if state.Artifacts.ReconcileReceipt == nil || state.Artifacts.Profile == nil {
		return fmt.Errorf("adopted reconcile evidence is incomplete")
	}
	return validateDistributionReconcileEvidence(stateDir, plan, state, verified, *state.Artifacts.ReconcileReceipt, *state.Artifacts.Profile)
}

// validateAdoptedReconcileEvidenceLocal validates the protected run copy
// without account access. It is used by status and completion verification;
// resume additionally performs live VerifyReconcileCompletionFromEvidence first.
func validateAdoptedReconcileEvidenceLocal(stateDir string, plan persistedDistributionPlan, state persistedDistributionRunState) error {
	if state.Artifacts.ReconcileReceipt == nil || state.Artifacts.Profile == nil {
		return fmt.Errorf("adopted reconcile evidence is incomplete")
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, state.RunID)
	if err != nil {
		return fmt.Errorf("open protected distribution run evidence")
	}
	data, err := readDistributionRunEvidenceFileExact(runRoot, state.Artifacts.ReconcileReceipt.Path, distributionStateMaxBytes, state.Artifacts.ReconcileReceipt.SHA256)
	_ = runRoot.Close()
	if err != nil {
		return fmt.Errorf("reconcile receipt does not match its protected exact hash")
	}
	var evidence distributionReconcileReceiptEvidence
	if err := decodeStrictDistributionJSON(data, &evidence); err != nil {
		return fmt.Errorf("reconcile receipt is not strict valid JSON")
	}
	var mainPath string
	for _, action := range evidence.Actions {
		if (action.Kind == "createProfile" || action.Kind == "downloadProfile") && action.ResourceID == state.Artifacts.Profile.ResourceID {
			if mainPath != "" {
				return fmt.Errorf("reconcile receipt does not uniquely bind the main profile")
			}
			mainPath = action.OutputPath
		}
	}
	verified := signing.ReconcileReceiptView{
		SchemaVersion: 1, PlanHash: plan.Reconcile.PlanHash, Complete: true, ReceiptPath: plan.Reconcile.ReceiptPath,
		MainProfile: &signing.ReconcileProfileView{
			TargetKind: "application", BundleID: state.Artifacts.Profile.BundleID,
			ResourceID: state.Artifacts.Profile.ResourceID, UUID: state.Artifacts.Profile.UUID, Path: mainPath, SHA256: state.Artifacts.Profile.SHA256,
		},
	}
	verified.Profiles = []signing.ReconcileProfileView{*verified.MainProfile}
	return validateAdoptedReconcileEvidence(stateDir, plan, state, verified)
}

// validateDistributionSigningEvidence validates a candidate attempt-scoped
// signing receipt before it proves an already-exported IPA safe to adopt.
func validateDistributionSigningEvidence(
	stateDir string,
	plan persistedDistributionPlan,
	state persistedDistributionRunState,
	receipt distributionFileArtifact,
) error {
	if filepath.Clean(stateDir) != filepath.Clean(plan.Paths.StateDir) || state.PlanID != plan.PlanID || state.PlanHash != plan.PlanHash {
		return fmt.Errorf("run identity does not match the exact distribution plan")
	}
	attempt, err := distributionSigningReceiptAttempt(receipt.Path)
	if err != nil || attempt < 1 || attempt > state.Attempt {
		return fmt.Errorf("signing receipt is not scoped to a completed run attempt")
	}
	if state.Artifacts.Profile == nil {
		return fmt.Errorf("signing receipt requires adopted profile evidence")
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, state.RunID)
	if err != nil {
		return fmt.Errorf("open protected distribution run evidence")
	}
	defer runRoot.Close()
	data, err := readDistributionRunEvidenceFileExact(runRoot, receipt.Path, distributionStateMaxBytes, receipt.SHA256)
	if err != nil {
		return fmt.Errorf("signing receipt does not match its protected exact hash")
	}
	var evidence distributionSigningReceiptEvidence
	if err := decodeStrictDistributionJSON(data, &evidence); err != nil {
		return fmt.Errorf("signing receipt is not strict valid JSON")
	}
	profileCleanupComplete := evidence.ProfileCleanupState == "removed" || evidence.ProfileCleanupState == "reused"
	if evidence.SchemaVersion != 1 || evidence.Purpose != "release-testing" || evidence.Outcome != "succeeded" ||
		evidence.ChildExitCode != 0 || !profileCleanupComplete || evidence.KeychainCleanupState != "deleted" ||
		!strings.EqualFold(evidence.CertificateSHA256, plan.Identity.CertificateSHA256) ||
		!strings.EqualFold(evidence.ProfileSHA256, state.Artifacts.Profile.SHA256) ||
		evidence.ProfileUUID != state.Artifacts.Profile.UUID || evidence.TeamID != plan.Identity.TeamID ||
		evidence.BundleID != plan.Archive.BundleID {
		return fmt.Errorf("signing receipt does not prove the exact successful cleaned-up signing run")
	}
	return nil
}

func validateAdoptedSigningEvidence(stateDir string, plan persistedDistributionPlan, state persistedDistributionRunState) error {
	if state.Artifacts.SigningReceipt == nil {
		return fmt.Errorf("adopted signing evidence is incomplete")
	}
	return validateDistributionSigningEvidence(stateDir, plan, state, *state.Artifacts.SigningReceipt)
}

func distributionSigningReceiptAttempt(relative string) (int, error) {
	const prefix = "signing/receipt-"
	const suffix = ".json"
	if !strings.HasPrefix(relative, prefix) || !strings.HasSuffix(relative, suffix) {
		return 0, fmt.Errorf("invalid signing receipt path")
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(relative, prefix), suffix)
	if len(raw) != 6 {
		return 0, fmt.Errorf("invalid signing receipt path")
	}
	attempt, err := strconv.Atoi(raw)
	if err != nil || fmt.Sprintf("%06d", attempt) != raw {
		return 0, fmt.Errorf("invalid signing receipt path")
	}
	return attempt, nil
}
