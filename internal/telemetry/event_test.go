package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildEventSanitizesCommand(t *testing.T) {
	clearContextEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	ev, ok := BuildEvent(
		[]string{"apps", "info", "edit", "--app", "123456789", "--bundle-id", "com.secret.app", "--issuer-id", "issuer-secret"},
		"asc apps info edit",
		"1.2.3",
		450*time.Millisecond,
		0,
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.CommandPath != "asc apps info edit" {
		t.Fatalf("CommandPath = %q", ev.CommandPath)
	}
	if ev.CommandFamily != "apps" {
		t.Fatalf("CommandFamily = %q", ev.CommandFamily)
	}
	if ev.DurationBucket != "100ms_500ms" {
		t.Fatalf("DurationBucket = %q", ev.DurationBucket)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, forbidden := range []string{"123456789", "com.secret.app", "issuer-secret", "--bundle-id", "--issuer-id"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("event leaked %q: %s", forbidden, data)
		}
	}
}

func TestBuildEventOmitsInstallIDForNonLocalContext(t *testing.T) {
	clearContextEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CI", "true")

	ev, ok := BuildEvent([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Second, 1)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.ExecutionContext != ContextCI {
		t.Fatalf("ExecutionContext = %q", ev.ExecutionContext)
	}
	if ev.InstallID != nil {
		t.Fatalf("expected nil install ID for CI, got %q", *ev.InstallID)
	}
}

func TestBuildEventDoesNotWaitForInstallIDLock(t *testing.T) {
	clearContextEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	unlock, err := lockState(path, lockTimeout)
	if err != nil {
		t.Fatalf("lockState() error = %v", err)
	}
	defer unlock()

	start := time.Now()
	ev, ok := BuildEvent([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Second, 0)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected event")
	}
	if ev.InstallID != nil {
		t.Fatalf("expected nil install ID while state is locked, got %q", *ev.InstallID)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("BuildEvent() elapsed = %s, want lock contention skipped before 500ms", elapsed)
	}
}

func TestBuildEventSkipsControlCommands(t *testing.T) {
	for _, commandPath := range []string{"asc", "asc completion", "asc version", "asc telemetry", "asc telemetry status"} {
		t.Run(commandPath, func(t *testing.T) {
			if _, ok := BuildEvent(nil, commandPath, "1.2.3", 0, 0); ok {
				t.Fatalf("expected %q to be skipped", commandPath)
			}
		})
	}
}

func TestNegativeDurationIsClamped(t *testing.T) {
	if got := durationMillis(-time.Second); got != 0 {
		t.Fatalf("durationMillis() = %d, want 0", got)
	}
	if got := durationBucket(-time.Second); got != "lt_100ms" {
		t.Fatalf("durationBucket() = %q, want %q", got, "lt_100ms")
	}
}
