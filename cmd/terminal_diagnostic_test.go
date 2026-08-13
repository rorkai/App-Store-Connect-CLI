package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRun_UnknownCommandSanitizesUnicodeTerminalControlsAndInvalidUTF8(t *testing.T) {
	resetReportFlags(t)

	hostile := "evil\u009b31m\u202e\u2028" + string([]byte{0xff}) + "tail"
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{hostile}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertSafeTerminalDiagnostic(t, stderr, "Error: unknown command `asc evil31m \ufffdtail`\n")
}

func TestRun_UnknownFlagSanitizesUnicodeTerminalControlsAndInvalidUTF8(t *testing.T) {
	resetReportFlags(t)

	hostile := "--evil\u009b31m\u202e\u2028" + string([]byte{0xff}) + "tail"
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "list", hostile}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertSafeTerminalDiagnostic(t, stderr, "Error: unknown flag `--evil31m \ufffdtail` for `asc builds list`\n")
}

func assertSafeTerminalDiagnostic(t *testing.T, stderr, wantLine string) {
	t.Helper()

	if !strings.HasPrefix(stderr, wantLine) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, wantLine)
	}
	if !utf8.ValidString(stderr) {
		t.Fatalf("stderr contains invalid UTF-8: %q", stderr)
	}
	for _, forbidden := range []rune{'\u009b', '\u202e', '\u2028'} {
		if strings.ContainsRune(stderr, forbidden) {
			t.Fatalf("stderr contains interpreted terminal character %U: %q", forbidden, stderr)
		}
	}
}
