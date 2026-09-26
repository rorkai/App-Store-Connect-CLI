package iap

import (
	"context"
	"strings"
	"testing"
)

func TestIAPPriceSchedulesCreateBaseTerritoryUsageDocumentsAcceptedForms(t *testing.T) {
	cmd := IAPPriceSchedulesCreateCommand()
	flag := cmd.FlagSet.Lookup("base-territory")
	if flag == nil {
		t.Fatal("expected --base-territory flag")
	}
	for _, want := range []string{"alpha-2", "alpha-3", "English country name"} {
		if !strings.Contains(flag.Usage, want) {
			t.Fatalf("--base-territory usage = %q, want mention of %q", flag.Usage, want)
		}
	}
}

func TestIAPPriceSchedulesCreateRejectsUnmappableBaseTerritory(t *testing.T) {
	// Without credentials the command can only fail with the territory error if
	// normalization runs before the client is built.
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_PRIVATE_KEY_PATH", "ASC_PROFILE"} {
		t.Setenv(key, "")
	}

	cmd := IAPPriceSchedulesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--iap-id", "9000000003",
		"--base-territory", "Atlantis",
		"--prices", "PP1:2026-03-01",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	var execErr error
	stderr := captureIAPLegacy441Stderr(t, func() {
		execErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

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
