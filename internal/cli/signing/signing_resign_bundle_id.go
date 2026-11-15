package signing

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	signingResignWatchCompanionKey = "WKCompanionAppBundleIdentifier"
	signingResignWatchAppKey       = "WKAppBundleIdentifier"
)

// sourceBundleID is the identifier the input signature was made for. It
// differs from BundleID only when --bundle-id maps the target to a new one.
func (target signingResignTarget) sourceBundleID() string {
	if target.SourceBundleID != "" {
		return target.SourceBundleID
	}
	return target.BundleID
}

// mapSigningResignBundleIDs plans a --bundle-id change without touching the
// tree: the main app gets newID, every nested target replaces the OLD. prefix
// with newID., and each target carries its rewritten Info.plist for
// prepareSigningResignTree to write once the whole plan is valid.
func mapSigningResignBundleIDs(ctx context.Context, tree rootfs.Root, archive signingResignArchive, newID string) (signingResignArchive, error) {
	if err := validateSigningResignBundleID(newID); err != nil {
		return archive, fmt.Errorf("--bundle-id %q is not a valid bundle identifier: %w", newID, err)
	}
	mainIndex := slices.IndexFunc(archive.Targets, func(target signingResignTarget) bool {
		return target.RelativePath == archive.MainPath
	})
	if mainIndex < 0 {
		return archive, fmt.Errorf("IPA main application target is missing")
	}
	oldID := archive.Targets[mainIndex].BundleID
	if strings.EqualFold(newID, oldID) {
		return archive, fmt.Errorf("--bundle-id %s equals the current bundle identifier %s", newID, oldID)
	}
	sources := make(map[string]string, len(archive.Targets))
	for _, target := range archive.Targets {
		sources[strings.ToLower(target.BundleID)] = target.RelativePath
	}
	mapped := signingResignArchive{MainPath: archive.MainPath, Targets: slices.Clone(archive.Targets)}
	byPath := make(map[string]signingResignTarget, len(mapped.Targets))
	for index := range mapped.Targets {
		target := &mapped.Targets[index]
		if target.Kind == "app-clip" {
			return archive, fmt.Errorf("--bundle-id does not support App Clip targets; %s is an App Clip", target.BundleID)
		}
		targetID := newID
		if index != mainIndex {
			suffix, ok := strings.CutPrefix(target.BundleID, oldID+".")
			if !ok {
				return archive, fmt.Errorf("--bundle-id requires every nested bundle identifier to start with %s.; %s does not", oldID, target.BundleID)
			}
			targetID = newID + "." + suffix
		}
		if err := validateSigningResignBundleID(targetID); err != nil {
			return archive, fmt.Errorf("--bundle-id maps %s to an invalid bundle identifier %s: %w", target.BundleID, targetID, err)
		}
		if owner, exists := sources[strings.ToLower(targetID)]; exists && owner != target.RelativePath {
			return archive, fmt.Errorf("--bundle-id maps %s to %s, which collides with the target at %s", target.BundleID, targetID, owner)
		}
		target.SourceBundleID, target.BundleID = target.BundleID, targetID
		byPath[target.RelativePath] = *target
	}
	for index := range mapped.Targets {
		if err := ctx.Err(); err != nil {
			return archive, err
		}
		target := &mapped.Targets[index]
		data, err := planSigningResignInfoPlist(tree, *target, mapped.Targets[mainIndex], byPath)
		if err != nil {
			return archive, fmt.Errorf("target %s: %w", target.SourceBundleID, err)
		}
		target.InfoPlistData = data
	}
	return mapped, nil
}

// planSigningResignInfoPlist returns the target's Info.plist with the new
// identifier and watch keys, encoded in its original format. Other strings
// that merely contain the old identifier are deliberately left unchanged.
func planSigningResignInfoPlist(tree rootfs.Root, target, main signingResignTarget, byPath map[string]signingResignTarget) ([]byte, error) {
	data, err := tree.ReadFileLimited(filepath.FromSlash(path.Join(target.RelativePath, "Info.plist")), infoplist.MaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read Info.plist: %w", err)
	}
	var info map[string]any
	format, err := plist.Unmarshal(data, &info)
	if err != nil {
		return nil, fmt.Errorf("decode Info.plist")
	}
	if plistString(info["CFBundleIdentifier"]) != target.SourceBundleID {
		return nil, fmt.Errorf("Info.plist CFBundleIdentifier changed during planning")
	}
	info["CFBundleIdentifier"] = target.BundleID
	if err := rewriteSigningResignWatchKey(info, signingResignWatchCompanionKey, main, true); err != nil {
		return nil, err
	}
	if extension, ok := info["NSExtension"].(map[string]any); ok {
		if attributes, ok := extension["NSExtensionAttributes"].(map[string]any); ok {
			watchApp, found := byPath[path.Dir(path.Dir(target.RelativePath))]
			found = found && watchApp.Kind == "watch-application"
			if err := rewriteSigningResignWatchKey(attributes, signingResignWatchAppKey, watchApp, found); err != nil {
				return nil, err
			}
		}
	}
	encoded, err := plist.MarshalIndent(info, format, "\t")
	if err != nil {
		return nil, fmt.Errorf("encode Info.plist: %w", err)
	}
	return encoded, nil
}

// rewriteSigningResignWatchKey points a watch key at owner's new identifier
// only when it currently names owner's old identifier exactly.
func rewriteSigningResignWatchKey(values map[string]any, key string, owner signingResignTarget, found bool) error {
	value, exists := values[key]
	if !exists {
		return nil
	}
	text, _ := value.(string)
	if !found || text != owner.SourceBundleID {
		return fmt.Errorf("--bundle-id cannot rewrite Info.plist %s %q: it does not name the bundle identifier of the app it belongs to", key, text)
	}
	values[key] = owner.BundleID
	return nil
}
