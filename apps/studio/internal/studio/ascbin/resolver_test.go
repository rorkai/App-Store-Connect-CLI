package ascbin

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolvePrefersBundledBinary(t *testing.T) {
	tmp := t.TempDir()
	bundled := filepath.Join(tmp, "bundled-asc")
	if err := os.WriteFile(bundled, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Resolve(ResolveOptions{
		BundledPath:   bundled,
		PreferBundled: true,
		LookPath: func(string) (string, error) {
			return "/usr/local/bin/asc", nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source != "bundled" {
		t.Fatalf("Source = %q, want bundled", got.Source)
	}
	if got.Path != bundled {
		t.Fatalf("Path = %q, want %q", got.Path, bundled)
	}
}

func TestResolveUsesSystemOverrideBeforeBundled(t *testing.T) {
	tmp := t.TempDir()
	override := filepath.Join(tmp, "system-asc")
	if err := os.WriteFile(override, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Resolve(ResolveOptions{
		BundledPath:    filepath.Join(tmp, "bundled-asc"),
		SystemOverride: override,
		PreferBundled:  true,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source != "system-override" {
		t.Fatalf("Source = %q, want system-override", got.Source)
	}
}

func TestResolveFallsBackToPathLookup(t *testing.T) {
	got, err := Resolve(ResolveOptions{
		PreferBundled: false,
		LookPath: func(string) (string, error) {
			return "/opt/homebrew/bin/asc", nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source != "path" {
		t.Fatalf("Source = %q, want path", got.Source)
	}
}

func TestResolveErrorsWhenNothingAvailable(t *testing.T) {
	_, err := Resolve(ResolveOptions{
		LookPath: func(string) (string, error) {
			return "", execErrNotFound
		},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
}

func TestResolveFallsBackToBundledWhenLookPathReturnsExecErrNotFound(t *testing.T) {
	tmp := t.TempDir()
	bundled := filepath.Join(tmp, "bundled-asc")
	if err := os.WriteFile(bundled, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Resolve(ResolveOptions{
		BundledPath:   bundled,
		PreferBundled: false,
		LookPath: func(string) (string, error) {
			return "", exec.ErrNotFound
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source != "bundled-fallback" {
		t.Fatalf("Source = %q, want bundled-fallback", got.Source)
	}
	if got.Path != bundled {
		t.Fatalf("Path = %q, want %q", got.Path, bundled)
	}
}

func TestResolveReturnsOverrideErrorWhenMissing(t *testing.T) {
	_, err := Resolve(ResolveOptions{
		SystemOverride: "/missing/asc",
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !errors.Is(err, os.ErrNotExist) && err.Error() == "" {
		t.Fatalf("Resolve() error = %v, want descriptive error", err)
	}
}
