package shared

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PriceRounding selects how a calculated price resolves to an Apple
// price-point ladder.
type PriceRounding string

const (
	PriceRoundingExact   PriceRounding = "exact"
	PriceRoundingNearest PriceRounding = "nearest"
	PriceRoundingUp      PriceRounding = "up"
	PriceRoundingDown    PriceRounding = "down"
)

type PriceLadderCandidate struct {
	PricePointID  string
	CustomerPrice string
}

type PriceLadderResolution struct {
	PricePointID  string
	CustomerPrice string
	Price         *big.Rat
}

func SummarizePriceLadderRows(rows []asc.SubscriptionPricingDeriveRow) asc.SubscriptionPricingDeriveSummary {
	summary := asc.SubscriptionPricingDeriveSummary{Total: len(rows)}
	for _, row := range rows {
		switch row.Status {
		case "planned":
			summary.Planned++
		case "noop":
			summary.Noop++
		case "applied":
			summary.Applied++
		case "verified":
			summary.Verified++
		case "unresolved":
			summary.Unresolved++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}
	return summary
}

func IsPriceDecimal(value string) bool {
	digits := 0
	dots := 0
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			digits++
		case character == '.':
			dots++
			if dots > 1 {
				return false
			}
		default:
			return false
		}
	}
	return digits > 0
}

func ParsePriceRounding(value string) (PriceRounding, error) {
	return parsePriceRoundingTo(os.Stderr, value)
}

// ParsePriceRounding is ParsePriceRounding with the "Error:" line written to
// d's stderr.
func (d *Deps) ParsePriceRounding(value string) (PriceRounding, error) {
	return parsePriceRoundingTo(d.IO.ErrOut, value)
}

func parsePriceRoundingTo(stderr io.Writer, value string) (PriceRounding, error) {
	normalized := PriceRounding(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case PriceRoundingExact,
		PriceRoundingNearest,
		PriceRoundingUp,
		PriceRoundingDown:
		return normalized, nil
	default:
		return "", usageErrorTo(stderr, "--round must be one of: exact, nearest, up, down")
	}
}

func ResolvePriceLadderPoint(
	candidates []PriceLadderCandidate,
	desired *big.Rat,
	rounding PriceRounding,
) (PriceLadderResolution, error) {
	if desired == nil || desired.Sign() <= 0 {
		return PriceLadderResolution{}, fmt.Errorf("desired target price must be positive")
	}
	if len(candidates) == 0 {
		return PriceLadderResolution{}, fmt.Errorf("target price-point ladder is empty")
	}

	type parsedCandidate struct {
		resolution PriceLadderResolution
		key        string
	}
	parsed := make([]parsedCandidate, 0, len(candidates))
	byPrice := make(map[string][]parsedCandidate)
	priceByID := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.PricePointID)
		if id == "" {
			return PriceLadderResolution{}, fmt.Errorf("target price point has an empty id")
		}

		price := new(big.Rat)
		if _, ok := price.SetString(strings.TrimSpace(candidate.CustomerPrice)); !ok || price.Sign() < 0 {
			return PriceLadderResolution{}, fmt.Errorf("target price point %q has invalid customer price %q", id, candidate.CustomerPrice)
		}
		if price.Sign() == 0 {
			// App and in-app purchase ladders start with a free point, which
			// is never a calculated price.
			continue
		}
		priceKey := price.RatString()
		if existingPrice, seen := priceByID[id]; seen {
			if existingPrice != priceKey {
				return PriceLadderResolution{}, fmt.Errorf("target price point %q has conflicting customer prices", id)
			}
			continue
		}
		priceByID[id] = priceKey
		item := parsedCandidate{
			resolution: PriceLadderResolution{
				PricePointID:  id,
				CustomerPrice: strings.TrimSpace(candidate.CustomerPrice),
				Price:         price,
			},
			key: priceKey,
		}
		parsed = append(parsed, item)
		byPrice[item.key] = append(byPrice[item.key], item)
	}
	if len(parsed) == 0 {
		return PriceLadderResolution{}, fmt.Errorf("target price-point ladder is empty")
	}

	var selected *parsedCandidate
	for i := range parsed {
		candidate := &parsed[i]
		cmp := candidate.resolution.Price.Cmp(desired)
		switch rounding {
		case PriceRoundingExact:
			if cmp == 0 {
				selected = candidate
			}
		case PriceRoundingUp:
			if cmp >= 0 && (selected == nil || candidate.resolution.Price.Cmp(selected.resolution.Price) < 0) {
				selected = candidate
			}
		case PriceRoundingDown:
			if cmp <= 0 && (selected == nil || candidate.resolution.Price.Cmp(selected.resolution.Price) > 0) {
				selected = candidate
			}
		case PriceRoundingNearest:
			if selected == nil || priceLadderCandidateIsNearer(candidate.resolution.Price, selected.resolution.Price, desired) {
				selected = candidate
			}
		default:
			return PriceLadderResolution{}, fmt.Errorf("unsupported rounding mode %q", rounding)
		}
	}

	desiredText := FormatExactPriceDecimal(desired)
	if selected == nil {
		switch rounding {
		case PriceRoundingExact:
			return PriceLadderResolution{}, fmt.Errorf("no exact target price point for %s", desiredText)
		case PriceRoundingUp:
			return PriceLadderResolution{}, fmt.Errorf("no target price point at or above %s", desiredText)
		case PriceRoundingDown:
			return PriceLadderResolution{}, fmt.Errorf("no target price point at or below %s", desiredText)
		default:
			return PriceLadderResolution{}, fmt.Errorf("no target price point for %s", desiredText)
		}
	}
	if duplicates := byPrice[selected.key]; len(duplicates) > 1 {
		return PriceLadderResolution{}, fmt.Errorf(
			"multiple target price points match selected price %s",
			FormatExactPriceDecimal(selected.resolution.Price),
		)
	}
	return selected.resolution, nil
}

func priceLadderCandidateIsNearer(candidate, selected, desired *big.Rat) bool {
	candidateDistance := new(big.Rat).Sub(candidate, desired)
	candidateDistance.Abs(candidateDistance)
	selectedDistance := new(big.Rat).Sub(selected, desired)
	selectedDistance.Abs(selectedDistance)
	comparison := candidateDistance.Cmp(selectedDistance)
	if comparison != 0 {
		return comparison < 0
	}
	return candidate.Cmp(selected) < 0
}

func FormatPriceDecimal(value *big.Rat, precision int) string {
	if value == nil {
		return ""
	}
	if value.IsInt() {
		return value.Num().String()
	}
	if precision < 0 {
		precision = 0
	}
	if value.Sign() != 0 {
		absoluteNumerator := new(big.Int).Abs(new(big.Int).Set(value.Num()))
		denominator := new(big.Int).Abs(new(big.Int).Set(value.Denom()))
		digitEstimate := len(denominator.String()) - len(absoluteNumerator.String()) - 1
		if digitEstimate > precision {
			precision = digitEstimate
		}
		scaledTwice := new(big.Int).Mul(absoluteNumerator, big.NewInt(2))
		if precision > 0 {
			scaledTwice.Mul(scaledTwice, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil))
		}
		for scaledTwice.Cmp(denominator) < 0 {
			precision++
			scaledTwice.Mul(scaledTwice, big.NewInt(10))
		}
	}
	formatted := value.FloatString(precision)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "-0" {
		return "0"
	}
	return formatted
}

func FormatExactPriceDecimal(value *big.Rat) string {
	if value == nil {
		return ""
	}
	denominator := new(big.Int).Abs(new(big.Int).Set(value.Denom()))
	two := big.NewInt(2)
	five := big.NewInt(5)
	one := big.NewInt(1)
	remainder := new(big.Int)
	twos := 0
	for remainder.Mod(denominator, two).Sign() == 0 {
		denominator.Quo(denominator, two)
		twos++
	}
	fives := 0
	for remainder.Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		fives++
	}
	if denominator.Cmp(one) != 0 {
		return value.RatString()
	}
	precision := twos
	if fives > precision {
		precision = fives
	}
	return FormatPriceDecimal(value, precision)
}

// PlanPriceLadderRow plans one territory: source price × multiplier resolved
// on the territory's candidate ladder. A nil multiplier marks the territory
// skipped, keeping its current price.
func PlanPriceLadderRow(
	source ResolvedPriceRow,
	current *ResolvedPriceRow,
	multiplier *big.Rat,
	candidates []PriceLadderCandidate,
	candidatesErr error,
	rounding PriceRounding,
) asc.SubscriptionPricingDeriveRow {
	territory := strings.ToUpper(strings.TrimSpace(source.Territory))
	row := asc.SubscriptionPricingDeriveRow{
		Territory:          territory,
		Currency:           strings.TrimSpace(source.Currency),
		SourcePrice:        strings.TrimSpace(source.CustomerPrice),
		SourcePricePointID: strings.TrimSpace(source.PricePointID),
		Action:             "error",
		Status:             "unresolved",
	}
	if current != nil {
		row.CurrentTargetPrice = strings.TrimSpace(current.CustomerPrice)
		row.CurrentTargetPointID = strings.TrimSpace(current.PricePointID)
	}
	if multiplier == nil {
		row.Action = "skip"
		row.Status = "skipped"
		return row
	}
	row.RequestedMultiple = FormatExactPriceDecimal(multiplier)

	sourcePrice := new(big.Rat)
	if _, ok := sourcePrice.SetString(row.SourcePrice); !ok || sourcePrice.Sign() <= 0 {
		row.Error = fmt.Sprintf("source territory %s has invalid customer price %q", territory, row.SourcePrice)
		return row
	}
	desired := new(big.Rat).Mul(sourcePrice, multiplier)
	row.DesiredPrice = FormatExactPriceDecimal(desired)

	if candidatesErr != nil {
		row.Error = candidatesErr.Error()
		return row
	}
	resolution, err := ResolvePriceLadderPoint(candidates, desired, rounding)
	if err != nil {
		row.Error = fmt.Sprintf("%s: %v", territory, err)
		return row
	}

	row.TargetPrice = resolution.CustomerPrice
	row.TargetPricePointID = resolution.PricePointID
	achieved := new(big.Rat).Quo(resolution.Price, sourcePrice)
	row.AchievedMultiple = FormatPriceDecimal(achieved, 6)
	delta := new(big.Rat).Sub(achieved, multiplier)
	row.MultipleDelta = FormatPriceDecimal(delta, 6)
	if row.CurrentTargetPointID == row.TargetPricePointID {
		row.Action = "noop"
		row.Status = "noop"
	} else {
		row.Action = "update"
		row.Status = "planned"
	}
	return row
}
