//go:build !darwin

package signing

import (
	"context"
	"fmt"
)

func defaultRunKeychainSecurity(context.Context, []byte, ...string) ([]byte, []byte, error) {
	return nil, nil, fmt.Errorf("signing keychain is supported only on macOS")
}
