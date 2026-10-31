package assetlibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	localizationResource = "appStoreVersionLocalizations"
	nativeMediaCategory  = "APP_SCREENSHOTS_AND_PREVIEWS"
)

// LocalizationUpload describes media to upload to the Asset Library and place
// on one App Store version localization, in the order of Files.
type LocalizationUpload struct {
	Command        string
	LocalizationID string
	PlacementType  string
	Slot           string
	PlacementGroup string
	Files          []string
	DryRun         bool
}

// libraryFile is a validated file ready to send to the library.
type libraryFile struct {
	path  string
	send  func(ctx context.Context, client *asc.Client, libraryID string) (string, error)
	close func()
}

// UploadToLocalization validates everything it can before its first write, then
// uploads every file before placing any, so that an upload failure leaves the
// localization unchanged. A returned non-nil result is a receipt to print even
// when the error is also non-nil. Nothing is deleted automatically.
func UploadToLocalization(ctx context.Context, client *asc.Client, upload LocalizationUpload) (*asc.AssetLibraryLocalizationUploadResult, error) {
	libraryID, err := localizationLibraryID(ctx, client, upload.LocalizationID)
	if err != nil {
		return nil, err
	}
	reference, err := getLibraryResource(ctx, client, "/v1/appAssetLibraryRefData")
	if err != nil {
		return nil, fmt.Errorf("read specifications: %w", err)
	}
	group, maxCount, err := resolvePlacementGroup(reference, upload)
	if err != nil {
		return nil, err
	}
	existing, err := existingGroupPlacements(ctx, client, upload.LocalizationID, upload.PlacementType, group)
	if err != nil {
		return nil, err
	}
	if total := len(existing) + len(upload.Files); total > maxCount {
		return nil, fmt.Errorf("placement group %s allows %d placement(s) for %s; the localization has %d and this upload adds %d. Remove placements with asc localizations placements delete, or upload fewer files", group, maxCount, upload.PlacementType, len(existing), len(upload.Files))
	}
	files, err := prepareLibraryFiles(ctx, upload, group, reference)
	defer func() {
		for _, file := range files {
			file.close()
		}
	}()
	if err != nil {
		return nil, err
	}
	result := &asc.AssetLibraryLocalizationUploadResult{
		VersionLocalizationID: upload.LocalizationID, LibraryID: libraryID, PlacementType: upload.PlacementType,
		PlacementGroup: group, MaxCount: maxCount, ExistingPlacementIDs: existing, DryRun: upload.DryRun,
		Items: make([]asc.AssetLibraryLocalizationUploadItem, len(files)),
	}
	initialState := "pending"
	if upload.DryRun {
		initialState = "would-upload"
	}
	for i, file := range files {
		result.Items[i] = asc.AssetLibraryLocalizationUploadItem{FileName: filepath.Base(file.path), FilePath: file.path, State: initialState}
	}
	if upload.DryRun {
		return result, nil
	}
	if err := uploadLibraryFiles(ctx, client, libraryID, files, result); err != nil {
		return result, err
	}
	if err := placeLibraryFiles(ctx, client, upload, result); err != nil {
		return result, err
	}
	placementIDs := slices.Clone(existing)
	for _, item := range result.Items {
		placementIDs = append(placementIDs, item.PlacementID)
	}
	if len(placementIDs) > 1 {
		result.OrderingRequestID, err = shared.OrderLibraryPlacements(ctx, client, localizationResource, upload.LocalizationID, group, placementIDs)
		if err != nil {
			return result, fmt.Errorf("%d placed in %s, but they could not be put in order: %w. Inspect with %s, then use asc localizations placements reorder", len(result.Items), group, err, placementInspection(result))
		}
	}
	result.Ordered, result.Complete = true, true
	return result, nil
}

func uploadLibraryFiles(ctx context.Context, client *asc.Client, libraryID string, files []libraryFile, result *asc.AssetLibraryLocalizationUploadResult) error {
	for i, file := range files {
		item := &result.Items[i]
		var err error
		item.AssetID, err = file.send(ctx, client, libraryID)
		if err != nil {
			item.State, item.Error = "upload-failed", err.Error()
			return fmt.Errorf("upload %s: %w (%d of %d file(s) uploaded, 0 placed; the localization is unchanged, and the receipt lists uploaded assets)", item.FileName, err, i, len(files))
		}
		item.State = "uploaded"
	}
	return nil
}

func placeLibraryFiles(ctx context.Context, client *asc.Client, upload LocalizationUpload, result *asc.AssetLibraryLocalizationUploadResult) error {
	relationship := "image"
	if upload.PlacementType == "APP_PREVIEW" {
		relationship = "video"
	}
	for i := range result.Items {
		item := &result.Items[i]
		placementID, state, err := shared.CreateLibraryPlacement(ctx, client, localizationResource, upload.LocalizationID, relationship, item.AssetID, upload.PlacementType, result.PlacementGroup)
		if err != nil {
			item.State, item.Error = "placement-failed", err.Error()
			return fmt.Errorf("place %s: %w (%d of %d file(s) uploaded, %d placed; the receipt lists every asset. Inspect with %s before retrying, then place the rest with asc localizations placements create)", item.FileName, err, len(result.Items), len(result.Items), i, placementInspection(result))
		}
		item.PlacementID, item.PlacementState, item.State = placementID, state, "placed"
	}
	return nil
}

func placementInspection(result *asc.AssetLibraryLocalizationUploadResult) string {
	return fmt.Sprintf("asc localizations placements list --localization-id %s --placement-group %s --sort placementGroupPosition", result.VersionLocalizationID, result.PlacementGroup)
}

func prepareLibraryFiles(ctx context.Context, upload LocalizationUpload, group string, reference []byte) ([]libraryFile, error) {
	files := make([]libraryFile, 0, len(upload.Files))
	var videoSpecs map[string]bool
	if upload.PlacementType == "APP_PREVIEW" {
		var err error
		if videoSpecs, err = groupSpecIDs(reference, upload.PlacementType, group); err != nil {
			return nil, err
		}
	}
	for _, path := range upload.Files {
		var (
			file libraryFile
			err  error
		)
		if upload.PlacementType == "APP_PREVIEW" {
			file, err = prepareLibraryVideo(ctx, upload.Command, path, reference, videoSpecs)
		} else {
			file, err = prepareLibraryImage(upload.Command, path)
		}
		if err != nil {
			return files, err
		}
		files = append(files, file)
	}
	return files, nil
}

func prepareLibraryImage(command, path string) (libraryFile, error) {
	file, size, err := openLibraryImage(command, "--path", path)
	if err != nil {
		return libraryFile{}, err
	}
	return libraryFile{path: path, close: func() { file.Close() }, send: func(ctx context.Context, client *asc.Client, libraryID string) (string, error) {
		uploadCtx, cancel := shared.ContextWithUploadTimeout(ctx)
		defer cancel()
		result, err := client.UploadAssetLibraryImageWithCategory(uploadCtx, libraryID, filepath.Base(path), file, size, nativeMediaCategory, "", shared.ContextWithTimeout)
		return result.ImageID, err
	}}, nil
}

func prepareLibraryVideo(ctx context.Context, command, path string, reference []byte, specs map[string]bool) (libraryFile, error) {
	staged, err := stageLibraryVideo(ctx, command, "--path", path)
	if err != nil {
		return libraryFile{}, err
	}
	if err := matchLibraryVideoSpecIn(reference, nativeMediaCategory, specs, staged.properties, staged.size, filepath.Ext(path)); err != nil {
		staged.close()
		return libraryFile{}, shared.UsageErrorf("%s: %s: %v", command, filepath.Base(path), err)
	}
	return libraryFile{path: path, close: staged.close, send: func(ctx context.Context, client *asc.Client, libraryID string) (string, error) {
		uploadCtx, cancel := libraryVideoUploadContext(ctx)
		defer cancel()
		result, err := client.UploadAssetLibraryVideoWithCategory(uploadCtx, libraryID, filepath.Base(path), staged.file, staged.size, nativeMediaCategory, "", shared.ContextWithTimeout)
		return result.VideoID, err
	}}, nil
}

// groupSpecIDs returns the specifications Apple accepts for a placement group.
func groupSpecIDs(reference []byte, placementType, group string) (map[string]bool, error) {
	var envelope struct {
		Data []struct {
			Attributes struct {
				PlacementTypes []struct {
					ID           string `json:"placementTypeId"`
					SpecMappings []struct {
						GroupID string   `json:"placementGroupId"`
						Specs   []string `json:"specs"`
					} `json:"specMappings"`
				} `json:"placementTypes"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reference, &envelope); err != nil {
		return nil, fmt.Errorf("invalid asset reference data: %w", err)
	}
	specs := map[string]bool{}
	for _, datum := range envelope.Data {
		for _, kind := range datum.Attributes.PlacementTypes {
			if kind.ID != placementType {
				continue
			}
			for _, mapping := range kind.SpecMappings {
				if mapping.GroupID == group {
					for _, id := range mapping.Specs {
						specs[id] = true
					}
				}
			}
		}
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("asset reference data maps no specifications to %s for %s", group, placementType)
	}
	return specs, nil
}

// localizationLibraryID follows the localization to its own app, so the
// configured default app cannot select another app's library.
func localizationLibraryID(ctx context.Context, client *asc.Client, localizationID string) (string, error) {
	versionID, err := relatedResourceID(ctx, client, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localizationID)+"?include=appStoreVersion", "appStoreVersion")
	if err != nil {
		return "", fmt.Errorf("find version of localization %s: %w", localizationID, err)
	}
	appID, err := relatedResourceID(ctx, client, "/v1/appStoreVersions/"+url.PathEscape(versionID)+"?include=app", "app")
	if err != nil {
		return "", fmt.Errorf("find app of version %s: %w", versionID, err)
	}
	raw, err := getLibraryResource(ctx, client, "/v1/apps/"+url.PathEscape(appID)+"/assetLibrary")
	if err != nil {
		return "", fmt.Errorf("find Asset Library of app %s: %w", appID, err)
	}
	var library struct {
		Data struct{ ID, Type string } `json:"data"`
	}
	if err := json.Unmarshal(raw, &library); err != nil {
		return "", fmt.Errorf("invalid Asset Library response: %w", err)
	}
	if library.Data.ID == "" || library.Data.Type != "appAssetLibraries" {
		return "", errors.New("invalid Asset Library response")
	}
	return library.Data.ID, nil
}

func relatedResourceID(ctx context.Context, client *asc.Client, path, relationship string) (string, error) {
	raw, err := getLibraryResource(ctx, client, path)
	if err != nil {
		return "", err
	}
	var resource struct {
		Data struct {
			Relationships map[string]struct {
				Data struct{ ID string } `json:"data"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resource); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	id := resource.Data.Relationships[relationship].Data.ID
	if id == "" {
		return "", fmt.Errorf("response has no %s relationship", relationship)
	}
	return id, nil
}

func getLibraryResource(ctx context.Context, client *asc.Client, path string) ([]byte, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.RawRequest(requestCtx, "GET", path, nil)
}

func existingGroupPlacements(ctx context.Context, client *asc.Client, localizationID, placementType, group string) ([]string, error) {
	query := url.Values{"filter[placementType]": {placementType}, "filter[placementGroup]": {group}, "sort": {"placementGroupPosition"}, "limit": {"200"}}
	raw, err := client.RawPaginatedGET(ctx, "/v1/"+localizationResource+"/"+url.PathEscape(localizationID)+"/placements?"+query.Encode(), shared.ContextWithTimeout)
	if err != nil {
		return nil, fmt.Errorf("list existing placements: %w", err)
	}
	var response struct {
		Data []struct{ ID string } `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("list existing placements: invalid response: %w", err)
	}
	ids := make([]string, len(response.Data))
	for i, item := range response.Data {
		ids[i] = item.ID
	}
	return ids, nil
}

// resolvePlacementGroup reads the group from Apple's reference data, because
// Apple publishes no naming rule for group IDs and adds families without an API
// version change. It returns the group and its placement limit.
func resolvePlacementGroup(reference []byte, upload LocalizationUpload) (string, int, error) {
	var envelope struct {
		Data []struct {
			Attributes struct {
				Features []struct {
					FeatureID         string `json:"featureId"`
					PlacementPolicies []struct {
						PlacementType string `json:"placementType"`
						GroupLimits   []struct {
							GroupIDs []string `json:"groupIds"`
							MaxCount int      `json:"maxCount"`
						} `json:"groupLimits"`
					} `json:"placementPolicies"`
				} `json:"features"`
				Groups []struct {
					ID           string `json:"placementProfileGroupId"`
					Platform     string `json:"platform"`
					DisplayClass string `json:"displayClassId"`
				} `json:"placementProfileGroups"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reference, &envelope); err != nil {
		return "", 0, fmt.Errorf("invalid asset reference data: %w", err)
	}
	limits := map[string]int{}
	profile, hasProfile := asc.AssetLibraryProfileForSlot(upload.Slot)
	matches := []string{}
	for _, datum := range envelope.Data {
		for _, feature := range datum.Attributes.Features {
			if feature.FeatureID != "APP_STORE_VERSIONS" {
				continue
			}
			for _, policy := range feature.PlacementPolicies {
				if policy.PlacementType != upload.PlacementType {
					continue
				}
				for _, limit := range policy.GroupLimits {
					for _, id := range limit.GroupIDs {
						limits[id] = limit.MaxCount
					}
				}
			}
		}
		for _, group := range datum.Attributes.Groups {
			if hasProfile && group.DisplayClass == profile.DisplayClass && group.Platform == profile.Platform {
				if _, offered := limits[group.ID]; offered {
					matches = append(matches, group.ID)
				}
			}
		}
	}
	offered := make([]string, 0, len(limits))
	for id := range limits {
		offered = append(offered, id)
	}
	slices.Sort(offered)
	if upload.PlacementGroup != "" {
		maxCount, ok := limits[upload.PlacementGroup]
		if !ok {
			return "", 0, shared.UsageErrorf("placement group %q is not offered for %s on App Store versions; offered groups: %s", upload.PlacementGroup, upload.PlacementType, strings.Join(offered, ", "))
		}
		return upload.PlacementGroup, maxCount, nil
	}
	if len(matches) != 1 {
		return "", 0, shared.UsageErrorf("%d placement groups match %s for %s; choose one with --placement-group from: %s", len(matches), upload.Slot, upload.PlacementType, strings.Join(offered, ", "))
	}
	return matches[0], limits[matches[0]], nil
}
