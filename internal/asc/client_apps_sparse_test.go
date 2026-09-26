package asc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAppAttributesSparseJSONRoundTrip(t *testing.T) {
	for _, fixture := range []string{
		`{}`,
		`{"name":"","bundleId":"com.example.app","sku":"SKU"}`,
		`{"subscriptionStatusUrl":null,"subscriptionStatusUrlVersion":null,"subscriptionStatusUrlForSandbox":null,"subscriptionStatusUrlVersionForSandbox":null}`,
		`{"name":null,"futureAttribute":{"enabled":false}}`,
	} {
		t.Run(fixture, func(t *testing.T) {
			var attrs AppAttributes
			if err := json.Unmarshal([]byte(fixture), &attrs); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(attrs)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal([]byte(fixture), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip = %s, want %s", encoded, fixture)
			}
		})
	}
}

func TestAppAttributesSparseJSONRetainsCallerChanges(t *testing.T) {
	var attrs AppAttributes
	if err := json.Unmarshal([]byte(`{"name":"Original","subscriptionStatusUrl":"https://example.com/old"}`), &attrs); err != nil {
		t.Fatal(err)
	}
	attrs.Name = "Renamed"
	attrs.BundleID = "com.example.new"
	*attrs.SubscriptionStatusURL = "https://example.com/new"
	encoded, err := json.Marshal(attrs)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"name": "Renamed", "bundleId": "com.example.new", "subscriptionStatusUrl": "https://example.com/new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %s", encoded)
	}
}
