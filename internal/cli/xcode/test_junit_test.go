package xcode

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const junitSummaryFixture = `{
  "totalTestCount": 2,
  "passedTests": 1,
  "failedTests": 1,
  "skippedTests": 0,
  "expectedFailures": 0
}`

const junitCasesFixture = `{
  "testNodes": [
    {
      "nodeType": "Test Case",
      "nodeIdentifier": "DemoTests/Smoke/testPass",
      "name": "testPass",
      "result": "Passed"
    },
    {
      "nodeType": "Test Case",
      "nodeIdentifier": "DemoTests/Smoke/testFail",
      "name": "testFail",
      "result": "Failed",
      "message": "expected true"
    }
  ]
}`

func TestXcodeTestJUnitWritesGoldenReportFromFixtureSummary(t *testing.T) {
	summary, err := localxcode.ParseTestResultSummary([]byte(junitSummaryFixture))
	if err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	cases, err := localxcode.ParseTestResultCases([]byte(junitCasesFixture))
	if err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	summary.Cases = cases
	reportFile := filepath.Join(t.TempDir(), "junit.xml")
	t.Cleanup(SetXCResultSummaryLoaderForTesting(func(ctx context.Context, _ string) (*localxcode.TestSummary, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("loader context has no deadline")
		}
		return summary, nil
	}))

	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestJUnitCommand(d)
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json", "--allow-failures"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatal(err)
	}
	xml := string(data)
	for _, want := range []string{`name="testPass"`, `name="testFail"`, `failures="1"`, `asc xcode test junit`} {
		if !strings.Contains(xml, want) {
			t.Fatalf("junit xml missing %q:\n%s", want, xml)
		}
	}
}

const junitVariantSummaryFixture = `{
  "totalTestCount": 5,
  "passedTests": 2,
  "failedTests": 3,
  "skippedTests": 0,
  "expectedFailures": 0
}`

// Trimmed from a merged two-device result bundle run with retries enabled.
const junitVariantCasesFixture = `{
  "testNodes": [{
    "nodeType": "Test Plan",
    "name": "DemoKit",
    "result": "Failed",
    "children": [{
      "nodeType": "Unit test bundle",
      "name": "DemoKitTests",
      "result": "Failed",
      "children": [
        {"nodeType": "Test Case", "nodeIdentifier": "greetsByName()", "name": "greetsByName()", "result": "Passed", "children": [
          {"nodeType": "Device", "name": "iPhone 18 Pro", "result": "Passed"},
          {"nodeType": "Device", "name": "My Mac", "result": "Passed"}
        ]},
        {"nodeType": "Test Case", "nodeIdentifier": "failsOnlyOnPhone()", "name": "failsOnlyOnPhone()", "result": "Failed", "children": [
          {"nodeType": "Device", "name": "iPhone 18 Pro", "result": "Failed", "children": [
            {"nodeType": "Repetition", "name": "First Run", "result": "Failed", "children": [
              {"nodeType": "Failure Message", "name": "Expectation failed: fails on iPhone only"}
            ]},
            {"nodeType": "Repetition", "name": "Retry 1", "result": "Failed", "children": [
              {"nodeType": "Failure Message", "name": "Expectation failed: fails on iPhone only"}
            ]}
          ]},
          {"nodeType": "Device", "name": "My Mac", "result": "Passed"}
        ]},
        {"nodeType": "Test Case", "nodeIdentifier": "parameterised(value:)", "name": "parameterised(value:)", "result": "Failed", "children": [
          {"nodeType": "Device", "name": "iPhone 18 Pro", "result": "Failed", "children": [
            {"nodeType": "Arguments", "name": "1", "result": "Passed"},
            {"nodeType": "Arguments", "name": "2", "result": "Failed", "children": [
              {"nodeType": "Repetition", "name": "First Run", "result": "Failed", "children": [
                {"nodeType": "Failure Message", "name": "Expectation failed: value != 2"}
              ]}
            ]},
            {"nodeType": "Arguments", "name": "3", "result": "Passed"}
          ]},
          {"nodeType": "Device", "name": "My Mac", "result": "Failed", "children": [
            {"nodeType": "Arguments", "name": "1", "result": "Passed"},
            {"nodeType": "Arguments", "name": "2", "result": "Failed", "children": [
              {"nodeType": "Repetition", "name": "First Run", "result": "Failed", "children": [
                {"nodeType": "Failure Message", "name": "Expectation failed: value != 2"}
              ]}
            ]},
            {"nodeType": "Arguments", "name": "3", "result": "Passed"}
          ]}
        ]},
        {"nodeType": "Test Case", "nodeIdentifier": "withCaseMessage()", "name": "withCaseMessage()", "result": "Failed", "message": "case failed", "children": [
          {"nodeType": "Device", "name": "iPhone 18 Pro", "result": "Failed", "children": [
            {"nodeType": "Failure Message", "name": "Expectation failed: case on iPhone"}
          ]}
        ]},
        {"nodeType": "Test Suite", "name": "FlakyTests", "result": "Passed", "children": [
          {"nodeType": "Test Case", "nodeIdentifier": "FlakyTests/testPassesOnSecondAttempt()", "name": "testPassesOnSecondAttempt()", "result": "Passed", "children": [
            {"nodeType": "Device", "name": "iPhone 18 Pro", "result": "Passed", "children": [
              {"nodeType": "Repetition", "name": "First Run", "result": "Failed", "children": [
                {"nodeType": "Failure Message", "name": "failed - first attempt fails"}
              ]},
              {"nodeType": "Repetition", "name": "Retry 1", "result": "Passed"}
            ]}
          ]}
        ]}
      ]
    }]
  }]
}`

func TestXcodeTestJUnitAttributesFailuresToVariants(t *testing.T) {
	summary, err := localxcode.ParseTestResultSummary([]byte(junitVariantSummaryFixture))
	if err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	cases, err := localxcode.ParseTestResultCases([]byte(junitVariantCasesFixture))
	if err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	summary.Cases = cases
	reportFile := filepath.Join(t.TempDir(), "junit.xml")
	t.Cleanup(SetXCResultSummaryLoaderForTesting(func(context.Context, string) (*localxcode.TestSummary, error) {
		return summary, nil
	}))

	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestJUnitCommand(d)
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json", "--allow-failures"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Tests     int `xml:"tests,attr"`
		Failures  int `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("decode junit: %v\n%s", err, data)
	}
	if suite.Tests != 5 || suite.Failures != 3 {
		t.Fatalf("tests=%d failures=%d, want 5 and 3:\n%s", suite.Tests, suite.Failures, data)
	}
	want := map[string]string{
		"greetsByName()": "",
		"failsOnlyOnPhone()": "iPhone 18 Pro, First Run: Expectation failed: fails on iPhone only\n" +
			"iPhone 18 Pro, Retry 1: Expectation failed: fails on iPhone only",
		"parameterised(value:)": "iPhone 18 Pro, argument '2', First Run: Expectation failed: value != 2\n" +
			"My Mac, argument '2', First Run: Expectation failed: value != 2",
		"withCaseMessage()":           "iPhone 18 Pro: Expectation failed: case on iPhone",
		"testPassesOnSecondAttempt()": "",
	}
	for _, testCase := range suite.TestCases {
		wantMessage, ok := want[testCase.Name]
		if !ok {
			t.Fatalf("unexpected testcase %q:\n%s", testCase.Name, data)
		}
		gotMessage := ""
		if testCase.Failure != nil {
			gotMessage = testCase.Failure.Message
		}
		if (testCase.Failure != nil) != (wantMessage != "") || gotMessage != wantMessage {
			t.Errorf("testcase %q failure = %q, want %q", testCase.Name, gotMessage, wantMessage)
		}
	}
}

func TestXcodeTestJUnitFailsClosedWhenCaseEnrichmentFails(t *testing.T) {
	reportFile := filepath.Join(t.TempDir(), "junit.xml")
	t.Cleanup(SetXCResultSummaryLoaderForTesting(func(ctx context.Context, _ string) (*localxcode.TestSummary, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("loader context has no deadline")
		}
		return &localxcode.TestSummary{
			Total:  1,
			Passed: 1,
		}, errors.New("case enrichment unavailable")
	}))

	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestJUnitCommand(d)
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "case enrichment unavailable") {
		t.Fatalf("run error = %v, want case-enrichment failure", err)
	}
	if _, statErr := os.Stat(reportFile); !os.IsNotExist(statErr) {
		t.Fatalf("report stat error = %v, want no report after failed enrichment", statErr)
	}
}

func TestXcodeTestJUnitRequiresPathsBeforeLoading(t *testing.T) {
	called := false
	t.Cleanup(SetXCResultSummaryLoaderForTesting(func(context.Context, string) (*localxcode.TestSummary, error) {
		called = true
		return nil, nil
	}))
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestJUnitCommand(d)
	cmd.FlagSet.SetOutput(ioDiscard(t))
	if err := cmd.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--xcresult is required") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("loader ran before usage validation")
	}
}

func TestXcodeTestDestinationsParsesFixtureAndFiltersUnavailable(t *testing.T) {
	var calls int
	t.Cleanup(SetSimulatorListLoaderForTesting(func(ctx context.Context) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("loader context has no deadline")
		}
		calls++
		return []byte(`{
		  "runtimes": [
		    {"identifier":"com.apple.CoreSimulator.SimRuntime.iOS-27-1","version":"27.1"},
		    {"identifier":"com.apple.CoreSimulator.SimRuntime.iOS-27-2-iPhone19-4","version":"27.2"}
		  ],
		  "devices": {
		    "com.apple.CoreSimulator.SimRuntime.iOS-27-2-iPhone19-4": [
		      {"udid":"11111111-1111-1111-1111-111111111111","name":"iPhone 16","state":"Shutdown","isAvailable":true},
		      {"udid":"22222222-2222-2222-2222-222222222222","name":"iPhone 16 Unavailable","state":"Shutdown","isAvailable":false}
		    ],
		    "com.apple.CoreSimulator.SimRuntime.iOS-18-2": [
		      {"udid":"44444444-4444-4444-4444-444444444444","name":"iPhone 16 Legacy","state":"Shutdown","isAvailable":true}
		    ],
		    "com.apple.CoreSimulator.SimRuntime.xrOS-2-1": [
		      {"udid":"33333333-3333-3333-3333-333333333333","name":"Apple Vision Pro","state":"Shutdown","isAvailable":true}
		    ]
		  }
		}`), nil
	}))
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestDestinationsCommand(d)
	if err := cmd.Parse([]string{"--platform", "iOS", "--available-only", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout := captureTestDestinations(t, d, cmd)
	if calls != 1 {
		t.Fatalf("simctl calls = %d", calls)
	}
	if !strings.Contains(stdout, `platform=iOS Simulator,id=11111111-1111-1111-1111-111111111111`) {
		t.Fatalf("stdout = %s", stdout)
	}
	if strings.Contains(stdout, "22222222") || strings.Contains(stdout, "visionOS") || strings.Contains(stdout, "platform=macOS") {
		t.Fatalf("filtered destinations leaked: %s", stdout)
	}
	var result asc.XcodeTestDestinationsResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode destinations: %v\n%s", err, stdout)
	}
	wantVersions := map[string]string{
		"11111111-1111-1111-1111-111111111111": "27.2",
		"44444444-4444-4444-4444-444444444444": "18.2",
	}
	if len(result.Destinations) != len(wantVersions) {
		t.Fatalf("destinations = %+v, want %d", result.Destinations, len(wantVersions))
	}
	for _, destination := range result.Destinations {
		if want := wantVersions[destination.UDID]; destination.Runtime != want {
			t.Errorf("destination %s runtime = %q, want %q", destination.UDID, destination.Runtime, want)
		}
	}
}

func TestXcodeTestDestinationsMacDoesNotCallSimctl(t *testing.T) {
	previous := xcodeCommandGOOS
	xcodeCommandGOOS = "darwin"
	t.Cleanup(func() { xcodeCommandGOOS = previous })
	t.Cleanup(SetSimulatorListLoaderForTesting(func(context.Context) ([]byte, error) {
		t.Fatal("simctl should not run for --platform macOS")
		return nil, nil
	}))
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestDestinationsCommand(d)
	if err := cmd.Parse([]string{"--platform", "macOS", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout := captureTestDestinations(t, d, cmd)
	if !strings.Contains(stdout, `"destinationString":"platform=macOS"`) {
		t.Fatalf("stdout = %s", stdout)
	}
}

func TestXcodeTestDestinationsMacRefusesNonDarwinBeforeSimctl(t *testing.T) {
	previous := xcodeCommandGOOS
	xcodeCommandGOOS = "linux"
	t.Cleanup(func() { xcodeCommandGOOS = previous })
	t.Cleanup(SetSimulatorListLoaderForTesting(func(context.Context) ([]byte, error) {
		t.Fatal("simctl should not run on a non-macOS host")
		return nil, nil
	}))
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestDestinationsCommand(d)
	if err := cmd.Parse([]string{"--platform", "macOS", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("error = %v", err)
	}
}

func TestXcodeTestDestinationsRejectsUnknownPlatformBeforeSimctl(t *testing.T) {
	t.Cleanup(SetSimulatorListLoaderForTesting(func(context.Context) ([]byte, error) {
		t.Fatal("simctl should not run for a usage error")
		return nil, nil
	}))
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestDestinationsCommand(d)
	cmd.FlagSet.SetOutput(ioDiscard(t))
	if err := cmd.Parse([]string{"--platform", "android"}); err != nil {
		if !strings.Contains(err.Error(), "--platform must be one of") {
			t.Fatalf("parse error = %v", err)
		}
		return
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--platform must be one of") {
		t.Fatalf("error = %v", err)
	}
}

func TestXcodeTestJUnitRefusesNonDarwinBeforeLoaderWhenUnset(t *testing.T) {
	previous := xcodeCommandGOOS
	xcodeCommandGOOS = "linux"
	t.Cleanup(func() { xcodeCommandGOOS = previous })
	d := newXcodeTestDeps(nil)
	cmd := newXcodeTestJUnitCommand(d)
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", filepath.Join(t.TempDir(), "junit.xml")}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("error = %v", err)
	}
}

func captureTestDestinations(t *testing.T, d *shared.Deps, cmd interface {
	Run(context.Context) error
},
) string {
	t.Helper()
	runErr := cmd.Run(context.Background())
	stdout, _ := depsOutput(d)
	if runErr != nil {
		t.Fatalf("run: %v\nstdout: %s", runErr, stdout)
	}
	return stdout
}

func ioDiscard(t *testing.T) *os.File {
	t.Helper()
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	return file
}
