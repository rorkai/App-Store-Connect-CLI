package appleads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type platformPageDetail struct {
	TotalCount *int `json:"totalCount,omitempty"`
	Offset     int  `json:"offset"`
	PageSize   int  `json:"pageSize"`
}

type platformPaginatedEnvelope struct {
	Result     []json.RawMessage  `json:"result"`
	Pagination platformPageDetail `json:"pagination"`
}

// MaxPlatformPaginationPages bounds how many pages any Apple Ads paginator
// fetches for a single logical query. Servers that keep returning full pages
// without a total count would otherwise loop forever.
const MaxPlatformPaginationPages = 1000

// PaginateAll fetches all pages for an offset-paginated endpoint.
func (c *Client) PaginateAll(ctx context.Context, spec EndpointSpec, pathParams map[string]string, query url.Values, startOffset, pageSize int, body json.RawMessage) (RawResponse, error) {
	if spec.Name == "platform-get-change-history-detail" {
		return c.paginatePlatformChangeHistoryDetail(ctx, spec, pathParams, query, startOffset, pageSize, body)
	}
	return c.paginatePlatformGET(ctx, spec, pathParams, query, startOffset, pageSize, body)
}

func (c *Client) paginatePlatformChangeHistoryDetail(ctx context.Context, spec EndpointSpec, pathParams map[string]string, query url.Values, startOffset, pageSize int, body json.RawMessage) (RawResponse, error) {
	if spec.Method != "GET" || len(body) != 0 {
		return nil, fmt.Errorf("platform API v1 change-history pagination requires a bodyless GET endpoint")
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	offset := max(0, startOffset)
	var output map[string]json.RawMessage
	var aggregated []json.RawMessage
	var total *int
	for pages := 0; ; pages++ {
		if pages >= MaxPlatformPaginationPages {
			return nil, fmt.Errorf("platform API v1 pagination exceeded the %d-page safety limit; narrow your query or use --offset to continue from a smaller result set", MaxPlatformPaginationPages)
		}
		pageQuery := cloneValues(query)
		pageQuery.Set("limit", strconv.Itoa(pageSize))
		pageQuery.Set("offset", strconv.Itoa(offset))
		raw, err := c.Do(ctx, spec, pathParams, pageQuery, body)
		if err != nil {
			return nil, err
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil, fmt.Errorf("parse paginated change-history response: %w", err)
		}
		var page platformPaginatedEnvelope
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse paginated change-history response: %w", err)
		}
		if output == nil {
			output = document
			aggregated = append([]json.RawMessage(nil), page.Result...)
		} else if aggregated, err = mergeChangeHistoryResults(aggregated, page.Result); err != nil {
			return nil, err
		}
		changesOnPage, err := countChangeHistoryChanges(page.Result)
		if err != nil {
			return nil, err
		}
		if page.Pagination.TotalCount != nil {
			value := *page.Pagination.TotalCount
			total = &value
		}
		if changesOnPage == 0 {
			break
		}
		step := page.Pagination.PageSize
		if step <= 0 {
			step = changesOnPage
		}
		nextOffset := page.Pagination.Offset + step
		if total != nil && nextOffset >= *total {
			break
		}
		if total == nil && changesOnPage < step {
			break
		}
		if nextOffset <= offset {
			nextOffset = offset + changesOnPage
		}
		offset = nextOffset
	}
	resultJSON, err := json.Marshal(aggregated)
	if err != nil {
		return nil, err
	}
	paginationJSON, err := json.Marshal(platformPageDetail{
		TotalCount: total,
		Offset:     max(0, startOffset),
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}
	output["result"] = resultJSON
	output["pagination"] = paginationJSON
	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return RawResponse(data), nil
}

func mergeChangeHistoryResults(dst, src []json.RawMessage) ([]json.RawMessage, error) {
	for srcIndex, srcResult := range src {
		srcObject, err := rawObject(srcResult, "change-history result")
		if err != nil {
			return nil, err
		}
		dstIndex := matchingRawObject(dst, "detailId", rawString(srcObject["detailId"]), srcIndex)
		if dstIndex < 0 {
			dst = append(dst, srcResult)
			continue
		}
		dstObject, err := rawObject(dst[dstIndex], "change-history result")
		if err != nil {
			return nil, err
		}
		var dstDetails, srcDetails []json.RawMessage
		if err := json.Unmarshal(dstObject["details"], &dstDetails); err != nil {
			return nil, fmt.Errorf("parse change-history details: %w", err)
		}
		if err := json.Unmarshal(srcObject["details"], &srcDetails); err != nil {
			return nil, fmt.Errorf("parse change-history details: %w", err)
		}
		for detailIndex, srcDetail := range srcDetails {
			srcDetailObject, err := rawObject(srcDetail, "change-history activity detail")
			if err != nil {
				return nil, err
			}
			target := detailIndex
			if target >= len(dstDetails) {
				dstDetails = append(dstDetails, srcDetail)
				continue
			}
			dstDetailObject, err := rawObject(dstDetails[target], "change-history activity detail")
			if err != nil {
				return nil, err
			}
			var dstChanges, srcChanges []json.RawMessage
			if err := json.Unmarshal(dstDetailObject["changes"], &dstChanges); err != nil {
				return nil, fmt.Errorf("parse change-history changes: %w", err)
			}
			if err := json.Unmarshal(srcDetailObject["changes"], &srcChanges); err != nil {
				return nil, fmt.Errorf("parse change-history changes: %w", err)
			}
			dstChanges = append(dstChanges, srcChanges...)
			dstDetailObject["changes"], err = json.Marshal(dstChanges)
			if err != nil {
				return nil, err
			}
			dstDetails[target], err = json.Marshal(dstDetailObject)
			if err != nil {
				return nil, err
			}
		}
		dstObject["details"], err = json.Marshal(dstDetails)
		if err != nil {
			return nil, err
		}
		dst[dstIndex], err = json.Marshal(dstObject)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func countChangeHistoryChanges(results []json.RawMessage) (int, error) {
	total := 0
	for _, result := range results {
		resultObject, err := rawObject(result, "change-history result")
		if err != nil {
			return 0, err
		}
		var details []json.RawMessage
		if err := json.Unmarshal(resultObject["details"], &details); err != nil {
			return 0, fmt.Errorf("parse change-history details: %w", err)
		}
		for _, detail := range details {
			detailObject, err := rawObject(detail, "change-history activity detail")
			if err != nil {
				return 0, err
			}
			var changes []json.RawMessage
			if err := json.Unmarshal(detailObject["changes"], &changes); err != nil {
				return 0, fmt.Errorf("parse change-history changes: %w", err)
			}
			total += len(changes)
		}
	}
	return total, nil
}

func rawObject(raw json.RawMessage, label string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	return object, nil
}

func matchingRawObject(objects []json.RawMessage, key, value string, fallback int) int {
	if value != "" {
		for index, raw := range objects {
			object, err := rawObject(raw, key)
			if err == nil && rawString(object[key]) == value {
				return index
			}
		}
	}
	if fallback >= 0 && fallback < len(objects) {
		return fallback
	}
	return -1
}

func rawString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func (c *Client) paginatePlatformGET(ctx context.Context, spec EndpointSpec, pathParams map[string]string, query url.Values, startOffset, pageSize int, body json.RawMessage) (RawResponse, error) {
	if spec.Method != "GET" || len(body) != 0 {
		return nil, fmt.Errorf("platform API v1 body pagination is not supported by the GET offset paginator")
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := max(0, startOffset)
	pageSizeParam := platformPageSizeParam(spec)
	var aggregated []json.RawMessage
	var total *int
	pages := 0
	for {
		if pages >= MaxPlatformPaginationPages {
			return nil, fmt.Errorf("platform API v1 pagination exceeded the %d-page safety limit; narrow your query or use --offset to continue from a smaller result set", MaxPlatformPaginationPages)
		}
		pages++
		pageQuery := cloneValues(query)
		pageQuery.Set(pageSizeParam, strconv.Itoa(pageSize))
		pageQuery.Set("offset", strconv.Itoa(offset))
		raw, err := c.Do(ctx, spec, pathParams, pageQuery, body)
		if err != nil {
			return nil, err
		}
		var page platformPaginatedEnvelope
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse paginated response: %w", err)
		}
		aggregated = append(aggregated, page.Result...)
		if page.Pagination.TotalCount != nil {
			value := *page.Pagination.TotalCount
			total = &value
		}
		if len(page.Result) == 0 {
			break
		}
		step := page.Pagination.PageSize
		if step <= 0 {
			step = len(page.Result)
		}
		nextOffset := page.Pagination.Offset + step
		if total != nil && nextOffset >= *total {
			break
		}
		if total == nil && len(page.Result) < step {
			break
		}
		if nextOffset <= offset {
			nextOffset = offset + len(page.Result)
		}
		offset = nextOffset
	}

	out := platformPaginatedEnvelope{
		Result: aggregated,
		Pagination: platformPageDetail{
			TotalCount: total,
			Offset:     max(0, startOffset),
			PageSize:   pageSize,
		},
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return RawResponse(data), nil
}

// platformPageSizeParam returns the query parameter used to control the page
// size for a Platform API v1 GET endpoint. Most endpoints use limit, while
// geo search follows the service's pageSize spelling.
func platformPageSizeParam(spec EndpointSpec) string {
	for _, param := range spec.QueryParams {
		if param.Name == "pageSize" {
			return "pageSize"
		}
	}
	return "limit"
}

// MaxPageLimit returns the endpoint-specific maximum page size.
func MaxPageLimit(spec EndpointSpec) int {
	for _, param := range spec.QueryParams {
		if param.Name == "limit" && param.Max > 0 {
			return param.Max
		}
	}
	return 0
}

func cloneValues(values url.Values) url.Values {
	cloned := url.Values{}
	for key, items := range values {
		for _, item := range items {
			cloned.Add(key, item)
		}
	}
	return cloned
}
