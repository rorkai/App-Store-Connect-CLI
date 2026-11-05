package subscriptions

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionPricePointEqualizationCommandsExpose441Filters(t *testing.T) {
	for _, cmd := range []*ffcli.Command{
		SubscriptionsPricePointsEqualizationsCommand(),
		SubscriptionsPricePointsAdjustedEqualizationsCommand(),
	} {
		for _, name := range []string{
			"territory",
			"subscription-id",
			"upfront-price-point-id",
			"plan-type",
			"fields",
			"territory-fields",
			"include",
			"limit",
			"next",
			"paginate",
		} {
			if cmd.FlagSet.Lookup(name) == nil {
				t.Fatalf("%s: expected --%s flag", cmd.Name, name)
			}
		}
	}

	parent := SubscriptionsPricePointsCommand()
	found := false
	for _, subcommand := range parent.Subcommands {
		if subcommand.Name == "adjusted-equalizations" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected adjusted-equalizations under price-points")
	}

	list := SubscriptionsPricePointsListCommand()
	for _, name := range []string{"upfront-price-point-id", "plan-type", "fields", "territory-fields", "include"} {
		if list.FlagSet.Lookup(name) == nil {
			t.Fatalf("list: expected --%s flag", name)
		}
	}
}

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

func TestSubscriptionPriceMatchesTargetComparesDefaultScheduleAndPreserved(t *testing.T) {
	price := asc.Resource[asc.SubscriptionPriceAttributes]{
		ID:         "existing-price-1",
		Type:       "subscriptionPrices",
		Attributes: asc.SubscriptionPriceAttributes{},
		Relationships: json.RawMessage(`{
			"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PP_ID"}},
			"territory":{"data":{"type":"territories","id":"USA"}}
		}`),
	}

	if !subscriptionPriceMatchesTarget(price, "PP_ID", "USA", asc.SubscriptionPriceCreateAttributes{}) {
		t.Fatal("expected default price to match default requested state")
	}

	price.Attributes.StartDate = "2026-05-01"
	if subscriptionPriceMatchesTarget(price, "PP_ID", "USA", asc.SubscriptionPriceCreateAttributes{}) {
		t.Fatal("expected scheduled price not to match omitted start date")
	}
	price.Attributes.StartDate = ""

	price.Attributes.Preserved = true
	if subscriptionPriceMatchesTarget(price, "PP_ID", "USA", asc.SubscriptionPriceCreateAttributes{}) {
		t.Fatal("expected preserved price not to match omitted preserved flag")
	}
	price.Attributes.Preserved = false

	price.Attributes.PlanType = asc.SubscriptionPlanTypeMonthly
	if subscriptionPriceMatchesTarget(price, "PP_ID", "USA", asc.SubscriptionPriceCreateAttributes{}) {
		t.Fatal("expected monthly plan price not to match omitted upfront plan type")
	}
	price.Attributes.PlanType = asc.SubscriptionPlanTypeUpfront
	if !subscriptionPriceMatchesTarget(price, "PP_ID", "USA", asc.SubscriptionPriceCreateAttributes{}) {
		t.Fatal("expected upfront plan price to match omitted plan type")
	}
}
