package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetActor retrieves a single actor by ID.
func (c *Client) GetActor(ctx context.Context, actorID string, fields []string) (*ActorResponse, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, fmt.Errorf("actorID is required")
	}

	var lastNotFound error
	for _, candidate := range actorIDCandidates(actorID) {
		response, err := c.getActorByID(ctx, candidate, fields)
		if err == nil {
			return response, nil
		}
		if IsNotFound(err) {
			lastNotFound = err
			continue
		}
		return nil, err
	}
	if lastNotFound != nil {
		return nil, lastNotFound
	}
	return nil, fmt.Errorf("actor not found")
}

func actorIDCandidates(actorID string) []string {
	if strings.Contains(actorID, ":") {
		return []string{actorID}
	}
	normalized := strings.TrimSpace(actorID)
	if normalized == "" {
		return nil
	}
	if strings.EqualFold(normalized, "APPLE") {
		return []string{normalized}
	}
	return []string{
		normalized,
		"USER:" + normalized,
		"API_KEY:" + normalized,
		"XCODE_CLOUD:" + normalized,
	}
}
