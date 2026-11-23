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
	ID                                 string `json:"id"`
	GroupID                            string `json:"groupId,omitempty"`
	GroupReferenceName                 string `json:"groupReferenceName,omitempty"`
	ProductID                          string `json:"productId,omitempty"`
	Name                               string `json:"name,omitempty"`
	State                              string `json:"state,omitempty"`
	IsAppStoreReviewInProgress         bool   `json:"isAppStoreReviewInProgress"`
	SubmitWithNextAppStoreVersion      bool   `json:"submitWithNextAppStoreVersion"`
	SubmitWithNextAppStoreVersionKnown bool   `json:"submitWithNextAppStoreVersionKnown"`
}

func decodeReviewSubscriptions(resources []jsonAPIResource, included []jsonAPIResource) []ReviewSubscription {
	if len(resources) == 0 {
		return []ReviewSubscription{}
	}
	includedMap := buildIncludedMap(included)
	subscriptions := make([]ReviewSubscription, 0, len(included))
	seenSubscriptionIDs := make(map[string]struct{}, len(included))
	for _, group := range resources {
		groupID := strings.TrimSpace(group.ID)
		groupName := stringAttr(group.Attributes, "referenceName")
		for _, ref := range relationshipRefs(group, "subscriptions") {
			if !strings.EqualFold(strings.TrimSpace(ref.Type), "subscriptions") {
				continue
			}
			subscriptionID := strings.TrimSpace(ref.ID)
			if _, seen := seenSubscriptionIDs[subscriptionID]; seen {
				continue
			}
			seenSubscriptionIDs[subscriptionID] = struct{}{}
			resource, ok := includedMap[jsonAPIResourceKey(ref.Type, ref.ID)]
			if !ok {
				resource = jsonAPIResource{ID: ref.ID, Type: ref.Type}
			}
			attached, attachedKnown := boolAttrKnown(resource.Attributes, "submitWithNextAppStoreVersion")
			subscriptions = append(subscriptions, ReviewSubscription{
				ID:                                 subscriptionID,
				GroupID:                            groupID,
				GroupReferenceName:                 groupName,
				ProductID:                          stringAttr(resource.Attributes, "productId"),
				Name:                               stringAttr(resource.Attributes, "name"),
				State:                              stringAttr(resource.Attributes, "state"),
				IsAppStoreReviewInProgress:         boolAttr(resource.Attributes, "isAppStoreReviewInProgress"),
				SubmitWithNextAppStoreVersion:      attached,
				SubmitWithNextAppStoreVersionKnown: attachedKnown,
			})
		}
	}
	return subscriptions
}

func boolAttrKnown(attrs map[string]any, key string) (bool, bool) {
	if attrs == nil {
		return false, false
	}
	value, ok := attrs[key]
	if !ok {
		return false, false
	}
	result, ok := value.(bool)
	return result, ok
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
		for _, resource := range payload.Data {
			if resourceType := strings.TrimSpace(resource.Type); !strings.EqualFold(resourceType, "subscriptionGroups") {
				return nil, fmt.Errorf("failed to parse review subscriptions response: unexpected resource type %q", resource.Type)
			}
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

// DeleteSubscriptionSubmission detaches a subscription from the next app version review via the private web flow.
func (c *Client) DeleteSubscriptionSubmission(ctx context.Context, subscriptionID string) error {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return fmt.Errorf("subscription id is required")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/subscriptionSubmissions/"+url.PathEscape(subscriptionID), nil)
	return err
}
