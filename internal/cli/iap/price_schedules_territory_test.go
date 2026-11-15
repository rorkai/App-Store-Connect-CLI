package iap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIAPPriceSchedulesCreateRejectsUnmappableBaseTerritory(t *testing.T) {
	// Without credentials the command can only fail with the territory error if
	// normalization runs before the client is built.
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_PRIVATE_KEY_PATH", "ASC_PROFILE"} {
		t.Setenv(key, "")
	}

	clientRequested := false
	d, stdout, stderrBuf := newTestDeps(func() (*asc.Client, error) {
		clientRequested = true
		return nil, errors.New("client must not be created")
	})
	cmd := newIAPPriceSchedulesCreateCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--iap-id", "9000000003",
		"--base-territory", "Atlantis",
		"--prices", "PP1:2026-03-01",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	execErr := cmd.Exec(context.Background(), cmd.FlagSet.Args())
	stderr := stderrBuf.String()
	if clientRequested {
		t.Fatal("client requested before --base-territory validation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	if execErr == nil {
		t.Fatal("expected unmappable --base-territory to fail before any client use")
	}
	if !strings.Contains(execErr.Error(), `territory "Atlantis" could not be mapped`) {
		t.Fatalf("error = %v, want unmappable territory error", execErr)
	}
	if count := strings.Count(stderr, "Error:"); count != 1 {
		t.Fatalf("stderr = %q, want exactly one diagnostic", stderr)
	}
}
