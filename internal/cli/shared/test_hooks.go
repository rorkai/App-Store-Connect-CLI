package shared

import (
	"fmt"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

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

// ResetTierCacheForTest empties dir and routes tier-cache reads and writes to it.
func ResetTierCacheForTest(dir string) {
	tierCacheDirOverrideMu.Lock()
	tierCacheDirOverride = dir
	tierCacheDirOverrideMu.Unlock()

	if err := os.RemoveAll(dir); err != nil {
		panic(fmt.Errorf("reset isolated tier cache dir: %w", err))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

// ResetEquivalentVersionNotesForTest forgets which equivalent-version notes
// were already printed, so each in-process command run reports them again.
func ResetEquivalentVersionNotesForTest() {
	equivalentVersionNoteMu.Lock()
	defer equivalentVersionNoteMu.Unlock()

	equivalentVersionNotes = map[string]struct{}{}
}
