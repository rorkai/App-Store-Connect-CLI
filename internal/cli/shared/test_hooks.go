package shared

import (
	"fmt"
	"os"
)

var mkdirTempForTest = os.MkdirTemp

// ResetTierCacheForTest routes tier-cache reads and writes to an isolated temp dir for tests.
func ResetTierCacheForTest() {
	tierCacheDirOverrideMu.Lock()
	override := tierCacheDirOverride
	if override == "" {
		tempDir, err := mkdirTempForTest("", "asc-tier-cache-*")
		if err != nil {
			tierCacheDirOverrideMu.Unlock()
			panic(fmt.Errorf("create isolated tier cache dir: %w", err))
		}
		override = tempDir
		tierCacheDirOverride = override
	}
	tierCacheDirOverrideMu.Unlock()

	if err := os.RemoveAll(override); err != nil {
		panic(fmt.Errorf("reset isolated tier cache dir: %w", err))
	}
	if err := os.MkdirAll(override, 0o755); err != nil {
		panic(fmt.Errorf("recreate isolated tier cache dir: %w", err))
	}
}

func resetTierCacheDirOverrideForTest() {
	tierCacheDirOverrideMu.Lock()
	override := tierCacheDirOverride
	tierCacheDirOverride = ""
	tierCacheDirOverrideMu.Unlock()

	if override == "" {
		return
	}
	_ = os.RemoveAll(override)
}
