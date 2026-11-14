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
				asc.Limit[asc.AppStoreVersionsOption](10),
			)
		})
		if err != nil {
			return nil, false, err
		}
		if versions == nil {
			return nil, false, fmt.Errorf("empty app store versions response")
		}
		pageHasNext := strings.TrimSpace(versions.Links.Next) != ""

		switch len(versions.Data) {
		case 0:
			if pageHasNext {
				return nil, false, shared.MarkAmbiguousSelectionSample(shared.AmbiguousAppStoreVersionError(version, platformValue, versions.Data, "", ""))
			}
			return nil, false, nil
		case 1:
			if pageHasNext {
				return nil, false, shared.MarkAmbiguousSelectionSample(shared.AmbiguousAppStoreVersionError(version, platformValue, versions.Data, "", ""))
			}
			return &asc.AppStoreVersionResponse{Data: versions.Data[0]}, true, nil
		default:
			ambiguous := shared.AmbiguousAppStoreVersionError(version, platformValue, versions.Data, "", "")
			if pageHasNext {
				ambiguous = shared.MarkAmbiguousSelectionSample(ambiguous)
			}
			return nil, false, ambiguous
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
