//go:build (!darwin && !linux) || !cgo || !fido2

package securitykey

import (
	"context"
)

// Available reports whether this build includes the native signer.
func Available() bool { return false }

// Sign obtains an assertion when the native backend is compiled in.
func Sign(context.Context, Request, string) (*Assertion, error) {
	return nil, ErrUnavailable
}
