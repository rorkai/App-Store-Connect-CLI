package iap

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompletedReplacementCommandPreservesLiteralPOSIXArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell argv verification is not available on Windows")
	}
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not installed")
	}

	screenshotID := "shot $(printf INJECTED) `printf BACKTICK`"
	iapID := `iap "quoted" value's`
	fileName := "replacement image's $(printf FILE).png"
	deleteCommand, createCommand := completedIAPReviewScreenshotReplacementCommands(screenshotID, iapID, fileName)
	relationships := json.RawMessage(`{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap \"quoted\" value's"}}}`)
	remediationErr := completedIAPReviewScreenshotReplacementError(screenshotID, relationships, fileName, true)
	if remediationErr == nil || !strings.Contains(remediationErr.Error(), deleteCommand) || !strings.Contains(remediationErr.Error(), createCommand) {
		t.Fatalf("replacement remediation = %v, want safe commands %q and %q", remediationErr, deleteCommand, createCommand)
	}
	commands := []string{deleteCommand, createCommand}
	want := [][]string{
		{"asc", "iap", "review-screenshots", "delete", "--screenshot-id", screenshotID, "--confirm"},
		{"asc", "iap", "review-screenshots", "create", "--iap-id", iapID, "--file", fileName},
	}
	if len(commands) != len(want) {
		t.Fatalf("replacement command has %d shell commands, want %d: %#v", len(commands), len(want), commands)
	}
	for index, command := range commands {
		got := zshReplacementCommandArguments(t, command)
		if !reflect.DeepEqual(got, want[index]) {
			t.Fatalf("replacement command %d argv = %#v, want %#v; command=%q", index, got, want[index], command)
		}
	}
}

func TestCompletedReplacementDiagnosticDoesNotExecuteCommandsWhenPasted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell execution is not available on Windows")
	}
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not installed")
	}

	screenshotID := "shot $(printf INJECTED)"
	fileName := "replacement image's $(printf FILE).png"
	relationships := json.RawMessage(`{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap \"quoted\" value's"}}}`)
	diagnosticErr := completedIAPReviewScreenshotReplacementError(screenshotID, relationships, fileName, true)
	if diagnosticErr == nil {
		t.Fatal("replacement diagnostic is nil")
	}

	marker := filepath.Join(t.TempDir(), "asc-invoked")
	fakeASC := filepath.Join(filepath.Dir(marker), "asc")
	if err := os.WriteFile(fakeASC, []byte("#!/bin/sh\nprintf invoked > \"$MARKER\"\n"), 0o700); err != nil {
		t.Fatalf("write fake asc: %v", err)
	}

	cmd := exec.Command("zsh", "-f", "-c", diagnosticErr.Error())
	cmd.Env = make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "PATH=") || strings.HasPrefix(value, "MARKER=") {
			continue
		}
		cmd.Env = append(cmd.Env, value)
	}
	cmd.Env = append(cmd.Env, "PATH="+filepath.Dir(marker)+string(os.PathListSeparator)+os.Getenv("PATH"), "MARKER="+marker)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("pasting the full diagnostic invoked asc; marker stat error = %v", err)
	}
}

func TestCompletedReplacementCommandUsesSafePlaceholdersForUnrenderableValues(t *testing.T) {
	screenshotID := "shot\x1b[31m\n"
	iapID := "iap\xff"
	fileName := "file\u202e.png"
	relationships := json.RawMessage(`{"inAppPurchaseV2":{"data":{"id":"` + iapID + `"}}}`)
	deleteCommand, createCommand := completedIAPReviewScreenshotReplacementCommands(screenshotID, iapID, fileName)
	command := deleteCommand + " " + createCommand
	if strings.Contains(command, "INJECTED") || strings.ContainsAny(command, "\x00\x1b\n\r\u0085\u2028\u2029\u202e\u2066") {
		t.Fatalf("replacement command contains unsafe provider/local data: %q", command)
	}
	wantDelete := "asc iap review-screenshots delete --screenshot-id SCREENSHOT_ID --confirm"
	wantCreate := "asc iap review-screenshots create --iap-id IAP_ID --file FILE_PATH"
	if deleteCommand != wantDelete || createCommand != wantCreate {
		t.Fatalf("replacement commands = %q and %q, want %q and %q", deleteCommand, createCommand, wantDelete, wantCreate)
	}

	err := completedIAPReviewScreenshotReplacementError(screenshotID, relationships, fileName, true)
	if err == nil || !utf8.ValidString(err.Error()) || strings.ContainsAny(err.Error(), "\x00\x1b\n\r\u2028\u2029\u202e") {
		t.Fatalf("replacement error contains unsafe data: %v", err)
	}
	if strings.Contains(err.Error(), "�") || !strings.Contains(err.Error(), "--iap-id IAP_ID") {
		t.Fatalf("replacement error = %q, want the safe IAP_ID placeholder for invalid relationship bytes", err.Error())
	}
}

func TestShellQuotedRemediationArgumentUsesPlaceholderForUnrenderableValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "NUL", value: "a\x00b"},
		{name: "DEL", value: "a\x7fb"},
		{name: "C1 separator", value: "a\u0085b"},
		{name: "line separator", value: "a\u2028b"},
		{name: "paragraph separator", value: "a\u2029b"},
		{name: "bidi isolate", value: "a\u2066b"},
		{name: "invalid UTF-8", value: "a\xffb"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shellQuotedRemediationArgument(test.value, "VALUE"); got != "VALUE" {
				t.Fatalf("shellQuotedRemediationArgument(%q) = %q, want VALUE", test.value, got)
			}
		})
	}
}

func zshReplacementCommandArguments(t *testing.T, command string) []string {
	t.Helper()
	script := "set -- " + command + "; printf '%s\\0' \"$@\""
	output, err := exec.Command("zsh", "-f", "-c", script).Output()
	if err != nil {
		t.Fatalf("zsh replacement command failed: %v; command=%q", err, command)
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
}
