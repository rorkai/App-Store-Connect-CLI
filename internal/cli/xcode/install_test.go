package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeCommandIncludesInstall(t *testing.T) {
	d := newXcodeTestDeps(nil)
	command := NewXcodeCommand(d)
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "install" {
			return
		}
	}
	t.Fatal("xcode command does not expose the install subcommand")
}

func TestXcodeInstallRequiresInputs(t *testing.T) {
	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureCommandOutput(t, d, func() error {
		runErr = command.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--ipa is required") {
		t.Fatalf("Exec() error/stderr = %v/%q, want required IPA usage error", runErr, stderr)
	}
}

func TestXcodeInstallPrintsPrivacySafeResult(t *testing.T) {
	previous := runInstall
	t.Cleanup(func() { runInstall = previous })
	runInstall = func(context.Context, localxcode.InstallOptions) (*asc.XcodeInstallResult, error) {
		return &asc.XcodeInstallResult{
			SchemaVersion: 1, Operation: "xcode.install", Success: true, Installed: true, Verified: true,
			IPA: asc.XcodeInstallArtifact{
				BundleID: "com.example.demo", Version: "1.2.3", BuildNumber: "45", SizeBytes: 4,
				SHA256: strings.Repeat("a", 64),
			},
			Device: &asc.XcodeInstallDevice{
				IdentifierSHA256: strings.Repeat("b", 64), Platform: "IOS",
				PairingState: "paired", ConnectionState: "connected",
			},
			DurationMS: 12,
		}, nil
	}
	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--device-id", "SELECTOR_CANARY", "--timeout", "5m", "--output", "json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	stdout, stderr := captureCommandOutput(t, d, func() error { return command.Exec(context.Background(), nil) })
	var result asc.XcodeInstallResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("JSON output error = %v; stdout=%q", err, stdout)
	}
	if result.Launch != nil || strings.Contains(stdout, `"launch"`) {
		t.Fatalf("default installation unexpectedly includes launch receipt: %q", stdout)
	}
	if !result.Success || !result.Verified || result.Device == nil || result.Device.IdentifierSHA256 == "SELECTOR_CANARY" {
		t.Fatalf("privacy-safe output = %#v", result)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr = %q", stderr)
	}
}

func TestXcodeInstallReportsOperationalFailureAfterResult(t *testing.T) {
	previous := runInstall
	t.Cleanup(func() { runInstall = previous })
	rawError := "materialize app member MEMBER_CANARY from /private/tmp/SOURCE_CANARY into /private/tmp/TEMP_CANARY for DEVICE_CANARY"
	runInstall = func(context.Context, localxcode.InstallOptions) (*asc.XcodeInstallResult, error) {
		return &asc.XcodeInstallResult{
			SchemaVersion: 1, Operation: "xcode.install", Installed: true,
			FailureStage: "verification", FailureCode: "verification_failed",
		}, errors.New(rawError)
	}
	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--device-id", "SELECTOR_CANARY", "--output", "json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	stdout, stderr := captureCommandOutput(t, d, func() error { return command.Exec(context.Background(), nil) })
	if !strings.Contains(stdout, `"installed":true`) || !strings.Contains(stderr, "Error: xcode install failed at verification (verification_failed)") {
		t.Fatalf("stdout/stderr = %q/%q", stdout, stderr)
	}
	if strings.Contains(stderr, rawError) || strings.Contains(stderr, "MEMBER_CANARY") || strings.Contains(stderr, "SOURCE_CANARY") || strings.Contains(stderr, "TEMP_CANARY") || strings.Contains(stderr, "DEVICE_CANARY") {
		t.Fatalf("operational diagnostic leaked raw error data: %q", stderr)
	}
}

func TestXcodeInstallRedactsRendererFailure(t *testing.T) {
	previousRunInstall := runInstall
	previousPrintInstallOutput := printInstallOutput
	t.Cleanup(func() {
		runInstall = previousRunInstall
		printInstallOutput = previousPrintInstallOutput
	})

	rawInstallErr := errors.New("install failed for /private/tmp/SOURCE_RENDER_CANARY using SECRET_RENDER_CANARY on UDID_RENDER_CANARY")
	renderErr := errors.New("renderer failed while opening /private/tmp/RENDER_CANARY")
	runInstall = func(context.Context, localxcode.InstallOptions) (*asc.XcodeInstallResult, error) {
		return &asc.XcodeInstallResult{
			SchemaVersion: 1, Operation: "xcode.install", FailureStage: "verification", FailureCode: "verification_failed",
		}, rawInstallErr
	}
	printInstallOutput = func(*shared.Deps, any, string, bool) error { return renderErr }

	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--device-id", "SELECTOR_CANARY", "--output", "json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = command.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || runErr.Error() != "xcode install output failed" {
		t.Fatalf("Exec() error = %v, want stable renderer failure", runErr)
	}
	if !errors.Is(runErr, rawInstallErr) || !errors.Is(runErr, renderErr) {
		t.Fatalf("Exec() error lost internal causes: %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout/stderr = %q/%q, want no output from failed renderer", stdout, stderr)
	}
	for _, canary := range []string{"SOURCE_RENDER_CANARY", "SECRET_RENDER_CANARY", "UDID_RENDER_CANARY", "RENDER_CANARY"} {
		if strings.Contains(runErr.Error(), canary) || strings.Contains(stdout, canary) || strings.Contains(stderr, canary) {
			t.Fatalf("renderer failure leaked %q: error=%q stdout=%q stderr=%q", canary, runErr, stdout, stderr)
		}
	}
}

func TestXcodeInstallRedactsNilResultOperationalFailure(t *testing.T) {
	previousRunInstall := runInstall
	t.Cleanup(func() { runInstall = previousRunInstall })

	rawInstallErr := errors.New("install failed at /private/tmp/PATH_NIL_CANARY with SECRET_NIL_CANARY for UDID_NIL_CANARY")
	runInstall = func(context.Context, localxcode.InstallOptions) (*asc.XcodeInstallResult, error) {
		return nil, rawInstallErr
	}

	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--device-id", "SELECTOR_CANARY", "--output", "json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = command.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || runErr.Error() != "xcode install failed" {
		t.Fatalf("Exec() error = %v, want stable nil-result failure", runErr)
	}
	if !errors.Is(runErr, rawInstallErr) {
		t.Fatalf("Exec() error lost internal cause: %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout/stderr = %q/%q, want no output from nil-result failure", stdout, stderr)
	}
	for _, canary := range []string{"PATH_NIL_CANARY", "SECRET_NIL_CANARY", "UDID_NIL_CANARY"} {
		if strings.Contains(runErr.Error(), canary) || strings.Contains(stdout, canary) || strings.Contains(stderr, canary) {
			t.Fatalf("nil-result failure leaked %q: error=%q stdout=%q stderr=%q", canary, runErr, stdout, stderr)
		}
	}
}

func TestXcodeInstallRejectsInvalidTimeoutAsUsage(t *testing.T) {
	d := newXcodeTestDeps(nil)
	command := newXcodeInstallCommand(d)
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--device-id", "SELECTOR_CANARY", "--timeout", "1s"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureCommandOutput(t, d, func() error {
		runErr = command.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "between") {
		t.Fatalf("Exec() error/stderr = %v/%q, want timeout usage error", runErr, stderr)
	}
}
