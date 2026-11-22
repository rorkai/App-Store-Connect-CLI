package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebSignInKeysCreateRequiresBundleID(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--name", "Sway", "--output-dir", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" || !strings.Contains(stderr, "--bundle-id is required") {
		t.Fatalf("unexpected output: %q %q", stdout, stderr)
	}
}

func TestWebSignInKeysCreateRequiresConfirmBeforeSessionResolution(t *testing.T) {
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })
	resolveCalls := 0
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return nil, "", errors.New("session resolution must not run without confirmation")
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysCreateCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--name", "Sway",
		"--bundle-id", "BUNDLE123",
		"--output-dir", t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if resolveCalls != 0 {
		t.Fatalf("resolved a web session %d time(s) without --confirm", resolveCalls)
	}
	if stdout != "" || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("unexpected output: %q %q", stdout, stderr)
	}
}

func TestWebSignInKeysDownloadRequiresConfirmBeforeSessionResolution(t *testing.T) {
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })
	resolveCalls := 0
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return nil, "", errors.New("session resolution must not run without confirmation")
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysDownloadCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--key-id", "KEY123",
		"--output-dir", t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if resolveCalls != 0 {
		t.Fatalf("resolved a web session %d time(s) without --confirm", resolveCalls)
	}
	if stdout != "" || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("unexpected output: %q %q", stdout, stderr)
	}
}

func TestWebSignInKeysCreateSavesPrivateFileAndReceipt(t *testing.T) {
	testWebSignInKeysCreateSavesPrivateFileAndReceipt(t, t.TempDir())
}

func testWebSignInKeysCreateSavesPrivateFileAndReceipt(t *testing.T, directory string) {
	originalResolve, originalClient, originalPersist := resolveSessionFn, newWebClientFn, persistWebSessionFn
	originalCreate, originalDownload := createDeveloperSignInKeyFn, downloadDeveloperSignInKeyFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalClient
		persistWebSessionFn = originalPersist
		createDeveloperSignInKeyFn = originalCreate
		downloadDeveloperSignInKeyFn = originalDownload
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	createCalls := 0
	createDeveloperSignInKeyFn = func(_ context.Context, _ *webcore.Client, name, bundleID string) (*webcore.DeveloperSignInKey, error) {
		createCalls++
		if name != "Sway" || bundleID != "BUNDLE123" {
			t.Fatal("wrong create request")
		}
		return &webcore.DeveloperSignInKey{KeyID: "KEY123"}, nil
	}
	calls := 0
	downloadDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string) ([]byte, error) {
		calls++
		return []byte("PRIVATE-TEST-MATERIAL"), nil
	}
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--name", "Sway", "--bundle-id", "BUNDLE123", "--output-dir", directory, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := command.Exec(context.Background(), nil)
		if runtime.GOOS == "windows" {
			if !errors.Is(err, rootfs.ErrFileIdentityMutationUnsupported) {
				t.Fatalf("expected unsupported private publication: %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
	})
	if runtime.GOOS == "windows" {
		if calls != 0 || createCalls != 0 {
			t.Fatal("created or consumed a key on unsupported platform")
		}
		return
	}
	if strings.Contains(stdout+stderr, "PRIVATE-TEST-MATERIAL") {
		t.Fatal("private key leaked")
	}
	var receipt struct {
		KeyID  string `json:"keyId"`
		P8Path string `json:"p8Path"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.KeyID != "KEY123" || receipt.P8Path != filepath.Join(directory, "AuthKey_KEY123.p8") {
		t.Fatalf("wrong receipt %+v", receipt)
	}
	info, err := os.Stat(receipt.P8Path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("private permissions not preserved")
	}
	saved, err := os.ReadFile(receipt.P8Path)
	if err != nil || string(saved) != "PRIVATE-TEST-MATERIAL" {
		t.Fatal("key not saved")
	}
	d, _, _ = newWebTestDeps()
	download := newWebSignInKeysDownloadCommand(d)
	if err := download.FlagSet.Parse([]string{"--key-id", "KEY123", "--output-dir", directory, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	if err := download.Exec(context.Background(), nil); err == nil {
		t.Fatal("overwrote private key")
	}
	if calls != 1 {
		t.Fatal("consumed download despite existing destination")
	}
	d, _, _ = newWebTestDeps()
	createCollision := newWebSignInKeysCreateCommand(d)
	if err := createCollision.FlagSet.Parse([]string{"--name", "Sway", "--bundle-id", "BUNDLE123", "--output-dir", directory, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	if err := createCollision.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "P8 was not downloaded") || !strings.Contains(err.Error(), "--key-id KEY123 --output-dir OTHER_DIR") {
		t.Fatalf("missing safe recovery instructions: %v", err)
	}
	if calls != 1 {
		t.Fatal("consumed key after create destination collision")
	}
	unchanged, err := os.ReadFile(receipt.P8Path)
	if err != nil || string(unchanged) != "PRIVATE-TEST-MATERIAL" {
		t.Fatal("changed existing key after create collision")
	}
	for _, downloadFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("destination replaced, download fails=%t", downloadFails), func(t *testing.T) {
			racedDir := t.TempDir()
			destination := filepath.Join(racedDir, "AuthKey_KEY123.p8")
			downloadDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string) ([]byte, error) {
				if err := os.Remove(destination); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(destination, []byte("OTHER-FILE"), 0o600); err != nil {
					t.Fatal(err)
				}
				if downloadFails {
					return nil, errors.New("download failed")
				}
				return []byte("PRIVATE-TEST-MATERIAL"), nil
			}
			d, _, _ := newWebTestDeps()
			command := newWebSignInKeysDownloadCommand(d)
			if err := command.FlagSet.Parse([]string{"--key-id", "KEY123", "--output-dir", racedDir, "--confirm"}); err != nil {
				t.Fatal(err)
			}
			if err := command.Exec(context.Background(), nil); err == nil {
				t.Fatal("expected destination-change error")
			}
			remaining, err := os.ReadFile(destination)
			if err != nil || string(remaining) != "OTHER-FILE" {
				t.Fatal("replaced destination was overwritten or removed")
			}
			if !downloadFails {
				copies, err := filepath.Glob(filepath.Join(racedDir, ".asc-api-key-*.p8"))
				if err != nil || len(copies) != 1 {
					t.Fatalf("expected one recovery copy: %v %v", copies, err)
				}
				material, err := os.ReadFile(copies[0])
				if err != nil || string(material) != "PRIVATE-TEST-MATERIAL" {
					t.Fatal("lost one-time private key")
				}
			}
		})
	}
}

func TestWebSignInKeysDownloadRequiresKeyID(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysDownloadCommand(d)
	if err := command.FlagSet.Parse([]string{"--output-dir", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--key-id is required") {
		t.Fatalf("missing error: %q", stderr)
	}
}

type fakeSignInKeyDownloadRecoveryError struct {
	body []byte
}

func (e *fakeSignInKeyDownloadRecoveryError) Error() string {
	return "invalid api key download response"
}

func (e *fakeSignInKeyDownloadRecoveryError) Unwrap() error {
	return webcore.ErrAPIKeyResponseInvalid
}

func (e *fakeSignInKeyDownloadRecoveryError) RecoveryBody() []byte {
	return append([]byte(nil), e.body...)
}

func TestWebSignInKeysDownloadRetainsMalformed2xxResponse(t *testing.T) {
	testWebSignInKeysRetainsMalformed2xxResponse(t, false)
}

func TestWebSignInKeysCreateRetainsMalformed2xxResponse(t *testing.T) {
	testWebSignInKeysRetainsMalformed2xxResponse(t, true)
}

func testWebSignInKeysRetainsMalformed2xxResponse(t *testing.T, create bool) {
	const raw = "provider-error-secret"
	originalResolve, originalClient, originalPersist := resolveSessionFn, newWebClientFn, persistWebSessionFn
	originalCreate, originalDownload := createDeveloperSignInKeyFn, downloadDeveloperSignInKeyFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalClient
		persistWebSessionFn = originalPersist
		createDeveloperSignInKeyFn = originalCreate
		downloadDeveloperSignInKeyFn = originalDownload
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	createCalls := 0
	createDeveloperSignInKeyFn = func(_ context.Context, _ *webcore.Client, name, bundleID string) (*webcore.DeveloperSignInKey, error) {
		createCalls++
		if name != "Sway" || bundleID != "BUNDLE123" {
			t.Fatalf("wrong create request: %q %q", name, bundleID)
		}
		return &webcore.DeveloperSignInKey{KeyID: "KEY123"}, nil
	}
	downloadCalls := 0
	downloadDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string) ([]byte, error) {
		downloadCalls++
		return nil, &fakeSignInKeyDownloadRecoveryError{body: []byte(raw)}
	}

	outputDir := t.TempDir()
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysDownloadCommand(d)
	args := []string{"--key-id", "KEY123", "--output-dir", outputDir, "--confirm", "--output", "json"}
	if create {
		command = newWebSignInKeysCreateCommand(d)
		args = []string{"--name", "Sway", "--bundle-id", "BUNDLE123", "--output-dir", outputDir, "--confirm", "--output", "json"}
	}
	if err := command.FlagSet.Parse(args); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runtime.GOOS == "windows" {
		if !errors.Is(runErr, rootfs.ErrFileIdentityMutationUnsupported) {
			t.Fatalf("expected unsupported private publication: %v", runErr)
		}
		if createCalls != 0 || downloadCalls != 0 {
			t.Fatal("created or consumed a key on unsupported platform")
		}
		return
	}
	if runErr == nil {
		t.Fatal("expected malformed successful response to fail")
	}
	if !errors.Is(runErr, webcore.ErrAPIKeyResponseInvalid) {
		t.Fatalf("expected invalid response error, got %v", runErr)
	}
	if strings.Contains(stdout+stderr+runErr.Error(), raw) {
		t.Fatalf("raw response leaked to command output: stdout=%q stderr=%q err=%v", stdout, stderr, runErr)
	}
	if !strings.Contains(runErr.Error(), "inspect") || !strings.Contains(runErr.Error(), "do not retry") {
		t.Fatalf("missing safe recovery guidance: %v", runErr)
	}
	copies, err := filepath.Glob(filepath.Join(outputDir, ".asc-api-key-*.p8"))
	if err != nil || len(copies) != 1 {
		t.Fatalf("expected one retained recovery file: %v %v", copies, err)
	}
	info, err := os.Stat(copies[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("recovery file permissions = %v, err = %v", info.Mode().Perm(), err)
	}
	material, err := os.ReadFile(copies[0])
	if err != nil || string(material) != raw {
		t.Fatalf("recovery body = %q, err = %v", material, err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "AuthKey_KEY123.p8")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canonical P8 unexpectedly exists, stat error = %v", err)
	}
	if createCalls != btoi(create) || downloadCalls != 1 {
		t.Fatalf("create calls = %d, download calls = %d", createCalls, downloadCalls)
	}
}

func TestWebSignInKeysDownloadFailureCleansEmptyStage(t *testing.T) {
	originalResolve, originalClient, originalPersist := resolveSessionFn, newWebClientFn, persistWebSessionFn
	originalDownload := downloadDeveloperSignInKeyFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalClient
		persistWebSessionFn = originalPersist
		downloadDeveloperSignInKeyFn = originalDownload
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	downloadDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string) ([]byte, error) {
		return nil, errors.New("transport failed")
	}
	outputDir := t.TempDir()
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebSignInKeysDownloadCommand(d)
	if err := command.FlagSet.Parse([]string{"--key-id", "KEY123", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = command.Exec(context.Background(), nil) })
	if runtime.GOOS == "windows" {
		if !errors.Is(runErr, rootfs.ErrFileIdentityMutationUnsupported) {
			t.Fatalf("expected unsupported private publication: %v", runErr)
		}
		return
	}
	if runErr == nil {
		t.Fatal("expected download failure")
	}
	copies, err := filepath.Glob(filepath.Join(outputDir, ".asc-api-key-*.p8"))
	if err != nil || len(copies) != 0 {
		t.Fatalf("unexpected retained recovery files: %v %v", copies, err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "AuthKey_KEY123.p8")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canonical P8 unexpectedly exists, stat error = %v", err)
	}
}

func btoi(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestWebPushKeysCreateUsesAPNsServiceAndSavesKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private P8 publication is unsupported on Windows")
	}
	originalResolve, originalClient, originalPersist := resolveSessionFn, newWebClientFn, persistWebSessionFn
	originalSignInCreate, originalSignInDownload := createDeveloperSignInKeyFn, downloadDeveloperSignInKeyFn
	originalCreate, originalDownload := createDeveloperPushKeyFn, downloadDeveloperPushKeyFn
	t.Cleanup(func() {
		resolveSessionFn, newWebClientFn, persistWebSessionFn = originalResolve, originalClient, originalPersist
		createDeveloperSignInKeyFn, downloadDeveloperSignInKeyFn = originalSignInCreate, originalSignInDownload
		createDeveloperPushKeyFn, downloadDeveloperPushKeyFn = originalCreate, originalDownload
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	createDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string, string) (*webcore.DeveloperSignInKey, error) {
		t.Fatal("push-keys create used the Sign in with Apple service")
		return nil, nil
	}
	downloadDeveloperSignInKeyFn = func(context.Context, *webcore.Client, string) ([]byte, error) {
		t.Fatal("push-keys create used the Sign in with Apple download")
		return nil, nil
	}
	createDeveloperPushKeyFn = func(_ context.Context, _ *webcore.Client, name string) (*webcore.DeveloperSignInKey, error) {
		if name != "Push" {
			t.Fatalf("name = %q", name)
		}
		return &webcore.DeveloperSignInKey{KeyID: "KEY123"}, nil
	}
	downloadDeveloperPushKeyFn = func(_ context.Context, _ *webcore.Client, keyID string) ([]byte, error) {
		if keyID != "KEY123" {
			t.Fatalf("keyID = %q", keyID)
		}
		return []byte("PRIVATE-TEST-MATERIAL"), nil
	}
	directory := t.TempDir()
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebPushKeysCommand(d).Subcommands[0]
	if err := command.FlagSet.Parse([]string{"--name", "Push", "--output-dir", directory, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(stdout+stderr, "PRIVATE-TEST-MATERIAL") {
		t.Fatal("private key leaked")
	}
	want := fmt.Sprintf(`{"keyId":"KEY123","name":"Push","p8Path":%q}`, filepath.Join(directory, "AuthKey_KEY123.p8"))
	if strings.TrimSpace(stdout) != want {
		t.Fatalf("receipt = %s, want %s", stdout, want)
	}
	saved, err := os.ReadFile(filepath.Join(directory, "AuthKey_KEY123.p8"))
	if err != nil || string(saved) != "PRIVATE-TEST-MATERIAL" {
		t.Fatal("key not saved")
	}
}
