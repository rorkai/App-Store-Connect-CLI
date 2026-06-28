package shared

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ResolvedPriceRow represents the currently effective price for a territory.
type ResolvedPriceRow struct {
	Territory     string `json:"territory"`
	PlanType      string `json:"planType,omitempty"`
	PriceID       string `json:"priceId"`
	PricePointID  string `json:"pricePointId"`
	CustomerPrice string `json:"customerPrice,omitempty"`
	Currency      string `json:"currency,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
	ProceedsYear2 string `json:"proceedsYear2,omitempty"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	Manual        *bool  `json:"manual,omitempty"`
	Preserved     *bool  `json:"preserved,omitempty"`
}

// ResolvedPricesResult is the command-local response envelope for --resolved pricing.
type ResolvedPricesResult struct {
	Prices []ResolvedPriceRow `json:"prices"`
}

// SortResolvedPrices orders prices deterministically for stable output.
func SortResolvedPrices(rows []ResolvedPriceRow) {
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		if left.Territory != right.Territory {
			return left.Territory < right.Territory
		}
		if left.PlanType != right.PlanType {
			return left.PlanType < right.PlanType
		}
		if left.StartDate != right.StartDate {
			return left.StartDate < right.StartDate
		}
		if left.PriceID != right.PriceID {
			return left.PriceID < right.PriceID
		}
		return left.PricePointID < right.PricePointID
	})
}

// PrintResolvedPrices renders resolved price rows without registering global raw renderers.
func PrintResolvedPrices(result *ResolvedPricesResult, format string, pretty bool) error {
	return PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return printResolvedPricesTable(result, false) },
		func() error { return printResolvedPricesTable(result, true) },
	)
}

func printResolvedPricesTable(result *ResolvedPricesResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("resolved prices result is nil")
	}

	rows := make([]ResolvedPriceRow, len(result.Prices))
	copy(rows, result.Prices)
	SortResolvedPrices(rows)

	headers := []string{
		"Territory",
		"Plan Type",
		"Price ID",
		"Price Point ID",
		"Customer Price",
		"Currency",
		"Proceeds",
		"Proceeds Y2",
		"Start Date",
		"End Date",
		"Manual",
		"Preserved",
	}

	values := make([][]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, []string{
			row.Territory,
			row.PlanType,
			row.PriceID,
			row.PricePointID,
			row.CustomerPrice,
			row.Currency,
			row.Proceeds,
			row.ProceedsYear2,
			row.StartDate,
			row.EndDate,
			optionalBoolString(row.Manual),
			optionalBoolString(row.Preserved),
		})
	}

	if markdown {
		asc.RenderMarkdown(headers, values)
		return nil
	}

	asc.RenderTable(headers, values)
	return nil
}

func optionalBoolString(value *bool) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%t", *value))
}
