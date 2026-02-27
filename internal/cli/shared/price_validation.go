package shared

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateFinitePriceFlag validates a numeric flag value that must be finite.
func ValidateFinitePriceFlag(flagName, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if _, err := parseFinitePrice(trimmed); err != nil {
		if errors.Is(err, errNonFinitePrice) {
			return fmt.Errorf("%s must be a finite number", flagName)
		}
		return fmt.Errorf("%s must be a number", flagName)
	}

	return nil
}
