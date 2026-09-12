package cmdtest

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestProfilesLocalInstall_ForceActionIsInstalledWhenNoExisting(t *testing.T) {
	installDir := t.TempDir()
	uuid := "00000000-0000-0000-0000-0000000000AB"

	sourcePath := filepath.Join(t.TempDir(), "profile.mobileprovision")
	sourceBytes := buildMobileprovision(t, uuid, "Test Profile", time.Now().Add(24*time.Hour))
	if err := os.WriteFile(sourcePath, sourceBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(sourcePath) error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"profiles", "local", "install",
			"--path", sourcePath,
			"--install-dir", installDir,
			"--force",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v (stdout=%q)", err, stdout)
	}
	if result.Action != "installed" {
		t.Fatalf("action=%q, want %q", result.Action, "installed")
	}
}

func TestProfilesInspect_JSONIncludesEntitlements(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "profile.mobileprovision")
	sourceBytes := buildMobileprovisionWithEntitlements(
		t,
		"00000000-0000-0000-0000-0000000000AD",
		"Inspect Profile",
		time.Now().Add(24*time.Hour),
		map[string]any{
			"com.apple.developer.family-controls":                       true,
			"com.apple.developer.family-controls.app-and-website-usage": true,
		},
	)
	if err := os.WriteFile(sourcePath, sourceBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(sourcePath) error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"profiles", "inspect",
			"--path", sourcePath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		UUID                  string         `json:"uuid"`
		Name                  string         `json:"name"`
		TeamID                string         `json:"teamId"`
		BundleID              string         `json:"bundleId"`
		ApplicationIdentifier string         `json:"applicationIdentifier"`
		Entitlements          map[string]any `json:"entitlements"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v (stdout=%q)", err, stdout)
	}
	if result.UUID != "00000000-0000-0000-0000-0000000000AD" {
		t.Fatalf("uuid=%q", result.UUID)
	}
	if result.Name != "Inspect Profile" {
		t.Fatalf("name=%q", result.Name)
	}
	if result.TeamID != "TEAM12345" {
		t.Fatalf("teamId=%q", result.TeamID)
	}
	if result.BundleID != "com.example.app" {
		t.Fatalf("bundleId=%q", result.BundleID)
	}
	if result.ApplicationIdentifier != "TEAM12345.com.example.app" {
		t.Fatalf("applicationIdentifier=%q", result.ApplicationIdentifier)
	}
	if got, ok := result.Entitlements["com.apple.developer.family-controls"].(bool); !ok || !got {
		t.Fatalf("expected family-controls entitlement true, got %#v", result.Entitlements["com.apple.developer.family-controls"])
	}
}

func TestProfilesInspect_EntitlementsFlagRendersEntitlementRows(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "profile.mobileprovision")
	sourceBytes := buildMobileprovisionWithEntitlements(
		t,
		"00000000-0000-0000-0000-0000000000AE",
		"Entitlements Profile",
		time.Now().Add(24*time.Hour),
		map[string]any{
			"com.apple.developer.family-controls": true,
		},
	)
	if err := os.WriteFile(sourcePath, sourceBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(sourcePath) error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"profiles", "inspect",
			"--path", sourcePath,
			"--entitlements",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "com.apple.developer.family-controls") {
		t.Fatalf("expected entitlement key in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "true") {
		t.Fatalf("expected entitlement value in output, got %q", stdout)
	}
}

func TestProfilesInspect_MissingPath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "inspect"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--path is required") {
		t.Fatalf("expected --path error, got %q", stderr)
	}
}

func TestProfilesLocalInstall_ForceActionIsReplacedWhenExisting(t *testing.T) {
	installDir := t.TempDir()
	uuid := "00000000-0000-0000-0000-0000000000AC"

	sourcePath := filepath.Join(t.TempDir(), "profile.mobileprovision")
	sourceBytes := buildMobileprovision(t, uuid, "Test Profile", time.Now().Add(24*time.Hour))
	if err := os.WriteFile(sourcePath, sourceBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(sourcePath) error: %v", err)
	}

	// Pre-create the destination file so --force truly overwrites it.
	destPath := filepath.Join(installDir, uuid+".mobileprovision")
	if err := os.WriteFile(destPath, []byte("preexisting"), 0o600); err != nil {
		t.Fatalf("WriteFile(destPath) error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"profiles", "local", "install",
			"--path", sourcePath,
			"--install-dir", installDir,
			"--force",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v (stdout=%q)", err, stdout)
	}
	if result.Action != "replaced" {
		t.Fatalf("action=%q, want %q", result.Action, "replaced")
	}
}

func TestProfilesLocalClean_ConfirmRequiresMode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "local", "clean", "--confirm", "--install-dir", t.TempDir(), "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "at least one clean mode is required") {
		t.Fatalf("expected mode-required error, got %q", stderr)
	}
}

type localProfileItem struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	BundleID  string `json:"bundleId,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Path      string `json:"path"`
	Expired   bool   `json:"expired"`
}

type localSkippedItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type localListResult struct {
	InstallDir string             `json:"installDir"`
	Total      int                `json:"total"`
	Listed     int                `json:"listed"`
	Skipped    int                `json:"skipped"`
	Items      []localProfileItem `json:"items"`

	SkippedItems []localSkippedItem `json:"skippedItems"`
}

func TestProfilesLocalDefaultFollowsActiveXcodeWithoutCombiningStores(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("active Xcode profile directory is macOS-specific")
	}

	developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
	binDir := filepath.Join(developerDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake developer directory: %v", err)
	}
	xcodebuildPath := filepath.Join(binDir, "xcodebuild")
	if err := os.WriteFile(xcodebuildPath, []byte("#!/bin/sh\nprintf 'Xcode 16.0\\nBuild version 16A1\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake xcodebuild: %v", err)
	}
	fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
	xcrunScript := "#!/bin/sh\nif [ \"$1\" = \"--find\" ] && [ \"$2\" = \"xcodebuild\" ]; then\n  printf '%s\\n' \"$DEVELOPER_DIR/usr/bin/xcodebuild\"\n  exit 0\nfi\nexit 2\n"
	if err := os.WriteFile(fakeXcrun, []byte(xcrunScript), 0o700); err != nil {
		t.Fatalf("write fake trusted xcrun: %v", err)
	}
	t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
	t.Setenv("DEVELOPER_DIR", developerDir)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	modernDir := filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles")
	legacyDir := filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles")
	if err := os.MkdirAll(modernDir, 0o755); err != nil {
		t.Fatalf("create modern profile directory: %v", err)
	}
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("create legacy profile directory: %v", err)
	}

	modernUUID := "00000000-0000-0000-0000-000000000016"
	legacyUUID := "00000000-0000-0000-0000-000000000015"
	modernPath := filepath.Join(modernDir, modernUUID+".mobileprovision")
	legacyPath := filepath.Join(legacyDir, legacyUUID+".mobileprovision")
	expiresAt := time.Now().Add(-24 * time.Hour)
	if err := os.WriteFile(modernPath, buildMobileprovision(t, modernUUID, "Modern Profile", expiresAt), 0o600); err != nil {
		t.Fatalf("write modern profile: %v", err)
	}
	if err := os.WriteFile(legacyPath, buildMobileprovision(t, legacyUUID, "Legacy Profile", expiresAt), 0o600); err != nil {
		t.Fatalf("write legacy profile: %v", err)
	}

	run := func(args []string) (string, string, error) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		return stdout, stderr, runErr
	}

	stdout, stderr, err := run([]string{"profiles", "local", "list", "--output", "json"})
	if err != nil {
		t.Fatalf("default list error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty list stderr, got %q", stderr)
	}
	var list localListResult
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("decode list JSON: %v (stdout=%q)", err, stdout)
	}
	if list.InstallDir != modernDir || list.Listed != 1 || len(list.Items) != 1 || list.Items[0].UUID != modernUUID {
		t.Fatalf("default list combined or selected the wrong store: %+v", list)
	}

	_, stderr, err = run([]string{"profiles", "local", "clean", "--expired", "--confirm", "--output", "json"})
	if err != nil {
		t.Fatalf("default clean error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty clean stderr, got %q", stderr)
	}
	if _, err := os.Stat(modernPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("modern profile must be deleted, stat error = %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy profile must remain outside the selected store: %v", err)
	}
}

func TestProfilesLocalDefaultFallsBackToLegacyDirectoryWithoutFullXcode(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("active Xcode profile directory is macOS-specific")
	}

	developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
	binDir := filepath.Join(developerDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake developer directory: %v", err)
	}
	xcodebuildPath := filepath.Join(binDir, "xcodebuild")
	script := "#!/bin/sh\nprintf 'xcode-select: error: tool '\\''xcodebuild'\\'' requires Xcode\\n' >&2\nexit 1\n"
	if err := os.WriteFile(xcodebuildPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake xcodebuild: %v", err)
	}
	fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
	xcrunScript := "#!/bin/sh\nif [ \"$1\" = \"--find\" ] && [ \"$2\" = \"xcodebuild\" ]; then\n  printf '%s\\n' \"$DEVELOPER_DIR/usr/bin/xcodebuild\"\n  exit 0\nfi\nexit 2\n"
	if err := os.WriteFile(fakeXcrun, []byte(xcrunScript), 0o700); err != nil {
		t.Fatalf("write fake trusted xcrun: %v", err)
	}
	t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
	t.Setenv("DEVELOPER_DIR", developerDir)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	legacyDir := filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("create legacy profile directory: %v", err)
	}
	legacyUUID := "00000000-0000-0000-0000-000000000015"
	legacyPath := filepath.Join(legacyDir, legacyUUID+".mobileprovision")
	if err := os.WriteFile(legacyPath, buildMobileprovision(t, legacyUUID, "Legacy Profile", time.Now().Add(24*time.Hour)), 0o600); err != nil {
		t.Fatalf("write legacy profile: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "local", "list", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("profiles local list error = %v, stderr = %q", runErr, stderr)
	}

	var list localListResult
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("decode list JSON: %v (stdout=%q)", err, stdout)
	}
	if list.InstallDir != legacyDir {
		t.Fatalf("install dir = %q, want the legacy default %q", list.InstallDir, legacyDir)
	}
	if list.Listed != 1 || len(list.Items) != 1 || list.Items[0].UUID != legacyUUID {
		t.Fatalf("legacy profiles were not listed: %+v", list)
	}

	if lines := strings.Count(strings.TrimSpace(stderr), "\n") + 1; lines != 1 {
		t.Fatalf("expected a single stderr notice, got %q", stderr)
	}
	for _, want := range []string{"active Xcode", legacyDir, "--install-dir"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr notice %q missing %q", stderr, want)
		}
	}
}

func TestProfilesLocal_InstallListCleanExpired(t *testing.T) {
	run := func(args []string) (string, string, error) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		return stdout, stderr, runErr
	}

	installDir := t.TempDir()

	activeUUID := "00000000-0000-0000-0000-000000000001"
	expiredUUID := "00000000-0000-0000-0000-000000000002"

	activeSource := filepath.Join(t.TempDir(), "active.mobileprovision")
	expiredSource := filepath.Join(t.TempDir(), "expired.mobileprovision")

	activeBytes := buildMobileprovision(t, activeUUID, "Active Profile", time.Now().Add(24*time.Hour))
	expiredBytes := buildMobileprovision(t, expiredUUID, "Expired Profile", time.Now().Add(-24*time.Hour))

	if err := os.WriteFile(activeSource, activeBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(activeSource) error: %v", err)
	}
	if err := os.WriteFile(expiredSource, expiredBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(expiredSource) error: %v", err)
	}

	// Install both profiles.
	_, stderr, err := run([]string{"profiles", "local", "install", "--path", activeSource, "--install-dir", installDir, "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	_, stderr, err = run([]string{"profiles", "local", "install", "--path", expiredSource, "--install-dir", installDir, "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	activeInstalled := filepath.Join(installDir, activeUUID+".mobileprovision")
	expiredInstalled := filepath.Join(installDir, expiredUUID+".mobileprovision")

	// List should include both.
	stdout, stderr, err := run([]string{"profiles", "local", "list", "--install-dir", installDir, "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var list localListResult
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("decode list JSON: %v (stdout=%q)", err, stdout)
	}
	if list.Listed != 2 || len(list.Items) != 2 {
		t.Fatalf("expected 2 profiles, got listed=%d len(items)=%d", list.Listed, len(list.Items))
	}
	if list.Skipped != 0 || len(list.SkippedItems) != 0 {
		t.Fatalf("expected 0 skipped, got skipped=%d len(skippedItems)=%d", list.Skipped, len(list.SkippedItems))
	}

	// Dry-run clean should plan deletion but not delete.
	_, stderr, err = run([]string{"profiles", "local", "clean", "--install-dir", installDir, "--expired", "--dry-run", "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if _, err := os.Stat(expiredInstalled); err != nil {
		t.Fatalf("expected expired profile to still exist after dry-run, stat error: %v", err)
	}

	// Clean without --confirm should be a usage error.
	_, stderr, err = run([]string{"profiles", "local", "clean", "--install-dir", installDir, "--expired", "--output", "json"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected confirm required error, got %q", stderr)
	}

	// Confirmed clean should delete the expired one only.
	_, stderr, err = run([]string{"profiles", "local", "clean", "--install-dir", installDir, "--expired", "--confirm", "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if _, err := os.Stat(expiredInstalled); err == nil {
		t.Fatalf("expected expired profile to be deleted")
	}
	if _, err := os.Stat(activeInstalled); err != nil {
		t.Fatalf("expected active profile to remain, stat error: %v", err)
	}
}

func TestProfilesLocalList_SkipsUnreadableProfilesAndReports(t *testing.T) {
	run := func(args []string) (string, string, error) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		return stdout, stderr, runErr
	}

	installDir := t.TempDir()

	goodUUID := "00000000-0000-0000-0000-000000000003"
	goodSource := filepath.Join(t.TempDir(), "good.mobileprovision")
	goodBytes := buildMobileprovision(t, goodUUID, "Good Profile", time.Now().Add(24*time.Hour))
	if err := os.WriteFile(goodSource, goodBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(goodSource) error: %v", err)
	}

	_, stderr, err := run([]string{"profiles", "local", "install", "--path", goodSource, "--install-dir", installDir, "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	// Drop an unreadable/corrupt profile file into the install dir.
	badPath := filepath.Join(installDir, "bad.mobileprovision")
	if err := os.WriteFile(badPath, []byte("not a valid profile"), 0o600); err != nil {
		t.Fatalf("WriteFile(badPath) error: %v", err)
	}

	stdout, stderr, err := run([]string{"profiles", "local", "list", "--install-dir", installDir, "--output", "json"})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var list localListResult
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("decode list JSON: %v (stdout=%q)", err, stdout)
	}
	if list.Total != 2 {
		t.Fatalf("expected total=2, got %d", list.Total)
	}
	if list.Listed != 1 || len(list.Items) != 1 {
		t.Fatalf("expected listed=1, got listed=%d len(items)=%d", list.Listed, len(list.Items))
	}
	if list.Skipped != 1 || len(list.SkippedItems) != 1 {
		t.Fatalf("expected skipped=1, got skipped=%d len(skippedItems)=%d", list.Skipped, len(list.SkippedItems))
	}
	if filepath.Clean(list.SkippedItems[0].Path) != filepath.Clean(badPath) {
		t.Fatalf("expected skipped path %q, got %q", badPath, list.SkippedItems[0].Path)
	}
	if strings.TrimSpace(list.SkippedItems[0].Reason) == "" {
		t.Fatalf("expected skipped reason, got empty")
	}
}

func TestProfilesLocalInstall_ByID_DownloadsAndInstalls(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	uuid := "00000000-0000-0000-0000-0000000000AA"
	content := buildMobileprovision(t, uuid, "Downloaded Profile", time.Now().Add(24*time.Hour))
	b64 := base64.StdEncoding.EncodeToString(content)

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.appstoreconnect.apple.com" {
			t.Fatalf("unexpected host: %s", req.URL.Host)
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}

		body := `{"data":{"type":"profiles","id":"p1","attributes":{"profileContent":"` + b64 + `"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	installDir := t.TempDir()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "local", "install", "--id", "p1", "--install-dir", installDir, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	destPath := filepath.Join(installDir, uuid+".mobileprovision")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile(destPath) error: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("expected installed profile bytes to match downloaded bytes")
	}
}

func buildMobileprovision(t *testing.T, uuid, name string, expires time.Time) []byte {
	t.Helper()

	return buildMobileprovisionWithEntitlements(t, uuid, name, expires, nil)
}

func buildMobileprovisionWithEntitlements(t *testing.T, uuid, name string, expires time.Time, extraEntitlements map[string]any) []byte {
	t.Helper()

	const teamID = "TEAM12345"
	const bundleID = "com.example.app"
	now := time.Now().UTC()
	entitlements := map[string]any{
		"application-identifier":              teamID + "." + bundleID,
		"com.apple.developer.team-identifier": teamID,
	}
	for key, value := range extraEntitlements {
		entitlements[key] = value
	}
	payload := map[string]any{
		"UUID":           uuid,
		"Name":           name,
		"TeamIdentifier": []string{teamID},
		"CreationDate":   now.Add(-1 * time.Hour),
		"ExpirationDate": expires.UTC(),
		"Entitlements":   entitlements,
	}

	plistBytes, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}

	cert, key := selfSignedCert(t)
	sd, err := pkcs7.NewSignedData(plistBytes)
	if err != nil {
		t.Fatalf("pkcs7.NewSignedData() error: %v", err)
	}
	if err := sd.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("SignedData.AddSigner() error: %v", err)
	}
	out, err := sd.Finish()
	if err != nil {
		t.Fatalf("SignedData.Finish() error: %v", err)
	}
	return out
}

func selfSignedCert(t *testing.T) (*x509.Certificate, crypto.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error: %v", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int() error: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "Test Signer",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() error: %v", err)
	}
	return cert, key
}
