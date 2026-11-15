//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

const (
	migrateLowFDHelperEnv = "ASC_MIGRATE_LOW_FD_HELPER"
	migrateLowFDDirEnv    = "ASC_MIGRATE_LOW_FD_DIR"
)

func TestDiscoverScreenshotPlanStaysWithinLowFileDescriptorLimit(t *testing.T) {
	if os.Getenv(migrateLowFDHelperEnv) == "1" {
		runMigrateLowFDHelper(t)
		return
	}

	screenshotsDir := t.TempDir()
	localeDir := filepath.Join(screenshotsDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	seedPath := filepath.Join(localeDir, "iphone_65_000.png")
	writePNG(t, seedPath, 1242, 2688)
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("ReadFile(seed) error = %v", err)
	}
	for index := 1; index < 64; index++ {
		path := filepath.Join(localeDir, fmt.Sprintf("iphone_65_%03d.png", index))
		if err := os.WriteFile(path, seed, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	command := exec.Command(testBinary, "-test.run=^TestDiscoverScreenshotPlanStaysWithinLowFileDescriptorLimit$")
	command.Env = append(
		os.Environ(),
		migrateLowFDHelperEnv+"=1",
		migrateLowFDDirEnv+"="+screenshotsDir,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("low-FD discovery failed: %v\n%s", err, output)
	}
}

func runMigrateLowFDHelper(t *testing.T) {
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatalf("Getrlimit() error = %v", err)
	}
	if limit.Max < 24 {
		t.Skipf("hard file descriptor limit %d is too low for the test process", limit.Max)
	}
	limit.Cur = 24
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatalf("Setrlimit() error = %v", err)
	}

	plans, _, err := discoverScreenshotPlanForUpload(migrateTestDeps(), os.Getenv(migrateLowFDDirEnv))
	if err != nil {
		t.Fatalf("discoverScreenshotPlanForUpload() error = %v", err)
	}
	defer closeScreenshotPlans(plans)
	if len(plans) == 0 {
		t.Fatal("discoverScreenshotPlanForUpload() returned no plans")
	}
}
