//go:build darwin

package screenshots

func newMacOSProvider() (Provider, error) {
	return &MacOSProvider{}, nil
}
