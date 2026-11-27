package artifacts

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"howett.net/plist"
)

const maxPrivacyMetadataBytes = 16 << 20

type PrivacyManifestInventory struct {
	Complete  bool
	Manifests []PrivacyManifest
}

type PrivacyManifest struct {
	Path         string
	BundlePath   string
	BundleKind   string
	BundleID     string
	Declarations map[string]any
	Error        string
}

func inspectPrivacyManifests(files []*zip.File, appRoot string) (*PrivacyManifestInventory, error) {
	inventory := &PrivacyManifestInventory{Complete: true, Manifests: []PrivacyManifest{}}
	members := map[string][]*zip.File{}
	var selected []*zip.File
	for _, file := range files {
		name := zipMemberName(file.Name)
		if !strings.HasPrefix(name, appRoot) {
			continue
		}
		members[name] = append(members[name], file)
		if path.Base(name) == "PrivacyInfo.xcprivacy" && !file.FileInfo().IsDir() {
			selected = append(selected, file)
		}
	}
	slices.SortFunc(selected, func(a, b *zip.File) int { return strings.Compare(zipMemberName(a.Name), zipMemberName(b.Name)) })
	remaining := uint64(maxPrivacyMetadataBytes)
	type ownerInfo struct {
		id  string
		err error
	}
	owners := map[string]ownerInfo{}
	var failures []error
	for _, file := range selected {
		name := zipMemberName(file.Name)
		entry := PrivacyManifest{Path: name}
		var entryErr error
		if path.Clean(name) != name {
			entryErr = fmt.Errorf("non-canonical archive path")
		} else if len(members[name]) != 1 {
			entryErr = fmt.Errorf("duplicate archive member")
		} else {
			for dir := path.Dir(name); strings.HasPrefix(dir+"/", appRoot); dir = path.Dir(dir) {
				switch path.Ext(dir) {
				case ".app":
					entry.BundleKind = "app"
				case ".appex":
					entry.BundleKind = "extension"
				case ".framework":
					entry.BundleKind = "framework"
				case ".bundle":
					entry.BundleKind = "bundle"
				}
				if entry.BundleKind != "" {
					entry.BundlePath = dir
					break
				}
			}
			data, err := readPrivacyMember(file, &remaining)
			if err == nil {
				_, err = plist.Unmarshal(data, &entry.Declarations)
			}
			if err == nil {
				_, err = json.Marshal(entry.Declarations)
			}
			if err != nil {
				entry.Declarations = nil
				entryErr = err
			}
			if entry.BundlePath != "" {
				ownerKey := entry.BundlePath
				relative := strings.TrimPrefix(name, entry.BundlePath+"/")
				if parts := strings.SplitN(relative, "/", 3); len(parts) == 3 && parts[0] == "Versions" {
					ownerKey += "/Versions/" + parts[1]
				}
				owner, ok := owners[ownerKey]
				if !ok {
					candidates := []string{entry.BundlePath + "/Info.plist", entry.BundlePath + "/Contents/Info.plist", entry.BundlePath + "/Resources/Info.plist"}
					relative := strings.TrimPrefix(name, entry.BundlePath+"/")
					if parts := strings.SplitN(relative, "/", 3); len(parts) == 3 && parts[0] == "Versions" {
						candidates = append([]string{entry.BundlePath + "/Versions/" + parts[1] + "/Resources/Info.plist"}, candidates...)
					}
					for _, candidate := range candidates {
						matches := members[candidate]
						if len(matches) == 0 {
							continue
						}
						if len(matches) != 1 {
							owner.err = fmt.Errorf("duplicate owner Info.plist")
							break
						}
						data, err := readPrivacyMember(matches[0], &remaining)
						var info bundlePlist
						if err == nil {
							info, err = decodeBundlePlist(data)
						}
						owner.id, owner.err = info.BundleID, err
						break
					}
					owners[ownerKey] = owner
				}
				entry.BundleID = owner.id
				entryErr = errors.Join(entryErr, owner.err)
			}
		}
		if entryErr != nil {
			inventory.Complete = false
			entry.Error = entryErr.Error()
			failures = append(failures, fmt.Errorf("privacy manifest %s: %w", name, entryErr))
		}
		inventory.Manifests = append(inventory.Manifests, entry)
	}
	return inventory, errors.Join(failures...)
}

func readPrivacyMember(file *zip.File, remaining *uint64) ([]byte, error) {
	if !file.Mode().IsRegular() {
		return nil, fmt.Errorf("archive member is not a regular file")
	}
	if err := infoplist.CheckDeclaredSize(file.UncompressedSize64); err != nil {
		return nil, err
	}
	if file.UncompressedSize64 > *remaining {
		return nil, fmt.Errorf("privacy metadata exceeds %d byte aggregate limit", maxPrivacyMetadataBytes)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	limit := min(uint64(infoplist.MaxBytes), *remaining)
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	*remaining -= min(uint64(len(data)), *remaining)
	if uint64(len(data)) > limit {
		return nil, fmt.Errorf("privacy metadata exceeds its read limit")
	}
	if err != nil {
		return nil, err
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return nil, err
	}
	return data, nil
}
