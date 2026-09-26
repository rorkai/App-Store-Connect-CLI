package asc

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxXcodeTestHumanCases    = 10000
	maxXcodeTestHumanFailures = 100
	maxXcodeTestHumanMessage  = 4096
)

// XcodeTestResult is the stable output receipt for a local Xcode test action.
// It deliberately lives in the output package so JSON and human renderers use
// the same exported camelCase contract as other computed CLI results.
type XcodeTestResult struct {
	Workspace        string            `json:"workspace,omitempty"`
	Project          string            `json:"project,omitempty"`
	Scheme           string            `json:"scheme,omitempty"`
	Action           string            `json:"action"`
	Configuration    string            `json:"configuration,omitempty"`
	Destinations     []string          `json:"destinations,omitempty"`
	TestPlan         string            `json:"testPlan,omitempty"`
	XctestrunPath    string            `json:"xctestrunPath,omitempty"`
	DerivedDataPath  string            `json:"derivedDataPath,omitempty"`
	ResultBundlePath string            `json:"resultBundlePath,omitempty"`
	Tests            *XcodeTestSummary `json:"tests,omitempty"`
	Clean            bool              `json:"clean"`
	NoCodeSigning    bool              `json:"noCodeSigning"`
	Success          bool              `json:"success"`
	DurationMs       int64             `json:"durationMs"`
	ExitStatus       *int              `json:"exitStatus,omitempty"`
}

// XcodeJUnitResult is the receipt for converting an existing xcresult to JUnit.
type XcodeJUnitResult struct {
	XCResult   string `json:"xcresult"`
	ReportFile string `json:"reportFile"`
	Tests      int    `json:"tests"`
	Passed     int    `json:"passed"`
	Failures   int    `json:"failures"`
	Skipped    int    `json:"skipped"`
}

// XcodeTestDestination is one destination string for asc xcode test --destination.
type XcodeTestDestination struct {
	Name              string `json:"name"`
	UDID              string `json:"udid,omitempty"`
	Runtime           string `json:"runtime"`
	State             string `json:"state"`
	DestinationString string `json:"destinationString"`
	Available         bool   `json:"available"`
}

// XcodeTestDestinationsResult lists local destinations without mutating Simulator state.
type XcodeTestDestinationsResult struct {
	Destinations []XcodeTestDestination `json:"destinations"`
}

// XcodeTestSummary is the structured test aggregate in an Xcode result
// receipt.
type XcodeTestSummary struct {
	Total            int                `json:"total"`
	Passed           int                `json:"passed"`
	Failed           int                `json:"failed"`
	Skipped          int                `json:"skipped"`
	ExpectedFailures int                `json:"expectedFailures"`
	DurationMs       int64              `json:"durationMs"`
	Cases            []XcodeTestCase    `json:"cases,omitempty"`
	Failures         []XcodeTestFailure `json:"failures,omitempty"`
}

// XcodeTestCase is one parsed Xcode test case.
type XcodeTestCase struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	Classname  string `json:"className,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Message    string `json:"message,omitempty"`
}

// XcodeTestFailure is bounded structured failure detail from an Xcode result.
type XcodeTestFailure struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message,omitempty"`
}

func xcodeTestResultRows(result *XcodeTestResult) ([]string, [][]string) {
	if result == nil {
		result = &XcodeTestResult{}
	}
	rows := make([][]string, 0, 22)
	if result.Workspace != "" {
		rows = append(rows, []string{"workspace", result.Workspace})
	}
	if result.Project != "" {
		rows = append(rows, []string{"project", result.Project})
	}
	if result.Scheme != "" {
		rows = append(rows, []string{"scheme", result.Scheme})
	}
	rows = append(rows, []string{"action", result.Action})
	if result.Configuration != "" {
		rows = append(rows, []string{"configuration", result.Configuration})
	}
	if len(result.Destinations) > 0 {
		rows = append(rows, []string{"destinations", joinOutputValues(result.Destinations)})
	}
	if result.TestPlan != "" {
		rows = append(rows, []string{"test_plan", result.TestPlan})
	}
	if result.XctestrunPath != "" {
		rows = append(rows, []string{"xctestrun_path", result.XctestrunPath})
	}
	if result.DerivedDataPath != "" {
		rows = append(rows, []string{"derived_data_path", result.DerivedDataPath})
	}
	if result.ResultBundlePath != "" {
		rows = append(rows, []string{"result_bundle_path", result.ResultBundlePath})
	}
	rows = append(
		rows,
		[]string{"clean", formatBool(result.Clean)},
		[]string{"no_code_signing", formatBool(result.NoCodeSigning)},
	)
	if result.Tests != nil {
		rows = append(
			rows,
			[]string{"tests_total", formatInt(result.Tests.Total)},
			[]string{"tests_passed", formatInt(result.Tests.Passed)},
			[]string{"tests_failed", formatInt(result.Tests.Failed)},
			[]string{"tests_skipped", formatInt(result.Tests.Skipped)},
			[]string{"tests_expected_failures", formatInt(result.Tests.ExpectedFailures)},
			[]string{"tests_duration_ms", formatInt64(result.Tests.DurationMs)},
		)
		for index, testCase := range result.Tests.Cases {
			if index >= maxXcodeTestHumanCases {
				break
			}
			rows = append(rows, []string{"test_case", formatXcodeTestCase(testCase)})
		}
		for index, failure := range result.Tests.Failures {
			if index >= maxXcodeTestHumanFailures {
				break
			}
			rows = append(rows, []string{"test_failure", formatXcodeTestFailure(failure)})
		}
	}
	rows = append(
		rows,
		[]string{"success", formatBool(result.Success)},
		[]string{"duration_ms", formatInt64(result.DurationMs)},
	)
	if result.ExitStatus != nil {
		rows = append(rows, []string{"exit_status", formatInt(*result.ExitStatus)})
	}
	return []string{"field", "value"}, rows
}

func joinOutputValues(values []string) string {
	return strings.Join(values, "\n")
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatXcodeTestCase(testCase XcodeTestCase) string {
	identifier := strings.TrimSpace(testCase.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(testCase.Name)
	}
	if identifier == "" {
		identifier = "unnamed-test"
	}
	status := strings.TrimSpace(testCase.Status)
	if status == "" {
		status = "unknown"
	}
	return fmt.Sprintf("%s [%s]", identifier, status)
}

func formatXcodeTestFailure(failure XcodeTestFailure) string {
	identifier := strings.TrimSpace(failure.Identifier)
	if identifier == "" {
		identifier = "unknown-test"
	}
	message := strings.TrimSpace(failure.Message)
	if len(message) > maxXcodeTestHumanMessage {
		message = truncateUTF8ToBytes(message, maxXcodeTestHumanMessage)
	}
	if message == "" {
		return identifier
	}
	return identifier + ": " + message
}

func xcodeJUnitResultRows(result *XcodeJUnitResult) ([]string, [][]string) {
	headers := []string{"XCResult", "Report", "Tests", "Passed", "Failures", "Skipped"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{
		result.XCResult,
		result.ReportFile,
		strconv.Itoa(result.Tests),
		strconv.Itoa(result.Passed),
		strconv.Itoa(result.Failures),
		strconv.Itoa(result.Skipped),
	}}
}

func xcodeTestDestinationsRows(result *XcodeTestDestinationsResult) ([]string, [][]string) {
	headers := []string{"Name", "UDID", "Runtime", "State", "Destination", "Available"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(result.Destinations))
	for _, destination := range result.Destinations {
		rows = append(rows, []string{
			destination.Name,
			destination.UDID,
			destination.Runtime,
			destination.State,
			destination.DestinationString,
			strconv.FormatBool(destination.Available),
		})
	}
	return headers, rows
}

func truncateUTF8ToBytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
