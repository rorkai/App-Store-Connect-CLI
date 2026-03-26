package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var fetchAvailableTerritoryDetailsFn = fetchAvailableTerritoryDetails

func fetchAvailableTerritoryDetails(ctx context.Context, client *asc.Client, appID string) (string, []string, int, error) {
	availabilityID := ""
	availableTerritories := 0
	decodedAvailableTerritories := 0
	territoryIDs := make(map[string]struct{})

	availabilityResp, err := client.GetAppAvailabilityV2(ctx, appID)
	if err != nil {
		if shared.IsAppAvailabilityMissing(err) {
			return "", nil, 0, nil
		}
		return "", nil, 0, fmt.Errorf("failed to fetch app availability: %w", err)
	}

	availabilityID = strings.TrimSpace(availabilityResp.Data.ID)
	if availabilityID == "" {
		return "", nil, 0, nil
	}

	nextURL := ""
	for {
		var territoryResp *asc.TerritoryAvailabilitiesResponse
		if strings.TrimSpace(nextURL) != "" {
			territoryResp, err = client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesNextURL(nextURL))
		} else {
			territoryResp, err = client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesLimit(200))
		}
		if err != nil {
			return availabilityID, nil, availableTerritories, fmt.Errorf("failed to fetch territory availabilities: %w", err)
		}

		for _, territoryAvailability := range territoryResp.Data {
			if territoryAvailability.Attributes.Available {
				availableTerritories++
				territoryID, err := territoryAvailabilityTerritoryID(territoryAvailability.Relationships)
				if err != nil || strings.TrimSpace(territoryID) == "" {
					continue
				}
				decodedAvailableTerritories++
				territoryIDs[strings.TrimSpace(territoryID)] = struct{}{}
			}
		}

		nextURL = strings.TrimSpace(territoryResp.Links.Next)
		if nextURL == "" {
			break
		}
	}

	ids := make([]string, 0, len(territoryIDs))
	for territoryID := range territoryIDs {
		ids = append(ids, territoryID)
	}
	slices.Sort(ids)
	if availableTerritories > 0 && decodedAvailableTerritories != availableTerritories {
		return availabilityID, nil, availableTerritories, nil
	}

	return availabilityID, ids, availableTerritories, nil
}

func territoryAvailabilityTerritoryID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var relationships asc.TerritoryAvailabilityRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", fmt.Errorf("decode territory availability relationships: %w", err)
	}
	return strings.TrimSpace(relationships.Territory.Data.ID), nil
}

func availabilityCheckSkipReason(err error) (string, bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Subscription pricing coverage verification was skipped because the App Store Connect availability endpoints timed out", true
	case errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err):
		return "Subscription pricing coverage verification was skipped because this App Store Connect account cannot read app availability territories", true
	case asc.IsRetryable(err):
		return "Subscription pricing coverage verification was skipped because the App Store Connect availability endpoints were temporarily unavailable or rate limited", true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return "Subscription pricing coverage verification was skipped because the App Store Connect availability endpoints could not be reached", true
	}

	return "", false
}
