// Package securitykey obtains assertions from attached hardware authenticators.
package securitykey

import (
	"errors"
	"time"
)

// AssertionTimeout bounds a hardware assertion independently of HTTP requests.
const AssertionTimeout = time.Minute

// ErrUnavailable identifies a build without the optional native backend.
var ErrUnavailable = errors.New("security-key support requires an opt-in macOS or Linux build with CGO_ENABLED=1 and -tags fido2 (install libfido2 1.14 or newer and pkg-config first); Windows is unavailable")

// Request contains the relying party, hash of the exact client-data JSON, and
// server-provided credential allow-list. It never creates credentials.
type Request struct {
	RelyingParty   string
	ClientDataHash [32]byte
	CredentialIDs  [][]byte
}

// Assertion contains raw WebAuthn bytes, without CBOR or base64 wrapping.
type Assertion struct {
	CredentialID      []byte
	AuthenticatorData []byte
	Signature         []byte
	UserID            []byte
}
