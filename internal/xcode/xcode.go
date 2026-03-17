package xcode

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"howett.net/plist"
)

var (
	runtimeGOOS          = runtime.GOOS
	lookPathFn           = exec.LookPath
	commandContextFn     = exec.CommandContext
	activeDeveloperDirFn = activeDeveloperDir
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

type bundleInfo struct {
	BundleID    string
	Version     string
	BuildNumber string
	Platform    string
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
	if err := moveExportedIPA(exportedIPAPath, opts.IPAPath, opts.Overwrite); err != nil {
		return nil, err
	}

	info, err := readIPABundleInfo(opts.IPAPath)
	if err != nil {
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
	if err := runAltoolValidate(ctx, buildValidateCommand(opts), opts.LogWriter); err != nil {
		return nil, err
	}
	return &ValidateResult{
		IPAPath:   opts.IPAPath,
		Validated: true,
	}, nil
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
	return nil
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
	output, err := cmd.Output()
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

func buildValidateCommand(opts ValidateOptions) []string {
	args := []string{
		"altool",
		"--validate-app",
		"--file", opts.IPAPath,
		"--type", "ios",
	}
	if opts.APIKey != "" {
		args = append(args, "--apiKey", opts.APIKey)
	}
	if opts.APIIssuer != "" {
		args = append(args, "--apiIssuer", opts.APIIssuer)
	}
	return args
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
	return runCommandWithTail(ctx, "xcodebuild", args, logWriter, summarizeAction(args), "xcodebuild")
}

func runAltoolValidate(ctx context.Context, args []string, logWriter io.Writer) error {
	return runCommandWithTail(ctx, "xcrun", args, logWriter, "validate", "xcrun altool")
}

func runCommandWithTail(ctx context.Context, name string, args []string, logWriter io.Writer, action string, commandLabel string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, name, args...)
	outputTail := newTailBuffer(xcodebuildErrorTailLimit)
	writer := io.Writer(outputTail)
	if logWriter != nil {
		writer = io.MultiWriter(logWriter, outputTail)
	}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(outputTail.String())
		if detail != "" {
			if outputTail.Truncated() {
				return fmt.Errorf(
					"%s %s failed (showing last %d bytes): %s",
					commandLabel,
					action,
					xcodebuildErrorTailLimit,
					detail,
				)
			}
			return fmt.Errorf("%s %s failed: %s", commandLabel, action, detail)
		}
		return fmt.Errorf("%s %s failed: %w", commandLabel, action, err)
	}
	return nil
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
	return string(b.data)
}

func (b *tailBuffer) Truncated() bool {
	return b.truncated
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
		if removeErr := os.Remove(ipaPath); removeErr != nil {
			return fmt.Errorf("remove existing ipa path: %w", removeErr)
		}
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
	if overwrite {
		if err := os.Remove(destinationPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove existing ipa path: %w", err)
		}
	}
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
	reader, err := file.Open()
	if err != nil {
		return bundleInfo{}, fmt.Errorf("open Info.plist: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
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
	applicationPath := coercePlistValueToString(appProps["ApplicationPath"])
	if strings.TrimSpace(applicationPath) == "" {
		return "", fmt.Errorf("archive Info.plist missing ApplicationPath")
	}

	appBundlePath := filepath.Join(archivePath, "Products", filepath.FromSlash(applicationPath))
	candidatePaths := []string{filepath.Join(appBundlePath, "Info.plist")}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(appBundlePath)), ".app") {
		candidatePaths = append(candidatePaths, filepath.Join(appBundlePath, "Contents", "Info.plist"))
	}

	var (
		data    []byte
		lastErr error
	)
	for _, candidatePath := range candidatePaths {
		data, lastErr = os.ReadFile(candidatePath)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("read archived app Info.plist: %w", lastErr)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode archived app Info.plist: %w", err)
	}
	platform := inferAppStorePlatformFromPlist(payload)
	if platform == "" {
		return "", fmt.Errorf("archived app Info.plist did not contain a supported platform marker")
	}
	return platform, nil
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
