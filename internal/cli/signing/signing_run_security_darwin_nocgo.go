//go:build darwin && !cgo

package signing

import "fmt"

func signingRunSecurityAvailable() bool { return false }

func createKeychainWithSecurityFramework(string, []byte) error {
	return fmt.Errorf("signing run requires a cgo-enabled macOS build")
}

func createPersistentKeychainWithSecurityFramework(string, []byte) (bool, error) {
	return false, fmt.Errorf("signing run requires a cgo-enabled macOS build")
}

func importPKCS12WithSecurityFramework(string, []byte, []byte) error {
	return fmt.Errorf("signing run requires a cgo-enabled macOS build")
}

func importLegacyPKCS12WithSecurityFramework(string, []byte, []byte) error {
	return fmt.Errorf("signing run requires a cgo-enabled macOS build")
}
