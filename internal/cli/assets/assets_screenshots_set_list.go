package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// screenshotSetListFields mirrors fields[appScreenshots] for
// GET /v1/appScreenshotSets/{id}/appScreenshots.
var screenshotSetListFields = []string{"fileSize", "fileName", "sourceFileChecksum", "imageAsset", "assetToken", "assetType", "uploadOperations", "assetDeliveryState", "appScreenshotSet"}

func executeScreenshotSetListCommand(ctx context.Context, setID, fields string, limit int, paginate bool, output shared.OutputFlags) error {
	query := url.Values{}
	if strings.TrimSpace(fields) != "" {
		values := strings.Split(fields, ",")
		for i, value := range values {
			value = strings.TrimSpace(value)
			if !slices.Contains(screenshotSetListFields, value) {
				return shared.UsageErrorf("screenshots list: --fields must be one of: %s", strings.Join(screenshotSetListFields, ", "))
			}
			values[i] = value
		}
		query.Set("fields[appScreenshots]", strings.Join(values, ","))
	}
	if paginate && limit == 0 {
		limit = 200
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	target := "/v1/appScreenshotSets/" + url.PathEscape(setID) + "/appScreenshots"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("screenshots list: %w", err)
	}
	var response []byte
	if paginate {
		response, err = client.RawPaginatedGET(ctx, target, shared.ContextWithTimeout)
	} else {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		response, err = client.RawRequest(requestCtx, "GET", target, nil)
	}
	if err != nil {
		return fmt.Errorf("screenshots list: %w", err)
	}
	var page struct {
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal(response, &page); err == nil && page.Links.Next != "" {
		fmt.Fprintln(os.Stderr, "Warning: more pages exist; use --paginate to list every screenshot in the set")
	}
	headers, rows, err := asc.AppScreenshotRows(response)
	if err != nil {
		return fmt.Errorf("screenshots list: %w", err)
	}
	return shared.PrintOutputRows(json.RawMessage(response), *output.Output, *output.Pretty, headers, rows)
}
