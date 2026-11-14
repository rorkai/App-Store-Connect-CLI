package xcode

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const buildUploadLookupLimit = 200

func waitForBuildUploadID(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time, pollInterval time.Duration) (string, error) {
	if client == nil {
		return "", fmt.Errorf("client is required")
	}
	if pollInterval <= 0 {
		pollInterval = shared.PublishDefaultPollInterval
	}

	return asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (string, bool, error) {
		return findRecentBuildUploadID(ctx, client, appID, version, buildNumber, platform, exportStartedAt, exportCompletedAt)
	})
}

func findRecentBuildUploadID(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time) (string, bool, error) {
	if !exportStartedAt.IsZero() && !exportCompletedAt.IsZero() && exportCompletedAt.Before(exportStartedAt) {
		exportCompletedAt = exportStartedAt
	}
	resp, err := client.GetBuildUploads(
		ctx, appID,
		asc.WithBuildUploadsCFBundleShortVersionStrings([]string{version}),
		asc.WithBuildUploadsCFBundleVersions([]string{buildNumber}),
		asc.WithBuildUploadsPlatforms([]string{platform}),
		asc.WithBuildUploadsSort("-uploadedDate"),
		asc.Limit[asc.BuildUploadsOption](buildUploadLookupLimit),
	)
	if err != nil {
		return "", false, err
	}

	for {
		for _, upload := range resp.Data {
			associationAt, hasAssociationAt := buildUploadAssociationTime(upload.Attributes)
			if !hasAssociationAt {
				if !exportStartedAt.IsZero() {
					continue
				}
				return strings.TrimSpace(upload.ID), true, nil
			}
			if !exportCompletedAt.IsZero() && associationAt.After(exportCompletedAt) {
				continue
			}
			if !exportStartedAt.IsZero() && associationAt.Before(exportStartedAt) {
				continue
			}
			return strings.TrimSpace(upload.ID), true, nil
		}

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return "", false, nil
		}
		resp, err = client.GetBuildUploads(ctx, appID, asc.NextURL[asc.BuildUploadsOption](nextURL))
		if err != nil {
			return "", false, err
		}
	}
}

func buildUploadAssociationTime(attr asc.BuildUploadAttributes) (time.Time, bool) {
	for _, candidate := range []*string{attr.CreatedDate, attr.UploadedDate} {
		parsed, ok := parseBuildUploadTime(candidate)
		if ok {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseBuildUploadTime(value *string) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	candidate := strings.TrimSpace(*value)
	if candidate == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, candidate)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
