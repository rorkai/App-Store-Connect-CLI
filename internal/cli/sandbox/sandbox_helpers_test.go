package sandbox

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestValidateSandboxEmail(t *testing.T) {
	if err := validateSandboxEmail("tester@example.com"); err != nil {
		t.Fatalf("expected valid email, got %v", err)
	}
	if err := validateSandboxEmail("not-an-email"); err == nil {
		t.Fatalf("expected invalid email error")
	}
}

func TestNormalizeSandboxTerritory(t *testing.T) {
	if got, err := NormalizeSandboxTerritoryCode("usa"); err != nil || got != "USA" {
		t.Fatalf("expected USA, got %q err %v", got, err)
	}
	if _, err := NormalizeSandboxTerritoryCode("ZZZ"); err == nil {
		t.Fatalf("expected error for invalid territory")
	}
}

func TestNormalizeSandboxTerritoryFilter(t *testing.T) {
	if got, err := normalizeSandboxTerritoryFilter(""); err != nil || got != "" {
		t.Fatalf("expected empty territory, got %q err %v", got, err)
	}
}

func TestNormalizeSandboxRenewalRate(t *testing.T) {
	if got, err := normalizeSandboxRenewalRate("monthly-renewal-every-one-hour"); err != nil {
		t.Fatalf("expected valid renewal rate, got %v", err)
	} else if got == "" {
		t.Fatalf("expected non-empty renewal rate")
	}
	if _, err := normalizeSandboxRenewalRate("invalid"); err == nil {
		t.Fatalf("expected error for invalid renewal rate")
	}
}

func TestOptionalBool(t *testing.T) {
	var value shared.OptionalBool
	if value.IsSet() {
		t.Fatalf("expected unset optionalBool by default")
	}
	if err := value.Set("true"); err != nil {
		t.Fatalf("expected Set to succeed, got %v", err)
	}
	if !value.IsSet() || !value.Value() {
		t.Fatalf("expected optionalBool to be set to true")
	}
}
