package apps

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	cliweb "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
)

func captureAppsCreateOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutReader)
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrReader)
		stderrCh <- buf.String()
	}()

	closeWriters := func() {
		if stdoutWriter != nil {
			_ = stdoutWriter.Close()
			stdoutWriter = nil
		}
		if stderrWriter != nil {
			_ = stderrWriter.Close()
			stderrWriter = nil
		}
	}

	defer func() {
		closeWriters()
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	fn()

	closeWriters()
	os.Stdout = origStdout
	os.Stderr = origStderr

	return <-stdoutCh, <-stderrCh
}

func TestAppsCreateCommandHelpMentionsDeprecationAndCanonicalPath(t *testing.T) {
	cmd := AppsCreateCommand()

	if !strings.Contains(cmd.ShortHelp, "[deprecated]") {
		t.Fatalf("expected deprecated short help, got %q", cmd.ShortHelp)
	}
	if !strings.Contains(cmd.LongHelp, "asc web apps create") {
		t.Fatalf("expected canonical command in long help, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "removed after one release cycle") {
		t.Fatalf("expected removal window in long help, got %q", cmd.LongHelp)
	}
}

func TestAppsCreateCommandPreservesLegacyFlagSurface(t *testing.T) {
	cmd := AppsCreateCommand()

	if cmd.FlagSet.Lookup("password") == nil {
		t.Fatal("expected legacy password flag to remain on deprecated shim")
	}
	if cmd.FlagSet.Lookup("version") == nil {
		t.Fatal("expected deprecated shim to accept --version")
	}
	if cmd.FlagSet.Lookup("company-name") == nil {
		t.Fatal("expected deprecated shim to accept --company-name")
	}
}

func TestAppsCreateCommandPrintsWarningAndForwardsToWebRunner(t *testing.T) {
	origRunAppsCreateShim := runAppsCreateShimFn
	t.Cleanup(func() {
		runAppsCreateShimFn = origRunAppsCreateShim
	})

	expectedErr := errors.New("stop after forwarding")
	var received cliweb.AppsCreateRunOptions
	runAppsCreateShimFn = func(ctx context.Context, opts cliweb.AppsCreateRunOptions) error {
		received = opts
		return expectedErr
	}
	passwordFlag := "--" + "password"

	cmd := AppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--primary-locale", "en-GB",
		"--platform", "IOS",
		"--version", "2.3.4",
		"--company-name", "Example Co",
		"--apple-id", "user@example.com",
		passwordFlag, "fixture-password",
		"--two-factor-code", "123456",
		"--two-factor-code-command", "printf 123456",
		"--auto-rename=false",
		"--output", "json",
		"--pretty",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	_, stderr := captureAppsCreateOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, expectedErr) {
		t.Fatalf("expected forwarded error %v, got %v", expectedErr, runErr)
	}
	if !strings.Contains(runErr.Error(), "apps create:") {
		t.Fatalf("expected deprecated shim error prefix, got %v", runErr)
	}
	if !strings.Contains(stderr, appsCreateDeprecationWarning) {
		t.Fatalf("expected deprecation warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, appsCreateMigrationGuidance) {
		t.Fatalf("expected migration guidance in stderr, got %q", stderr)
	}
	if received.Name != "My App" {
		t.Fatalf("expected forwarded name, got %q", received.Name)
	}
	if received.BundleID != "com.example.app" {
		t.Fatalf("expected forwarded bundle id, got %q", received.BundleID)
	}
	if received.SKU != "SKU123" {
		t.Fatalf("expected forwarded sku, got %q", received.SKU)
	}
	if received.PrimaryLocale != "en-GB" {
		t.Fatalf("expected forwarded locale, got %q", received.PrimaryLocale)
	}
	if received.Platform != "IOS" {
		t.Fatalf("expected forwarded platform, got %q", received.Platform)
	}
	if received.Version != "2.3.4" {
		t.Fatalf("expected forwarded version, got %q", received.Version)
	}
	if received.CompanyName != "Example Co" {
		t.Fatalf("expected forwarded company name, got %q", received.CompanyName)
	}
	if received.AppleID != "user@example.com" {
		t.Fatalf("expected forwarded apple id, got %q", received.AppleID)
	}
	if received.Password != "fixture-password" {
		t.Fatalf("expected forwarded password, got %q", received.Password)
	}
	if received.TwoFactorCode != "123456" {
		t.Fatalf("expected forwarded 2fa code, got %q", received.TwoFactorCode)
	}
	if received.TwoFactorCodeCommand != "printf 123456" {
		t.Fatalf("expected forwarded 2fa command, got %q", received.TwoFactorCodeCommand)
	}
	if received.AutoRename {
		t.Fatal("expected deprecated shim to forward --auto-rename=false")
	}
	if received.Output != "json" {
		t.Fatalf("expected forwarded output, got %q", received.Output)
	}
	if !received.Pretty {
		t.Fatal("expected forwarded pretty flag")
	}
	if !received.PromptForAppleIDWithPassword {
		t.Fatal("expected deprecated shim to preserve password-without-apple-id prompting behavior")
	}
	if !received.DisableBundleIDPreflight {
		t.Fatal("expected deprecated shim to preserve old bundle-id preflight contract")
	}
}
