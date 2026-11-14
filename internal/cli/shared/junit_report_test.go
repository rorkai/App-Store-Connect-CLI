package shared

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
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

	tmpDir := t.ArtifactDir()
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

	tmpDir := t.ArtifactDir()
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

	if !strings.Contains(string(data), "<failure") {
		t.Error("expected <failure> element in XML")
	}
}

func TestJUnitReport_WriteWithSkippedTest(t *testing.T) {
	report := JUnitReport{
		Tests:     []JUnitTestCase{{Name: "skipped-test", Classname: "tests", Skipped: true}},
		Timestamp: time.Now(),
	}
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), "<skipped></skipped>") {
		t.Fatalf("JUnit output = %s, want skipped element", data)
	}
	if !strings.Contains(string(data), `skipped="1"`) {
		t.Fatalf("JUnit output = %s, want skipped suite count", data)
	}
}

func TestJUnitReport_EscapeSpecialChars(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{
				Name:      "test-with-chars",
				Classname: "builds",
				Failure:   "Error",
				Message:   "Error with <xml> & 'quotes'",
				Time:      0,
			},
		},
		Timestamp: time.Now(),
	}

	tmpDir := t.ArtifactDir()
	path := filepath.Join(tmpDir, "junit.xml")

	err := report.Write(path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Should contain properly escaped XML in failure message
	if !strings.Contains(string(data), "&lt;xml&gt;") {
		t.Errorf("expected &lt;xml&gt; in output, got: %s", data)
	}
	if !strings.Contains(string(data), "&amp;") {
		t.Errorf("expected &amp; in output, got: %s", data)
	}
	// Go xml.Encoder uses &#39; for single quotes
	if !strings.Contains(string(data), "&#39;") {
		t.Errorf("expected &#39; in output, got: %s", data)
	}

	// Should NOT contain raw special chars in content
	if strings.Contains(string(data), "<xml>") {
		t.Error("expected escaped <xml>, got raw <xml>")
	}
}

func TestValidateReportFlags(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		file      string
		wantError bool
	}{
		{"empty format is valid", "", "", false},
		{"report file without format is error", "", "/tmp/report.xml", true},
		{"junit without file is error", "junit", "", true},
		{"junit with file is valid", "junit", "/tmp/report.xml", false},
		{"invalid format returns error", "nope", "", true},
		{"invalid format with file is still error", "nope", "/tmp/report.xml", true},
		{"another invalid format", "xml", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetReportFormat(tt.format)
			SetReportFile(tt.file)
			err := ValidateReportFlags()
			if tt.wantError && err == nil {
				t.Errorf("ValidateReportFlags() = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateReportFlags() = %v, want nil", err)
			}
		})
	}
}

func TestJUnitReport_WriteCreatesRestrictedPermissions(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{Name: "test", Classname: "suite", Time: time.Second},
		},
		Timestamp: time.Now(),
	}

	path := filepath.Join(t.ArtifactDir(), "junit.xml")
	if err := report.Write(path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("file mode = %o, want %o", got, want)
	}
}

func TestJUnitReport_WriteRefusesOverwrite(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{Name: "test", Classname: "suite", Time: time.Second},
		},
		Timestamp: time.Now(),
	}

	path := filepath.Join(t.ArtifactDir(), "junit.xml")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := report.Write(path)
	if err == nil {
		t.Fatal("expected error when writing to existing file, got nil")
	}
}

func TestJUnitReport_WriteRefusesSymlink(t *testing.T) {
	report := JUnitReport{
		Tests: []JUnitTestCase{
			{Name: "test", Classname: "suite", Time: time.Second},
		},
		Timestamp: time.Now(),
	}

	tmpDir := t.ArtifactDir()
	target := filepath.Join(tmpDir, "target.xml")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	link := filepath.Join(tmpDir, "report.xml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	err := report.Write(link)
	if err == nil {
		t.Fatal("expected error when writing to symlink path, got nil")
	}
}

func TestJUnitReport_SuiteDurationPrefersAggregateOverCaseSum(t *testing.T) {
	report := &JUnitReport{
		Name:      "asc xcode test",
		Timestamp: time.Now(),
		Duration:  time.Second,
		Tests: []JUnitTestCase{
			{Name: "testA", Time: 100 * time.Millisecond},
			{Name: "testB", Time: 100 * time.Millisecond},
		},
	}
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `time="1.000"`) {
		t.Fatalf("JUnit output = %s, want aggregate suite duration", data)
	}
}

func TestJUnitReport_SuiteDurationFallsBackToCaseSum(t *testing.T) {
	report := &JUnitReport{
		Name:      "asc xcode test",
		Timestamp: time.Now(),
		Tests: []JUnitTestCase{
			{Name: "testA", Time: 100 * time.Millisecond},
			{Name: "testB", Time: 150 * time.Millisecond},
		},
	}
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `time="0.250"`) {
		t.Fatalf("JUnit output = %s, want summed case duration", data)
	}
}
