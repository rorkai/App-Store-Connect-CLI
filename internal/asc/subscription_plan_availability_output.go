package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func subscriptionPlanAvailabilitiesRows(resp *SubscriptionPlanAvailabilitiesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Plan Type", "Available In New Territories"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.PlanType),
			formatOptionalSubscriptionBool(item.Attributes.AvailableInNewTerritories),
		})
	}
	return headers, rows
}

func subscriptionPlanAvailabilityShowRows(resp *SubscriptionPlanAvailabilitiesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Plan Type", "Available In New Territories", "Territories"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryIDs, total, totalKnown := SubscriptionPlanAvailabilityIncludedTerritories(item.Relationships)
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.PlanType),
			formatOptionalSubscriptionBool(item.Attributes.AvailableInNewTerritories),
			formatPlanAvailabilityShowTerritoryCell(territoryIDs, total, totalKnown),
		})
	}
	return headers, rows
}

// PrintSubscriptionPlanAvailabilityShowTable renders plan-availability show as a table.
func PrintSubscriptionPlanAvailabilityShowTable(resp *SubscriptionPlanAvailabilitiesResponse) error {
	headers, rows := subscriptionPlanAvailabilityShowRows(resp)
	return renderTable(os.Stdout, headers, rows)
}

// PrintSubscriptionPlanAvailabilityShowMarkdown renders plan-availability show as Markdown.
func PrintSubscriptionPlanAvailabilityShowMarkdown(resp *SubscriptionPlanAvailabilitiesResponse) error {
	headers, rows := subscriptionPlanAvailabilityShowRows(resp)
	return renderMarkdown(os.Stdout, headers, rows)
}

// SubscriptionPlanAvailabilityIncludedTerritories extracts included available
// territory IDs and the relationship paging total from a plan availability
// resource. totalKnown is false when paging metadata is absent.
func SubscriptionPlanAvailabilityIncludedTerritories(raw json.RawMessage) (ids []string, total int, totalKnown bool) {
	if len(raw) == 0 {
		return nil, 0, false
	}
	var relationships SubscriptionPlanAvailabilityRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil || relationships.AvailableTerritories == nil {
		return nil, 0, false
	}
	ids = make([]string, 0, len(relationships.AvailableTerritories.Data))
	for _, item := range relationships.AvailableTerritories.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	total, totalKnown = ParsePagingTotalOK(relationships.AvailableTerritories.Meta)
	if !totalKnown {
		total = len(ids)
	}
	return ids, total, totalKnown
}

func formatPlanAvailabilityShowTerritoryCell(ids []string, total int, totalKnown bool) string {
	displayed := ids
	omitted := 0
	if len(displayed) > subscriptionPlanAvailabilityTerritoryCellLimit {
		omitted = len(displayed) - subscriptionPlanAvailabilityTerritoryCellLimit
		displayed = displayed[:subscriptionPlanAvailabilityTerritoryCellLimit]
	}
	if totalKnown && total > len(ids) {
		omitted += total - len(ids)
	}
	cell := strings.Join(displayed, ",")
	if omitted > 0 {
		if cell == "" {
			return fmt.Sprintf("(+%d more)", omitted)
		}
		return fmt.Sprintf("%s (+%d more)", cell, omitted)
	}
	return cell
}

func formatOptionalSubscriptionBool(value *bool) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%t", *value)
}
