package app_events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const appEventAssetPollInterval = 2 * time.Second

func waitForAppEventScreenshotDelivery(ctx context.Context, d *shared.Deps, client *asc.Client, screenshotID string) (*asc.AppEventScreenshotResponse, error) {
	var lastResp *asc.AppEventScreenshotResponse
	for {
		resp, err := client.GetAppEventScreenshot(ctx, screenshotID)
		if err != nil {
			return lastResp, err
		}
		lastResp = resp

		state, errors := resolveAppEventAssetState(resp.Data.Attributes.AssetDeliveryState)
		if state != "" {
			switch state {
			case "COMPLETE":
				return resp, nil
			case "FAILED":
				return resp, fmt.Errorf("asset %s delivery failed: %s", screenshotID, formatAppEventStateErrors(errors))
			}
		}

		select {
		case <-ctx.Done():
			return lastResp, fmt.Errorf("timed out waiting for asset %s delivery: %w", screenshotID, ctx.Err())
		case <-d.After(appEventAssetPollInterval):
		}
	}
}

func waitForAppEventVideoClipDelivery(ctx context.Context, d *shared.Deps, client *asc.Client, clipID string) (*asc.AppEventVideoClipResponse, error) {
	var lastResp *asc.AppEventVideoClipResponse
	for {
		resp, err := client.GetAppEventVideoClip(ctx, clipID)
		if err != nil {
			return lastResp, err
		}
		lastResp = resp

		state, errors := resolveAppEventVideoState(resp.Data.Attributes)
		if state != "" {
			switch state {
			case "COMPLETE":
				return resp, nil
			case "FAILED":
				return resp, fmt.Errorf("video clip %s delivery failed: %s", clipID, formatAppEventStateErrors(errors))
			}
		}

		select {
		case <-ctx.Done():
			return lastResp, fmt.Errorf("timed out waiting for video clip %s delivery: %w", clipID, ctx.Err())
		case <-d.After(appEventAssetPollInterval):
		}
	}
}

func resolveAppEventAssetState(state *asc.AppMediaAssetState) (string, []asc.StateDetail) {
	if state == nil || state.State == nil {
		return "", nil
	}
	return strings.ToUpper(strings.TrimSpace(*state.State)), state.Errors
}

func resolveAppEventVideoState(attrs asc.AppEventVideoClipAttributes) (string, []asc.StateDetail) {
	if attrs.VideoDeliveryState != nil && attrs.VideoDeliveryState.State != nil {
		return strings.ToUpper(strings.TrimSpace(*attrs.VideoDeliveryState.State)), attrs.VideoDeliveryState.Errors
	}
	return resolveAppEventAssetState(attrs.AssetDeliveryState)
}

func formatAppEventStateErrors(details []asc.StateDetail) string {
	if len(details) == 0 {
		return "unknown error"
	}
	parts := make([]string, 0, len(details))
	for _, item := range details {
		if item.Code != "" && item.Message != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", item.Code, item.Message))
			continue
		}
		if item.Message != "" {
			parts = append(parts, item.Message)
			continue
		}
		if item.Code != "" {
			parts = append(parts, item.Code)
		}
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}
