package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/deviceset"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	signingReconcileSchemaV1       = 1
	defaultSigningReconcileDir     = ".asc/distribution/signing"
	defaultSigningReconcilePlan    = defaultSigningReconcileDir + "/plan.json"
	actionRegisterDevice           = "registerDevice"
	actionCreateBundleID           = "createBundleId"
	actionCreateProfile            = "createProfile"
	actionDownloadProfile          = "downloadProfile"
	reconcileProfileType           = "IOS_APP_ADHOC"
	reconcileCertificateType       = "IOS_DISTRIBUTION,DISTRIBUTION"
	deviceFingerprintPrefixLength  = 16
	profileNameFingerprintLength   = 12
	reconcileProtectedFileMaxBytes = 1 << 20
	reconcilePlanFileMaxBytes      = 16 << 20
	reconcileDeviceNameMaxBytes    = 128
	reconcileProfileMaxBytes       = 16 << 20
	reconcileMaximumValidityDays   = 3650
	reconcileWorkflowTimeout       = 15 * time.Minute
)

func validateSigningReconcilePlatform(goos string) error {
	if goos != "darwin" {
		return shared.UsageError("signing reconcile requires macOS because archive entitlement inspection uses codesign")
	}
	return nil
}

func requireSigningReconcilePlatform() error {
	return validateSigningReconcilePlatform(runtime.GOOS)
}

type signingDevicesFile struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Devices       []signingDeviceInput `json:"devices"`
}

type signingDeviceInput struct {
	Name        string `json:"name"`
	UDID        string `json:"-"`
	Platform    string `json:"platform"`
	Fingerprint string `json:"fingerprint"`
}

type signingTarget struct {
	Kind         string         `json:"kind"`
	RelativePath string         `json:"relativePath"`
	BundleID     string         `json:"bundleId"`
	AppIDPrefix  string         `json:"appIdPrefix"`
	Executable   string         `json:"executable"`
	Entitlements map[string]any `json:"entitlements"`
}

type signingCertificateRef struct {
	ID              string `json:"id"`
	CertificateType string `json:"certificateType"`
	SerialNumber    string `json:"serialNumber,omitempty"`
	ExpirationDate  string `json:"expirationDate"`
	SHA256          string `json:"sha256"`
	TeamID          string `json:"teamId"`
}

type signingDesiredDevice struct {
	Platform    string `json:"platform"`
	Fingerprint string `json:"fingerprint"`
	NameSHA256  string `json:"nameSha256"`
	ResourceID  string `json:"resourceId,omitempty"`
	Status      string `json:"status,omitempty"`
}

type signingObservedBundle struct {
	BundleID            string                   `json:"bundleId"`
	ResourceID          string                   `json:"resourceId,omitempty"`
	Platform            string                   `json:"platform,omitempty"`
	EnabledCapabilities []string                 `json:"enabledCapabilities,omitempty"`
	Profiles            []signingObservedProfile `json:"profiles,omitempty"`
	SelectedProfileID   string                   `json:"selectedProfileId,omitempty"`
}

type signingObservedProfile struct {
	ID             string   `json:"id"`
	State          string   `json:"state"`
	ExpirationDate string   `json:"expirationDate"`
	CertificateIDs []string `json:"certificateIds"`
	DeviceCount    int      `json:"deviceCount"`
	DeviceSetHash  string   `json:"deviceSetHash"`
	Suitable       bool     `json:"suitable"`
}

type signingAction struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	BundleID           string   `json:"bundleId,omitempty"`
	ResourceID         string   `json:"resourceId,omitempty"`
	DeviceFingerprint  string   `json:"deviceFingerprint,omitempty"`
	Platform           string   `json:"platform,omitempty"`
	CertificateID      string   `json:"certificateId,omitempty"`
	DeviceResourceIDs  []string `json:"deviceResourceIds,omitempty"`
	ProfileID          string   `json:"profileId,omitempty"`
	ProfileName        string   `json:"profileName,omitempty"`
	OutputRelativePath string   `json:"outputRelativePath,omitempty"`
}

type signingReconcilePaths struct {
	ArchivePath string `json:"archivePath"`
	DevicesFile string `json:"devicesFile"`
	StateDir    string `json:"stateDir"`
	PlanPath    string `json:"planPath"`
	ReceiptPath string `json:"receiptPath"`
	ProfilesDir string `json:"profilesDir"`
}

type signingReconcilePlanArtifact struct {
	SchemaVersion       int                     `json:"schemaVersion"`
	GeneratedAt         string                  `json:"generatedAt"`
	Command             string                  `json:"command"`
	PlanHash            string                  `json:"planHash"`
	Ready               bool                    `json:"ready"`
	TeamID              string                  `json:"teamId"`
	MinimumValidityDays int                     `json:"minimumValidityDays"`
	MaxMutations        int                     `json:"maxMutations"`
	MutationCount       int                     `json:"mutationCount"`
	DeviceSetSHA256     string                  `json:"deviceSetSha256"`
	Paths               signingReconcilePaths   `json:"paths"`
	Certificate         *signingCertificateRef  `json:"certificate,omitempty"`
	Targets             []signingTarget         `json:"targets"`
	Devices             []signingDesiredDevice  `json:"devices"`
	ObservedBundles     []signingObservedBundle `json:"observedBundles"`
	Actions             []signingAction         `json:"actions"`
	Blockers            []string                `json:"blockers,omitempty"`
}

type signingActionReceipt struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	ResourceID string `json:"resourceId,omitempty"`
	OutputPath string `json:"outputPath,omitempty"`
	Error      string `json:"error,omitempty"`
}

type signingReconcileReceipt struct {
	SchemaVersion int                    `json:"schemaVersion"`
	PlanHash      string                 `json:"planHash"`
	StartedAt     string                 `json:"startedAt"`
	UpdatedAt     string                 `json:"updatedAt"`
	Complete      bool                   `json:"complete"`
	StateDir      string                 `json:"stateDir"`
	ReceiptPath   string                 `json:"receiptPath"`
	Actions       []signingActionReceipt `json:"actions"`
}

type signingReconcilePlanOptions struct {
	ArchivePath         string
	DevicesFile         string
	CertificateID       string
	CertificateSHA256   string
	MinimumValidityDays int
	MaxMutations        int
	StateDir            string
	Overwrite           bool
}

// SigningReconcileCommand returns the artifact-backed signing reconciliation group.
func SigningReconcileCommand() *ffcli.Command {
	fs := flag.NewFlagSet("signing reconcile", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "reconcile",
		ShortUsage: "asc signing reconcile <subcommand> [flags]",
		ShortHelp:  "Plan and apply additive ad hoc signing changes.",
		LongHelp: `Plan and apply deterministic, additive signing changes for an Xcode archive.

Planning never mutates App Store Connect. Apply requires --confirm and only
registers missing devices, creates safe baseline App IDs, and creates successor
ad hoc profiles. It never deletes, patches, renames, enables capabilities, or
creates certificates.

Examples:
  asc signing reconcile plan --archive-path .asc/artifacts/App.xcarchive --devices-file .asc/distribution/devices.json
  asc signing reconcile apply --plan .asc/distribution/signing/plan.json --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SigningReconcilePlanCommand(),
			SigningReconcileApplyCommand(),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// SigningReconcilePlanCommand returns the read-only reconcile planner.
func SigningReconcilePlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("signing reconcile plan", flag.ExitOnError)
	archivePath := fs.String("archive-path", "", "Xcode .xcarchive path (required)")
	devicesFile := fs.String("devices-file", "", "Strict JSON v1 desired devices file (required)")
	certificateID := fs.String("certificate", "", "Explicit iOS distribution certificate resource ID")
	minimumValidityDays := fs.Int("minimum-validity-days", 7, "Minimum remaining profile validity in days")
	maxMutations := fs.Int("max-mutations", 32, "Maximum additive remote mutations in the plan")
	stateDir := fs.String("state-dir", defaultSigningReconcileDir, "Signing reconciliation state directory")
	overwrite := fs.Bool("overwrite", false, "Replace an existing plan artifact")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc signing reconcile plan --archive-path PATH --devices-file PATH [flags]",
		ShortHelp:  "Inspect signing state and write a deterministic plan.",
		LongHelp: `Inspect an archive, desired devices, and current signing state without mutation.

A blocked state is written successfully with ready=false so an agent can inspect
and resolve it before apply.

Example:
  asc signing reconcile plan --archive-path .asc/artifacts/App.xcarchive --devices-file .asc/distribution/devices.json --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateSigningReconcilePlanFlags(*archivePath, *devicesFile, *minimumValidityDays, *maxMutations, *stateDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return signingReconcilePlanUsageError(err)
			}
			plan, err := executeSigningReconcilePlan(ctx, signingReconcilePlanOptions{
				ArchivePath:         *archivePath,
				DevicesFile:         *devicesFile,
				CertificateID:       *certificateID,
				MinimumValidityDays: *minimumValidityDays,
				MaxMutations:        *maxMutations,
				StateDir:            *stateDir,
				Overwrite:           *overwrite,
			})
			if err != nil {
				return fmt.Errorf("signing reconcile plan: %w", err)
			}
			return printSigningReconcilePlan(plan, *output.Output, *output.Pretty)
		},
	}
}

// SigningReconcileApplyCommand returns the confirmed reconcile executor.
func SigningReconcileApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("signing reconcile apply", flag.ExitOnError)
	planPath := fs.String("plan", defaultSigningReconcilePlan, "Reconciliation plan artifact")
	confirm := fs.Bool("confirm", false, "Confirm the additive App Store Connect mutations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc signing reconcile apply [--plan PATH] --confirm [flags]",
		ShortHelp:  "Apply an exact ready signing reconciliation plan.",
		LongHelp: `Apply an exact ready reconciliation plan and write a resumable receipt.

The command re-reads protected local inputs and remote preconditions. Completed
actions are reverified idempotently on retry. No delete or patch request is ever sent.

Example:
  asc signing reconcile apply --plan .asc/distribution/signing/plan.json --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			if strings.TrimSpace(*planPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --plan is required")
				return shared.MissingRequiredUsageError("--plan")
			}
			receipt, err := executeSigningReconcileApply(ctx, *planPath)
			if err != nil {
				return fmt.Errorf("signing reconcile apply: %w", err)
			}
			return printSigningReconcileReceipt(receipt, *output.Output, *output.Pretty)
		},
	}
}

func validateSigningReconcilePlanFlags(archivePath, devicesFile string, minimumValidityDays, maxMutations int, stateDir string) error {
	if strings.TrimSpace(archivePath) == "" {
		return shared.WithDiagnostic(fmt.Errorf("--archive-path is required"), shared.DiagnosticRequiredInputMissing, "--archive-path")
	}
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(archivePath)), ".xcarchive") {
		return shared.WithDiagnostic(fmt.Errorf("--archive-path must end with .xcarchive"), shared.DiagnosticInvalidInput, "--archive-path")
	}
	if strings.TrimSpace(devicesFile) == "" {
		return shared.WithDiagnostic(fmt.Errorf("--devices-file is required"), shared.DiagnosticRequiredInputMissing, "--devices-file")
	}
	if minimumValidityDays < 0 {
		return shared.WithDiagnostic(fmt.Errorf("--minimum-validity-days must be at least 0"), shared.DiagnosticInvalidInput, "--minimum-validity-days")
	}
	if minimumValidityDays > reconcileMaximumValidityDays {
		return shared.WithDiagnostic(fmt.Errorf("--minimum-validity-days must be at most %d", reconcileMaximumValidityDays), shared.DiagnosticInvalidInput, "--minimum-validity-days")
	}
	if maxMutations < 1 {
		return shared.WithDiagnostic(fmt.Errorf("--max-mutations must be at least 1"), shared.DiagnosticInvalidInput, "--max-mutations")
	}
	if strings.TrimSpace(stateDir) == "" {
		return shared.WithDiagnostic(fmt.Errorf("--state-dir is required"), shared.DiagnosticRequiredInputMissing, "--state-dir")
	}
	return nil
}

func signingReconcilePlanUsageError(err error) error {
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		return flag.ErrHelp
	}
	return shared.WithDiagnostic(flag.ErrHelp, diagnostic.Code, diagnostic.Parameter)
}

func readProtectedFile(path string) ([]byte, error) {
	return readProtectedFileBounded(path, reconcileProtectedFileMaxBytes)
}

func readProtectedFileBounded(path string, limit int64) ([]byte, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("protected input is not a regular file")
	}
	if err := validateReconcileProtectedFilePlatform(file, info); err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o177 != 0 {
		return nil, fmt.Errorf("protected input permissions must be 0600 or stricter")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("protected input exceeds %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("protected input exceeds %d bytes", limit)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validateReconcileProtectedFilePlatform(file, after); err != nil {
		return nil, err
	}
	if !os.SameFile(info, after) || info.Mode() != after.Mode() || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("protected input changed while reading")
	}
	return data, nil
}

func protectedDevicesFileUsageError(err error) error {
	rendered := shared.UsageError("invalid devices file: " + protectedInputDiagnostic(err))
	return shared.NewErrorWithCause(rendered, err)
}

func protectedInputDiagnostic(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "protected input was not found"
	case errors.Is(err, rootfs.ErrSymlink):
		return "protected input contains a symlinked path component"
	case strings.Contains(err.Error(), "0600 or stricter"):
		return "protected input permissions must be 0600 or stricter"
	case strings.Contains(err.Error(), "not a regular file"):
		return "protected input is not a regular file"
	case strings.Contains(err.Error(), "exceeds"):
		return "protected input exceeds the size limit"
	default:
		return "protected input could not be read safely"
	}
}

func invalidDevicesFileUsageError(err error) error {
	rendered := shared.UsageError("invalid devices file: " + invalidDevicesFileDiagnostic(err))
	return shared.NewErrorWithCause(rendered, err)
}

func invalidDevicesFileDiagnostic(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "unknown field"):
		return "contains an unknown field"
	case strings.Contains(message, "decode devices file"):
		return "contains invalid JSON"
	case strings.Contains(message, "schemaVersion"):
		return fmt.Sprintf("schemaVersion must be %d", signingReconcileSchemaV1)
	case strings.Contains(message, "devices must contain"):
		return "must contain at least one device"
	case strings.Contains(message, ".name"):
		return "contains an invalid device name"
	case strings.Contains(message, ".udid"):
		return "contains an invalid device UDID"
	case strings.Contains(message, ".platform"):
		return "contains an unsupported device platform"
	case strings.Contains(message, "duplicate device"):
		return "contains a duplicate device UDID"
	default:
		return "failed validation"
	}
}

func decodeSigningDevicesFile(data []byte) (signingDevicesFile, error) {
	type rawDevice struct {
		Name     string `json:"name"`
		UDID     string `json:"udid"`
		Platform string `json:"platform"`
	}
	type rawFile struct {
		SchemaVersion int         `json:"schemaVersion"`
		Devices       []rawDevice `json:"devices"`
	}
	var raw rawFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return signingDevicesFile{}, fmt.Errorf("decode devices file: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return signingDevicesFile{}, fmt.Errorf("decode devices file: %w", err)
	}
	if raw.SchemaVersion != signingReconcileSchemaV1 {
		return signingDevicesFile{}, fmt.Errorf("schemaVersion must be %d", signingReconcileSchemaV1)
	}
	if len(raw.Devices) == 0 {
		return signingDevicesFile{}, fmt.Errorf("devices must contain at least one device")
	}
	result := signingDevicesFile{SchemaVersion: raw.SchemaVersion}
	seen := make(map[string]struct{}, len(raw.Devices))
	for index, device := range raw.Devices {
		name := strings.TrimSpace(device.Name)
		udid := strings.TrimSpace(device.UDID)
		platform := strings.ToUpper(strings.TrimSpace(device.Platform))
		if name == "" {
			return signingDevicesFile{}, fmt.Errorf("devices[%d].name is required", index)
		}
		if len(name) > reconcileDeviceNameMaxBytes || !safeReconcileDeviceName(name) {
			return signingDevicesFile{}, fmt.Errorf("devices[%d].name must be 1-%d printable characters without control or bidi formatting", index, reconcileDeviceNameMaxBytes)
		}
		if udid == "" {
			return signingDevicesFile{}, fmt.Errorf("devices[%d].udid is required", index)
		}
		if platform != string(asc.DevicePlatformIOS) {
			return signingDevicesFile{}, fmt.Errorf("devices[%d].platform must be IOS", index)
		}
		if !validReconcileUDID(udid) {
			return signingDevicesFile{}, fmt.Errorf("devices[%d].udid has invalid format", index)
		}
		normalized := normalizeReconcileUDID(udid)
		if _, ok := seen[normalized]; ok {
			return signingDevicesFile{}, fmt.Errorf("duplicate device UDID at devices[%d]", index)
		}
		seen[normalized] = struct{}{}
		result.Devices = append(result.Devices, signingDeviceInput{
			Name: name, UDID: udid, Platform: platform, Fingerprint: fingerprintDevice(normalized),
		})
	}
	sort.Slice(result.Devices, func(i, j int) bool { return result.Devices[i].Fingerprint < result.Devices[j].Fingerprint })
	return result, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return err
}

func normalizeReconcileUDID(value string) string {
	replacer := strings.NewReplacer("-", "", ":", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(value)))
}

func safeReconcileDeviceName(value string) bool {
	for _, character := range value {
		if !unicode.IsPrint(character) || unicode.In(character, unicode.Bidi_Control) {
			return false
		}
	}
	return true
}

func validReconcileUDID(value string) bool {
	if len(value) < 8 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '-' {
			continue
		}
		return false
	}
	normalized := normalizeReconcileUDID(value)
	return len(normalized) >= 8 && len(normalized) <= 48
}

func fingerprintDevice(normalizedUDID string) string {
	sum := sha256.Sum256([]byte(normalizedUDID))
	return hex.EncodeToString(sum[:])[:deviceFingerprintPrefixLength]
}

func fingerprintReconcileName(name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	return hex.EncodeToString(sum[:])
}

func digestSigningDeviceInputs(devices []signingDeviceInput) deviceset.Result {
	values := make([]string, 0, len(devices))
	for _, device := range devices {
		values = append(values, device.UDID)
	}
	return deviceset.Digest(values)
}

func hashSigningReconcilePlan(plan signingReconcilePlanArtifact) (string, error) {
	plan.GeneratedAt = ""
	plan.PlanHash = ""
	data, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeSigningStateJSON(stateDir, relativePath string, value any, overwrite bool) error {
	root, err := rootfs.New(stateDir)
	if err != nil {
		return err
	}
	defer root.Close()
	parent := filepath.Dir(relativePath)
	if parent != "." {
		if err := root.MkdirAll(parent, 0o700); err != nil {
			return err
		}
	} else if err := root.MkdirAll(".", 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if relativePath == "plan.json" && len(data) > reconcilePlanFileMaxBytes {
		return fmt.Errorf("signing reconcile plan exceeds %d bytes", reconcilePlanFileMaxBytes)
	}
	if overwrite {
		return root.WriteFile(relativePath, data, 0o600)
	}
	return root.CreateNewFileAtomic(relativePath, data, 0o600)
}

func readSigningPlanArtifact(path string) (signingReconcilePlanArtifact, error) {
	return readSigningPlanArtifactWithUsage(path, shared.UsageError)
}

func readSigningPlanArtifactSilent(path string) (signingReconcilePlanArtifact, error) {
	return readSigningPlanArtifactWithUsage(path, func(message string) error { return errors.New(message) })
}

func readSigningPlanArtifactWithUsage(path string, usageError func(string) error) (signingReconcilePlanArtifact, error) {
	data, err := readProtectedFileBounded(path, reconcilePlanFileMaxBytes)
	if err != nil {
		message := "invalid signing reconcile plan: " + protectedInputDiagnostic(err)
		if errors.Is(err, os.ErrNotExist) {
			message = fmt.Sprintf("plan artifact not found at %s; run asc signing reconcile plan first", path)
		}
		return signingReconcilePlanArtifact{}, shared.NewErrorWithCause(usageError(message), err)
	}
	var plan signingReconcilePlanArtifact
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&plan); err != nil {
		return signingReconcilePlanArtifact{}, usageError("invalid signing reconcile plan: contains invalid JSON")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return signingReconcilePlanArtifact{}, usageError("invalid signing reconcile plan: contains invalid JSON")
	}
	if plan.SchemaVersion != signingReconcileSchemaV1 {
		return signingReconcilePlanArtifact{}, usageError(fmt.Sprintf("unsupported signing reconcile plan schema version %d", plan.SchemaVersion))
	}
	if strings.TrimSpace(plan.PlanHash) == "" {
		return signingReconcilePlanArtifact{}, usageError("signing reconcile plan is missing planHash")
	}
	actual, err := hashSigningReconcilePlan(plan)
	if err != nil {
		return signingReconcilePlanArtifact{}, err
	}
	if actual != plan.PlanHash {
		return signingReconcilePlanArtifact{}, usageError("signing reconcile plan hash does not match its contents; rerun asc signing reconcile plan")
	}
	return plan, nil
}

func printSigningReconcilePlan(plan signingReconcilePlanArtifact, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		plan, format, pretty,
		func() error { return renderSigningPlan(plan, false) },
		func() error { return renderSigningPlan(plan, true) },
	)
}

func printSigningReconcileReceipt(receipt signingReconcileReceipt, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		receipt, format, pretty,
		func() error { return renderSigningReceipt(receipt, false) },
		func() error { return renderSigningReceipt(receipt, true) },
	)
}

func renderSigningPlan(plan signingReconcilePlanArtifact, markdown bool) error {
	headers := []string{"Ready", "Plan Hash", "Targets", "Devices", "Mutations", "Blockers", "Plan Path"}
	rows := [][]string{{fmt.Sprint(plan.Ready), plan.PlanHash, fmt.Sprint(len(plan.Targets)), fmt.Sprint(len(plan.Devices)), fmt.Sprintf("%d/%d", plan.MutationCount, plan.MaxMutations), fmt.Sprint(len(plan.Blockers)), plan.Paths.PlanPath}}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return nil
	}
	asc.RenderTable(headers, rows)
	return nil
}

func renderSigningReceipt(receipt signingReconcileReceipt, markdown bool) error {
	headers := []string{"Complete", "Plan Hash", "Actions", "Receipt Path"}
	rows := [][]string{{fmt.Sprint(receipt.Complete), receipt.PlanHash, fmt.Sprint(len(receipt.Actions)), receipt.ReceiptPath}}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return nil
	}
	asc.RenderTable(headers, rows)
	return nil
}

func reconcilePaths(options signingReconcilePlanOptions) signingReconcilePaths {
	stateDir := filepath.Clean(strings.TrimSpace(options.StateDir))
	return signingReconcilePaths{
		ArchivePath: filepath.Clean(strings.TrimSpace(options.ArchivePath)),
		DevicesFile: filepath.Clean(strings.TrimSpace(options.DevicesFile)),
		StateDir:    stateDir,
		PlanPath:    filepath.Join(stateDir, "plan.json"),
		ReceiptPath: filepath.Join(stateDir, "receipt.json"),
		ProfilesDir: filepath.Join(stateDir, "profiles"),
	}
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
