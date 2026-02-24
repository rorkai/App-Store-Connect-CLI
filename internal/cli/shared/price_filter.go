package shared

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// PriceFilter holds parsed price filter criteria.
type PriceFilter struct {
	Price    string
	MinPrice string
	MaxPrice string
}

var errNonFinitePrice = errors.New("non-finite price")

func parseFinitePrice(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errNonFinitePrice
	}
	return value, nil
}

// HasFilter returns true if any price filter is set.
func (pf PriceFilter) HasFilter() bool {
	return pf.Price != "" || pf.MinPrice != "" || pf.MaxPrice != ""
}

// Validate checks that filter values are valid numbers and not contradictory.
func (pf PriceFilter) Validate() error {
	if pf.Price != "" && (pf.MinPrice != "" || pf.MaxPrice != "") {
		return fmt.Errorf("--price and --min-price/--max-price are mutually exclusive")
	}
	var min float64
	var max float64
	hasMin := false
	hasMax := false
	if pf.Price != "" {
		if _, err := parseFinitePrice(pf.Price); err != nil {
			if errors.Is(err, errNonFinitePrice) {
				return fmt.Errorf("--price must be a finite number")
			}
			return fmt.Errorf("--price must be a number: %w", err)
		}
	}
	if pf.MinPrice != "" {
		parsed, err := parseFinitePrice(pf.MinPrice)
		if err != nil {
			if errors.Is(err, errNonFinitePrice) {
				return fmt.Errorf("--min-price must be a finite number")
			}
			return fmt.Errorf("--min-price must be a number: %w", err)
		}
		min = parsed
		hasMin = true
	}
	if pf.MaxPrice != "" {
		parsed, err := parseFinitePrice(pf.MaxPrice)
		if err != nil {
			if errors.Is(err, errNonFinitePrice) {
				return fmt.Errorf("--max-price must be a finite number")
			}
			return fmt.Errorf("--max-price must be a number: %w", err)
		}
		max = parsed
		hasMax = true
	}
	if hasMin && hasMax {
		if min > max {
			return fmt.Errorf("--min-price (%s) cannot exceed --max-price (%s)", pf.MinPrice, pf.MaxPrice)
		}
	}
	return nil
}

// MatchesPrice returns true if the given customerPrice string passes the filter.
func (pf PriceFilter) MatchesPrice(customerPrice string) bool {
	if !pf.HasFilter() {
		return true
	}
	price, err := parseFinitePrice(customerPrice)
	if err != nil {
		return false
	}
	if pf.Price != "" {
		target, err := parseFinitePrice(pf.Price)
		if err != nil {
			return false
		}
		return math.Abs(price-target) < 0.005
	}
	if pf.MinPrice != "" {
		min, err := parseFinitePrice(pf.MinPrice)
		if err != nil {
			return false
		}
		if price < min-0.005 {
			return false
		}
	}
	if pf.MaxPrice != "" {
		max, err := parseFinitePrice(pf.MaxPrice)
		if err != nil {
			return false
		}
		if price > max+0.005 {
			return false
		}
	}
	return true
}
