package asc

import (
	"fmt"
	"strings"
)

// SubscriptionPriceTierInlineCreate defines a tier in a compound price request.
type SubscriptionPriceTierInlineCreate struct {
	Type          ResourceType                             `json:"type"`
	ID            string                                   `json:"id,omitempty"`
	Attributes    SubscriptionPriceTierInlineAttributes    `json:"attributes"`
	Relationships SubscriptionPriceTierInlineRelationships `json:"relationships"`
}

// SubscriptionPriceTierInlineAttributes defines a new tier's minimum quantity.
type SubscriptionPriceTierInlineAttributes struct {
	MinQuantity int `json:"minQuantity"`
}

// SubscriptionPriceTierInlineRelationships links a new tier to its price point.
type SubscriptionPriceTierInlineRelationships struct {
	SubscriptionPricePoint Relationship `json:"subscriptionPricePoint"`
}

func subscriptionPriceTierResources(tiers []ProposedSubscriptionPriceTier, prefix string) (*RelationshipList, []SubscriptionPriceTierInlineCreate) {
	if tiers == nil {
		return nil, nil
	}
	linkage := &RelationshipList{Data: make([]ResourceData, 0, len(tiers))}
	included := make([]SubscriptionPriceTierInlineCreate, 0, len(tiers))
	for _, tier := range tiers {
		id := fmt.Sprintf("${%stier-%d}", prefix, tier.MinQuantity)
		linkage.Data = append(linkage.Data, ResourceData{Type: ResourceType("subscriptionPriceTiers"), ID: id})
		included = append(included, SubscriptionPriceTierInlineCreate{
			Type: ResourceType("subscriptionPriceTiers"),
			ID:   id,
			Attributes: SubscriptionPriceTierInlineAttributes{
				MinQuantity: tier.MinQuantity,
			},
			Relationships: SubscriptionPriceTierInlineRelationships{
				SubscriptionPricePoint: Relationship{Data: ResourceData{
					Type: ResourceTypeSubscriptionPricePoints,
					ID:   strings.TrimSpace(tier.SubscriptionPricePointID),
				}},
			},
		})
	}
	return linkage, included
}
