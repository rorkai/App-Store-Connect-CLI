package assetlibrary

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const creativeAssetCategory = "CREATIVE_ASSETS"

type creativeManifest struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Entries       []creativeManifestEntry `json:"entries"`
}
type creativeManifestEntry struct {
	Locale               string  `json:"locale"`
	ExpectedPlacementID  string  `json:"expectedPlacementId,omitempty"`
	ExpectedAssetID      string  `json:"expectedAssetId,omitempty"`
	File                 string  `json:"file"`
	PlacementType        string  `json:"placementType"`
	PlacementGroup       string  `json:"placementGroup,omitempty"`
	PreviewFrameTimeCode *string `json:"previewFrameTimeCode,omitempty"`
}

func manifestPlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("asset-library manifest plan", flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "Owned App Store version ID")
	path := fs.String("manifest", "", "JSON manifest; asset files are relative to its directory")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "plan", ShortUsage: "asc asset-library manifest plan --app APP_ID --version-id VERSION_ID --manifest ./creative.json",
		ShortHelp: "Plan creative targets and digest-keyed local sources without writes.",
		LongHelp: `Plan a multi-locale creative manifest without uploading or placing assets.

The JSON object has schemaVersion: 1 and entries. Each entry requires locale,
file and placementType (PRODUCT_PAGE_HEADER_ASSET or APP_STORE_SEARCH_RESULTS_ASSET).
placementGroup defaults to DEFAULT_PROFILE; other groups are refused. Video
entries may specify previewFrameTimeCode as HH:MM:SS:FF or HH:MM:SS.mmm. Omitted
video frames are distinct from explicitly selecting the first frame.

Files are rooted at the manifest directory. Image/video preflight uses Apple's
current reference specifications; video inspection requires ffprobe. The owned
version, localization targets and current placement slots are read before source
files. Unknown fields, duplicate target slots and unsafe paths are refused.

sourceKey is SHA256 of a versioned JSON array containing category, media type,
media format, local SHA256, poster mode and exact time code. Paths, filenames,
locales and placement IDs are excluded. Equal keys group local upload sources;
they do not prove remote identity. Apple's library responses omit the original
source checksum, so filename/size matches never authorize reuse. This command
reports remoteReuseVerified=false and does not create, upload, swap or delete.

Example:
  asc asset-library manifest plan --app APP_ID --version-id VERSION_ID --manifest ./creative.json`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appID, versionID, manifestPath := d.ResolveAppID(*app), strings.TrimSpace(*version), strings.TrimSpace(*path)
			for _, required := range []struct{ name, value string }{{"--app", appID}, {"--version-id", versionID}, {"--manifest", manifestPath}} {
				if required.value == "" {
					return d.RequiredFlag(required.name)
				}
			}
			if !shared.IsNumericAppID(appID) || strings.HasPrefix(appID, "0") {
				return d.UsageError("--app requires a canonical numeric app ID")
			}
			absolute, err := filepath.Abs(d.OSPath(manifestPath))
			if err != nil {
				return err
			}
			root, err := rootfs.New(filepath.Dir(absolute))
			if err != nil {
				return err
			}
			defer root.Close()
			data, err := root.ReadFileLimited(filepath.Base(absolute), 1<<20)
			if err != nil {
				return d.UsageErrorf("manifest: %v", err)
			}
			manifest, err := decodeCreativeManifest(data)
			if err != nil {
				return d.UsageErrorf("manifest: %v", err)
			}
			client, err := d.Client()
			if err != nil {
				return err
			}
			result, err := buildCreativeManifestPlan(ctx, d, client, appID, versionID, root, manifest)
			if err != nil {
				return err
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func buildCreativeManifestPlan(ctx context.Context, d *shared.Deps, client *asc.Client, appID, versionID string, root rootfs.Root, manifest creativeManifest) (*asc.CreativeManifestPlan, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	requestCtx, cancel := d.Timeout(ctx)
	owned, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, appID, versionID, "")
	cancel()
	if err != nil {
		return nil, err
	}
	locales, err := creativeManifestLocalizations(ctx, d, client, versionID)
	if err != nil {
		return nil, err
	}
	libraryRaw, err := getLibraryResource(ctx, d, client, "/v1/apps/"+appID+"/assetLibrary")
	if err != nil {
		return nil, err
	}
	var library struct{ Data struct{ ID, Type string } }
	if err := json.Unmarshal(libraryRaw, &library); err != nil {
		return nil, err
	}
	if library.Data.Type != "appAssetLibraries" || !resourceIDPattern.MatchString(library.Data.ID) {
		return nil, fmt.Errorf("invalid owned Asset Library response")
	}
	reference, err := getLibraryResource(ctx, d, client, "/v1/appAssetLibraryRefData")
	if err != nil {
		return nil, err
	}
	result := &asc.CreativeManifestPlan{AppID: appID, VersionID: versionID, VersionState: string(owned.Attributes.AppStoreState), LibraryID: library.Data.ID, ReadOnly: true, Entries: []asc.CreativeManifestPlanEntry{}}
	sources := map[string]bool{}
	for i, entry := range manifest.Entries {
		if _, _, err := resolvePlacementGroup(d, reference, LocalizationUpload{PlacementType: entry.PlacementType, PlacementGroup: entry.PlacementGroup}); err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}
		placements := []asc.CreativeManifestPlacement{}
		localizationID := locales[entry.Locale]
		if localizationID != "" {
			placements, err = creativeManifestPlacements(ctx, d, client, localizationID, entry.PlacementType, entry.PlacementGroup)
			if err != nil {
				return nil, fmt.Errorf("entry %d: %w", i+1, err)
			}
		}
		planned, err := planCreativeSource(ctx, d, root, entry, reference)
		if err != nil {
			return nil, fmt.Errorf("entry %d (%s): %w", i+1, entry.Locale, err)
		}
		planned.Locale, planned.LocalizationID, planned.PlacementType, planned.PlacementGroup = entry.Locale, localizationID, entry.PlacementType, entry.PlacementGroup
		planned.ExistingPlacements = placements
		planned.TargetStatus = "empty"
		switch {
		case localizationID == "":
			planned.TargetStatus = "missingLocalization"
		case len(placements) > 1:
			planned.TargetStatus = "ambiguous"
		case len(placements) == 1:
			planned.TargetStatus = "occupied"
			if placements[0].MediaType != planned.MediaType {
				planned.TargetStatus = "mediaTypeConflict"
			} else if placements[0].AssetID == "" {
				planned.TargetStatus = "unverifiedExistingMedia"
			}
		}
		sources[planned.SourceKey] = true
		result.Entries = append(result.Entries, planned)
	}
	result.UniqueSources = len(sources)
	return result, nil
}

func decodeCreativeManifest(data []byte) (creativeManifest, error) {
	var manifest creativeManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return manifest, fmt.Errorf("trailing manifest data")
	}
	if manifest.SchemaVersion != 1 || len(manifest.Entries) == 0 {
		return manifest, fmt.Errorf("schemaVersion must be 1 and entries must not be empty")
	}
	targets := map[string]bool{}
	for i := range manifest.Entries {
		entry := &manifest.Entries[i]
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(entry.Locale)
		if err != nil {
			return manifest, fmt.Errorf("entry %d locale: %w", i+1, err)
		}
		if (entry.ExpectedPlacementID == "") != (entry.ExpectedAssetID == "") || (entry.ExpectedPlacementID != "" && (!resourceIDPattern.MatchString(entry.ExpectedPlacementID) || !resourceIDPattern.MatchString(entry.ExpectedAssetID))) {
			return manifest, fmt.Errorf("entry %d expectedPlacementId and expectedAssetId require two resource IDs", i+1)
		}
		entry.Locale = locale
		if err := rootfs.ValidateRelative(entry.File); err != nil {
			return manifest, fmt.Errorf("entry %d file: %w", i+1, err)
		}
		entry.PlacementType = strings.ToUpper(strings.TrimSpace(entry.PlacementType))
		if entry.PlacementType != "PRODUCT_PAGE_HEADER_ASSET" && entry.PlacementType != "APP_STORE_SEARCH_RESULTS_ASSET" {
			return manifest, fmt.Errorf("entry %d has unsupported creative placement type", i+1)
		}
		entry.PlacementGroup = strings.TrimSpace(entry.PlacementGroup)
		if entry.PlacementGroup == "" {
			entry.PlacementGroup = "DEFAULT_PROFILE"
		}
		if entry.PlacementGroup != "DEFAULT_PROFILE" {
			return manifest, fmt.Errorf("entry %d creative assets require DEFAULT_PROFILE", i+1)
		}
		mediaType, err := creativeManifestMediaType(entry.File)
		if err != nil {
			return manifest, fmt.Errorf("entry %d: %w", i+1, err)
		}
		if entry.PreviewFrameTimeCode != nil {
			value := strings.TrimSpace(*entry.PreviewFrameTimeCode)
			if mediaType != "VIDEO" || !libraryTimeCode.MatchString(value) {
				return manifest, fmt.Errorf("entry %d previewFrameTimeCode requires video and HH:MM:SS:FF or HH:MM:SS.mmm", i+1)
			}
			entry.PreviewFrameTimeCode = &value
		}
		key := entry.Locale + "/" + entry.PlacementType + "/" + entry.PlacementGroup
		if targets[key] {
			return manifest, fmt.Errorf("duplicate target slot %s", key)
		}
		targets[key] = true
	}
	return manifest, nil
}

func creativeManifestMediaType(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return "IMAGE", nil
	case ".mp4", ".m4v", ".mov":
		return "VIDEO", nil
	default:
		return "", fmt.Errorf("file must be PNG, JPEG, MP4, M4V or MOV")
	}
}

func planCreativeSource(ctx context.Context, d *shared.Deps, root rootfs.Root, entry creativeManifestEntry, reference []byte) (asc.CreativeManifestPlanEntry, error) {
	var result asc.CreativeManifestPlanEntry
	file, err := root.OpenFile(entry.File)
	if err != nil {
		return result, err
	}
	defer file.Close()
	result.File = entry.File
	result.MediaType, _ = creativeManifestMediaType(entry.File)
	result.PosterFrameMode = "none"
	specs, err := groupSpecIDs(reference, entry.PlacementType, entry.PlacementGroup)
	if err != nil {
		return result, err
	}
	if result.MediaType == "IMAGE" {
		result.FileSize, err = validateLibraryImage(d, "manifest plan", "file", entry.File, file)
		if err != nil {
			return result, err
		}
		if err := matchLibraryImageSpec(file, entry.PlacementGroup, reference, specs); err != nil {
			return result, err
		}
		result.MediaFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.File)), ".")
		if result.MediaFormat == "jpg" {
			result.MediaFormat = "jpeg"
		}
	} else {
		staged, err := stageLibraryVideoFile(ctx, d, "manifest plan", entry.File, file)
		if err != nil {
			return result, err
		}
		defer staged.close()
		if err := matchLibraryVideoSpecIn(reference, creativeAssetCategory, specs, staged.properties, staged.size, filepath.Ext(entry.File)); err != nil {
			return result, err
		}
		file = staged.file
		result.FileSize = staged.size
		result.MediaFormat = staged.properties.MIMEType
		result.PosterFrameMode = "default"
		if entry.PreviewFrameTimeCode != nil {
			result.PosterFrameMode = "explicit"
			result.PreviewFrameTimeCode = *entry.PreviewFrameTimeCode
		}
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, result.FileSize+1))
	if err != nil {
		return result, err
	}
	if n != result.FileSize {
		return result, fmt.Errorf("source changed while hashing")
	}
	result.ContentSHA256 = hex.EncodeToString(hash.Sum(nil))
	identity, _ := json.Marshal([]string{"asc-creative-source-v1", creativeAssetCategory, result.MediaType, result.MediaFormat, result.ContentSHA256, result.PosterFrameMode, result.PreviewFrameTimeCode})
	key := sha256.Sum256(identity)
	result.SourceKey = "sha256:" + hex.EncodeToString(key[:])
	return result, nil
}

func creativeManifestCollection(ctx context.Context, d *shared.Deps, client *asc.Client, path string, filters url.Values) ([]json.RawMessage, error) {
	target := path
	if len(filters) > 0 {
		target += "?" + filters.Encode()
	}
	seen := map[string]bool{}
	result := []json.RawMessage{}
	for {
		raw, err := getLibraryResource(ctx, d, client, target)
		if err != nil {
			return nil, err
		}
		var page struct {
			Data  []json.RawMessage
			Links struct{ Next string }
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		if page.Data == nil {
			return nil, fmt.Errorf("manifest collection %s requires a data array", path)
		}
		result = append(result, page.Data...)
		if page.Links.Next == "" {
			return result, nil
		}
		next, err := client.ValidateNoRedirectReadContinuationQuery(ctx, page.Links.Next, path, filters)
		if err != nil {
			return nil, fmt.Errorf("manifest pagination: %w", err)
		}
		key := asc.PaginationURLIdentity(next)
		if seen[key] {
			return nil, fmt.Errorf("manifest pagination repeats a page")
		}
		seen[key] = true
		target = next
	}
}

func creativeManifestLocalizations(ctx context.Context, d *shared.Deps, client *asc.Client, versionID string) (map[string]string, error) {
	items, err := creativeManifestCollection(ctx, d, client, "/v1/appStoreVersions/"+url.PathEscape(versionID)+"/appStoreVersionLocalizations", nil)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, raw := range items {
		var item asc.Resource[asc.AppStoreVersionLocalizationAttributes]
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(item.Attributes.Locale)
		if err != nil {
			return nil, err
		}
		if item.Type != asc.ResourceTypeAppStoreVersionLocalizations || !resourceIDPattern.MatchString(item.ID) || result[locale] != "" {
			return nil, fmt.Errorf("ambiguous localization %s", locale)
		}
		result[locale] = item.ID
	}
	return result, nil
}

func creativeManifestPlacements(ctx context.Context, d *shared.Deps, client *asc.Client, localizationID, placementType, group string) ([]asc.CreativeManifestPlacement, error) {
	filters := url.Values{"filter[placementType]": {placementType}, "filter[placementGroup]": {group}, "include": {"image,video"}}
	items, err := creativeManifestCollection(ctx, d, client, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localizationID)+"/placements", filters)
	if err != nil {
		return nil, err
	}
	result := []asc.CreativeManifestPlacement{}
	seen := map[string]bool{}
	for _, raw := range items {
		var item struct {
			ID, Type      string
			Attributes    struct{ MediaType, PlacementType, PlacementGroup, State string }
			Relationships map[string]struct{ Data struct{ ID, Type string } }
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		if item.Type != "appAssetLibraryPlacements" || !resourceIDPattern.MatchString(item.ID) || item.Attributes.PlacementType != placementType || item.Attributes.PlacementGroup != group {
			return nil, fmt.Errorf("placement does not match the selected target")
		}
		owner := item.Relationships["appStoreVersionLocalization"].Data
		if owner.ID != "" && (owner.ID != localizationID || owner.Type != "appStoreVersionLocalizations") {
			return nil, fmt.Errorf("placement belongs to a different localization")
		}
		relationship := "image"
		expectedType := "appAssetLibraryImages"
		if item.Attributes.MediaType == "VIDEO" {
			relationship = "video"
			expectedType = "appAssetLibraryVideos"
		} else if item.Attributes.MediaType != "IMAGE" {
			return nil, fmt.Errorf("placement has unknown media type")
		}
		media := item.Relationships[relationship].Data
		if media.ID != "" && (media.Type != expectedType || !resourceIDPattern.MatchString(media.ID)) {
			return nil, fmt.Errorf("invalid placement media")
		}
		if seen[item.ID] {
			return nil, fmt.Errorf("duplicate placement %q in target slot", item.ID)
		}
		seen[item.ID] = true
		result = append(result, asc.CreativeManifestPlacement{ID: item.ID, MediaType: item.Attributes.MediaType, AssetID: media.ID, State: item.Attributes.State})
	}
	return result, nil
}
