//go:build !darwin

package screenshots

import "fmt"

func newMacOSProvider() (Provider, error) {
	return nil, fmt.Errorf("provider %q is only supported on macOS", ProviderMacOS)
}
