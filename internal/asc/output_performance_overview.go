package asc

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func performanceOverviewRows(resp *PerformanceOverviewResponse) ([]string, [][]string, error) {
	if resp == nil {
		return nil, nil, fmt.Errorf("performance overview response is nil")
	}
	var payload struct {
		AppMetadata struct {
			AppID         string `json:"appId"`
			BundleID      string `json:"bundleId"`
			LatestVersion string `json:"latestVersion"`
			Platform      string `json:"platform"`
		} `json:"appMetadata"`
		Insights struct {
			Regressions []json.RawMessage `json:"regressions"`
			TrendingUp  []json.RawMessage `json:"trendingUp"`
		} `json:"insights"`
		Categories []json.RawMessage `json:"categories"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode performance overview: %w", err)
	}
	return []string{"App ID", "Bundle ID", "Latest Version", "Platform", "Categories", "Regressions", "Trending Up"}, [][]string{{payload.AppMetadata.AppID, payload.AppMetadata.BundleID, payload.AppMetadata.LatestVersion, payload.AppMetadata.Platform, strconv.Itoa(len(payload.Categories)), strconv.Itoa(len(payload.Insights.Regressions)), strconv.Itoa(len(payload.Insights.TrendingUp))}}, nil
}
