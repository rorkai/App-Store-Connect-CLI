//go:build (darwin || linux) && cgo && fido2

package securitykey

import (
	"context"
	"errors"
	"testing"
)

// Tagged builds must reach the native signer on each supported OS, while a
// cancelled request must stop before discovering or opening a physical key.
func TestSignCancelledBeforeDeviceDiscovery(t *testing.T) {
	if !Available() {
		t.Fatal("tagged macOS and Linux builds must include the native security-key signer")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assertion, err := Sign(ctx, Request{
		RelyingParty:  "apple.com",
		CredentialIDs: [][]byte{{1}},
	}, "")
	if !errors.Is(err, context.Canceled) || assertion != nil {
		t.Fatalf("cancelled assertion = %+v, %v; want nil, context.Canceled", assertion, err)
	}
}
