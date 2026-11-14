package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ErrReconcilePlanDrift marks a terminal mismatch between a hash-confirmed
// signing plan and the current local or App Store Connect state. Adapter
// callers may use errors.Is without receiving the underlying provider detail.
var ErrReconcilePlanDrift = errors.New("signing reconcile plan no longer matches current state")

// ReconcileExecutionErrorKind is the closed, redacted failure classification
// returned by the orchestration-safe reconcile adapters.
type ReconcileExecutionErrorKind string

const (
	ReconcileExecutionErrorPlanInvalid ReconcileExecutionErrorKind = "plan_invalid"
	ReconcileExecutionErrorPlanDrift   ReconcileExecutionErrorKind = "plan_drift"
	ReconcileExecutionErrorRetryable   ReconcileExecutionErrorKind = "retryable"
)

type reconcileExecutionError struct {
	kind ReconcileExecutionErrorKind
}

func (err reconcileExecutionError) Error() string {
	switch err.kind {
	case ReconcileExecutionErrorPlanInvalid:
		return "signing reconcile request or protected plan is invalid"
	case ReconcileExecutionErrorPlanDrift:
		return ErrReconcilePlanDrift.Error()
	default:
		return "signing reconciliation could not be completed; retry may succeed"
	}
}

func (err reconcileExecutionError) Is(target error) bool {
	return err.kind == ReconcileExecutionErrorPlanDrift && target == ErrReconcilePlanDrift
}

// ClassifyReconcileExecutionError returns a closed adapter-safe category.
func ClassifyReconcileExecutionError(err error) ReconcileExecutionErrorKind {
	var executionErr reconcileExecutionError
	if errors.As(err, &executionErr) {
		return executionErr.kind
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ReconcileExecutionErrorRetryable
	}
	return ""
}

type reconcilePlanDriftCause struct {
	err error
}

type reconcilePlanInvalidCause struct {
	err error
}

func (cause reconcilePlanInvalidCause) Error() string {
	return cause.err.Error()
}

func (cause reconcilePlanInvalidCause) Unwrap() error {
	return cause.err
}

func newReconcilePlanInvalid(err error) error {
	return reconcilePlanInvalidCause{err: err}
}

func (cause reconcilePlanDriftCause) Error() string { return cause.err.Error() }
func (cause reconcilePlanDriftCause) Unwrap() []error {
	return []error{cause.err, ErrReconcilePlanDrift}
}

func newReconcilePlanDrift(err error) error {
	if err == nil {
		return ErrReconcilePlanDrift
	}
	return reconcilePlanDriftCause{err: err}
}

func redactedReconcileExecutionError(kind ReconcileExecutionErrorKind) error {
	return reconcileExecutionError{kind: kind}
}

// ReconcilePlanOptions defines a protected, read-only signing reconciliation plan.
type ReconcilePlanOptions struct {
	ArchivePath         string
	DevicesFile         string
	CertificateID       string
	CertificateSHA256   string
	MinimumValidityDays int
	MaxMutations        int
	StateDir            string
	Overwrite           bool
}

// ReconcileApplyOptions identifies and confirms an exact protected plan artifact.
type ReconcileApplyOptions struct {
	PlanPath         string
	ExpectedPlanHash string
	Confirm          bool
}

// ReadReconcilePlan returns the redacted, hash-verified protected plan without
// authentication, network access, or mutation.
func ReadReconcilePlan(options ReconcileApplyOptions) (ReconcilePlanView, error) {
	plan, err := readExpectedReconcilePlan(options)
	if err != nil {
		return ReconcilePlanView{}, err
	}
	return newReconcilePlanView(plan), nil
}

// ReconcileCertificateView is the selected certificate identity safe for orchestration.
type ReconcileCertificateView struct {
	ResourceID     string `json:"resourceId"`
	SHA256         string `json:"sha256"`
	TeamID         string `json:"teamId"`
	ExpirationDate string `json:"expirationDate"`
}

// ReconcileTargetView identifies a signing target without exposing entitlements.
type ReconcileTargetView struct {
	Kind     string `json:"kind"`
	BundleID string `json:"bundleId"`
}

// ReconcileActionView describes a planned operation without device identifiers.
type ReconcileActionView struct {
	Kind     string `json:"kind"`
	BundleID string `json:"bundleId,omitempty"`
}

// ReconcilePlanView is the redacted orchestration projection of a protected plan.
type ReconcilePlanView struct {
	SchemaVersion       int                       `json:"schemaVersion"`
	PlanHash            string                    `json:"planHash"`
	Ready               bool                      `json:"ready"`
	MutationCount       int                       `json:"mutationCount"`
	MaxMutations        int                       `json:"maxMutations"`
	DeviceCount         int                       `json:"deviceCount"`
	DeviceSetSHA256     string                    `json:"deviceSetSha256"`
	TeamID              string                    `json:"teamId"`
	ArchivePath         string                    `json:"archivePath"`
	DevicesFile         string                    `json:"devicesFile"`
	StateDir            string                    `json:"stateDir"`
	PlanPath            string                    `json:"planPath"`
	ReceiptPath         string                    `json:"receiptPath"`
	ProfilesDir         string                    `json:"profilesDir"`
	MinimumValidityDays int                       `json:"minimumValidityDays"`
	Certificate         *ReconcileCertificateView `json:"certificate,omitempty"`
	Targets             []ReconcileTargetView     `json:"targets"`
	Actions             []ReconcileActionView     `json:"actions"`
	Blockers            []string                  `json:"blockers,omitempty"`
}

// ReconcileProfileView identifies one exact verified local provisioning profile.
type ReconcileProfileView struct {
	TargetKind string `json:"targetKind"`
	BundleID   string `json:"bundleId"`
	ResourceID string `json:"resourceId"`
	UUID       string `json:"uuid"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
}

// ReconcileActionReceiptView reports converged action state without internal IDs.
type ReconcileActionReceiptView struct {
	Kind     string `json:"kind"`
	BundleID string `json:"bundleId,omitempty"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// ReconcileReceiptView is the redacted apply result for orchestration.
type ReconcileReceiptView struct {
	SchemaVersion int                          `json:"schemaVersion"`
	PlanHash      string                       `json:"planHash"`
	Complete      bool                         `json:"complete"`
	ReceiptPath   string                       `json:"receiptPath"`
	MainProfile   *ReconcileProfileView        `json:"mainProfile,omitempty"`
	Profiles      []ReconcileProfileView       `json:"profiles,omitempty"`
	Actions       []ReconcileActionReceiptView `json:"actions,omitempty"`
}

// ReconcileProfileEvidence identifies an exact protected local profile copy
// that may be used instead of the nested reconcile output during resume.
type ReconcileProfileEvidence struct {
	ResourceID string `json:"resourceId"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
}

// ReconcileCompletionEvidence identifies exact protected run-local copies of
// the completed receipt and every profile output. The receipt contents remain
// bound to the nested plan; only the local read authority is overridden.
type ReconcileCompletionEvidence struct {
	ReceiptPath   string                     `json:"receiptPath"`
	ReceiptSHA256 string                     `json:"receiptSha256"`
	Profiles      []ReconcileProfileEvidence `json:"profiles"`
}

// ExecuteReconcilePlan creates a protected plan selected by an exact certificate fingerprint.
func ExecuteReconcilePlan(ctx context.Context, options ReconcilePlanOptions) (ReconcilePlanView, error) {
	if err := validateSigningReconcilePlanFlags(options.ArchivePath, options.DevicesFile, options.MinimumValidityDays, options.MaxMutations, options.StateDir); err != nil {
		return ReconcilePlanView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	fingerprint, err := normalizeReconcileCertificateSHA256(options.CertificateSHA256)
	if err != nil {
		return ReconcilePlanView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	if fingerprint == "" {
		return ReconcilePlanView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	plan, err := executeSigningReconcilePlan(ctx, signingReconcilePlanOptions{
		ArchivePath: options.ArchivePath, DevicesFile: options.DevicesFile,
		CertificateID: options.CertificateID, CertificateSHA256: fingerprint,
		MinimumValidityDays: options.MinimumValidityDays, MaxMutations: options.MaxMutations,
		StateDir: options.StateDir, Overwrite: options.Overwrite,
	})
	if err != nil {
		var invalid reconcilePlanInvalidCause
		if errors.As(err, &invalid) || shared.ClassifyUsageError(err) != "" {
			return ReconcilePlanView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
		}
		return ReconcilePlanView{}, redactedReconcileExecutionError(ReconcileExecutionErrorRetryable)
	}
	return newReconcilePlanView(plan), nil
}

// ExecuteReconcileApply validates and applies one exact plan after explicit confirmation.
func ExecuteReconcileApply(ctx context.Context, options ReconcileApplyOptions) (ReconcileReceiptView, error) {
	if !options.Confirm {
		return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	plan, err := readExpectedReconcilePlan(options)
	if err != nil {
		return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	receipt, applyErr := executeSigningReconcileApplyPlanSilent(ctx, plan)
	if applyErr != nil {
		if errors.Is(applyErr, ErrReconcilePlanDrift) {
			return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanDrift)
		}
		return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorRetryable)
	}
	view, viewErr := newReconcileReceiptView(plan, receipt)
	if viewErr != nil {
		return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanDrift)
	}
	return view, nil
}

// VerifyReconcileCompletionFromEvidence verifies remote completion using an
// immutable archive snapshot and exact protected run-local receipt/profile
// copies. It does not depend on the nested generated receipt/profile paths.
func VerifyReconcileCompletionFromEvidence(ctx context.Context, options ReconcileApplyOptions, archivePath string, evidence ReconcileCompletionEvidence) (ReconcileReceiptView, error) {
	if !canonicalReconcileEvidencePath(archivePath) || !canonicalReconcileEvidencePath(evidence.ReceiptPath) || !canonicalReconcileDigest(evidence.ReceiptSHA256) || len(evidence.Profiles) == 0 {
		return ReconcileReceiptView{}, redactedReconcileExecutionError(ReconcileExecutionErrorPlanInvalid)
	}
	return verifyReconcileCompletionAdapter(ctx, options, archivePath, &evidence)
}

func verifyReconcileCompletionAdapter(ctx context.Context, options ReconcileApplyOptions, archivePath string, evidence *ReconcileCompletionEvidence) (ReconcileReceiptView, error) {
	view, kind, err := verifyReconcileCompletion(ctx, options, archivePath, evidence)
	if err == nil {
		return view, nil
	}
	return ReconcileReceiptView{}, redactedReconcileExecutionError(kind)
}

func verifyReconcileCompletion(ctx context.Context, options ReconcileApplyOptions, archivePath string, evidence *ReconcileCompletionEvidence) (ReconcileReceiptView, ReconcileExecutionErrorKind, error) {
	plan, err := readExpectedReconcilePlan(options)
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanInvalid, err
	}
	if !plan.Ready || len(plan.Blockers) > 0 {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, fmt.Errorf("signing reconcile plan is blocked")
	}
	if err := validateSigningApplyPlan(plan); err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, fmt.Errorf("invalid signing reconcile plan: %w", err)
	}
	if plan.MutationCount > plan.MaxMutations {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, fmt.Errorf("signing reconcile plan exceeds its mutation ceiling")
	}
	if err := verifySigningLocalInputsAtArchive(plan, archivePath); err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, fmt.Errorf("local inputs changed: %w", err)
	}
	receipt, err := readCompleteSigningReceiptForEvidence(plan, evidence)
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, err
	}
	view, err := newReconcileReceiptViewForEvidence(plan, receipt, evidence)
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, err
	}
	client, err := shared.GetASCClient()
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorRetryable, err
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	if err := verifyCurrentReconcileCertificate(requestCtx, client, plan); err != nil {
		kind := ReconcileExecutionErrorPlanDrift
		if isRetryableReconcileFailure(err) {
			kind = ReconcileExecutionErrorRetryable
		}
		return ReconcileReceiptView{}, kind, err
	}
	deviceData, err := readProtectedFile(plan.Paths.DevicesFile)
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, err
	}
	devicesFile, err := decodeSigningDevicesFile(deviceData)
	if err != nil {
		return ReconcileReceiptView{}, ReconcileExecutionErrorPlanDrift, err
	}
	if err := verifyReconcileRemoteCompletion(requestCtx, client, plan, devicesFile, receipt, view); err != nil {
		kind := ReconcileExecutionErrorPlanDrift
		if isRetryableReconcileFailure(err) {
			kind = ReconcileExecutionErrorRetryable
		}
		return ReconcileReceiptView{}, kind, fmt.Errorf("remote signing state changed: %w", err)
	}
	return view, "", nil
}

func canonicalReconcileDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func canonicalReconcileEvidencePath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value
}

func readCompleteSigningReceiptForEvidence(plan signingReconcilePlanArtifact, evidence *ReconcileCompletionEvidence) (signingReconcileReceipt, error) {
	if evidence == nil {
		return readCompleteSigningReceipt(plan)
	}
	data, err := readProtectedFileBounded(filepath.Clean(evidence.ReceiptPath), reconcileProtectedFileMaxBytes)
	if err != nil {
		return signingReconcileReceipt{}, err
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != evidence.ReceiptSHA256 {
		return signingReconcileReceipt{}, fmt.Errorf("completion receipt copy digest differs from expected evidence")
	}
	return decodeCompleteSigningReceipt(plan, data)
}

func newReconcileReceiptViewForEvidence(plan signingReconcilePlanArtifact, receipt signingReconcileReceipt, evidence *ReconcileCompletionEvidence) (ReconcileReceiptView, error) {
	if evidence == nil {
		return newReconcileReceiptView(plan, receipt)
	}
	profiles := make(map[string][]byte, len(evidence.Profiles))
	for _, profile := range evidence.Profiles {
		if strings.TrimSpace(profile.ResourceID) == "" || !canonicalReconcileEvidencePath(profile.Path) || !canonicalReconcileDigest(profile.SHA256) {
			return ReconcileReceiptView{}, fmt.Errorf("profile evidence is incomplete")
		}
		if _, duplicate := profiles[profile.ResourceID]; duplicate {
			return ReconcileReceiptView{}, fmt.Errorf("profile evidence contains duplicate resource IDs")
		}
		data, err := readProtectedFileBounded(profile.Path, reconcileProfileMaxBytes)
		if err != nil {
			return ReconcileReceiptView{}, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != profile.SHA256 {
			return ReconcileReceiptView{}, fmt.Errorf("profile copy digest differs from expected evidence")
		}
		profiles[profile.ResourceID] = data
	}
	return newReconcileReceiptViewWithProfileEvidence(plan, receipt, profiles)
}

func readExpectedReconcilePlan(options ReconcileApplyOptions) (signingReconcilePlanArtifact, error) {
	planPath := strings.TrimSpace(options.PlanPath)
	if planPath == "" {
		return signingReconcilePlanArtifact{}, fmt.Errorf("plan path is required")
	}
	expectedHash, err := normalizeReconcilePlanHash(options.ExpectedPlanHash)
	if err != nil {
		return signingReconcilePlanArtifact{}, err
	}
	plan, err := readSigningPlanArtifactSilent(filepath.Clean(planPath))
	if err != nil {
		return signingReconcilePlanArtifact{}, err
	}
	if plan.PlanHash != expectedHash {
		return signingReconcilePlanArtifact{}, fmt.Errorf("expected plan hash does not match the protected plan artifact")
	}
	return plan, nil
}

func isRetryableReconcileFailure(err error) bool {
	if err == nil {
		return false
	}
	var terminalProfileOutput reconcileTerminalProfileOutputError
	if errors.As(err, &terminalProfileOutput) {
		return false
	}
	var localPersistence reconcileLocalPersistenceError
	if errors.As(err, &localPersistence) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	var statusError interface{ HTTPStatusCode() int }
	if !errors.As(err, &statusError) {
		return false
	}
	status := statusError.HTTPStatusCode()
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func normalizeReconcileCertificateSHA256(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("certificate SHA-256 must be exactly 64 hexadecimal characters")
	}
	return normalized, nil
}

func normalizeReconcilePlanHash(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256.Size || normalized != strings.ToLower(normalized) {
		return "", fmt.Errorf("expected plan hash must be the exact lowercase SHA-256 from planning")
	}
	return normalized, nil
}

func newReconcilePlanView(plan signingReconcilePlanArtifact) ReconcilePlanView {
	view := ReconcilePlanView{
		SchemaVersion: plan.SchemaVersion, PlanHash: plan.PlanHash, Ready: plan.Ready,
		MutationCount: plan.MutationCount, MaxMutations: plan.MaxMutations, DeviceCount: len(plan.Devices),
		DeviceSetSHA256: plan.DeviceSetSHA256, TeamID: plan.TeamID,
		ArchivePath: plan.Paths.ArchivePath, DevicesFile: plan.Paths.DevicesFile, StateDir: plan.Paths.StateDir,
		PlanPath: plan.Paths.PlanPath, ReceiptPath: plan.Paths.ReceiptPath, ProfilesDir: plan.Paths.ProfilesDir,
		MinimumValidityDays: plan.MinimumValidityDays,
	}
	for _, blocker := range plan.Blockers {
		view.Blockers = append(view.Blockers, redactReconcileViewText(blocker, plan.Devices))
	}
	if plan.Certificate != nil {
		view.Certificate = &ReconcileCertificateView{
			ResourceID: plan.Certificate.ID, SHA256: plan.Certificate.SHA256,
			TeamID: plan.Certificate.TeamID, ExpirationDate: plan.Certificate.ExpirationDate,
		}
	}
	for _, target := range plan.Targets {
		view.Targets = append(view.Targets, ReconcileTargetView{Kind: target.Kind, BundleID: target.BundleID})
	}
	for _, action := range plan.Actions {
		view.Actions = append(view.Actions, ReconcileActionView{Kind: action.Kind, BundleID: action.BundleID})
	}
	return view
}

func newReconcileReceiptView(plan signingReconcilePlanArtifact, receipt signingReconcileReceipt) (ReconcileReceiptView, error) {
	return newReconcileReceiptViewWithProfileEvidence(plan, receipt, nil)
}

func newReconcileReceiptViewWithProfileEvidence(plan signingReconcilePlanArtifact, receipt signingReconcileReceipt, profileEvidence map[string][]byte) (ReconcileReceiptView, error) {
	view := ReconcileReceiptView{
		SchemaVersion: receipt.SchemaVersion, PlanHash: plan.PlanHash,
		Complete: receipt.Complete, ReceiptPath: receipt.ReceiptPath,
	}
	if view.SchemaVersion == 0 {
		view.SchemaVersion = signingReconcileSchemaV1
	}
	if view.ReceiptPath == "" {
		view.ReceiptPath = plan.Paths.ReceiptPath
	}
	actionsByID := make(map[string]signingAction, len(plan.Actions))
	for _, action := range plan.Actions {
		actionsByID[action.ID] = action
	}
	profileByBundle := make(map[string]ReconcileProfileView)
	usedEvidence := make(map[string]struct{}, len(profileEvidence))
	for _, item := range receipt.Actions {
		action, ok := actionsByID[item.ID]
		if !ok {
			return view, fmt.Errorf("receipt action is not present in the exact plan")
		}
		if item.Kind != action.Kind {
			return view, fmt.Errorf("receipt action kind differs from the exact plan")
		}
		view.Actions = append(view.Actions, ReconcileActionReceiptView{
			Kind: item.Kind, BundleID: action.BundleID, Status: item.Status,
			Error: redactReconcileViewText(item.Error, plan.Devices),
		})
		if item.Status != "completed" || (action.Kind != actionCreateProfile && action.Kind != actionDownloadProfile) {
			continue
		}
		var profile ReconcileProfileView
		var err error
		if profileEvidence == nil {
			profile, err = reconcileProfileView(plan, action, item)
		} else if content, ok := profileEvidence[item.ResourceID]; ok {
			profile, err = reconcileProfileViewFromContent(plan, action, item, content)
			usedEvidence[item.ResourceID] = struct{}{}
		} else {
			err = fmt.Errorf("profile action has no exact evidence copy")
		}
		if err != nil {
			return view, err
		}
		if _, exists := profileByBundle[action.BundleID]; exists {
			return view, fmt.Errorf("multiple completed profiles exist for bundle %s", action.BundleID)
		}
		profileByBundle[action.BundleID] = profile
	}
	for _, target := range plan.Targets {
		profile, ok := profileByBundle[target.BundleID]
		if !ok {
			if receipt.Complete {
				return view, fmt.Errorf("completed receipt is missing profile for bundle %s", target.BundleID)
			}
			continue
		}
		view.Profiles = append(view.Profiles, profile)
		if target.Kind == "application" {
			if view.MainProfile != nil {
				return view, fmt.Errorf("completed receipt contains multiple main application profiles")
			}
			copy := profile
			view.MainProfile = &copy
		}
	}
	if receipt.Complete && view.MainProfile == nil {
		return view, fmt.Errorf("completed receipt is missing the main application profile")
	}
	if profileEvidence != nil && len(usedEvidence) != len(profileEvidence) {
		return view, fmt.Errorf("profile evidence contains resources outside the exact receipt")
	}
	return view, nil
}

func reconcileProfileView(plan signingReconcilePlanArtifact, action signingAction, item signingActionReceipt) (ReconcileProfileView, error) {
	profilesDir, outputPath, relative, err := reconcileProfileOutputPath(plan, action, item)
	if err != nil {
		return ReconcileProfileView{}, err
	}
	root, err := rootfs.New(profilesDir)
	if err != nil {
		return ReconcileProfileView{}, err
	}
	defer root.Close()
	file, err := root.OpenFile(relative)
	if err != nil {
		return ReconcileProfileView{}, fmt.Errorf("open verified profile: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ReconcileProfileView{}, err
	}
	if info.Size() > reconcileProfileMaxBytes {
		return ReconcileProfileView{}, fmt.Errorf("verified profile exceeds %d bytes", reconcileProfileMaxBytes)
	}
	content, err := io.ReadAll(io.LimitReader(file, reconcileProfileMaxBytes+1))
	if err != nil {
		return ReconcileProfileView{}, err
	}
	if len(content) > reconcileProfileMaxBytes {
		return ReconcileProfileView{}, fmt.Errorf("verified profile exceeds %d bytes", reconcileProfileMaxBytes)
	}
	return reconcileProfileViewFromValidatedContent(plan, action, item, outputPath, relative, content)
}

func reconcileProfileViewFromContent(plan signingReconcilePlanArtifact, action signingAction, item signingActionReceipt, content []byte) (ReconcileProfileView, error) {
	_, outputPath, relative, err := reconcileProfileOutputPath(plan, action, item)
	if err != nil {
		return ReconcileProfileView{}, err
	}
	return reconcileProfileViewFromValidatedContent(plan, action, item, outputPath, relative, content)
}

func reconcileProfileOutputPath(plan signingReconcilePlanArtifact, action signingAction, item signingActionReceipt) (string, string, string, error) {
	if strings.TrimSpace(item.ResourceID) == "" {
		return "", "", "", fmt.Errorf("verified profile resource ID is missing")
	}
	if action.Kind == actionDownloadProfile && item.ResourceID != action.ProfileID {
		return "", "", "", fmt.Errorf("downloaded profile resource ID differs from the exact plan")
	}
	profilesDir, err := filepath.Abs(filepath.Clean(plan.Paths.ProfilesDir))
	if err != nil {
		return "", "", "", err
	}
	outputPath, err := filepath.Abs(filepath.Clean(item.OutputPath))
	if err != nil {
		return "", "", "", err
	}
	relative, err := filepath.Rel(profilesDir, outputPath)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.Dir(relative) != "." {
		return "", "", "", fmt.Errorf("profile output is outside the planned profiles directory")
	}
	return profilesDir, outputPath, relative, nil
}

func reconcileProfileViewFromValidatedContent(plan signingReconcilePlanArtifact, action signingAction, item signingActionReceipt, outputPath, relative string, content []byte) (ReconcileProfileView, error) {
	parsed, err := decodeReconcileMobileProvision(content)
	if err != nil {
		return ReconcileProfileView{}, fmt.Errorf("decode verified profile: %w", err)
	}
	if filepath.Base(relative) != parsed.UUID+".mobileprovision" {
		return ReconcileProfileView{}, fmt.Errorf("verified profile UUID does not match its output path")
	}
	if !safeProfileUUID(parsed.UUID) || !parsed.ExpirationDate.After(time.Now().Add(time.Duration(plan.MinimumValidityDays)*24*time.Hour)) {
		return ReconcileProfileView{}, fmt.Errorf("verified profile no longer satisfies the exact plan validity")
	}
	target, ok := targetByBundleID(plan.Targets, action.BundleID)
	if !ok {
		return ReconcileProfileView{}, fmt.Errorf("profile target is missing from the exact plan")
	}
	if plan.Certificate == nil || !entitlementsContain(parsed.Entitlements, target.Entitlements) || !mobileProvisionContainsCertificate(parsed, plan.Certificate.SHA256) {
		return ReconcileProfileView{}, fmt.Errorf("verified profile no longer matches the exact plan")
	}
	provisioned := make(map[string]struct{}, len(parsed.ProvisionedDevices))
	for _, udid := range parsed.ProvisionedDevices {
		provisioned[fingerprintDevice(normalizeReconcileUDID(udid))] = struct{}{}
	}
	if len(provisioned) != len(plan.Devices) {
		return ReconcileProfileView{}, fmt.Errorf("verified profile device set no longer matches the exact plan")
	}
	for _, device := range plan.Devices {
		if _, ok := provisioned[device.Fingerprint]; !ok {
			return ReconcileProfileView{}, fmt.Errorf("verified profile device set no longer matches the exact plan")
		}
	}
	digest := sha256.Sum256(content)
	return ReconcileProfileView{
		TargetKind: target.Kind, BundleID: target.BundleID, ResourceID: item.ResourceID,
		UUID: parsed.UUID, Path: outputPath, SHA256: hex.EncodeToString(digest[:]),
	}, nil
}

func redactReconcileViewText(value string, devices []signingDesiredDevice) string {
	for _, device := range devices {
		for _, sensitive := range []string{device.Fingerprint, device.NameSHA256, device.ResourceID} {
			if sensitive != "" {
				value = strings.ReplaceAll(value, sensitive, "[redacted-device]")
			}
		}
	}
	return value
}
