package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func useLookPathAsTrustedResolver(t *testing.T) {
	t.Helper()
	previous := trustedXcodeToolPathFn
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		return lookPathFn(tool)
	}
	t.Cleanup(func() { trustedXcodeToolPathFn = previous })
}

func TestValidateTestOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    TestOptions
		wantErr string
	}{
		{
			name: "project test",
			opts: TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}},
		},
		{
			name: "workspace build for testing",
			opts: TestOptions{WorkspacePath: "Demo.xcworkspace", Scheme: "Demo", Action: string(TestActionBuildForTesting), Destinations: []string{"generic/platform=iOS"}},
		},
		{
			name: "test without building",
			opts: TestOptions{Action: string(TestActionTestWithoutBuilding), XctestrunPath: "Demo.xctestrun", Destinations: []string{"generic/platform=iOS"}},
		},
		{
			name:    "missing selector",
			opts:    TestOptions{Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "exactly one of --workspace or --project",
		},
		{
			name:    "both selectors",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", WorkspacePath: "Demo.xcworkspace", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "exactly one of --workspace or --project",
		},
		{
			name:    "missing destination",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo"},
			wantErr: "--destination is required",
		},
		{
			name:    "invalid action",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Action: "archive", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--action must be one of",
		},
		{
			name:    "without building requires xctestrun",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--xctestrun is required",
		},
		{
			name:    "without building rejects project",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), ProjectPath: "Demo.xcodeproj", XctestrunPath: "Demo.xctestrun", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--project and --workspace cannot be used",
		},
		{
			name:    "without building rejects clean",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), XctestrunPath: "Demo.xctestrun", Clean: true, Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--clean cannot be used",
		},
		{
			name:    "without building rejects configuration",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), XctestrunPath: "Demo.xctestrun", Configuration: "Debug", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--configuration cannot be used",
		},
		{
			name:    "without building rejects derived data path",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), XctestrunPath: "Demo.xctestrun", DerivedDataPath: "/tmp/DerivedData", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--derived-data-path cannot be used",
		},
		{
			name:    "test action rejects xctestrun",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XctestrunPath: "Demo.xctestrun", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--xctestrun is only valid",
		},
		{
			name:    "test plan conflicts with xctestrun",
			opts:    TestOptions{Action: string(TestActionTestWithoutBuilding), TestPlan: "Demo", XctestrunPath: "Demo.xctestrun", Destinations: []string{"generic/platform=iOS"}},
			wantErr: "--test-plan cannot be used",
		},
		{
			name:    "reserved result flag",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}, XcodebuildArgs: []string{"-resultBundlePath", "/tmp/other.xcresult"}},
			wantErr: "cannot override asc-managed argument",
		},
		{
			name:    "reserved test filter",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}, XcodebuildArgs: []string{"-only-testing:DemoTests/Smoke"}},
			wantErr: "cannot override asc-managed argument",
		},
		{
			name:    "empty raw flag",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}, XcodebuildArgs: []string{" "}},
			wantErr: "--xcodebuild-flag cannot be empty",
		},
		{
			name:    "control character in destination",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS\x00"}},
			wantErr: "--destination cannot contain control characters",
		},
		{
			name:    "control character in test filter",
			opts:    TestOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Destinations: []string{"generic/platform=iOS"}, OnlyTesting: []string{"DemoTests/Smoke\n"}},
			wantErr: "--only-testing cannot contain control characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTestOptions(test.opts)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateTestOptions() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateTestOptions() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestBuildTestCommandUsesTypedOptionsAndPreservesOrder(t *testing.T) {
	opts := TestOptions{
		WorkspacePath:    "Demo App.xcworkspace",
		Scheme:           "Demo App",
		Action:           string(TestActionTest),
		Configuration:    "Release Candidate",
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro", "platform=iOS Simulator,name=iPad Pro"},
		TestPlan:         "DemoTests",
		OnlyTesting:      []string{"DemoTests/LoginTests", "DemoTests/SmokeTests"},
		SkipTesting:      []string{"DemoTests/FlakyTests"},
		DerivedDataPath:  "/tmp/Derived Data/Demo",
		ResultBundlePath: "/tmp/Results/Demo.xcresult",
		Clean:            true,
		NoCodeSigning:    true,
		XcodebuildArgs:   []string{"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_TEST"},
	}

	want := []string{
		"-workspace", "Demo App.xcworkspace",
		"-scheme", "Demo App",
		"-configuration", "Release Candidate",
		"-destination", "platform=iOS Simulator,name=iPhone 17 Pro",
		"-destination", "platform=iOS Simulator,name=iPad Pro",
		"-testPlan", "DemoTests",
		"-derivedDataPath", "/tmp/Derived Data/Demo",
		"-resultBundlePath", "/tmp/Results/Demo.xcresult",
		"-only-testing:DemoTests/LoginTests",
		"-only-testing:DemoTests/SmokeTests",
		"-skip-testing:DemoTests/FlakyTests",
		"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_TEST",
		"CODE_SIGNING_ALLOWED=NO",
		"clean",
		"test",
	}
	if got := buildTestCommand(opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("buildTestCommand() = %#v\nwant %#v", got, want)
	}
}

func TestNormalizeTestOptionsPreservesTestSelectorValues(t *testing.T) {
	wantDestination := " platform=iOS Simulator,name=iPhone 17 Pro "
	wantOnlyTesting := " DemoTests/Smoke "
	wantSkipTesting := " DemoTests/Flaky "
	wantRawFlag := "  OTHER_SWIFT_FLAGS=-D ASC_TEST  "
	opts := normalizeTestOptions(TestOptions{
		ProjectPath:    "Demo.xcodeproj",
		Scheme:         "Demo",
		Destinations:   []string{wantDestination},
		OnlyTesting:    []string{wantOnlyTesting},
		SkipTesting:    []string{wantSkipTesting},
		XcodebuildArgs: []string{wantRawFlag},
	})
	if !reflect.DeepEqual(opts.Destinations, []string{wantDestination}) {
		t.Fatalf("Destinations = %#v, want literal value %#v", opts.Destinations, wantDestination)
	}
	if !reflect.DeepEqual(opts.OnlyTesting, []string{wantOnlyTesting}) {
		t.Fatalf("OnlyTesting = %#v, want literal value %#v", opts.OnlyTesting, wantOnlyTesting)
	}
	if !reflect.DeepEqual(opts.SkipTesting, []string{wantSkipTesting}) {
		t.Fatalf("SkipTesting = %#v, want literal value %#v", opts.SkipTesting, wantSkipTesting)
	}
	if !reflect.DeepEqual(opts.XcodebuildArgs, []string{wantRawFlag}) {
		t.Fatalf("XcodebuildArgs = %#v, want literal value %#v", opts.XcodebuildArgs, wantRawFlag)
	}
	args := buildTestCommand(opts)
	for _, want := range []string{
		"-destination", wantDestination,
		"-only-testing:" + wantOnlyTesting,
		"-skip-testing:" + wantSkipTesting,
		wantRawFlag,
	} {
		found := false
		for _, arg := range args {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("argv = %#v, want literal argument %q", args, want)
		}
	}
}

func TestBuildTestCommandOmitsDerivedDataForTestWithoutBuilding(t *testing.T) {
	args := buildTestCommand(TestOptions{
		Action:          string(TestActionTestWithoutBuilding),
		XctestrunPath:   "Demo.xctestrun",
		Destinations:    []string{"generic/platform=iOS"},
		Configuration:   "Debug",
		DerivedDataPath: "/tmp/DerivedData",
	})
	for index, arg := range args {
		if arg == "-derivedDataPath" || arg == "-configuration" || (index > 0 && (args[index-1] == "-derivedDataPath" || args[index-1] == "-configuration")) {
			t.Fatalf("argv = %#v, must not include build-only paths for test-without-building", args)
		}
	}
}

func TestValidateTestOptionsAuthenticationPassthroughValues(t *testing.T) {
	base := TestOptions{
		ProjectPath:   "Demo.xcodeproj",
		Scheme:        "Demo",
		Action:        string(TestActionTest),
		Destinations:  []string{"generic/platform=iOS"},
		NoCodeSigning: true,
		Clean:         true,
	}

	for _, authFlag := range []string{"-authenticationKeyPath", "-authenticationKeyID", "-authenticationKeyIssuerID"} {
		t.Run(strings.TrimPrefix(authFlag, "-")+"/missing", func(t *testing.T) {
			opts := base
			opts.XcodebuildArgs = []string{authFlag}
			err := ValidateTestOptions(opts)
			want := fmt.Sprintf("--xcodebuild-flag %q requires a following value", authFlag)
			if err == nil || err.Error() != want {
				t.Fatalf("ValidateTestOptions() error = %v, want %q", err, want)
			}
		})
	}

	for _, next := range []string{
		"-authenticationKeyID",
		"-destination",
		"CODE_SIGNING_ALLOWED=NO",
		"clean",
		"test",
	} {
		t.Run("recognized value/"+next, func(t *testing.T) {
			opts := base
			opts.XcodebuildArgs = []string{"-authenticationKeyPath", next}
			err := ValidateTestOptions(opts)
			want := fmt.Sprintf("--xcodebuild-flag %q requires a value; %q is a recognized xcodebuild option or asc-managed argument", "-authenticationKeyPath", next)
			if err == nil || err.Error() != want {
				t.Fatalf("ValidateTestOptions() error = %v, want %q", err, want)
			}
		})
	}

	for _, next := range []string{"-quiet", "-verbose", "-showBuildTimingSummary"} {
		t.Run("option token value/"+next, func(t *testing.T) {
			opts := base
			opts.XcodebuildArgs = []string{"-authenticationKeyPath", next}
			err := ValidateTestOptions(opts)
			want := fmt.Sprintf("--xcodebuild-flag %q requires a value; %q is an xcodebuild option", "-authenticationKeyPath", next)
			if err == nil || err.Error() != want {
				t.Fatalf("ValidateTestOptions() error = %v, want %q", err, want)
			}
		})
	}

	t.Run("empty value keeps raw validation", func(t *testing.T) {
		opts := base
		opts.XcodebuildArgs = []string{"-authenticationKeyPath", ""}
		if err := ValidateTestOptions(opts); err == nil || err.Error() != "--xcodebuild-flag cannot be empty" {
			t.Fatalf("ValidateTestOptions() error = %v, want raw empty-value error", err)
		}
	})

	t.Run("control value keeps raw validation", func(t *testing.T) {
		opts := base
		opts.XcodebuildArgs = []string{"-authenticationKeyPath", "AuthKey\x00.p8"}
		if err := ValidateTestOptions(opts); err == nil || err.Error() != "--xcodebuild-flag cannot contain control characters" {
			t.Fatalf("ValidateTestOptions() error = %v, want raw control-character error", err)
		}
	})

	t.Run("valid pairs preserve bytes and managed boundaries", func(t *testing.T) {
		raw := []string{
			"-authenticationKeyPath", "  /tmp/Auth Key.p8  ",
			"-authenticationKeyID", "Key ID 123",
			"-authenticationKeyIssuerID", "Issuer ID 456",
			"-quiet",
		}
		opts := base
		opts.XcodebuildArgs = raw
		if err := ValidateTestOptions(opts); err != nil {
			t.Fatalf("ValidateTestOptions() error = %v", err)
		}
		args := buildTestCommand(opts)
		wantSuffix := append(append([]string(nil), raw...), "CODE_SIGNING_ALLOWED=NO", "clean", "test")
		if got := args[len(args)-len(wantSuffix):]; !reflect.DeepEqual(got, wantSuffix) {
			t.Fatalf("buildTestCommand() suffix = %#v, want %#v", got, wantSuffix)
		}
	})

	for _, authFlag := range []string{"-authenticationKeyPath", "-authenticationKeyID", "-authenticationKeyIssuerID"} {
		t.Run("equals form/"+authFlag, func(t *testing.T) {
			raw := authFlag + "=/tmp/Auth Key.p8"
			opts := base
			opts.XcodebuildArgs = []string{raw, "-quiet"}
			if err := ValidateTestOptions(opts); err != nil {
				t.Fatalf("ValidateTestOptions() error = %v, want equals-form argument accepted", err)
			}
			args := buildTestCommand(opts)
			wantSuffix := []string{raw, "-quiet", "CODE_SIGNING_ALLOWED=NO", "clean", "test"}
			if got := args[len(args)-len(wantSuffix):]; !reflect.DeepEqual(got, wantSuffix) {
				t.Fatalf("buildTestCommand() suffix = %#v, want %#v", got, wantSuffix)
			}
		})
	}

	for _, authFlag := range []string{"-authenticationKeyPath", "-authenticationKeyID", "-authenticationKeyIssuerID"} {
		for _, value := range []string{"", "   "} {
			t.Run("equals empty/"+authFlag+"/"+fmt.Sprintf("%q", value), func(t *testing.T) {
				raw := authFlag + "=" + value
				opts := base
				opts.XcodebuildArgs = []string{raw}
				want := fmt.Sprintf("--xcodebuild-flag %q cannot have an empty value", strings.TrimSpace(raw))
				if err := ValidateTestOptions(opts); err == nil || err.Error() != want {
					t.Fatalf("ValidateTestOptions() error = %v, want %q", err, want)
				}
			})
		}
	}
}

func TestParseTestResultCasesRejectsUnknownStatus(t *testing.T) {
	data := []byte(`[{"identifier":"DemoTests/Smoke/testPending","status":"running"}]`)
	if _, err := ParseTestResultCases(data); err == nil || !strings.Contains(err.Error(), "unsupported status") {
		t.Fatalf("ParseTestResultCases() error = %v, want unsupported-status error", err)
	}
}

func TestParseTestResultCasesRejectsMalformedNodeDuration(t *testing.T) {
	data := []byte(`{"testNodes":[{"nodeType":"Test Case","nodeIdentifier":"Demo/test","result":"Passed","durationMs":"not-a-number"}]}`)
	if _, err := ParseTestResultCases(data); err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("ParseTestResultCases() error = %v, want malformed-duration error", err)
	}
}

func TestParseTestResultCasesPreservesDurationMilliseconds(t *testing.T) {
	data := []byte(`[{"identifier":"DemoTests/Smoke/testPass","status":"passed","durationMs":17}]`)
	cases, err := ParseTestResultCases(data)
	if err != nil {
		t.Fatalf("ParseTestResultCases() error = %v", err)
	}
	if len(cases) != 1 || cases[0].DurationMS != 17 {
		t.Fatalf("cases = %+v, want duration 17ms", cases)
	}
}

func TestParseTestResultSummaryPreservesDurationMilliseconds(t *testing.T) {
	data := []byte(`{"totalTestCount":1,"passedTests":1,"failedTests":0,"skippedTests":0,"durationMs":17}`)
	summary, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if summary.DurationMS != 17 {
		t.Fatalf("DurationMS = %d, want 17", summary.DurationMS)
	}
}

func TestParseTestResultDurationsPreferExplicitZeroMilliseconds(t *testing.T) {
	summary, err := ParseTestResultSummary([]byte(`{"totalTestCount":0,"passedTests":0,"failedTests":0,"skippedTests":0,"durationMs":0,"testDuration":3.5}`))
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if summary.DurationMS != 0 {
		t.Fatalf("summary DurationMS = %d, want explicit zero", summary.DurationMS)
	}
	cases, err := ParseTestResultCases([]byte(`[{"identifier":"Demo/test","status":"passed","durationMs":0,"duration":0.25}]`))
	if err != nil {
		t.Fatalf("ParseTestResultCases() error = %v", err)
	}
	if len(cases) != 1 || cases[0].DurationMS != 0 {
		t.Fatalf("cases = %+v, want explicit zero duration", cases)
	}
}

func TestParseTestResultSummaryPreservesFlattenedCasesWhenUnitsDiffer(t *testing.T) {
	data := []byte(`{
  "totalTestCount":1,
  "passedTests":1,
  "failedTests":0,
  "skippedTests":0,
  "tests":[
    {"identifier":"Demo/testDestinationA","status":"passed"},
    {"identifier":"Demo/testDestinationB","status":"passed"}
  ]
}`)
	summary, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v, want aggregate/case unit mismatch to remain representable", err)
	}
	if summary.Total != 1 || len(summary.Cases) != 2 {
		t.Fatalf("summary = %+v, want aggregate total 1 and both flattened cases", summary)
	}
}

func TestRunXcresulttoolJSONRejectsOversizedOutput(t *testing.T) {
	useTrustedTestCommandNames(t)
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 16777217 /dev/zero")
	}
	_, err := runXcresulttoolJSON(context.Background(), "summary", "/tmp/Demo.xcresult")
	if err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("runXcresulttoolJSON() error = %v, want bounded-output error", err)
	}
}

func TestRunXcresulttoolJSONPreservesBoundedDiagnostics(t *testing.T) {
	useTrustedTestCommandNames(t)
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	commandContextFn = helperCommandContext(t, filepath.Join(t.TempDir(), "commands.log"))
	t.Setenv("ASC_XCODE_HELPER_XCRESULT_STDERR", "result bundle is unreadable")
	t.Setenv("ASC_XCODE_HELPER_XCRESULT_EXIT_CODE", "65")

	_, err := runXcresulttoolJSON(context.Background(), "summary", "/tmp/Demo.xcresult")
	if err == nil || !strings.Contains(err.Error(), "result bundle is unreadable") {
		t.Fatalf("runXcresulttoolJSON() error = %v, want bounded xcresulttool diagnostics", err)
	}
}

func TestBuildTestCommandSupportsWithoutBuilding(t *testing.T) {
	opts := TestOptions{
		Action:           string(TestActionTestWithoutBuilding),
		XctestrunPath:    "/tmp/App.xctestrun",
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		ResultBundlePath: "/tmp/App-tests.xcresult",
		OnlyTesting:      []string{"AppTests/Smoke"},
	}
	want := []string{
		"-destination", "platform=iOS Simulator,name=iPhone 17 Pro",
		"-xctestrun", "/tmp/App.xctestrun",
		"-resultBundlePath", "/tmp/App-tests.xcresult",
		"-only-testing:AppTests/Smoke",
		"test-without-building",
	}
	if got := buildTestCommand(opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("buildTestCommand() = %#v\nwant %#v", got, want)
	}
}

func TestParseTestResultSummaryWithCases(t *testing.T) {
	data := []byte(`{
  "tests": [
    {"testIdentifier":"DemoTests/Login/testValid","status":"Passed","duration":0.25},
    {"testIdentifier":"DemoTests/Login/testInvalid","status":"Failed","durationMs":125,"failureMessage":"assertion failed"},
    {"testIdentifier":"DemoTests/Login/testSkipped","status":"Skipped"}
  ],
  "testDuration": 1.25,
  "testFailures": [{"testIdentifier":"DemoTests/Login/testInvalid","message":"assertion failed"}]
}`)

	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if got.Total != 3 || got.Passed != 1 || got.Failed != 1 || got.Skipped != 1 || got.DurationMS != 1250 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if len(got.Cases) != 3 || got.Cases[1].Status != "failed" || got.Cases[0].DurationMS != 250 || got.Cases[1].DurationMS != 125 {
		t.Fatalf("unexpected cases: %+v", got.Cases)
	}
	if len(got.Failures) != 1 || got.Failures[0].Identifier != "DemoTests/Login/testInvalid" {
		t.Fatalf("unexpected failures: %+v", got.Failures)
	}
}

func TestParseTestResultSummaryAllowsExpectedFailures(t *testing.T) {
	data := []byte(`{
  "totalTestCount":2,
  "passedTests":1,
  "failedTests":0,
  "skippedTests":0,
  "expectedFailures":1,
  "tests":[
    {"testIdentifier":"DemoTests/Smoke/testPass","status":"Passed"},
    {"testIdentifier":"DemoTests/Smoke/testKnownIssue","status":"Expected Failure"}
  ]
}`)

	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v, want expected failure to remain nonfailing", err)
	}
	if got.Total != 2 || got.Passed != 1 || got.Failed != 0 || got.Skipped != 0 || got.ExpectedFailures != 1 {
		t.Fatalf("unexpected expected-failure summary: %+v", got)
	}
	if len(got.Cases) != 2 || got.Cases[1].Status != "expected-failure" {
		t.Fatalf("unexpected expected-failure cases: %+v", got.Cases)
	}
}

func TestParseTestResultCasesAcceptsExpectedFailureStatus(t *testing.T) {
	for _, status := range []string{"Expected Failure", "expectedFailure", "expected-failure", "expected_failure"} {
		t.Run(status, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{
  "testNodes":[{
    "nodeType":"Test Suite",
    "children":[{
      "nodeType":"Test Case",
      "nodeIdentifier":"DemoTests/Smoke/testKnownIssue",
      "result":%q
    }]
  }]
}`, status))

			cases, err := ParseTestResultCases(data)
			if err != nil {
				t.Fatalf("ParseTestResultCases() error = %v, want expected failure status accepted", err)
			}
			if len(cases) != 1 || cases[0].Status != "expected-failure" {
				t.Fatalf("expected-failure cases = %+v, want normalized nonfailing status", cases)
			}
		})
	}
}

func TestParseTestResultSummaryAcceptsExpectedFailureCountForms(t *testing.T) {
	data := []byte(`{
  "passedTests":1,
  "failedTests":0,
  "skippedTests":0,
  "expectedFailures":"1",
  "tests":[
    {"testIdentifier":"DemoTests/Smoke/testPass","status":"passed"},
    {"testIdentifier":"DemoTests/Smoke/testKnownIssue","status":"expectedFailure"}
  ]
}`)
	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v, want string expected-failure count accepted", err)
	}
	if got.Total != 2 || got.ExpectedFailures != 1 || len(got.Cases) != 2 {
		t.Fatalf("summary = %+v, want inferred total and expected-failure count", got)
	}
}

func TestParseTestResultSummaryRejectsInvalidExpectedFailureCounts(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"totalTestCount":1,"passedTests":0,"failedTests":0,"skippedTests":0,"expectedFailures":-1}`),
		[]byte(`{"totalTestCount":1,"passedTests":0,"failedTests":0,"skippedTests":0,"expectedFailures":2}`),
		[]byte(`{"totalTestCount":2,"passedTests":1,"failedTests":0,"skippedTests":0,"expectedFailures":0}`),
	} {
		if _, err := ParseTestResultSummary(data); err == nil {
			t.Fatalf("ParseTestResultSummary(%s) succeeded, want invalid expected-failure counts rejected", data)
		}
	}
}

func TestReadTestResultSummaryBoundsMergedCaseFailures(t *testing.T) {
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
	})

	cases := make([]map[string]string, maxTestFailureCount+1)
	for index := range cases {
		cases[index] = map[string]string{
			"identifier": "DemoTests/Smoke/test" + strconv.Itoa(index),
			"status":     "failed",
		}
	}
	encodedCases, err := json.Marshal(cases)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	summaryOutput := `{"totalTestCount":101,"passedTests":0,"failedTests":101,"skippedTests":0}`
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcrun", nil }
	commandContextFn = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		output := summaryOutput
		if len(args) > 3 && args[3] == "tests" {
			output = string(encodedCases)
		}
		return exec.CommandContext(ctx, "printf", "%s", output)
	}

	got, err := readTestResultSummary(context.Background(), "/tmp/Demo.xcresult")
	if err != nil {
		t.Fatalf("readTestResultSummary() error = %v", err)
	}
	if len(got.Failures) != maxTestFailureCount {
		t.Fatalf("failure count = %d, want cap %d", len(got.Failures), maxTestFailureCount)
	}
}

func TestParseTestResultCasesAllowsStructuralNodesAfterCaseLimit(t *testing.T) {
	cases := make([]rawTestNode, maxTestCaseCount)
	for index := range cases {
		cases[index] = rawTestNode{
			NodeType:   "Test Case",
			Identifier: "DemoTests/Smoke/test" + strconv.Itoa(index),
			Result:     "passed",
		}
	}
	data, err := json.Marshal(rawTestResults{TestNodes: []rawTestNode{
		{NodeType: "Test Suite", Children: cases},
		{NodeType: "Test Suite"},
	}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	got, err := ParseTestResultCases(data)
	if err != nil {
		t.Fatalf("ParseTestResultCases() error = %v, want structural nodes after cap accepted", err)
	}
	if len(got) != maxTestCaseCount {
		t.Fatalf("case count = %d, want %d", len(got), maxTestCaseCount)
	}
}

func TestParseTestResultSummaryWithCounts(t *testing.T) {
	data := []byte(`{"tests":4,"passedTests":3,"failedTests":1,"skippedTests":0,"testDuration":2.5}`)
	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if got.Total != 4 || got.Passed != 3 || got.Failed != 1 || got.Skipped != 0 || got.DurationMS != 2500 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if len(got.Cases) != 0 {
		t.Fatalf("expected no cases in count-only summary, got %+v", got.Cases)
	}
}

func TestParseTestResultSummaryUsesCurrentXcodeFields(t *testing.T) {
	data := []byte(`{
  "result":"Failed",
  "totalTestCount":3,
  "passedTests":1,
  "failedTests":1,
  "skippedTests":1,
  "startTime":10.25,
  "finishTime":12.5,
  "testFailures":[{
    "testIdentifier":14,
    "testIdentifierString":"DemoTests/Login/testInvalid",
    "testName":"testInvalid",
    "targetName":"DemoTests",
    "failureText":"assertion failed"
  }]
}`)
	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if got.Total != 3 || got.Passed != 1 || got.Failed != 1 || got.Skipped != 1 || got.DurationMS != 2250 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if len(got.Failures) != 1 || got.Failures[0].Identifier != "DemoTests/Login/testInvalid" || got.Failures[0].Message != "assertion failed" {
		t.Fatalf("unexpected failures: %+v", got.Failures)
	}
}

func TestParseTestResultCasesWalksCurrentXcodeTree(t *testing.T) {
	data := []byte(`{
  "testNodes":[{
    "nodeType":"Test Plan",
    "children":[{
      "nodeType":"Unit test bundle",
      "children":[{
        "nodeType":"Test Suite",
        "children":[
          {"nodeType":"Test Case","nodeIdentifier":"DemoTests/Login/testValid","name":"testValid","result":"Passed","duration":"0.25"},
          {"nodeType":"Test Case","nodeIdentifier":"DemoTests/Login/testInvalid","name":"testInvalid","result":"Failed","children":[{"nodeType":"Failure Message","name":"assertion failed"}]},
          {"nodeType":"Test Case","nodeIdentifier":"DemoTests/Login/testSkipped","name":"testSkipped","result":"Skipped"}
        ]
      }]
    }]
  }]
}`)
	cases, err := ParseTestResultCases(data)
	if err != nil {
		t.Fatalf("ParseTestResultCases() error = %v", err)
	}
	if len(cases) != 3 {
		t.Fatalf("len(cases) = %d, want 3", len(cases))
	}
	if cases[0].Identifier != "DemoTests/Login/testValid" || cases[0].Classname != "DemoTests" || cases[0].DurationMS != 250 {
		t.Fatalf("unexpected passing case: %+v", cases[0])
	}
	if cases[1].Status != "failed" || cases[1].Message != "assertion failed" {
		t.Fatalf("unexpected failing case: %+v", cases[1])
	}
	if cases[2].Status != "skipped" {
		t.Fatalf("unexpected skipped case: %+v", cases[2])
	}
}

func TestParseTestResultSummaryRejectsInvalidCountsAndMissingCount(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"tests":2,"passedTests":2,"failedTests":1}`),
		[]byte(`{"tests":2,"passedTests":1,"failedTests":0,"skippedTests":0,"expectedFailures":0}`),
		[]byte(`{"totalTestCount":2}`),
		[]byte(`{"result":"Passed"}`),
		[]byte(`{"tests":[{"identifier":"DemoTests/Smoke/testPending","status":"running"}]}`),
		[]byte(`{"totalTestCount":1,"passedTests":1,"failedTests":0,"skippedTests":0,"tests":[1]}`),
		[]byte(`{"totalTestCount":1,"passedTests":0,"failedTests":1,"skippedTests":0,"tests":[{"identifier":"DemoTests/Smoke/testPass","status":"passed"}]}`),
	} {
		if _, err := ParseTestResultSummary(data); err == nil {
			t.Fatalf("ParseTestResultSummary(%s) succeeded, want error", data)
		}
	}
}

func TestParseTestResultSummaryBoundsFailureMessage(t *testing.T) {
	message := strings.Repeat("x", maxTestFailureMessage+100)
	data, err := json.Marshal(map[string]any{
		"tests":        1,
		"passedTests":  0,
		"failedTests":  1,
		"skippedTests": 0,
		"testFailures": []map[string]string{{"testIdentifier": "Demo/test", "message": message}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	got, err := ParseTestResultSummary(data)
	if err != nil {
		t.Fatalf("ParseTestResultSummary() error = %v", err)
	}
	if len(got.Failures) != 1 || len(got.Failures[0].Message) != maxTestFailureMessage {
		t.Fatalf("failure message length = %d, want %d", len(got.Failures[0].Message), maxTestFailureMessage)
	}
}

func TestParseTestMessagesStayValidUTF8AtByteCap(t *testing.T) {
	for _, suffix := range []string{"é", "☃", "😀"} {
		t.Run(suffix, func(t *testing.T) {
			message := strings.Repeat("a", maxTestFailureMessage-1) + suffix

			casesJSON := []byte(fmt.Sprintf(`{
  "testNodes":[{
    "nodeType":"Test Case",
    "nodeIdentifier":"DemoTests/Smoke/testInvalid",
    "result":"Failed",
    "message":%q
  }]
}`, message))
			cases, err := ParseTestResultCases(casesJSON)
			if err != nil {
				t.Fatalf("ParseTestResultCases() error = %v", err)
			}
			if len(cases) != 1 || !utf8.ValidString(cases[0].Message) || len(cases[0].Message) > maxTestFailureMessage {
				t.Fatalf("case message = %q, valid UTF-8 = %t, bytes = %d; want valid and <= %d", cases[0].Message, utf8.ValidString(cases[0].Message), len(cases[0].Message), maxTestFailureMessage)
			}

			summaryJSON := []byte(fmt.Sprintf(`{
  "totalTestCount":1,
  "passedTests":0,
  "failedTests":1,
  "skippedTests":0,
  "testFailures":[{"testIdentifier":"DemoTests/Smoke/testInvalid","message":%q}],
  "tests":[{"testIdentifier":"DemoTests/Smoke/testEnriched","status":"Failed","message":%q}]
}`, message, message))
			summary, err := ParseTestResultSummary(summaryJSON)
			if err != nil {
				t.Fatalf("ParseTestResultSummary() error = %v", err)
			}
			if len(summary.Failures) != 2 {
				t.Fatalf("summary failures = %+v, want raw and enriched failures", summary.Failures)
			}
			for _, failure := range summary.Failures {
				if !utf8.ValidString(failure.Message) || len(failure.Message) > maxTestFailureMessage {
					t.Fatalf("failure %q message valid UTF-8 = %t, bytes = %d; want valid and <= %d", failure.Identifier, utf8.ValidString(failure.Message), len(failure.Message), maxTestFailureMessage)
				}
			}
		})
	}
}

func TestResolveTestPaths(t *testing.T) {
	originalCache := userCacheDirFn
	originalNow := testNowFn
	t.Cleanup(func() {
		userCacheDirFn = originalCache
		testNowFn = originalNow
	})
	userCacheDirFn = func() (string, error) { return "/tmp/asc-cache", nil }
	testNowFn = func() time.Time { return time.Unix(1700000000, 1234) }

	opts := TestOptions{
		ProjectPath:  "Demo.xcodeproj",
		Scheme:       "Demo App",
		Action:       string(TestActionTest),
		Destinations: []string{"generic/platform=iOS"},
	}
	derived, err := resolveTestDerivedDataPath(opts)
	if err != nil {
		t.Fatalf("resolveTestDerivedDataPath() error = %v", err)
	}
	if !strings.HasPrefix(derived, filepath.Join("/tmp/asc-cache", "asc", "xcode-test", "demo-app-")) {
		t.Fatalf("derived path = %q, want cache prefix", derived)
	}
	result, err := resolveTestResultBundlePath(opts)
	if err != nil {
		t.Fatalf("resolveTestResultBundlePath() error = %v", err)
	}
	if !strings.HasSuffix(result, ".xcresult") || !strings.Contains(result, "1700000000000001234") {
		t.Fatalf("result path = %q, want timestamped xcresult", result)
	}
}

func TestFindXctestrunPathRequiresExactlyOneRegularCandidate(t *testing.T) {
	derived := t.TempDir()
	products := filepath.Join(derived, "Build", "Products")
	if err := os.MkdirAll(products, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(products, "Demo.xctestrun")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := findXctestrunPath(derived); got != path {
		t.Fatalf("findXctestrunPath() = %q, want %q", got, path)
	}
	if err := os.WriteFile(filepath.Join(products, "Other.xctestrun"), []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := findXctestrunPath(derived); got != "" {
		t.Fatalf("findXctestrunPath() = %q, want empty for ambiguous candidates", got)
	}
}

func TestValidateTestResultBundleDestination(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
	}{
		{name: "regular file", setup: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
		}},
		{name: "directory", setup: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("Mkdir() error = %v", err)
			}
		}},
		{name: "dangling symlink", setup: func(t *testing.T, path string) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows symlink creation requires elevated privileges")
			}
			if err := os.Symlink(filepath.Join(filepath.Dir(path), "missing-target"), path); err != nil {
				t.Fatalf("Symlink() error = %v", err)
			}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "existing.xcresult")
			test.setup(t, path)
			if err := validateTestResultBundleDestination(path); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("validateTestResultBundleDestination() error = %v, want existing-path error", err)
			}
		})
	}

	if err := validateTestResultBundleDestination(filepath.Join(t.TempDir(), "new.xcresult")); err != nil {
		t.Fatalf("validateTestResultBundleDestination() error = %v", err)
	}
}

func TestValidateTestResultBundleDestinationRejectsSymlinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}
	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	symlinkParent := filepath.Join(root, "link")
	if err := os.Symlink(realParent, symlinkParent); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	path := filepath.Join(symlinkParent, "new.xcresult")
	if err := validateTestResultBundleDestination(path); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("validateTestResultBundleDestination() error = %v, want symlink-parent error", err)
	}
}

func TestSetTestExitStatusLeavesSignalsWithoutStatus(t *testing.T) {
	result := &TestResult{}
	setTestExitStatus(result, errors.New("not an exit error"))
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil", result.ExitStatus)
	}
}

func TestReadTestResultSummaryUsesCurrentXcodeOperations(t *testing.T) {
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
	})
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcrun", nil }
	var commands [][]string
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commands = append(commands, append([]string{name}, args...))
		output := `{"totalTestCount":1,"passedTests":1,"failedTests":0,"skippedTests":0}`
		if len(args) > 3 && args[3] == "tests" {
			output = `{"testNodes":[{"nodeType":"Test Plan","children":[{"nodeType":"Unit test bundle","children":[{"nodeType":"Test Suite","children":[{"nodeType":"Test Case","nodeIdentifier":"DemoTests/Smoke/testPass","name":"testPass","result":"Passed"},{"nodeType":"Test Case","nodeIdentifier":"DemoTests/Smoke/testPassAgain","name":"testPassAgain","result":"Passed"}]}]}]}]}`
		}
		return exec.CommandContext(ctx, "printf", "%s", output)
	}

	got, err := readTestResultSummary(context.Background(), "/tmp/Demo.xcresult")
	if err != nil {
		t.Fatalf("readTestResultSummary() error = %v", err)
	}
	if got.Total != 1 || got.Passed != 1 || len(got.Cases) != 2 || got.Cases[0].Identifier != "DemoTests/Smoke/testPass" || got.Cases[1].Identifier != "DemoTests/Smoke/testPassAgain" {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if len(commands) != 2 {
		t.Fatalf("xcresulttool command count = %d, want 2", len(commands))
	}
	wantPrefix := []string{"/usr/bin/xcrun", "xcresulttool", "get", "test-results", "summary", "--path", "/tmp/Demo.xcresult", "--compact"}
	if !reflect.DeepEqual(commands[0], wantPrefix) {
		t.Fatalf("summary command = %#v, want %#v", commands[0], wantPrefix)
	}
	if commands[1][4] != "tests" || commands[1][len(commands[1])-1] != "--compact" {
		t.Fatalf("tests command = %#v, want tests operation with compact output", commands[1])
	}
}

func TestReadTestResultSummaryRetainsAggregateWhenCaseEnrichmentFails(t *testing.T) {
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
	})
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcrun", nil }
	commandContextFn = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		if len(args) > 3 && args[3] == "tests" {
			return exec.CommandContext(ctx, "sh", "-c", "echo tests enrichment unavailable >&2; exit 17")
		}
		return exec.CommandContext(ctx, "printf", "%s", `{"totalTestCount":3,"passedTests":1,"failedTests":1,"skippedTests":1,"testFailures":[{"testIdentifierString":"DemoTests/Smoke/testFail","failureText":"assertion failed"}]}`)
	}

	got, err := readTestResultSummary(context.Background(), "/tmp/Demo.xcresult")
	if err == nil || !strings.Contains(err.Error(), "run xcresulttool test-results tests") {
		t.Fatalf("readTestResultSummary() error = %v, want case-enrichment failure", err)
	}
	if got == nil {
		t.Fatal("readTestResultSummary() returned nil summary after aggregate succeeded")
	}
	if got.Total != 3 || got.Passed != 1 || got.Failed != 1 || got.Skipped != 1 {
		t.Fatalf("aggregate summary = %+v, want preserved counts", got)
	}
	if len(got.Failures) != 1 || got.Failures[0].Identifier != "DemoTests/Smoke/testFail" {
		t.Fatalf("aggregate failures = %+v, want preserved failure metadata", got.Failures)
	}
}

func TestTestRunsActionAndParsesResult(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	var gotArgs []string
	runXcodeTestCommand = func(_ context.Context, args []string, _ io.Writer) error {
		gotArgs = append([]string(nil), args...)
		for index, arg := range args {
			if arg == "-resultBundlePath" && index+1 < len(args) {
				if err := os.Mkdir(args[index+1], 0o755); err != nil {
					return err
				}
			}
		}
		return nil
	}
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		return &TestSummary{
			Total:      1,
			Passed:     1,
			DurationMS: 250,
			Cases:      []TestCase{{Identifier: "DemoTests/Smoke/testPass", Status: "passed"}},
		}, nil
	}

	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.Success || result.Tests == nil || result.Tests.Total != 1 || result.ExitStatus == nil || *result.ExitStatus != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.ResultBundlePath != resultPath || len(gotArgs) == 0 || gotArgs[len(gotArgs)-1] != "test" {
		t.Fatalf("result path/argv = %q/%#v, want explicit result path and test action", result.ResultBundlePath, gotArgs)
	}
}

func TestTestRejectsFailedPostProcessingSummary(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	runXcodeTestCommand = func(_ context.Context, args []string, _ io.Writer) error {
		for index, arg := range args {
			if arg == "-resultBundlePath" && index+1 < len(args) {
				return os.Mkdir(args[index+1], 0o755)
			}
		}
		return errors.New("result bundle argument missing")
	}
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		return &TestSummary{
			Total:  1,
			Failed: 1,
			Cases:  []TestCase{{Identifier: "DemoTests/Smoke/testFail", Status: "failed"}},
		}, nil
	}

	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if err == nil || !strings.Contains(err.Error(), "failed tests") {
		t.Fatalf("Test() error = %v, want failed-test result error", err)
	}
	var reportedFailures *ReportedTestFailuresError
	if !errors.As(err, &reportedFailures) {
		t.Fatalf("Test() error = %T %v, want *ReportedTestFailuresError", err, err)
	}
	if reportedFailures.Failed != 1 {
		t.Fatalf("ReportedTestFailuresError.Failed = %d, want 1", reportedFailures.Failed)
	}
	if got, want := err.Error(), "xcode test result reported 1 failed tests"; got != want {
		t.Fatalf("Test() error message = %q, want %q", got, want)
	}
	if result.Success || result.Tests == nil || result.Tests.Failed != 1 || result.ExitStatus != nil {
		t.Fatalf("unexpected failed-result payload: %+v", result)
	}
}

func TestTestRejectsResultBundleSymlinkCreatedAfterPreflight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	runXcodeTestCommand = func(_ context.Context, _ []string, _ io.Writer) error {
		return os.Symlink(filepath.Join(filepath.Dir(resultPath), "outside.xcresult"), resultPath)
	}
	readCalls := 0
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		readCalls++
		return &TestSummary{Total: 1, Passed: 1, Cases: []TestCase{{Identifier: "DemoTests/Smoke/testPass", Status: "passed"}}}, nil
	}

	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Test() error = %v, want result-bundle symlink error", err)
	}
	if result.Success || readCalls != 0 {
		t.Fatalf("unexpected symlink result: %+v, readCalls=%d", result, readCalls)
	}
}

func TestTestPreservesProcessFailureAndPartialSummary(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	processErr := exec.Command("sh", "-c", "exit 65").Run()
	runXcodeTestCommand = func(_ context.Context, _ []string, _ io.Writer) error {
		if err := os.Mkdir(resultPath, 0o755); err != nil {
			return err
		}
		return processErr
	}
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		return &TestSummary{Total: 1, Failed: 1}, nil
	}
	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if !errors.Is(err, processErr) {
		t.Fatalf("Test() error = %v, want process error %v", err, processErr)
	}
	if result.Success || result.Tests == nil || result.Tests.Failed != 1 || result.ExitStatus == nil || *result.ExitStatus != 65 {
		t.Fatalf("unexpected failure result: %+v", result)
	}
}

func TestTestRecoversPartialSummaryWithFreshPostProcessingContextAfterCancellation(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runXcodeTestCommand = func(_ context.Context, _ []string, _ io.Writer) error {
		if err := os.Mkdir(resultPath, 0o755); err != nil {
			return err
		}
		cancel()
		return context.Canceled
	}
	var gotSummaryContext context.Context
	var gotLiveSummaryContext bool
	readTestResultSummaryFn = func(summaryContext context.Context, _ string) (*TestSummary, error) {
		gotSummaryContext = summaryContext
		if summaryContext.Err() != nil {
			return nil, summaryContext.Err()
		}
		if _, ok := summaryContext.Deadline(); !ok {
			return nil, errors.New("post-processing context has no deadline")
		}
		gotLiveSummaryContext = true
		return &TestSummary{
			Total:  1,
			Failed: 1,
			Cases:  []TestCase{{Identifier: "DemoTests/Smoke/testFail", Status: "failed"}},
		}, nil
	}

	result, err := Test(ctx, TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Test() error = %v, want context cancellation", err)
	}
	if result == nil || result.Tests == nil || result.Tests.Failed != 1 {
		t.Fatalf("partial summary = %+v, want failed test summary", result)
	}
	if gotSummaryContext == nil {
		t.Fatal("readTestResultSummaryFn was not called")
	}
	if !gotLiveSummaryContext {
		t.Fatal("post-processing reader did not receive a live deadline-bearing context")
	}
}

func TestTestOmitsExitStatusForContextCancellation(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runXcodeTestCommand = func(ctx context.Context, _ []string, _ io.Writer) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}

	result, err := Test(ctx, TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Test() error = %v, want context cancellation", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Test() result = %+v, want canceled failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil for cancellation", result.ExitStatus)
	}
}

func TestTestOmitsExitStatusForResultPostProcessingFailure(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	runXcodeTestCommand = func(_ context.Context, _ []string, _ io.Writer) error {
		return os.Mkdir(resultPath, 0o755)
	}
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		return nil, errors.New("unsupported result summary")
	}
	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported result summary") {
		t.Fatalf("Test() error = %v, want post-processing failure", err)
	}
	if result.Success || result.ExitStatus != nil {
		t.Fatalf("unexpected post-processing result: %+v", result)
	}
}

func TestTestRetainsAggregateWhenCaseEnrichmentFails(t *testing.T) {
	originalRuntimeGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalRun := runXcodeTestCommand
	originalRead := readTestResultSummaryFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		runtimeGOOS = originalRuntimeGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		runXcodeTestCommand = originalRun
		readTestResultSummaryFn = originalRead
	})
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	resultPath := filepath.Join(t.TempDir(), "Demo-tests.xcresult")
	runXcodeTestCommand = func(_ context.Context, _ []string, _ io.Writer) error {
		return os.Mkdir(resultPath, 0o755)
	}
	enrichmentErr := errors.New("tests enrichment unavailable")
	readTestResultSummaryFn = func(context.Context, string) (*TestSummary, error) {
		return &TestSummary{
			Total:  2,
			Passed: 1,
			Failed: 1,
			Failures: []TestFailure{{
				Identifier: "DemoTests/Smoke/testFail",
				Message:    "assertion failed",
			}},
		}, enrichmentErr
	}

	result, err := Test(context.Background(), TestOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Action:           string(TestActionTest),
		Destinations:     []string{"platform=iOS Simulator,name=iPhone 17 Pro"},
		DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
		ResultBundlePath: resultPath,
	})
	if !errors.Is(err, enrichmentErr) {
		t.Fatalf("Test() error = %v, want enrichment error", err)
	}
	if result == nil || result.Success || result.Tests == nil {
		t.Fatalf("Test() result = %+v, want failed result with aggregate summary", result)
	}
	if result.Tests.Total != 2 || result.Tests.Failed != 1 || len(result.Tests.Failures) != 1 {
		t.Fatalf("retained aggregate = %+v, want counts and failure metadata", result.Tests)
	}
}
