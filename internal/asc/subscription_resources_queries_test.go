package asc

import (
	"net/url"
	"reflect"
	"testing"
)

func TestBuildSubscriptionIntroductoryOffersQueryFieldsAndInclude(t *testing.T) {
	query := &subscriptionIntroductoryOffersQuery{}
	WithSubscriptionIntroductoryOffersFields([]string{
		"startDate",
		"endDate",
		"duration",
		"offerMode",
		"numberOfPeriods",
		"targetSubscriptionPlanType",
		"territory",
		"subscriptionPricePoint",
	})(query)
	WithSubscriptionIntroductoryOffersInclude([]string{"territory", "subscriptionPricePoint"})(query)

	values, err := url.ParseQuery(buildSubscriptionIntroductoryOffersQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got := values.Get("fields[subscriptionIntroductoryOffers]"); got != "startDate,endDate,duration,offerMode,numberOfPeriods,targetSubscriptionPlanType,territory,subscriptionPricePoint" {
		t.Fatalf("unexpected introductory offer fields: %q", got)
	}
	if got := values.Get("include"); got != "territory,subscriptionPricePoint" {
		t.Fatalf("unexpected introductory offer include: %q", got)
	}
}

func TestBuildSubscriptionPricesQuery(t *testing.T) {
	tests := []struct {
		name string
		opts []SubscriptionPricesOption
		want url.Values
	}{
		{
			name: "plan type",
			opts: []SubscriptionPricesOption{WithSubscriptionPricesPlanType(SubscriptionPlanTypeMonthly)},
			want: url.Values{"filter[planType]": {"MONTHLY"}},
		},
		{
			name: "empty plan type is omitted",
			opts: []SubscriptionPricesOption{WithSubscriptionPricesPlanType("")},
			want: url.Values{},
		},
		{
			name: "price fields",
			opts: []SubscriptionPricesOption{WithSubscriptionPricesFields([]string{"startDate", "preserved", "planType", "territory", "subscriptionPricePoint"})},
			want: url.Values{"fields[subscriptionPrices]": {"startDate,preserved,planType,territory,subscriptionPricePoint"}},
		},
		{
			name: "price point filter",
			opts: []SubscriptionPricesOption{WithSubscriptionPricesPricePointIDs([]string{"point-1", " point-2 "})},
			want: url.Values{"filter[subscriptionPricePoint]": {"point-1,point-2"}},
		},
		{
			name: "plan type with other filters",
			opts: []SubscriptionPricesOption{
				WithSubscriptionPricesPlanType(SubscriptionPlanTypeUpfront),
				WithSubscriptionPricesTerritory("nor"),
				WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
				Limit[SubscriptionPricesOption](25),
			},
			want: url.Values{
				"filter[planType]":  {"UPFRONT"},
				"filter[territory]": {"NOR"},
				"include":           {"subscriptionPricePoint,territory"},
				"limit":             {"25"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := &subscriptionPricesQuery{}
			for _, opt := range tc.opts {
				opt(query)
			}
			values, err := url.ParseQuery(buildSubscriptionPricesQuery(query))
			if err != nil {
				t.Fatalf("parse query: %v", err)
			}
			if !reflect.DeepEqual(values, tc.want) {
				t.Fatalf("query = %v, want %v", values, tc.want)
			}
		})
	}
}
