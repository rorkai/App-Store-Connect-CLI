package signing

import (
	"strings"
	"testing"
	"time"
)

func TestProfileCreateNameForTargetTruncatesLongBundleIDsUniquely(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	shared := strings.Repeat("com.company.product.suite.feature.extension.", 4)
	left := profileCreateNameForTarget("IOS_APP_DEVELOPMENT", shared+"name-left", now)
	right := profileCreateNameForTarget("IOS_APP_DEVELOPMENT", shared+"name-right", now)

	if len(left) > maxProfileNameLength || len(right) > maxProfileNameLength {
		t.Fatalf("names exceed %d: %q (%d) %q (%d)", maxProfileNameLength, left, len(left), right, len(right))
	}
	if left == right {
		t.Fatalf("sibling bundle IDs collapsed to %q", left)
	}
	if !strings.HasPrefix(left, "IOS_APP_DEVELOPMENT-20260913-") {
		t.Fatalf("left = %q, want the type/date prefix", left)
	}
}

func TestValidateProfileNameLength(t *testing.T) {
	if err := ValidateProfileNameLength(strings.Repeat("a", maxProfileNameLength)); err != nil {
		t.Fatalf("max length should be accepted: %v", err)
	}
	err := ValidateProfileNameLength(strings.Repeat("a", maxProfileNameLength+1))
	if err == nil || !strings.Contains(err.Error(), "at most 64") {
		t.Fatalf("error = %v, want a 64-character limit", err)
	}
}
