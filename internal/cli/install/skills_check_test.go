package install

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestSkillsAutoCheckEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "default disabled", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "y", value: "y", want: true},
		{name: "on", value: "on", want: true},
		{name: "one", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "no", value: "no", want: false},
		{name: "n", value: "n", want: false},
		{name: "off", value: "off", want: false},
		{name: "zero", value: "0", want: false},
		{name: "invalid falls back to disabled", value: "maybe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skillsAutoCheckEnabled(tt.value); got != tt.want {
				t.Fatalf("skillsAutoCheckEnabled(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestShouldRunSkillsCheck(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	if !shouldRunSkillsCheck(now, "") {
		t.Fatal("expected empty timestamp to trigger check")
	}
	if !shouldRunSkillsCheck(now, "not-a-time") {
		t.Fatal("expected invalid timestamp to trigger check")
	}

	recent := now.Add(-2 * time.Hour).Format(skillsCheckedAtLayout)
	if shouldRunSkillsCheck(now, recent) {
		t.Fatal("expected recent timestamp to skip check")
	}

	old := now.Add(-26 * time.Hour).Format(skillsCheckedAtLayout)
	if !shouldRunSkillsCheck(now, old) {
		t.Fatal("expected old timestamp to trigger check")
	}
}

func TestSkillsOutputHasUpdates(t *testing.T) {
	if skillsOutputHasUpdates("all skills are up to date") {
		t.Fatal("expected up-to-date output to report no updates")
	}
	if skillsOutputHasUpdates("no update available") {
		t.Fatal("expected singular no-update output to report no updates")
	}
	if !skillsOutputHasUpdates("2 updates available") {
		t.Fatal("expected updates-available output to report updates")
	}
	if !skillsOutputHasUpdates("Update available for find-skills") {
		t.Fatal("expected singular update output to report updates")
	}
}

func TestMaybeCheckForSkillUpdates_NotifiesAndPersistsTimestamp(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origSave := saveConfigForSkillsCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		saveConfigForSkillsCheck = origSave
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")

	cfg := &config.Config{}
	loadConfigForSkillsCheck = func() (*config.Config, error) { return cfg, nil }

	savedAt := ""
	saveConfigForSkillsCheck = func(in *config.Config) error {
		savedAt = strings.TrimSpace(in.SkillsCheckedAt)
		return nil
	}

	fixedNow := time.Date(2026, 3, 5, 12, 30, 0, 0, time.UTC)
	nowForSkillsCheck = func() time.Time { return fixedNow }
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		return "2 updates available", nil
	}
	progressEnabledForCheck = func() bool { return true }

	stderr := captureStderr(t, func() {
		MaybeCheckForSkillUpdates(context.Background())
	})

	if savedAt != fixedNow.Format(skillsCheckedAtLayout) {
		t.Fatalf("SkillsCheckedAt = %q, want %q", savedAt, fixedNow.Format(skillsCheckedAtLayout))
	}
	if !strings.Contains(stderr, "npx skills update") {
		t.Fatalf("expected notification in stderr, got %q", stderr)
	}
}

func TestMaybeCheckForSkillUpdates_SkipsWhenCheckedRecently(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origSave := saveConfigForSkillsCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		saveConfigForSkillsCheck = origSave
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")

	fixedNow := time.Date(2026, 3, 5, 15, 0, 0, 0, time.UTC)
	nowForSkillsCheck = func() time.Time { return fixedNow }
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		return &config.Config{SkillsCheckedAt: fixedNow.Add(-1 * time.Hour).Format(skillsCheckedAtLayout)}, nil
	}
	saveConfigForSkillsCheck = func(in *config.Config) error {
		t.Fatal("save should not be called for recent checks")
		return nil
	}

	called := false
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		called = true
		return "", nil
	}
	progressEnabledForCheck = func() bool { return true }

	MaybeCheckForSkillUpdates(context.Background())
	if called {
		t.Fatal("expected skills check command to be skipped")
	}
}

func TestMaybeCheckForSkillUpdates_SkipsWhenDisabled(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "false")
	progressEnabledForCheck = func() bool { return true }
	loadCalled := false
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		loadCalled = true
		return nil, errors.New("should not load")
	}

	MaybeCheckForSkillUpdates(context.Background())
	if loadCalled {
		t.Fatal("expected config load to be skipped when disabled")
	}
}

func TestMaybeCheckForSkillUpdates_SkipsByDefaultWhenUnset(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "")
	progressEnabledForCheck = func() bool { return true }
	loadCalled := false
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		loadCalled = true
		return nil, errors.New("should not load")
	}

	MaybeCheckForSkillUpdates(context.Background())
	if loadCalled {
		t.Fatal("expected config load to be skipped when auto-check env var is unset")
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		done <- buf.String()
	}()

	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
	}()

	fn()
	_ = w.Close()
	return <-done
}
