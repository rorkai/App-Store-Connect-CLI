package subscriptions

import (
	"strings"
	"testing"
)

func TestSubscriptionsAvailabilityCommandMentionsAPIDeprecation(t *testing.T) {
	cmd := SubscriptionsAvailabilityCommand()

	if !strings.Contains(cmd.ShortHelp, "deprecated") {
		t.Fatalf("expected ShortHelp to mention deprecation, got %q", cmd.ShortHelp)
	}
	if !strings.Contains(cmd.LongHelp, "Subscription plan availability") {
		t.Fatalf("expected LongHelp to point at Subscription plan availability, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "asc subscriptions pricing monthly-commitment") {
		t.Fatalf("expected LongHelp to reference the plan-availability commands, got %q", cmd.LongHelp)
	}
}
