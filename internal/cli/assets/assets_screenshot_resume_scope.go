package assets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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
	mu                    sync.Mutex
	config                screenshotUploadConfig[asc.AppScreenshotUploadResult]
	setID                 string
	members               map[string]bool
	unobservedReservation string
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

// A successful create can precede its appearance in either membership view.
// All writes hold mu, so only the last reservation may still be propagating.
func (scope *screenshotResumeScope) verifyLiveMembership(ctx context.Context) error {
	const maxChecks = 3
	checks := 0
	_, err := asc.PollUntil(ctx, min(2*time.Second, asc.MediaPollInterval(ctx, 1)), func(ctx context.Context) (bool, bool, error) {
		checks++
		items, order, err := replacementInventory(ctx, scope.config, scope.setID)
		if err != nil && (scope.unobservedReservation == "" || !errors.Is(err, errScreenshotInventoryChanged)) {
			return false, false, err
		}
		// Each view has unique IDs. Every previously observed member must be in both.
		appearances := make(map[string]int, len(scope.members))
		for _, item := range items {
			appearances[item.ID]++
		}
		for _, id := range order {
			appearances[id]++
		}
		for id := range appearances {
			if !scope.members[id] {
				return false, false, fmt.Errorf("screenshot membership changed during resume")
			}
		}
		for id := range scope.members {
			if appearances[id] != 2 && id != scope.unobservedReservation {
				return false, false, fmt.Errorf("screenshot membership changed during resume")
			}
		}
		if id := scope.unobservedReservation; id != "" && appearances[id] != 2 {
			if checks == maxChecks {
				return false, false, fmt.Errorf("screenshot reservation %q is not yet visible in both views of the selected set; retry after it appears", id)
			}
			return false, false, nil
		}
		scope.unobservedReservation = ""
		return true, true, nil
	})
	return err
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
	if err := scope.verifyLiveMembership(ctx); err != nil {
		return reject(err)
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

// Delivery polls read the asset from its independently selected set. The initial
// pending-asset read and each mutation still verify the full ownership chain.
func getScreenshotDelivery(ctx context.Context, client *asc.Client, assetID string) (*asc.AppScreenshotResponse, error) {
	scope, _ := ctx.Value(screenshotResumeScopeKey{}).(*screenshotResumeScope)
	if scope == nil {
		return getScopedAppScreenshot(ctx, client, assetID)
	}
	scope.mu.Lock()
	known := client == scope.config.Client && scope.members[assetID]
	scope.mu.Unlock()
	if !known {
		return nil, fmt.Errorf("screenshot %q does not belong to the independently selected set", assetID)
	}
	response, err := getScopedAppScreenshots(ctx, client, scope.setID, scope.config.RequestContext)
	if err != nil {
		return nil, err
	}
	var found *asc.AppScreenshotResponse
	for _, item := range response.Data {
		if item.ID == assetID {
			if item.Type != asc.ResourceTypeAppScreenshots || found != nil {
				return nil, fmt.Errorf("cannot verify screenshot %q in selected set", assetID)
			}
			found = &asc.AppScreenshotResponse{Data: item}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("screenshot %q is missing from the selected set", assetID)
	}
	return found, nil
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
		if response.Data.Type != asc.ResourceTypeAppScreenshots || scope.members[response.Data.ID] {
			return nil, fmt.Errorf("screenshot reservation response does not identify a new screenshot")
		}
		scope.members[response.Data.ID] = true
		scope.unobservedReservation = response.Data.ID
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
