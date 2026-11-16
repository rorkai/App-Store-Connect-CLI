package signing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"howett.net/plist"
)

func TestSigningResignOperationalErrorHasStablePublicMessageAndCause(t *testing.T) {
	const secret = "/private/tmp/secret-generated-entitlement-selector"
	cause := fmt.Errorf("open %s: permission denied", secret)
	err := wrapSigningResignOperationalError(signingResignStagePreparation, signingResignCodeGeneratedEntitlements, cause)
	if got := err.Error(); got != "signing resign failed during preparation (generated-entitlements)" {
		t.Fatalf("public error = %q, want stable stage/code", got)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("public error leaked private cause: %q", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is() did not retain the original cause")
	}
}

func TestSigningResignCommandBoundaryRedactsInjectedOperationalCause(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	const secret = "/private/tmp/secret-cert-selector"
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	executeSigningResignFn = func(context.Context, *shared.Deps, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{}, fmt.Errorf("certificate read failed at %s: secret-password", secret)
	}
	d, _, _ := newSigningTestDeps()
	command := newSigningResignCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--ipa", "input.ipa",
		"--output", "output.ipa",
		"--identity", "identity.p12",
		"--profiles-manifest", "profiles.json",
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("SigningResignCommand().Exec() returned nil")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("command error leaked injected cause: %q", err)
	}
	if !strings.Contains(err.Error(), "signing resign failed during preparation") {
		t.Fatalf("command error = %q, want stable public operation error", err)
	}
}

func TestSigningResignCommandMalformedPlatformMetadataIsOperationalExitOne(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	originalExecute := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = originalExecute })
	executeSigningResignFn = executeSigningResignImplementation

	for _, test := range []struct {
		name   string
		info   map[string]any
		secret string
	}{
		{
			name: "DTPlatformName array",
			info: map[string]any{
				"DTPlatformName":             []any{"iphoneos"},
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			},
		},
		{
			name: "DTPlatformName scalar number",
			info: map[string]any{
				"DTPlatformName":             42,
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			},
		},
		{
			name: "DTPlatformName empty",
			info: map[string]any{
				"DTPlatformName":             "",
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			},
		},
		{
			name: "DTPlatformName control canary",
			info: map[string]any{
				"DTPlatformName":             "iphoneos\nprivate-platform-marker",
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			},
			secret: "private-platform-marker",
		},
		{
			name: "supported platforms mixed array",
			info: map[string]any{
				"DTPlatformName":             "iphoneos",
				"CFBundleSupportedPlatforms": []any{"iPhoneOS", 42},
			},
		},
		{
			name: "supported platforms scalar",
			info: map[string]any{
				"DTPlatformName":             "iphoneos",
				"CFBundleSupportedPlatforms": "iPhoneOS",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			inputPath := filepath.Join(temporary, "input.ipa")
			writeSigningResignIPAWithInfo(t, inputPath, test.info)
			outputPath := filepath.Join(temporary, "private-output-parent", "output.ipa")
			manifestPath := filepath.Join(temporary, "private-profiles-manifest.json")
			d, _, _ := newSigningTestDeps()
			command := newSigningResignCommand(d)
			if err := command.FlagSet.Parse([]string{
				"--ipa", inputPath,
				"--output", outputPath,
				"--identity", filepath.Join(temporary, "private-identity.p12"),
				"--profiles-manifest", manifestPath,
			}); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), nil)
			if err == nil || errors.Is(err, flag.ErrHelp) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want operational exit 1", err)
			}
			if !strings.Contains(err.Error(), "signing resign failed during preparation (filesystem)") {
				t.Fatalf("SigningResignCommand().Exec() error = %q, want closed preparation stage/code", err)
			}
			if test.secret != "" && strings.Contains(err.Error(), test.secret) {
				t.Fatalf("SigningResignCommand().Exec() leaked metadata canary %q: %q", test.secret, err)
			}
			if strings.Contains(err.Error(), inputPath) || strings.Contains(err.Error(), outputPath) || strings.Contains(err.Error(), manifestPath) {
				t.Fatalf("SigningResignCommand().Exec() leaked a private path: %q", err)
			}
			if _, statErr := os.Stat(filepath.Dir(outputPath)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("malformed metadata created output parent: stat error = %v", statErr)
			}
		})
	}
}

func TestSigningResignOutputPreflightUsesArtifactPublicationStage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	originalExecute := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = originalExecute })
	executeSigningResignFn = executeSigningResignImplementation
	temporary := t.TempDir()
	inputPath := filepath.Join(temporary, "input.ipa")
	writeSigningResignMinimalIPA(t, inputPath)
	outputPath := filepath.Join(temporary, "output.ipa")
	if err := os.WriteFile(outputPath, []byte("existing artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, _, _ := newSigningTestDeps()
	command := newSigningResignCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--ipa", inputPath,
		"--output", outputPath,
		"--identity", filepath.Join(temporary, "private-identity.p12"),
		"--profiles-manifest", filepath.Join(temporary, "private-profiles.json"),
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("SigningResignCommand().Exec() error = %v, want operational artifact failure", err)
	}
	if !strings.Contains(err.Error(), "signing resign failed during artifact verification (artifact-publish)") {
		t.Fatalf("SigningResignCommand().Exec() error = %q, want artifact publication stage/code", err)
	}
	if strings.Contains(err.Error(), inputPath) || strings.Contains(err.Error(), outputPath) {
		t.Fatalf("SigningResignCommand().Exec() leaked a private path: %q", err)
	}
}

func writeSigningResignIPAWithInfo(t *testing.T, pathValue string, info map[string]any) {
	t.Helper()
	info["CFBundleIdentifier"] = "com.example.app"
	info["CFBundleExecutable"] = "App"
	data, err := plist.Marshal(info, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	archive := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: data},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
	})
	if err := os.WriteFile(pathValue, archive, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSigningResignEnvironmentJoinsEarlyCleanupFailureWithoutLeakingCause(t *testing.T) {
	const (
		primarySecret = "/private/tmp/secret-keychain-primary"
		cleanupSecret = "/private/tmp/secret-keychain-cleanup"
	)
	primary := fmt.Errorf("keychain lookup failed at %s", primarySecret)
	cleanup := fmt.Errorf("remove temp directory failed at %s", cleanupSecret)
	temporary := t.TempDir()
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	attempted := false
	signingResignPlatformDepsFn = func(*shared.Deps) signingRunDeps {
		return minimalSigningResignDeps(
			temporary,
			func(context.Context) ([]string, error) { return nil, primary },
			func(string) error {
				attempted = true
				return cleanup
			},
		)
	}
	err := runSigningResignEnvironment(context.Background(), testSigningDeps(), testSigningResignIdentity(t), func(context.Context, string) error {
		t.Fatal("operation ran after early environment failure")
		return nil
	})
	if err == nil {
		t.Fatal("runSigningResignEnvironment(testSigningDeps()) returned nil")
	}
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("error = %v, want both primary and cleanup causes", err)
	}
	if !errors.Is(err, ErrSigningResignCleanupFailed) {
		t.Fatalf("error = %v, want cleanup-failed signal", err)
	}
	if strings.Contains(err.Error(), primarySecret) || strings.Contains(err.Error(), cleanupSecret) {
		t.Fatalf("environment error leaked private cause: %q", err)
	}
	if !attempted {
		t.Fatal("early failure did not attempt temporary-directory cleanup")
	}
}

func TestSigningResignEnvironmentReturnsCleanupOnlyFailureAsStableError(t *testing.T) {
	const cleanupSecret = "/private/tmp/secret-cleanup-only"
	cleanup := fmt.Errorf("remove temp directory failed at %s", cleanupSecret)
	temporary := t.TempDir()
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	signingResignPlatformDepsFn = func(*shared.Deps) signingRunDeps {
		return minimalSigningResignDeps(
			temporary,
			func(context.Context) ([]string, error) { return nil, nil },
			func(string) error { return cleanup },
		)
	}
	err := runSigningResignEnvironment(context.Background(), testSigningDeps(), testSigningResignIdentity(t), func(context.Context, string) error {
		return nil
	})
	if err == nil {
		t.Fatal("runSigningResignEnvironment(testSigningDeps()) returned nil")
	}
	if !errors.Is(err, cleanup) || !errors.Is(err, ErrSigningResignCleanupFailed) {
		t.Fatalf("error = %v, want cleanup cause and signal", err)
	}
	if strings.Contains(err.Error(), cleanupSecret) {
		t.Fatalf("cleanup error leaked private cause: %q", err)
	}
	if !strings.Contains(err.Error(), "signing resign failed during cleanup") {
		t.Fatalf("cleanup error = %q, want stable cleanup stage", err)
	}
}

func TestSigningResignEnvironmentSuccessfulEarlyCleanupRemainsOperational(t *testing.T) {
	primary := errors.New("keychain unavailable")
	temporary := t.TempDir()
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	removed := false
	signingResignPlatformDepsFn = func(*shared.Deps) signingRunDeps {
		return minimalSigningResignDeps(
			temporary,
			func(context.Context) ([]string, error) { return nil, primary },
			func(path string) error {
				removed = true
				return os.RemoveAll(path)
			},
		)
	}
	err := runSigningResignEnvironment(context.Background(), testSigningDeps(), testSigningResignIdentity(t), func(context.Context, string) error {
		t.Fatal("operation ran after early environment failure")
		return nil
	})
	if err == nil || !errors.Is(err, primary) {
		t.Fatalf("runSigningResignEnvironment(testSigningDeps()) error = %v, want primary failure", err)
	}
	if !removed {
		t.Fatal("early failure did not attempt temporary-directory cleanup")
	}
}

func TestSigningResignEnvironmentPreservesUnlockAndCleanupFailures(t *testing.T) {
	const (
		unlockSecret = "/private/tmp/secret-unlock-cleanup"
		tempSecret   = "/private/tmp/secret-unlock-temp"
	)
	unlockErr := fmt.Errorf("release lock failed at %s", unlockSecret)
	tempErr := fmt.Errorf("remove temp directory failed at %s", tempSecret)
	temporary := t.TempDir()
	deps := minimalSigningResignDeps(
		temporary,
		func(context.Context) ([]string, error) { return nil, nil },
		func(string) error { return tempErr },
	)
	deps.AcquireLock = func(context.Context) (func() error, error) {
		return func() error { return unlockErr }, nil
	}
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	signingResignPlatformDepsFn = func(*shared.Deps) signingRunDeps { return deps }

	err := runSigningResignEnvironment(context.Background(), testSigningDeps(), testSigningResignIdentity(t), func(context.Context, string) error {
		return nil
	})
	if err == nil {
		t.Fatal("runSigningResignEnvironment(testSigningDeps()) returned nil")
	}
	for _, cause := range []error{unlockErr, tempErr} {
		if !errors.Is(err, cause) {
			t.Fatalf("error = %v, want cause %v", err, cause)
		}
	}
	if !errors.Is(err, ErrSigningResignCleanupFailed) {
		t.Fatalf("error = %v, want cleanup-failed signal", err)
	}
	if strings.Contains(err.Error(), unlockSecret) || strings.Contains(err.Error(), tempSecret) {
		t.Fatalf("error leaked private cleanup cause: %q", err)
	}
	if !strings.Contains(err.Error(), "signing resign failed during cleanup (cleanup)") {
		t.Fatalf("error = %q, want stable cleanup stage", err)
	}
}

func TestSigningResignEnvironmentWriteJournalFailureAttemptsBothCleanupPaths(t *testing.T) {
	const (
		primarySecret = "/private/tmp/secret-journal-primary"
		journalSecret = "/private/tmp/secret-journal-cleanup"
		tempSecret    = "/private/tmp/secret-temp-cleanup"
	)
	primary := fmt.Errorf("journal write failed at %s", primarySecret)
	journalCleanup := fmt.Errorf("journal cleanup failed at %s", journalSecret)
	tempCleanup := fmt.Errorf("temporary cleanup failed at %s", tempSecret)
	temporary := t.TempDir()
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	journalAttempted, tempAttempted := false, false
	deps := minimalSigningResignDeps(
		temporary,
		func(context.Context) ([]string, error) { return nil, nil },
		func(string) error {
			tempAttempted = true
			return tempCleanup
		},
	)
	deps.WriteJournal = func(signingRunJournal, bool) error { return primary }
	deps.RemoveJournal = func() error {
		journalAttempted = true
		return journalCleanup
	}
	signingResignPlatformDepsFn = func(*shared.Deps) signingRunDeps { return deps }

	err := runSigningResignEnvironment(context.Background(), testSigningDeps(), testSigningResignIdentity(t), func(context.Context, string) error {
		t.Fatal("operation ran after journal failure")
		return nil
	})
	if err == nil {
		t.Fatal("runSigningResignEnvironment(testSigningDeps()) returned nil")
	}
	for _, cause := range []error{primary, journalCleanup, tempCleanup} {
		if !errors.Is(err, cause) {
			t.Fatalf("error = %v, want cause %v", err, cause)
		}
	}
	if !errors.Is(err, ErrSigningResignCleanupFailed) {
		t.Fatalf("error = %v, want cleanup-failed signal", err)
	}
	if !journalAttempted || !tempAttempted {
		t.Fatalf("journal cleanup attempted=%t, temp cleanup attempted=%t; want both", journalAttempted, tempAttempted)
	}
	for _, secret := range []string{primarySecret, journalSecret, tempSecret} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked private cause %q: %v", secret, err)
		}
	}
}

func minimalSigningResignDeps(temporary string, searchList func(context.Context) ([]string, error), removeTemp func(string) error) signingRunDeps {
	return signingRunDeps{
		GOOS:        "darwin",
		RandomBytes: func(size int) ([]byte, error) { return make([]byte, size), nil },
		TempDir: func() (string, error) {
			path := filepath.Join(temporary, "session")
			return path, os.Mkdir(path, 0o700)
		},
		RemoveTempDir:             removeTemp,
		AcquireLock:               func(context.Context) (func() error, error) { return func() error { return nil }, nil },
		Recover:                   func(context.Context) error { return nil },
		WriteJournal:              func(signingRunJournal, bool) error { return nil },
		RemoveJournal:             func() error { return nil },
		KeychainSearchList:        searchList,
		CreateKeychain:            func(context.Context, string, []byte) error { return nil },
		ImportIdentity:            func(context.Context, string, []byte, []byte, []byte, string) error { return nil },
		SetKeychainSearchList:     func(context.Context, []string) error { return nil },
		RemoveKeychainSearchEntry: func(context.Context, string) error { return nil },
		DeleteKeychain:            func(context.Context, string) error { return nil },
	}
}

func TestSigningResignOperationalErrorTreeRejectsPlainWrappers(t *testing.T) {
	typed := wrapSigningResignOperationalError(signingResignStageVerification, signingResignCodeVerification, errors.New("private cause"))
	plain := fmt.Errorf("verify re-signed IPA after repack: %w", typed)
	if signingResignOperationalErrorTree(plain) {
		t.Fatal("plain wrapper around a typed cause must not count as public-safe")
	}
	public := wrapSigningResignOperationalError(signingResignStageVerification, signingResignCodeVerification, plain)
	if public.Error() != "signing resign failed during verification (verification)" {
		t.Fatalf("public error = %q, want the closed stage/code text only", public.Error())
	}
	if !errors.Is(public, typed) {
		t.Fatal("detailed cause chain must remain reachable through Unwrap")
	}
	join := errors.Join(
		typed,
		wrapSigningResignOperationalError(signingResignStageCleanup, signingResignCodeCleanup, errors.New("cleanup cause")),
	)
	if !signingResignOperationalErrorTree(join) {
		t.Fatal("aggregates of typed errors remain public-safe")
	}
}

func TestSigningResignStageCleanupAfterPublicationIsAmbiguous(t *testing.T) {
	const stageSecret = "/private/tmp/asc-signing-resign.secret-stage"
	cleanup := fmt.Errorf("remove private re-signing directory failed at %s", stageSecret)
	err := signingResignStageCleanupFailure(true, cleanup)
	if err == nil {
		t.Fatal("signingResignStageCleanupFailure() returned nil")
	}
	if !errors.Is(err, ErrSigningResignPublicationAmbiguous) {
		t.Fatalf("error = %v, want publication ambiguity once the IPA is published", err)
	}
	if !errors.Is(err, ErrSigningResignCleanupFailed) || !errors.Is(err, cleanup) {
		t.Fatalf("error = %v, want cleanup signal and cause", err)
	}
	if strings.Contains(err.Error(), stageSecret) {
		t.Fatalf("cleanup error leaked private cause: %q", err)
	}
	if got := err.Error(); got != "signing resign failed during cleanup (cleanup)" {
		t.Fatalf("public error = %q, want the closed cleanup stage/code text", got)
	}
	// The pipeline joins this failure onto a nil result error on the success
	// path, and the outermost operational wrapper must keep that aggregate
	// unchanged instead of relabelling it with the last executed stage.
	joined := wrapSigningResignOperationalError(
		signingResignStagePreparation,
		signingResignCodeFilesystem,
		errors.Join(nil, err),
	)
	if !errors.Is(joined, ErrSigningResignPublicationAmbiguous) {
		t.Fatalf("joined error = %v, want the publication ambiguity preserved", joined)
	}
	if got := joined.Error(); got != "signing resign failed during cleanup (cleanup)" {
		t.Fatalf("joined public error = %q, want the closed cleanup stage/code text", got)
	}
}

func TestSigningResignStageCleanupBeforePublicationIsNotAmbiguous(t *testing.T) {
	cleanup := errors.New("remove private re-signing directory failed")
	err := signingResignStageCleanupFailure(false, cleanup)
	if err == nil {
		t.Fatal("signingResignStageCleanupFailure() returned nil")
	}
	if errors.Is(err, ErrSigningResignPublicationAmbiguous) {
		t.Fatalf("error = %v, want no publication ambiguity before the IPA is published", err)
	}
	if !errors.Is(err, ErrSigningResignCleanupFailed) || !errors.Is(err, cleanup) {
		t.Fatalf("error = %v, want cleanup signal and cause", err)
	}
	if got := err.Error(); got != "signing resign failed during cleanup (cleanup)" {
		t.Fatalf("public error = %q, want the closed cleanup stage/code text", got)
	}
}

func TestSigningResignStageCleanupSuccessReturnsNil(t *testing.T) {
	if err := signingResignStageCleanupFailure(true, nil); err != nil {
		t.Fatalf("signingResignStageCleanupFailure(true, nil) = %v, want nil", err)
	}
}
