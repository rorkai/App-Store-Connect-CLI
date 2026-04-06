package assets

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type screenshotUploadProgress struct {
	Results      []asc.AssetUploadResultItem
	OrderedIDs   []string
	PendingFiles []string
	FailedFile   string
}

// UploadScreenshotsToSet uploads screenshots in the provided file order and then
// applies that order to the remote screenshot set.
func UploadScreenshotsToSet(ctx context.Context, client *asc.Client, setID string, files []string, preserveExistingOrder bool) ([]asc.AssetUploadResultItem, error) {
	orderedIDs := make([]string, 0, len(files))
	if preserveExistingOrder {
		existingIDs, err := GetOrderedAppScreenshotIDs(ctx, client, setID)
		if err != nil {
			return nil, err
		}
		orderedIDs = append(orderedIDs, existingIDs...)
	}

	progress, err := uploadScreenshotsWithOrderState(ctx, client, setID, orderedIDs, files, false)
	if err != nil {
		return nil, err
	}
	return progress.Results, nil
}

func uploadScreenshotsWithOrderState(ctx context.Context, client *asc.Client, setID string, orderedIDs, files []string, syncIfNoNew bool) (screenshotUploadProgress, error) {
	progress := screenshotUploadProgress{
		Results:    make([]asc.AssetUploadResultItem, 0, len(files)),
		OrderedIDs: append([]string(nil), orderedIDs...),
	}

	for idx, filePath := range files {
		item, err := uploadScreenshotAsset(ctx, client, setID, filePath)
		if err != nil {
			progress.PendingFiles = append([]string{filePath}, files[idx+1:]...)
			progress.FailedFile = filePath
			return progress, err
		}
		progress.Results = append(progress.Results, item)
		progress.OrderedIDs = appendUniqueScreenshotID(progress.OrderedIDs, item.AssetID)
	}

	if len(progress.OrderedIDs) == 0 {
		return progress, nil
	}
	if len(progress.Results) == 0 && !syncIfNoNew {
		return progress, nil
	}
	if err := SetOrderedAppScreenshots(ctx, client, setID, progress.OrderedIDs); err != nil {
		return progress, err
	}
	return progress, nil
}

// GetOrderedAppScreenshotIDs returns screenshot IDs in the current remote order.
func GetOrderedAppScreenshotIDs(ctx context.Context, client *asc.Client, setID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	firstPage, err := client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, setID, asc.WithLinkagesLimit(200))
	if err != nil {
		return nil, err
	}

	orderedIDs := make([]string, 0, len(firstPage.Data))
	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, "", asc.WithLinkagesNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		linkages, ok := page.(*asc.LinkagesResponse)
		if !ok {
			return fmt.Errorf("unexpected screenshot relationship response type %T", page)
		}
		for _, item := range linkages.Data {
			orderedIDs = appendUniqueScreenshotID(orderedIDs, item.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return orderedIDs, nil
}

// SetOrderedAppScreenshots replaces the screenshot relationships for a set in the provided order.
func SetOrderedAppScreenshots(ctx context.Context, client *asc.Client, setID string, orderedIDs []string) error {
	if client == nil {
		return fmt.Errorf("client is required")
	}
	return client.UpdateAppScreenshotSetAppScreenshotsRelationship(ctx, setID, normalizeScreenshotIDs(orderedIDs))
}

func normalizeScreenshotIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func appendUniqueScreenshotID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}
