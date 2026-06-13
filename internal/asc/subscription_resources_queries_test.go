package asc

import (
	"net/url"
	"testing"
)

func TestBuildSubscriptionPricesQueryPlanType(t *testing.T) {
	query := &subscriptionPricesQuery{}
	WithSubscriptionPricesPlanType(SubscriptionPlanTypeMonthly)(query)

	values, err := url.ParseQuery(buildSubscriptionPricesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got := values.Get("filter[planType]"); got != "MONTHLY" {
		t.Fatalf("expected filter[planType]=MONTHLY, got %q", got)
	}
}

func TestBuildSubscriptionPricesQueryRejectsEmptyPlanType(t *testing.T) {
	query := &subscriptionPricesQuery{}
	WithSubscriptionPricesPlanType("")(query)

	values, err := url.ParseQuery(buildSubscriptionPricesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got := values.Get("filter[planType]"); got != "" {
		t.Fatalf("expected no planType filter, got %q", got)
	}
}

func TestBuildSubscriptionPricesQueryPlanTypeWithOtherFilters(t *testing.T) {
	query := &subscriptionPricesQuery{}
	WithSubscriptionPricesPlanType(SubscriptionPlanTypeUpfront)(query)
	WithSubscriptionPricesTerritory("nor")(query)
	WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"})(query)
	WithSubscriptionPricesLimit(25)(query)

	values, err := url.ParseQuery(buildSubscriptionPricesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got := values.Get("filter[planType]"); got != "UPFRONT" {
		t.Fatalf("expected filter[planType]=UPFRONT, got %q", got)
	}
	if got := values.Get("filter[territory]"); got != "NOR" {
		t.Fatalf("expected filter[territory]=NOR, got %q", got)
	}
	if got := values.Get("include"); got != "subscriptionPricePoint,territory" {
		t.Fatalf("expected combined include values, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}
