package shared

import (
	"fmt"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var mkdirTempForTest = os.MkdirTemp

// SetASCClientFactoryForTesting replaces the general command ASC client factory.
// It returns a restore function to reset the previous handler.
func SetASCClientFactoryForTesting(fn func() (*asc.Client, error)) func() {
	ascClientFactoryMu.Lock()
	previous := ascClientFactory
	if fn == nil {
		ascClientFactory = getASCClient
	} else {
		ascClientFactory = fn
	}
	ascClientFactoryMu.Unlock()
	return func() {
		ascClientFactoryMu.Lock()
		ascClientFactory = previous
		ascClientFactoryMu.Unlock()
	}
}

// SetAvailabilityClientFactory replaces the ASC client factory for availability tests.
// It returns a restore function to reset the previous handler.
func SetAvailabilityClientFactory(fn func() (*asc.Client, error)) func() {
	previous := availabilityClientFactory
	if fn == nil {
		availabilityClientFactory = getASCClient
	} else {
		availabilityClientFactory = fn
	}
	return func() {
		availabilityClientFactory = previous
	}
}

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
