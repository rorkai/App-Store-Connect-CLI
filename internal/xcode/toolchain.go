package xcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	toolchainProbeDiagnosticLimit = 4 * 1024
	trustedXcrunPath              = "/usr/bin/xcrun"
)

// ToolchainStatus is the overall status of a local Xcode toolchain report.
type ToolchainStatus string

const (
	ToolchainStatusOK   ToolchainStatus = "ok"
	ToolchainStatusWarn ToolchainStatus = "warn"
	ToolchainStatusFail ToolchainStatus = "fail"
)

// ToolchainCheckStatus is the status of one local Xcode toolchain check.
type ToolchainCheckStatus string

const (
	ToolchainCheckStatusOK   ToolchainCheckStatus = "ok"
	ToolchainCheckStatusWarn ToolchainCheckStatus = "warn"
	ToolchainCheckStatusFail ToolchainCheckStatus = "fail"
)

// ToolchainSource describes how the developer directory was selected.
type ToolchainSource string

const (
	ToolchainSourceFlag        ToolchainSource = "flag"
	ToolchainSourceEnvironment ToolchainSource = "environment"
	ToolchainSourceXcodeSelect ToolchainSource = "xcode-select"
)

// XcodeVersion contains the version and build values reported by xcodebuild.
type XcodeVersion struct {
	Version string
	Build   string
}

// ToolchainOptions controls local Xcode toolchain inspection.
type ToolchainOptions struct {
	DeveloperDir string
	SDK          string
	LogWriter    io.Writer
}

// ToolchainCheck is one check in a local Xcode toolchain report. It is a
// probe-layer type, not a serialization contract; command output flows
// through internal/asc's XcodeToolchainDoctorCheck.
type ToolchainCheck struct {
	Name    string
	Status  ToolchainCheckStatus
	Path    string
	Message string
}

// ToolchainReport is the structured result for local toolchain checks. It is
// a probe-layer type, not a serialization contract; command output flows
// through internal/asc's registered camelCase XcodeToolchainDoctorResult.
type ToolchainReport struct {
	Status       ToolchainStatus
	Source       ToolchainSource
	DeveloperDir string
	XcodePath    string
	XcodeVersion string
	XcodeBuild   string
	// Beta is nil until developer-directory selection and normalization have
	// completed, so a failed selection cannot be reported as stable Xcode.
	Beta   *bool
	Checks []ToolchainCheck
}

var (
	xcodeVersionLinePattern = regexp.MustCompile(`(?m)^\s*Xcode[\t ]+(.+?)\s*$`)
	xcodeBuildLinePattern   = regexp.MustCompile(`(?m)^\s*Build[\t ]+version[\t ]+(.+?)\s*$`)
)

// InspectToolchain resolves and verifies a local Xcode developer directory.
// It returns a report alongside probe failures whenever enough information is
// available to produce one.
func InspectToolchain(ctx context.Context, opts ToolchainOptions) (*ToolchainReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if runtimeGOOS != "darwin" {
		return nil, fmt.Errorf("supported on macOS only; current platform is %s", runtimeGOOS)
	}

	report := &ToolchainReport{
		Status: ToolchainStatusOK,
		Checks: make([]ToolchainCheck, 0, 4),
	}
	var firstErr error
	addCheck := func(check ToolchainCheck, err error) {
		report.Checks = append(report.Checks, check)
		if check.Status == ToolchainCheckStatusFail {
			report.Status = ToolchainStatusFail
			if firstErr == nil {
				firstErr = err
			}
		} else if check.Status == ToolchainCheckStatusWarn && report.Status == ToolchainStatusOK {
			report.Status = ToolchainStatusWarn
		}
	}

	developerDir, source, err := resolveToolchainDeveloperDir(ctx, opts.DeveloperDir)
	report.Source = source
	if err != nil {
		addCheck(
			ToolchainCheck{
				Name:    "developer_dir",
				Status:  ToolchainCheckStatusFail,
				Message: sanitizeToolchainError(err),
			},
			fmt.Errorf("resolve developer directory: %w", err),
		)
		return report, firstErr
	}

	developerDir, xcodePath, commandLineTools, err := normalizeToolchainDeveloperDir(developerDir)
	report.DeveloperDir = developerDir
	report.XcodePath = xcodePath
	if err != nil {
		addCheck(
			ToolchainCheck{
				Name:    "developer_dir",
				Status:  ToolchainCheckStatusFail,
				Message: sanitizeToolchainError(err),
			},
			fmt.Errorf("inspect developer directory: %w", err),
		)
		return report, firstErr
	}

	if commandLineTools {
		addCheck(
			ToolchainCheck{
				Name:    "developer_dir",
				Status:  ToolchainCheckStatusFail,
				Message: "selected developer directory is the Command Line Tools package; full Xcode is required for xcode doctor",
			},
			errors.New("full Xcode is required; selected developer directory is the Command Line Tools package"),
		)
	} else {
		addCheck(
			ToolchainCheck{
				Name:    "developer_dir",
				Status:  ToolchainCheckStatusOK,
				Message: "developer directory is available",
			},
			nil,
		)
	}

	beta, betaErr := classifyBetaXcodePath(developerDir, xcodePath)
	if betaErr != nil {
		addCheck(
			ToolchainCheck{
				Name:    "beta",
				Status:  ToolchainCheckStatusFail,
				Message: "beta status is unavailable because the selected toolchain could not be canonicalized",
			},
			fmt.Errorf("classify beta Xcode: %w", betaErr),
		)
	} else {
		report.Beta = beta
	}

	xcrunPath, xcrunLookupErr := resolveToolchainXcrunPath()
	var resolvedXcodebuildPath string
	var resolvedXcodebuildErr error
	var xcrunCheck ToolchainCheck
	var xcrunCheckErr error
	if xcrunLookupErr != nil {
		message := "trusted xcrun is not available"
		if !errors.Is(xcrunLookupErr, exec.ErrNotFound) {
			message = sanitizeToolchainError(fmt.Errorf("locate xcrun: %w", xcrunLookupErr))
		}
		xcrunCheck = ToolchainCheck{Name: "xcrun", Status: ToolchainCheckStatusFail, Message: message}
		xcrunCheckErr = fmt.Errorf("locate xcrun: %w", xcrunLookupErr)
		resolvedXcodebuildErr = fmt.Errorf("xcrun is unavailable: %w", xcrunLookupErr)
	} else {
		stdout, stderr, probeErr := runToolchainProbe(ctx, xcrunPath, []string{"--find", "xcodebuild"}, developerDir, opts.LogWriter)
		resolvedXcodebuild := strings.TrimSpace(stdout)
		switch {
		case probeErr != nil:
			message := toolchainProbeFailureMessage("xcrun --find xcodebuild", stderr, probeErr)
			xcrunCheck = ToolchainCheck{Name: "xcrun", Status: ToolchainCheckStatusFail, Path: xcrunPath, Message: message}
			xcrunCheckErr = fmt.Errorf("xcrun --find xcodebuild: %w", probeErr)
			resolvedXcodebuildErr = xcrunCheckErr
		case resolvedXcodebuild == "":
			message := "xcrun did not resolve xcodebuild"
			xcrunCheck = ToolchainCheck{Name: "xcrun", Status: ToolchainCheckStatusFail, Path: xcrunPath, Message: message}
			xcrunCheckErr = errors.New(message)
			resolvedXcodebuildErr = xcrunCheckErr
		default:
			validatedPath, validateErr := validateResolvedXcodebuildPath(resolvedXcodebuild, developerDir)
			if validateErr != nil {
				message := sanitizeToolchainError(validateErr)
				xcrunCheck = ToolchainCheck{Name: "xcrun", Status: ToolchainCheckStatusFail, Path: xcrunPath, Message: message}
				xcrunCheckErr = fmt.Errorf("validate xcrun xcodebuild path: %w", validateErr)
				resolvedXcodebuildErr = xcrunCheckErr
			} else {
				resolvedXcodebuildPath = validatedPath
				xcrunCheck = ToolchainCheck{Name: "xcrun", Status: ToolchainCheckStatusOK, Path: xcrunPath, Message: "xcrun resolved xcodebuild from the selected toolchain"}
			}
		}
	}

	if resolvedXcodebuildErr != nil {
		message := "xcodebuild cannot be verified with the selected toolchain: " + sanitizeToolchainError(resolvedXcodebuildErr)
		addCheck(
			ToolchainCheck{Name: "xcodebuild", Status: ToolchainCheckStatusFail, Message: truncateUTF8Prefix(message, 512)},
			fmt.Errorf("resolve selected xcodebuild: %w", resolvedXcodebuildErr),
		)
	} else {
		stdout, stderr, probeErr := runToolchainProbe(ctx, resolvedXcodebuildPath, []string{"-version"}, developerDir, opts.LogWriter)
		if probeErr != nil {
			message := toolchainProbeFailureMessage("xcodebuild -version", stderr, probeErr)
			addCheck(
				ToolchainCheck{Name: "xcodebuild", Status: ToolchainCheckStatusFail, Path: resolvedXcodebuildPath, Message: message},
				fmt.Errorf("xcodebuild -version: %w", probeErr),
			)
		} else {
			version, parseErr := parseXcodeVersion(stdout)
			if parseErr != nil {
				addCheck(
					ToolchainCheck{Name: "xcodebuild", Status: ToolchainCheckStatusFail, Path: resolvedXcodebuildPath, Message: sanitizeToolchainError(parseErr)},
					fmt.Errorf("parse xcodebuild -version: %w", parseErr),
				)
			} else {
				report.XcodeVersion = version.Version
				report.XcodeBuild = version.Build
				addCheck(
					ToolchainCheck{Name: "xcodebuild", Status: ToolchainCheckStatusOK, Path: resolvedXcodebuildPath, Message: "xcodebuild -version succeeded"},
					nil,
				)
			}
		}
	}
	addCheck(xcrunCheck, xcrunCheckErr)

	if sdk := strings.TrimSpace(opts.SDK); sdk != "" {
		checkName := "sdk:" + sdk
		if xcrunLookupErr != nil {
			message := fmt.Sprintf("xcrun is unavailable; cannot resolve SDK %q", sdk)
			addCheck(
				ToolchainCheck{Name: checkName, Status: ToolchainCheckStatusFail, Message: message},
				fmt.Errorf("resolve SDK %q: xcrun unavailable: %w", sdk, xcrunLookupErr),
			)
		} else {
			stdout, stderr, probeErr := runToolchainProbe(ctx, xcrunPath, []string{"--sdk", sdk, "--show-sdk-path"}, developerDir, opts.LogWriter)
			sdkPath := strings.TrimSpace(stdout)
			switch {
			case probeErr != nil:
				message := toolchainProbeFailureMessage("xcrun --sdk "+sdk+" --show-sdk-path", stderr, probeErr)
				addCheck(
					ToolchainCheck{Name: checkName, Status: ToolchainCheckStatusFail, Message: message},
					fmt.Errorf("resolve SDK %q: %w", sdk, probeErr),
				)
			case sdkPath == "":
				message := fmt.Sprintf("SDK %q did not resolve to a path", sdk)
				addCheck(
					ToolchainCheck{Name: checkName, Status: ToolchainCheckStatusFail, Message: message},
					errors.New(message),
				)
			default:
				addCheck(
					ToolchainCheck{Name: checkName, Status: ToolchainCheckStatusOK, Path: sdkPath, Message: "SDK is available"},
					nil,
				)
			}
		}
	}

	if report.Beta != nil && *report.Beta {
		betaCheck := ToolchainCheck{
			Name:    "beta",
			Status:  ToolchainCheckStatusWarn,
			Message: "selected developer directory appears to be a beta Xcode build",
		}
		addCheck(betaCheck, nil)
	}

	if firstErr != nil {
		return report, firstErr
	}
	return report, nil
}

func resolveToolchainDeveloperDir(ctx context.Context, explicit string) (string, ToolchainSource, error) {
	if value := strings.TrimSpace(explicit); value != "" {
		return value, ToolchainSourceFlag, nil
	}
	if value := strings.TrimSpace(os.Getenv("DEVELOPER_DIR")); value != "" {
		return value, ToolchainSourceEnvironment, nil
	}
	value, err := activeDeveloperDirFn(ctx)
	if err != nil {
		return "", ToolchainSourceXcodeSelect, err
	}
	if strings.TrimSpace(value) == "" {
		return "", ToolchainSourceXcodeSelect, errors.New("xcode-select returned an empty developer directory")
	}
	return value, ToolchainSourceXcodeSelect, nil
}

func normalizeToolchainDeveloperDir(value string) (developerDir, xcodePath string, commandLineTools bool, err error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", false, errors.New("developer directory is empty")
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve developer directory path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Stat(absolute)
	if err != nil {
		return "", "", false, fmt.Errorf("developer directory %q is unavailable: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", "", false, fmt.Errorf("developer directory %q is not a directory", absolute)
	}

	canonicalAbsolute, err := canonicalToolchainPath(absolute)
	if err != nil {
		return "", "", false, err
	}
	canonicalLower := strings.ToLower(filepath.ToSlash(canonicalAbsolute))
	commandLineTools = strings.HasSuffix(canonicalLower, "/library/developer/commandlinetools") || strings.HasSuffix(canonicalLower, "/commandlinetools")
	if commandLineTools {
		return absolute, "", true, nil
	}

	if strings.EqualFold(filepath.Ext(canonicalAbsolute), ".app") {
		candidate := filepath.Join(absolute, "Contents", "Developer")
		candidateInfo, candidateErr := os.Stat(candidate)
		if candidateErr != nil {
			return "", "", false, fmt.Errorf("xcode application %q has no Contents/Developer directory: %w", absolute, candidateErr)
		}
		if !candidateInfo.IsDir() {
			return "", "", false, fmt.Errorf("xcode application developer path %q is not a directory", candidate)
		}
		return filepath.Clean(candidate), absolute, false, nil
	}

	parent := filepath.Dir(absolute)
	canonicalParent := filepath.Dir(canonicalAbsolute)
	if strings.EqualFold(filepath.Base(canonicalParent), "Contents") && strings.EqualFold(filepath.Ext(filepath.Dir(canonicalParent)), ".app") && strings.EqualFold(filepath.Base(parent), "Contents") && strings.EqualFold(filepath.Ext(filepath.Dir(parent)), ".app") {
		xcodePath = filepath.Dir(parent)
	}
	return absolute, xcodePath, commandLineTools, nil
}

func canonicalToolchainPath(pathValue string) (string, error) {
	canonicalPath, err := filepath.EvalSymlinks(pathValue)
	if err != nil {
		return "", fmt.Errorf("resolve selected toolchain path symlinks: %w", err)
	}
	return filepath.Clean(canonicalPath), nil
}

func classifyBetaXcodePath(developerDir, xcodePath string) (*bool, error) {
	paths := []string{developerDir}
	if strings.TrimSpace(xcodePath) != "" {
		paths = append(paths, xcodePath)
	}

	beta := false
	for _, pathValue := range paths {
		canonicalPath, err := canonicalToolchainPath(pathValue)
		if err != nil {
			return nil, err
		}
		beta = beta || isBetaXcodePath(canonicalPath)
	}
	return &beta, nil
}

func resolveToolchainXcrunPath() (string, error) {
	return trustedXcrunPathFn()
}

func validateResolvedXcodebuildPath(pathValue, developerDir string) (string, error) {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return "", errors.New("xcrun returned an empty xcodebuild path")
	}
	if !filepath.IsAbs(pathValue) {
		return "", fmt.Errorf("xcrun returned a non-absolute xcodebuild path %q", pathValue)
	}
	pathValue = filepath.Clean(pathValue)
	if !strings.EqualFold(filepath.Base(pathValue), "xcodebuild") {
		return "", fmt.Errorf("xcrun returned an unexpected executable path %q", pathValue)
	}

	canonicalDeveloperDir, err := filepath.EvalSymlinks(developerDir)
	if err != nil {
		return "", fmt.Errorf("resolve selected developer directory symlinks: %w", err)
	}
	canonicalDeveloperDir = filepath.Clean(canonicalDeveloperDir)
	canonicalPath, err := filepath.EvalSymlinks(pathValue)
	if err != nil {
		// Preserve the more useful containment diagnostic for a path outside
		// the selected directory even when that path no longer exists.
		if !pathWithinDirectoryFold(canonicalDeveloperDir, pathValue) {
			return "", fmt.Errorf("xcrun resolved xcodebuild outside selected developer directory %q", canonicalDeveloperDir)
		}
		return "", fmt.Errorf("resolved xcodebuild path %q is unavailable: %w", pathValue, err)
	}
	canonicalPath = filepath.Clean(canonicalPath)

	contained, err := pathIdentityWithinDirectory(canonicalDeveloperDir, canonicalPath)
	if err != nil {
		return "", fmt.Errorf("compare xcodebuild path with developer directory: %w", err)
	}
	if !contained {
		return "", fmt.Errorf("xcrun resolved xcodebuild outside selected developer directory %q", canonicalDeveloperDir)
	}

	info, err := statPathFn(canonicalPath)
	if err != nil {
		return "", fmt.Errorf("resolved xcodebuild path %q is unavailable: %w", canonicalPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("resolved xcodebuild path %q is not a regular file", canonicalPath)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("resolved xcodebuild path %q is not executable", canonicalPath)
	}
	// Preserve xcrun's resolved spelling in the report and command invocation;
	// the canonical path above is used for containment and file validation.
	return pathValue, nil
}

// pathWithinDirectoryFold reports whether pathValue's leading components equal
// directory's components under Unicode case folding. It is a lexical
// best-effort used only to pick a diagnostic for paths that no longer exist,
// where filesystem identity cannot be consulted; existing paths must use
// pathIdentityWithinDirectory instead.
func pathWithinDirectoryFold(directory, pathValue string) bool {
	directoryComponents := strings.Split(filepath.ToSlash(filepath.Clean(directory)), "/")
	pathComponents := strings.Split(filepath.ToSlash(filepath.Clean(pathValue)), "/")
	if len(pathComponents) < len(directoryComponents) {
		return false
	}
	for index, component := range directoryComponents {
		if !strings.EqualFold(component, pathComponents[index]) {
			return false
		}
	}
	return true
}

// pathIdentityWithinDirectory reports whether pathValue sits inside directory
// by comparing filesystem identity (os.SameFile) of pathValue's ancestors with
// directory. Unlike a lexical prefix comparison, this accepts alternate
// spellings of the same physical directory on case-insensitive volumes while
// never accepting a genuinely distinct directory on case-sensitive ones. Both
// arguments must already be symlink-resolved absolute paths.
func pathIdentityWithinDirectory(directory, pathValue string) (bool, error) {
	directoryInfo, err := statPathFn(directory)
	if err != nil {
		return false, fmt.Errorf("inspect selected developer directory %q: %w", directory, err)
	}
	current := filepath.Clean(pathValue)
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
		currentInfo, err := statPathFn(current)
		if err != nil {
			return false, fmt.Errorf("inspect resolved xcodebuild ancestor %q: %w", current, err)
		}
		if os.SameFile(directoryInfo, currentInfo) {
			return true, nil
		}
	}
}

func parseXcodeVersion(output string) (XcodeVersion, error) {
	versionMatch := xcodeVersionLinePattern.FindStringSubmatch(output)
	if len(versionMatch) != 2 || strings.TrimSpace(versionMatch[1]) == "" {
		detail := strings.TrimSpace(output)
		if detail == "" {
			detail = "empty output"
		} else {
			detail = truncateUTF8Prefix(detail, 256)
		}
		return XcodeVersion{}, fmt.Errorf("unexpected xcodebuild -version output: %q", detail)
	}
	buildMatch := xcodeBuildLinePattern.FindStringSubmatch(output)
	if len(buildMatch) != 2 || strings.TrimSpace(buildMatch[1]) == "" {
		return XcodeVersion{}, errors.New("missing Build version in xcodebuild -version output")
	}
	return XcodeVersion{
		Version: strings.TrimSpace(versionMatch[1]),
		Build:   strings.TrimSpace(buildMatch[1]),
	}, nil
}

func runToolchainProbe(ctx context.Context, name string, args []string, developerDir string, logWriter io.Writer) (stdout, stderr string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, name, args...)
	cmd.Env = toolchainProbeEnvironment(developerDir)
	stdoutBuffer := newTailBuffer(toolchainProbeDiagnosticLimit)
	stderrBuffer := newTailBuffer(toolchainProbeDiagnosticLimit)
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderrBuffer
	err = runXcodeCommand(cmd)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
	}
	stdout = stdoutBuffer.String()
	stderr = stderrBuffer.String()
	writeToolchainProbeLog(logWriter, stdout)
	writeToolchainProbeLog(logWriter, stderr)
	return stdout, stderr, err
}

func writeToolchainProbeLog(logWriter io.Writer, output string) {
	if logWriter == nil || output == "" {
		return
	}
	_, _ = io.WriteString(logWriter, output)
	if !strings.HasSuffix(output, "\n") {
		_, _ = io.WriteString(logWriter, "\n")
	}
}

func toolchainProbeEnvironment(developerDir string) []string {
	original := os.Environ()
	environment := make([]string, 0, len(original)+1)
	for _, entry := range original {
		if strings.HasPrefix(entry, "DEVELOPER_DIR=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "DEVELOPER_DIR="+developerDir)
}

func toolchainProbeFailureMessage(command string, stderr string, err error) string {
	detail := strings.TrimSpace(stderr)
	if detail != "" {
		return fmt.Sprintf("%s failed: %s", command, truncateUTF8Prefix(detail, 256))
	}
	return fmt.Sprintf("%s failed: %s", command, sanitizeToolchainError(err))
}

func sanitizeToolchainError(err error) string {
	if err == nil {
		return ""
	}
	return truncateUTF8Prefix(strings.TrimSpace(err.Error()), 512)
}
