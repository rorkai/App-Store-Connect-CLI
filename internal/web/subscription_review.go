package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const reviewSubscriptionsFields = "productId,name,state,isAppStoreReviewInProgress,submitWithNextAppStoreVersion"

// ReviewSubscription summarizes a subscription's attach state for the next app version review.
type ReviewSubscription struct {
	ID                            string `json:"id"`
	GroupID                       string `json:"groupId,omitempty"`
	GroupReferenceName            string `json:"groupReferenceName,omitempty"`
	ProductID                     string `json:"productId,omitempty"`
	Name                          string `json:"name,omitempty"`
	State                         string `json:"state,omitempty"`
	IsAppStoreReviewInProgress    bool   `json:"isAppStoreReviewInProgress"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

// ReviewSubscriptionSubmission captures the hidden submission resource returned by the web attach flow.
type ReviewSubscriptionSubmission struct {
	ID                            string `json:"id"`
	SubscriptionID                string `json:"subscriptionId,omitempty"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

func decodeReviewSubscriptions(resources []jsonAPIResource, included []jsonAPIResource) []ReviewSubscription {
	if len(resources) == 0 {
		return []ReviewSubscription{}
	}
	includedMap := buildIncludedMap(included)
	subscriptions := make([]ReviewSubscription, 0, len(included))
	for _, group := range resources {
		groupID := strings.TrimSpace(group.ID)
		groupName := stringAttr(group.Attributes, "referenceName")
		for _, ref := range relationshipRefs(group, "subscriptions") {
			if !strings.EqualFold(strings.TrimSpace(ref.Type), "subscriptions") {
				continue
			}
			resource, ok := includedMap[jsonAPIResourceKey(ref.Type, ref.ID)]
			if !ok {
				resource = jsonAPIResource{ID: ref.ID, Type: ref.Type}
			}
			subscriptions = append(subscriptions, ReviewSubscription{
				ID:                            strings.TrimSpace(ref.ID),
				GroupID:                       groupID,
				GroupReferenceName:            groupName,
				ProductID:                     stringAttr(resource.Attributes, "productId"),
				Name:                          stringAttr(resource.Attributes, "name"),
				State:                         stringAttr(resource.Attributes, "state"),
				IsAppStoreReviewInProgress:    boolAttr(resource.Attributes, "isAppStoreReviewInProgress"),
				SubmitWithNextAppStoreVersion: boolAttr(resource.Attributes, "submitWithNextAppStoreVersion"),
			})
		}
	}
	return subscriptions
}

// ListReviewSubscriptions lists subscriptions and their next-version attach state for an app.
func (c *Client) ListReviewSubscriptions(ctx context.Context, appID string) ([]ReviewSubscription, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	query := url.Values{}
	query.Set("include", "subscriptions")
	query.Set("limit", "300")
	query.Set("limit[subscriptions]", "1000")
	query.Set("sort", "referenceName")
	query.Set("fields[subscriptions]", reviewSubscriptionsFields)

	nextPath := queryPath("/apps/"+url.PathEscape(appID)+"/subscriptionGroups", query)
	allResources := make([]jsonAPIResource, 0, 64)
	allIncluded := make([]jsonAPIResource, 0, 256)
	visited := map[string]struct{}{}

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("review subscriptions pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		responseBody, err := c.doRequest(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, err
		}

		var payload jsonAPIListPayload
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse review subscriptions response: %w", err)
		}
		allResources = append(allResources, payload.Data...)
		allIncluded = append(allIncluded, payload.Included...)

		nextLink, err := extractNextLink(payload.Links)
		if err != nil {
			return nil, fmt.Errorf("failed to parse review subscriptions pagination links: %w", err)
		}
		if strings.TrimSpace(nextLink) == "" {
			nextPath = ""
			continue
		}
		nextPath, err = normalizeNextPath(nextLink, c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize review subscriptions pagination link: %w", err)
		}
	}

	decoded := decodeReviewSubscriptions(allResources, allIncluded)
	if len(decoded) == 0 {
		return []ReviewSubscription{}, nil
	}
	return decoded, nil
}

// CreateSubscriptionSubmission attaches a subscription to the next app version review via the private web flow.
func (c *Client) CreateSubscriptionSubmission(ctx context.Context, subscriptionID string) (ReviewSubscriptionSubmission, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return ReviewSubscriptionSubmission{}, fmt.Errorf("subscription id is required")
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "subscriptionSubmissions",
			"attributes": map[string]any{
				"submitWithNextAppStoreVersion": true,
			},
			"relationships": map[string]any{
				"subscription": map[string]any{
					"data": map[string]string{
						"type": "subscriptions",
						"id":   subscriptionID,
					},
				},
			},
		},
	}

	responseBody, err := c.doRequest(ctx, http.MethodPost, "/subscriptionSubmissions", body)
	if err != nil {
		return ReviewSubscriptionSubmission{}, err
	}

	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return ReviewSubscriptionSubmission{}, fmt.Errorf("failed to parse subscription submission response: %w", err)
	}

	result := ReviewSubscriptionSubmission{
		ID:                            strings.TrimSpace(payload.Data.ID),
		SubmitWithNextAppStoreVersion: boolAttr(payload.Data.Attributes, "submitWithNextAppStoreVersion"),
	}
	if ref := firstRelationshipRef(payload.Data, "subscription"); ref != nil {
		result.SubscriptionID = strings.TrimSpace(ref.ID)
	}
	if result.SubscriptionID == "" {
		result.SubscriptionID = subscriptionID
	}
	return result, nil
}

// DeleteSubscriptionSubmission detaches a subscription from the next app version review via the private web flow.
func (c *Client) DeleteSubscriptionSubmission(ctx context.Context, subscriptionID string) error {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return fmt.Errorf("subscription id is required")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/subscriptionSubmissions/"+url.PathEscape(subscriptionID), nil)
	return err
}
