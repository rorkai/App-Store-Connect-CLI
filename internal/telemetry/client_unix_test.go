//go:build darwin || linux

package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestEmitHonorsEnvironmentOptOutBeforeStateRead(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	t.Setenv("DO_NOT_TRACK", "")

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("create blocking state file: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		Emit("asc builds list", "1.2.3", time.Millisecond, 0)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		writer, err := os.OpenFile(path, os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("open state writer to unblock Emit(): %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close state writer: %v", err)
		}
		<-done
		t.Fatal("Emit() read telemetry state despite the environment opt-out")
	}
}

func TestEmitSkipsIgnoredCommandBeforeStateRead(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("create blocking state file: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		Emit("asc telemetry status", "1.2.3", time.Millisecond, 0)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		writer, err := os.OpenFile(path, os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("open state writer to unblock Emit(): %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close state writer: %v", err)
		}
		<-done
		t.Fatal("Emit() read telemetry state for an ignored command")
	}
}
