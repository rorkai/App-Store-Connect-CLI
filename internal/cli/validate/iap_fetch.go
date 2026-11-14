package validate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var fetchIAPsFn = fetchIAPs

func fetchIAPs(ctx context.Context, client *asc.Client, appID string, strict bool) ([]validation.IAP, error) {
	ctx = withReadinessRequestGate(ctx)
	get := client.GetInAppPurchasesV2
	if strict {
		get = client.GetInAppPurchasesV2Strict
	}
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.InAppPurchasesV2Response, error) {
		return get(requestCtx, strings.TrimSpace(appID), asc.Limit[asc.IAPOption](200))
	})
	if err != nil {
		return nil, err
	}

	total, hasTotal := asc.ParsePagingTotalOK(firstPage.Meta)
	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			page, err := get(requestCtx, strings.TrimSpace(appID), asc.NextURL[asc.IAPOption](nextURL))
			if page != nil {
				if n, ok := asc.ParsePagingTotalOK(page.Meta); ok && (!hasTotal || n > total) {
					total, hasTotal = n, true
				}
			}
			return page, err
		})
	})
	if err != nil {
		return nil, err
	}

	iaps, err := mapIAPsResponse(paginated)
	if strict {
		ids := map[string]validation.IAP{}
		for _, iap := range iaps {
			if previous, ok := ids[iap.ID]; ok && previous != iap {
				return nil, fmt.Errorf("conflicting IAP catalog identity")
			}
			ids[iap.ID] = iap
		}
		if hasTotal && total > len(ids) {
			return nil, fmt.Errorf("incomplete IAP catalog")
		}
	}
	return iaps, err
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
	sort.SliceStable(iaps, func(i, j int) bool {
		if iaps[i].ProductID != iaps[j].ProductID {
			return iaps[i].ProductID < iaps[j].ProductID
		}
		return iaps[i].ID < iaps[j].ID
	})

	return iaps, nil
}
