package asc

import "encoding/json"

func decodeFutureTerritories(data []byte) (value, present bool, err error) {
	var payload struct {
		Future *bool `json:"availableInNewTerritories"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return false, false, err
	}
	if payload.Future == nil {
		return false, false, nil
	}
	return *payload.Future, true, nil
}

func (a *SubscriptionAvailabilityAttributes) UnmarshalJSON(data []byte) error {
	value, present, err := decodeFutureTerritories(data)
	if err != nil {
		return err
	}
	a.futureTerritoriesKnown = present
	if present {
		a.AvailableInNewTerritories = value
	}
	return nil
}

func (a *InAppPurchaseAvailabilityAttributes) UnmarshalJSON(data []byte) error {
	value, present, err := decodeFutureTerritories(data)
	if err != nil {
		return err
	}
	a.futureTerritoriesKnown = present
	if present {
		a.AvailableInNewTerritories = value
	}
	return nil
}

// FutureTerritoriesKnown reports whether the provider sent a non-null setting.
func (a SubscriptionAvailabilityAttributes) FutureTerritoriesKnown() bool {
	return a.futureTerritoriesKnown
}

// FutureTerritoriesKnown reports whether the provider sent a non-null setting.
func (a InAppPurchaseAvailabilityAttributes) FutureTerritoriesKnown() bool {
	return a.futureTerritoriesKnown
}
