//go:build !windows

package signing

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIOSPackageSIGTERMStopsSignerAndCleansStage(t *testing.T) {
	if os.Getenv("ASC_IOS_SIGNAL_HELPER") == "1" {
		_, err := PackageIOSApp(context.Background(), testSigningDeps(), IOSPackageOptions{
			AppPath: os.Getenv("ASC_IOS_SIGNAL_APP"), IPAPath: os.Getenv("ASC_IOS_SIGNAL_IPA"), AdHoc: true, LogWriter: io.Discard,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
		return
	}
	directory, app := newIOSPackageFixture(t)
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\npwd > \"$TMPDIR/stage.path\"\necho $$ > \"$TMPDIR/signer.pid\"\nexec /bin/sleep 60\n"
	if err := os.WriteFile(filepath.Join(tools, "rcodesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ipa := filepath.Join(directory, "App.ipa")
	t.Setenv("ASC_IOS_SIGNAL_HELPER", "1")
	t.Setenv("ASC_IOS_SIGNAL_APP", app)
	t.Setenv("ASC_IOS_SIGNAL_IPA", ipa)
	t.Setenv("TMPDIR", directory)
	t.Setenv("PATH", tools)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestIOSPackageSIGTERMStopsSignerAndCleansStage$")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() { _ = command.Process.Kill() }()
	var pid int
	for pid == 0 {
		data, _ := os.ReadFile(filepath.Join(directory, "signer.pid"))
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if pid != 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("helper exited before signer started: %v", err)
		case <-ctx.Done():
			t.Fatal("signer did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }()
	stage, err := os.ReadFile(filepath.Join(directory, "stage.path"))
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("SIGTERM must return through cancellation and cleanup: %v", err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("signer survived cancellation: %v", err)
	}
	if _, err := os.Stat(strings.TrimSpace(string(stage))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private stage retained after SIGTERM: %v", err)
	}
	if _, err := os.Stat(ipa); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled signer published an IPA: %v", err)
	}
}
