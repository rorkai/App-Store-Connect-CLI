//go:build darwin

package signing

import "context"

func defaultRunKeychainSecurity(ctx context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	return runSigningUtility(ctx, stdin, args...)
}
