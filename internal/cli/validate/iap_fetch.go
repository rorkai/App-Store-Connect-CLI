package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var fetchIAPsFn = fetchIAPs

func fetchIAPs(ctx context.Context, client *asc.Client, appID string) ([]validation.IAP, error) {
	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	defer firstCancel()

	firstPage, err := client.GetInAppPurchasesV2(firstCtx, strings.TrimSpace(appID), asc.WithIAPLimit(200))
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetInAppPurchasesV2(pageCtx, strings.TrimSpace(appID), asc.WithIAPNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	return mapIAPsResponse(paginated)
}

func mapIAPsResponse(paginated asc.PaginatedResponse) ([]validation.IAP, error) {
	if paginated == nil {
		return nil, fmt.Errorf("unexpected nil in-app purchases pagination response")
	}

	resp, ok := paginated.(*asc.InAppPurchasesV2Response)
	if !ok {
		return nil, fmt.Errorf("unexpected in-app purchases pagination response type: %T", paginated)
	}

	iaps := make([]validation.IAP, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		iaps = append(iaps, validation.IAP{
			ID:        item.ID,
			Name:      attrs.Name,
			ProductID: attrs.ProductID,
			Type:      attrs.InAppPurchaseType,
			State:     attrs.State,
		})
	}

	return iaps, nil
}
