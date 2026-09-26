package storeassets

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Changes describes the full target and content fingerprint used by metadata
// review/approval. No transfer is allowed to bypass these entries.
func (p *ImportPlan) Changes() []asc.StoreAssetChange {
	var changes []asc.StoreAssetChange
	add := func(kind, locale, path, action, from, to string) {
		changes = append(changes, asc.StoreAssetChange{Kind: kind, Locale: locale, Path: path, Action: action, From: from, To: to})
	}
	if p.Clip != nil {
		if p.Experience.ID == "" {
			add("app_clip", "", "action.txt", "create", "", p.ClipID+":"+p.VersionID+":"+p.Clip.Action)
		} else if p.Clip.Action != "" && p.Clip.Action != string(p.Experience.Attributes.Action) {
			add("app_clip", "", "action.txt", "update", p.Experience.ID+":"+string(p.Experience.Attributes.Action), p.Clip.Action)
		}
		locales := map[string]bool{}
		for l := range p.Clip.Subtitles {
			locales[l] = true
		}
		for l := range p.Clip.HeaderImages {
			locales[l] = true
		}
		for locale := range locales {
			loc := p.Localizations[locale]
			subtitle, exists := p.Clip.Subtitles[locale]
			if loc.ID == "" {
				add("app_clip_localization", locale, "subtitle.txt", "create", "", subtitle)
			} else if exists && subtitle != loc.Attributes.Subtitle {
				add("app_clip_localization", locale, "subtitle.txt", "update", loc.ID+":"+loc.Attributes.Subtitle, subtitle)
			}
			if p.Clip.HeaderImages[locale] != "" {
				current := p.Headers[locale]
				checksum := p.Clip.headerChecksums[locale]
				if current.ID == "" || !strings.EqualFold(current.Attributes.SourceFileChecksum, checksum) {
					action := "upload"
					if current.ID != "" {
						action = "replace"
					}
					add("app_clip_header", locale, "header_image.png", action, current.ID+":"+current.Attributes.SourceFileChecksum, checksum)
				}
			}
		}
	}
	for _, preview := range p.Previews {
		group := p.previewGroups[preview.Locale+"/"+strings.ToUpper(preview.DeviceType)]
		var match *asc.Resource[asc.AppPreviewAttributes]
		for i := range group.Existing {
			if group.Existing[i].ID == preview.existingID {
				match = &group.Existing[i]
				break
			}
		}
		path := preview.DeviceType + "/" + preview.FileName
		if match == nil {
			add("preview", preview.Locale, path, "upload", "", preview.checksum+":"+preview.PosterFrame)
		} else if preview.PosterFrame != "" && match.Attributes.PreviewFrameTimeCode != preview.PosterFrame {
			add("preview", preview.Locale, path, "update", match.ID+":"+match.Attributes.PreviewFrameTimeCode, preview.checksum+":"+preview.PosterFrame)
		}
	}
	// The desired content order is part of the review hash, including remote-only
	// previews that will be retained after the local files.
	for _, group := range p.previewGroups {
		current := append([]string(nil), group.CurrentOrder...)
		desired := []string{}
		for _, preview := range p.Previews {
			if preview.Locale != group.Locale || !strings.EqualFold(preview.DeviceType, group.Device) {
				continue
			}
			id := "new:" + preview.checksum
			if preview.existingID != "" {
				id = preview.existingID
			}
			if !contains(desired, id) {
				desired = append(desired, id)
			}
		}
		for _, id := range current {
			if !contains(desired, id) {
				desired = append(desired, id)
			}
		}
		before, _ := json.Marshal(current)
		after, _ := json.Marshal(desired)
		if string(before) != string(after) {
			add("preview_order", group.Locale, strings.ToLower(group.Device), "update", group.SetID+":"+string(before), string(after))
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Kind+"/"+changes[i].Locale+"/"+changes[i].Path < changes[j].Kind+"/"+changes[j].Locale+"/"+changes[j].Path
	})
	return changes
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
