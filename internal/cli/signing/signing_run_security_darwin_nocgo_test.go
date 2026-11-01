//go:build darwin && !cgo

package signing

import (
	"context"
	"strings"
	"testing"
)

func TestSigningRunRejectsNoCGOBeforeReadingInputs(t *testing.T) {
	err := executeSigningOperation(context.Background(), signingRunOptions{}, func(context.Context) error {
		t.Fatal("signing operation must not run without Security.framework support")
		return nil
	})
	if !strings.Contains(err.Error(), "cgo-enabled macOS build") {
		t.Fatalf("error = %v, want cgo capability diagnostic", err)
	}
}

func TestSigningRunSecurityFrameworkRequiresCGO(t *testing.T) {
	for _, err := range []error{
		createKeychainWithSecurityFramework("unused", nil),
		importPKCS12WithSecurityFramework("unused", nil, nil),
		importLegacyPKCS12WithSecurityFramework("unused", nil, nil),
	} {
		if err == nil || !strings.Contains(err.Error(), "requires a cgo-enabled macOS build") {
			t.Fatalf("error = %v, want cgo requirement", err)
		}
	}
}
