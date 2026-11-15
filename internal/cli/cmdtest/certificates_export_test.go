package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestCertificatesExport_MissingRequiredFlags(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, nil, "certificates", "export")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if got, want := err.Error(), "--certificate"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --certificate is required") {
		t.Fatalf("expected missing certificate error, got %q", stderr)
	}
}
