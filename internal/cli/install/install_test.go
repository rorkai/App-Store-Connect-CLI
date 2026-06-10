package install

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestInstallSkillsRunsNpxSkillsAdd(t *testing.T) {
	originalLookup := lookupNpx
	originalRun := runCommand
	t.Cleanup(func() {
		lookupNpx = originalLookup
		runCommand = originalRun
	})

	lookupNpx = func(name string) (string, error) {
		if name != "npx" {
			t.Fatalf("expected lookup for npx, got %q", name)
		}
		return "/bin/npx", nil
	}

	var gotName string
	var gotArgs []string
	runCommand = func(ctx context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string{}, args...)
		return nil
	}

	cmd := InstallSkillsCommand()
	if err := cmd.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if gotName != "/bin/npx" {
		t.Fatalf("expected npx path /bin/npx, got %q", gotName)
	}
	expected := []string{"--yes", "skills", "add", defaultSkillsPackage, "--global", "--agent", "codex", "--yes"}
	if !reflect.DeepEqual(gotArgs, expected) {
		t.Fatalf("expected args %v, got %v", expected, gotArgs)
	}
}

func TestInstallSkillsAppliesASCTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1m")

	originalLookup := lookupNpx
	originalRun := runCommand
	t.Cleanup(func() {
		lookupNpx = originalLookup
		runCommand = originalRun
	})

	lookupNpx = func(name string) (string, error) {
		return "/bin/npx", nil
	}

	runCommand = func(ctx context.Context, name string, args ...string) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected install command context to have a deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > time.Minute {
			t.Fatalf("expected ASC_TIMEOUT deadline within 1m, got %s", remaining)
		}
		return nil
	}

	cmd := InstallSkillsCommand()
	if err := cmd.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestInstallSkillsFailsWhenNpxMissing(t *testing.T) {
	originalLookup := lookupNpx
	originalRun := runCommand
	t.Cleanup(func() {
		lookupNpx = originalLookup
		runCommand = originalRun
	})

	lookupNpx = func(name string) (string, error) {
		return "", errors.New("missing")
	}
	runCommand = func(ctx context.Context, name string, args ...string) error {
		t.Fatal("runCommand should not be called when npx is missing")
		return nil
	}

	cmd := InstallSkillsCommand()
	if err := cmd.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errNpxNotFound) {
		t.Fatalf("expected npx error, got %q", err.Error())
	}
}
