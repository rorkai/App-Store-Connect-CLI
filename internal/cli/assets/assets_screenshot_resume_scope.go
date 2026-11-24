package assets

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type screenshotResumeTarget struct {
	AppID          string
	LocalizationID string
	DisplayType    string
}

type screenshotResumeScopeKey struct{}

type screenshotResumeScope struct {
	mu      sync.Mutex
	config  screenshotUploadConfig[asc.AppScreenshotUploadResult]
	setID   string
	members map[string]bool
}

func prepareScreenshotResumeScope(ctx context.Context, d *shared.Deps, client *asc.Client, artifact screenshotUploadFailureArtifact, target screenshotResumeTarget) (context.Context, error) {
	appID, err := screenshotMutationAppID(d, target.AppID, "--resume")
	if err != nil {
		return ctx, err
	}
	if target.LocalizationID == "" || target.DisplayType == "" {
		return ctx, fmt.Errorf("--resume requires independently selected --version-localization and --device-type")
	}
	artifactDisplay := strings.TrimSpace(artifact.DisplayType)
	if artifactDisplay == "" && strings.TrimSpace(artifact.DeviceType) != "" {
		artifactDisplay, err = normalizeScreenshotDisplayType(artifact.DeviceType)
		if err != nil {
			return ctx, err
		}
		artifactDisplay = asc.CanonicalScreenshotDisplayTypeForAPI(artifactDisplay)
	}
	if artifact.VersionLocalizationID != target.LocalizationID || artifactDisplay != target.DisplayType {
		return ctx, fmt.Errorf("cannot verify resume receipt against the independently selected localization and device")
	}
	scope := &screenshotResumeScope{
		config: screenshotUploadConfig[asc.AppScreenshotUploadResult]{
			Deps: d, Client: client, AppID: appID, LocalizationID: target.LocalizationID,
			DisplayType: target.DisplayType, Access: appStoreVersionScreenshotSetAccess, RequestContext: d.Timeout,
		},
		setID: artifact.SetID, members: make(map[string]bool),
	}
	_, order, err := replacementInventory(ctx, scope.config, scope.setID)
	if err != nil {
		return ctx, err
	}
	for _, id := range order {
		scope.members[id] = true
	}
	proposed := make(map[string]bool)
	for _, id := range artifact.OrderedIDs {
		if !scope.members[id] || proposed[id] {
			return ctx, fmt.Errorf("resume receipt has invalid screenshot order membership")
		}
		proposed[id] = true
	}
	for _, asset := range append(append([]screenshotPendingAsset{}, artifact.PendingAssets...), artifact.CleanupFailures...) {
		if !scope.members[asset.AssetID] {
			return ctx, fmt.Errorf("pending screenshot %q does not belong to the independently selected set", asset.AssetID)
		}
		proposed[asset.AssetID] = true
	}
	for _, result := range artifact.Results {
		if result.State == "deleted" {
			continue
		}
		if !scope.members[result.AssetID] {
			return ctx, fmt.Errorf("recorded screenshot %q does not belong to the independently selected set", result.AssetID)
		}
		if artifact.SkipExisting || artifact.Replace || artifact.Replacement != nil {
			proposed[result.AssetID] = true
		}
	}
	if !artifact.SkipExisting || len(artifact.Files) == 0 {
		for id := range scope.members {
			if !proposed[id] {
				return ctx, fmt.Errorf("live screenshot membership differs from the resume receipt; inspect the selected set before continuing")
			}
		}
	}
	return context.WithValue(ctx, screenshotResumeScopeKey{}, scope), nil
}

func lockScreenshotResumeScope(ctx context.Context, client *asc.Client, setID, assetID string, orderedIDs []string) (*screenshotResumeScope, func(), error) {
	scope, _ := ctx.Value(screenshotResumeScopeKey{}).(*screenshotResumeScope)
	if scope == nil {
		if resumed, _ := ctx.Value(screenshotResumeKey{}).(bool); resumed {
			return nil, nil, fmt.Errorf("screenshot resume requires an independently verified target")
		}
		return nil, func() {}, nil
	}
	scope.mu.Lock()
	reject := func(err error) (*screenshotResumeScope, func(), error) {
		scope.mu.Unlock()
		return nil, nil, err
	}
	if client != scope.config.Client || (setID != "" && setID != scope.setID) {
		return reject(fmt.Errorf("resume request does not match the independently selected screenshot set"))
	}
	_, order, err := replacementInventory(ctx, scope.config, scope.setID)
	if err != nil {
		return reject(err)
	}
	if len(order) != len(scope.members) {
		return reject(fmt.Errorf("screenshot membership changed during resume"))
	}
	for _, id := range order {
		if !scope.members[id] {
			return reject(fmt.Errorf("screenshot membership changed during resume"))
		}
	}
	if assetID != "" && !scope.members[assetID] {
		return reject(fmt.Errorf("screenshot %q does not belong to the independently selected set", assetID))
	}
	if orderedIDs != nil {
		proposed := make(map[string]bool, len(orderedIDs))
		for _, id := range orderedIDs {
			if !scope.members[id] || proposed[id] {
				return reject(fmt.Errorf("resume order must contain every current screenshot exactly once"))
			}
			proposed[id] = true
		}
		if len(proposed) != len(scope.members) {
			return reject(fmt.Errorf("resume order must contain every current screenshot exactly once"))
		}
	}
	return scope, scope.mu.Unlock, nil
}

func getScopedAppScreenshot(ctx context.Context, client *asc.Client, assetID string) (*asc.AppScreenshotResponse, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, fmt.Errorf("screenshot ID is required")
	}
	_, unlock, err := lockScreenshotResumeScope(ctx, client, "", assetID, nil)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return client.GetAppScreenshot(ctx, assetID)
}

func createScopedAppScreenshot(ctx context.Context, client *asc.Client, setID, fileName string, fileSize int64) (*asc.AppScreenshotResponse, error) {
	scope, unlock, err := lockScreenshotResumeScope(ctx, client, setID, "", nil)
	if err != nil {
		return nil, err
	}
	defer unlock()
	response, err := client.CreateAppScreenshot(ctx, setID, fileName, fileSize)
	if err == nil && scope != nil {
		if response == nil || strings.TrimSpace(response.Data.ID) == "" {
			return nil, fmt.Errorf("screenshot reservation response is missing its ID")
		}
		scope.members[response.Data.ID] = true
	}
	return response, err
}

func commitScopedAppScreenshot(ctx context.Context, client *asc.Client, assetID, checksum string) (*asc.AppScreenshotResponse, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, fmt.Errorf("screenshot ID is required")
	}
	_, unlock, err := lockScreenshotResumeScope(ctx, client, "", assetID, nil)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return client.UpdateAppScreenshot(ctx, assetID, true, checksum)
}
