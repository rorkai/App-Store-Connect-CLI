package storeassets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type AppClipLayout struct {
	sourceRoot      rootfs.Root
	stagedHeaders   map[string]string
	headerChecksums map[string]string
	Action          string            `json:"action,omitempty"`
	Subtitles       map[string]string `json:"subtitles,omitempty"`
	HeaderImages    map[string]string `json:"headerImages,omitempty"`
}
type PreviewLayout struct {
	sourceRoot     rootfs.Root
	stagedPath     string
	checksum       string
	existingID     string
	sourceChecksum string
	Locale         string `json:"locale"`
	DeviceType     string `json:"deviceType"`
	FileName       string `json:"fileName"`
	Path           string `json:"path"`
	PosterFrame    string `json:"posterFrame,omitempty"`
}

// ImportPlan is a preflight snapshot. It selects only the requested version's
// experience and resolves missing-target ambiguity before any mutation.
type ImportPlan struct {
	previewGroups map[string]*previewGroup
	Clip          *AppClipLayout
	Previews      []PreviewLayout
	Experience    asc.Resource[asc.AppClipDefaultExperienceAttributes]
	ClipID        string
	VersionID     string
	Localizations map[string]asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]
	Headers       map[string]asc.Resource[asc.AppClipHeaderImageAttributes]
}

func PrepareImport(ctx context.Context, client *asc.Client, appID, versionID string, clip *AppClipLayout, previews []PreviewLayout) (*ImportPlan, error) {
	plan := &ImportPlan{Clip: clip, Previews: previews, VersionID: versionID, Localizations: map[string]asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]{}, Headers: map[string]asc.Resource[asc.AppClipHeaderImageAttributes]{}}
	if clip == nil {
		return preparePreviews(ctx, client, plan)
	}
	exp, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceResponse, error) {
		return client.GetAppStoreVersionAppClipDefaultExperience(c, versionID)
	})
	if err != nil && !asc.IsNotFound(err) {
		return nil, err
	}
	if exp != nil {
		plan.Experience = exp.Data
	}
	if plan.Experience.ID == "" {
		clips, err := pages(ctx, func(c context.Context, next string) (*asc.AppClipsResponse, error) {
			return client.GetAppClips(c, appID, asc.WithAppClipsNextURL(next))
		})
		if err != nil {
			return nil, err
		}
		if len(clips) != 1 {
			return nil, fmt.Errorf("app_clip folder requires exactly one App Clip when the version has no default experience (found %d)", len(clips))
		}
		plan.ClipID = clips[0].ID
		return preparePreviews(ctx, client, plan)
	}
	locs, err := clipLocalizations(ctx, client, plan.Experience.ID)
	if err != nil {
		return nil, err
	}
	for _, loc := range locs {
		plan.Localizations[loc.Attributes.Locale] = loc
	}
	for locale := range clip.HeaderImages {
		loc := plan.Localizations[locale]
		if loc.ID == "" {
			continue
		}
		header, err := request(ctx, func(c context.Context) (*asc.AppClipHeaderImageResponse, error) {
			return client.GetAppClipDefaultExperienceLocalizationHeaderImage(c, loc.ID)
		})
		if err != nil && !asc.IsNotFound(err) {
			return nil, err
		}
		if header != nil {
			plan.Headers[locale] = header.Data
		}
	}
	return preparePreviews(ctx, client, plan)
}

func (p *ImportPlan) Apply(ctx context.Context, client *asc.Client, localeToID map[string]string) ([]asc.StoreAssetResult, error) {
	if err := p.checkPreviewMatches(ctx, client); err != nil {
		return nil, err
	}
	var results []asc.StoreAssetResult
	record := func(kind, locale, path, id, action string, err error) {
		status := "applied"
		if action == "skip" {
			status = "skipped"
		}
		message := ""
		if err != nil {
			status = "failed"
			message = shared.SanitizeTerminal(err.Error())
		}
		results = append(results, asc.StoreAssetResult{Kind: kind, Locale: locale, Path: path, ID: id, Action: action, Status: status, Error: message})
	}
	if p.Clip != nil {
		expID := p.Experience.ID
		if expID == "" {
			var action *asc.AppClipAction
			if p.Clip.Action != "" {
				v := asc.AppClipAction(p.Clip.Action)
				action = &v
			}
			created, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceResponse, error) {
				return client.CreateAppClipDefaultExperience(c, p.ClipID, &asc.AppClipDefaultExperienceCreateAttributes{Action: action}, p.VersionID, "")
			})
			if created != nil {
				expID = created.Data.ID
			}
			record("app_clip", "", "", expID, "create", err)
			if err != nil {
				return results, err
			}
		} else if p.Clip.Action != "" && string(p.Experience.Attributes.Action) != p.Clip.Action {
			action := asc.AppClipAction(p.Clip.Action)
			_, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceResponse, error) {
				return client.UpdateAppClipDefaultExperience(c, expID, &asc.AppClipDefaultExperienceUpdateAttributes{Action: &action}, "")
			})
			record("app_clip", "", "", expID, "update", err)
			if err != nil {
				return results, err
			}
		}
		locales := map[string]bool{}
		for l := range p.Clip.Subtitles {
			locales[l] = true
		}
		for l := range p.Clip.HeaderImages {
			locales[l] = true
		}
		ordered := make([]string, 0, len(locales))
		for l := range locales {
			ordered = append(ordered, l)
		}
		sort.Strings(ordered)
		for _, locale := range ordered {
			existing := p.Localizations[locale]
			locID := existing.ID
			subtitle, hasSubtitle := p.Clip.Subtitles[locale]
			if locID == "" {
				var value *string
				if hasSubtitle {
					value = &subtitle
				}
				created, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceLocalizationResponse, error) {
					return client.CreateAppClipDefaultExperienceLocalization(c, expID, asc.AppClipDefaultExperienceLocalizationCreateAttributes{Locale: locale, Subtitle: value})
				})
				if created != nil {
					locID = created.Data.ID
				}
				record("app_clip_localization", locale, "", locID, "create", err)
				if err != nil {
					return results, err
				}
			} else if hasSubtitle && existing.Attributes.Subtitle != subtitle {
				_, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceLocalizationResponse, error) {
					return client.UpdateAppClipDefaultExperienceLocalization(c, locID, &asc.AppClipDefaultExperienceLocalizationUpdateAttributes{Subtitle: &subtitle})
				})
				record("app_clip_localization", locale, "", locID, "update", err)
				if err != nil {
					return results, err
				}
			}
			if path := p.Clip.HeaderImages[locale]; path != "" {
				current := p.Headers[locale]
				id, action, deleted, err := uploadHeader(ctx, client, locID, p.Clip.stagedHeaders[locale], current)
				record("app_clip_header", locale, path, id, action, err)
				if deleted {
					results[len(results)-1].PreviousID = current.ID
					results[len(results)-1].PreviousDeleted = true
				}
				if err != nil {
					return results, err
				}
			}
		}
	}
	for _, preview := range p.Previews {
		locID := localeToID[preview.Locale]
		if locID == "" {
			created, err := request(ctx, func(c context.Context) (*asc.AppStoreVersionLocalizationResponse, error) {
				return client.CreateAppStoreVersionLocalization(c, p.VersionID, asc.AppStoreVersionLocalizationAttributes{Locale: preview.Locale})
			})
			if created != nil {
				locID = created.Data.ID
				localeToID[preview.Locale] = locID
			}
			record("version_localization", preview.Locale, "", locID, "create", err)
			if err != nil {
				return results, err
			}
		}
		group := p.previewGroups[preview.Locale+"/"+strings.ToUpper(preview.DeviceType)]
		setID := group.SetID
		if setID == "" {
			created, err := request(ctx, func(c context.Context) (*asc.AppPreviewSetResponse, error) {
				return client.CreateAppPreviewSet(c, locID, strings.ToUpper(preview.DeviceType))
			})
			if created != nil {
				setID = created.Data.ID
				group.SetID = setID
			}
			record("preview_set", preview.Locale, "", setID, "create", err)
			if err != nil {
				return results, err
			}
		}
		id, action, err := uploadPreview(ctx, client, setID, preview)
		record("preview", preview.Locale, preview.Path, id, action, err)
		if err != nil {
			return results, err
		}
		group.DesiredIDs = append(group.DesiredIDs, id)
	}
	keys := make([]string, 0, len(p.previewGroups))
	for key := range p.previewGroups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := p.previewGroups[key]
		current, err := previewOrder(ctx, client, group.SetID)
		if err != nil {
			return results, err
		}
		desired := []string{}
		for _, id := range group.DesiredIDs {
			if !slices.Contains(desired, id) {
				desired = append(desired, id)
			}
		}
		for _, id := range current {
			if !slices.Contains(desired, id) {
				desired = append(desired, id)
			}
		}
		if !slices.Equal(current, desired) {
			_, err := request(ctx, func(ctx context.Context) (bool, error) {
				return true, client.UpdateAppPreviewSetAppPreviewsRelationship(ctx, group.SetID, desired)
			})
			record("preview_order", group.Locale, "", group.SetID, "update", err)
			if err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

func uploadHeader(ctx context.Context, c *asc.Client, locID, path string, current asc.Resource[asc.AppClipHeaderImageAttributes]) (string, string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "upload", false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", "upload", false, err
	}
	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return "", "upload", false, err
	}
	if current.ID != "" && strings.EqualFold(current.Attributes.SourceFileChecksum, checksum.Hash) {
		return current.ID, "skip", false, nil
	}
	if current.ID != "" {
		_, err := request(ctx, func(ctx context.Context) (bool, error) { return true, c.DeleteAppClipHeaderImage(ctx, current.ID) })
		if err != nil {
			return current.ID, "replace", false, err
		}
	}
	created, err := request(ctx, func(ctx context.Context) (*asc.AppClipHeaderImageResponse, error) {
		return c.CreateAppClipHeaderImage(ctx, locID, filepath.Base(path), info.Size())
	})
	if err != nil {
		return "", "upload", current.ID != "", err
	}
	id := created.Data.ID
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return id, "upload", current.ID != "", fmt.Errorf("header reservation returned no upload operations")
	}
	uploadCtx, cancel := shared.ContextWithUploadTimeout(shared.ContextWithoutTimeout(ctx))
	defer cancel()
	if err := asc.UploadAssetFromFile(uploadCtx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return id, "upload", current.ID != "", err
	}
	_, err = request(ctx, func(ctx context.Context) (*asc.AppClipHeaderImageResponse, error) {
		return c.UpdateAppClipHeaderImage(ctx, id, true)
	})
	return id, "upload", current.ID != "", err
}

func uploadPreview(ctx context.Context, c *asc.Client, setID string, preview PreviewLayout) (string, string, error) {
	existing, err := previewItems(ctx, c, setID)
	if err != nil {
		return "", "upload", err
	}
	id := ""
	action := "skip"
	existingFrame := ""
	for _, p := range existing {
		if preview.existingID != "" && p.ID == preview.existingID {
			if !strings.EqualFold(p.Attributes.SourceFileChecksum, preview.sourceChecksum) {
				return "", "skip", fmt.Errorf("preview %s changed since preflight; retry", p.ID)
			}
			id = p.ID
			existingFrame = p.Attributes.PreviewFrameTimeCode
			break
		}
		if preview.existingID == "" && strings.EqualFold(p.Attributes.SourceFileChecksum, preview.checksum) {
			id = p.ID
			existingFrame = p.Attributes.PreviewFrameTimeCode
			break
		}
	}
	if preview.existingID != "" && id == "" {
		return "", "skip", fmt.Errorf("preview %s no longer belongs to set %s; retry", preview.existingID, setID)
	}
	if id == "" {
		if len(existing) >= 3 {
			return "", "upload", fmt.Errorf("preview set %s already contains three videos; no assets were deleted", setID)
		}
		uploadCtx, cancel := shared.ContextWithUploadTimeout(shared.ContextWithoutTimeout(ctx))
		item, err := assets.UploadPreviewAsset(uploadCtx, c, setID, preview.stagedPath)
		cancel()
		id = item.AssetID
		action = "upload"
		if err != nil {
			return id, action, err
		}
	}
	if preview.PosterFrame != "" && preview.PosterFrame != existingFrame {
		if action == "skip" {
			action = "update"
		}
		_, err := request(ctx, func(ctx context.Context) (*asc.AppPreviewResponse, error) {
			return c.SetAppPreviewFrameTimeCode(ctx, id, preview.PosterFrame)
		})
		if err != nil {
			return id, action, err
		}
	}
	return id, action, nil
}

type previewGroup struct {
	LocalizationID string
	CurrentOrder   []string
	Locale         string
	Device         string
	SetID          string
	DesiredIDs     []string
	Existing       []asc.Resource[asc.AppPreviewAttributes]
}

func preparePreviews(ctx context.Context, c *asc.Client, p *ImportPlan) (*ImportPlan, error) {
	p.previewGroups = map[string]*previewGroup{}
	if len(p.Previews) == 0 {
		return p, nil
	}
	locs, err := versionLocalizations(ctx, c, p.VersionID)
	if err != nil {
		return nil, err
	}
	locIDs := map[string]string{}
	for _, loc := range locs {
		locIDs[loc.Attributes.Locale] = loc.ID
	}
	checksums := map[string]map[string]bool{}
	for i := range p.Previews {
		preview := &p.Previews[i]
		key := preview.Locale + "/" + strings.ToUpper(preview.DeviceType)
		group := p.previewGroups[key]
		if group == nil {
			group = &previewGroup{Locale: preview.Locale, Device: strings.ToUpper(preview.DeviceType), LocalizationID: locIDs[preview.Locale]}
			p.previewGroups[key] = group
			if locID := locIDs[preview.Locale]; locID != "" {
				sets, err := previewSets(ctx, c, locID)
				if err != nil {
					return nil, err
				}
				for _, set := range sets {
					if strings.EqualFold(set.Attributes.PreviewType, preview.DeviceType) {
						group.SetID = set.ID
						break
					}
				}
				if group.SetID != "" {
					group.CurrentOrder, err = previewOrder(ctx, c, group.SetID)
					if err != nil {
						return nil, err
					}
					group.Existing, err = previewItems(ctx, c, group.SetID)
					if err != nil {
						return nil, err
					}
				}
			}
			checksums[key] = map[string]bool{}
		}
		if err := resolvePreviewMatch(ctx, c, preview, group.Existing); err != nil {
			return nil, err
		}
		if preview.existingID == "" {
			checksums[key][strings.ToLower(preview.checksum)] = true
		}
	}
	for key, group := range p.previewGroups {
		// Count each remote reservation, even when its checksum is unavailable.
		added := len(checksums[key])
		if len(group.Existing)+added > 3 {
			return nil, fmt.Errorf("preview set %s would exceed three videos; remove unwanted previews explicitly before importing", key)
		}
	}
	return p, nil
}

func previewOrder(ctx context.Context, c *asc.Client, setID string) ([]string, error) {
	first, err := request(ctx, func(ctx context.Context) (*asc.LinkagesResponse, error) {
		return c.GetAppPreviewSetAppPreviewsRelationships(ctx, setID, asc.WithLinkagesLimit(200))
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	err = asc.PaginateEach(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		return request(ctx, func(ctx context.Context) (*asc.LinkagesResponse, error) {
			return c.GetAppPreviewSetAppPreviewsRelationships(ctx, setID, asc.WithLinkagesNextURL(next))
		})
	}, func(page asc.PaginatedResponse) error {
		for _, item := range page.(*asc.LinkagesResponse).Data {
			ids = append(ids, item.ID)
		}
		return nil
	})
	return ids, err
}
