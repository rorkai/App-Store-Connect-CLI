package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CreativePlacementRows validates a placement collection and derives human
// display rows while leaving the original JSON response untouched.
func CreativePlacementRows(payload []byte) ([]string, [][]string, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, nil, fmt.Errorf("placements: invalid response: %w", err)
	}
	raw := bytes.TrimSpace(envelope.Data)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, nil, fmt.Errorf("placements: expected a data collection")
	}
	return creativePlacementItemRows(raw)
}

// CreativePlacementResourceRows validates a single placement response and
// derives one human display row while leaving the original JSON untouched.
func CreativePlacementResourceRows(payload []byte) ([]string, [][]string, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, nil, fmt.Errorf("placements: invalid response: %w", err)
	}
	raw := bytes.TrimSpace(envelope.Data)
	if bytes.Equal(raw, []byte("null")) {
		return creativePlacementItemRows([]byte("[]"))
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, nil, fmt.Errorf("placements: expected a data resource")
	}
	return creativePlacementItemRows(append(append([]byte{'['}, raw...), ']'))
}

func creativePlacementItemRows(raw []byte) ([]string, [][]string, error) {
	var items []struct {
		ID         string                     `json:"id"`
		Type       string                     `json:"type"`
		Attributes map[string]json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nil, fmt.Errorf("placements: invalid collection: %w", err)
	}
	headers := []string{"ID", "Media", "Placement Type", "Group", "State"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type != "appAssetLibraryPlacements" {
			return nil, nil, fmt.Errorf("placements: invalid placement resource")
		}
		text := func(key string) string {
			var value string
			_ = json.Unmarshal(item.Attributes[key], &value)
			return value
		}
		rows = append(rows, []string{item.ID, text("mediaType"), text("placementType"), text("placementGroup"), text("state")})
	}
	return headers, rows, nil
}
