package xcode

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
)

var (
	runtimeGOOS          = runtime.GOOS
	lookPathFn           = exec.LookPath
	commandContextFn     = exec.CommandContext
	activeDeveloperDirFn = activeDeveloperDir
	altoolHelpOutputFn   = readAltoolHelpOutput

	altoolValidationErrorPrefixes = [...]*regexp.Regexp{
		regexp.MustCompile(`(?i)^[[:space:]]*\*{3}[[:space:]]*error:[[:space:]]*`),
		regexp.MustCompile(`(?i)^[[:space:]]*[0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]]+[0-9]{2}:[0-9]{2}:[0-9]{2}([.][0-9]+)?[[:space:]]+error:[[:space:]]*`),
	}
)

const xcodebuildErrorTailLimit = 64 * 1024

type ArchiveOptions struct {
	WorkspacePath  string
	ProjectPath    string
	Scheme         string
	Configuration  string
	ArchivePath    string
	Clean          bool
	Overwrite      bool
	XcodebuildArgs []string
	LogWriter      io.Writer
}

type ArchiveResult struct {
	ArchivePath   string `json:"archive_path"`
	BundleID      string `json:"bundle_id,omitempty"`
	Version       string `json:"version,omitempty"`
	BuildNumber   string `json:"build_number,omitempty"`
	Scheme        string `json:"scheme,omitempty"`
	Configuration string `json:"configuration,omitempty"`
}

type ExportOptions struct {
	ArchivePath    string
	ExportOptions  string
	IPAPath        string
	Overwrite      bool
	XcodebuildArgs []string
	LogWriter      io.Writer
}

type ExportResult struct {
	ArchivePath string `json:"archive_path"`
	IPAPath     string `json:"ipa_path"`
	BundleID    string `json:"bundle_id,omitempty"`
	Version     string `json:"version,omitempty"`
	BuildNumber string `json:"build_number,omitempty"`
}

type ValidateOptions struct {
	IPAPath   string
	APIKey    string
	APIIssuer string
	LogWriter io.Writer
}

type ValidateResult struct {
	IPAPath   string `json:"ipa_path"`
	Validated bool   `json:"validated"`
}

type BuildStatusOptions struct {
	AppleID            string
	BundleID           string
	BundleVersion      string
	BundleShortVersion string
	Platform           string
	APIKey             string
	APIIssuer          string
	P8FilePath         string
	LogWriter          io.Writer
}

type BuildStatusResult struct {
	BuildStatus      string   `json:"build_status,omitempty"`
	DeliveryUUID     string   `json:"delivery_uuid,omitempty"`
	ImportStatus     string   `json:"import_status,omitempty"`
	ProcessingErrors []string `json:"processing_errors,omitempty"`
}

type bundleInfo struct {
	BundleID    string
	Version     string
	BuildNumber string
	Platform    string
}

type exportDestinationUsageError struct {
	message string
}

func (e exportDestinationUsageError) Error() string {
	return e.message
}

// IsExportDestinationUsageError reports whether destination preflight failed
// because of deterministic CLI input rather than a filesystem access error.
func IsExportDestinationUsageError(err error) bool {
	var usageErr exportDestinationUsageError
	return errors.As(err, &usageErr)
}

// PreflightExport verifies that local Xcode export tooling is available before
// callers perform any preparatory filesystem mutation.
func PreflightExport(ctx context.Context) error {
	return ensureXcodeAvailable(ctx)
}

// ValidateExportDestination checks an IPA destination without mutating the
// filesystem.
func ValidateExportDestination(ipaPath string, overwrite, directUpload bool) error {
	ipaPath = strings.TrimSpace(ipaPath)
	if ipaPath == "" {
		return exportDestinationUsageError{message: "--ipa-path is required"}
	}
	if !strings.EqualFold(filepath.Ext(ipaPath), ".ipa") {
		return exportDestinationUsageError{message: "--ipa-path must end with .ipa"}
	}
	if directUpload {
		return nil
	}
	info, err := os.Lstat(ipaPath)
	switch {
	case err == nil && info.IsDir():
		return exportDestinationUsageError{message: fmt.Sprintf("--ipa-path must not be a directory: %s", ipaPath)}
	case err == nil && !overwrite:
		return exportDestinationUsageError{message: fmt.Sprintf("--ipa-path already exists: %s (use --overwrite to replace it)", ipaPath)}
	case err == nil && overwrite:
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("lstat ipa path: %w", err)
	}
}

// PreflightExportDestination validates an IPA destination and proves its
// parent writable with a transient probe. Export repeats the check before the
// final mutation.
func PreflightExportDestination(ipaPath string, overwrite, directUpload bool) error {
	if err := ValidateExportDestination(ipaPath, overwrite, directUpload); err != nil {
		return err
	}
	return preflightWritableParent(strings.TrimSpace(ipaPath), "ipa output")
}

func preflightWritableParent(path, description string) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create %s parent directory: %w", description, err)
	}
	probe, err := os.CreateTemp(parent, ".asc-output-preflight-*")
	if err != nil {
		return fmt.Errorf("preflight %s parent: %w", description, err)
	}
	probePath := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(probePath)
	if err := errors.Join(closeErr, removeErr); err != nil {
		return fmt.Errorf("clean up %s parent preflight: %w", description, err)
	}
	return nil
}

func Archive(ctx context.Context, opts ArchiveOptions) (*ArchiveResult, error) {
	opts = normalizeArchiveOptions(opts)
	if err := validateArchiveOptions(opts); err != nil {
		return nil, err
	}
	if err := ensureXcodeAvailable(ctx); err != nil {
		return nil, err
	}
	if err := validateArchiveInputPaths(opts); err != nil {
		return nil, err
	}
	if err := prepareArchiveDestination(opts.ArchivePath, opts.Overwrite); err != nil {
		return nil, err
	}

	args := buildArchiveCommand(opts)
	if err := runXcodebuild(ctx, args, opts.LogWriter); err != nil {
		return nil, err
	}

	info, err := readArchiveBundleInfo(opts.ArchivePath)
	if err != nil {
		return nil, err
	}

	return &ArchiveResult{
		ArchivePath:   opts.ArchivePath,
		BundleID:      info.BundleID,
		Version:       info.Version,
		BuildNumber:   info.BuildNumber,
		Scheme:        strings.TrimSpace(opts.Scheme),
		Configuration: strings.TrimSpace(opts.Configuration),
	}, nil
}

func Export(ctx context.Context, opts ExportOptions) (*ExportResult, error) {
	opts = normalizeExportOptions(opts)
	if err := validateExportOptions(opts); err != nil {
		return nil, err
	}
	if err := ensureXcodeAvailable(ctx); err != nil {
		return nil, err
	}
	if err := validateExportInputPaths(opts); err != nil {
		return nil, err
	}

	// Always ensure parent dir exists (needed for temp dir creation below).
	if err := os.MkdirAll(filepath.Dir(opts.IPAPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	// When the ExportOptions plist has destination=upload, xcodebuild uploads
	// directly to App Store Connect and does not produce a local .ipa file.
	// This is the normal path for tvOS and some macOS exports. Detect this
	// mode before prepareIPAPath to avoid deleting an existing IPA that will
	// never be replaced.
	uploadMode := isDirectUploadMode(opts.ExportOptions)

	if !uploadMode {
		if err := prepareIPAPath(opts.IPAPath, opts.Overwrite); err != nil {
			return nil, err
		}
	}
	maybeWarnAboutBetaXcodeForAppStoreExport(ctx, opts.ExportOptions, opts.LogWriter)

	tempExportDir, err := os.MkdirTemp(filepath.Dir(opts.IPAPath), ".asc-xcode-export-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary export directory: %w", err)
	}
	defer os.RemoveAll(tempExportDir)

	args := buildExportCommand(opts, tempExportDir)
	if err := runXcodebuild(ctx, args, opts.LogWriter); err != nil {
		return nil, err
	}

	if uploadMode {
		// xcodebuild uploaded directly — no local IPA produced.
		info, err := readArchiveBundleInfo(opts.ArchivePath)
		if err != nil {
			return nil, fmt.Errorf("read archive bundle info after direct upload: %w", err)
		}
		return &ExportResult{
			ArchivePath: opts.ArchivePath,
			IPAPath:     "",
			BundleID:    info.BundleID,
			Version:     info.Version,
			BuildNumber: info.BuildNumber,
		}, nil
	}

	exportedIPAPath, err := findExportedIPA(tempExportDir)
	if err != nil {
		return nil, err
	}
	info, err := readIPABundleInfo(exportedIPAPath)
	if err != nil {
		return nil, fmt.Errorf("inspect exported IPA before installation: %w", err)
	}
	if err := moveExportedIPA(exportedIPAPath, opts.IPAPath, opts.Overwrite); err != nil {
		return nil, err
	}

	return &ExportResult{
		ArchivePath: opts.ArchivePath,
		IPAPath:     opts.IPAPath,
		BundleID:    info.BundleID,
		Version:     info.Version,
		BuildNumber: info.BuildNumber,
	}, nil
}

func Validate(ctx context.Context, opts ValidateOptions) (*ValidateResult, error) {
	opts = normalizeValidateOptions(opts)
	if err := validateValidateOptions(opts); err != nil {
		return nil, err
	}
	if err := ensureXcodeAvailable(ctx); err != nil {
		return nil, err
	}
	if _, err := lookPathFn("xcrun"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("xcrun not available; install Xcode and ensure the active developer directory is configured")
		}
		return nil, fmt.Errorf("locate xcrun: %w", err)
	}
	if err := validateExistingFile(opts.IPAPath, "--ipa"); err != nil {
		return nil, err
	}
	platform, err := inferValidatePlatform(opts.IPAPath)
	if err != nil {
		return nil, err
	}
	if err := runAltoolValidate(ctx, buildValidateCommand(opts, platform), opts.LogWriter); err != nil {
		return nil, err
	}
	return &ValidateResult{
		IPAPath:   opts.IPAPath,
		Validated: true,
	}, nil
}

func BuildStatus(ctx context.Context, opts BuildStatusOptions) (*BuildStatusResult, error) {
	opts = normalizeBuildStatusOptions(opts)
	if err := validateBuildStatusOptions(opts); err != nil {
		return nil, err
	}
	if err := ensureXcodeAvailable(ctx); err != nil {
		return nil, err
	}
	if _, err := lookPathFn("xcrun"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("xcrun not available; install Xcode and ensure the active developer directory is configured")
		}
		return nil, fmt.Errorf("locate xcrun: %w", err)
	}
	if opts.P8FilePath != "" {
		if err := validateExistingFile(opts.P8FilePath, "--p8-file-path"); err != nil {
			return nil, err
		}
	}

	output, err := runAltoolAndCapture(ctx, buildBuildStatusCommand(opts), opts.LogWriter, "build-status")
	if err != nil {
		return nil, err
	}
	return parseBuildStatusOutput(output), nil
}

// SupportsBuildStatusBundleID reports whether the current altool help output
// advertises a dedicated --bundle-id flag for build-status lookups.
func SupportsBuildStatusBundleID(ctx context.Context) bool {
	helpOutput, err := altoolHelpOutputFn(ctx)
	if err != nil {
		return false
	}
	return strings.Contains(helpOutput, "--bundle-id ")
}

// IsDirectUploadMode reports whether ExportOptions.plist uploads directly to
// App Store Connect instead of producing a local IPA artifact.
func IsDirectUploadMode(exportOptionsPlistPath string) bool {
	return isDirectUploadMode(exportOptionsPlistPath)
}

// InferArchivePlatform returns the App Store platform for the archived app by
// reading the embedded app Info.plist inside the .xcarchive.
func InferArchivePlatform(archivePath string) (string, error) {
	info, err := readArchiveBundleInfo(archivePath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(info.Platform) == "" {
		return "", fmt.Errorf("could not infer App Store platform from archive")
	}
	return info.Platform, nil
}

func validateArchiveOptions(opts ArchiveOptions) error {
	if err := validateWorkspaceProjectPair(opts.WorkspacePath, opts.ProjectPath); err != nil {
		return err
	}
	if opts.Scheme == "" {
		return fmt.Errorf("--scheme is required")
	}
	if opts.ArchivePath == "" {
		return fmt.Errorf("--archive-path is required")
	}
	if !strings.EqualFold(filepath.Ext(opts.ArchivePath), ".xcarchive") {
		return fmt.Errorf("--archive-path must end with .xcarchive")
	}
	return nil
}

func validateArchiveInputPaths(opts ArchiveOptions) error {
	if opts.WorkspacePath != "" {
		if err := validateExistingPath(opts.WorkspacePath, ".xcworkspace", "--workspace"); err != nil {
			return err
		}
	}
	if opts.ProjectPath != "" {
		if err := validateExistingPath(opts.ProjectPath, ".xcodeproj", "--project"); err != nil {
			return err
		}
	}
	return nil
}

func validateExportOptions(opts ExportOptions) error {
	if opts.ArchivePath == "" {
		return fmt.Errorf("--archive-path is required")
	}
	if opts.ExportOptions == "" {
		return fmt.Errorf("--export-options is required")
	}
	if opts.IPAPath == "" {
		return fmt.Errorf("--ipa-path is required")
	}
	if !strings.EqualFold(filepath.Ext(opts.IPAPath), ".ipa") {
		return fmt.Errorf("--ipa-path must end with .ipa")
	}
	if err := ValidateExportXcodebuildArgs(opts.XcodebuildArgs); err != nil {
		return err
	}
	return nil
}

// ValidateExportXcodebuildArgs rejects passthrough arguments that would switch
// xcodebuild away from the export operation or override paths managed by asc.
func ValidateExportXcodebuildArgs(args []string) error {
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("--xcodebuild-flag cannot be empty")
		}
	}
	if reserved := reservedExportPassthroughArgument(args); reserved != "" {
		return fmt.Errorf("--xcodebuild-flag cannot override asc-managed argument %q", reserved)
	}
	return nil
}

func reservedExportPassthroughArgument(args []string) string {
	if reserved := reservedBuildPassthroughArgument(args); reserved != "" {
		return reserved
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		trimmed := strings.TrimSpace(arg)
		normalized := strings.ToLower(trimmed)
		if xcodebuildPassthroughArgumentTakesValue(normalized) {
			index++
			continue
		}
		for _, managed := range []string{"-exportpath", "-exportoptionsplist"} {
			if normalized == managed || strings.HasPrefix(normalized, managed+"=") {
				return strings.SplitN(trimmed, "=", 2)[0]
			}
		}
	}
	return ""
}

func validateExportInputPaths(opts ExportOptions) error {
	if err := validateExistingPath(opts.ArchivePath, ".xcarchive", "--archive-path"); err != nil {
		return err
	}
	if err := validateExistingFile(opts.ExportOptions, "--export-options"); err != nil {
		return err
	}
	return nil
}

func validateValidateOptions(opts ValidateOptions) error {
	if opts.IPAPath == "" {
		return fmt.Errorf("--ipa is required")
	}
	if !strings.EqualFold(filepath.Ext(opts.IPAPath), ".ipa") {
		return fmt.Errorf("--ipa must end with .ipa")
	}
	if (opts.APIKey == "") != (opts.APIIssuer == "") {
		return fmt.Errorf("--api-key and --api-issuer must be provided together")
	}
	return nil
}

func validateBuildStatusOptions(opts BuildStatusOptions) error {
	if opts.AppleID == "" {
		return fmt.Errorf("--apple-id is required")
	}
	if opts.BundleVersion == "" {
		return fmt.Errorf("--bundle-version is required")
	}
	if (opts.APIKey == "") != (opts.APIIssuer == "") {
		return fmt.Errorf("--api-key and --api-issuer must be provided together")
	}
	return nil
}

func validateWorkspaceProjectPair(workspacePath, projectPath string) error {
	hasWorkspace := workspacePath != ""
	hasProject := projectPath != ""
	if hasWorkspace == hasProject {
		return fmt.Errorf("exactly one of --workspace or --project is required")
	}
	return nil
}

func normalizeArchiveOptions(opts ArchiveOptions) ArchiveOptions {
	opts.WorkspacePath = normalizeDirectoryPath(opts.WorkspacePath)
	opts.ProjectPath = normalizeDirectoryPath(opts.ProjectPath)
	opts.Scheme = strings.TrimSpace(opts.Scheme)
	opts.Configuration = strings.TrimSpace(opts.Configuration)
	opts.ArchivePath = normalizeDirectoryPath(opts.ArchivePath)
	return opts
}

func normalizeExportOptions(opts ExportOptions) ExportOptions {
	opts.ArchivePath = normalizeDirectoryPath(opts.ArchivePath)
	opts.ExportOptions = strings.TrimSpace(opts.ExportOptions)
	opts.IPAPath = strings.TrimSpace(opts.IPAPath)
	return opts
}

func normalizeValidateOptions(opts ValidateOptions) ValidateOptions {
	opts.IPAPath = strings.TrimSpace(opts.IPAPath)
	opts.APIKey = strings.TrimSpace(opts.APIKey)
	opts.APIIssuer = strings.TrimSpace(opts.APIIssuer)
	return opts
}

func normalizeBuildStatusOptions(opts BuildStatusOptions) BuildStatusOptions {
	opts.AppleID = strings.TrimSpace(opts.AppleID)
	opts.BundleID = strings.TrimSpace(opts.BundleID)
	opts.BundleVersion = strings.TrimSpace(opts.BundleVersion)
	opts.BundleShortVersion = strings.TrimSpace(opts.BundleShortVersion)
	opts.Platform = strings.TrimSpace(opts.Platform)
	opts.APIKey = strings.TrimSpace(opts.APIKey)
	opts.APIIssuer = strings.TrimSpace(opts.APIIssuer)
	opts.P8FilePath = strings.TrimSpace(opts.P8FilePath)
	return opts
}

func normalizeDirectoryPath(pathValue string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func validateExistingPath(pathValue, suffix, flagName string) error {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return fmt.Errorf("%s is required", flagName)
	}
	normalized := filepath.Clean(trimmed)
	if !strings.EqualFold(filepath.Ext(normalized), suffix) {
		return fmt.Errorf("%s must end with %s", flagName, suffix)
	}
	info, err := os.Stat(normalized)
	if err != nil {
		return fmt.Errorf("%s: %w", flagName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s must point to a directory", flagName)
	}
	return nil
}

func validateExistingFile(pathValue, flagName string) error {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return fmt.Errorf("%s is required", flagName)
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return fmt.Errorf("%s: %w", flagName, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must point to a file", flagName)
	}
	return nil
}

func ensureXcodeAvailable(ctx context.Context) error {
	if runtimeGOOS != "darwin" {
		return fmt.Errorf("supported on macOS only; current platform is %s", runtimeGOOS)
	}
	if _, err := lookPathFn("xcodebuild"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("xcodebuild not available; install Xcode and ensure the active developer directory is configured")
		}
		return fmt.Errorf("locate xcodebuild: %w", err)
	}
	if err := runXcodebuild(ctx, []string{"-version"}, io.Discard); err != nil {
		return fmt.Errorf("xcodebuild not usable: %w", err)
	}
	return nil
}

func maybeWarnAboutBetaXcodeForAppStoreExport(ctx context.Context, exportOptionsPath string, logWriter io.Writer) {
	if logWriter == nil || !isAppStoreExport(exportOptionsPath) {
		return
	}
	developerDir, err := activeDeveloperDirFn(ctx)
	if err != nil || !isBetaXcodePath(developerDir) {
		return
	}
	fmt.Fprintf(
		logWriter,
		"Warning: active Xcode developer directory %q appears to be a beta build. App Store Connect may accept uploads from beta Xcode, but App Store review can later reject builds for unsupported SDK/Xcode. Prefer a stable Xcode via DEVELOPER_DIR or xcode-select for App Store submission exports.\n",
		developerDir,
	)
}

func isAppStoreExport(exportOptionsPath string) bool {
	data, err := os.ReadFile(strings.TrimSpace(exportOptionsPath))
	if err != nil {
		return false
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return false
	}

	method, _ := payload["method"].(string)
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "app-store", "app-store-connect":
		return true
	}

	destination, _ := payload["destination"].(string)
	return strings.EqualFold(strings.TrimSpace(destination), "upload")
}

func activeDeveloperDir(ctx context.Context) (string, error) {
	if developerDir := strings.TrimSpace(os.Getenv("DEVELOPER_DIR")); developerDir != "" {
		return filepath.Clean(developerDir), nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "xcode-select", "-p")
	output, err := outputXcodeCommand(cmd)
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(string(output))), nil
}

func isBetaXcodePath(pathValue string) bool {
	if strings.TrimSpace(pathValue) == "" {
		return false
	}
	for _, segment := range strings.Split(filepath.Clean(pathValue), string(os.PathSeparator)) {
		normalized := strings.ToLower(strings.TrimSpace(segment))
		if strings.Contains(normalized, "xcode") && strings.Contains(normalized, "beta") {
			return true
		}
	}
	return false
}

func buildArchiveCommand(opts ArchiveOptions) []string {
	args := make([]string, 0, 16+len(opts.XcodebuildArgs))
	if trimmed := strings.TrimSpace(opts.WorkspacePath); trimmed != "" {
		args = append(args, "-workspace", trimmed)
	}
	if trimmed := strings.TrimSpace(opts.ProjectPath); trimmed != "" {
		args = append(args, "-project", trimmed)
	}
	args = append(args, "-scheme", strings.TrimSpace(opts.Scheme))
	if trimmed := strings.TrimSpace(opts.Configuration); trimmed != "" {
		args = append(args, "-configuration", trimmed)
	}
	args = append(args, cloneStrings(opts.XcodebuildArgs)...)
	if opts.Clean {
		args = append(args, "clean")
	}
	args = append(args, "archive", "-archivePath", strings.TrimSpace(opts.ArchivePath))
	return args
}

func buildExportCommand(opts ExportOptions, exportDir string) []string {
	args := []string{
		"-exportArchive",
		"-archivePath", strings.TrimSpace(opts.ArchivePath),
		"-exportPath", exportDir,
		"-exportOptionsPlist", strings.TrimSpace(opts.ExportOptions),
	}
	args = append(args, cloneStrings(opts.XcodebuildArgs)...)
	return args
}

func inferValidatePlatform(ipaPath string) (string, error) {
	info, err := readIPABundleInfo(ipaPath)
	if err != nil {
		return "", fmt.Errorf("inspect IPA metadata before validation: %w", err)
	}
	if platform := mapAppStorePlatformToAltoolType(info.Platform); platform != "" {
		return platform, nil
	}
	return "ios", nil
}

func buildValidateCommand(opts ValidateOptions, platform string) []string {
	if strings.TrimSpace(platform) == "" {
		platform = "ios"
	}
	args := []string{
		"altool",
		"--validate-app",
		"--file", opts.IPAPath,
		"--type", platform,
	}
	if opts.APIKey != "" {
		args = append(args, "--apiKey", opts.APIKey)
	}
	if opts.APIIssuer != "" {
		args = append(args, "--apiIssuer", opts.APIIssuer)
	}
	return args
}

func buildBuildStatusCommand(opts BuildStatusOptions) []string {
	platform := mapAppStorePlatformToAltoolType(opts.Platform)
	if platform == "" {
		platform = "ios"
	}
	args := []string{
		"altool",
		"--build-status",
		"--apple-id", opts.AppleID,
		"--bundle-version", opts.BundleVersion,
		"--platform", platform,
		"--output-format", "json",
	}
	if opts.BundleID != "" {
		args = append(args, "--bundle-id", opts.BundleID)
	}
	if opts.BundleShortVersion != "" {
		args = append(args, "--bundle-short-version-string", opts.BundleShortVersion)
	}
	if opts.APIKey != "" {
		args = append(args, "--apiKey", opts.APIKey)
	}
	if opts.APIIssuer != "" {
		args = append(args, "--apiIssuer", opts.APIIssuer)
	}
	if opts.P8FilePath != "" {
		args = append(args, "--p8-file-path", opts.P8FilePath)
	}
	return args
}

func mapAppStorePlatformToAltoolType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IOS":
		return "ios"
	case "TV_OS":
		return "appletvos"
	case "VISION_OS":
		return "visionos"
	case "MAC_OS":
		return "macos"
	default:
		return ""
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	return cloned
}

func runXcodebuild(ctx context.Context, args []string, logWriter io.Writer) error {
	return runCommandWithBoundedOutput(ctx, "xcodebuild", args, logWriter, summarizeAction(args), "xcodebuild")
}

func runXcodebuildForBuild(ctx context.Context, args []string, logWriter io.Writer) error {
	return runCommandWithBoundedOutputMode(ctx, "xcodebuild", args, logWriter, summarizeAction(args), "xcodebuild", true)
}

func runAltoolValidate(ctx context.Context, args []string, logWriter io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, "xcrun", args...)
	outputTail := newTailBuffer(xcodebuildErrorTailLimit)
	combinedOutput := io.Writer(outputTail)
	if logWriter != nil {
		combinedOutput = io.MultiWriter(logWriter, outputTail)
	}
	serializedOutput := &synchronizedWriter{writer: combinedOutput}
	stdoutOutput := newAltoolValidationOutputWriter(serializedOutput)
	stderrOutput := newAltoolValidationOutputWriter(serializedOutput)
	cmd.Stdout = stdoutOutput
	cmd.Stderr = stderrOutput
	if err := runXcodeCommand(cmd); err != nil {
		failureOutput := newAltoolValidationFailureOutput(
			append(stdoutOutput.Details(), stderrOutput.Details()...),
			outputTail,
			xcodebuildErrorTailLimit,
		)
		return formatCommandOutputError(ctx, err, failureOutput, "validate", "xcrun altool", true)
	}

	details := append(stdoutOutput.Details(), stderrOutput.Details()...)
	details = UniqueDiagnosticDetails(details)
	details = boundDiagnosticDetails(details, xcodebuildErrorTailLimit)
	if len(details) == 0 {
		return nil
	}
	return fmt.Errorf("xcrun altool validate failed: %s", strings.Join(details, "; "))
}

// altoolValidationFailureOutput renders the diagnostics altool already
// classified ahead of the raw output tail, so a recognized failure survives the
// trailing upload progress noise that would otherwise evict it. It mirrors
// xcodeDiagnosticBuffer: an untruncated tail is reported verbatim, and the
// rendered message stays within the same byte budget.
type altoolValidationFailureOutput struct {
	details []string
	tail    *tailBuffer
	limit   int
}

func newAltoolValidationFailureOutput(details []string, tail *tailBuffer, limit int) *altoolValidationFailureOutput {
	return &altoolValidationFailureOutput{
		details: UniqueDiagnosticDetails(details),
		tail:    tail,
		limit:   max(0, limit),
	}
}

func (o *altoolValidationFailureOutput) String() string {
	tail := strings.TrimSpace(o.tail.String())
	if !o.tail.Truncated() {
		return tail
	}

	detail := strings.Join(boundDiagnosticDetails(o.details, min(o.limit/2, xcodeDiagnosticPrefixLimit)), "; ")
	switch {
	case detail == "":
		return tail
	case tail == "":
		return detail
	}

	tail = strings.TrimSpace(truncateUTF8Suffix(tail, o.limit-len(detail)-len("\n")))
	if tail == "" {
		return detail
	}
	return detail + "\n" + tail
}

func (o *altoolValidationFailureOutput) Truncated() bool {
	return o.tail.Truncated()
}

func (o *altoolValidationFailureOutput) TruncationDescription() string {
	if len(o.details) == 0 {
		return o.tail.TruncationDescription()
	}
	return fmt.Sprintf("output truncated to %d bytes; preserving recognized errors and final output", o.limit)
}

func boundDiagnosticDetails(details []string, maxBytes int) []string {
	if maxBytes <= 0 {
		return nil
	}

	bounded := make([]string, 0, len(details))
	usedBytes := 0
	for _, detail := range details {
		separatorBytes := 0
		if len(bounded) > 0 {
			separatorBytes = len("; ")
		}
		remaining := maxBytes - usedBytes - separatorBytes
		if remaining <= 0 {
			break
		}

		boundedDetail := truncateUTF8Prefix(detail, remaining)
		if boundedDetail == "" {
			break
		}
		bounded = append(bounded, boundedDetail)
		usedBytes += separatorBytes + len(boundedDetail)
		if len(boundedDetail) < len(detail) {
			break
		}
	}
	return bounded
}

type altoolValidationOutputWriter struct {
	logWriter   io.Writer
	line        []byte
	details     []string
	seen        map[string]struct{}
	detailBytes int
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func newAltoolValidationOutputWriter(logWriter io.Writer) *altoolValidationOutputWriter {
	return &altoolValidationOutputWriter{
		logWriter: logWriter,
		seen:      make(map[string]struct{}),
	}
}

func (w *altoolValidationOutputWriter) Write(p []byte) (int, error) {
	written := len(p)
	var err error
	if w.logWriter != nil {
		written, err = w.logWriter.Write(p)
	}
	w.consume(p[:written])
	return written, err
}

func (w *altoolValidationOutputWriter) Details() []string {
	if len(w.line) > 0 {
		w.recordLine()
	}
	return append([]string(nil), w.details...)
}

func (w *altoolValidationOutputWriter) consume(p []byte) {
	for len(p) > 0 {
		newline := bytes.IndexByte(p, '\n')
		if newline < 0 {
			w.appendLine(p)
			return
		}
		w.appendLine(p[:newline])
		w.recordLine()
		p = p[newline+1:]
	}
}

func (w *altoolValidationOutputWriter) appendLine(fragment []byte) {
	remaining := xcodebuildErrorTailLimit - len(w.line)
	if remaining <= 0 {
		return
	}
	if len(fragment) > remaining {
		fragment = fragment[:remaining]
	}
	w.line = append(w.line, fragment...)
}

func (w *altoolValidationOutputWriter) recordLine() {
	detail, ok := parseAltoolValidationErrorLine(string(w.line))
	w.line = w.line[:0]
	if !ok {
		return
	}

	separatorBytes := 0
	if len(w.details) > 0 {
		separatorBytes = len("; ")
	}
	remaining := xcodebuildErrorTailLimit - w.detailBytes - separatorBytes
	if remaining <= 0 {
		return
	}
	detail = truncateUTF8Prefix(detail, remaining)
	if detail == "" {
		return
	}
	if _, exists := w.seen[detail]; exists {
		return
	}
	w.seen[detail] = struct{}{}
	w.details = append(w.details, detail)
	w.detailBytes += separatorBytes + len(detail)
}

func parseAltoolValidationErrorLine(line string) (string, bool) {
	for _, pattern := range altoolValidationErrorPrefixes {
		if match := pattern.FindStringIndex(line); match != nil {
			detail := strings.TrimSpace(line[match[1]:])
			if detail == "" {
				detail = "altool reported an unspecified validation error"
			}
			return detail, true
		}
	}
	return "", false
}

func truncateUTF8Prefix(value string, limit int) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	if len(value) <= limit {
		return value
	}
	prefix := []byte(value[:limit])
	for len(prefix) > 0 && !utf8.Valid(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return string(prefix)
}

func runAltoolAndCapture(ctx context.Context, args []string, logWriter io.Writer, action string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, "xcrun", args...)
	var stdout strings.Builder
	var stderr strings.Builder
	outputTail := newTailBuffer(xcodebuildErrorTailLimit)
	stdoutWriter := io.Writer(&stdout)
	stderrWriter := io.Writer(io.MultiWriter(&stderr, outputTail))
	if logWriter != nil {
		stdoutWriter = io.MultiWriter(logWriter, &stdout)
		stderrWriter = io.MultiWriter(logWriter, &stderr, outputTail)
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := runXcodeCommand(cmd); err != nil {
		detail := strings.TrimSpace(outputTail.String())
		if detail == "" {
			detail = strings.TrimSpace(mergeCapturedCommandOutput(stdout.String(), stderr.String()))
		}
		if detail != "" {
			if outputTail.Truncated() {
				return "", fmt.Errorf("xcrun altool %s failed (showing last %d bytes): %s", action, xcodebuildErrorTailLimit, detail)
			}
			return "", fmt.Errorf("xcrun altool %s failed: %s", action, detail)
		}
		return "", fmt.Errorf("xcrun altool %s failed: %w", action, err)
	}
	return mergeCapturedCommandOutput(stdout.String(), stderr.String()), nil
}

func mergeCapturedCommandOutput(stdout, stderr string) string {
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

func readAltoolHelpOutput(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, "xcrun", "altool", "--help")
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := runXcodeCommand(cmd); err != nil {
		return "", err
	}
	return mergeCapturedCommandOutput(stdout.String(), stderr.String()), nil
}

func parseBuildStatusOutput(output string) *BuildStatusResult {
	if result, ok := parseBuildStatusJSONOutput(output); ok {
		return result
	}
	return parseBuildStatusTextOutput(output)
}

func parseBuildStatusJSONOutput(output string) (*BuildStatusResult, bool) {
	payload, ok := extractBuildStatusJSONValue(output)
	if !ok {
		return nil, false
	}

	result := &BuildStatusResult{}
	populateBuildStatusResultFromJSON(result, payload)
	if result.BuildStatus == "" && result.DeliveryUUID == "" && result.ImportStatus == "" && len(result.ProcessingErrors) == 0 {
		return nil, false
	}
	result.ProcessingErrors = UniqueDiagnosticDetails(result.ProcessingErrors)
	return result, true
}

func extractBuildStatusJSONValue(output string) (any, bool) {
	for i := 0; i < len(output); i++ {
		if output[i] != '{' && output[i] != '[' {
			continue
		}

		decoder := json.NewDecoder(strings.NewReader(output[i:]))
		var payload any
		if err := decoder.Decode(&payload); err == nil {
			return payload, true
		}
	}
	return nil, false
}

func populateBuildStatusResultFromJSON(result *BuildStatusResult, value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			switch normalizeBuildStatusKey(key) {
			case "buildstatus":
				if result.BuildStatus == "" {
					result.BuildStatus = jsonStringValue(nested)
				}
			case "deliveryuuid", "deliveryid":
				if result.DeliveryUUID == "" {
					result.DeliveryUUID = jsonStringValue(nested)
				}
			case "importstatus":
				if result.ImportStatus == "" {
					result.ImportStatus = jsonStringValue(nested)
				}
			case "processingerrors":
				result.ProcessingErrors = append(result.ProcessingErrors, extractBuildStatusJSONProcessingErrors(nested)...)
			}
			populateBuildStatusResultFromJSON(result, nested)
		}
	case []any:
		for _, nested := range typed {
			populateBuildStatusResultFromJSON(result, nested)
		}
	}
}

func extractBuildStatusJSONProcessingErrors(value any) []string {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	case []any:
		var details []string
		for _, item := range typed {
			details = append(details, extractBuildStatusJSONProcessingErrors(item)...)
		}
		return details
	case map[string]any:
		var details []string
		for key, nested := range typed {
			switch normalizeBuildStatusKey(key) {
			case "code", "serverwarning", "serverwarnings":
				continue
			case "description", "detail", "details", "message", "messages":
				if text := jsonStringValue(nested); text != "" {
					details = append(details, text)
					continue
				}
			}
			details = append(details, extractBuildStatusJSONProcessingErrors(nested)...)
		}
		return details
	default:
		return nil
	}
}

func jsonStringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func normalizeBuildStatusKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func parseBuildStatusTextOutput(output string) *BuildStatusResult {
	result := &BuildStatusResult{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), max(bufio.MaxScanTokenSize, len(output)+1))
	inProcessingErrors := false

	for scanner.Scan() {
		line := normalizeBuildStatusLine(scanner.Text())
		if line == "" {
			continue
		}
		if inProcessingErrors {
			if key, value, ok := parseBuildStatusField(line); ok && isBuildStatusSummaryField(key) {
				inProcessingErrors = false
				assignBuildStatusField(result, key, value)
				continue
			}
			if detail := parseBuildStatusProcessingError(line); detail != "" {
				result.ProcessingErrors = append(result.ProcessingErrors, detail)
			}
			continue
		}
		if line == "PROCESSING-ERRORS:" {
			inProcessingErrors = true
			continue
		}
		if key, value, ok := parseBuildStatusField(line); ok {
			assignBuildStatusField(result, key, value)
		}
	}

	return result
}

func UniqueDiagnosticDetails(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	details := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		details = append(details, trimmed)
	}
	return details
}

func normalizeBuildStatusLine(raw string) string {
	line := strings.TrimSpace(raw)
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "=") {
		line = strings.TrimSpace(strings.TrimLeft(line, "= "))
	}
	return line
}

func parseBuildStatusField(line string) (string, string, bool) {
	index := strings.Index(line, ":")
	if index <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	value := strings.TrimSpace(line[index+1:])
	if key == "" {
		return "", "", false
	}
	for _, r := range key {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) && r != '-' {
			return "", "", false
		}
	}
	return key, value, true
}

func assignBuildStatusField(result *BuildStatusResult, key, value string) {
	if result == nil {
		return
	}
	switch normalizeBuildStatusKey(key) {
	case "buildstatus":
		result.BuildStatus = value
	case "deliveryuuid", "deliveryid":
		result.DeliveryUUID = value
	case "importstatus":
		result.ImportStatus = value
	}
}

func isBuildStatusSummaryField(key string) bool {
	switch normalizeBuildStatusKey(key) {
	case "buildstatus", "deliveryuuid", "deliveryid", "importstatus":
		return true
	default:
		return false
	}
}

func parseBuildStatusProcessingError(line string) string {
	key, value, ok := parseBuildStatusMetadataField(line)
	if ok {
		switch normalizeBuildStatusKey(key) {
		case "serverwarning", "serverwarnings", "code":
			return ""
		case "description":
			return value
		}
	}
	return strings.TrimSpace(line)
}

func parseBuildStatusMetadataField(line string) (string, string, bool) {
	index := strings.Index(line, ":")
	if index <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(line[index+1:]), true
}

func runCommandWithBoundedOutput(ctx context.Context, name string, args []string, logWriter io.Writer, action string, commandLabel string) error {
	return runCommandWithBoundedOutputMode(ctx, name, args, logWriter, action, commandLabel, false)
}

func runCommandWithBoundedOutputMode(ctx context.Context, name string, args []string, logWriter io.Writer, action string, commandLabel string, preserveProcessError bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, name, args...)
	outputWindow := newXcodeDiagnosticBuffer(xcodebuildErrorTailLimit, logWriter)
	cmd.Stdout = outputWindow
	cmd.Stderr = outputWindow
	if err := runXcodeCommand(cmd); err != nil {
		return formatCommandOutputError(ctx, err, outputWindow, action, commandLabel, preserveProcessError)
	}
	return nil
}

type formattedCommandOutput interface {
	String() string
	Truncated() bool
	TruncationDescription() string
}

func formatCommandOutputError(ctx context.Context, err error, output formattedCommandOutput, action string, commandLabel string, preserveProcessError bool) error {
	detail := strings.TrimSpace(output.String())
	if ctxErr := ctx.Err(); ctxErr != nil {
		if detail != "" {
			if output.Truncated() {
				return fmt.Errorf(
					"%s %s timed out or was canceled (%s): %s: %w",
					commandLabel,
					action,
					output.TruncationDescription(),
					detail,
					ctxErr,
				)
			}
			return fmt.Errorf("%s %s timed out or was canceled: %s: %w", commandLabel, action, detail, ctxErr)
		}
		return fmt.Errorf("%s %s timed out or was canceled: %w", commandLabel, action, ctxErr)
	}
	if detail != "" {
		if output.Truncated() {
			if preserveProcessError {
				return fmt.Errorf(
					"%s %s failed (%s): %s: %w",
					commandLabel,
					action,
					output.TruncationDescription(),
					detail,
					err,
				)
			}
			return fmt.Errorf(
				"%s %s failed (%s): %s",
				commandLabel,
				action,
				output.TruncationDescription(),
				detail,
			)
		}
		if preserveProcessError {
			return fmt.Errorf("%s %s failed: %s: %w", commandLabel, action, detail, err)
		}
		return fmt.Errorf("%s %s failed: %s", commandLabel, action, detail)
	}
	return fmt.Errorf("%s %s failed: %w", commandLabel, action, err)
}

const (
	xcodeDiagnosticLineLimit   = 8 * 1024
	xcodeDiagnosticPrefixLimit = 16 * 1024
)

var sourceLocationXcodeErrorPattern = regexp.MustCompile(
	`^.+:[0-9]+(:[0-9]+)?:[[:space:]]+(fatal[[:space:]]+)?error:[[:space:]]+[^[:space:]]`,
)

var xcodeErrorToolPrefixes = map[string]struct{}{
	"actool":     {},
	"clang":      {},
	"clang++":    {},
	"codesign":   {},
	"ibtool":     {},
	"ld":         {},
	"swiftc":     {},
	"xcodebuild": {},
	"xcrun":      {},
}

var xcodeDiagnosticPathExtensions = map[string]struct{}{
	".app":          {},
	".appex":        {},
	".c":            {},
	".cc":           {},
	".cpp":          {},
	".entitlements": {},
	".framework":    {},
	".h":            {},
	".hpp":          {},
	".m":            {},
	".metal":        {},
	".mm":           {},
	".modulemap":    {},
	".plist":        {},
	".storyboard":   {},
	".strings":      {},
	".swift":        {},
	".xcassets":     {},
	".xcconfig":     {},
	".xcodeproj":    {},
	".xcworkspace":  {},
	".xib":          {},
}

type xcodeDiagnosticBuffer struct {
	mu              sync.Mutex
	limit           int
	tail            *tailBuffer
	logWriter       io.Writer
	totalBytes      int64
	diagnostics     []string
	diagnosticSet   map[string]struct{}
	diagnosticBytes int
	pending         []byte
	overflow        bool
}

func newXcodeDiagnosticBuffer(limit int, logWriter io.Writer) *xcodeDiagnosticBuffer {
	return &xcodeDiagnosticBuffer{
		limit:         max(0, limit),
		tail:          newTailBuffer(max(0, limit)),
		logWriter:     logWriter,
		diagnosticSet: make(map[string]struct{}),
	}
}

func (b *xcodeDiagnosticBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.logWriter != nil {
		written, err := b.logWriter.Write(p)
		if err != nil {
			return written, err
		}
		if written != len(p) {
			return written, io.ErrShortWrite
		}
	}
	written := len(p)
	b.totalBytes += int64(written)
	_, _ = b.tail.Write(p)
	b.consumeDiagnosticLinesLocked(p)
	return written, nil
}

func (b *xcodeDiagnosticBuffer) consumeDiagnosticLinesLocked(p []byte) {
	for len(p) > 0 {
		newline := bytes.IndexByte(p, '\n')
		if newline < 0 {
			b.appendDiagnosticFragmentLocked(p)
			return
		}
		b.appendDiagnosticFragmentLocked(p[:newline])
		b.finishDiagnosticLineLocked()
		p = p[newline+1:]
	}
}

func (b *xcodeDiagnosticBuffer) appendDiagnosticFragmentLocked(fragment []byte) {
	remaining := xcodeDiagnosticLineLimit - len(b.pending)
	if remaining > 0 {
		prefixLength := min(remaining, len(fragment))
		b.pending = append(b.pending, fragment[:prefixLength]...)
		fragment = fragment[prefixLength:]
	}
	if len(fragment) > 0 {
		b.overflow = true
	}
}

func (b *xcodeDiagnosticBuffer) finishDiagnosticLineLocked() {
	if len(b.pending) == 0 && !b.overflow {
		return
	}

	line := strings.TrimSuffix(strings.ToValidUTF8(string(b.pending), ""), "\r")
	if b.overflow {
		const omissionMarker = "…"
		line = truncateUTF8Prefix(line, xcodeDiagnosticLineLimit-len(omissionMarker)) + omissionMarker
	}
	b.pending = b.pending[:0]
	b.overflow = false

	line = strings.TrimSpace(line)
	if !isXcodeErrorDiagnostic(line) {
		return
	}
	b.addDiagnosticLocked(line)
}

func (b *xcodeDiagnosticBuffer) addDiagnosticLocked(line string) {
	if _, exists := b.diagnosticSet[line]; exists {
		return
	}

	remaining := b.diagnosticBudget() - b.diagnosticBytes
	required := len(line) + 1
	if required > remaining {
		return
	}

	b.diagnosticSet[line] = struct{}{}
	b.diagnostics = append(b.diagnostics, line)
	b.diagnosticBytes += required
}

func (b *xcodeDiagnosticBuffer) diagnosticBudget() int {
	return min(b.limit/2, xcodeDiagnosticPrefixLimit)
}

func (b *xcodeDiagnosticBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.finishDiagnosticLineLocked()
	tail := strings.ToValidUTF8(b.tail.String(), "")
	if b.totalBytes <= int64(b.limit) {
		return tail
	}

	tailDiagnostics := newXcodeDiagnosticTailIndex(tail)
	diagnostics := make([]string, 0, len(tailDiagnostics.diagnostics)+len(b.diagnostics))
	diagnosticSet := make(map[string]struct{}, cap(diagnostics))
	for _, diagnostic := range append(tailDiagnostics.diagnostics, b.diagnostics...) {
		if _, exists := diagnosticSet[diagnostic]; exists {
			continue
		}
		diagnosticSet[diagnostic] = struct{}{}
		diagnostics = append(diagnostics, diagnostic)
	}

	prefixBytes := 0
	for {
		boundary := max(0, len(tail)-(b.limit-prefixBytes))
		newPrefixBytes := appendMissingXcodeDiagnostics(
			nil, diagnostics, tailDiagnostics, boundary, b.diagnosticBudget(),
		)
		if newPrefixBytes <= prefixBytes {
			break
		}
		prefixBytes = newPrefixBytes
	}
	boundary := max(0, len(tail)-(b.limit-prefixBytes))

	var prefix strings.Builder
	prefix.Grow(prefixBytes)
	appendMissingXcodeDiagnostics(&prefix, diagnostics, tailDiagnostics, boundary, b.diagnosticBudget())
	diagnosticPrefix := prefix.String()
	// Preserve the calculated tail boundary when complete-line packing leaves unused prefix space.
	tail = truncateUTF8Suffix(tail, b.limit-prefixBytes)
	return diagnosticPrefix + tail
}

func appendMissingXcodeDiagnostics(
	destination *strings.Builder,
	diagnostics []string,
	tailDiagnostics xcodeDiagnosticTailIndex,
	boundary int,
	limit int,
) int {
	written := 0
	for _, diagnostic := range diagnostics {
		if tailDiagnostics.containsAtOrAfter(diagnostic, boundary) {
			continue
		}
		required := len(diagnostic) + 1
		if required > limit-written {
			continue
		}
		if destination != nil {
			destination.WriteString(diagnostic)
			destination.WriteByte('\n')
		}
		written += required
	}
	return written
}

type xcodeDiagnosticTailIndex struct {
	lines       map[string]int
	diagnostics []string
}

func newXcodeDiagnosticTailIndex(tail string) xcodeDiagnosticTailIndex {
	index := xcodeDiagnosticTailIndex{lines: make(map[string]int)}
	diagnosticSet := make(map[string]struct{})
	offset := 0
	for {
		newline := strings.IndexByte(tail[offset:], '\n')
		lineEnd := len(tail)
		if newline >= 0 {
			lineEnd = offset + newline
		}
		line := tail[offset:lineEnd]
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			lineStart := offset + strings.Index(tail[offset:lineEnd], line)
			index.lines[line] = max(index.lines[line], lineStart)
			diagnostic := line
			if len(diagnostic) > xcodeDiagnosticLineLimit {
				const omissionMarker = "…"
				diagnostic = truncateUTF8Prefix(diagnostic, xcodeDiagnosticLineLimit-len(omissionMarker)) + omissionMarker
			}
			if isXcodeErrorDiagnostic(diagnostic) {
				if _, exists := diagnosticSet[diagnostic]; !exists {
					diagnosticSet[diagnostic] = struct{}{}
					index.diagnostics = append(index.diagnostics, diagnostic)
				}
			}
		}
		if newline < 0 {
			return index
		}
		offset = lineEnd + 1
	}
}

func (i xcodeDiagnosticTailIndex) containsAtOrAfter(diagnostic string, boundary int) bool {
	diagnostic = strings.TrimSpace(diagnostic)
	if diagnostic == "" {
		return false
	}
	if start, exists := i.lines[diagnostic]; exists && start >= boundary {
		return true
	}

	if !strings.HasSuffix(diagnostic, "…") {
		return false
	}
	match := strings.TrimSuffix(diagnostic, "…")
	for line, start := range i.lines {
		if start >= boundary && strings.HasPrefix(line, match) {
			return true
		}
	}
	return false
}

func (b *xcodeDiagnosticBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalBytes > int64(b.limit)
}

func (b *xcodeDiagnosticBuffer) TruncationDescription() string {
	return fmt.Sprintf("output truncated to %d bytes; preserving recognized errors and final output", b.limit)
}

func isXcodeErrorDiagnostic(line string) bool {
	line = strings.TrimSpace(line)
	if hasXcodeErrorMessagePrefix(line) || sourceLocationXcodeErrorPattern.MatchString(line) {
		return true
	}

	colon := strings.IndexByte(line, ':')
	if colon <= 0 {
		return false
	}
	prefix := line[:colon]
	if !hasXcodeErrorMessagePrefix(strings.TrimSpace(line[colon+1:])) {
		return false
	}
	if _, ok := xcodeErrorToolPrefixes[prefix]; ok {
		return true
	}
	if strings.ContainsRune(prefix, filepath.Separator) {
		return true
	}
	_, ok := xcodeDiagnosticPathExtensions[strings.ToLower(filepath.Ext(prefix))]
	return ok
}

func hasXcodeErrorMessagePrefix(line string) bool {
	for _, prefix := range []string{"error:", "fatal error:"} {
		if strings.HasPrefix(line, prefix) && strings.TrimSpace(line[len(prefix):]) != "" {
			return true
		}
	}
	return false
}

func truncateUTF8Suffix(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	value = value[len(value)-limit:]
	for len(value) > 0 && !utf8.RuneStart(value[0]) {
		value = value[1:]
	}
	return value
}

type tailBuffer struct {
	limit     int
	data      []byte
	truncated bool
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	if len(p) >= b.limit {
		b.data = append(b.data[:0], p[len(p)-b.limit:]...)
		b.truncated = true
		return len(p), nil
	}

	if overflow := len(b.data) + len(p) - b.limit; overflow > 0 {
		if overflow >= len(b.data) {
			b.data = b.data[:0]
		} else {
			b.data = append(b.data[:0], b.data[overflow:]...)
		}
		b.truncated = true
	}

	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *tailBuffer) String() string {
	data := b.data
	if b.truncated {
		for len(data) > 0 && !utf8.RuneStart(data[0]) {
			data = data[1:]
		}
	}
	return string(data)
}

func (b *tailBuffer) Truncated() bool {
	return b.truncated
}

func (b *tailBuffer) TruncationDescription() string {
	return fmt.Sprintf("showing last %d bytes", b.limit)
}

func summarizeAction(args []string) string {
	if containsArg(args, "-version") {
		return "-version"
	}
	if containsArg(args, "-exportArchive") {
		return "export"
	}
	if containsArg(args, "archive") {
		return "archive"
	}
	if containsArg(args, "build") {
		return "build"
	}
	return "command"
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func prepareArchiveDestination(archivePath string, overwrite bool) error {
	parent := filepath.Dir(archivePath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create archive output directory: %w", err)
	}
	_, err := os.Stat(archivePath)
	switch {
	case err == nil && !overwrite:
		return fmt.Errorf("--archive-path already exists: %s (use --overwrite to replace it)", archivePath)
	case err == nil && overwrite:
		if removeErr := os.RemoveAll(archivePath); removeErr != nil {
			return fmt.Errorf("remove existing archive path: %w", removeErr)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat archive path: %w", err)
	}
	return nil
}

func prepareIPAPath(ipaPath string, overwrite bool) error {
	parent := filepath.Dir(ipaPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create ipa output directory: %w", err)
	}
	_, err := os.Stat(ipaPath)
	switch {
	case err == nil && !overwrite:
		return fmt.Errorf("--ipa-path already exists: %s (use --overwrite to replace it)", ipaPath)
	case err == nil && overwrite:
		return nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat ipa path: %w", err)
	}
	return nil
}

func findExportedIPA(exportDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(exportDir, "*.ipa"))
	if err != nil {
		return "", fmt.Errorf("scan exported ipa: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("xcodebuild export did not produce an .ipa file")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("xcodebuild export produced multiple .ipa files")
	}
	return matches[0], nil
}

// isDirectUploadMode reads the ExportOptions plist and returns true when
// destination is set to "upload". In this mode xcodebuild uploads the build
// directly to App Store Connect and does not produce a local .ipa file.
func isDirectUploadMode(exportOptionsPlistPath string) bool {
	data, err := os.ReadFile(exportOptionsPlistPath)
	if err != nil {
		return false
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return false
	}
	dest, _ := payload["destination"].(string)
	return strings.EqualFold(dest, "upload")
}

func moveExportedIPA(sourcePath, destinationPath string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Lstat(destinationPath); err == nil {
			return fmt.Errorf("--ipa-path already exists: %s (use --overwrite to replace it)", destinationPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lstat ipa path: %w", err)
		}
	}
	// Export runs only on macOS, where rename replaces an existing regular file
	// atomically. Do not unlink the old artifact first: if the final move fails,
	// the caller's prior IPA remains intact.
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		return fmt.Errorf("move exported ipa: %w", err)
	}
	return nil
}

func readArchiveBundleInfo(archivePath string) (bundleInfo, error) {
	data, err := os.ReadFile(filepath.Join(archivePath, "Info.plist"))
	if err != nil {
		return bundleInfo{}, fmt.Errorf("read archive Info.plist: %w", err)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return bundleInfo{}, fmt.Errorf("decode archive Info.plist: %w", err)
	}
	appProps, _ := payload["ApplicationProperties"].(map[string]any)
	info := bundleInfo{
		BundleID:    coercePlistValueToString(appProps["CFBundleIdentifier"]),
		Version:     coercePlistValueToString(appProps["CFBundleShortVersionString"]),
		BuildNumber: coercePlistValueToString(appProps["CFBundleVersion"]),
	}
	if platform, err := inferArchivePlatformFromAppBundle(archivePath, appProps); err == nil {
		info.Platform = platform
	}
	return info, nil
}

func readIPABundleInfo(ipaPath string) (bundleInfo, error) {
	reader, err := zip.OpenReader(ipaPath)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("open IPA: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !isTopLevelAppInfoPlist(file.Name) {
			continue
		}
		return readBundleInfoFromZip(file)
	}
	return bundleInfo{}, fmt.Errorf("missing Info.plist in IPA")
}

func isTopLevelAppInfoPlist(name string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(name))
	if !strings.HasPrefix(cleaned, "Payload/") || !strings.HasSuffix(cleaned, "/Info.plist") {
		return false
	}
	dir := filepath.ToSlash(filepath.Dir(cleaned))
	if !strings.HasSuffix(dir, ".app") {
		return false
	}
	return filepath.ToSlash(filepath.Dir(dir)) == "Payload"
}

func readBundleInfoFromZip(file *zip.File) (bundleInfo, error) {
	if err := infoplist.CheckDeclaredSize(file.UncompressedSize64); err != nil {
		return bundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
	}

	reader, err := file.Open()
	if err != nil {
		return bundleInfo{}, fmt.Errorf("open Info.plist: %w", err)
	}
	defer reader.Close()

	data, err := infoplist.ReadBounded(reader)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return bundleInfo{}, fmt.Errorf("decode Info.plist: %w", err)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return bundleInfo{}, fmt.Errorf("decode Info.plist: %w", err)
	}
	return bundleInfo{
		BundleID:    coercePlistValueToString(payload["CFBundleIdentifier"]),
		Version:     coercePlistValueToString(payload["CFBundleShortVersionString"]),
		BuildNumber: coercePlistValueToString(payload["CFBundleVersion"]),
		Platform:    inferAppStorePlatformFromPlist(payload),
	}, nil
}

func inferArchivePlatformFromAppBundle(archivePath string, appProps map[string]any) (string, error) {
	payload, err := readArchivedAppInfoPlist(archivePath, appProps)
	if err != nil {
		return "", err
	}
	platform := inferAppStorePlatformFromPlist(payload)
	if platform == "" {
		return "", fmt.Errorf("archived app Info.plist did not contain a supported platform marker")
	}
	return platform, nil
}

func readArchivedAppInfoPlist(archivePath string, appProps map[string]any) (map[string]any, error) {
	archiveRoot, err := os.OpenRoot(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = archiveRoot.Close() }()
	return readArchivedAppInfoPlistFromRoot(archiveRoot, appProps)
}

func readArchivedAppInfoPlistFromRoot(archiveRoot *os.Root, appProps map[string]any) (map[string]any, error) {
	applicationPath := coercePlistValueToString(appProps["ApplicationPath"])
	if strings.TrimSpace(applicationPath) == "" {
		return nil, fmt.Errorf("archive Info.plist missing ApplicationPath")
	}

	relativeApplicationPath := filepath.Clean(filepath.FromSlash(applicationPath))
	if filepath.IsAbs(relativeApplicationPath) || relativeApplicationPath == ".." || strings.HasPrefix(relativeApplicationPath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("archive Info.plist contains unsafe ApplicationPath %q", applicationPath)
	}
	productsRoot, err := archiveRoot.OpenRoot("Products")
	if err != nil {
		return nil, fmt.Errorf("open archive Products directory: %w", err)
	}
	defer func() { _ = productsRoot.Close() }()
	appRoot, err := productsRoot.OpenRoot(relativeApplicationPath)
	if err != nil {
		return nil, fmt.Errorf("open archived app bundle: %w", err)
	}
	defer func() { _ = appRoot.Close() }()

	candidatePaths := []string{"Info.plist"}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(relativeApplicationPath)), ".app") {
		candidatePaths = append(candidatePaths, filepath.Join("Contents", "Info.plist"))
	}

	var (
		data    []byte
		lastErr error
	)
	for _, candidatePath := range candidatePaths {
		data, lastErr = readRegularFileFromRoot(appRoot, candidatePath)
		if lastErr == nil {
			break
		}
		if !errors.Is(lastErr, os.ErrNotExist) {
			return nil, fmt.Errorf("read archived app Info.plist: %w", lastErr)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("read archived app Info.plist: %w", lastErr)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode archived app Info.plist: %w", err)
	}
	return payload, nil
}

func readRegularFileFromRoot(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("metadata path must be a regular file: %s", name)
	}
	return io.ReadAll(file)
}

func inferAppStorePlatformFromPlist(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if platform := mapXcodePlatformToAppStorePlatform(coercePlistValueToString(payload["DTPlatformName"])); platform != "" {
		return platform
	}
	if platform := mapXcodePlatformToAppStorePlatform(firstPlistString(payload["CFBundleSupportedPlatforms"])); platform != "" {
		return platform
	}
	return ""
}

func firstPlistString(value any) string {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if text := coercePlistValueToString(item); text != "" {
				return text
			}
		}
	case []string:
		for _, item := range v {
			if text := strings.TrimSpace(item); text != "" {
				return text
			}
		}
	}
	return ""
}

func mapXcodePlatformToAppStorePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "iphoneos", "iphonesimulator":
		return "IOS"
	case "watchos", "watchsimulator":
		return "IOS"
	case "appletvos", "appletvsimulator":
		return "TV_OS"
	case "xros", "xrsimulator":
		return "VISION_OS"
	case "macosx":
		return "MAC_OS"
	default:
		return ""
	}
}

func coercePlistValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case int, int8, int16, int32, int64:
		return fmt.Sprint(v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v)
	case float32, float64:
		return strings.TrimSpace(fmt.Sprint(v))
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}
