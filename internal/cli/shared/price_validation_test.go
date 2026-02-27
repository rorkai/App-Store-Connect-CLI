package shared

import "testing"

func TestValidateFinitePriceFlag_Valid(t *testing.T) {
	if err := ValidateFinitePriceFlag("--price", "4.99"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateFinitePriceFlag_Empty(t *testing.T) {
	if err := ValidateFinitePriceFlag("--price", "  "); err != nil {
		t.Fatalf("expected no error for empty value, got %v", err)
	}
}

func TestValidateFinitePriceFlag_InvalidNumber(t *testing.T) {
	err := ValidateFinitePriceFlag("--price", "abc")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "--price must be a number" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestValidateFinitePriceFlag_NonFinite(t *testing.T) {
	err := ValidateFinitePriceFlag("--price", "NaN")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "--price must be a finite number" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
