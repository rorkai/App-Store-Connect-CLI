package xcode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	installMinimumTimeout    = 5 * time.Second
	installMaximumTimeout    = 10 * time.Minute
	installJSONLimit         = 8 << 20
	installDiagnosticLimit   = 32 << 10
	installLegacyJSONVersion = 4
	installJSONVersion       = 5
	installJSONHeadroom      = 2 * time.Second
	installDeviceHashDomain  = "asc.xcode.install.device.v1:\x00"
)

var (
	installRunner                installDeviceRunner = processInstallDeviceRunner{}
	openInstallIPASource                             = defaultOpenInstallIPASource
	installNow                                       = time.Now
	installAfterJSONLstatForTest func()
)

// installIPASource is the deferred-extraction seam between the command flow
// and distribution materialization: profile eligibility and device discovery
// read Inspection first, and MaterializeApp extracts the app payload only
// after those cheap checks pass.
type installIPASource interface {
	Inspection() distribution.Inspection
	MaterializeApp(context.Context) (*distribution.MaterializedApp, error)
	Cleanup()
}

func defaultOpenInstallIPASource(ctx context.Context, file *os.File, size int64, options distribution.InspectOptions) (installIPASource, error) {
	source, err := distribution.OpenIPAAppSourceContext(ctx, file, size, options)
	if err != nil {
		return nil, err
	}
	return source, nil
}

// InstallOptions describes a local IPA installation on one exact connected
// CoreDevice. Environment is optional; nil means use the current process
// environment after the strict Xcode-child allowlist is applied.
type InstallOptions struct {
	IPAPath     string
	DeviceID    string
	Timeout     time.Duration
	Environment []string
}

// ValidateInstallOptions checks deterministic command inputs without opening
// files or invoking any local tool.
func ValidateInstallOptions(options InstallOptions) error {
	return validateInstallOptions(options)
}

// InstallUnsupportedError reports that connected-device installation is not
// available on the current platform.
type InstallUnsupportedError struct {
	Platform string
}

func (e *InstallUnsupportedError) Error() string {
	return fmt.Sprintf("connected-device installation is unsupported on %s; macOS with Xcode devicectl is required", e.Platform)
}

// InstallInputError identifies a malformed or inaccessible command input.
// The CLI maps this error to its usage exit code without emitting an
// operational result, preserving the stdout-empty usage contract.
type InstallInputError struct {
	message string
}

func (e *InstallInputError) Error() string {
	if e == nil || e.message == "" {
		return "invalid IPA input"
	}
	return e.message
}

type installDeviceNotFoundError struct{}

func (*installDeviceNotFoundError) Error() string {
	return "the requested CoreDevice was not found"
}

type installDeviceUnavailableError struct{}

func (*installDeviceUnavailableError) Error() string {
	return "the requested CoreDevice is not a paired, available physical iOS device"
}

type installDeviceProfileMismatchError struct{}

func (*installDeviceProfileMismatchError) Error() string {
	return "the requested device is not provisioned by the inspected IPA"
}

type installDeviceMembershipUnavailableError struct{}

func (*installDeviceMembershipUnavailableError) Error() string {
	return "unable to verify profile device membership for the requested device"
}

type installCommandError struct {
	Stage    string
	Outcome  string
	Code     int
	ExitCode int
}

func (e *installCommandError) Error() string {
	stage := e.Stage
	if stage == "" {
		stage = "devicectl"
	}
	if e.Code != 0 {
		return fmt.Sprintf("%s failed (code=%d)", stage, e.Code)
	}
	if e.ExitCode >= 0 {
		return fmt.Sprintf("%s failed (exit=%d)", stage, e.ExitCode)
	}
	return fmt.Sprintf("%s failed", stage)
}

type installVerificationError struct {
	reason string
}

func (e *installVerificationError) Error() string {
	if e.reason == "" {
		return "installed app could not be verified on the requested device"
	}
	return "installed app verification failed: " + e.reason
}

type installDeviceRunner interface {
	ResolveDevicectl(context.Context, []string) (string, error)
	Run(context.Context, string, []string, []string) error
}

type processInstallDeviceRunner struct{}

func (processInstallDeviceRunner) ResolveDevicectl(ctx context.Context, environment []string) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "/usr/bin/xcrun", "--find", "devicectl")
	command.Env = environment
	command.Stdin = nil
	stdout := &limitedInstallBuffer{limit: installDiagnosticLimit}
	command.Stdout = stdout
	command.Stderr = &limitedInstallBuffer{limit: installDiagnosticLimit}
	if err := runXcodeCommand(command); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return "", contextErr
		}
		return "", fmt.Errorf("resolve devicectl through the active Xcode toolchain")
	}
	pathValue := strings.TrimSpace(stdout.String())
	if pathValue == "" || !filepath.IsAbs(pathValue) || strings.ContainsAny(pathValue, "\r\n\x00") {
		return "", fmt.Errorf("resolve devicectl through the active Xcode toolchain: invalid absolute path")
	}
	info, err := os.Lstat(pathValue)
	if err != nil {
		return "", fmt.Errorf("resolve devicectl through the active Xcode toolchain: tool is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("resolve devicectl through the active Xcode toolchain: tool is not a regular executable")
	}
	return filepath.Clean(pathValue), nil
}

func (processInstallDeviceRunner) Run(ctx context.Context, pathValue string, args, environment []string) error {
	command := exec.CommandContext(ctx, pathValue, args...)
	command.Env = environment
	command.Stdin = nil
	command.Stdout = &limitedInstallBuffer{limit: installDiagnosticLimit}
	command.Stderr = &limitedInstallBuffer{limit: installDiagnosticLimit}
	return runXcodeCommand(command)
}

type limitedInstallBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedInstallBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.buffer.Write(data)
	}
	return written, nil
}

func (b *limitedInstallBuffer) String() string {
	return b.buffer.String()
}

// Install validates, materializes, installs, and verifies one IPA on one exact
// connected iOS device. A non-nil result is returned for every operational
// failure after input validation; callers can therefore report failure without
// exposing the device identifier or temporary paths.
func Install(ctx context.Context, options InstallOptions) (*asc.XcodeInstallResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateInstallOptions(options); err != nil {
		return nil, err
	}
	started := installNow()
	fail := func(inspection *distribution.Inspection, device *installDevice, stage, code string, err error) (*asc.XcodeInstallResult, error) {
		return installFailureResult(started, inspection, device, stage, code, err)
	}
	if runtimeGOOS != "darwin" {
		return fail(nil, nil, "input", "unsupported_platform", &InstallUnsupportedError{Platform: runtimeGOOS})
	}
	requestContext, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	if err := contextError(requestContext); err != nil {
		return fail(nil, nil, "input", installContextFailureCode(err), err)
	}

	file, size, err := openInstallIPA(options.IPAPath)
	if err != nil {
		var inputErr *InstallInputError
		if errors.As(err, &inputErr) {
			return nil, err
		}
		return nil, &InstallInputError{message: "IPA input cannot be opened safely"}
	}
	defer file.Close()

	source, err := openInstallIPASource(requestContext, file, size, distribution.InspectOptions{
		IncludeDevices: true,
		Now:            installNow(),
	})
	if err != nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			return fail(nil, nil, "profile-preflight", installContextFailureCode(contextErr), contextErr)
		}
		return fail(nil, nil, "profile-preflight", "ipa_preflight_failed", fmt.Errorf("inspect IPA: %w", err))
	}
	if source == nil {
		return fail(nil, nil, "profile-preflight", "ipa_preflight_failed", fmt.Errorf("IPA inspector returned no source"))
	}
	defer source.Cleanup()
	inspection := source.Inspection()
	if err := validateInstallInspection(inspection); err != nil {
		return fail(&inspection, nil, "profile-preflight", "profile_not_installable", err)
	}

	runner := installRunner
	if runner == nil {
		return fail(&inspection, nil, "device-discovery", "devicectl_unavailable", fmt.Errorf("connected-device installation runner is required"))
	}
	environment := installEnvironment(options.Environment)
	devicectlPath, err := runner.ResolveDevicectl(requestContext, environment)
	if err != nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			return fail(&inspection, nil, "device-discovery", installContextFailureCode(contextErr), contextErr)
		}
		return fail(&inspection, nil, "device-discovery", "devicectl_unavailable", fmt.Errorf("resolve devicectl through the active Xcode toolchain"))
	}
	if !filepath.IsAbs(devicectlPath) {
		return fail(&inspection, nil, "device-discovery", "devicectl_unavailable", fmt.Errorf("resolved devicectl path is not absolute"))
	}
	jsonDir, err := os.MkdirTemp("", ".asc-xcode-install-json-")
	if err != nil {
		return fail(&inspection, nil, "device-discovery", "private_output_failed", fmt.Errorf("create private devicectl output directory"))
	}
	defer os.RemoveAll(jsonDir)
	if err := os.Chmod(jsonDir, 0o700); err != nil {
		return fail(&inspection, nil, "device-discovery", "private_output_failed", fmt.Errorf("protect private devicectl output directory"))
	}

	device, err := discoverInstallDevice(requestContext, runner, devicectlPath, environment, jsonDir, options.Timeout, options.DeviceID, inspection.Signing.Devices)
	if err != nil {
		return fail(&inspection, nil, "device-discovery", installFailureCodeForDeviceError(err), err)
	}

	materialized, err := source.MaterializeApp(requestContext)
	if err != nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			return fail(&inspection, &device, "materialization", installContextFailureCode(contextErr), contextErr)
		}
		return fail(&inspection, &device, "materialization", "materialization_failed", fmt.Errorf("materialize IPA app: %w", err))
	}
	if materialized == nil || materialized.Path == "" {
		return fail(&inspection, &device, "materialization", "materialization_failed", fmt.Errorf("IPA materializer returned no app"))
	}
	defer materialized.Cleanup()

	result := installResultFromInspection(inspection, &device)
	if err := runInstallCommand(requestContext, runner, devicectlPath, environment, jsonDir, options.Timeout, device, materialized.Path, inspection.App.BundleID); err != nil {
		finishInstallResult(result, started, false, false, false)
		setInstallFailure(result, "install", installFailureCodeForCommandError(err))
		return result, err
	}
	result.Installed = true
	if err := verifyInstalledApp(requestContext, runner, devicectlPath, environment, jsonDir, options.Timeout, device, inspection.App.BundleID, inspection.App.Version, inspection.App.BuildNumber); err != nil {
		finishInstallResult(result, started, true, false, false)
		setInstallFailure(result, "verification", installFailureCodeForVerificationError(err))
		return result, err
	}
	finishInstallResult(result, started, true, true, true)
	return result, nil
}

func validateInstallOptions(options InstallOptions) error {
	if options.IPAPath == "" {
		return fmt.Errorf("--ipa is required")
	}
	if !strings.EqualFold(filepath.Ext(options.IPAPath), ".ipa") {
		return fmt.Errorf("--ipa must end with .ipa")
	}
	if err := validateInstallText(options.DeviceID, "--device-id"); err != nil {
		return err
	}
	if options.Timeout < installMinimumTimeout || options.Timeout > installMaximumTimeout {
		return fmt.Errorf("--timeout must be between %s and %s", installMinimumTimeout, installMaximumTimeout)
	}
	return nil
}

func installFailureResult(started time.Time, inspection *distribution.Inspection, device *installDevice, stage, code string, err error) (*asc.XcodeInstallResult, error) {
	var result *asc.XcodeInstallResult
	if inspection != nil {
		result = installResultFromInspection(*inspection, device)
	} else {
		result = &asc.XcodeInstallResult{
			SchemaVersion: 1,
			Operation:     "xcode.install",
		}
	}
	finishInstallResult(result, started, false, false, false)
	setInstallFailure(result, stage, code)
	return result, err
}

func setInstallFailure(result *asc.XcodeInstallResult, stage, code string) {
	if result == nil {
		return
	}
	result.FailureStage = stage
	result.FailureCode = code
	result.Success = false
}

func installContextFailureCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "operation_cancelled"
}

func installFailureCodeForDeviceError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var notFound *installDeviceNotFoundError
	if errors.As(err, &notFound) {
		return "device_not_found"
	}
	var unavailable *installDeviceUnavailableError
	if errors.As(err, &unavailable) {
		return "device_unavailable"
	}
	var mismatch *installDeviceProfileMismatchError
	if errors.As(err, &mismatch) {
		return "profile_device_mismatch"
	}
	var membershipUnavailable *installDeviceMembershipUnavailableError
	if errors.As(err, &membershipUnavailable) {
		return "profile_device_membership_unavailable"
	}
	var commandErr *installCommandError
	if errors.As(err, &commandErr) {
		return "device_discovery_failed"
	}
	return "device_discovery_invalid_response"
}

func installFailureCodeForCommandError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var commandErr *installCommandError
	if errors.As(err, &commandErr) {
		return "install_failed"
	}
	return "install_invalid_response"
}

func installFailureCodeForVerificationError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "verification_failed"
}

func validateInstallText(value, label string) error {
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not contain leading or trailing whitespace", label)
	}
	if len(value) > 1024 {
		return fmt.Errorf("%s exceeds %d bytes", label, 1024)
	}
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", label)
		}
	}
	return nil
}

func openInstallIPA(pathValue string) (*os.File, int64, error) {
	absolute, err := filepath.Abs(pathValue)
	if err != nil {
		return nil, 0, &InstallInputError{message: "IPA path cannot be resolved safely"}
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, 0, &InstallInputError{message: "IPA parent directory cannot be opened safely"}
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Base(absolute))
	if err != nil {
		return nil, 0, &InstallInputError{message: "IPA must be an accessible regular file"}
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, &InstallInputError{message: "IPA metadata cannot be inspected safely"}
	}
	if err := validateExactIPAFileInfo(info, "IPA"); err != nil {
		_ = file.Close()
		return nil, 0, &InstallInputError{message: "IPA must be a regular, single-link file owned by the current user"}
	}
	if info.Size() <= 0 || info.Size() > distribution.MaxIPABytes {
		_ = file.Close()
		return nil, 0, &InstallInputError{message: fmt.Sprintf("IPA size must be between 1 and %d bytes", distribution.MaxIPABytes)}
	}
	return file, info.Size(), nil
}

func validateInstallInspection(inspection distribution.Inspection) error {
	if inspection.Platform != "IOS" {
		return fmt.Errorf("IPA platform is not iOS")
	}
	if inspection.DistributionMethod != "release-testing" {
		return fmt.Errorf("IPA distribution method is not supported for connected-device installation")
	}
	if inspection.Signing.ProfileClass != distribution.ProfileClassDevelopment && inspection.Signing.ProfileClass != distribution.ProfileClassAdHoc {
		return fmt.Errorf("IPA provisioning profile must be development or ad-hoc")
	}
	if strings.TrimSpace(inspection.App.BundleID) == "" {
		return fmt.Errorf("IPA main-app bundle identifier is missing")
	}
	if strings.TrimSpace(inspection.App.Version) == "" {
		return fmt.Errorf("IPA main-app version is missing")
	}
	if strings.TrimSpace(inspection.App.BuildNumber) == "" {
		return fmt.Errorf("IPA main-app build number is missing")
	}
	if strings.TrimSpace(inspection.Signing.ProfileUUID) == "" {
		return fmt.Errorf("IPA provisioning profile UUID is missing")
	}
	expiresAt := strings.TrimSpace(inspection.Signing.ExpiresAt)
	if expiresAt == "" {
		return fmt.Errorf("IPA provisioning profile expiration is missing")
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || !expires.After(installNow()) {
		return fmt.Errorf("IPA provisioning profile is expired or invalid")
	}
	if len(inspection.Signing.ProfileCertificateSHA256Fingerprints) == 0 {
		return fmt.Errorf("IPA provisioning profile contains no signing certificates")
	}
	if inspection.Signing.DeviceCount < 1 || len(inspection.Signing.Devices) == 0 {
		return fmt.Errorf("IPA provisioning profile contains no provisioned devices")
	}
	if inspection.Signing.ProfileIntegrityVerification.Status != distribution.CodeSignatureVerified {
		return fmt.Errorf("IPA provisioning profile integrity is not verified")
	}
	if inspection.Signing.ProfileTrustVerification.Status != distribution.CodeSignatureVerified {
		return fmt.Errorf("IPA provisioning profile trust is not verified")
	}
	if inspection.Signing.CodeSignatureVerification.Status != distribution.CodeSignatureVerified {
		return fmt.Errorf("IPA main-app code signature is not verified")
	}
	if err := validateInstallPreparationIssues(inspection.Preparation.Issues, inspection.Signing.ProfileClass); err != nil {
		return err
	}
	return nil
}

func validateInstallPreparationIssues(issues []string, profileClass distribution.ProfileClass) error {
	for _, issue := range issues {
		switch issue {
		case "app title is missing", "embedded targets require target-by-target signing validation before preparation":
			continue
		case "provisioning profile class is development; expected ad-hoc":
			if profileClass == distribution.ProfileClassDevelopment {
				continue
			}
		}
		return fmt.Errorf("IPA contains an install-blocking preparation issue")
	}
	return nil
}

type installDevice struct {
	Identifier      string
	UDID            string
	Platform        string
	Reality         string
	PairingState    string
	ConnectionState string
	VisibilityClass string
}

func discoverInstallDevice(ctx context.Context, runner installDeviceRunner, devicectlPath string, environment []string, jsonDir string, timeout time.Duration, requestedID string, provisioned []string) (installDevice, error) {
	seconds := toolTimeoutSeconds(timeout)
	jsonName := "devices.json"
	payload, runErr := runInstallJSON(ctx, runner, devicectlPath, environment, jsonDir, jsonName, []string{
		"--quiet",
		"--timeout", seconds,
		"--json-output", filepath.Join(jsonDir, jsonName),
		"--omit-deprecated-fields-in-json",
		"list", "devices",
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return installDevice{}, contextErr
	}
	if runErr != nil && len(payload) == 0 {
		return installDevice{}, &installCommandError{Stage: "devicectl device discovery", ExitCode: installExitCode(runErr)}
	}
	envelope, err := parseDevicectlEnvelope(payload, "devicectl.list.devices")
	if err != nil {
		return installDevice{}, err
	}
	if envelope.Info.Outcome != "success" {
		return installDevice{}, structuredInstallCommandError("devicectl device discovery", envelope, runErr)
	}
	if runErr != nil {
		return installDevice{}, &installCommandError{Stage: "devicectl device discovery", ExitCode: installExitCode(runErr)}
	}
	var listed installDeviceListResult
	if err := decodeStrictDevicectlResult(envelope.Result, envelope.Info.JSONVersion, &listed); err != nil {
		return installDevice{}, fmt.Errorf("parse devicectl device discovery result: invalid schema")
	}
	if listed.Devices == nil {
		return installDevice{}, fmt.Errorf("parse devicectl device discovery result: devices are required")
	}
	var matched *installDevice
	for _, entry := range listed.Devices {
		if err := validateInstallDeviceEntry(entry); err != nil {
			return installDevice{}, err
		}
		if entry.Identifier != requestedID {
			continue
		}
		if matched != nil {
			return installDevice{}, fmt.Errorf("parse devicectl device discovery result: requested device is ambiguous")
		}
		candidate, err := normalizeInstallDevice(entry)
		if err != nil {
			return installDevice{}, err
		}
		matched = &candidate
	}
	if matched == nil {
		return installDevice{}, &installDeviceNotFoundError{}
	}
	if !isUsableInstallDevice(*matched) {
		return installDevice{}, &installDeviceUnavailableError{}
	}
	if !slices.Contains(provisioned, matched.UDID) {
		return installDevice{}, &installDeviceProfileMismatchError{}
	}
	return *matched, nil
}

func runInstallCommand(ctx context.Context, runner installDeviceRunner, devicectlPath string, environment []string, jsonDir string, timeout time.Duration, device installDevice, appPath, bundleID string) error {
	jsonName := "install.json"
	payload, runErr := runInstallJSON(ctx, runner, devicectlPath, environment, jsonDir, jsonName, []string{
		"--quiet",
		"--timeout", toolTimeoutSeconds(timeout),
		"--json-output", filepath.Join(jsonDir, jsonName),
		"--omit-deprecated-fields-in-json",
		"device", "install", "app",
		"--device", device.Identifier,
		appPath,
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if len(payload) == 0 {
		return &installCommandError{Stage: "devicectl app installation", ExitCode: installExitCode(runErr)}
	}
	envelope, err := parseDevicectlEnvelope(payload, "devicectl.device.install.app")
	if err != nil {
		return err
	}
	if envelope.Info.Outcome != "success" {
		return structuredInstallCommandError("devicectl app installation", envelope, runErr)
	}
	if runErr != nil {
		return &installCommandError{Stage: "devicectl app installation", ExitCode: installExitCode(runErr)}
	}
	var installed installDeviceInstallResult
	if err := decodeStrictDevicectlResult(envelope.Result, envelope.Info.JSONVersion, &installed); err != nil {
		return fmt.Errorf("parse devicectl app installation result: invalid schema")
	}
	if installed.DeviceIdentifier == "" || installed.DeviceIdentifier != device.Identifier {
		return fmt.Errorf("parse devicectl app installation result: target device is missing or unexpected")
	}
	if len(installed.InstalledApplications) == 0 {
		return fmt.Errorf("parse devicectl app installation result: installed applications are required")
	}
	found := false
	for _, app := range installed.InstalledApplications {
		value, err := requiredInstallString(app, "bundleID", "bundleIdentifier")
		if err != nil {
			return err
		}
		if value != bundleID {
			return fmt.Errorf("parse devicectl app installation result: unexpected installed bundle")
		}
		if found {
			return fmt.Errorf("parse devicectl app installation result: duplicate installed bundle")
		}
		found = true
	}
	if !found {
		return fmt.Errorf("parse devicectl app installation result: expected bundle was not installed")
	}
	return nil
}

func verifyInstalledApp(ctx context.Context, runner installDeviceRunner, devicectlPath string, environment []string, jsonDir string, timeout time.Duration, device installDevice, bundleID, version, build string) error {
	jsonName := "apps.json"
	payload, runErr := runInstallJSON(ctx, runner, devicectlPath, environment, jsonDir, jsonName, []string{
		"--quiet",
		"--timeout", toolTimeoutSeconds(timeout),
		"--json-output", filepath.Join(jsonDir, jsonName),
		"--omit-deprecated-fields-in-json",
		"device", "info", "apps",
		"--device", device.Identifier,
		"--bundle-id", bundleID,
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if len(payload) == 0 {
		return &installCommandError{Stage: "devicectl app verification", ExitCode: installExitCode(runErr)}
	}
	envelope, err := parseDevicectlEnvelope(payload, "devicectl.device.info.apps")
	if err != nil {
		return err
	}
	if envelope.Info.Outcome != "success" {
		return structuredInstallCommandError("devicectl app verification", envelope, runErr)
	}
	if runErr != nil {
		return &installCommandError{Stage: "devicectl app verification", ExitCode: installExitCode(runErr)}
	}
	var apps installAppsResult
	if err := decodeStrictDevicectlResult(envelope.Result, envelope.Info.JSONVersion, &apps); err != nil {
		return &installVerificationError{reason: "structured app list is invalid"}
	}
	if apps.Apps == nil {
		return &installVerificationError{reason: "structured app list is missing"}
	}
	matched := false
	for _, app := range *apps.Apps {
		actualBundle, err := requiredInstallString(app, "bundleIdentifier")
		if err != nil {
			return &installVerificationError{reason: "installed app identity is invalid"}
		}
		if actualBundle != bundleID {
			return &installVerificationError{reason: "device returned an unexpected app"}
		}
		if matched {
			return &installVerificationError{reason: "device returned duplicate app records"}
		}
		actualVersion, versionErr := requiredInstallString(app, "version")
		actualBuild, buildErr := requiredInstallString(app, "bundleVersion")
		if versionErr != nil || buildErr != nil {
			return &installVerificationError{reason: "installed app version metadata is missing"}
		}
		if actualVersion != version || actualBuild != build {
			return &installVerificationError{reason: "installed app version or build does not match the IPA"}
		}
		matched = true
	}
	if !matched {
		return &installVerificationError{reason: "expected app was not found on the device"}
	}
	return nil
}

func installResultFromInspection(inspection distribution.Inspection, device *installDevice) *asc.XcodeInstallResult {
	result := &asc.XcodeInstallResult{
		SchemaVersion: 1,
		Operation:     "xcode.install",
		IPA: asc.XcodeInstallArtifact{
			SHA256:      inspection.Artifact.SHA256,
			SizeBytes:   inspection.Artifact.SizeBytes,
			BundleID:    inspection.App.BundleID,
			Version:     inspection.App.Version,
			BuildNumber: inspection.App.BuildNumber,
		},
	}
	if device != nil {
		result.Device = &asc.XcodeInstallDevice{
			IdentifierSHA256: hashInstallIdentifier(device.Identifier),
			Platform:         strings.ToUpper(device.Platform),
			PairingState:     strings.ToLower(device.PairingState),
			ConnectionState:  strings.ToLower(device.ConnectionState),
		}
	}
	return result
}

func finishInstallResult(result *asc.XcodeInstallResult, started time.Time, installed, verified, success bool) {
	result.Installed = installed
	result.Verified = verified
	result.Success = success
	duration := installNow().Sub(started)
	if duration < 0 {
		duration = 0
	}
	result.DurationMS = duration.Milliseconds()
}

func hashInstallIdentifier(value string) string {
	sum := sha256.Sum256([]byte(installDeviceHashDomain + value))
	return hex.EncodeToString(sum[:])
}

func toolTimeoutSeconds(timeout time.Duration) string {
	toolTimeout := timeout - installJSONHeadroom
	if toolTimeout < time.Second {
		toolTimeout = time.Second
	}
	seconds := (toolTimeout + time.Second - 1) / time.Second
	return fmt.Sprintf("%d", int64(seconds))
}

func runInstallJSON(ctx context.Context, runner installDeviceRunner, pathValue string, environment []string, jsonDir, jsonName string, args []string) ([]byte, error) {
	if _, err := os.Lstat(filepath.Join(jsonDir, jsonName)); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return nil, fmt.Errorf("refusing to replace private devicectl output")
		}
		return nil, fmt.Errorf("inspect private devicectl output destination")
	}
	runErr := runner.Run(ctx, pathValue, args, environment)
	payload, readErr := readInstallJSON(jsonDir, jsonName)
	if readErr != nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, readErr
	}
	return payload, runErr
}

func readInstallJSON(directory, name string) ([]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open private devicectl output directory")
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read devicectl JSON output")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("devicectl JSON output is not a regular file")
	}
	if info.Size() > installJSONLimit {
		return nil, fmt.Errorf("devicectl JSON output exceeds the %d-byte size limit", installJSONLimit)
	}
	_, links, ok := exactIPAStatIdentity(info)
	if !ok || links != 1 {
		return nil, fmt.Errorf("devicectl JSON output must be a single-link file")
	}
	if installAfterJSONLstatForTest != nil {
		installAfterJSONLstatForTest()
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(root, name)
	if err != nil {
		return nil, fmt.Errorf("open devicectl JSON output")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	var openedLinks uint64
	openedOK := false
	if err == nil {
		_, openedLinks, openedOK = exactIPAStatIdentity(openedInfo)
	}
	if err != nil || !os.SameFile(info, openedInfo) || !openedOK || openedLinks != 1 || openedInfo.Size() != info.Size() || openedInfo.Mode() != info.Mode() {
		return nil, fmt.Errorf("devicectl JSON output changed while being read")
	}
	payload, err := io.ReadAll(io.LimitReader(file, installJSONLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read devicectl JSON output")
	}
	if len(payload) > installJSONLimit {
		return nil, fmt.Errorf("devicectl JSON output exceeds the %d-byte size limit", installJSONLimit)
	}
	after, err := file.Stat()
	var afterLinks uint64
	afterOK := false
	if err == nil {
		_, afterLinks, afterOK = exactIPAStatIdentity(after)
	}
	if err != nil || !os.SameFile(openedInfo, after) || !afterOK || afterLinks != 1 || after.Size() != openedInfo.Size() || after.Mode() != openedInfo.Mode() {
		return nil, fmt.Errorf("devicectl JSON output changed while being read")
	}
	pathAfter, err := root.Lstat(name)
	if err != nil || pathAfter.Mode()&os.ModeSymlink != 0 || !pathAfter.Mode().IsRegular() {
		return nil, fmt.Errorf("devicectl JSON output changed while being read")
	}
	_, pathLinks, pathOK := exactIPAStatIdentity(pathAfter)
	if !pathOK || pathLinks != 1 || !os.SameFile(after, pathAfter) || after.Size() != pathAfter.Size() || after.Mode() != pathAfter.Mode() {
		return nil, fmt.Errorf("devicectl JSON output changed while being read")
	}
	return payload, nil
}

type devicectlEnvelope struct {
	Info   devicectlInfo   `json:"info"`
	Result json.RawMessage `json:"result"`
	Error  *devicectlError `json:"error,omitempty"`
}

type devicectlInfo struct {
	Arguments   []string                   `json:"arguments"`
	CommandType string                     `json:"commandType"`
	Details     string                     `json:"details,omitempty"`
	Environment map[string]json.RawMessage `json:"environment"`
	JSONVersion int                        `json:"jsonVersion"`
	Outcome     string                     `json:"outcome"`
	Version     string                     `json:"version"`
}

type devicectlError struct {
	Code     int             `json:"code"`
	Domain   string          `json:"domain"`
	UserInfo json.RawMessage `json:"userInfo"`
}

type installDeviceListResult struct {
	Devices []installDeviceEntry `json:"devices"`
}

type installDeviceEntry struct {
	Capabilities         json.RawMessage `json:"capabilities,omitempty"`
	Identifier           string          `json:"identifier"`
	Properties           json.RawMessage `json:"properties,omitempty"`
	PropertyDisplayNames json.RawMessage `json:"propertyDisplayNames,omitempty"`
	VisibilityClass      string          `json:"visibilityClass,omitempty"`
	HardwareProperties   json.RawMessage `json:"hardwareProperties,omitempty"`
	DeviceProperties     json.RawMessage `json:"deviceProperties,omitempty"`
	ConnectionProperties json.RawMessage `json:"connectionProperties,omitempty"`
}

type installDeviceInstallResult struct {
	DeviceIdentifier      string                       `json:"deviceIdentifier"`
	InstalledApplications []map[string]json.RawMessage `json:"installedApplications"`
}

type installAppsResult struct {
	Apps *[]map[string]json.RawMessage `json:"apps"`
}

func parseDevicectlEnvelope(payload []byte, commandType string) (devicectlEnvelope, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return devicectlEnvelope{}, fmt.Errorf("devicectl JSON output is empty")
	}
	if err := rejectDuplicateInstallJSONKeys(payload); err != nil {
		return devicectlEnvelope{}, fmt.Errorf("parse devicectl JSON output: invalid JSON structure")
	}
	var envelope devicectlEnvelope
	if err := decodeStrictInstallJSON(payload, &envelope); err != nil {
		return devicectlEnvelope{}, fmt.Errorf("parse devicectl JSON output: invalid schema")
	}
	if envelope.Info.CommandType != commandType {
		return devicectlEnvelope{}, fmt.Errorf("parse devicectl JSON output: unexpected command type")
	}
	if !supportedInstallJSONVersion(envelope.Info.JSONVersion) {
		return devicectlEnvelope{}, fmt.Errorf("parse devicectl JSON output: unsupported JSON schema version %d", envelope.Info.JSONVersion)
	}
	if envelope.Info.Outcome == "" {
		return devicectlEnvelope{}, fmt.Errorf("parse devicectl JSON output: outcome is required")
	}
	return envelope, nil
}

func decodeStrictDevicectlResult(payload []byte, version int, destination any) error {
	if version != installJSONVersion {
		return decodeStrictInstallJSON(payload, destination)
	}
	if err := rejectDuplicateInstallJSONKeys(payload); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := decodeStrictInstallJSON(payload, &fields); err != nil {
		return err
	}
	if _, exists := fields["_deprecationNotice"]; exists {
		delete(fields, "_deprecationNotice")
		stripped, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		payload = stripped
	}
	return decodeStrictInstallJSON(payload, destination)
}

func supportedInstallJSONVersion(version int) bool {
	switch version {
	case installLegacyJSONVersion, installJSONVersion:
		return true
	default:
		return false
	}
}

func decodeStrictInstallJSON(payload []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func rejectDuplicateInstallJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := scanInstallJSONValue(decoder); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON token %v", token)
	}
	return nil
}

func scanInstallJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key")
			}
			seen[key] = struct{}{}
			if err := scanInstallJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := scanInstallJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("array is not closed")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}

func validateInstallDeviceEntry(entry installDeviceEntry) error {
	if err := validateInstallText(entry.Identifier, "device identifier"); err != nil {
		return fmt.Errorf("parse devicectl device discovery result: %w", err)
	}
	return nil
}

func normalizeInstallDevice(entry installDeviceEntry) (installDevice, error) {
	device := installDevice{Identifier: entry.Identifier, VisibilityClass: entry.VisibilityClass}
	properties, err := decodeInstallObject(entry.Properties)
	if err != nil && len(entry.Properties) > 0 {
		return installDevice{}, fmt.Errorf("parse devicectl device discovery result: invalid properties")
	}
	hardware, _ := objectFromObject(properties, "hardware")
	connection, _ := objectFromObject(properties, "connection")
	state, _ := objectFromObject(properties, "state")
	device.Platform = firstInstallObjectString(hardware, "platform")
	device.Reality = firstInstallObjectString(hardware, "reality")
	device.UDID = firstInstallObjectString(hardware, "udid")
	device.PairingState = firstInstallObjectString(connection, "pairingState")
	device.ConnectionState = firstInstallObjectString(connection, "state")
	if device.VisibilityClass == "" {
		device.VisibilityClass = firstInstallObjectString(state, "visibilityClass")
	}
	if device.Platform == "" || device.Reality == "" || device.UDID == "" || device.PairingState == "" || device.ConnectionState == "" {
		legacyHardware, legacyConnection, legacyState := decodeLegacyInstallObjects(entry)
		if device.Platform == "" {
			device.Platform = firstInstallObjectString(legacyHardware, "platform")
		}
		if device.Reality == "" {
			device.Reality = firstInstallObjectString(legacyHardware, "reality")
		}
		if device.UDID == "" {
			device.UDID = firstInstallObjectString(legacyHardware, "udid")
		}
		if device.PairingState == "" {
			device.PairingState = firstInstallObjectString(legacyConnection, "pairingState")
		}
		if device.ConnectionState == "" {
			device.ConnectionState = firstInstallObjectString(legacyConnection, "state", "tunnelState")
		}
		if device.VisibilityClass == "" {
			device.VisibilityClass = firstInstallObjectString(legacyState, "visibilityClass")
		}
	}
	if device.UDID == "" {
		return installDevice{}, &installDeviceMembershipUnavailableError{}
	}
	if err := validateInstallText(device.UDID, "device UDID"); err != nil {
		return installDevice{}, fmt.Errorf("parse devicectl device discovery result: %w", err)
	}
	return device, nil
}

func decodeLegacyInstallObjects(entry installDeviceEntry) (map[string]json.RawMessage, map[string]json.RawMessage, map[string]json.RawMessage) {
	hardware, _ := decodeInstallObject(entry.HardwareProperties)
	connection, _ := decodeInstallObject(entry.ConnectionProperties)
	state, _ := decodeInstallObject(entry.DeviceProperties)
	return hardware, connection, state
}

func decodeInstallObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func objectFromObject(parent map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	raw, ok := parent[key]
	if !ok {
		return nil, nil
	}
	return decodeInstallObject(raw)
}

func firstInstallObjectString(values map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isUsableInstallDevice(device installDevice) bool {
	if !strings.EqualFold(device.Platform, "iOS") || !strings.EqualFold(device.Reality, "physical") || !strings.EqualFold(device.PairingState, "paired") {
		return false
	}
	if strings.EqualFold(device.ConnectionState, "disconnected") || strings.EqualFold(device.ConnectionState, "unavailable") || strings.EqualFold(device.ConnectionState, "offline") {
		return false
	}
	if !strings.EqualFold(device.ConnectionState, "connected") && !strings.EqualFold(device.ConnectionState, "available") {
		return false
	}
	if strings.Contains(strings.ToLower(device.VisibilityClass), "simulator") {
		return false
	}
	return true
}

func requiredInstallString(values map[string]json.RawMessage, keys ...string) (string, error) {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("parse devicectl result: %s must be a string", key)
		}
		if err := validateInstallText(value, key); err != nil {
			return "", fmt.Errorf("parse devicectl result: %w", err)
		}
		return value, nil
	}
	return "", fmt.Errorf("parse devicectl result: required field is missing")
}

func structuredInstallCommandError(stage string, envelope devicectlEnvelope, runErr error) error {
	value := &installCommandError{Stage: stage, Outcome: envelope.Info.Outcome, ExitCode: installExitCode(runErr)}
	if envelope.Error != nil {
		value.Code = envelope.Error.Code
	}
	return value
}

func installExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func installEnvironment(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	allowed := map[string]struct{}{
		"DEVELOPER_DIR": {},
		"HOME":          {},
		"LANG":          {},
		"LC_ALL":        {},
		"LC_CTYPE":      {},
		"PATH":          {},
		"SDKROOT":       {},
		"TEMP":          {},
		"TMP":           {},
		"TMPDIR":        {},
		"TOOLCHAINS":    {},
		"TZ":            {},
	}
	result := make([]string, 0, len(allowed))
	indexes := make(map[string]int, len(allowed))
	for _, entry := range base {
		if strings.ContainsRune(entry, '\x00') {
			continue
		}
		name, _, found := strings.Cut(entry, "=")
		if !found || name == "" {
			continue
		}
		if _, ok := allowed[name]; !ok {
			continue
		}
		entry = strings.Clone(entry)
		if index, ok := indexes[name]; ok {
			result[index] = entry
			continue
		}
		indexes[name] = len(result)
		result = append(result, entry)
	}
	return result
}
