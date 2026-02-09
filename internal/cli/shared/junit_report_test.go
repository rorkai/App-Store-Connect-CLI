package shared

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJUnitReport_Write(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{
				Name:      "build-123",
				Classname: "builds",
				Time:      1500 * time.Millisecond,
			},
		},
		Timestamp: time.Now(),
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "junit.xml")

	err := report.Write(path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var result struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Errors   int      `xml:"errors,attr"`
		Time     string   `xml:"time,attr"`
		Cases    []struct {
			Name      string `xml:"name,attr"`
			Classname string `xml:"classname,attr"`
		} `xml:"testcase"`
	}

	err = xml.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("XML unmarshal error = %v", err)
	}

	if result.Tests != 1 {
		t.Errorf("expected 1 test, got %d", result.Tests)
	}
	if result.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", result.Failures)
	}
	if len(result.Cases) != 1 || result.Cases[0].Name != "build-123" {
		t.Errorf("unexpected test case: %+v", result.Cases)
	}
}

func TestJUnitReport_WriteWithFailure(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{
				Name:      "build-456",
				Classname: "builds",
				Failure:   "BUILD_FAILED",
				Message:   "Invalid build state",
				Time:      500 * time.Millisecond,
			},
		},
		Timestamp: time.Now(),
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "junit.xml")

	err := report.Write(path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var result struct {
		Failures int `xml:"failures,attr"`
	}

	err = xml.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("XML unmarshal error = %v", err)
	}

	if result.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", result.Failures)
	}

	if !contains(data, []byte("<failure")) {
		t.Error("expected <failure> element in XML")
	}
}

func TestJUnitReport_EscapeSpecialChars(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{
				Name:      "test-with-special-chars",
				Classname: "builds",
				Message:   "Error with <xml> & 'quotes'",
				Time:      0,
			},
		},
		Timestamp: time.Now(),
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "junit.xml")

	err := report.Write(path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if contains(data, []byte("<xml>")) {
		t.Error("expected escaped XML, got raw <xml>")
	}
	if contains(data, []byte("& quotes")) {
		t.Error("expected escaped ampersand, got raw &")
	}
}

func TestCIReportFlags(t *testing.T) {
	if ReportFormat() != "" {
		t.Errorf("ReportFormat() = %q, want empty", ReportFormat())
	}
	if ReportFile() != "" {
		t.Errorf("ReportFile() = %q, want empty", ReportFile())
	}

	SetReportFormat("junit")
	SetReportFile("/tmp/report.xml")

	if ReportFormat() != "junit" {
		t.Errorf("ReportFormat() = %q, want 'junit'", ReportFormat())
	}
	if ReportFile() != "/tmp/report.xml" {
		t.Errorf("ReportFile() = %q, want '/tmp/report.xml'", ReportFile())
	}
}

func contains(b []byte, sub []byte) bool {
	for i := 0; i <= len(b)-len(sub); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return true
		}
	}
	return false
}
