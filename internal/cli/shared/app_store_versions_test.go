package shared

import (
	"strings"
	"testing"
)

func TestNormalizeAppStoreVersionStatesAcceptsReadyForDistribution(t *testing.T) {
	states, err := NormalizeAppStoreVersionStates([]string{"READY_FOR_DISTRIBUTION"})
	if err != nil {
		t.Fatalf("NormalizeAppStoreVersionStates() error = %v", err)
	}
	if len(states) != 1 || states[0] != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("NormalizeAppStoreVersionStates() = %#v, want READY_FOR_DISTRIBUTION", states)
	}
}

func TestAppStoreVersionStateListMentionsReadyForDistribution(t *testing.T) {
	_, err := NormalizeAppStoreVersionStates([]string{"NOPE"})
	if err == nil {
		t.Fatal("expected unsupported state error, got nil")
	}
	if !strings.Contains(err.Error(), "READY_FOR_DISTRIBUTION") {
		t.Fatalf("expected error to mention READY_FOR_DISTRIBUTION, got %q", err.Error())
	}
}

func TestValidateAppStoreVersionStateFilterCombinationRejectsMixedFilterOnlyStates(t *testing.T) {
	err := ValidateAppStoreVersionStateFilterCombination([]string{"READY_FOR_SALE", "READY_FOR_DISTRIBUTION"})
	if err == nil {
		t.Fatal("expected mixed filter-only states to be rejected")
	}
	if !strings.Contains(err.Error(), "READY_FOR_SALE") {
		t.Fatalf("expected error to mention appStoreState-only state, got %q", err.Error())
	}
}

func TestValidateAppStoreVersionStateFilterCombinationAllowsCompatibleVersionStates(t *testing.T) {
	if err := ValidateAppStoreVersionStateFilterCombination([]string{"READY_FOR_REVIEW", "READY_FOR_DISTRIBUTION"}); err != nil {
		t.Fatalf("ValidateAppStoreVersionStateFilterCombination() error = %v", err)
	}
}
