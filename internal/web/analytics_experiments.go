package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// AnalyticsExperimentDefaultBaseline identifies the original product page in analytics requests.
const AnalyticsExperimentDefaultBaseline = "default"

// NormalizeAnalyticsExperimentID validates an analytics experiment or treatment UUID.
func NormalizeAnalyticsExperimentID(value string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid experiment or treatment UUID: %w", err)
	}
	return id.String(), nil
}

// NormalizeAnalyticsExperimentBaseline accepts the original page or a treatment UUID.
func NormalizeAnalyticsExperimentBaseline(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == AnalyticsExperimentDefaultBaseline {
		return value, nil
	}
	return NormalizeAnalyticsExperimentID(value)
}

// GetAnalyticsExperiments returns Apple's complete product page optimization list envelope.
func (c *Client) GetAnalyticsExperiments(ctx context.Context, appID string) (json.RawMessage, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	return c.getAnalyticsExperimentJSON(ctx, "/app-info/"+url.PathEscape(appID)+"/ab-tests", analyticsAppReferer(appID), "results")
}

// GetAnalyticsExperimentResults returns the complete result, including intervals and daily data.
func (c *Client) GetAnalyticsExperimentResults(ctx context.Context, experimentID, baseline string) (json.RawMessage, error) {
	experimentID, err := NormalizeAnalyticsExperimentID(experimentID)
	if err != nil {
		return nil, err
	}
	baseline, err = NormalizeAnalyticsExperimentBaseline(baseline)
	if err != nil {
		return nil, err
	}
	query := url.Values{"baseline": {baseline}}
	return c.getAnalyticsExperimentJSON(ctx, "/ab-tests/"+url.PathEscape(experimentID)+"?"+query.Encode(), appStoreBaseURL+"/apps", "treatments")
}

func (c *Client) getAnalyticsExperimentJSON(ctx context.Context, path, referer, collection string) (json.RawMessage, error) {
	body, err := c.doAnalyticsGetRequest(ctx, path, referer)
	if err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse analytics experiment response: %w", err)
	}
	if value := strings.TrimSpace(string(envelope[collection])); !strings.HasPrefix(value, "[") {
		return nil, fmt.Errorf("analytics experiment response is missing the %s array", collection)
	}
	return json.RawMessage(body), nil
}
