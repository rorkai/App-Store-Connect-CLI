package iap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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
	return shared.ContextWithResolvedTimeout(ctx, iapAssetUploadDefaultTimeout)
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

// snapshotImageFile copies the selected image into a private temporary file
// before any checksum or upload work begins. Hashing and uploading the source
// pathname independently would allow an in-place rewrite between those reads
// to commit a checksum for different bytes than the upload sent.
func snapshotImageFile(source *os.File, size int64) (*os.File, func(), error) {
	if source == nil {
		return nil, func() {}, fmt.Errorf("image source file is required")
	}
	if size <= 0 {
		return nil, func() {}, fmt.Errorf("image source file must not be empty")
	}

	snapshot, err := os.CreateTemp("", ".asc-iap-image-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create image snapshot: %w", err)
	}
	snapshotPath := snapshot.Name()
	// Unlink immediately where the OS permits removing an open file. Some
	// platforms keep the path until cleanup closes the snapshot descriptor.
	snapshotPathLinked := os.Remove(snapshotPath) != nil
	cleanup := func() {
		_ = snapshot.Close()
		if snapshotPathLinked {
			_ = os.Remove(snapshotPath)
		}
	}

	if _, err := io.CopyN(snapshot, io.NewSectionReader(source, 0, size), size); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("copy image snapshot: %w", err)
	}
	if _, err := snapshot.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("rewind image snapshot: %w", err)
	}
	return snapshot, cleanup, nil
}

func parseOfferCodeEligibilities(value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
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
	entries, err := shared.ParseASCTerritoryValueCSV(value)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	prices := make([]asc.InAppPurchaseOfferCodePrice, 0, len(entries))
	for _, entry := range entries {
		prices = append(prices, asc.InAppPurchaseOfferCodePrice{
			TerritoryID:  entry.TerritoryID,
			PricePointID: entry.Value,
		})
	}

	return prices, nil
}

func parsePriceSchedulePrices(value string) ([]asc.InAppPurchasePriceSchedulePrice, error) {
	entries := shared.SplitCSV(value)
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

type relationshipReference struct {
	Data asc.ResourceData `json:"data"`
}

func relationshipResourceID(relationships json.RawMessage, key string) (string, error) {
	if len(relationships) == 0 {
		return "", fmt.Errorf("missing relationships")
	}
	if !utf8.Valid(relationships) {
		return "", fmt.Errorf("relationships contain invalid UTF-8")
	}

	var references map[string]relationshipReference
	if err := json.Unmarshal(relationships, &references); err != nil {
		return "", fmt.Errorf("parse relationships: %w", err)
	}

	reference, ok := references[key]
	if !ok {
		return "", fmt.Errorf("missing %s relationship", key)
	}

	id := strings.TrimSpace(reference.Data.ID)
	if id == "" {
		return "", fmt.Errorf("missing %s relationship id", key)
	}

	return id, nil
}

func normalizeIAPDate(value, label string) (string, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", fmt.Errorf("%s must be in YYYY-MM-DD format", label)
	}
	return parsed.Format("2006-01-02"), nil
}
