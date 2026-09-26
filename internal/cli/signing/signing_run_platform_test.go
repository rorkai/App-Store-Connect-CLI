//go:build darwin

package signing

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type signingRunPartialWriter struct {
	written int
	err     error
}

func (w signingRunPartialWriter) Write([]byte) (int, error) {
	return w.written, w.err
}

func TestParseKeychainSearchList(t *testing.T) {
	got, err := parseKeychainSearchList([]byte("    \"/Users/me/Library/Keychains/login.keychain-db\"\n    \"/private/tmp/path with spaces.keychain-db\"\n"))
	if err != nil {
		t.Fatalf("parseKeychainSearchList() error: %v", err)
	}
	want := []string{"/Users/me/Library/Keychains/login.keychain-db", "/private/tmp/path with spaces.keychain-db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestRunSigningRunChildPreservesExitCode(t *testing.T) {
	err := runSigningRunChild(context.Background(), []string{"/bin/sh", "-c", "exit 42"})
	if code, ok := shared.ProcessExitCode(err); !ok || code != 42 {
		t.Fatalf("error = %v, want exit code 42", err)
	}
}

func TestRunSigningRunChildRejectsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := runSigningRunChild(ctx, []string{"/bin/sh", "-c", "sleep 30"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled before launch", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("pre-canceled child did not return promptly")
	}
}

func TestRunSigningRunChildDoesNotSignalProcessGroupAfterWait(t *testing.T) {
	previous := signingRunKillProcessGroupFn
	var signals []syscall.Signal
	signingRunKillProcessGroupFn = func(_ int, signal syscall.Signal) error {
		if signal == 0 {
			return syscall.ESRCH
		}
		signals = append(signals, signal)
		return nil
	}
	t.Cleanup(func() { signingRunKillProcessGroupFn = previous })

	if err := runSigningRunChild(context.Background(), []string{"/bin/sh", "-c", "exit 0"}); err != nil {
		t.Fatalf("runSigningRunChild() error: %v", err)
	}
	if len(signals) != 0 {
		t.Fatalf("signals after child wait = %v, want none", signals)
	}
}

func TestRunSigningRunChildSignalsProcessGroupBeforeWaitOnCancellation(t *testing.T) {
	previous := signingRunKillProcessGroupFn
	var signals []syscall.Signal
	signingRunKillProcessGroupFn = func(pid int, signal syscall.Signal) error {
		signals = append(signals, signal)
		return previous(pid, signal)
	}
	t.Cleanup(func() { signingRunKillProcessGroupFn = previous })

	ctx, cancel := context.WithCancelCause(context.Background())
	timer := time.AfterFunc(50*time.Millisecond, func() {
		cancel(&signingRunSignalCause{signal: syscall.SIGTERM})
	})
	defer timer.Stop()
	err := runSigningRunChild(ctx, []string{"/bin/sleep", "30"})
	if code, ok := shared.ProcessExitCode(err); !ok || code != 143 {
		t.Fatalf("error = %v, want signal exit code 143", err)
	}
	if !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGTERM}) {
		t.Fatalf("cancellation signals = %v, want SIGTERM before wait only", signals)
	}
}

func TestSigningRunContextPreservesTriggeringSignal(t *testing.T) {
	for _, want := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(want.String(), func(t *testing.T) {
			signals := make(chan os.Signal, 1)
			ctx, stop := contextWithSigningRunSignals(context.Background(), signals, func() {})
			t.Cleanup(stop)

			signals <- want
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("signal did not cancel signing context")
			}
			if got := signingRunCancellationSignal(ctx); got != want {
				t.Fatalf("cancellation signal = %v, want %v", got, want)
			}
		})
	}
}

func TestParseSigningRunCertificateFingerprints(t *testing.T) {
	const fingerprint = "F05FBB6DC3E25BCCE5BB96697F633D1FC9CBBFD0"
	got := parseSigningRunCertificateFingerprints([]byte("SHA-256 hash: 0123456789\nSHA-1 hash: " + fingerprint + "\n"))
	if !reflect.DeepEqual(got, []string{fingerprint}) {
		t.Fatalf("fingerprints = %#v", got)
	}
}

func TestUtilityFailureIncludesBoundedSanitizedDiagnostic(t *testing.T) {
	wantErr := errors.New("exit status 1")
	diagnostic := "security: keychain is locked\nsecond line should be rendered safely"
	err := utilityFailure("import identity", []byte(diagnostic), wantErr)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped process error", err)
	}
	if got := err.Error(); !strings.Contains(got, "keychain is locked") || !strings.Contains(got, "second line") || strings.ContainsAny(got, "\r\n") {
		t.Fatalf("error = %q, want bounded terminal-safe diagnostic", got)
	}

	longDiagnostic := strings.Repeat("x", 1024)
	err = utilityFailure("import identity", []byte(longDiagnostic), wantErr)
	if got := err.Error(); len(got) > signingUtilityDiagnosticLimit+128 {
		t.Fatalf("error length = %d, want bounded diagnostic", len(got))
	}
}

func TestRunSigningRunChildDoesNotInheritSigningSecrets(t *testing.T) {
	t.Setenv("ASC_PRIVATE_KEY", "secret")
	t.Setenv("ASC_PRIVATE_KEY_B64", "secret")
	t.Setenv("ASC_ISSUER_ID", "issuer")
	err := runSigningRunChild(context.Background(), []string{
		"/bin/sh", "-c", "test -z \"$ASC_PRIVATE_KEY\" && test -z \"$ASC_PRIVATE_KEY_B64\" && test -z \"$ASC_ISSUER_ID\"",
	})
	if err != nil {
		t.Fatalf("runSigningRunChild() error = %v, want secrets filtered from child environment", err)
	}
}

func TestInstallSigningRunProfileRejectsCanceledContextBeforeDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	discovered := false
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) {
		discovered = true
		return t.TempDir(), nil
	}
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })

	_, err := installSigningRunProfile(ctx, "A7EFEF21-3432-404F-A488-083800B570FF", []byte("profile"), strings.Repeat("A", sha256.Size*2), func(signingRunProfileInstall) error {
		t.Fatal("canceled installation must not journal a profile")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if discovered {
		t.Fatal("profile directory discovery ran after cancellation")
	}
}

func TestSigningRunProfileInstallDirDistinguishesUnavailableXcode(t *testing.T) {
	previous := signingRunActiveXcodeMajorVersion
	t.Cleanup(func() { signingRunActiveXcodeMajorVersion = previous })

	home := t.TempDir()
	t.Setenv("HOME", home)

	signingRunActiveXcodeMajorVersion = func(context.Context) (int, error) {
		return 0, &exec.Error{Name: "xcodebuild", Err: exec.ErrNotFound}
	}
	legacy, err := signingRunProfileInstallDir(context.Background())
	if err != nil {
		t.Fatalf("unavailable xcodebuild error = %v, want legacy fallback", err)
	}
	if want := filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"); legacy != want {
		t.Fatalf("legacy directory = %q, want %q", legacy, want)
	}

	failure := errors.New("xcodebuild exited unsuccessfully")
	signingRunActiveXcodeMajorVersion = func(context.Context) (int, error) { return 0, failure }
	if _, err := signingRunProfileInstallDir(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("xcodebuild failure = %v, want wrapped failure", err)
	}
}

func TestActiveSigningRunXcodeUsesTrustedAbsolutePath(t *testing.T) {
	previous := signingRunCommandContext
	var gotPath string
	signingRunCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotPath = name
		return exec.CommandContext(ctx, "/usr/bin/printf", "Xcode 16.0\\n")
	}
	t.Cleanup(func() { signingRunCommandContext = previous })

	major, err := activeSigningRunXcodeMajorVersion(context.Background())
	if err != nil {
		t.Fatalf("activeSigningRunXcodeMajorVersion() error = %v", err)
	}
	if major != 16 {
		t.Fatalf("major = %d, want 16", major)
	}
	if gotPath != signingRunXcodebuildPath || !filepath.IsAbs(gotPath) {
		t.Fatalf("xcodebuild path = %q, want trusted absolute %q", gotPath, signingRunXcodebuildPath)
	}
}

func TestSigningRunProfileInstallDirRejectsNilContext(t *testing.T) {
	if _, err := signingRunProfileInstallDir(signingRunNilContext()); !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("error = %v, want required context", err)
	}
}

func TestValidateSigningRunJournal(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "asc-signing-run.fixture")
	valid := signingRunJournal{
		SchemaVersion: 1,
		TempDir:       tempDir,
		KeychainPath:  filepath.Join(tempDir, "signing.keychain-db"),
	}
	if err := validateSigningRunJournal(valid); err != nil {
		t.Fatalf("valid journal rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*signingRunJournal)
	}{
		{name: "schema", mutate: func(journal *signingRunJournal) { journal.SchemaVersion = 2 }},
		{name: "broad temp", mutate: func(journal *signingRunJournal) { journal.TempDir = os.TempDir() }},
		{name: "outside temp", mutate: func(journal *signingRunJournal) { journal.TempDir = "/Users/me/asc-signing-run.bad" }},
		{name: "keychain mismatch", mutate: func(journal *signingRunJournal) { journal.KeychainPath = "/tmp/other" }},
		{name: "unplanned profile", mutate: func(journal *signingRunJournal) { journal.ProfilePath = "/tmp/profile" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			journal := valid
			test.mutate(&journal)
			if err := validateSigningRunJournal(journal); err == nil {
				t.Fatalf("expected invalid journal: %+v", journal)
			}
		})
	}
}

func TestRemoveSigningRunTempDirRejectsBroadOrForeignPaths(t *testing.T) {
	for _, path := range []string{os.TempDir(), filepath.Join(os.TempDir(), "other"), "/", "/Users/me/asc-signing-run.fake"} {
		if err := removeSigningRunTempDir(path); err == nil {
			t.Fatalf("removeSigningRunTempDir(%q) unexpectedly succeeded", path)
		}
	}
}

func TestRemoveSigningRunTempDirRemovesRegularSidecarFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "asc-signing-run.")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	if err := os.WriteFile(filepath.Join(tempDir, ".fl12345678"), []byte("lock"), 0o600); err != nil {
		t.Fatalf("write keychain sidecar: %v", err)
	}

	if err := removeSigningRunTempDir(tempDir); err != nil {
		t.Fatalf("removeSigningRunTempDir() error: %v", err)
	}
	if _, err := os.Stat(tempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir stat error = %v, want not exist", err)
	}
}

func TestRemoveSigningRunTempDirRejectsNestedAndSymlinkEntries(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func(string) error
	}{
		{name: "directory", create: func(dir string) error { return os.Mkdir(filepath.Join(dir, "nested"), 0o700) }},
		{name: "symlink", create: func(dir string) error { return os.Symlink("/tmp", filepath.Join(dir, "link")) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "asc-signing-run.")
			if err != nil {
				t.Fatalf("create temp dir: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
			if err := test.create(tempDir); err != nil {
				t.Fatalf("create unsafe entry: %v", err)
			}
			if err := removeSigningRunTempDir(tempDir); err == nil {
				t.Fatal("expected unsafe entry rejection")
			}
		})
	}
}

func TestRecoverSigningRunJournalRemovesCodesignProbeCrashResidue(t *testing.T) {
	stateDir := t.TempDir()
	tempDir, err := os.MkdirTemp("", "asc-signing-run.")
	if err != nil {
		t.Fatalf("create signing temp dir: %v", err)
	}
	if err := os.Chmod(tempDir, 0o700); err != nil {
		t.Fatalf("chmod signing temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	if err := os.WriteFile(filepath.Join(tempDir, "codesign-probe"), []byte("probe"), 0o700); err != nil {
		t.Fatalf("write crash residue: %v", err)
	}

	previousStateAnchor := signingRunStateAnchorFn
	previousRemoveSearch := signingRunRecoveryRemoveSearchEntryFn
	previousDeleteKeychain := signingRunRecoveryDeleteKeychainFn
	signingRunStateAnchorFn = func() (*signingRunStateAnchor, error) {
		stateRoot, rootErr := rootfs.New(stateDir)
		if rootErr != nil {
			return nil, rootErr
		}
		return &signingRunStateAnchor{root: stateRoot, relative: "."}, nil
	}
	signingRunRecoveryRemoveSearchEntryFn = func(context.Context, string) error { return nil }
	signingRunRecoveryDeleteKeychainFn = func(context.Context, string) error { return nil }
	t.Cleanup(func() {
		signingRunStateAnchorFn = previousStateAnchor
		signingRunRecoveryRemoveSearchEntryFn = previousRemoveSearch
		signingRunRecoveryDeleteKeychainFn = previousDeleteKeychain
	})

	journal := signingRunJournal{
		SchemaVersion: 1,
		TempDir:       tempDir,
		KeychainPath:  filepath.Join(tempDir, "signing.keychain-db"),
	}
	if err := writeSigningRunJournal(journal, false); err != nil {
		t.Fatalf("write recovery journal: %v", err)
	}
	if err := recoverSigningRunJournal(context.Background()); err != nil {
		t.Fatalf("recover signing run: %v", err)
	}
	if _, err := os.Stat(tempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "journal.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal stat error = %v, want not exist", err)
	}
}

func TestRecoverSigningRunJournalDistinguishesPartialWriteFromReplacement(t *testing.T) {
	for _, test := range []struct {
		name         string
		recorded     []byte
		staged       []byte
		wantErr      bool
		wantPreserve bool
	}{
		{name: "recorded empty stage", recorded: nil, staged: nil},
		{name: "unrecorded partial write", recorded: nil, staged: []byte("signed"), wantErr: true, wantPreserve: true},
		{name: "recorded full write", recorded: []byte("signed-profile"), staged: []byte("signed-profile")},
		{name: "foreign replacement", recorded: nil, staged: []byte("foreign replacement"), wantErr: true, wantPreserve: true},
		{name: "oversized foreign replacement", recorded: nil, staged: bytes.Repeat([]byte("x"), signingRunInputLimit+1), wantErr: true, wantPreserve: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateDir := t.TempDir()
			tempDir, err := os.MkdirTemp("", "asc-signing-run.")
			if err != nil {
				t.Fatalf("create signing temp dir: %v", err)
			}
			if err := os.Chmod(tempDir, 0o700); err != nil {
				t.Fatalf("chmod signing temp dir: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			profileDir := filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles")
			if err := os.MkdirAll(profileDir, 0o700); err != nil {
				t.Fatalf("create profile dir: %v", err)
			}
			stagedPath := filepath.Join(profileDir, ".asc-signing-run-profile-recovery")
			if err := os.WriteFile(stagedPath, test.staged, 0o600); err != nil {
				t.Fatalf("write staged profile: %v", err)
			}
			info, err := os.Stat(stagedPath)
			if err != nil {
				t.Fatalf("stat staged profile: %v", err)
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				t.Fatal("staged profile has no platform file identity")
			}
			profileData := []byte("signed-profile")
			digest := sha256.Sum256(profileData)
			stagedDigest := sha256.Sum256(test.recorded)

			previousStateAnchor := signingRunStateAnchorFn
			previousRemoveSearch := signingRunRecoveryRemoveSearchEntryFn
			previousDeleteKeychain := signingRunRecoveryDeleteKeychainFn
			signingRunStateAnchorFn = func() (*signingRunStateAnchor, error) {
				stateRoot, rootErr := rootfs.New(stateDir)
				if rootErr != nil {
					return nil, rootErr
				}
				return &signingRunStateAnchor{root: stateRoot, relative: "."}, nil
			}
			signingRunRecoveryRemoveSearchEntryFn = func(context.Context, string) error { return nil }
			signingRunRecoveryDeleteKeychainFn = func(context.Context, string) error { return nil }
			t.Cleanup(func() {
				signingRunStateAnchorFn = previousStateAnchor
				signingRunRecoveryRemoveSearchEntryFn = previousRemoveSearch
				signingRunRecoveryDeleteKeychainFn = previousDeleteKeychain
			})

			journal := signingRunJournal{
				SchemaVersion: 1,
				TempDir:       tempDir,
				KeychainPath:  filepath.Join(tempDir, "signing.keychain-db"),
				ProfilePath: filepath.Join(
					profileDir, "a7efef21-3432-404f-a488-083800b570ff.mobileprovision",
				),
				StagedProfilePath:   stagedPath,
				ProfileDigest:       hex.EncodeToString(digest[:]),
				StagedProfileDigest: hex.EncodeToString(stagedDigest[:]),
				ProfileDevice:       uint64(stat.Dev),
				ProfileInode:        stat.Ino,
				ProfileCreated:      true,
			}
			if err := writeSigningRunJournal(journal, false); err != nil {
				t.Fatalf("write recovery journal: %v", err)
			}
			recoveryErr := recoverSigningRunJournal(context.Background())
			if test.wantErr != errors.Is(recoveryErr, errSigningRunStagedProfileChanged) {
				t.Fatalf("recover signing run error = %v, changed=%t", recoveryErr, test.wantErr)
			}
			_, stagedErr := os.Stat(stagedPath)
			if test.wantPreserve {
				if stagedErr != nil {
					t.Fatalf("preserved staged profile stat: %v", stagedErr)
				}
			} else if !errors.Is(stagedErr, os.ErrNotExist) {
				t.Fatalf("staged profile stat error = %v, want not exist", stagedErr)
			}
			if _, err := os.Stat(tempDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temp dir stat error = %v, want not exist", err)
			}
			if _, err := os.Stat(filepath.Join(stateDir, "journal.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("journal stat error = %v, want not exist", err)
			}
		})
	}
}

func TestLimitedSigningBufferBoundsOutput(t *testing.T) {
	buffer := &limitedSigningBuffer{limit: 4}
	if written, err := buffer.Write([]byte("123456789")); err != nil || written != 9 {
		t.Fatalf("Write() = %d, %v", written, err)
	}
	if got := string(buffer.Bytes()); got != "1234" {
		t.Fatalf("buffer = %q", got)
	}
}

func TestUtilityFailureIncludesSanitizedBoundedStderrAndRedactsSecrets(t *testing.T) {
	secret := []byte("keychain-password")
	cause := errors.New("exit status 1")
	stderr := append([]byte("security: failed to unlock \x1b[31m"), secret...)
	stderr = append(stderr, " encoded=6b6579636861696e2d70617373776f7264\n"...)

	err := utilityFailure("restrict key partition list", stderr, cause, secret)
	if !errors.Is(err, cause) {
		t.Fatalf("utilityFailure() error = %v, want injected cause", err)
	}
	message := err.Error()
	if !strings.Contains(message, "security: failed to unlock [31m[REDACTED]") {
		t.Fatalf("utilityFailure() = %q, want sanitized stderr diagnostic", message)
	}
	if strings.Contains(message, string(secret)) {
		t.Fatalf("utilityFailure() leaked secret: %q", message)
	}
	if strings.ContainsAny(message, "\x1b\r\n") {
		t.Fatalf("utilityFailure() contains terminal or line controls: %q", message)
	}

	large := bytes.Repeat([]byte("x"), signingUtilityDiagnosticLimit+1)
	largeMessage := utilityFailure("verify imported signing identity", large, cause).Error()
	if len(largeMessage) > signingUtilityDiagnosticLimit+128 {
		t.Fatalf("utilityFailure() diagnostic is not bounded: %d bytes", len(largeMessage))
	}
	if !strings.Contains(largeMessage, "[truncated]") {
		t.Fatalf("utilityFailure() = %q, want truncation marker", largeMessage)
	}
}

func TestVerifySigningRunIdentityUsableIncludesInjectedCodesignStderr(t *testing.T) {
	previous := signingRunCommandContext
	signingRunCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "printf 'codesign: injected diagnostic\\n' >&2; exit 1")
	}
	t.Cleanup(func() { signingRunCommandContext = previous })

	err := verifySigningRunIdentityUsable(
		context.Background(),
		t.TempDir(),
		filepath.Join(t.TempDir(), "signing.keychain-db"),
		strings.Repeat("A", 40),
	)
	if err == nil || !strings.Contains(err.Error(), "codesign: injected diagnostic") {
		t.Fatalf("verifySigningRunIdentityUsable() = %v, want injected codesign diagnostic", err)
	}
}

func TestWithSigningRunPartitionPasswordInputClearsDerivedBuffer(t *testing.T) {
	var captured []byte
	wantErr := errors.New("stop")
	err := withSigningRunPartitionPasswordInput([]byte{0x01, 0xab, 0xff}, func(stdin []byte) error {
		captured = stdin
		if string(stdin) != "01abff\n" {
			t.Fatalf("partition-list input = %q", stdin)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want injected error", err)
	}
	if !bytes.Equal(captured, make([]byte, len(captured))) {
		t.Fatalf("derived password input was not cleared: %v", captured)
	}
}

func TestReadBoundedSigningRunFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.p12")
	if err := os.WriteFile(path, []byte("identity"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, true); err != nil {
		t.Fatalf("private input rejected: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, true); err == nil {
		t.Fatal("expected group/world-readable private input rejection")
	}
	if _, err := readBoundedSigningRunFile(path, 32, false); err != nil {
		t.Fatalf("read-only profile input rejected: %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("chmod writable: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, false); err == nil {
		t.Fatal("expected group/world-writable profile input rejection")
	}
}

func TestInstallSigningRunProfileReusesAndProtectsExistingFiles(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	digestBytes := sha256.Sum256(data)
	digest := strings.ToUpper(hex.EncodeToString(digestBytes[:]))
	journaled := signingRunProfileInstall{}
	var stagedDigests []string
	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(planned signingRunProfileInstall) error {
		journaled = planned
		stagedDigests = append(stagedDigests, planned.StagedDigest)
		return nil
	})
	if err != nil {
		t.Fatalf("installSigningRunProfile: %v", err)
	}
	if !installed.Created || journaled.StagedPath == "" || journaled.Device == 0 || journaled.Inode == 0 {
		t.Fatalf("missing staged ownership proof: installed=%+v journaled=%+v", installed, journaled)
	}
	emptyDigest := sha256.Sum256(nil)
	wantStagedDigests := []string{hex.EncodeToString(emptyDigest[:]), strings.ToLower(digest)}
	if !reflect.DeepEqual(stagedDigests, wantStagedDigests) {
		t.Fatalf("staged journal digests = %v, want %v", stagedDigests, wantStagedDigests)
	}
	reused, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error {
		t.Fatal("reused profile must not create a new journal entry")
		return nil
	})
	if err != nil || reused.Created {
		t.Fatalf("reuse = %+v, %v", reused, err)
	}
	if _, err := installSigningRunProfile(context.Background(), uuid, []byte("different"), strings.Repeat("A", 64), func(signingRunProfileInstall) error { return nil }); err == nil {
		t.Fatal("expected different existing profile conflict")
	}
	if err := removeSigningRunProfile(installed); err != nil {
		t.Fatalf("removeSigningRunProfile: %v", err)
	}
}

func TestInstallSigningRunProfileRetainsOwnershipAfterJournalFailure(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	digestBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(digestBytes[:])
	journalErr := errors.New("journal unavailable")

	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error {
		return journalErr
	})
	if !errors.Is(err, journalErr) {
		t.Fatalf("installSigningRunProfile() error = %v, want journal failure", err)
	}
	if !installed.Created || installed.StagedPath == "" || installed.Device == 0 || installed.Inode == 0 {
		t.Fatalf("missing retained ownership proof: %+v", installed)
	}
	if _, statErr := os.Stat(installed.Path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("published profile stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(installed.StagedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("staged profile stat error = %v, want defer cleanup", statErr)
	}
}

func TestWriteSigningRunStagedProfileHashesPartialWrite(t *testing.T) {
	data := []byte("signed-profile")
	wantErr := errors.New("injected partial write")
	digest, err := writeSigningRunStagedProfile(signingRunPartialWriter{written: 6, err: wantErr}, data)
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeSigningRunStagedProfile() error = %v, want injected failure", err)
	}
	wantDigest := sha256.Sum256(data[:6])
	if digest != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("partial digest = %q, want %q", digest, hex.EncodeToString(wantDigest[:]))
	}

	_, err = writeSigningRunStagedProfile(signingRunPartialWriter{written: 6}, data)
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v, want io.ErrShortWrite", err)
	}
}

func TestInstallSigningRunProfilePreservesStagedReplacementAfterJournalFailure(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	replacement := []byte("foreign replacement")
	digestBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(digestBytes[:])
	journalErr := errors.New("journal unavailable")
	var planned signingRunProfileInstall

	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(candidate signingRunProfileInstall) error {
		planned = candidate
		if removeErr := os.Remove(candidate.StagedPath); removeErr != nil {
			t.Fatalf("remove original staged profile: %v", removeErr)
		}
		if writeErr := os.WriteFile(candidate.StagedPath, replacement, 0o600); writeErr != nil {
			t.Fatalf("write staged replacement: %v", writeErr)
		}
		return journalErr
	})

	if !errors.Is(err, journalErr) || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("installSigningRunProfile() error = %v, want journal and identity failures", err)
	}
	if installed.StagedPath != planned.StagedPath || installed.Device != planned.Device || installed.Inode != planned.Inode {
		t.Fatalf("retained ownership proof changed: installed=%+v planned=%+v", installed, planned)
	}
	if _, statErr := os.Stat(installed.Path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("published profile stat error = %v, want not exist", statErr)
	}
	got, readErr := os.ReadFile(planned.StagedPath)
	if readErr != nil {
		t.Fatalf("read staged replacement: %v", readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("staged replacement = %q, want %q", got, replacement)
	}
}

func TestInstallSigningRunProfileRejectsStagedReplacementBeforePublish(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	replacement := []byte("foreign replacement")
	digestBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(digestBytes[:])
	var planned signingRunProfileInstall

	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(candidate signingRunProfileInstall) error {
		planned = candidate
		if removeErr := os.Remove(candidate.StagedPath); removeErr != nil {
			t.Fatalf("remove original staged profile: %v", removeErr)
		}
		if writeErr := os.WriteFile(candidate.StagedPath, replacement, 0o600); writeErr != nil {
			t.Fatalf("write staged replacement: %v", writeErr)
		}
		return nil
	})

	if err == nil || (!strings.Contains(err.Error(), "identity changed") && !strings.Contains(err.Error(), "content changed")) {
		t.Fatalf("installSigningRunProfile() error = %v, want replacement rejection", err)
	}
	if installed.StagedPath != planned.StagedPath || installed.Device != planned.Device || installed.Inode != planned.Inode {
		t.Fatalf("retained ownership proof changed: installed=%+v planned=%+v", installed, planned)
	}
	if _, statErr := os.Stat(installed.Path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("published profile stat error = %v, want not exist", statErr)
	}
	got, readErr := os.ReadFile(installed.StagedPath)
	if readErr != nil {
		t.Fatalf("read restored staged replacement: %v", readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("restored staged replacement = %q, want %q", got, replacement)
	}
}

func TestRemoveSigningRunStagedProfileRequiresIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".asc-signing-run-profile-unowned")
	data := []byte("unowned staged profile")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}

	err := removeSigningRunStagedProfile(path, 0, 0, strings.Repeat("A", sha256.Size*2))
	if err == nil || !strings.Contains(err.Error(), "identity is unavailable") {
		t.Fatalf("removeSigningRunStagedProfile() error = %v, want unavailable identity", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved staged profile: %v", readErr)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("preserved staged profile = %q, want %q", got, data)
	}
}

func TestInstallSigningRunProfileRejectsOversizedExistingFile(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	path := filepath.Join(installDir, strings.ToLower(uuid)+".mobileprovision")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create oversized profile: %v", err)
	}
	if err := file.Truncate(signingRunInputLimit + 1); err != nil {
		_ = file.Close()
		t.Fatalf("truncate oversized profile: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close oversized profile: %v", err)
	}
	_, err = installSigningRunProfile(context.Background(), uuid, []byte("profile"), strings.Repeat("A", sha256.Size*2), func(signingRunProfileInstall) error {
		t.Fatal("oversized existing profile must not create a journal entry")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds the size limit") {
		t.Fatalf("error = %v, want size-limit rejection", err)
	}
}

func TestRemoveSigningRunProfileRefusesReplacement(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	digestBytes := sha256.Sum256(data)
	digest := strings.ToUpper(hex.EncodeToString(digestBytes[:]))
	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error { return nil })
	if err != nil {
		t.Fatalf("installSigningRunProfile: %v", err)
	}
	if err := os.Remove(installed.Path); err != nil {
		t.Fatalf("remove original: %v", err)
	}
	if err := os.WriteFile(installed.Path, data, 0o600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if err := removeSigningRunProfile(installed); err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("error = %v, want file identity refusal", err)
	}
}

func TestRemoveSigningRunProfileSupportsFullInputLimit(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := bytes.Repeat([]byte("p"), (8<<20)+1)
	digestBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(digestBytes[:])

	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error {
		return nil
	})
	if err != nil {
		t.Fatalf("installSigningRunProfile() error: %v", err)
	}
	if err := removeSigningRunProfile(installed); err != nil {
		t.Fatalf("removeSigningRunProfile() error for profile above default identity limit: %v", err)
	}
	if _, err := os.Stat(installed.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("profile stat error = %v, want profile removed", err)
	}
}

func TestRemoveSigningRunStagedProfileQuarantinesBeforeRemoval(t *testing.T) {
	installDir := t.TempDir()
	path := filepath.Join(installDir, ".asc-signing-run-profile-staged")
	if err := os.WriteFile(path, []byte("staged-profile"), 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat staged profile: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("staged profile has no platform file identity")
	}
	digest := sha256.Sum256([]byte("staged-profile"))
	if err := removeSigningRunStagedProfile(path, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:])); err != nil {
		t.Fatalf("removeSigningRunStagedProfile() error: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged profile stat error = %v, want not exist", err)
	}
}

func TestRemoveSigningRunStagedProfileSupportsFullInputLimit(t *testing.T) {
	installDir := t.TempDir()
	path := filepath.Join(installDir, ".asc-signing-run-profile-large")
	data := bytes.Repeat([]byte("p"), (8<<20)+1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat staged profile: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("staged profile has no platform file identity")
	}
	digest := sha256.Sum256(data)
	if err := removeSigningRunStagedProfile(path, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:])); err != nil {
		t.Fatalf("removeSigningRunStagedProfile() error: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged profile stat error = %v, want not exist", err)
	}
}

func TestRemoveSigningRunStagedProfilePreservesSameInodeReplacement(t *testing.T) {
	installDir := t.TempDir()
	path := filepath.Join(installDir, ".asc-signing-run-profile-staged")
	original := []byte("staged-profile")
	replacement := []byte("foreign same-inode replacement")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat staged profile: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("staged profile has no platform file identity")
	}
	if err := os.WriteFile(path, replacement, 0o600); err != nil {
		t.Fatalf("replace staged profile contents: %v", err)
	}
	replacedInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat replaced staged profile: %v", err)
	}
	replacedStat, ok := replacedInfo.Sys().(*syscall.Stat_t)
	if !ok || replacedStat.Dev != stat.Dev || replacedStat.Ino != stat.Ino {
		t.Fatal("test setup did not preserve the staged profile inode")
	}

	digest := sha256.Sum256(original)
	err = removeSigningRunStagedProfile(path, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:]))
	if err == nil || !strings.Contains(err.Error(), "content changed") {
		t.Fatalf("removeSigningRunStagedProfile() error = %v, want content-change refusal", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved staged replacement: %v", readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("preserved staged replacement = %q, want %q", got, replacement)
	}
}

func TestRemoveSigningRunStagedProfileClassifiesRemovalRaceAsChanged(t *testing.T) {
	installDir := t.TempDir()
	name := ".asc-signing-run-profile-staged"
	path := filepath.Join(installDir, name)
	original := []byte("staged-profile")
	replacement := []byte("foreign replacement")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat staged profile: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("staged profile has no platform file identity")
	}
	digest := sha256.Sum256(original)
	installRoot, err := rootfs.New(installDir)
	if err != nil {
		t.Fatalf("open install root: %v", err)
	}
	t.Cleanup(func() { _ = installRoot.Close() })

	err = removeSigningRunStagedProfileEntryWithHook(
		installRoot, name, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:]),
		func() error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.WriteFile(path, replacement, 0o600)
		},
	)
	if !errors.Is(err, errSigningRunStagedProfileChanged) || !errors.Is(err, rootfs.ErrFileIdentityChanged) {
		t.Fatalf("remove staged profile error = %v, want staged and rootfs identity-change sentinels", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved replacement: %v", readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("preserved replacement = %q, want %q", got, replacement)
	}
}

func TestRemoveSigningRunStagedProfileClassifiesSpecialFileReplacementsAsChanged(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func(t *testing.T, path string)
		mode   os.FileMode
	}{
		{
			name: "symlink",
			create: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Symlink("foreign-target", path); err != nil {
					t.Fatalf("create staged symlink replacement: %v", err)
				}
			},
			mode: os.ModeSymlink,
		},
		{
			name: "directory",
			create: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("create staged directory replacement: %v", err)
				}
			},
			mode: os.ModeDir,
		},
		{
			name: "fifo",
			create: func(t *testing.T, path string) {
				t.Helper()
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatalf("create staged FIFO replacement: %v", err)
				}
			},
			mode: os.ModeNamedPipe,
		},
	} {
		for _, phase := range []string{"initial verification", "identity capture"} {
			t.Run(test.name+"/"+phase, func(t *testing.T) {
				installDir := t.TempDir()
				name := ".asc-signing-run-profile-staged"
				path := filepath.Join(installDir, name)
				original := []byte("staged-profile")
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatalf("write staged profile: %v", err)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("stat staged profile: %v", err)
				}
				stat, ok := info.Sys().(*syscall.Stat_t)
				if !ok {
					t.Fatal("staged profile has no platform file identity")
				}
				if err := os.Remove(path); err != nil {
					t.Fatalf("remove staged profile: %v", err)
				}
				test.create(t, path)
				digest := sha256.Sum256(original)

				switch phase {
				case "initial verification":
					err = removeSigningRunStagedProfile(path, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:]))
				case "identity capture":
					installRoot, rootErr := rootfs.New(installDir)
					if rootErr != nil {
						t.Fatalf("open install root: %v", rootErr)
					}
					t.Cleanup(func() { _ = installRoot.Close() })
					err = removeSigningRunStagedProfileEntry(installRoot, name, uint64(stat.Dev), uint64(stat.Ino), hex.EncodeToString(digest[:]))
				}
				if !errors.Is(err, errSigningRunStagedProfileChanged) {
					t.Fatalf("remove staged profile error = %v, want staged-profile-changed sentinel", err)
				}
				replacementInfo, lstatErr := os.Lstat(path)
				if lstatErr != nil {
					t.Fatalf("lstat preserved special-file replacement: %v", lstatErr)
				}
				if replacementInfo.Mode()&test.mode == 0 {
					t.Fatalf("preserved replacement mode = %v, want %v", replacementInfo.Mode(), test.mode)
				}
			})
		}
	}
}

func TestClassifySigningRunStagedProfileRemovalErrorPreservesOperationalFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{
			name: "quarantine cleanup uncertainty",
			err:  errors.Join(rootfs.ErrFileIdentityChanged, rootfs.ErrQuarantineCleanupUncertain),
		},
		{
			name: "directory sync failure",
			err:  errors.Join(rootfs.ErrFileIdentityChanged, errors.New("sync parent directory")),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := classifySigningRunStagedProfileRemovalError(test.err)
			if errors.Is(got, errSigningRunStagedProfileChanged) {
				t.Fatalf("classified operational failure as staged change: %v", got)
			}
			if !errors.Is(got, rootfs.ErrFileIdentityChanged) {
				t.Fatalf("lost identity-change evidence: %v", got)
			}
		})
	}
}

func TestRemoveSigningRunStagedProfilePreservesJoinedCaptureFailure(t *testing.T) {
	installDir := t.TempDir()
	name := ".asc-signing-run-profile-staged"
	path := filepath.Join(installDir, name)
	data := []byte("staged-profile")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write staged profile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat staged profile: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("staged profile has no platform file identity")
	}
	installRoot, err := rootfs.New(installDir)
	if err != nil {
		t.Fatalf("open install root: %v", err)
	}
	t.Cleanup(func() { _ = installRoot.Close() })
	operationalErr := errors.New("reinspect staged profile")
	digest := sha256.Sum256(data)
	err = removeSigningRunStagedProfileEntryWithCaptureHook(
		installRoot,
		name,
		uint64(stat.Dev),
		uint64(stat.Ino),
		hex.EncodeToString(digest[:]),
		func() (*rootfs.FileIdentity, error) {
			return nil, errors.Join(rootfs.ErrFileIdentityChanged, operationalErr)
		},
		nil,
	)
	if !errors.Is(err, operationalErr) || errors.Is(err, errSigningRunStagedProfileChanged) {
		t.Fatalf("remove staged profile error = %v, want retained operational failure", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(got, data) {
		t.Fatalf("preserved staged profile = %q, %v; want %q", got, readErr, data)
	}
}

func TestRemoveSigningRunProfilePreservesReplacementDuringCleanup(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	replacement := []byte("replacement")
	digestBytes := sha256.Sum256(data)
	digest := strings.ToUpper(hex.EncodeToString(digestBytes[:]))
	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error { return nil })
	if err != nil {
		t.Fatalf("installSigningRunProfile: %v", err)
	}

	err = removeSigningRunProfileWithHook(installed, func() error {
		if err := os.Remove(installed.Path); err != nil {
			return err
		}
		return os.WriteFile(installed.Path, replacement, 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "changed during cleanup") {
		t.Fatalf("error = %v, want concurrent replacement refusal", err)
	}
	got, readErr := os.ReadFile(installed.Path)
	if readErr != nil {
		t.Fatalf("read preserved replacement: %v", readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("profile content = %q, want replacement %q", got, replacement)
	}
}

func TestRemoveSigningRunProfileRecoversQuarantinedProfile(t *testing.T) {
	installDir := t.TempDir()
	previous := signingRunProfileInstallDirFn
	signingRunProfileInstallDirFn = func(context.Context) (string, error) { return installDir, nil }
	t.Cleanup(func() { signingRunProfileInstallDirFn = previous })
	const uuid = "A7EFEF21-3432-404F-A488-083800B570FF"
	data := []byte("signed-profile")
	digestBytes := sha256.Sum256(data)
	digest := strings.ToUpper(hex.EncodeToString(digestBytes[:]))
	installed, err := installSigningRunProfile(context.Background(), uuid, data, digest, func(signingRunProfileInstall) error { return nil })
	if err != nil {
		t.Fatalf("installSigningRunProfile: %v", err)
	}
	quarantinePath := filepath.Join(installDir, ".asc-signing-run-profile-remove-"+filepath.Base(installed.Path))
	if err := os.Rename(installed.Path, quarantinePath); err != nil {
		t.Fatalf("simulate interrupted quarantine: %v", err)
	}

	if err := removeSigningRunProfile(installed); err != nil {
		t.Fatalf("recover quarantined profile: %v", err)
	}
	for _, path := range []string{installed.Path, quarantinePath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s stat error = %v, want not exist", path, err)
		}
	}
}

func TestSigningRunDisposableKeychainSmoke(t *testing.T) {
	if os.Getenv("ASC_SIGNING_RUN_LIVE_TEST") != "1" {
		t.Skip("set ASC_SIGNING_RUN_LIVE_TEST=1 to exercise a disposable macOS keychain")
	}
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{})
	inspection, err := inspectSigningRunInputs(fixture.identity, []byte(fixture.password), fixture.profile, fixture.roots, fixture.now)
	if err != nil {
		t.Fatalf("inspect fixture: %v", err)
	}
	tempDir, err := createSigningRunTempDir()
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	keychainPath := filepath.Join(tempDir, "signing.keychain-db")
	password, err := signingRunRandomBytes(32)
	if err != nil {
		t.Fatalf("random password: %v", err)
	}
	original, err := keychainSearchList(context.Background())
	if err != nil {
		t.Fatalf("read original search list: %v", err)
	}
	t.Cleanup(func() {
		_ = removeKeychainSearchEntry(context.Background(), keychainPath)
		_ = deleteSigningRunKeychain(context.Background(), keychainPath)
		_ = removeSigningRunTempDir(tempDir)
	})
	if err := createSigningRunKeychain(context.Background(), keychainPath, password); err != nil {
		t.Fatalf("create keychain: %v", err)
	}
	if err := removeKeychainSearchEntry(context.Background(), keychainPath); err != nil {
		t.Fatalf("isolate keychain: %v", err)
	}
	digest := sha1.Sum(inspection.Certificate.Raw)
	if err := importSigningRunIdentity(
		context.Background(), keychainPath, password, fixture.identity, []byte(fixture.password), strings.ToUpper(hex.EncodeToString(digest[:])),
	); err != nil {
		t.Fatalf("import identity: %v", err)
	}
	if err := deleteSigningRunKeychain(context.Background(), keychainPath); err != nil {
		t.Fatalf("delete keychain: %v", err)
	}
	if err := removeSigningRunTempDir(tempDir); err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}
	after, err := keychainSearchList(context.Background())
	if err != nil {
		t.Fatalf("read final search list: %v", err)
	}
	if !reflect.DeepEqual(after, original) {
		t.Fatalf("keychain search list changed: before=%v after=%v", original, after)
	}
}
