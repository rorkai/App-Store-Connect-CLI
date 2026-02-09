package shared

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"
)

// JUnitTestCase represents a single test case in a JUnit report.
type JUnitTestCase struct {
	Name      string        // Test name (e.g., build-123)
	Classname string        // Test class/category (e.g., builds)
	Time      time.Duration // Test duration
	Failure   string        // Failure type (empty if passed)
	Message   string        // Failure message
	SystemOut string        // Standard output
	SystemErr string        // Standard error
}

// JUnitReport represents a JUnit XML test report.
type JUnitReport struct {
	Tests     []JUnitTestCase // Test cases in this report
	Timestamp time.Time       // Report generation time
	Name      string          // Test suite name (default: "asc")
}

// Write writes the JUnit report to the specified file path.
func (r *JUnitReport) Write(path string) error {
	if path == "" {
		return fmt.Errorf("report file path is empty")
	}

	data, err := r.MarshalXML()
	if err != nil {
		return fmt.Errorf("failed to marshal JUnit report: %w", err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// MarshalXML marshals the JUnit report to XML.
func (r *JUnitReport) MarshalXML() ([]byte, error) {
	name := r.Name
	if name == "" {
		name = "asc"
	}

	tests := len(r.Tests)
	failures := 0
	for _, tc := range r.Tests {
		if tc.Failure != "" {
			failures++
		}
	}

	var testCases []testCaseXML
	for _, tc := range r.Tests {
		testCases = append(testCases, tc.toXML())
	}

	ts := testsuiteXML{
		Name:      name,
		Tests:     tests,
		Failures:  failures,
		Errors:    0,
		Time:      formatDuration(totalDuration(r.Tests)),
		Timestamp: r.Timestamp.Format(time.RFC3339),
		TestCases: testCases,
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")

	enc := xml.NewEncoder(&sb)
	err := enc.Encode(ts)
	if err != nil {
		return nil, err
	}

	return []byte(sb.String()), nil
}

// testCaseXML is the internal XML structure for test cases.
type testCaseXML struct {
	XMLName   xml.Name    `xml:"testcase"`
	Name      string      `xml:"name,attr"`
	Classname string      `xml:"classname,attr"`
	Time      string      `xml:"time,attr"`
	Failure   *failureXML `xml:"failure,omitempty"`
	SystemOut string      `xml:"system-out,omitempty"`
	SystemErr string      `xml:"system-err,omitempty"`
}

// failureXML is the internal XML structure for failures.
type failureXML struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
}

func (tc JUnitTestCase) toXML() testCaseXML {
	x := testCaseXML{
		Name:      tc.Name,
		Classname: tc.Classname,
		Time:      formatDuration(tc.Time),
	}

	if tc.Failure != "" {
		x.Failure = &failureXML{
			Message: tc.Message,
			Type:    tc.Failure,
		}
	}

	if tc.SystemOut != "" {
		x.SystemOut = tc.SystemOut
	}

	if tc.SystemErr != "" {
		x.SystemErr = tc.SystemErr
	}

	return x
}

// testsuiteXML is the internal XML structure for the test suite.
type testsuiteXML struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Failures  int           `xml:"failures,attr"`
	Errors    int           `xml:"errors,attr"`
	Time      string        `xml:"time,attr"`
	Timestamp string        `xml:"timestamp,attr,omitempty"`
	TestCases []testCaseXML `xml:"testcase"`
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

func totalDuration(tests []JUnitTestCase) time.Duration {
	var total time.Duration
	for _, tc := range tests {
		total += tc.Time
	}
	return total
}
