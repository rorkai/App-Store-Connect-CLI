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
