// Package pppindex embeds a purchasing-power price-level index per territory.
//
// Each value is the World Bank World Development Indicators PPP conversion
// factor, GDP (LCU per international $, PA.NUS.PPP) divided by the official
// exchange rate (LCU per US$, period average, PA.NUS.FCRF) for the latest year
// from 2021 through 2025 in which both series are reported. The United States
// is 1. Only App Store Connect territory IDs are kept, and values outside 0.05
// to 2.0 are omitted because they come from series with mismatched currency
// units or official exchange rates. Bulgaria's exchange rate is converted from
// BGN to EUR at the fixed 1.95583 rate because its PPP factor is reported in
// EUR. Data is licensed under CC BY 4.0 and was retrieved from
// api.worldbank.org on 2026-10-22 (series last updated 2026-10-08).
package pppindex

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

const (
	// Source names the dataset and series behind the index.
	Source = "World Bank, World Development Indicators: PA.NUS.PPP / PA.NUS.FCRF"
	// License is the dataset license.
	License = "CC BY 4.0"
	// AsOf is the date the source series were last updated.
	AsOf = "2026-10-08"
)

//go:embed price_levels.csv
var priceLevelsCSV string

var priceLevels = sync.OnceValues(func() (map[string]*big.Rat, error) {
	records, err := csv.NewReader(strings.NewReader(priceLevelsCSV)).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || strings.Join(records[0], ",") != "territory,priceLevel,year" {
		return nil, fmt.Errorf("unexpected header")
	}
	levels := make(map[string]*big.Rat, len(records)-1)
	for _, record := range records[1:] {
		level, ok := new(big.Rat).SetString(record[1])
		if !ok || level.Sign() <= 0 {
			return nil, fmt.Errorf("invalid price level %q for %s", record[1], record[0])
		}
		if _, duplicate := levels[record[0]]; duplicate {
			return nil, fmt.Errorf("duplicate territory %s", record[0])
		}
		levels[record[0]] = level
	}
	return levels, nil
})

// PriceLevel returns the price level for a 3-letter territory ID.
func PriceLevel(territory string) (*big.Rat, bool) {
	levels, err := priceLevels()
	if err != nil {
		panic(fmt.Sprintf("pppindex: embedded price levels are invalid: %v", err))
	}
	level, ok := levels[strings.ToUpper(strings.TrimSpace(territory))]
	if !ok {
		return nil, false
	}
	return new(big.Rat).Set(level), true
}
