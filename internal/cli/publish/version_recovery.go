package publish

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func findOrCreatePublishAppStoreVersion(ctx context.Context, d *shared.Deps, client *asc.Client, appID, version string, platform asc.Platform) (*asc.AppStoreVersionResponse, error) {
	appID = strings.TrimSpace(appID)
	version = strings.TrimSpace(version)
	platformValue := strings.ToUpper(strings.TrimSpace(string(platform)))
	if appID == "" || version == "" || platformValue == "" {
		return nil, fmt.Errorf("app ID, version, and platform are required")
	}

	readVersion := func(parent context.Context) (*asc.AppStoreVersionResponse, bool, error) {
		versions, err := shared.RetryReadWithFreshTimeoutWith(parent, d, func(requestCtx context.Context) (*asc.AppStoreVersionsResponse, error) {
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

	readBackFailed := false
	created, _, err := shared.RunReconciledMutationWith(
		ctx,
		d,
		func(requestCtx context.Context) (*asc.AppStoreVersionResponse, error) {
			return client.CreateAppStoreVersion(requestCtx, appID, asc.AppStoreVersionCreateAttributes{
				Platform:      asc.Platform(platformValue),
				VersionString: version,
			})
		},
		func(readbackCtx context.Context) (*asc.AppStoreVersionResponse, bool, error) {
			existing, found, readErr := readVersion(readbackCtx)
			readBackFailed = readBackFailed || readErr != nil
			return existing, found, readErr
		},
	)
	if err != nil {
		// Name the versions that block the create, unless the read-back
		// already failed, in which case another read is unlikely to help.
		if !readBackFailed {
			err = shared.WithAppStoreVersionCreateConflictRerunDiagnostics(ctx, client, appID, version, platformValue, err)
		}
		return nil, err
	}
	return created, nil
}
