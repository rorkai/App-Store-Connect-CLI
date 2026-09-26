package signing

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestProfileCreateNameForTargetTruncatesLongBundleIDsUniquely(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	shared := strings.Repeat("com.company.product.suite.feature.extension.", 4)
	leftBundle := shared + strings.Repeat("l", 24)
	rightBundle := shared + strings.Repeat("r", 24)
	if utf8.RuneCountInString(leftBundle) != 200 || utf8.RuneCountInString(rightBundle) != 200 {
		t.Fatalf("test fixtures = %d and %d characters, want 200", utf8.RuneCountInString(leftBundle), utf8.RuneCountInString(rightBundle))
	}
	left := profileCreateNameForTarget("IOS_APP_DEVELOPMENT", leftBundle, now)
	right := profileCreateNameForTarget("IOS_APP_DEVELOPMENT", rightBundle, now)

	if got := utf8.RuneCountInString(left); got > maxProfileNameLength {
		t.Fatalf("left name exceeds %d characters: %q (%d)", maxProfileNameLength, left, got)
	}
	if got := utf8.RuneCountInString(right); got > maxProfileNameLength {
		t.Fatalf("right name exceeds %d characters: %q (%d)", maxProfileNameLength, right, got)
	}
	if left == right {
		t.Fatalf("sibling bundle IDs collapsed to %q", left)
	}
	if !strings.HasPrefix(left, "IOS_APP_DEVELOPMENT-20260913-") {
		t.Fatalf("left = %q, want the type/date prefix", left)
	}
}

func TestValidateProfileNameLength(t *testing.T) {
	tests := []struct {
		name      string
		wantError string
	}{
		{name: strings.Repeat("a", maxProfileNameLength)},
		{name: strings.Repeat("€", maxProfileNameLength)},
		{name: strings.Repeat("a", maxProfileNameLength+1), wantError: "got 65"},
		{name: strings.Repeat("€", maxProfileNameLength+1), wantError: "got 65"},
	}
	for _, test := range tests {
		err := ValidateProfileNameLength(test.name)
		if test.wantError == "" {
			if err != nil {
				t.Errorf("ValidateProfileNameLength(%q) = %v, want nil", test.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), test.wantError) || !strings.Contains(err.Error(), "at most 64") {
			t.Errorf("ValidateProfileNameLength(%q) = %v, want a 64-character limit with %q", test.name, err, test.wantError)
		}
	}
}

func TestProfileCreateNameForTargetTruncatesOnRuneBoundaries(t *testing.T) {
	name := profileCreateNameForTarget(
		"IOS_APP_DEVELOPMENT",
		strings.Repeat("€", 200),
		time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	)
	if !utf8.ValidString(name) {
		t.Fatalf("generated profile name is not valid UTF-8: %q", name)
	}
	if got := utf8.RuneCountInString(name); got > maxProfileNameLength {
		t.Fatalf("generated profile name has %d characters, want <= %d: %q", got, maxProfileNameLength, name)
	}
}
