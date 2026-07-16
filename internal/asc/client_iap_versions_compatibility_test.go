package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

const iapVersionRelationshipJSON = `{"data":{"type":"inAppPurchases","id":"iap-1","relationships":{"versions":{"data":[{"type":"inAppPurchaseVersions","id":"version-1"}]}}},"links":{}}`

const includedIAPVersionRelationshipJSON = `{"data":{"type":"placeholder","id":"resource-1"},"included":[{"type":"inAppPurchases","id":"iap-1","relationships":{"versions":{"data":[{"type":"inAppPurchaseVersions","id":"version-1"}]}}}],"links":{}}`

func TestLegacyIAPResponsesPreserveIncludedVersionRelationships(t *testing.T) {
	tests := []struct {
		name   string
		decode func(*Client) (json.RawMessage, error)
	}{
		{
			name: "review screenshot read response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.GetInAppPurchaseAppStoreReviewScreenshot(context.Background(), "screenshot-1")
				if err != nil {
					return nil, err
				}
				return response.Included, nil
			},
		},
		{
			name: "image read response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.GetInAppPurchaseImage(context.Background(), "image-1")
				if err != nil {
					return nil, err
				}
				return response.Included, nil
			},
		},
		{
			name: "localization read response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.GetInAppPurchaseLocalization(context.Background(), "localization-1")
				if err != nil {
					return nil, err
				}
				return response.Included, nil
			},
		},
		{
			name: "submission create response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-1")
				if err != nil {
					return nil, err
				}
				return response.Included, nil
			},
		},
		{
			name: "promoted purchase read response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.GetPromotedPurchase(context.Background(), "promoted-purchase-1")
				if err != nil {
					return nil, err
				}
				return response.Included, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusOK, includedIAPVersionRelationshipJSON))
			included, err := test.decode(client)
			if err != nil {
				t.Fatalf("decode inherited IAP response: %v", err)
			}
			assertIncludedIAPVersionRelationship(t, included)
		})
	}
}

func TestInAppPurchaseV2WriteResponsesPreserveVersionRelationships(t *testing.T) {
	tests := []struct {
		name   string
		decode func(*Client) (json.RawMessage, error)
	}{
		{
			name: "create response",
			decode: func(client *Client) (json.RawMessage, error) {
				response, err := client.CreateInAppPurchaseV2(context.Background(), "app-1", InAppPurchaseV2CreateAttributes{
					Name:              "Pro",
					ProductID:         "com.example.pro",
					InAppPurchaseType: "CONSUMABLE",
				})
				if err != nil {
					return nil, err
				}
				return response.Data.Relationships, nil
			},
		},
		{
			name: "update response",
			decode: func(client *Client) (json.RawMessage, error) {
				name := "Updated Pro"
				response, err := client.UpdateInAppPurchaseV2(context.Background(), "iap-1", InAppPurchaseV2UpdateAttributes{Name: &name})
				if err != nil {
					return nil, err
				}
				return response.Data.Relationships, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusOK, iapVersionRelationshipJSON))
			relationships, err := test.decode(client)
			if err != nil {
				t.Fatalf("decode IAP v2 write response: %v", err)
			}
			assertIAPVersionRelationship(t, relationships)
		})
	}
}

func assertIncludedIAPVersionRelationship(t *testing.T, included json.RawMessage) {
	t.Helper()

	var resources []struct {
		Type          ResourceType    `json:"type"`
		Relationships json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(included, &resources); err != nil {
		t.Fatalf("decode included resources: %v", err)
	}
	if len(resources) != 1 || resources[0].Type != ResourceTypeInAppPurchases {
		t.Fatalf("expected one included inAppPurchases resource, got %+v", resources)
	}
	assertIAPVersionRelationship(t, resources[0].Relationships)
}

func assertIAPVersionRelationship(t *testing.T, relationships json.RawMessage) {
	t.Helper()

	var decoded struct {
		Versions RelationshipList `json:"versions"`
	}
	if err := json.Unmarshal(relationships, &decoded); err != nil {
		t.Fatalf("decode IAP relationships: %v", err)
	}
	if len(decoded.Versions.Data) != 1 {
		t.Fatalf("expected one version relationship, got %+v", decoded.Versions.Data)
	}
	version := decoded.Versions.Data[0]
	if version.Type != ResourceTypeInAppPurchaseVersions || version.ID != "version-1" {
		t.Fatalf("unexpected version relationship: %+v", version)
	}
}
