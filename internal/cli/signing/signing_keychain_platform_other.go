//go:build !darwin

package signing

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

func platformSigningKeychainInstallDeps(*shared.Deps, bool) signingKeychainInstallDeps {
	return signingKeychainInstallDeps{GOOS: "unsupported"}
}
