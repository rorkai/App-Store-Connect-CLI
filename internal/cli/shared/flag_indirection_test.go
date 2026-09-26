package shared

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveFlagValueIndirectionPassesPlainValuesThrough(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "plain", value: "hello"},
		{name: "empty", value: ""},
		{name: "at handle", value: "@username"},
		{name: "email", value: "user@example.com"},
		{name: "env without colon", value: "@envNAME"},
		{name: "file without colon", value: "@fileNAME"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveFlagValueIndirection("channel", test.value)
			if err != nil {
				t.Fatalf("ResolveFlagValueIndirection() error = %v", err)
			}
			if got != test.value {
				t.Fatalf("ResolveFlagValueIndirection() = %q, want %q", got, test.value)
			}
		})
	}
}

func TestResolveFlagValueIndirectionUnescapesDoubleAt(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "@@env:HOME", want: "@env:HOME"},
		{value: "@@file:notes.txt", want: "@file:notes.txt"},
		{value: "@@", want: "@"},
		{value: "@@@", want: "@@"},
		{value: "@@literal", want: "@literal"},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ResolveFlagValueIndirection("whats-new", test.value)
			if err != nil {
				t.Fatalf("ResolveFlagValueIndirection() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ResolveFlagValueIndirection() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveFlagValueIndirectionReadsEnvironment(t *testing.T) {
	t.Setenv("ASC_TEST_INDIRECT_VALUE", "  release notes with spaces \n")
	got, err := ResolveFlagValueIndirection("whats-new", "@env:ASC_TEST_INDIRECT_VALUE")
	if err != nil {
		t.Fatalf("ResolveFlagValueIndirection() error = %v", err)
	}
	if got != "  release notes with spaces \n" {
		t.Fatalf("ResolveFlagValueIndirection() = %q, want environment value verbatim", got)
	}
}

func TestResolveFlagValueIndirectionEnvironmentErrors(t *testing.T) {
	t.Setenv("ASC_TEST_INDIRECT_EMPTY", "")
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "unset", value: "@env:ASC_TEST_INDIRECT_UNSET", wantErr: "--secret: environment variable ASC_TEST_INDIRECT_UNSET is not set"},
		{name: "empty", value: "@env:ASC_TEST_INDIRECT_EMPTY", wantErr: "--secret: environment variable ASC_TEST_INDIRECT_EMPTY is empty"},
		{name: "missing name", value: "@env:", wantErr: "--secret: @env: requires an environment variable name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveFlagValueIndirection("secret", test.value)
			if err == nil {
				t.Fatalf("ResolveFlagValueIndirection() = %q, want error", got)
			}
			assertIndirectionUsageError(t, err, "--secret", test.wantErr)
		})
	}
}

func TestResolveFlagValueIndirectionReadsFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "single trailing newline trimmed", content: "hunter2\n", want: "hunter2"},
		{name: "crlf trimmed", content: "hunter2\r\n", want: "hunter2"},
		{name: "only one trailing newline trimmed", content: "line one\nline two\n\n", want: "line one\nline two\n"},
		{name: "no trailing newline", content: "hunter2", want: "hunter2"},
		{name: "interior whitespace preserved", content: "  padded  \n", want: "  padded  "},
		{name: "lone trailing carriage return preserved", content: "hunter2\r", want: "hunter2\r"},
		{name: "only the crlf pair is trimmed", content: "hunter2\r\r\n", want: "hunter2\r"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(test.name, " ", "-")+".txt")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			got, err := ResolveFlagValueIndirection("whats-new", "@file:"+path)
			if err != nil {
				t.Fatalf("ResolveFlagValueIndirection() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ResolveFlagValueIndirection() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveFlagValueIndirectionFileErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	newlineOnly := filepath.Join(dir, "newline.txt")
	if err := os.WriteFile(newlineOnly, []byte("\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	oversize := filepath.Join(dir, "oversize.txt")
	if err := os.WriteFile(oversize, make([]byte, MaxIndirectFileValueSize+1), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "missing path", value: "@file:", wantErr: "--whats-new: @file: requires a file path"},
		{name: "missing file", value: "@file:" + missing, wantErr: "--whats-new: cannot read file " + missing + ": file does not exist"},
		{name: "directory", value: "@file:" + dir, wantErr: "--whats-new: cannot read file " + dir + ": not a regular file"},
		{name: "empty", value: "@file:" + empty, wantErr: "--whats-new: file " + empty + " is empty"},
		{name: "newline only", value: "@file:" + newlineOnly, wantErr: "--whats-new: file " + newlineOnly + " is empty"},
		{name: "oversize", value: "@file:" + oversize, wantErr: "--whats-new: file " + oversize + " exceeds the 1 MiB limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveFlagValueIndirection("whats-new", test.value)
			if err == nil {
				t.Fatalf("ResolveFlagValueIndirection() = %q, want error", got)
			}
			assertIndirectionUsageError(t, err, "--whats-new", test.wantErr)
		})
	}
}

func TestResolveFlagValueIndirectionRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hunter2\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	got, err := ResolveFlagValueIndirection("secret", "@file:"+link)
	if err == nil {
		t.Fatalf("ResolveFlagValueIndirection() = %q, want symlink error", got)
	}
	assertIndirectionUsageError(t, err, "--secret", "--secret: cannot read file "+link+":")
}

func assertIndirectionUsageError(t *testing.T, err error, parameter, wantMessage string) {
	t.Helper()
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), wantMessage)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error %v should classify as a usage error", err)
	}
	if kind := ClassifyUsageError(err); kind != UsageErrorInvalidValue {
		t.Fatalf("ClassifyUsageError() = %v, want %v", kind, UsageErrorInvalidValue)
	}
	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		t.Fatalf("error %v should carry a diagnostic", err)
	}
	if diagnostic.Code != DiagnosticInvalidInput || diagnostic.Parameter != parameter {
		t.Fatalf("diagnostic = %+v, want %s on %s", diagnostic, DiagnosticInvalidInput, parameter)
	}
}

func TestHasFlagValueIndirection(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "@env:NAME", want: true},
		{value: "@file:/tmp/x", want: true},
		{value: "@@literal", want: true},
		{value: "@env:", want: true},
		{value: "@handle", want: false},
		{value: "plain", want: false},
		{value: "", want: false},
	}
	for _, test := range tests {
		if got := HasFlagValueIndirection(test.value); got != test.want {
			t.Fatalf("HasFlagValueIndirection(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}
