package shared

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PPPScheduleSnapshot retains the actual schedule base and complete forward prices.
type PPPScheduleSnapshot struct {
	BaseTerritory string
	Prices        []ResolvedPriceRow
}

// PPPTerritorySelection validates an optional, explicitly supplied mutation scope.
func PPPTerritorySelection(value *OnceCSVValue) ([]string, error) {
	return pppTerritorySelectionTo(os.Stderr, value)
}

// PPPTerritorySelection is PPPTerritorySelection with the "Error:" line
// written to d's stderr.
func (d *Deps) PPPTerritorySelection(value *OnceCSVValue) ([]string, error) {
	return pppTerritorySelectionTo(d.IO.ErrOut, value)
}

func pppTerritorySelectionTo(stderr io.Writer, value *OnceCSVValue) ([]string, error) {
	if !value.Provided() {
		return nil, nil
	}
	territories, err := NormalizeASCTerritoryCSV(value.String())
	if err != nil {
		return nil, usageErrorTo(stderr, fmt.Sprintf("invalid --territory: %v", err))
	}
	if len(territories) == 0 {
		return nil, usageErrorTo(stderr, "invalid --territory: provide at least one territory")
	}
	sort.Strings(territories)
	return slices.Compact(territories), nil
}

// SelectPPPCurrentPrices requires every requested territory to be currently priced.
func SelectPPPCurrentPrices(prices []ResolvedPriceRow, territories []string, at time.Time) ([]ResolvedPriceRow, error) {
	selected := make([]ResolvedPriceRow, 0, len(territories))
	for _, territory := range territories {
		row, ok := pppPriceAt(prices, territory, at)
		if !ok {
			return nil, fmt.Errorf("requested territory %s has no current price", territory)
		}
		selected = append(selected, row)
	}
	return selected, nil
}

func pppPriceAt(prices []ResolvedPriceRow, territory string, at time.Time) (ResolvedPriceRow, bool) {
	day := PricingDate(at).Format("2006-01-02")
	var found ResolvedPriceRow
	ok := false
	for _, row := range prices {
		if row.Territory != territory || row.StartDate > day || (row.EndDate != "" && row.EndDate <= day) {
			continue
		}
		if !ok || (row.Manual != nil && *row.Manual && (found.Manual == nil || !*found.Manual)) ||
			(row.Manual != nil && found.Manual != nil && *row.Manual == *found.Manual && row.StartDate > found.StartDate) {
			found, ok = row, true
		}
	}
	return found, ok
}

// ValidatePPPScheduleSnapshot fails closed before reconstructing a schedule.
func ValidatePPPScheduleSnapshot(snapshot PPPScheduleSnapshot) error {
	if snapshot.BaseTerritory == "" {
		return fmt.Errorf("price schedule is missing its base territory")
	}
	forward := append([]ResolvedPriceRow(nil), snapshot.Prices...)
	SortResolvedPrices(forward)
	ends := make(map[string]string)
	today := PricingDate(PricingNow()).Format("2006-01-02")
	for _, row := range forward {
		if row.Territory == "" || row.PricePointID == "" || row.Manual == nil {
			return fmt.Errorf("cannot resolve complete schedule price %q", row.PriceID)
		}
		for _, date := range []string{row.StartDate, row.EndDate} {
			if date != "" {
				if _, err := time.Parse("2006-01-02", date); err != nil {
					return fmt.Errorf("price %q: invalid schedule date %q", row.PriceID, date)
				}
			}
		}
		if row.StartDate != "" && row.EndDate != "" && row.StartDate >= row.EndDate {
			return fmt.Errorf("price %q has an invalid interval", row.PriceID)
		}
		if row.EndDate != "" && row.EndDate <= today {
			continue
		}
		mode := fmt.Sprintf("%s/%t", row.Territory, *row.Manual)
		if previousEnd, exists := ends[mode]; exists && (previousEnd == "" || row.StartDate < previousEnd) {
			return fmt.Errorf("overlapping schedule intervals for %s", mode)
		}
		ends[mode] = row.EndDate
	}
	if _, ok := pppPriceAt(snapshot.Prices, snapshot.BaseTerritory, PricingNow()); !ok {
		return fmt.Errorf("price schedule has no current base price")
	}
	return nil
}

func scopedPPPPrices(snapshot PPPScheduleSnapshot, rows []asc.SubscriptionPricingDeriveRow, run priceSchedulePPPRun, ladders map[string][]PriceLadderCandidate, key func(string) string) ([]ScheduledPricePoint, error) {
	changed := make(map[string]string)
	for _, row := range rows {
		if row.Status == "planned" {
			changed[row.Territory] = row.TargetPricePointID
		}
	}
	if _, changesBase := changed[snapshot.BaseTerritory]; changesBase {
		return nil, fmt.Errorf("cannot change the schedule base timeline with --territory; omit --territory for a global base change")
	}
	today := PricingDate(PricingNow()).Format("2006-01-02")
	effective := run.startDate
	if effective == "" {
		effective = today
	}
	prices := make([]ScheduledPricePoint, 0, len(snapshot.Prices)+len(changed))
	for _, row := range snapshot.Prices {
		if !*row.Manual || (row.EndDate != "" && row.EndDate <= today) {
			continue
		}
		if _, changed := changed[row.Territory]; changed {
			if row.StartDate >= effective {
				continue
			}
			if row.EndDate == "" || row.EndDate > effective {
				row.EndDate = effective
			}
			if row.EndDate <= today {
				continue
			}
		}
		if row.StartDate <= today {
			row.StartDate = ""
		}
		point := ladderPricePointID(ladders[row.Territory], row.PricePointID, key)
		if !priceLadderHasID(ladders[row.Territory], point) {
			return nil, fmt.Errorf("cannot preserve %s price point %q: missing from ladder", row.Territory, row.PricePointID)
		}
		prices = append(prices, ScheduledPricePoint{PricePointID: point, StartDate: row.StartDate, EndDate: row.EndDate})
	}
	for _, row := range rows {
		if point, ok := changed[row.Territory]; ok {
			prices = append(prices, ScheduledPricePoint{PricePointID: point, StartDate: run.startDate})
		}
	}
	return prices, nil
}

func pppSnapshotKeys(snapshot PPPScheduleSnapshot, key func(string) string, retain func(ResolvedPriceRow) bool) []string {
	today := PricingDate(PricingNow()).Format("2006-01-02")
	keys := []string{snapshot.BaseTerritory}
	for _, row := range snapshot.Prices {
		if (row.EndDate != "" && row.EndDate <= today) || !retain(row) {
			continue
		}
		if row.StartDate <= today {
			row.StartDate = ""
		}
		amount := ""
		if !*row.Manual {
			amount = row.CustomerPrice
			if value, ok := new(big.Rat).SetString(amount); ok {
				amount = value.RatString()
			}
		}
		keys = append(keys, fmt.Sprintf("%s|%s|%s|%s|%t|%s", row.Territory, key(row.PricePointID), row.StartDate, row.EndDate, *row.Manual, amount))
	}
	sort.Strings(keys)
	return keys
}

func samePPPSnapshot(a, b PPPScheduleSnapshot, key func(string) string) bool {
	all := func(ResolvedPriceRow) bool { return true }
	return slices.Equal(pppSnapshotKeys(a, key, all), pppSnapshotKeys(b, key, all))
}

func verifyScopedPPPSchedule(ctx context.Context, surface PriceSchedulePPPSurface, before PPPScheduleSnapshot, rows []asc.SubscriptionPricingDeriveRow, prices []ScheduledPricePoint, ladders map[string][]PriceLadderCandidate, effective string, key func(string) string) error {
	after, err := surface.ReadSchedule(ctx)
	if err != nil {
		return fmt.Errorf("preservation verification readback failed: %w", err)
	}
	changed := make(map[string]bool)
	for _, row := range rows {
		if row.Status != "noop" && row.Status != "skipped" {
			changed[row.Territory] = true
		}
	}
	if effective == "" {
		effective = PricingDate(PricingNow()).Format("2006-01-02")
	}
	clipAutomatic := func(row ResolvedPriceRow) (ResolvedPriceRow, bool) {
		if *row.Manual || !changed[row.Territory] {
			return row, true
		}
		if row.StartDate >= effective {
			return row, false
		}
		if row.EndDate == "" || row.EndDate > effective {
			row.EndDate = effective
		}
		return row, true
	}
	expected := PPPScheduleSnapshot{BaseTerritory: before.BaseTerritory}
	for _, row := range before.Prices {
		if !*row.Manual {
			if clipped, ok := clipAutomatic(row); ok {
				expected.Prices = append(expected.Prices, clipped)
			}
		}
	}
	territoryByPoint := make(map[string]string)
	for territory, candidates := range ladders {
		for _, point := range candidates {
			territoryByPoint[point.PricePointID] = territory
		}
	}
	manual := true
	for _, price := range prices {
		expected.Prices = append(expected.Prices, ResolvedPriceRow{Territory: territoryByPoint[price.PricePointID], PricePointID: price.PricePointID, StartDate: price.StartDate, EndDate: price.EndDate, Manual: &manual})
	}
	normalizedAfter := PPPScheduleSnapshot{BaseTerritory: after.BaseTerritory}
	for _, row := range after.Prices {
		if clipped, ok := clipAutomatic(row); ok {
			normalizedAfter.Prices = append(normalizedAfter.Prices, clipped)
		}
	}
	retain := func(ResolvedPriceRow) bool { return true }
	if !slices.Equal(pppSnapshotKeys(expected, key, retain), pppSnapshotKeys(normalizedAfter, key, retain)) {
		manualOnly := func(row ResolvedPriceRow) bool { return *row.Manual }
		if slices.Equal(pppSnapshotKeys(expected, key, manualOnly), pppSnapshotKeys(normalizedAfter, key, manualOnly)) {
			return fmt.Errorf("preservation verification failed: observed automatic pricing readback drift (price, timeline, or mode)")
		}
		return fmt.Errorf("preservation verification failed: manual timelines or schedule base differ")
	}
	return nil
}
