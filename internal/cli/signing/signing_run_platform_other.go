//go:build !darwin

package signing

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func platformSigningRunDeps(d *shared.Deps) signingRunDeps {
	return signingRunDeps{GOOS: "unsupported", Stderr: d.IO.ErrOut}
}

func signingRunSecurityAvailable() bool { return false }

func systemSigningRunRoots() (*x509.CertPool, error) { return x509.SystemCertPool() }

func platformSigningRunContext(ctx context.Context) (context.Context, func()) {
	return ctx, func() {}
}

func runSigningRunChild(context.Context, func(time.Duration) <-chan time.Time, []string) error {
	return shared.NewValidationError(fmt.Errorf("signing run is supported only on macOS"))
}
