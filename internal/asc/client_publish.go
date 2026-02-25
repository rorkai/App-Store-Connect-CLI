package asc

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WaitForBuildProcessing polls a build until processing completes.
func (c *Client) WaitForBuildProcessing(ctx context.Context, buildID string, pollInterval time.Duration) (*BuildResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("build ID is required")
	}
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}

	return PollUntil(ctx, pollInterval, func(ctx context.Context) (*BuildResponse, bool, error) {
		build, err := c.GetBuild(ctx, buildID)
		if err != nil {
			return nil, false, err
		}

		state := strings.ToUpper(strings.TrimSpace(build.Data.Attributes.ProcessingState))
		switch state {
		case BuildProcessingStateValid:
			return build, true, nil
		case BuildProcessingStateFailed:
			return nil, false, fmt.Errorf("build processing failed: %s", state)
		case BuildProcessingStateInvalid:
			return nil, false, fmt.Errorf("build processing failed: %s", state)
		}
		return nil, false, nil
	})
}

// FindOrCreateAppStoreVersion finds an existing app store version or creates one.
func (c *Client) FindOrCreateAppStoreVersion(ctx context.Context, appID, version string, platform Platform) (*AppStoreVersionResponse, error) {
	appID = strings.TrimSpace(appID)
	version = strings.TrimSpace(version)
	platformValue := strings.ToUpper(strings.TrimSpace(string(platform)))
	if appID == "" || version == "" || platformValue == "" {
		return nil, fmt.Errorf("app ID, version, and platform are required")
	}

	versions, err := c.GetAppStoreVersions(ctx, appID,
		WithAppStoreVersionsVersionStrings([]string{version}),
		WithAppStoreVersionsPlatforms([]string{platformValue}),
		WithAppStoreVersionsLimit(10),
	)
	if err != nil {
		return nil, err
	}

	switch len(versions.Data) {
	case 0:
		return c.CreateAppStoreVersion(ctx, appID, AppStoreVersionCreateAttributes{
			Platform:      Platform(platformValue),
			VersionString: version,
		})
	case 1:
		return &AppStoreVersionResponse{Data: versions.Data[0]}, nil
	default:
		return nil, fmt.Errorf("multiple app store versions found for version %q and platform %q", version, platformValue)
	}
}
