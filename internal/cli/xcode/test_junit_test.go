package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
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

	cmd := XcodeTestJUnitCommand()
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json"}); err != nil {
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

	cmd := XcodeTestJUnitCommand()
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
	cmd := XcodeTestJUnitCommand()
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
		  "devices": {
		    "com.apple.CoreSimulator.SimRuntime.iOS-18-2": [
		      {"udid":"11111111-1111-1111-1111-111111111111","name":"iPhone 16","state":"Shutdown","isAvailable":true},
		      {"udid":"22222222-2222-2222-2222-222222222222","name":"iPhone 16 Unavailable","state":"Shutdown","isAvailable":false}
		    ],
		    "com.apple.CoreSimulator.SimRuntime.xrOS-2-1": [
		      {"udid":"33333333-3333-3333-3333-333333333333","name":"Apple Vision Pro","state":"Shutdown","isAvailable":true}
		    ]
		  }
		}`), nil
	}))
	cmd := XcodeTestDestinationsCommand()
	if err := cmd.Parse([]string{"--platform", "iOS", "--available-only", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout := captureTestDestinations(t, cmd)
	if calls != 1 {
		t.Fatalf("simctl calls = %d", calls)
	}
	if !strings.Contains(stdout, `platform=iOS Simulator,id=11111111-1111-1111-1111-111111111111`) {
		t.Fatalf("stdout = %s", stdout)
	}
	if strings.Contains(stdout, "22222222") || strings.Contains(stdout, "visionOS") || strings.Contains(stdout, "platform=macOS") {
		t.Fatalf("filtered destinations leaked: %s", stdout)
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
	cmd := XcodeTestDestinationsCommand()
	if err := cmd.Parse([]string{"--platform", "macOS", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout := captureTestDestinations(t, cmd)
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
	cmd := XcodeTestDestinationsCommand()
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
	cmd := XcodeTestDestinationsCommand()
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
	cmd := XcodeTestJUnitCommand()
	if err := cmd.Parse([]string{"--xcresult", "Test.xcresult", "--report-file", filepath.Join(t.TempDir(), "junit.xml")}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("error = %v", err)
	}
}

func captureTestDestinations(t *testing.T, cmd interface {
	Run(context.Context) error
},
) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = original })
	runErr := cmd.Run(context.Background())
	writer.Close()
	os.Stdout = original
	buf := make([]byte, 1<<20)
	n, _ := reader.Read(buf)
	if runErr != nil {
		t.Fatalf("run: %v\nstdout: %s", runErr, buf[:n])
	}
	return string(buf[:n])
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
