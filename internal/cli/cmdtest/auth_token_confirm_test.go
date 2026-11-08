package cmdtest

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// isolateAuthTokenProfile keeps the root --profile override from leaking
// between tests because the selected profile is process-global state.
func isolateAuthTokenProfile(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	setCmdtestHome(t)

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})
}

func TestAuthTokenMissingConfirmPrintsExactReinvocation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// wantRerun holds the POSIX rendering; posixOnly cases assert quoting
		// that shared.ShellQuote only emits off Windows.
		wantRerun string
		posixOnly bool
	}{
		{
			name:      "bare invocation",
			args:      []string{"auth", "token"},
			wantRerun: "asc auth token --confirm",
		},
		{
			name:      "preserves root profile flag",
			args:      []string{"--profile", "release", "auth", "token"},
			wantRerun: "asc --profile release auth token --confirm",
		},
		{
			name:      "preserves root strict auth flag",
			args:      []string{"--strict-auth", "auth", "token"},
			wantRerun: "asc --strict-auth auth token --confirm",
		},
		{
			name:      "preserves command flags",
			args:      []string{"--profile", "release", "auth", "token", "--name", "MyKey", "--output", "json", "--pretty"},
			wantRerun: "asc --profile release auth token --name MyKey --output json --pretty --confirm",
		},
		{
			name:      "rewrites explicit confirm false",
			args:      []string{"auth", "token", "--confirm=false", "--output", "json"},
			wantRerun: "asc auth token --output json --confirm",
		},
		{
			name:      "shell-quotes values so the printed command cannot expand",
			args:      []string{"--profile", "$(whoami) key", "auth", "token", "--name", "it's mine"},
			wantRerun: `asc --profile '$(whoami) key' auth token --name 'it'\''s mine' --confirm`,
			posixOnly: true,
		},
	}

	const wantReason = "Error: --confirm is required because `asc auth token` prints a live bearer token to stdout, " +
		"where it can leak into shell history, logs, or CI output\n"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.posixOnly && runtime.GOOS == "windows" {
				t.Skip("PowerShell quoting is covered by the shared package unit tests")
			}
			isolateAuthTokenProfile(t)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
			}
			if kind := shared.ClassifyUsageError(runErr); kind != shared.UsageErrorMissingRequired {
				t.Fatalf("usage error kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			// The reason line and the exact re-invocation must lead stderr, ahead
			// of the usage page ffcli renders for flag.ErrHelp.
			wantBlock := wantReason + "Re-run: " + test.wantRerun + "\n"
			if !strings.HasPrefix(stderr, wantBlock) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, wantBlock)
			}
		})
	}
}

// TestAuthTokenMissingConfirmOmitsUnprintableReinvocation covers the values
// that have no exact, terminal-safe rendering: the gate still explains itself,
// but no re-run line is printed rather than one that would run with a
// different profile than the caller supplied.
func TestAuthTokenMissingConfirmOmitsUnprintableReinvocation(t *testing.T) {
	isolateAuthTokenProfile(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"--profile", "a\x1b[31mb", "auth", "token"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
	}
	wantReason := "Error: --confirm is required because `asc auth token` prints a live bearer token to stdout, " +
		"where it can leak into shell history, logs, or CI output\n"
	if !strings.HasPrefix(stderr, wantReason) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, wantReason)
	}
	if strings.Contains(stderr, "Re-run:") {
		t.Fatalf("expected no re-run suggestion for an unprintable profile, got %q", stderr)
	}
}

func TestAuthTokenAcceptsPrivateKeyFlattenedBySecretStore(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	begin, body, end := lines[0], strings.Join(lines[1:len(lines)-1], ""), lines[len(lines)-1]

	tests := []struct {
		name    string
		value   string
		encoded string
	}{
		{name: "everything on one line", value: begin + body + end},
		{name: "spaces for newlines", value: strings.Join(lines, " ")},
		{name: "literal newlines around one line", value: begin + `\n` + body + `\n` + end},
		{name: "body without markers", value: body},
		{name: "base64", encoded: base64.StdEncoding.EncodeToString(data)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateAuthTokenProfile(t)
			t.Setenv("ASC_KEY_ID", "ENVKEY")
			t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
			t.Setenv("ASC_PRIVATE_KEY_PATH", "")
			t.Setenv("ASC_PRIVATE_KEY_B64", test.encoded)
			t.Setenv("ASC_PRIVATE_KEY", test.value)

			root := RootCommand("1.2.3")
			if err := root.Parse([]string{"auth", "token", "--confirm"}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			var err error
			stdout, stderr := captureOutput(t, func() {
				// Inline API signing must work without a writable temporary directory.
				t.Setenv("TMPDIR", keyPath)
				t.Setenv("TMP", keyPath)
				err = root.Run(context.Background())
			})
			if err != nil {
				t.Fatalf("auth token error: %v (stderr %q)", err, stderr)
			}
			if got := strings.Count(strings.TrimSpace(stdout), "."); got != 2 {
				t.Fatalf("expected a signed JWT on stdout, got %d dots", got)
			}
		})
	}
}
