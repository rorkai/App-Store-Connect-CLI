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
	runtimeGOOS      = runtime.GOOS
	lookPathFn       = exec.LookPath
	commandContextFn = exec.CommandContext
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

type bundleInfo struct {
	BundleID    string
	Version     string
	BuildNumber string
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
	if err := prepareIPAPath(opts.IPAPath, opts.Overwrite); err != nil {
		return nil, err
	}

	tempExportDir, err := os.MkdirTemp(filepath.Dir(opts.IPAPath), ".asc-xcode-export-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary export directory: %w", err)
	}
	defer os.RemoveAll(tempExportDir)

	args := buildExportCommand(opts, tempExportDir)
	if err := runXcodebuild(ctx, args, opts.LogWriter); err != nil {
		return nil, err
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
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, "xcodebuild", args...)
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
					"xcodebuild %s failed (showing last %d bytes): %s",
					summarizeAction(args),
					xcodebuildErrorTailLimit,
					detail,
				)
			}
			return fmt.Errorf("xcodebuild %s failed: %s", summarizeAction(args), detail)
		}
		return fmt.Errorf("xcodebuild %s failed: %w", summarizeAction(args), err)
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
	return bundleInfo{
		BundleID:    coercePlistValueToString(appProps["CFBundleIdentifier"]),
		Version:     coercePlistValueToString(appProps["CFBundleShortVersionString"]),
		BuildNumber: coercePlistValueToString(appProps["CFBundleVersion"]),
	}, nil
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
	}, nil
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
