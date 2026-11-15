//go:build !darwin

package signing

func platformSigningKeychainInstallDeps(bool) signingKeychainInstallDeps {
	return signingKeychainInstallDeps{GOOS: "unsupported"}
}
