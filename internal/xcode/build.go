package xcode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

var userCacheDirFn = os.UserCacheDir

// BuildOptions describes an ordinary local xcodebuild compile.
type BuildOptions struct {
	WorkspacePath    string
	ProjectPath      string
	Scheme           string
	Configuration    string
	Destination      string
	DerivedDataPath  string
	ResultBundlePath string
	Clean            bool
	NoCodeSigning    bool
	XcodebuildArgs   []string
	LogWriter        io.Writer
}

// BuildResult is the stable structured result for an ordinary Xcode build.
type BuildResult struct {
	WorkspacePath     string `json:"workspace,omitempty"`
	ProjectPath       string `json:"project,omitempty"`
	Scheme            string `json:"scheme"`
	Configuration     string `json:"configuration,omitempty"`
	Destination       string `json:"destination,omitempty"`
	DerivedDataPath   string `json:"derived_data_path"`
	ResultBundlePath  string `json:"result_bundle_path,omitempty"`
	BuildProductsPath string `json:"build_products_path,omitempty"`
	Clean             bool   `json:"clean"`
	NoCodeSigning     bool   `json:"no_code_signing"`
	Success           bool   `json:"success"`
	DurationMS        int64  `json:"duration_ms"`
	ExitStatus        *int   `json:"exit_status,omitempty"`
}

// ValidateBuildOptions checks deterministic command-shape errors without
// reading the filesystem or starting a subprocess.
func ValidateBuildOptions(opts BuildOptions) error {
	opts = normalizeBuildOptions(opts)
	if err := validateWorkspaceProjectPair(opts.WorkspacePath, opts.ProjectPath); err != nil {
		return err
	}
	if opts.Scheme == "" {
		return fmt.Errorf("--scheme is required")
	}
	if opts.ProjectPath != "" && !strings.EqualFold(filepath.Ext(opts.ProjectPath), ".xcodeproj") {
		return fmt.Errorf("--project must end with .xcodeproj")
	}
	if opts.WorkspacePath != "" && !strings.EqualFold(filepath.Ext(opts.WorkspacePath), ".xcworkspace") {
		return fmt.Errorf("--workspace must end with .xcworkspace")
	}
	for _, arg := range opts.XcodebuildArgs {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("--xcodebuild-flag cannot be empty")
		}
	}
	if reserved := reservedBuildPassthroughArgument(opts.XcodebuildArgs); reserved != "" {
		return fmt.Errorf("--xcodebuild-flag cannot override asc-managed argument %q", reserved)
	}
	return nil
}

// Build runs an ordinary local xcodebuild compile. If xcodebuild starts and
// fails, the returned result records failure details while the error preserves
// the subprocess or context error chain.
func Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	startedAt := time.Now()
	opts = normalizeBuildOptions(opts)
	result := &BuildResult{
		WorkspacePath: opts.WorkspacePath,
		ProjectPath:   opts.ProjectPath,
		Scheme:        opts.Scheme,
		Configuration: opts.Configuration,
		Destination:   opts.Destination,
		Clean:         opts.Clean,
		NoCodeSigning: opts.NoCodeSigning,
	}
	finish := func(err error) (*BuildResult, error) {
		result.DurationMS = max(int64(0), time.Since(startedAt).Milliseconds())
		result.Success = err == nil
		return result, err
	}

	if err := ValidateBuildOptions(opts); err != nil {
		return finish(err)
	}
	derivedDataPath, err := resolveBuildDerivedDataPath(opts)
	if err != nil {
		return finish(err)
	}
	opts.DerivedDataPath = derivedDataPath
	result.DerivedDataPath = derivedDataPath
	resultBundlePath, err := resolveBuildResultBundlePath(opts.ResultBundlePath)
	if err != nil {
		return finish(err)
	}
	opts.ResultBundlePath = resultBundlePath
	result.ResultBundlePath = resultBundlePath
	if err := validateBuildResultBundleDestination(resultBundlePath); err != nil {
		return finish(err)
	}

	if err := ensureXcodeAvailable(ctx); err != nil {
		return finish(err)
	}
	if err := validateBuildInputPaths(opts); err != nil {
		return finish(err)
	}
	productsPath := filepath.Join(derivedDataPath, "Build", "Products")
	productsPathExisted := existingDirectory(productsPath)
	if buildErr := runXcodebuildForBuild(ctx, buildBuildCommand(opts), opts.LogWriter); buildErr != nil {
		var exitErr *exec.ExitError
		if errors.As(buildErr, &exitErr) {
			exitStatus := exitErr.ExitCode()
			if exitStatus >= 0 {
				result.ExitStatus = &exitStatus
			}
		}
		return finish(buildErr)
	}
	exitStatus := 0
	result.ExitStatus = &exitStatus

	if !productsPathExisted && existingDirectory(productsPath) {
		result.BuildProductsPath = productsPath
	}
	return finish(nil)
}

func normalizeBuildOptions(opts BuildOptions) BuildOptions {
	opts.WorkspacePath = normalizeDirectoryPath(opts.WorkspacePath)
	opts.ProjectPath = normalizeDirectoryPath(opts.ProjectPath)
	opts.Scheme = strings.TrimSpace(opts.Scheme)
	opts.Configuration = strings.TrimSpace(opts.Configuration)
	opts.Destination = strings.TrimSpace(opts.Destination)
	opts.DerivedDataPath = normalizeDirectoryPath(opts.DerivedDataPath)
	opts.ResultBundlePath = normalizeDirectoryPath(opts.ResultBundlePath)
	return opts
}

func validateBuildInputPaths(opts BuildOptions) error {
	if opts.WorkspacePath != "" {
		return validateExistingPath(opts.WorkspacePath, ".xcworkspace", "--workspace")
	}
	return validateExistingPath(opts.ProjectPath, ".xcodeproj", "--project")
}

func reservedBuildPassthroughArgument(args []string) string {
	managedFlags := []string{
		"-dry-run",
		"-project",
		"-workspace",
		"-scheme",
		"-target",
		"-alltargets",
		"-configuration",
		"-destination",
		"-deriveddatapath",
		"-resultbundlepath",
		"-archivepath",
		"-exportarchive",
		"-usage",
		"-help",
		"-license",
		"-checkfirstlaunchstatus",
		"-runfirstlaunch",
		"-preparedevicesupport",
		"-downloadplatform",
		"-downloadallplatforms",
		"-importplatform",
		"-downloadcomponent",
		"-importcomponent",
		"-deletecomponent",
		"-showcomponent",
		"-showsdks",
		"-showdestinations",
		"-showtestplans",
		"-showbuildsettings",
		"-showbuildsettingsforindex",
		"-list",
		"-find-executable",
		"-find-library",
		"-version",
		"-exportnotarizedapp",
		"-exportlocalizations",
		"-importlocalizations",
		"-resolvepackagedependencies",
		"-create-xcframework",
	}
	xcodebuildActions := map[string]struct{}{
		"build":                 {},
		"build-for-testing":     {},
		"analyze":               {},
		"archive":               {},
		"test":                  {},
		"test-without-building": {},
		"docbuild":              {},
		"installsrc":            {},
		"installhdrs":           {},
		"install":               {},
		"clean":                 {},
	}
	managedBuildSettings := []string{
		"action",
		"code_signing_allowed",
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		trimmed := strings.TrimSpace(arg)
		normalized := strings.ToLower(trimmed)
		if xcodebuildPassthroughArgumentTakesValue(normalized) {
			index++
			continue
		}
		for _, managed := range managedFlags {
			if normalized == managed || strings.HasPrefix(normalized, managed+"=") {
				return strings.SplitN(trimmed, "=", 2)[0]
			}
		}
		if _, isAction := xcodebuildActions[normalized]; isAction {
			return trimmed
		}
		for _, managed := range managedBuildSettings {
			if strings.HasPrefix(normalized, managed+"=") || strings.HasPrefix(normalized, managed+"[") {
				return trimmed[:len(managed)]
			}
		}
	}
	return ""
}

func xcodebuildPassthroughArgumentTakesValue(normalized string) bool {
	if strings.Contains(normalized, "=") {
		return false
	}
	switch normalized {
	case "-authenticationkeypath", "-authenticationkeyid", "-authenticationkeyissuerid":
		return true
	default:
		return false
	}
}

func resolveBuildDerivedDataPath(opts BuildOptions) (string, error) {
	if opts.DerivedDataPath != "" {
		absolutePath, err := filepath.Abs(opts.DerivedDataPath)
		if err != nil {
			return "", fmt.Errorf("resolve derived data path: %w", err)
		}
		return filepath.Clean(absolutePath), nil
	}
	selector := opts.ProjectPath
	if selector == "" {
		selector = opts.WorkspacePath
	}
	absoluteSelector, err := filepath.Abs(selector)
	if err != nil {
		return "", fmt.Errorf("resolve Xcode project/workspace path: %w", err)
	}
	cacheDir, err := userCacheDirFn()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory for derived data: %w", err)
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("resolve user cache directory for derived data: empty path")
	}

	digest := sha256.Sum256([]byte(strings.Join([]string{
		absoluteSelector,
		opts.Scheme,
		opts.Configuration,
		opts.Destination,
	}, "\x00")))
	hash := hex.EncodeToString(digest[:])[:12]
	return filepath.Join(cacheDir, "asc", "xcode-build", safeBuildPathComponent(opts.Scheme)+"-"+hash), nil
}

func resolveBuildResultBundlePath(pathValue string) (string, error) {
	if pathValue == "" {
		return "", nil
	}
	absolutePath, err := filepath.Abs(pathValue)
	if err != nil {
		return "", fmt.Errorf("resolve result bundle path: %w", err)
	}
	return absolutePath, nil
}

func validateBuildResultBundleDestination(pathValue string) error {
	if pathValue == "" {
		return nil
	}
	if _, err := os.Lstat(pathValue); err == nil {
		return fmt.Errorf("--result-bundle-path already exists: %s", pathValue)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect --result-bundle-path: %w", err)
	}
	return nil
}

func safeBuildPathComponent(value string) string {
	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if builder.Len() > 0 && !lastWasSeparator {
			builder.WriteByte('-')
			lastWasSeparator = true
		}
	}
	component := strings.Trim(builder.String(), "-")
	if component == "" {
		component = "build"
	}
	if runes := []rune(component); len(runes) > 48 {
		component = strings.TrimRight(string(runes[:48]), "-")
	}
	return component
}

func existingDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func buildBuildCommand(opts BuildOptions) []string {
	args := make([]string, 0, 20+len(opts.XcodebuildArgs))
	if opts.WorkspacePath != "" {
		args = append(args, "-workspace", opts.WorkspacePath)
	}
	if opts.ProjectPath != "" {
		args = append(args, "-project", opts.ProjectPath)
	}
	args = append(args, "-scheme", opts.Scheme)
	if opts.Configuration != "" {
		args = append(args, "-configuration", opts.Configuration)
	}
	if opts.Destination != "" {
		args = append(args, "-destination", opts.Destination)
	}
	args = append(args, "-derivedDataPath", opts.DerivedDataPath)
	if opts.ResultBundlePath != "" {
		args = append(args, "-resultBundlePath", opts.ResultBundlePath)
	}
	args = append(args, cloneStrings(opts.XcodebuildArgs)...)
	if opts.NoCodeSigning {
		args = append(args, "CODE_SIGNING_ALLOWED=NO")
	}
	if opts.Clean {
		args = append(args, "clean")
	}
	return append(args, "build")
}
