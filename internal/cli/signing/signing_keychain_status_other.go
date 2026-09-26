//go:build !darwin || !cgo

package signing

import "fmt"

func defaultKeychainLockState(string) (bool, bool, error) {
	return false, false, fmt.Errorf("reading keychain lock state requires a cgo-enabled macOS build")
}
