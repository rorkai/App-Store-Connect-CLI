package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func executeSigningReconcileApply(ctx context.Context, planPath string) (signingReconcileReceipt, error) {
	plan, err := readSigningPlanArtifact(filepath.Clean(strings.TrimSpace(planPath)))
	if err != nil {
		return signingReconcileReceipt{}, err
	}
	return executeSigningReconcileApplyPlan(ctx, plan)
}

func executeSigningReconcileApplyPlan(ctx context.Context, plan signingReconcilePlanArtifact) (signingReconcileReceipt, error) {
	return executeSigningReconcileApplyPlanWithUsage(ctx, plan, shared.UsageError)
}

func executeSigningReconcileApplyPlanSilent(ctx context.Context, plan signingReconcilePlanArtifact) (signingReconcileReceipt, error) {
	return executeSigningReconcileApplyPlanWithUsage(ctx, plan, func(message string) error {
		return errors.New(message)
	})
}

func executeSigningReconcileApplyPlanWithUsage(ctx context.Context, plan signingReconcilePlanArtifact, usageError func(string) error) (signingReconcileReceipt, error) {
	if !plan.Ready || len(plan.Blockers) > 0 {
		return signingReconcileReceipt{}, newReconcilePlanDrift(usageError("signing reconcile plan is blocked; rerun plan after resolving blockers"))
	}
	if err := validateSigningApplyPlan(plan); err != nil {
		return signingReconcileReceipt{}, newReconcilePlanDrift(usageError(fmt.Sprintf("invalid signing reconcile plan: %v", err)))
	}
	if plan.MutationCount > plan.MaxMutations {
		return signingReconcileReceipt{}, newReconcilePlanDrift(usageError("signing reconcile plan exceeds its mutation ceiling"))
	}
	devicesFile, err := verifySigningLocalInputs(plan)
	if err != nil {
		return signingReconcileReceipt{}, newReconcilePlanDrift(usageError(fmt.Sprintf("local inputs changed: %v; rerun asc signing reconcile plan", err)))
	}
	if err := prepareReconcileProfileOutput(plan.Paths.StateDir); err != nil {
		return signingReconcileReceipt{}, usageError(fmt.Sprintf("invalid profile output directory: %v", err))
	}

	receipt, err := loadOrStartSigningReceiptWithUsage(plan, usageError)
	if err != nil {
		return signingReconcileReceipt{}, err
	}
	client, err := shared.GetASCClient()
	if err != nil {
		return receipt, err
	}
	requestCtx, cancel := signingRequestContext(ctx)
	defer cancel()
	if err := verifyCurrentReconcileCertificate(requestCtx, client, plan); err != nil {
		failure := usageError(fmt.Sprintf("%v; rerun asc signing reconcile plan", err))
		if !isRetryableReconcileFailure(err) {
			failure = newReconcilePlanDrift(failure)
		}
		return receipt, failure
	}
	if err := preflightSigningApplySanitized(requestCtx, client, plan, devicesFile); err != nil {
		failure := usageError(fmt.Sprintf("remote signing state changed: %v; rerun asc signing reconcile plan", err))
		if !isRetryableReconcileFailure(err) {
			failure = newReconcilePlanDrift(failure)
		}
		return receipt, failure
	}
	createdProfiles := make(map[string]string)

	for _, action := range plan.Actions {
		item := signingActionReceipt{ID: action.ID, Kind: action.Kind, Status: "running"}
		receipt.Actions = append(receipt.Actions, item)
		index := len(receipt.Actions) - 1
		if err := persistSigningReceipt(&receipt); err != nil {
			return receipt, err
		}

		resourceID, outputPath, actionErr := applySigningAction(requestCtx, client, plan, devicesFile, action, createdProfiles)
		receipt.Actions[index].ResourceID = resourceID
		receipt.Actions[index].OutputPath = outputPath
		if actionErr != nil {
			retryable := isRetryableReconcileFailure(actionErr)
			actionErr = sanitizeReconcileError(actionErr, devicesFile)
			receipt.Actions[index].Status = "failed"
			receipt.Actions[index].Error = actionErr.Error()
			_ = persistSigningReceipt(&receipt)
			failure := fmt.Errorf("action %s: %w", action.ID, actionErr)
			if !retryable {
				failure = newReconcilePlanDrift(failure)
			}
			return receipt, failure
		}
		receipt.Actions[index].Status = "completed"
		if action.Kind == actionCreateProfile {
			createdProfiles[action.BundleID] = resourceID
		}
		if err := persistSigningReceipt(&receipt); err != nil {
			return receipt, err
		}
	}
	receipt.Complete = true
	if err := persistSigningReceipt(&receipt); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func verifyCurrentReconcileCertificate(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact) error {
	certificates, err := getAllReconcileCertificates(ctx, client)
	if err != nil {
		return fmt.Errorf("reread certificates: %w", err)
	}
	selectedCertificate, blockers := selectReconcileCertificateWithFingerprint(certificates, plan.Certificate.ID, plan.Certificate.SHA256, time.Now(), plan.MinimumValidityDays)
	if len(blockers) > 0 || selectedCertificate == nil || !reflect.DeepEqual(*selectedCertificate, *plan.Certificate) {
		return fmt.Errorf("selected certificate changed or is no longer eligible")
	}
	return nil
}

func preflightSigningApply(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile) error {
	remoteDevices, err := getAllReconcileDevices(ctx, client)
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}
	resolved, deviceActions, blockers := planDesiredDevices(devicesFile.Devices, remoteDevices)
	if len(blockers) > 0 {
		return fmt.Errorf("device preconditions blocked: %s", strings.Join(blockers, "; "))
	}
	plannedMutations := make(map[string]string)
	for _, action := range plan.Actions {
		switch action.Kind {
		case actionRegisterDevice, actionCreateBundleID, actionCreateProfile:
			plannedMutations[action.ID] = action.Kind
		}
	}
	for _, action := range deviceActions {
		if plannedMutations[action.ID] != action.Kind {
			return fmt.Errorf("devices now require unplanned %s", action.Kind)
		}
	}
	for _, target := range plan.Targets {
		_, actions, targetBlockers, err := planSigningTarget(ctx, client, target, resolved, plan.Certificate, plan.MinimumValidityDays)
		if err != nil {
			return fmt.Errorf("bundle %s: %w", target.BundleID, err)
		}
		if len(targetBlockers) > 0 {
			return fmt.Errorf("bundle %s blocked: %s", target.BundleID, strings.Join(targetBlockers, "; "))
		}
		for _, action := range actions {
			switch action.Kind {
			case actionRegisterDevice, actionCreateBundleID, actionCreateProfile:
				if plannedMutations[action.ID] != action.Kind {
					return fmt.Errorf("bundle %s now requires unplanned %s", target.BundleID, action.Kind)
				}
			}
		}
	}
	for _, action := range plan.Actions {
		if action.Kind != actionDownloadProfile {
			continue
		}
		target, ok := targetByBundleID(plan.Targets, action.BundleID)
		if !ok {
			return fmt.Errorf("download target %s is missing", action.BundleID)
		}
		if _, _, err := verifyReconcileProfile(ctx, client, action.ProfileID, plan, devicesFile, target); err != nil {
			return fmt.Errorf("planned profile %s changed: %w", action.ProfileID, err)
		}
	}
	return nil
}

func preflightSigningApplySanitized(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile) error {
	return sanitizeReconcileError(preflightSigningApply(ctx, client, plan, devicesFile), devicesFile)
}

func validateSigningApplyPlan(plan signingReconcilePlanArtifact) error {
	if plan.Paths.ReceiptPath != filepath.Join(plan.Paths.StateDir, "receipt.json") {
		return fmt.Errorf("receipt path must be the state directory receipt.json")
	}
	if plan.Certificate == nil || strings.TrimSpace(plan.Certificate.ID) == "" {
		return fmt.Errorf("selected certificate is missing")
	}
	digest, err := hex.DecodeString(strings.TrimSpace(plan.Certificate.SHA256))
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("selected certificate SHA-256 is invalid")
	}
	deviceSetDigest, err := hex.DecodeString(strings.TrimSpace(plan.DeviceSetSHA256))
	if err != nil || len(deviceSetDigest) != sha256.Size {
		return fmt.Errorf("desired device set SHA-256 is invalid")
	}
	if plan.MaxMutations < 1 || plan.MutationCount < 0 {
		return fmt.Errorf("mutation limits are invalid")
	}
	if plan.MinimumValidityDays < 0 || plan.MinimumValidityDays > reconcileMaximumValidityDays {
		return fmt.Errorf("minimum validity days are invalid")
	}
	mutations := 0
	seenActions := make(map[string]struct{}, len(plan.Actions))
	for _, action := range plan.Actions {
		if strings.TrimSpace(action.ID) == "" {
			return fmt.Errorf("action ID is missing")
		}
		if _, exists := seenActions[action.ID]; exists {
			return fmt.Errorf("duplicate action ID %q", action.ID)
		}
		seenActions[action.ID] = struct{}{}
		switch action.Kind {
		case actionRegisterDevice:
			if action.ID != "device:"+action.DeviceFingerprint || !planContainsDevice(plan, action.DeviceFingerprint) {
				return fmt.Errorf("register-device action differs from desired devices")
			}
			mutations++
		case actionCreateBundleID:
			if action.ID != "bundle:"+action.BundleID || !planContainsTarget(plan, action.BundleID) {
				return fmt.Errorf("create-bundle action differs from targets")
			}
			mutations++
		case actionCreateProfile:
			if action.ID != "profile:"+action.BundleID || !planContainsTarget(plan, action.BundleID) || action.CertificateID != plan.Certificate.ID {
				return fmt.Errorf("create-profile action differs from targets or certificate")
			}
			mutations++
		case actionDownloadProfile:
			if action.ID != "download:"+action.BundleID || !planContainsTarget(plan, action.BundleID) || strings.TrimSpace(action.ProfileID) == "" {
				return fmt.Errorf("download-profile action differs from targets")
			}
		default:
			return fmt.Errorf("unsupported action kind %q", action.Kind)
		}
	}
	if mutations != plan.MutationCount {
		return fmt.Errorf("mutation count differs from actions")
	}
	return nil
}

func planContainsTarget(plan signingReconcilePlanArtifact, bundleID string) bool {
	_, ok := targetByBundleID(plan.Targets, bundleID)
	return ok
}

func planContainsDevice(plan signingReconcilePlanArtifact, fingerprint string) bool {
	for _, device := range plan.Devices {
		if device.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func verifySigningLocalInputs(plan signingReconcilePlanArtifact) (signingDevicesFile, error) {
	return verifySigningLocalInputsAtArchiveWithDevices(plan, "")
}

func verifySigningLocalInputsAtArchive(plan signingReconcilePlanArtifact, archivePath string) error {
	_, err := verifySigningLocalInputsAtArchiveWithDevices(plan, archivePath)
	return err
}

func verifySigningLocalInputsAtArchiveWithDevices(plan signingReconcilePlanArtifact, archivePath string) (signingDevicesFile, error) {
	data, err := readProtectedFile(plan.Paths.DevicesFile)
	if err != nil {
		return signingDevicesFile{}, fmt.Errorf("invalid devices file: %s", protectedInputDiagnostic(err))
	}
	devices, err := decodeSigningDevicesFile(data)
	if err != nil {
		return signingDevicesFile{}, fmt.Errorf("invalid devices file: %s", invalidDevicesFileDiagnostic(err))
	}
	if archivePath == "" {
		archivePath = plan.Paths.ArchivePath
	}
	archive, err := readSigningArchiveRequirements(archivePath)
	if err != nil {
		return signingDevicesFile{}, fmt.Errorf("inspect archive: %w", sanitizeReconcileError(err, devices))
	}
	if archive.TeamID != plan.TeamID || !signingTargetsEqual(archive.Targets, plan.Targets) {
		return signingDevicesFile{}, fmt.Errorf("archive signing requirements differ from plan")
	}
	if digestSigningDeviceInputs(devices.Devices).SHA256 != plan.DeviceSetSHA256 {
		return signingDevicesFile{}, fmt.Errorf("desired device set digest differs from plan")
	}
	if len(devices.Devices) != len(plan.Devices) {
		return signingDevicesFile{}, fmt.Errorf("desired device count differs from plan")
	}
	for index, device := range devices.Devices {
		planned := plan.Devices[index]
		if fingerprintReconcileName(device.Name) != planned.NameSHA256 || device.Platform != planned.Platform || device.Fingerprint != planned.Fingerprint {
			return signingDevicesFile{}, fmt.Errorf("desired device %s differs from plan", device.Fingerprint)
		}
	}
	return devices, nil
}

func signingTargetsEqual(left, right []signingTarget) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		leftTarget := left[index]
		rightTarget := right[index]
		leftEntitlements := leftTarget.Entitlements
		rightEntitlements := rightTarget.Entitlements
		leftTarget.Entitlements = nil
		rightTarget.Entitlements = nil
		if !reflect.DeepEqual(leftTarget, rightTarget) || !signingValuesEqual(leftEntitlements, rightEntitlements) {
			return false
		}
	}
	return true
}

func signingValuesEqual(left, right any) bool {
	leftNumber, leftIsNumber := exactSigningNumber(left)
	rightNumber, rightIsNumber := exactSigningNumber(right)
	if leftIsNumber || rightIsNumber {
		return leftIsNumber && rightIsNumber && leftNumber.Cmp(rightNumber) == 0
	}

	switch leftValue := left.(type) {
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, value := range leftValue {
			other, exists := rightValue[key]
			if !exists || !signingValuesEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for index := range leftValue {
			if !signingValuesEqual(leftValue[index], rightValue[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left, right)
	}
}

func exactSigningNumber(value any) (*big.Rat, bool) {
	integer := new(big.Int)
	switch typed := value.(type) {
	case json.Number:
		number, ok := new(big.Rat).SetString(string(typed))
		return number, ok
	case int:
		return new(big.Rat).SetInt64(int64(typed)), true
	case int8:
		return new(big.Rat).SetInt64(int64(typed)), true
	case int16:
		return new(big.Rat).SetInt64(int64(typed)), true
	case int32:
		return new(big.Rat).SetInt64(int64(typed)), true
	case int64:
		return new(big.Rat).SetInt64(typed), true
	case uint:
		integer.SetUint64(uint64(typed))
	case uint8:
		integer.SetUint64(uint64(typed))
	case uint16:
		integer.SetUint64(uint64(typed))
	case uint32:
		integer.SetUint64(uint64(typed))
	case uint64:
		integer.SetUint64(typed)
	case float32:
		return exactSigningFloat(float64(typed))
	case float64:
		return exactSigningFloat(typed)
	default:
		return nil, false
	}
	return new(big.Rat).SetInt(integer), true
}

func exactSigningFloat(value float64) (*big.Rat, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, false
	}
	const maximumSafeJSONInteger = float64(1<<53 - 1)
	if math.Trunc(value) == value && math.Abs(value) > maximumSafeJSONInteger {
		return nil, false
	}
	number := new(big.Rat).SetFloat64(value)
	return number, number != nil
}

func loadOrStartSigningReceipt(plan signingReconcilePlanArtifact) (signingReconcileReceipt, error) {
	return loadOrStartSigningReceiptWithUsage(plan, shared.UsageError)
}

func loadOrStartSigningReceiptWithUsage(plan signingReconcilePlanArtifact, usageError func(string) error) (signingReconcileReceipt, error) {
	receiptPath := filepath.Join(plan.Paths.StateDir, "receipt.json")
	data, err := readProtectedFile(receiptPath)
	if err == nil {
		var receipt signingReconcileReceipt
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&receipt); err != nil {
			return receipt, newReconcilePlanDrift(usageError("existing signing receipt contains invalid JSON"))
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return receipt, newReconcilePlanDrift(usageError("existing signing receipt contains invalid JSON"))
		}
		if receipt.SchemaVersion != signingReconcileSchemaV1 || receipt.PlanHash != plan.PlanHash ||
			filepath.Clean(receipt.StateDir) != filepath.Clean(plan.Paths.StateDir) ||
			filepath.Clean(receipt.ReceiptPath) != filepath.Clean(plan.Paths.ReceiptPath) {
			return receipt, newReconcilePlanDrift(usageError("existing receipt belongs to a different plan; move it aside before apply"))
		}
		// Receipt state is a recovery hint, never proof of a durable outcome. A
		// resumed apply reruns every idempotent ensure/verification action so
		// remote drift and missing/corrupt local profiles are repaired or blocked.
		receipt.Actions = nil
		receipt.Complete = false
		receipt.StateDir = plan.Paths.StateDir
		receipt.ReceiptPath = receiptPath
		return receipt, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return signingReconcileReceipt{}, newReconcilePlanDrift(usageError("existing signing receipt could not be read safely"))
	}
	now := nowRFC3339()
	return signingReconcileReceipt{
		SchemaVersion: signingReconcileSchemaV1, PlanHash: plan.PlanHash,
		StartedAt: now, UpdatedAt: now, StateDir: plan.Paths.StateDir,
		ReceiptPath: receiptPath,
	}, nil
}

func readCompleteSigningReceipt(plan signingReconcilePlanArtifact) (signingReconcileReceipt, error) {
	data, err := readProtectedFile(plan.Paths.ReceiptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return signingReconcileReceipt{}, fmt.Errorf("completed signing reconcile receipt is missing")
		}
		return signingReconcileReceipt{}, err
	}
	return decodeCompleteSigningReceipt(plan, data)
}

func decodeCompleteSigningReceipt(plan signingReconcilePlanArtifact, data []byte) (signingReconcileReceipt, error) {
	var receipt signingReconcileReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return receipt, fmt.Errorf("decode receipt: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return receipt, fmt.Errorf("decode receipt: %w", err)
	}
	if receipt.SchemaVersion != signingReconcileSchemaV1 || receipt.PlanHash != plan.PlanHash {
		return receipt, fmt.Errorf("completed receipt belongs to a different plan")
	}
	if !receipt.Complete {
		return receipt, fmt.Errorf("signing reconcile receipt is not complete")
	}
	if filepath.Clean(receipt.StateDir) != filepath.Clean(plan.Paths.StateDir) || filepath.Clean(receipt.ReceiptPath) != filepath.Clean(plan.Paths.ReceiptPath) {
		return receipt, fmt.Errorf("completed receipt paths differ from the exact plan")
	}
	if len(receipt.Actions) != len(plan.Actions) {
		return receipt, fmt.Errorf("completed receipt action count differs from the exact plan")
	}
	planned := make(map[string]signingAction, len(plan.Actions))
	for _, action := range plan.Actions {
		planned[action.ID] = action
	}
	seen := make(map[string]struct{}, len(receipt.Actions))
	for _, item := range receipt.Actions {
		action, ok := planned[item.ID]
		if !ok || item.Kind != action.Kind {
			return receipt, fmt.Errorf("completed receipt action differs from the exact plan")
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return receipt, fmt.Errorf("completed receipt contains duplicate action %s", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Status != "completed" || item.Error != "" || strings.TrimSpace(item.ResourceID) == "" {
			return receipt, fmt.Errorf("receipt action %s is not durably complete", item.ID)
		}
		if (action.Kind == actionCreateProfile || action.Kind == actionDownloadProfile) && strings.TrimSpace(item.OutputPath) == "" {
			return receipt, fmt.Errorf("receipt profile action %s has no verified output", item.ID)
		}
	}
	return receipt, nil
}

func verifyReconcileRemoteCompletion(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile, receipt signingReconcileReceipt, view ReconcileReceiptView) error {
	remoteDevices, err := getAllReconcileDevices(ctx, client)
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}
	resolved, pendingDeviceActions, deviceBlockers := planDesiredDevices(devicesFile.Devices, remoteDevices)
	if len(deviceBlockers) > 0 {
		return fmt.Errorf("device preconditions blocked: %s", strings.Join(deviceBlockers, "; "))
	}
	if len(pendingDeviceActions) > 0 {
		return fmt.Errorf("completed receipt has missing devices")
	}
	resolvedByFingerprint := make(map[string]signingDesiredDevice, len(resolved))
	for _, device := range resolved {
		resolvedByFingerprint[device.Fingerprint] = device
	}
	receiptsByID := make(map[string]signingActionReceipt, len(receipt.Actions))
	for _, item := range receipt.Actions {
		receiptsByID[item.ID] = item
	}
	for _, plannedDevice := range plan.Devices {
		current, ok := resolvedByFingerprint[plannedDevice.Fingerprint]
		if !ok || (plannedDevice.ResourceID != "" && current.ResourceID != plannedDevice.ResourceID) {
			return fmt.Errorf("desired device resource changed")
		}
	}
	for _, action := range plan.Actions {
		if action.Kind == actionRegisterDevice && receiptsByID[action.ID].ResourceID != resolvedByFingerprint[action.DeviceFingerprint].ResourceID {
			return fmt.Errorf("registered device resource differs from completed receipt")
		}
	}

	profilesByResourceID := make(map[string]ReconcileProfileView, len(view.Profiles))
	for _, profile := range view.Profiles {
		profilesByResourceID[profile.ResourceID] = profile
	}
	for _, target := range plan.Targets {
		observed, currentActions, blockers, err := planSigningTarget(ctx, client, target, resolved, plan.Certificate, plan.MinimumValidityDays)
		if err != nil {
			return fmt.Errorf("bundle %s: %w", target.BundleID, err)
		}
		if len(blockers) > 0 {
			return fmt.Errorf("bundle %s blocked: %s", target.BundleID, strings.Join(blockers, "; "))
		}
		if observed.ResourceID == "" {
			return fmt.Errorf("bundle %s is missing", target.BundleID)
		}
		for _, currentAction := range currentActions {
			if currentAction.Kind == actionRegisterDevice || currentAction.Kind == actionCreateBundleID || currentAction.Kind == actionCreateProfile {
				return fmt.Errorf("bundle %s requires mutation %s", target.BundleID, currentAction.Kind)
			}
		}
		for _, plannedObserved := range plan.ObservedBundles {
			if plannedObserved.BundleID == target.BundleID && plannedObserved.ResourceID != "" && plannedObserved.ResourceID != observed.ResourceID {
				return fmt.Errorf("bundle %s resource changed", target.BundleID)
			}
		}
		for _, action := range plan.Actions {
			if action.BundleID != target.BundleID {
				continue
			}
			item := receiptsByID[action.ID]
			switch action.Kind {
			case actionCreateBundleID:
				if item.ResourceID != observed.ResourceID {
					return fmt.Errorf("bundle %s resource differs from completed receipt", target.BundleID)
				}
			case actionCreateProfile, actionDownloadProfile:
				profile, content, err := verifyReconcileProfile(ctx, client, item.ResourceID, plan, devicesFile, target)
				if err != nil {
					return fmt.Errorf("profile %s: %w", item.ResourceID, err)
				}
				bundle, err := client.GetProfileBundleID(ctx, item.ResourceID)
				if err != nil {
					return fmt.Errorf("profile %s bundle relationship: %w", item.ResourceID, err)
				}
				if bundle.Data.ID != observed.ResourceID || bundle.Data.Attributes.Identifier != target.BundleID {
					return fmt.Errorf("profile %s bundle relationship differs from plan", item.ResourceID)
				}
				local, ok := profilesByResourceID[item.ResourceID]
				remoteDigest := sha256.Sum256(content)
				if profile.ID != item.ResourceID || !ok || local.SHA256 != hex.EncodeToString(remoteDigest[:]) {
					return fmt.Errorf("profile %s content differs from verified local output", item.ResourceID)
				}
			}
		}
	}
	return nil
}

func persistSigningReceipt(receipt *signingReconcileReceipt) error {
	receipt.UpdatedAt = nowRFC3339()
	return writeSigningStateJSON(receipt.StateDir, "receipt.json", *receipt, true)
}

func sanitizeReconcileError(err error, devices signingDevicesFile) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	for _, device := range devices.Devices {
		secrets := []string{
			device.UDID,
			normalizeReconcileUDID(device.UDID),
			url.QueryEscape(device.UDID),
			url.PathEscape(device.UDID),
			device.Name,
			url.QueryEscape(device.Name),
			url.PathEscape(device.Name),
		}
		for _, secret := range uniqueSortedStrings(secrets) {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[redacted]")
			}
		}
	}
	if message == err.Error() {
		return err
	}
	return sanitizedReconcileError{message: message, cause: err}
}

type sanitizedReconcileError struct {
	message string
	cause   error
}

func (e sanitizedReconcileError) Error() string { return e.message }
func (e sanitizedReconcileError) Unwrap() error { return e.cause }

var writeReconcileVerifiedProfile = writeVerifiedProfile

type reconcileTerminalProfileOutputError struct{ err error }

func (err reconcileTerminalProfileOutputError) Error() string { return err.err.Error() }
func (err reconcileTerminalProfileOutputError) Unwrap() error { return err.err }

type reconcileLocalPersistenceError struct{ err error }

func (err reconcileLocalPersistenceError) Error() string { return err.err.Error() }
func (err reconcileLocalPersistenceError) Unwrap() error { return err.err }

func persistReconcileVerifiedProfile(stateDir string, content []byte) (string, error) {
	output, err := writeReconcileVerifiedProfile(stateDir, content)
	if err == nil {
		return output, nil
	}
	var terminal reconcileTerminalProfileOutputError
	if errors.As(err, &terminal) {
		return "", err
	}
	return "", reconcileLocalPersistenceError{err: err}
}

func applySigningAction(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile, action signingAction, createdProfiles map[string]string) (string, string, error) {
	switch action.Kind {
	case actionRegisterDevice:
		input, ok := desiredDeviceInput(devicesFile, action.DeviceFingerprint)
		if !ok {
			return "", "", fmt.Errorf("device input is missing")
		}
		resource, err := ensureReconcileDevice(ctx, client, input)
		if err != nil {
			return "", "", err
		}
		return resource.ID, "", nil
	case actionCreateBundleID:
		bundle, err := ensureReconcileBundleID(ctx, client, action.BundleID)
		if err != nil {
			return "", "", err
		}
		target, ok := targetByBundleID(plan.Targets, action.BundleID)
		if !ok {
			return "", "", fmt.Errorf("target is missing from plan")
		}
		if err := validateReconcileBundleSeed(*bundle, target); err != nil {
			return "", "", err
		}
		return bundle.ID, "", nil
	case actionCreateProfile:
		target, ok := targetByBundleID(plan.Targets, action.BundleID)
		if !ok {
			return "", "", fmt.Errorf("target is missing from plan")
		}
		profile, content, err := ensureReconcileProfile(ctx, client, plan, devicesFile, action, target)
		if err != nil {
			return "", "", err
		}
		output, err := persistReconcileVerifiedProfile(plan.Paths.StateDir, content)
		if err != nil {
			return profile.ID, "", err
		}
		return profile.ID, output, nil
	case actionDownloadProfile:
		profileID := action.ProfileID
		if created := createdProfiles[action.BundleID]; created != "" {
			profileID = created
		}
		target, ok := targetByBundleID(plan.Targets, action.BundleID)
		if !ok {
			return "", "", fmt.Errorf("target is missing from plan")
		}
		profile, content, err := verifyReconcileProfile(ctx, client, profileID, plan, devicesFile, target)
		if err != nil {
			return "", "", err
		}
		output, err := persistReconcileVerifiedProfile(plan.Paths.StateDir, content)
		if err != nil {
			return profile.ID, "", err
		}
		return profile.ID, output, nil
	default:
		return "", "", fmt.Errorf("unsupported planned action kind %q", action.Kind)
	}
}

func desiredDeviceInput(devices signingDevicesFile, fingerprint string) (signingDeviceInput, bool) {
	for _, device := range devices.Devices {
		if device.Fingerprint == fingerprint {
			return device, true
		}
	}
	return signingDeviceInput{}, false
}

func ensureReconcileDevice(ctx context.Context, client *asc.Client, input signingDeviceInput) (*asc.Resource[asc.DeviceAttributes], error) {
	find := func() (*asc.Resource[asc.DeviceAttributes], error) {
		devices, err := getAllReconcileDevices(ctx, client)
		if err != nil {
			return nil, err
		}
		var enabled, disabled []asc.Resource[asc.DeviceAttributes]
		for _, device := range devices {
			if normalizeReconcileUDID(device.Attributes.UDID) != normalizeReconcileUDID(input.UDID) {
				continue
			}
			if device.Attributes.Status == asc.DeviceStatusEnabled {
				enabled = append(enabled, device)
			} else {
				disabled = append(disabled, device)
			}
		}
		if len(enabled) == 1 {
			return &enabled[0], nil
		}
		if len(enabled) > 1 {
			return nil, fmt.Errorf("device %s resolves to multiple enabled resources", input.Fingerprint)
		}
		if len(disabled) > 0 {
			return nil, fmt.Errorf("device %s is disabled; refusing PATCH", input.Fingerprint)
		}
		return nil, nil
	}
	if existing, err := find(); err != nil || existing != nil {
		return existing, err
	}
	created, err := client.CreateDevice(ctx, asc.DeviceCreateAttributes{Name: input.Name, UDID: input.UDID, Platform: asc.DevicePlatformIOS})
	if err == nil {
		return &created.Data, nil
	}
	// Resolve conflict/uncertain completion by exact reread, never blind retry.
	if existing, readErr := find(); readErr == nil && existing != nil {
		return existing, nil
	}
	return nil, err
}

func ensureReconcileBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.Resource[asc.BundleIDAttributes], error) {
	if existing, err := findExactReconcileBundleID(ctx, client, identifier); err != nil || existing != nil {
		return existing, err
	}
	created, err := client.CreateBundleID(ctx, asc.BundleIDCreateAttributes{Name: "ASC " + identifier, Identifier: identifier, Platform: asc.BundleIDPlatformIOS})
	if err == nil {
		return &created.Data, nil
	}
	if existing, readErr := findExactReconcileBundleID(ctx, client, identifier); readErr == nil && existing != nil {
		return existing, nil
	}
	return nil, err
}

func ensureReconcileProfile(ctx context.Context, client *asc.Client, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile, action signingAction, target signingTarget) (*asc.Resource[asc.ProfileAttributes], []byte, error) {
	bundle, err := findExactReconcileBundleID(ctx, client, action.BundleID)
	if err != nil || bundle == nil {
		if err == nil {
			err = fmt.Errorf("bundle ID is missing after planned actions")
		}
		return nil, nil, err
	}
	if bundle.Attributes.Platform != asc.BundleIDPlatformIOS && bundle.Attributes.Platform != asc.BundleIDPlatformUniversal {
		return nil, nil, fmt.Errorf("bundle ID has incompatible platform %s", bundle.Attributes.Platform)
	}
	if err := validateReconcileBundleSeed(*bundle, target); err != nil {
		return nil, nil, err
	}
	if err := verifyReconcileBundleCapabilities(ctx, client, bundle.ID, target.Entitlements); err != nil {
		return nil, nil, err
	}
	resolvedDesired, deviceIDs, err := resolveApplyDesiredDevices(ctx, client, devicesFile)
	if err != nil {
		return nil, nil, err
	}
	// Accept monotonic drift when an exact suitable profile already exists.
	candidates, err := getProfileCandidates(ctx, client, *bundle, target, resolvedDesired, *plan.Certificate, plan.MinimumValidityDays)
	if err != nil {
		return nil, nil, err
	}
	for _, candidate := range candidates {
		if candidate.Suitable {
			return fetchVerifiedProfileContent(ctx, client, candidate.Profile.ID, plan, devicesFile, target)
		}
	}
	created, err := client.CreateProfile(ctx, asc.ProfileCreateAttributes{Name: action.ProfileName, ProfileType: reconcileProfileType}, bundle.ID, []string{plan.Certificate.ID}, deviceIDs)
	if err != nil {
		// A 409 or uncertain response is accepted only if a reread proves the
		// deterministic profile name and exact suitability.
		profiles, readErr := getAllBundleProfiles(ctx, client, bundle.ID)
		if readErr == nil {
			for _, profile := range profiles {
				if profile.Attributes.Name == action.ProfileName {
					if verified, content, verifyErr := fetchVerifiedProfileContent(ctx, client, profile.ID, plan, devicesFile, target); verifyErr == nil {
						return verified, content, nil
					}
				}
			}
		}
		return nil, nil, err
	}
	return fetchVerifiedProfileContent(ctx, client, created.Data.ID, plan, devicesFile, target)
}

func verifyReconcileBundleCapabilities(ctx context.Context, client *asc.Client, bundleResourceID string, entitlements map[string]any) error {
	required, unverified := signingCapabilitiesForEntitlements(entitlements)
	if len(unverified) > 0 {
		return fmt.Errorf("cannot verify entitlement capabilities safely: %s", strings.Join(unverified, ","))
	}
	if len(required) == 0 {
		return nil
	}
	capabilities, err := getAllBundleIDCapabilities(ctx, client, bundleResourceID)
	if err != nil {
		return err
	}
	var enabled []string
	for _, capability := range capabilities {
		enabled = append(enabled, strings.ToUpper(strings.TrimSpace(capability.Attributes.CapabilityType)))
	}
	for _, capability := range required {
		if !containsAllStrings(enabled, []string{capability}) {
			return fmt.Errorf("bundle ID is missing required capability %s; refusing capability mutation", capability)
		}
	}
	return nil
}

func verifyReconcileProfile(ctx context.Context, client *asc.Client, profileID string, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile, target signingTarget) (*asc.Resource[asc.ProfileAttributes], []byte, error) {
	if strings.TrimSpace(profileID) == "" {
		return nil, nil, fmt.Errorf("planned profile ID is empty")
	}
	return fetchVerifiedProfileContent(ctx, client, profileID, plan, devicesFile, target)
}

func fetchVerifiedProfileContent(ctx context.Context, client *asc.Client, profileID string, plan signingReconcilePlanArtifact, devicesFile signingDevicesFile, target signingTarget) (*asc.Resource[asc.ProfileAttributes], []byte, error) {
	profile, err := client.GetProfile(ctx, profileID)
	if err != nil {
		return nil, nil, err
	}
	if profile.Data.ID != profileID {
		return nil, nil, fmt.Errorf("profile lookup returned resource ID %q for requested profile %q", profile.Data.ID, profileID)
	}
	if profile.Data.Attributes.ProfileType != reconcileProfileType || profile.Data.Attributes.ProfileState != asc.ProfileStateActive {
		return nil, nil, fmt.Errorf("profile is not active IOS_APP_ADHOC")
	}
	expires, err := time.Parse(time.RFC3339, profile.Data.Attributes.ExpirationDate)
	if err != nil || !expires.After(time.Now().Add(time.Duration(plan.MinimumValidityDays)*24*time.Hour)) {
		return nil, nil, fmt.Errorf("profile does not satisfy minimum validity")
	}
	certs, err := getAllProfileCertificateIDs(ctx, client, profileID)
	if err != nil {
		return nil, nil, err
	}
	if len(certs) != 1 || certs[0] != plan.Certificate.ID {
		return nil, nil, fmt.Errorf("profile certificate set differs from plan")
	}
	profileDevices, err := getAllProfileDeviceIDs(ctx, client, profileID)
	if err != nil {
		return nil, nil, err
	}
	desiredIDs, err := resolveApplyDeviceIDs(ctx, client, devicesFile)
	if err != nil {
		return nil, nil, err
	}
	if !sameStringSet(profileDevices, desiredIDs) {
		return nil, nil, fmt.Errorf("profile device set differs from plan")
	}
	encodedContent := strings.TrimSpace(profile.Data.Attributes.ProfileContent)
	if len(encodedContent) > base64.StdEncoding.EncodedLen(reconcileProfileMaxBytes) {
		return nil, nil, fmt.Errorf("profile content exceeds %d bytes", reconcileProfileMaxBytes)
	}
	content, err := base64.StdEncoding.DecodeString(encodedContent)
	if err != nil {
		return nil, nil, fmt.Errorf("decode profile content: %w", err)
	}
	if len(content) > reconcileProfileMaxBytes {
		return nil, nil, fmt.Errorf("profile content exceeds %d bytes", reconcileProfileMaxBytes)
	}
	parsed, err := decodeReconcileMobileProvision(content)
	if err != nil {
		return nil, nil, fmt.Errorf("verify profile content: %w", err)
	}
	if !safeProfileUUID(parsed.UUID) {
		return nil, nil, fmt.Errorf("profile content has unsafe or missing UUID")
	}
	if !parsed.ExpirationDate.After(time.Now().Add(time.Duration(plan.MinimumValidityDays) * 24 * time.Hour)) {
		return nil, nil, fmt.Errorf("profile content does not satisfy minimum validity")
	}
	if !entitlementsContain(parsed.Entitlements, target.Entitlements) {
		return nil, nil, fmt.Errorf("profile entitlements do not contain target entitlements")
	}
	if !mobileProvisionContainsCertificate(parsed, plan.Certificate.SHA256) {
		return nil, nil, fmt.Errorf("profile content does not contain the selected certificate")
	}
	provisioned := make(map[string]struct{})
	for _, udid := range parsed.ProvisionedDevices {
		provisioned[normalizeReconcileUDID(udid)] = struct{}{}
	}
	for _, input := range devicesFile.Devices {
		if _, ok := provisioned[normalizeReconcileUDID(input.UDID)]; !ok {
			return nil, nil, fmt.Errorf("profile content is missing desired device %s", input.Fingerprint)
		}
	}
	if len(provisioned) != len(devicesFile.Devices) {
		return nil, nil, fmt.Errorf("profile content device set differs from plan")
	}
	profile.Data.Attributes.UUID = parsed.UUID
	return &profile.Data, content, nil
}

func resolveApplyDeviceIDs(ctx context.Context, client *asc.Client, devicesFile signingDevicesFile) ([]string, error) {
	_, ids, err := resolveApplyDesiredDevices(ctx, client, devicesFile)
	return ids, err
}

func resolveApplyDesiredDevices(ctx context.Context, client *asc.Client, devicesFile signingDevicesFile) ([]signingDesiredDevice, []string, error) {
	remote, err := getAllReconcileDevices(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	var ids []string
	var desiredResult []signingDesiredDevice
	for _, desired := range devicesFile.Devices {
		var matches []string
		for _, device := range remote {
			if device.Attributes.Status == asc.DeviceStatusEnabled && normalizeReconcileUDID(device.Attributes.UDID) == normalizeReconcileUDID(desired.UDID) {
				matches = append(matches, device.ID)
			}
		}
		if len(matches) != 1 {
			return nil, nil, fmt.Errorf("device %s resolves to %d enabled resources", desired.Fingerprint, len(matches))
		}
		ids = append(ids, matches[0])
		desiredResult = append(desiredResult, signingDesiredDevice{
			Platform: desired.Platform, Fingerprint: desired.Fingerprint, NameSHA256: fingerprintReconcileName(desired.Name),
			ResourceID: matches[0], Status: string(asc.DeviceStatusEnabled),
		})
	}
	sortedIDs := append([]string(nil), ids...)
	sort.Strings(sortedIDs)
	return desiredResult, sortedIDs, nil
}

func writeVerifiedProfile(stateDir string, content []byte) (string, error) {
	parsed, err := decodeReconcileMobileProvision(content)
	if err != nil {
		return "", reconcileTerminalProfileOutputError{err: err}
	}
	relative := profileOutputRelativePath(parsed.UUID)
	if relative == "" {
		return "", reconcileTerminalProfileOutputError{err: fmt.Errorf("profile UUID is unsafe")}
	}
	root, err := rootfs.New(stateDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.MkdirAll("profiles", 0o700); err != nil {
		return "", err
	}
	existing, found, err := readOptionalBoundedRootFile(root, relative, reconcileProfileMaxBytes)
	if err != nil {
		return "", err
	}
	if found {
		existingDigest := sha256.Sum256(existing)
		contentDigest := sha256.Sum256(content)
		if existingDigest != contentDigest {
			return "", reconcileTerminalProfileOutputError{err: fmt.Errorf("profile %s already exists with different content", parsed.UUID)}
		}
		return filepath.Join(stateDir, filepath.FromSlash(relative)), nil
	}
	if err := root.CreateNewFileAtomic(relative, content, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, found, readErr := readOptionalBoundedRootFile(root, relative, reconcileProfileMaxBytes)
			if readErr == nil && found {
				existingDigest := sha256.Sum256(existing)
				contentDigest := sha256.Sum256(content)
				if existingDigest == contentDigest {
					return filepath.Join(stateDir, filepath.FromSlash(relative)), nil
				}
				return "", reconcileTerminalProfileOutputError{err: fmt.Errorf("profile %s already exists with different content", parsed.UUID)}
			}
			if readErr != nil {
				return "", readErr
			}
		}
		return "", err
	}
	return filepath.Join(stateDir, filepath.FromSlash(relative)), nil
}

func prepareReconcileProfileOutput(stateDir string) error {
	root, err := rootfs.New(stateDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll("profiles", 0o700); err != nil {
		return err
	}
	return root.CheckDirectoryWritable("profiles", 0o600)
}

func readOptionalBoundedRootFile(root rootfs.Root, relative string, limit int64) ([]byte, bool, error) {
	file, err := root.OpenFile(relative)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() > limit {
		return nil, false, fmt.Errorf("existing profile exceeds %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, false, fmt.Errorf("existing profile exceeds %d bytes", limit)
	}
	return data, true, nil
}

func targetByBundleID(targets []signingTarget, bundleID string) (signingTarget, bool) {
	for _, target := range targets {
		if target.BundleID == bundleID {
			return target, true
		}
	}
	return signingTarget{}, false
}
