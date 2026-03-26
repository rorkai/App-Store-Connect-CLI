package validate

import (
	"context"
	"errors"
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var fetchAppBuildCountFn = fetchAppBuildCount

func fetchAppBuildCount(ctx context.Context, client *asc.Client, appID string) (int, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetBuilds(reqCtx, appID)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "app builds"); ok {
			return 0, metadataCheckStatus{SkipReason: reason}, nil
		}
		return 0, metadataCheckStatus{}, fmt.Errorf("failed to fetch app builds: %w", err)
	}

	if total, ok := asc.ParsePagingTotalOK(resp.Meta); ok {
		return total, metadataCheckStatus{Verified: true}, nil
	}

	return len(resp.Data), metadataCheckStatus{Verified: true}, nil
}
