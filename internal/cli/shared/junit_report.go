package shared

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// JUnitTestCase represents a single test case in a JUnit report.
type JUnitTestCase struct {
	Name      string        // Test name (e.g., build-123)
	Classname string        // Test class/category (e.g., builds)
	Time      time.Duration // Test duration
	Skipped   bool          // Whether the test was skipped
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
	// Duration optionally reports wall time for the whole suite. Leaf case
	// durations often exclude setup, teardown, and repeated or multi-destination
	// work, so their sum can understate the run. When Duration exceeds that sum
	// it is used for the suite time attribute; zero keeps the summed behavior.
	Duration time.Duration
}

// Write writes the JUnit report to the specified file path.
func (r *JUnitReport) Write(path string) error {
	if path == "" {
		return fmt.Errorf("report file path is empty")
	}

	data, err := r.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal JUnit report: %w", err)
	}

	_, err = WriteStreamToFile(path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// Marshal marshals the JUnit report to XML.
// Note: xml.Encoder handles escaping automatically, so we don't pre-escape.
func (r *JUnitReport) Marshal() ([]byte, error) {
	name := r.Name
	if name == "" {
		name = "asc"
	}

	tests := len(r.Tests)
	failures := 0
	skipped := 0
	for _, tc := range r.Tests {
		if tc.Failure != "" {
			failures++
		}
		if tc.Skipped {
			skipped++
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
		Skipped:   skipped,
		Errors:    0,
		Time:      formatDuration(r.suiteDuration()),
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
	if err := enc.Close(); err != nil {
		return nil, err
	}

	return []byte(sb.String()), nil
}

// testCaseXML is the internal XML structure for test cases.
// Content is NOT pre-escaped - xml.Encoder handles it.
type testCaseXML struct {
	XMLName   xml.Name    `xml:"testcase"`
	Name      string      `xml:"name,attr"`
	Classname string      `xml:"classname,attr"`
	Time      string      `xml:"time,attr"`
	Skipped   *struct{}   `xml:"skipped,omitempty"`
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
	xml := testCaseXML{
		Name:      tc.Name,
		Classname: tc.Classname,
		Time:      formatDuration(tc.Time),
	}

	if tc.Skipped {
		xml.Skipped = &struct{}{}
	}

	if tc.Failure != "" {
		xml.Failure = &failureXML{
			Message: tc.Message,
			Type:    tc.Failure,
		}
	}

	if tc.SystemOut != "" {
		xml.SystemOut = tc.SystemOut
	}

	if tc.SystemErr != "" {
		xml.SystemErr = tc.SystemErr
	}

	return xml
}

// testsuiteXML is the internal XML structure for the test suite.
type testsuiteXML struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Failures  int           `xml:"failures,attr"`
	Skipped   int           `xml:"skipped,attr"`
	Errors    int           `xml:"errors,attr"`
	Time      string        `xml:"time,attr"`
	Timestamp string        `xml:"timestamp,attr,omitempty"`
	TestCases []testCaseXML `xml:"testcase"`
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

// suiteDuration reports the suite time attribute, never understating the run:
// the summed case durations unless an explicit aggregate exceeds them.
func (r *JUnitReport) suiteDuration() time.Duration {
	summed := totalDuration(r.Tests)
	if r.Duration > summed {
		return r.Duration
	}
	return summed
}

func totalDuration(tests []JUnitTestCase) time.Duration {
	var total time.Duration
	for _, tc := range tests {
		total += tc.Time
	}
	return total
}
