package subscriptions

import (
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsPricesListCommand_HasResolvedFlag(t *testing.T) {
	cmd := SubscriptionsPricesListCommand()

	if cmd.FlagSet.Lookup("resolved") == nil {
		t.Fatal("expected --resolved flag")
	}
	if !strings.Contains(cmd.LongHelp, "--resolved") {
		t.Fatalf("expected long help to mention --resolved, got %q", cmd.LongHelp)
	}
}

func TestMergeSubscriptionPricesPlanTypePreservesRelativeNextURL(t *testing.T) {
	next := "/v1/subscriptions/sub-1/prices?cursor=next"
	merged, err := mergeSubscriptionPricesPlanType(next, asc.SubscriptionPlanTypeMonthly)
	if err != nil {
		t.Fatalf("mergeSubscriptionPricesPlanType() error = %v", err)
	}
	parsed, err := url.Parse(merged)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Path != "/v1/subscriptions/sub-1/prices" || parsed.Query().Get("cursor") != "next" {
		t.Fatalf("relative next URL changed unexpectedly: %q", merged)
	}
	if got := parsed.Query().Get("filter[planType]"); got != "MONTHLY" {
		t.Fatalf("expected filter[planType]=MONTHLY, got %q", got)
	}
}

func TestMergeSubscriptionPricesPlanTypeLeavesUnfilteredRelativeNextURLUntouched(t *testing.T) {
	next := "/v1/subscriptions/sub-1/prices?cursor=next"
	merged, err := mergeSubscriptionPricesPlanType(next, "")
	if err != nil {
		t.Fatalf("mergeSubscriptionPricesPlanType() error = %v", err)
	}
	if merged != next {
		t.Fatalf("expected unfiltered relative next URL to be unchanged, got %q", merged)
	}
}
