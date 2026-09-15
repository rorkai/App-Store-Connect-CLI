package publish

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func findOrCreatePublishAppStoreVersion(ctx context.Context, client *asc.Client, appID, version string, platform asc.Platform) (*asc.AppStoreVersionResponse, error) {
	appID = strings.TrimSpace(appID)
	version = strings.TrimSpace(version)
	platformValue := strings.ToUpper(strings.TrimSpace(string(platform)))
	if appID == "" || version == "" || platformValue == "" {
		return nil, fmt.Errorf("app ID, version, and platform are required")
	}

	readVersion := func(parent context.Context) (*asc.AppStoreVersionResponse, bool, error) {
		versions, err := shared.RetryReadWithFreshTimeout(parent, func(requestCtx context.Context) (*asc.AppStoreVersionsResponse, error) {
			return client.GetAppStoreVersions(
				requestCtx,
				appID,
				asc.WithAppStoreVersionsVersionStrings([]string{version}),
				asc.WithAppStoreVersionsPlatforms([]string{platformValue}),
				asc.WithAppStoreVersionsLimit(10),
			)
		})
		if err != nil {
			return nil, false, err
		}

		switch len(versions.Data) {
		case 0:
			return nil, false, nil
		case 1:
			return &asc.AppStoreVersionResponse{Data: versions.Data[0]}, true, nil
		default:
			return nil, false, shared.AmbiguousAppStoreVersionError(version, platformValue, versions.Data, "", "")
		}
	}

	if existing, found, err := readVersion(ctx); err != nil || found {
		return existing, err
	}

	created, _, err := shared.RunReconciledMutation(
		ctx,
		func(requestCtx context.Context) (*asc.AppStoreVersionResponse, error) {
			return client.CreateAppStoreVersion(requestCtx, appID, asc.AppStoreVersionCreateAttributes{
				Platform:      asc.Platform(platformValue),
				VersionString: version,
			})
		},
		func(readbackCtx context.Context) (*asc.AppStoreVersionResponse, bool, error) {
			return readVersion(readbackCtx)
		},
	)
	return created, err
}
