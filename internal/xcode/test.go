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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TestAction identifies the xcodebuild operation selected by TestOptions.
type TestAction string

const (
	TestActionTest                  TestAction = "test"
	TestActionBuildForTesting       TestAction = "build-for-testing"
	TestActionTestWithoutBuilding   TestAction = "test-without-building"
	maxTestFailureMessage                      = 4096
	maxTestFailureCount                        = 100
	maxTestCaseCount                           = 10000
	maxXcresulttoolOutputBytes                 = 16 << 20
	maxXcresulttoolDiagnosticBytes             = 8 << 10
	testResultPostProcessingTimeout            = 30 * time.Second
)

// TestOptions describes a local xcodebuild test operation.
type TestOptions struct {
	WorkspacePath    string
	ProjectPath      string
	Scheme           string
	Action           string
	Configuration    string
	Destinations     []string
	TestPlan         string
	XctestrunPath    string
	OnlyTesting      []string
	SkipTesting      []string
	DerivedDataPath  string
	ResultBundlePath string
	Clean            bool
	NoCodeSigning    bool
	XcodebuildArgs   []string
	LogWriter        io.Writer
}

// TestResult is the stable structured result for a local Xcode test operation.
type TestResult struct {
	WorkspacePath    string       `json:"workspace,omitempty"`
	ProjectPath      string       `json:"project,omitempty"`
	Scheme           string       `json:"scheme,omitempty"`
	Action           string       `json:"action"`
	Configuration    string       `json:"configuration,omitempty"`
	Destinations     []string     `json:"destinations,omitempty"`
	TestPlan         string       `json:"test_plan,omitempty"`
	XctestrunPath    string       `json:"xctestrun_path,omitempty"`
	DerivedDataPath  string       `json:"derived_data_path,omitempty"`
	ResultBundlePath string       `json:"result_bundle_path,omitempty"`
	Tests            *TestSummary `json:"tests,omitempty"`
	Clean            bool         `json:"clean"`
	NoCodeSigning    bool         `json:"no_code_signing"`
	Success          bool         `json:"success"`
	DurationMS       int64        `json:"duration_ms"`
	ExitStatus       *int         `json:"exit_status,omitempty"`
}

// TestSummary contains the structured counts extracted from an xcresult.
type TestSummary struct {
	Total            int           `json:"total"`
	Passed           int           `json:"passed"`
	Failed           int           `json:"failed"`
	Skipped          int           `json:"skipped"`
	ExpectedFailures int           `json:"expected_failures,omitempty"`
	DurationMS       int64         `json:"duration_ms"`
	Cases            []TestCase    `json:"cases,omitempty"`
	Failures         []TestFailure `json:"failures,omitempty"`

	// expectedFailuresSet distinguishes a parsed aggregate (where zero is an
	// authoritative value) from a hand-built summary used by callers/tests.
	expectedFailuresSet bool
}

// TestCase is a bounded structured representation of one test case.
type TestCase struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	Classname  string `json:"classname,omitempty"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Message    string `json:"message,omitempty"`
}

// TestFailure describes a failed test without including its complete log.
type TestFailure struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message,omitempty"`
}

var (
	runXcodeTestCommand     = runXcodebuildForBuild
	readTestResultSummaryFn = readTestResultSummary
	testNowFn               = time.Now
)

// ValidateTestOptions checks deterministic command-shape errors without
// reading the filesystem or starting a subprocess.
func ValidateTestOptions(opts TestOptions) error {
	opts = normalizeTestOptions(opts)
	if opts.Action == "" {
		opts.Action = string(TestActionTest)
	}
	switch TestAction(opts.Action) {
	case TestActionTest, TestActionBuildForTesting:
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
		if opts.XctestrunPath != "" {
			return fmt.Errorf("--xctestrun is only valid with --action test-without-building")
		}
	case TestActionTestWithoutBuilding:
		if opts.ProjectPath != "" || opts.WorkspacePath != "" {
			return fmt.Errorf("--project and --workspace cannot be used with --action test-without-building")
		}
		if opts.Scheme != "" {
			return fmt.Errorf("--scheme cannot be used with --action test-without-building")
		}
		if opts.XctestrunPath == "" {
			return fmt.Errorf("--xctestrun is required")
		}
		if opts.Configuration != "" {
			return fmt.Errorf("--configuration cannot be used with --action test-without-building")
		}
		if opts.DerivedDataPath != "" {
			return fmt.Errorf("--derived-data-path cannot be used with --action test-without-building")
		}
		if !strings.EqualFold(filepath.Ext(opts.XctestrunPath), ".xctestrun") {
			return fmt.Errorf("--xctestrun must end with .xctestrun")
		}
	default:
		return fmt.Errorf("--action must be one of: test, build-for-testing, test-without-building")
	}
	if len(opts.Destinations) == 0 {
		return fmt.Errorf("--destination is required")
	}
	for _, destination := range opts.Destinations {
		if err := validateTestValue(destination, "--destination"); err != nil {
			return err
		}
	}
	if opts.TestPlan != "" && opts.XctestrunPath != "" {
		return fmt.Errorf("--test-plan cannot be used with --xctestrun")
	}
	if opts.Clean && TestAction(opts.Action) == TestActionTestWithoutBuilding {
		return fmt.Errorf("--clean cannot be used with --action test-without-building")
	}
	if opts.NoCodeSigning && TestAction(opts.Action) == TestActionTestWithoutBuilding {
		return fmt.Errorf("--no-code-signing cannot be used with --action test-without-building")
	}
	for _, value := range opts.OnlyTesting {
		if err := validateTestValue(value, "--only-testing"); err != nil {
			return err
		}
	}
	for _, value := range opts.SkipTesting {
		if err := validateTestValue(value, "--skip-testing"); err != nil {
			return err
		}
	}
	for _, arg := range opts.XcodebuildArgs {
		if err := validateTestValue(arg, "--xcodebuild-flag"); err != nil {
			return err
		}
	}
	if err := validateTestPassthroughArguments(opts.XcodebuildArgs); err != nil {
		return err
	}
	if reserved := reservedTestPassthroughArgument(opts.XcodebuildArgs); reserved != "" {
		return fmt.Errorf("--xcodebuild-flag cannot override asc-managed argument %q", reserved)
	}
	if opts.ResultBundlePath != "" && !strings.EqualFold(filepath.Ext(opts.ResultBundlePath), ".xcresult") {
		return fmt.Errorf("--result-bundle-path must end with .xcresult")
	}
	return nil
}

// Test runs a local xcodebuild test operation. For test-executing actions it
// reads the structured result bundle after xcodebuild exits. A subprocess
// failure is preserved even when its partial result bundle can be summarized.
func Test(ctx context.Context, opts TestOptions) (*TestResult, error) {
	startedAt := testNowFn()
	opts = normalizeTestOptions(opts)
	if opts.Action == "" {
		opts.Action = string(TestActionTest)
	}
	result := &TestResult{
		WorkspacePath: opts.WorkspacePath,
		ProjectPath:   opts.ProjectPath,
		Scheme:        opts.Scheme,
		Action:        opts.Action,
		Configuration: opts.Configuration,
		Destinations:  cloneTestValues(opts.Destinations),
		TestPlan:      opts.TestPlan,
		XctestrunPath: opts.XctestrunPath,
		Clean:         opts.Clean,
		NoCodeSigning: opts.NoCodeSigning,
	}
	finish := func(err error) (*TestResult, error) {
		result.DurationMS = max(int64(0), testNowFn().Sub(startedAt).Milliseconds())
		result.Success = err == nil
		return result, err
	}

	if err := ValidateTestOptions(opts); err != nil {
		return finish(err)
	}
	if opts.Action != string(TestActionTestWithoutBuilding) {
		derivedDataPath, err := resolveTestDerivedDataPath(opts)
		if err != nil {
			return finish(err)
		}
		opts.DerivedDataPath = derivedDataPath
		result.DerivedDataPath = derivedDataPath
	}
	if opts.Action != string(TestActionBuildForTesting) {
		resultBundlePath, err := resolveTestResultBundlePath(opts)
		if err != nil {
			return finish(err)
		}
		opts.ResultBundlePath = resultBundlePath
		result.ResultBundlePath = resultBundlePath
	} else if opts.ResultBundlePath != "" {
		resultBundlePath, err := resolveTestResultBundlePath(opts)
		if err != nil {
			return finish(err)
		}
		opts.ResultBundlePath = resultBundlePath
		result.ResultBundlePath = resultBundlePath
	}
	if err := validateTestResultBundleDestination(opts.ResultBundlePath); err != nil {
		return finish(err)
	}
	if err := validateTestInputPaths(opts); err != nil {
		return finish(err)
	}
	if err := ensureXcodeAvailable(ctx); err != nil {
		return finish(err)
	}
	if opts.ResultBundlePath != "" {
		if err := os.MkdirAll(filepath.Dir(opts.ResultBundlePath), 0o755); err != nil {
			return finish(fmt.Errorf("create result bundle parent directory: %w", err))
		}
		if err := validateTestResultBundlePathComponents(opts.ResultBundlePath); err != nil {
			return finish(err)
		}
	}

	command := buildTestCommand(opts)
	processErr := runXcodeTestCommand(ctx, command, opts.LogWriter)
	if processErr != nil {
		setTestExitStatus(result, processErr)
		if opts.Action != string(TestActionBuildForTesting) && validateTestResultBundlePathComponents(opts.ResultBundlePath) == nil && existingDirectory(opts.ResultBundlePath) {
			if summary, _ := readPartialTestResultSummary(ctx, opts.ResultBundlePath); summary != nil {
				result.Tests = summary
			}
		}
		return finish(processErr)
	}

	if opts.Action == string(TestActionBuildForTesting) {
		result.XctestrunPath = findXctestrunPath(opts.DerivedDataPath)
		exitStatus := 0
		result.ExitStatus = &exitStatus
		return finish(nil)
	}
	if err := validateTestResultBundlePathComponents(opts.ResultBundlePath); err != nil {
		return finish(err)
	}

	summary, err := readTestResultSummaryFn(ctx, opts.ResultBundlePath)
	if summary != nil {
		result.Tests = summary
	}
	if err != nil {
		return finish(fmt.Errorf("read test result summary: %w", err))
	}
	if err := validateTestSummary(summary); err != nil {
		return finish(fmt.Errorf("validate test result: %w", err))
	}
	if summary.Failed > 0 || hasFailedTestCase(summary.Cases) {
		reportedFailed := summary.Failed
		if reportedFailed == 0 {
			_, reportedFailed, _ = countTestCases(summary.Cases)
		}
		return finish(&ReportedTestFailuresError{Failed: reportedFailed})
	}
	exitStatus := 0
	result.ExitStatus = &exitStatus
	return finish(nil)
}

// ReportedTestFailuresError reports that the test action completed and its
// result bundle was parsed successfully, but the parsed summary contained
// failing test cases. The failures are already represented in the returned
// TestResult, so callers that build their own failure rows can recognize this
// cause and avoid counting the same outcome twice. Genuine post-processing and
// infrastructure errors keep their own types.
type ReportedTestFailuresError struct {
	Failed int
}

func (e *ReportedTestFailuresError) Error() string {
	return fmt.Sprintf("xcode test result reported %d failed tests", e.Failed)
}

// readPartialTestResultSummary gives post-processing a short, independent
// deadline after xcodebuild fails. The command context may already be canceled
// when xcodebuild is terminated, but a readable result bundle can still contain
// valuable partial test data. Preserve context values while deliberately
// dropping the caller's cancellation and deadline for this bounded read.
func readPartialTestResultSummary(ctx context.Context, resultBundlePath string) (*TestSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	postProcessContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), testResultPostProcessingTimeout)
	defer cancel()
	return readTestResultSummaryFn(postProcessContext, resultBundlePath)
}

func normalizeTestOptions(opts TestOptions) TestOptions {
	opts.WorkspacePath = normalizeDirectoryPath(opts.WorkspacePath)
	opts.ProjectPath = normalizeDirectoryPath(opts.ProjectPath)
	opts.Scheme = strings.TrimSpace(opts.Scheme)
	opts.Action = strings.TrimSpace(opts.Action)
	opts.Configuration = strings.TrimSpace(opts.Configuration)
	opts.Destinations = cloneTestValues(opts.Destinations)
	opts.TestPlan = strings.TrimSpace(opts.TestPlan)
	opts.XctestrunPath = normalizeDirectoryPath(opts.XctestrunPath)
	opts.OnlyTesting = cloneTestValues(opts.OnlyTesting)
	opts.SkipTesting = cloneTestValues(opts.SkipTesting)
	opts.DerivedDataPath = normalizeDirectoryPath(opts.DerivedDataPath)
	opts.ResultBundlePath = normalizeDirectoryPath(opts.ResultBundlePath)
	opts.XcodebuildArgs = cloneTestValues(opts.XcodebuildArgs)
	return opts
}

func cloneTestValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func validateTestValue(value, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", flagName)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s cannot contain control characters", flagName)
		}
	}
	return nil
}

func validateTestInputPaths(opts TestOptions) error {
	if opts.Action == string(TestActionTestWithoutBuilding) {
		return validateExistingFile(opts.XctestrunPath, "--xctestrun")
	}
	if opts.WorkspacePath != "" {
		return validateExistingPath(opts.WorkspacePath, ".xcworkspace", "--workspace")
	}
	return validateExistingPath(opts.ProjectPath, ".xcodeproj", "--project")
}

func resolveTestDerivedDataPath(opts TestOptions) (string, error) {
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
		return "", fmt.Errorf("resolve user cache directory for test derived data: %w", err)
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("resolve user cache directory for test derived data: empty path")
	}
	digest := sha256.Sum256([]byte(strings.Join(append([]string{
		absoluteSelector,
		opts.Scheme,
		opts.Action,
		opts.Configuration,
		opts.TestPlan,
	}, opts.Destinations...), "\x00")))
	hash := hex.EncodeToString(digest[:])[:12]
	return filepath.Join(cacheDir, "asc", "xcode-test", safeBuildPathComponent(opts.Scheme)+"-"+hash), nil
}

func resolveTestResultBundlePath(opts TestOptions) (string, error) {
	if opts.ResultBundlePath != "" {
		absolutePath, err := filepath.Abs(opts.ResultBundlePath)
		if err != nil {
			return "", fmt.Errorf("resolve result bundle path: %w", err)
		}
		return filepath.Clean(absolutePath), nil
	}
	cacheDir, err := userCacheDirFn()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory for test result bundle: %w", err)
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("resolve user cache directory for test result bundle: empty path")
	}
	now := testNowFn().UTC()
	selector := opts.ProjectPath
	if selector == "" {
		selector = opts.WorkspacePath
	}
	if selector == "" {
		selector = opts.XctestrunPath
	}
	digest := sha256.Sum256([]byte(strings.Join(append([]string{
		selector,
		opts.Scheme,
		opts.Action,
		opts.Configuration,
		opts.TestPlan,
	}, opts.Destinations...), "\x00")))
	hash := hex.EncodeToString(digest[:])[:12]
	stamp := strconv.FormatInt(now.UnixNano(), 10)
	return filepath.Join(cacheDir, "asc", "xcode-test", safeBuildPathComponent(opts.Scheme)+"-"+stamp+"-"+hash+".xcresult"), nil
}

func validateTestResultBundleDestination(pathValue string) error {
	if pathValue == "" {
		return nil
	}
	if _, err := os.Lstat(pathValue); err == nil {
		return fmt.Errorf("--result-bundle-path already exists: %s", pathValue)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect --result-bundle-path: %w", err)
	}
	if err := validateTestResultBundlePathComponents(pathValue); err != nil {
		return err
	}
	return nil
}

// validateTestResultBundlePathComponents refuses symlinks in every existing
// component of a result path. The destination is intentionally checked both
// before and after xcodebuild because the final path is created by a child
// process outside this package's control.
func validateTestResultBundlePathComponents(pathValue string) error {
	if pathValue == "" {
		return nil
	}
	cleanPath := filepath.Clean(pathValue)
	volume := filepath.VolumeName(cleanPath)
	remainder := strings.TrimPrefix(cleanPath, volume)
	current := volume
	if strings.HasPrefix(remainder, string(filepath.Separator)) {
		current += string(filepath.Separator)
		remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	}
	if current == "" {
		current = "."
	}
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		if current == string(filepath.Separator) || strings.HasSuffix(current, string(filepath.Separator)) {
			current += component
		} else {
			current = filepath.Join(current, component)
		}
		info, err := os.Lstat(current)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil
		case err != nil:
			return fmt.Errorf("inspect --result-bundle-path component %q: %w", current, err)
		case info.Mode()&os.ModeSymlink != 0:
			if isTrustedDarwinPathAlias(current) {
				continue
			}
			return fmt.Errorf("--result-bundle-path cannot use symlink component: %s", current)
		}
	}
	return nil
}

func isTrustedDarwinPathAlias(pathValue string) bool {
	if runtimeGOOS != "darwin" {
		return false
	}
	target, err := os.Readlink(pathValue)
	if err != nil {
		return false
	}
	switch filepath.Clean(pathValue) {
	case "/etc":
		return target == "private/etc"
	case "/tmp":
		return target == "private/tmp"
	case "/var":
		return target == "private/var"
	default:
		return false
	}
}

func buildTestCommand(opts TestOptions) []string {
	args := make([]string, 0, 24+len(opts.Destinations)*2+len(opts.OnlyTesting)+len(opts.SkipTesting)+len(opts.XcodebuildArgs))
	if opts.WorkspacePath != "" {
		args = append(args, "-workspace", opts.WorkspacePath)
	}
	if opts.ProjectPath != "" {
		args = append(args, "-project", opts.ProjectPath)
	}
	if opts.Scheme != "" {
		args = append(args, "-scheme", opts.Scheme)
	}
	if opts.Configuration != "" && opts.Action != string(TestActionTestWithoutBuilding) {
		args = append(args, "-configuration", opts.Configuration)
	}
	for _, destination := range opts.Destinations {
		args = append(args, "-destination", destination)
	}
	if opts.TestPlan != "" {
		args = append(args, "-testPlan", opts.TestPlan)
	}
	if opts.XctestrunPath != "" {
		args = append(args, "-xctestrun", opts.XctestrunPath)
	}
	if opts.DerivedDataPath != "" && opts.Action != string(TestActionTestWithoutBuilding) {
		args = append(args, "-derivedDataPath", opts.DerivedDataPath)
	}
	if opts.ResultBundlePath != "" {
		args = append(args, "-resultBundlePath", opts.ResultBundlePath)
	}
	for _, identifier := range opts.OnlyTesting {
		args = append(args, "-only-testing:"+identifier)
	}
	for _, identifier := range opts.SkipTesting {
		args = append(args, "-skip-testing:"+identifier)
	}
	args = append(args, cloneTestValues(opts.XcodebuildArgs)...)
	if opts.NoCodeSigning {
		args = append(args, "CODE_SIGNING_ALLOWED=NO")
	}
	if opts.Clean {
		args = append(args, "clean")
	}
	return append(args, opts.Action)
}

func reservedTestPassthroughArgument(args []string) string {
	if reserved := reservedBuildPassthroughArgument(args); reserved != "" {
		return reserved
	}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		normalized := strings.ToLower(trimmed)
		for _, managed := range []string{"-testplan", "-xctestrun", "-only-testing", "-skip-testing"} {
			if normalized == managed || strings.HasPrefix(normalized, managed+"=") || strings.HasPrefix(normalized, managed+":") {
				return strings.SplitN(strings.SplitN(trimmed, "=", 2)[0], ":", 2)[0]
			}
		}
	}
	return ""
}

func validateTestPassthroughArguments(args []string) error {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if isXcodebuildAuthenticationArgument(normalized) && strings.Contains(normalized, "=") {
			equalsIndex := strings.Index(arg, "=")
			if equalsIndex >= 0 && strings.TrimSpace(arg[equalsIndex+1:]) == "" {
				return fmt.Errorf("--xcodebuild-flag %q cannot have an empty value", strings.TrimSpace(arg))
			}
		}
		if !xcodebuildPassthroughArgumentTakesValue(normalized) {
			continue
		}
		if index+1 >= len(args) {
			return fmt.Errorf("--xcodebuild-flag %q requires a following value", strings.TrimSpace(arg))
		}
		value := args[index+1]
		if isRecognizedTestPassthroughArgument(value) {
			return fmt.Errorf("--xcodebuild-flag %q requires a value; %q is a recognized xcodebuild option or asc-managed argument", strings.TrimSpace(arg), strings.TrimSpace(value))
		}
		// Authentication values are paths and identifiers, never options. Any
		// leading dash means the caller omitted the value and xcodebuild would
		// silently consume the next option as the credential instead.
		if strings.HasPrefix(strings.TrimSpace(value), "-") {
			return fmt.Errorf("--xcodebuild-flag %q requires a value; %q is an xcodebuild option", strings.TrimSpace(arg), strings.TrimSpace(value))
		}
		index++
	}
	return nil
}

func isRecognizedTestPassthroughArgument(arg string) bool {
	normalized := strings.ToLower(strings.TrimSpace(arg))
	if isXcodebuildAuthenticationArgument(normalized) {
		return true
	}
	return reservedTestPassthroughArgument([]string{arg}) != ""
}

func setTestExitStatus(result *TestResult, err error) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitStatus := exitErr.ExitCode()
		if exitStatus >= 0 {
			result.ExitStatus = &exitStatus
		}
	}
}

func findXctestrunPath(derivedDataPath string) string {
	if strings.TrimSpace(derivedDataPath) == "" {
		return ""
	}
	productsPath := filepath.Join(derivedDataPath, "Build", "Products")
	entries, err := os.ReadDir(productsPath)
	if err != nil {
		return ""
	}
	candidates := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".xctestrun") {
			continue
		}
		candidates = append(candidates, filepath.Join(productsPath, entry.Name()))
	}
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

func readTestResultSummary(ctx context.Context, resultBundlePath string) (*TestSummary, error) {
	if _, err := trustedXcodeToolPathFn(ctx, "xcrun", nil); err != nil {
		if isTrustedXcodeToolUnavailable(err, "xcrun") {
			return nil, fmt.Errorf("xcrun not available; install Xcode and ensure the active developer directory is configured")
		}
		return nil, fmt.Errorf("locate xcrun: %w", err)
	}
	summaryOutput, err := runXcresulttoolJSON(ctx, "summary", resultBundlePath)
	if err != nil {
		return nil, fmt.Errorf("run xcresulttool test-results summary: %w", err)
	}
	summary, err := ParseTestResultSummary(summaryOutput)
	if err != nil {
		return nil, err
	}

	// The summary operation contains aggregate counts and failure metadata, but
	// current Xcode versions expose individual test cases through the separate
	// `tests` operation. Keep that second read in the same post-processing step
	// so JSON and JUnit can report the structured test tree rather than parsing
	// human-readable xcodebuild output.
	testsOutput, err := runXcresulttoolJSON(ctx, "tests", resultBundlePath)
	if err != nil {
		return summary, fmt.Errorf("run xcresulttool test-results tests: %w", err)
	}
	cases, err := ParseTestResultCases(testsOutput)
	if err != nil {
		return summary, err
	}
	summary.Cases = cases
	if err := validateTestSummary(summary); err != nil {
		return summary, err
	}
	for _, testCase := range cases {
		if len(summary.Failures) >= maxTestFailureCount {
			break
		}
		if normalizeTestStatus(testCase.Status) == "failed" && !containsTestFailure(summary.Failures, testCase.Identifier) {
			summary.Failures = append(summary.Failures, TestFailure{
				Identifier: testCase.Identifier,
				Message:    boundTestMessage(testCase.Message),
			})
		}
	}
	return summary, nil
}

func runXcresulttoolJSON(ctx context.Context, operation, resultBundlePath string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd, err := trustedXcodeCommand(ctx, "xcrun", []string{"xcresulttool", "get", "test-results", operation, "--path", resultBundlePath, "--compact"}, nil)
	if err != nil {
		return nil, err
	}
	var output boundedXcresulttoolOutput
	output.limit = maxXcresulttoolOutputBytes
	var diagnostics boundedXcresulttoolOutput
	diagnostics.limit = maxXcresulttoolDiagnosticBytes
	cmd.Stdout = &output
	cmd.Stderr = &diagnostics
	err = runXcodeCommand(cmd)
	if output.exceeded {
		if err != nil {
			err = fmt.Errorf("xcresulttool %s output exceeds %d bytes: %w", operation, maxXcresulttoolOutputBytes, err)
		} else {
			err = fmt.Errorf("xcresulttool %s output exceeds %d bytes", operation, maxXcresulttoolOutputBytes)
		}
	}
	if err != nil {
		if diagnostic := strings.TrimSpace(string(diagnostics.Bytes())); diagnostic != "" {
			if diagnostics.exceeded {
				diagnostic += " [truncated]"
			}
			err = fmt.Errorf("%w: xcresulttool %s diagnostics: %s", err, operation, diagnostic)
		}
		return nil, err
	}
	return append([]byte(nil), output.Bytes()...), nil
}

type boundedXcresulttoolOutput struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (output *boundedXcresulttoolOutput) Bytes() []byte {
	return output.buffer.Bytes()
}

func (output *boundedXcresulttoolOutput) Len() int {
	return output.buffer.Len()
}

func (output *boundedXcresulttoolOutput) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	remaining := output.limit - output.Len()
	if remaining <= 0 {
		output.exceeded = true
		return len(data), nil
	}
	if len(data) > remaining {
		_, _ = output.buffer.Write(data[:remaining])
		output.exceeded = true
		return len(data), nil
	}
	_, err := output.buffer.Write(data)
	return len(data), err
}

type rawTestResultSummary struct {
	Tests            json.RawMessage `json:"tests"`
	TestCases        json.RawMessage `json:"testCases"`
	TotalTestCount   json.RawMessage `json:"totalTestCount"`
	PassedTests      json.RawMessage `json:"passedTests"`
	FailedTests      json.RawMessage `json:"failedTests"`
	SkippedTests     json.RawMessage `json:"skippedTests"`
	ExpectedFailures json.RawMessage `json:"expectedFailures"`
	TestDuration     json.RawMessage `json:"testDuration"`
	DurationMS       json.RawMessage `json:"durationMs"`
	Duration         json.RawMessage `json:"duration"`
	StartTime        json.RawMessage `json:"startTime"`
	FinishTime       json.RawMessage `json:"finishTime"`
	TestFailures     json.RawMessage `json:"testFailures"`
}

type rawTestFailure struct {
	Identifier       json.RawMessage `json:"testIdentifier"`
	IdentifierAlt    string          `json:"identifier"`
	IdentifierString string          `json:"testIdentifierString"`
	IdentifierURL    string          `json:"testIdentifierURL"`
	TestName         string          `json:"testName"`
	TargetName       string          `json:"targetName"`
	Message          string          `json:"message"`
	FailureMessage   string          `json:"failureMessage"`
	FailureText      string          `json:"failureText"`
}

type rawTestCase struct {
	Identifier     string          `json:"testIdentifier"`
	IdentifierAlt  string          `json:"identifier"`
	Name           string          `json:"name"`
	TestName       string          `json:"testName"`
	Classname      string          `json:"classname"`
	Status         string          `json:"status"`
	TestStatus     string          `json:"testStatus"`
	Result         string          `json:"result"`
	Duration       json.RawMessage `json:"duration"`
	DurationInSecs json.RawMessage `json:"durationInSeconds"`
	DurationMS     json.RawMessage `json:"durationMs"`
	Message        string          `json:"message"`
	FailureMessage string          `json:"failureMessage"`
}

type rawTestResults struct {
	TestNodes []rawTestNode `json:"testNodes"`
}

type rawTestNode struct {
	Identifier        string          `json:"nodeIdentifier"`
	IdentifierURL     string          `json:"nodeIdentifierURL"`
	IdentifierAlt     string          `json:"identifier"`
	NodeType          string          `json:"nodeType"`
	Name              string          `json:"name"`
	TestName          string          `json:"testName"`
	Status            string          `json:"status"`
	TestStatus        string          `json:"testStatus"`
	Result            string          `json:"result"`
	Duration          json.RawMessage `json:"duration"`
	DurationInSeconds json.RawMessage `json:"durationInSeconds"`
	DurationMS        json.RawMessage `json:"durationMs"`
	Message           string          `json:"message"`
	FailureMessage    string          `json:"failureMessage"`
	Children          []rawTestNode   `json:"children"`
}

// ParseTestResultSummary parses the stable subset emitted by xcresulttool.
// It accepts both count-oriented summaries and summaries that include cases,
// because Xcode versions differ in how much detail they expose at this level.
func ParseTestResultSummary(data []byte) (*TestSummary, error) {
	var raw rawTestResultSummary
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode xcresulttool summary: %w", err)
	}
	cases, err := parseRawTestCasesPayload(raw.Tests)
	if err != nil {
		return nil, fmt.Errorf("decode test cases: %w", err)
	}
	if len(cases) == 0 {
		cases, err = parseRawTestCasesPayload(raw.TestCases)
		if err != nil {
			return nil, fmt.Errorf("decode test cases: %w", err)
		}
	}
	total, totalSet, err := decodeIntValue(raw.TotalTestCount)
	if err != nil {
		return nil, fmt.Errorf("decode test count: %w", err)
	}
	if !totalSet {
		total, totalSet, err = decodeIntValue(raw.Tests)
		if err != nil {
			return nil, fmt.Errorf("decode test count: %w", err)
		}
	}
	passed, passedSet, err := decodeIntValue(raw.PassedTests)
	if err != nil {
		return nil, fmt.Errorf("decode passed test count: %w", err)
	}
	failed, failedSet, err := decodeIntValue(raw.FailedTests)
	if err != nil {
		return nil, fmt.Errorf("decode failed test count: %w", err)
	}
	skipped, skippedSet, err := decodeIntValue(raw.SkippedTests)
	if err != nil {
		return nil, fmt.Errorf("decode skipped test count: %w", err)
	}
	expectedFailures, expectedFailuresSet, err := decodeIntValue(raw.ExpectedFailures)
	if err != nil {
		return nil, fmt.Errorf("decode expected-failure count: %w", err)
	}
	if !totalSet && len(cases) > 0 {
		total = len(cases)
		totalSet = true
	}
	casePassed, caseFailed, caseSkipped := countTestCases(cases)
	if !passedSet {
		passed = casePassed
	}
	if !failedSet {
		failed = caseFailed
	}
	if !skippedSet {
		skipped = caseSkipped
	}
	if !totalSet {
		if !passedSet && !failedSet && !skippedSet && !expectedFailuresSet {
			return nil, fmt.Errorf("xcresulttool summary did not include a test count")
		}
		total = passed + failed + skipped + expectedFailures
		totalSet = true
	}
	if len(cases) == 0 {
		missing := make([]string, 0, 4)
		if !totalSet {
			missing = append(missing, "totalTestCount")
		}
		if !passedSet {
			missing = append(missing, "passedTests")
		}
		if !failedSet {
			missing = append(missing, "failedTests")
		}
		if !skippedSet {
			missing = append(missing, "skippedTests")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("xcresulttool summary is missing required test counts: %s", strings.Join(missing, ", "))
		}
	}
	if !testCountsMatch(total, passed, failed, skipped) {
		return nil, fmt.Errorf("xcresulttool summary contains inconsistent test counts")
	}
	expectedRemainder, _ := testCountRemainder(total, passed, failed, skipped)
	if expectedFailuresSet && expectedFailures != expectedRemainder {
		return nil, fmt.Errorf("xcresulttool summary contains inconsistent expected-failure count")
	}
	if !expectedFailuresSet {
		expectedFailures = expectedRemainder
	}
	durationMS, err := decodeMilliseconds(raw.DurationMS)
	if err != nil {
		return nil, fmt.Errorf("decode test duration: %w", err)
	}
	if !hasJSONValue(raw.DurationMS) {
		durationMS, err = decodeDurationMS(raw.TestDuration)
	}
	if err != nil {
		return nil, fmt.Errorf("decode test duration: %w", err)
	}
	if !hasJSONValue(raw.DurationMS) && !hasJSONValue(raw.TestDuration) {
		durationMS, err = decodeDurationMS(raw.Duration)
		if err != nil {
			return nil, fmt.Errorf("decode test duration: %w", err)
		}
	}
	if !hasJSONValue(raw.DurationMS) && !hasJSONValue(raw.TestDuration) && !hasJSONValue(raw.Duration) {
		start, startSet, startErr := decodeFloatValue(raw.StartTime)
		finish, finishSet, finishErr := decodeFloatValue(raw.FinishTime)
		if startErr != nil || finishErr != nil {
			return nil, fmt.Errorf("decode test start/finish time: %w", errors.Join(startErr, finishErr))
		}
		if startSet && finishSet && finish >= start {
			durationMS = max(int64(0), int64((finish-start)*1000))
		}
	}
	failures, err := parseRawTestFailures(raw.TestFailures)
	if err != nil {
		return nil, fmt.Errorf("decode test failures: %w", err)
	}
	for _, testCase := range cases {
		if normalizeTestStatus(testCase.Status) == "failed" && len(failures) < maxTestFailureCount {
			if !containsTestFailure(failures, testCase.Identifier) {
				failures = append(failures, TestFailure{Identifier: testCase.Identifier, Message: boundTestMessage(testCase.Message)})
			}
		}
	}
	summary := &TestSummary{
		Total:               total,
		Passed:              passed,
		Failed:              failed,
		Skipped:             skipped,
		ExpectedFailures:    expectedFailures,
		expectedFailuresSet: true,
		DurationMS:          durationMS,
		Cases:               cases,
		Failures:            failures,
	}
	if err := validateTestSummary(summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// ParseTestResultCases parses the recursive test tree returned by
// `xcresulttool get test-results tests`. Only Test Case nodes are exposed;
// suites, plans, devices, and failure-message children are structural.
func ParseTestResultCases(data []byte) ([]TestCase, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, fmt.Errorf("xcresulttool tests output was empty")
	}
	if strings.HasPrefix(trimmed, "[") {
		var direct []rawTestCase
		if err := json.Unmarshal(data, &direct); err != nil {
			return nil, fmt.Errorf("decode xcresulttool test cases: %w", err)
		}
		cases, err := parseRawTestCases(direct)
		if err != nil {
			return nil, err
		}
		if err := validateTestCases(cases); err != nil {
			return nil, err
		}
		return cases, nil
	}

	var payload rawTestResults
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode xcresulttool tests: %w", err)
	}
	if payload.TestNodes == nil {
		return nil, fmt.Errorf("xcresulttool tests output did not include testNodes")
	}
	cases := make([]TestCase, 0)
	for _, node := range payload.TestNodes {
		if err := appendTestCases(&cases, node); err != nil {
			return nil, err
		}
	}
	if err := validateTestCases(cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func appendTestCases(cases *[]TestCase, node rawTestNode) error {
	if strings.EqualFold(strings.TrimSpace(node.NodeType), "test case") {
		if len(*cases) >= maxTestCaseCount {
			return fmt.Errorf("xcresulttool tests output exceeds %d test cases", maxTestCaseCount)
		}
		testCase, err := parseRawTestNode(node)
		if err != nil {
			return err
		}
		*cases = append(*cases, testCase)
		return nil
	}
	for _, child := range node.Children {
		if err := appendTestCases(cases, child); err != nil {
			return err
		}
	}
	return nil
}

func parseRawTestCases(rawCases []rawTestCase) ([]TestCase, error) {
	if len(rawCases) > maxTestCaseCount {
		return nil, fmt.Errorf("xcresulttool tests output exceeds %d test cases", maxTestCaseCount)
	}
	cases := make([]TestCase, 0, len(rawCases))
	for _, rawCase := range rawCases {
		durationMS, err := decodeMilliseconds(rawCase.DurationMS)
		if err != nil {
			return nil, err
		}
		if !hasJSONValue(rawCase.DurationMS) {
			durationMS, err = decodeDurationMS(rawCase.Duration)
			if err != nil {
				return nil, err
			}
		}
		if !hasJSONValue(rawCase.DurationMS) && !hasJSONValue(rawCase.Duration) {
			durationMS, err = decodeDurationMS(rawCase.DurationInSecs)
			if err != nil {
				return nil, err
			}
		}
		identifier := strings.TrimSpace(rawCase.Identifier)
		if identifier == "" {
			identifier = strings.TrimSpace(rawCase.IdentifierAlt)
		}
		name := strings.TrimSpace(rawCase.Name)
		if name == "" {
			name = strings.TrimSpace(rawCase.TestName)
		}
		status := normalizeTestStatus(rawCase.TestStatus)
		if status == "" {
			status = normalizeTestStatus(rawCase.Status)
		}
		if status == "" {
			status = normalizeTestStatus(rawCase.Result)
		}
		message := rawCase.Message
		if strings.TrimSpace(message) == "" {
			message = rawCase.FailureMessage
		}
		cases = append(cases, TestCase{
			Identifier: identifier,
			Name:       name,
			Classname:  strings.TrimSpace(rawCase.Classname),
			Status:     status,
			DurationMS: durationMS,
			Message:    boundTestMessage(message),
		})
	}
	return cases, nil
}

func parseRawTestCasesPayload(data json.RawMessage) ([]TestCase, error) {
	trimmed := strings.TrimSpace(string(data))
	if len(data) == 0 || trimmed == "null" {
		return nil, nil
	}
	var rawCases []rawTestCase
	if err := json.Unmarshal(data, &rawCases); err != nil {
		// A current summary uses no `tests` array, while older shapes may use an
		// object for that field. Treat an object as an absent case list so the
		// aggregate fields remain authoritative.
		if strings.HasPrefix(trimmed, "[") {
			return nil, fmt.Errorf("decode xcresulttool test cases: %w", err)
		}
		return nil, nil
	}
	return parseRawTestCases(rawCases)
}

func parseRawTestNode(node rawTestNode) (TestCase, error) {
	identifier := strings.TrimSpace(node.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(node.IdentifierAlt)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(node.IdentifierURL)
	}
	name := strings.TrimSpace(node.Name)
	if name == "" {
		name = strings.TrimSpace(node.TestName)
	}
	status := normalizeTestStatus(node.TestStatus)
	if status == "" {
		status = normalizeTestStatus(node.Status)
	}
	if status == "" {
		status = normalizeTestStatus(node.Result)
	}
	durationMS, err := decodeMilliseconds(node.DurationMS)
	if err != nil {
		return TestCase{}, fmt.Errorf("decode test case duration: %w", err)
	}
	if !hasJSONValue(node.DurationMS) {
		durationMS, err = decodeDurationMS(node.Duration)
		if err != nil {
			return TestCase{}, fmt.Errorf("decode test case duration: %w", err)
		}
	}
	if !hasJSONValue(node.DurationMS) && !hasJSONValue(node.Duration) {
		durationMS, err = decodeDurationMS(node.DurationInSeconds)
		if err != nil {
			return TestCase{}, fmt.Errorf("decode test case duration: %w", err)
		}
	}
	message := node.Message
	if strings.TrimSpace(message) == "" {
		message = node.FailureMessage
	}
	if strings.TrimSpace(message) == "" {
		message = testNodeFailureMessage(node.Children)
	}
	return TestCase{
		Identifier: identifier,
		Name:       name,
		Classname:  testClassname(identifier),
		Status:     status,
		DurationMS: durationMS,
		Message:    boundTestMessage(message),
	}, nil
}

func testNodeFailureMessage(children []rawTestNode) string {
	for _, child := range children {
		if strings.EqualFold(strings.TrimSpace(child.NodeType), "failure message") {
			message := child.Name
			if strings.TrimSpace(message) == "" {
				message = child.Message
			}
			if strings.TrimSpace(message) == "" {
				message = child.FailureMessage
			}
			if strings.TrimSpace(message) != "" {
				return boundTestMessage(message)
			}
		}
		if message := testNodeFailureMessage(child.Children); message != "" {
			return message
		}
	}
	return ""
}

func testClassname(identifier string) string {
	classname, _, ok := strings.Cut(identifier, "/")
	if !ok {
		return identifier
	}
	return classname
}

func parseRawTestFailures(data json.RawMessage) ([]TestFailure, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var rawFailures []rawTestFailure
	if err := json.Unmarshal(data, &rawFailures); err != nil {
		return nil, err
	}
	failures := make([]TestFailure, 0, min(len(rawFailures), maxTestFailureCount))
	for _, failure := range rawFailures {
		if len(failures) >= maxTestFailureCount {
			break
		}
		identifier := strings.TrimSpace(failure.IdentifierString)
		if identifier == "" {
			identifier = strings.TrimSpace(failure.IdentifierURL)
		}
		if identifier == "" {
			identifier = strings.TrimSpace(failure.IdentifierAlt)
		}
		if identifier == "" {
			identifier = decodeStringValue(failure.Identifier)
		}
		message := failure.FailureText
		if strings.TrimSpace(message) == "" {
			message = failure.Message
		}
		if strings.TrimSpace(message) == "" {
			message = failure.FailureMessage
		}
		failures = append(failures, TestFailure{Identifier: identifier, Message: boundTestMessage(message)})
	}
	return failures, nil
}

func decodeIntValue(data json.RawMessage) (int, bool, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, false, nil
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		return 0, false, nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		value, err := strconv.Atoi(string(number))
		if err != nil {
			return 0, false, err
		}
		return value, true, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, false, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func decodeStringValue(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		return strings.TrimSpace(string(number))
	}
	return ""
}

func decodeFloatValue(data json.RawMessage) (float64, bool, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, false, nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		return number, true, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, false, err
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, false, err
	}
	return number, true, nil
}

func decodeDurationMS(data json.RawMessage) (int64, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		return secondsToMilliseconds(number)
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(text)
	value, err := strconv.ParseFloat(trimmed, 64)
	if err == nil {
		return secondsToMilliseconds(value)
	}
	parsed, durationErr := time.ParseDuration(strings.ReplaceAll(trimmed, " ", ""))
	if durationErr != nil {
		return 0, durationErr
	}
	return max(int64(0), parsed.Milliseconds()), nil
}

func decodeMilliseconds(data json.RawMessage) (int64, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		return nonNegativeMilliseconds(number)
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, err
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, err
	}
	return nonNegativeMilliseconds(number)
}

func secondsToMilliseconds(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("duration is not finite")
	}
	if value <= 0 {
		return 0, nil
	}
	if value >= float64(math.MaxInt64)/1000 {
		return math.MaxInt64, nil
	}
	return nonNegativeMilliseconds(value * 1000)
}

func nonNegativeMilliseconds(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("duration is not finite")
	}
	if value <= 0 {
		return 0, nil
	}
	if value >= float64(math.MaxInt64) {
		return math.MaxInt64, nil
	}
	return int64(value), nil
}

func hasJSONValue(data json.RawMessage) bool {
	return len(data) > 0 && strings.TrimSpace(string(data)) != "null"
}

func normalizeTestStatus(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "passed", "pass", "success", "succeeded":
		return "passed"
	case "failed", "failure", "error", "errored":
		return "failed"
	case "skipped", "skip":
		return "skipped"
	}
	switch strings.NewReplacer("-", "", "_", "", " ", "").Replace(normalized) {
	case "expectedfailure", "expectedfailures":
		return "expected-failure"
	default:
		return normalized
	}
}

func countTestCases(cases []TestCase) (passed, failed, skipped int) {
	for _, testCase := range cases {
		switch normalizeTestStatus(testCase.Status) {
		case "passed":
			passed++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}
	return passed, failed, skipped
}

func countExpectedFailureCases(cases []TestCase) int {
	count := 0
	for _, testCase := range cases {
		if normalizeTestStatus(testCase.Status) == "expected-failure" {
			count++
		}
	}
	return count
}

func validateTestCases(cases []TestCase) error {
	for _, testCase := range cases {
		switch normalizeTestStatus(testCase.Status) {
		case "passed", "failed", "skipped", "expected-failure":
		default:
			return fmt.Errorf("xcresulttool test case %q has unsupported status %q", testCase.Identifier, testCase.Status)
		}
	}
	return nil
}

func validateTestSummary(summary *TestSummary) error {
	if summary == nil {
		return fmt.Errorf("xcresulttool summary was empty")
	}
	if !testCountsMatch(summary.Total, summary.Passed, summary.Failed, summary.Skipped) {
		return fmt.Errorf("xcresulttool summary contains inconsistent test counts")
	}
	if summary.expectedFailuresSet || summary.ExpectedFailures > 0 {
		expectedRemainder, _ := testCountRemainder(summary.Total, summary.Passed, summary.Failed, summary.Skipped)
		if summary.ExpectedFailures != expectedRemainder {
			return fmt.Errorf("xcresulttool summary contains inconsistent expected-failure count")
		}
	}
	if len(summary.Cases) == 0 {
		return nil
	}
	if err := validateTestCases(summary.Cases); err != nil {
		return err
	}
	// Xcode can report aggregate tests per destination or repetition while the
	// recursive `tests` operation exposes flattened leaf cases. Only compare
	// per-case status counts when both operations describe the same unit.
	if len(summary.Cases) != summary.Total {
		return nil
	}
	passed, failed, skipped := countTestCases(summary.Cases)
	expectedFailures := countExpectedFailureCases(summary.Cases)
	if passed != summary.Passed || failed != summary.Failed || skipped != summary.Skipped ||
		((summary.expectedFailuresSet || summary.ExpectedFailures > 0) && expectedFailures != summary.ExpectedFailures) {
		return fmt.Errorf("xcresulttool test case statuses do not match summary counts")
	}
	return nil
}

func testCountsMatch(total, passed, failed, skipped int) bool {
	_, ok := testCountRemainder(total, passed, failed, skipped)
	return ok
}

func testCountRemainder(total, passed, failed, skipped int) (int, bool) {
	if total < 0 || passed < 0 || failed < 0 || skipped < 0 || passed > total {
		return 0, false
	}
	remaining := total - passed
	if failed > remaining {
		return 0, false
	}
	remaining -= failed
	if skipped > remaining {
		return 0, false
	}
	return remaining - skipped, true
}

func hasFailedTestCase(cases []TestCase) bool {
	for _, testCase := range cases {
		if normalizeTestStatus(testCase.Status) == "failed" {
			return true
		}
	}
	return false
}

func boundTestMessage(value string) string {
	value = strings.TrimSpace(value)
	return truncateUTF8Prefix(value, maxTestFailureMessage)
}

func containsTestFailure(failures []TestFailure, identifier string) bool {
	for _, failure := range failures {
		if failure.Identifier == identifier && identifier != "" {
			return true
		}
	}
	return false
}
