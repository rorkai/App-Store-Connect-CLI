package asc

import (
	"encoding/json"
	"testing"
)

func TestLegacySubscriptionResponsesPreserveVersionAndPricePointRelationships(t *testing.T) {
	t.Parallel()

	type decodeIncluded func([]byte) (json.RawMessage, error)
	tests := []struct {
		name              string
		primaryType       ResourceType
		includePricePoint bool
		decode            decodeIncluded
	}{
		{
			name:        "review screenshot create and update",
			primaryType: ResourceTypeSubscriptionAppStoreReviewScreenshots,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionAppStoreReviewScreenshotResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "legacy image create and update",
			primaryType: ResourceTypeSubscriptionImages,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionImageResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "legacy localization create and update",
			primaryType: ResourceTypeSubscriptionLocalizations,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionLocalizationResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:              "introductory offer create and update",
			primaryType:       ResourceTypeSubscriptionIntroductoryOffers,
			includePricePoint: true,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionIntroductoryOfferResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "promotional offer create and update",
			primaryType: ResourceTypeSubscriptionPromotionalOffers,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionPromotionalOfferResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "offer code create and update",
			primaryType: ResourceTypeSubscriptionOfferCodes,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionOfferCodeResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "submission create",
			primaryType: ResourceTypeSubscriptionSubmissions,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionSubmissionResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name:        "promoted purchase create and update",
			primaryType: ResourceTypePromotedPurchases,
			decode: func(payload []byte) (json.RawMessage, error) {
				var response PromotedPurchaseResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := legacySubscriptionResponsePayload(t, tt.primaryType, tt.includePricePoint)
			includedJSON, err := tt.decode(payload)
			if err != nil {
				t.Fatalf("decode legacy response: %v", err)
			}
			assertSubscriptionCompoundResources(t, includedJSON, tt.includePricePoint)
		})
	}
}

func TestLegacySubscriptionPriceResponsesPreserveAdjustedEqualizations(t *testing.T) {
	t.Parallel()

	payload := legacySubscriptionPriceListResponsePayload(t)
	tests := []struct {
		name   string
		decode func([]byte) (json.RawMessage, error)
	}{
		{
			name: "promotional offer price reads",
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionPromotionalOfferPricesResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name: "offer code price reads",
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionOfferCodePricesResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name: "subscription price reads",
			decode: func(payload []byte) (json.RawMessage, error) {
				var response SubscriptionPricesResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
		{
			name: "win-back offer price reads",
			decode: func(payload []byte) (json.RawMessage, error) {
				var response WinBackOfferPricesResponse
				err := json.Unmarshal(payload, &response)
				return response.Included, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			includedJSON, err := tt.decode(payload)
			if err != nil {
				t.Fatalf("decode legacy price response: %v", err)
			}
			var included []Resource[json.RawMessage]
			if err := json.Unmarshal(includedJSON, &included); err != nil {
				t.Fatalf("decode included resources: %v", err)
			}
			if len(included) != 1 {
				t.Fatalf("included count = %d, want 1", len(included))
			}
			assertSubscriptionPricePointRelationships(t, included[0].Relationships)
		})
	}
}

func TestSubscriptionCreateUpdateResponsePreservesVersions(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"data": {
			"type": "subscriptions",
			"id": "sub-1",
			"attributes": {"name": "Pro"},
			"relationships": {
				"versions": {
					"data": [{"type": "subscriptionVersions", "id": "version-1"}],
					"links": {"related": "/v1/subscriptions/sub-1/versions"},
					"meta": {"paging": {"total": 1}}
				}
			}
		},
		"links": {"self": "/v1/subscriptions/sub-1"}
	}`)

	var response SubscriptionResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode subscription create/update response: %v", err)
	}
	assertSubscriptionRelationships(t, response.Data.Relationships)
}

func legacySubscriptionResponsePayload(t *testing.T, primaryType ResourceType, includePricePoint bool) []byte {
	t.Helper()

	included := []map[string]any{legacySubscriptionResource()}
	if includePricePoint {
		included = append(included, legacySubscriptionPricePointResource())
	}
	payload := map[string]any{
		"data": map[string]any{
			"type":       primaryType,
			"id":         "primary-1",
			"attributes": map[string]any{},
		},
		"included": included,
		"links":    map[string]any{"self": "/v1/resources/primary-1"},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return encoded
}

func legacySubscriptionPriceListResponsePayload(t *testing.T) []byte {
	t.Helper()

	payload := map[string]any{
		"data": []map[string]any{{
			"type":       ResourceTypeSubscriptionPrices,
			"id":         "price-1",
			"attributes": map[string]any{},
		}},
		"included": []map[string]any{legacySubscriptionPricePointResource()},
		"links":    map[string]any{"self": "/v1/prices"},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return encoded
}

func legacySubscriptionResource() map[string]any {
	return map[string]any{
		"type":       ResourceTypeSubscriptions,
		"id":         "sub-1",
		"attributes": map[string]any{"name": "Pro"},
		"relationships": map[string]any{
			"versions": map[string]any{
				"data": []map[string]any{{"type": ResourceTypeSubscriptionVersions, "id": "version-1"}},
				"links": map[string]any{
					"self":    "/v1/subscriptions/sub-1/relationships/versions",
					"related": "/v1/subscriptions/sub-1/versions",
				},
				"meta": map[string]any{"paging": map[string]any{"total": 1}},
			},
		},
	}
}

func legacySubscriptionPricePointResource() map[string]any {
	return map[string]any{
		"type":       ResourceTypeSubscriptionPricePoints,
		"id":         "price-point-1",
		"attributes": map[string]any{"customerPrice": "9.99"},
		"relationships": map[string]any{
			"adjustedEqualizations": map[string]any{
				"links": map[string]any{"related": "/v1/subscriptionPricePoints/price-point-1/adjustedEqualizations"},
			},
		},
	}
}

func assertSubscriptionCompoundResources(t *testing.T, includedJSON json.RawMessage, expectPricePoint bool) {
	t.Helper()

	var included []Resource[json.RawMessage]
	if err := json.Unmarshal(includedJSON, &included); err != nil {
		t.Fatalf("decode included resources: %v", err)
	}
	wantCount := 1
	if expectPricePoint {
		wantCount++
	}
	if len(included) != wantCount {
		t.Fatalf("included count = %d, want %d", len(included), wantCount)
	}
	assertSubscriptionRelationships(t, included[0].Relationships)
	if expectPricePoint {
		assertSubscriptionPricePointRelationships(t, included[1].Relationships)
	}
}

func assertSubscriptionRelationships(t *testing.T, relationshipsJSON json.RawMessage) {
	t.Helper()

	var relationships SubscriptionResponseRelationships
	if err := json.Unmarshal(relationshipsJSON, &relationships); err != nil {
		t.Fatalf("decode subscription relationships: %v", err)
	}
	if relationships.Versions == nil || len(relationships.Versions.Data) != 1 {
		t.Fatalf("versions relationship missing: %s", relationshipsJSON)
	}
	version := relationships.Versions.Data[0]
	if version.Type != ResourceTypeSubscriptionVersions || version.ID != "version-1" {
		t.Fatalf("version linkage = %s/%s", version.Type, version.ID)
	}
	if relationships.Versions.Links.Related != "/v1/subscriptions/sub-1/versions" {
		t.Fatalf("versions related link = %q", relationships.Versions.Links.Related)
	}
	var meta struct {
		Paging struct {
			Total int `json:"total"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(relationships.Versions.Meta, &meta); err != nil {
		t.Fatalf("decode versions meta: %v", err)
	}
	if meta.Paging.Total != 1 {
		t.Fatalf("versions total = %d, want 1", meta.Paging.Total)
	}
}

func assertSubscriptionPricePointRelationships(t *testing.T, relationshipsJSON json.RawMessage) {
	t.Helper()

	var relationships SubscriptionPricePointResponseRelationships
	if err := json.Unmarshal(relationshipsJSON, &relationships); err != nil {
		t.Fatalf("decode price-point relationships: %v", err)
	}
	if relationships.AdjustedEqualizations == nil {
		t.Fatalf("adjustedEqualizations relationship missing: %s", relationshipsJSON)
	}
	want := "/v1/subscriptionPricePoints/price-point-1/adjustedEqualizations"
	if got := relationships.AdjustedEqualizations.Links.Related; got != want {
		t.Fatalf("adjustedEqualizations related link = %q, want %q", got, want)
	}
}
