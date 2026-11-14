package devices

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDeviceBatchTSVAcceptsHeaderCommentsBlankRowsAndDefaultPlatform(t *testing.T) {
	path := writeDeviceBatchTestFile(t, strings.Join([]string{
		"# exported device list",
		"",
		"Device ID\tDevice Name\tDevice Platform",
		"00008110-001234567890001E\tPersonal iPhone\tios",
		"  # disabled device",
		"A1:B2:C3\tBuild Mac\t",
		"00006000-001A2B3C4D5E801E\tWork Mac\t Mac ",
		"",
	}, "\n"))

	records, err := readDeviceBatchTSV(path, "MAC_OS")
	if err != nil {
		t.Fatalf("readDeviceBatchTSV() error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d: %+v", len(records), records)
	}
	if records[0].Row != 4 || records[0].UDID != "00008110-001234567890001E" || records[0].Name != "Personal iPhone" || records[0].Platform != "IOS" {
		t.Fatalf("unexpected first record: %+v", records[0])
	}
	if records[1].Row != 6 || records[1].UDID != "A1:B2:C3" || records[1].Name != "Build Mac" || records[1].Platform != "MAC_OS" {
		t.Fatalf("unexpected second record: %+v", records[1])
	}
	if records[2].Row != 7 || records[2].Platform != "MAC_OS" {
		t.Fatalf("unexpected third record: %+v", records[2])
	}
}

func TestReadDeviceBatchTSVAcceptsHeaderlessRows(t *testing.T) {
	path := writeDeviceBatchTestFile(t, "UDID-1\tDevice One\nUDID-2\tDevice Two\tmac_os\n")

	records, err := readDeviceBatchTSV(path, "IOS")
	if err != nil {
		t.Fatalf("readDeviceBatchTSV() error: %v", err)
	}
	if len(records) != 2 || records[0].Platform != "IOS" || records[1].Platform != "MAC_OS" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func TestReadDeviceBatchTSVRejectsInvalidRowsBeforeMutation(t *testing.T) {
	tests := []struct {
		name            string
		contents        string
		defaultPlatform string
		want            string
	}{
		{name: "missing name", contents: "Device ID\tDevice Name\nUDID-1\t\n", defaultPlatform: "IOS", want: "line 2: device name is required"},
		{name: "missing platform", contents: "Device ID\tDevice Name\nUDID-1\tDevice\n", want: "line 2: device platform is required"},
		{name: "invalid platform", contents: "UDID-1\tDevice\tTV_OS\n", want: "line 1: --platform must be one of"},
		{name: "extra columns", contents: "UDID-1\tDevice\tIOS\textra\n", want: "line 1: expected 2 or 3 tab-separated columns"},
		{name: "bad header name", contents: "Device ID\tSerial\nUDID-1\tDevice\tIOS\n", want: "line 1: device header must start with Device ID and Device Name"},
		{name: "header with extra column", contents: "Device ID\tDevice Name\tDevice Platform\tExtra\n", want: "line 1: device header must contain 2 or 3 tab-separated columns"},
		{name: "bad platform header", contents: "Device ID\tDevice Name\tOS\n", want: "line 1: third header column must be Device Platform"},
		{name: "empty file", contents: "\n# comment\n", defaultPlatform: "IOS", want: "device file contains no records"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeDeviceBatchTestFile(t, test.contents)
			_, err := readDeviceBatchTSV(path, test.defaultPlatform)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestReadDeviceBatchTSVFromReaderRejectsInputBeyondLimit(t *testing.T) {
	contents := "UDID-1\tDevice One\tIOS\n#" + strings.Repeat("x", maxDeviceBatchFileSize)

	_, err := readDeviceBatchTSVFromReader(strings.NewReader(contents), "IOS")
	if err == nil || !strings.Contains(err.Error(), "device file exceeds") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}

func writeDeviceBatchTestFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devices.txt")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write device batch test file: %v", err)
	}
	return path
}
