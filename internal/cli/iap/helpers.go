package iap

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const iapAssetUploadDefaultTimeout = 10 * time.Minute

var validOfferCodeEligibilities = map[string]struct{}{
	"NON_SPENDER":     {},
	"ACTIVE_SPENDER":  {},
	"CHURNED_SPENDER": {},
}

func contextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, asc.ResolveTimeoutWithDefault(iapAssetUploadDefaultTimeout))
}

func openImageFile(path string) (*os.File, os.FileInfo, error) {
	if err := asc.ValidateImageFile(path); err != nil {
		return nil, nil, err
	}
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func parseOfferCodeEligibilities(value string) ([]string, error) {
	values := splitCSVUpper(value)
	if len(values) == 0 {
		return nil, nil
	}
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		if _, ok := validOfferCodeEligibilities[item]; !ok {
			return nil, fmt.Errorf("--eligibilities must be one of: NON_SPENDER, ACTIVE_SPENDER, CHURNED_SPENDER")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	return unique, nil
}

func parseOfferCodePrices(value string) ([]asc.InAppPurchaseOfferCodePrice, error) {
	entries := splitCSV(value)
	if len(entries) == 0 {
		return nil, nil
	}

	prices := make([]asc.InAppPurchaseOfferCodePrice, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("--prices must use TERRITORY:PRICE_POINT_ID entries")
		}
		territoryID := strings.ToUpper(strings.TrimSpace(parts[0]))
		pricePointID := strings.TrimSpace(parts[1])
		if territoryID == "" || pricePointID == "" {
			return nil, fmt.Errorf("--prices must use TERRITORY:PRICE_POINT_ID entries")
		}
		prices = append(prices, asc.InAppPurchaseOfferCodePrice{
			TerritoryID:  territoryID,
			PricePointID: pricePointID,
		})
	}

	return prices, nil
}

func parsePriceSchedulePrices(value string) ([]asc.InAppPurchasePriceSchedulePrice, error) {
	entries := splitCSV(value)
	if len(entries) == 0 {
		return nil, nil
	}

	prices := make([]asc.InAppPurchasePriceSchedulePrice, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) < 1 {
			continue
		}
		pricePointID := strings.TrimSpace(parts[0])
		if pricePointID == "" {
			return nil, fmt.Errorf("--prices must include a price point ID")
		}
		startDate := ""
		endDate := ""
		if len(parts) >= 2 {
			startDate = strings.TrimSpace(parts[1])
			if startDate != "" {
				normalized, err := normalizeIAPDate(startDate, "--prices start date")
				if err != nil {
					return nil, err
				}
				startDate = normalized
			}
		}
		if len(parts) == 3 {
			endDate = strings.TrimSpace(parts[2])
			if endDate != "" {
				normalized, err := normalizeIAPDate(endDate, "--prices end date")
				if err != nil {
					return nil, err
				}
				endDate = normalized
			}
		}
		prices = append(prices, asc.InAppPurchasePriceSchedulePrice{
			PricePointID: pricePointID,
			StartDate:    startDate,
			EndDate:      endDate,
		})
	}

	return prices, nil
}

func normalizeIAPDate(value, label string) (string, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", fmt.Errorf("%s must be in YYYY-MM-DD format", label)
	}
	return parsed.Format("2006-01-02"), nil
}
