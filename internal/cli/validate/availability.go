package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var fetchAvailableTerritoriesFn = fetchAvailableTerritories

func fetchAvailableTerritories(ctx context.Context, client *asc.Client, appID string) (string, int, error) {
	availabilityID := ""
	availableTerritories := 0

	availabilityResp, err := client.GetAppAvailabilityV2(ctx, appID)
	if err != nil {
		if shared.IsAppAvailabilityMissing(err) {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("failed to fetch app availability: %w", err)
	}

	availabilityID = strings.TrimSpace(availabilityResp.Data.ID)
	if availabilityID == "" {
		return "", 0, nil
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
			return availabilityID, availableTerritories, fmt.Errorf("failed to fetch territory availabilities: %w", err)
		}

		for _, territoryAvailability := range territoryResp.Data {
			if territoryAvailability.Attributes.Available {
				availableTerritories++
			}
		}

		nextURL = strings.TrimSpace(territoryResp.Links.Next)
		if nextURL == "" {
			break
		}
	}

	return availabilityID, availableTerritories, nil
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
