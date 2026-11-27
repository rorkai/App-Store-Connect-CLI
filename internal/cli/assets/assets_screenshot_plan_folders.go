package assets

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ScreenshotPlanGroup contains ordered relative paths for one locale and device.
type ScreenshotPlanGroup struct {
	Locale, DisplayType string
	Files               []string
}

// NormalizeScreenshotPlanDeviceTypes validates and canonicalizes device tokens.
func NormalizeScreenshotPlanDeviceTypes(deviceTypes []string) ([]string, error) {
	types := []string{}
	seenTypes := map[string]bool{}
	for _, token := range deviceTypes {
		display, err := normalizeScreenshotDisplayType(token)
		if err != nil {
			return nil, err
		}
		display = asc.CanonicalScreenshotDisplayTypeForAPI(display)
		if seenTypes[display] {
			return nil, fmt.Errorf("duplicate device type %q", token)
		}
		seenTypes[display] = true
		types = append(types, display)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("--device-type is required")
	}
	return types, nil
}

// DiscoverScreenshotPlanGroups reads locale-first or device-first screenshot trees.
func DiscoverScreenshotPlanGroups(path string, deviceTypes []string) ([]ScreenshotPlanGroup, []string, error) {
	types, err := NormalizeScreenshotPlanDeviceTypes(deviceTypes)
	if err != nil {
		return nil, nil, err
	}
	root, err := rootfs.New(path)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	groups := map[string]*ScreenshotPlanGroup{}
	localeNames := map[string]string{}
	digests := map[string]map[screenshotPixelDigest][]string{}
	skipped := []string{}
	var walk func(string) error
	walk = func(dir string) error {
		f, err := root.OpenDir(dir)
		if err != nil {
			return err
		}
		entries, err := f.ReadDir(-1)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if shouldIgnoreFanoutEntryName(entry.Name()) {
				continue
			}
			relative := filepath.Join(dir, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to read symlink %q", relative)
			}
			if entry.IsDir() {
				if err := walk(relative); err != nil {
					return err
				}
				continue
			}
			if !isSupportedScreenshotUploadFile(relative) {
				skipped = append(skipped, filepath.ToSlash(relative))
				continue
			}
			locale, rawLocale, err := screenshotPlanFileLocale(relative)
			if err != nil {
				return err
			}
			if previous := localeNames[locale]; previous != "" && previous != rawLocale {
				return fmt.Errorf("duplicate locale directories %q and %q resolve to %s", previous, rawLocale, locale)
			}
			localeNames[locale] = rawLocale
			display, img, err := readScreenshotPlanImage(root, relative, types)
			if err != nil {
				return err
			}
			if display == "" {
				skipped = append(skipped, filepath.ToSlash(relative))
				continue
			}
			key := locale + "/" + display
			if groups[key] == nil {
				groups[key] = &ScreenshotPlanGroup{Locale: locale, DisplayType: display, Files: []string{}}
				digests[key] = map[screenshotPixelDigest][]string{}
			}
			digest := screenshotPixelDigestFromImage(img)
			for _, previous := range digests[key][digest] {
				other, err := root.OpenFile(previous)
				if err != nil {
					return err
				}
				otherImage, _, decodeErr := image.Decode(other)
				closeErr := other.Close()
				if decodeErr != nil {
					return decodeErr
				}
				if closeErr != nil {
					return closeErr
				}
				if screenshotPixelsEqual(img, otherImage) {
					return fmt.Errorf("screenshot %q has decoded pixels identical to %q", relative, previous)
				}
			}
			digests[key][digest] = append(digests[key][digest], relative)
			groups[key].Files = append(groups[key].Files, filepath.ToSlash(relative))
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, nil, err
	}
	result := make([]ScreenshotPlanGroup, 0, len(groups))
	for _, group := range groups {
		sort.Strings(group.Files)
		if err := validateUniqueScreenshotUploadFileNames(group.Files); err != nil {
			return nil, nil, err
		}
		if _, err := limitScreenshotUploadFiles(group.Files, 0, group.Locale+"/"+group.DisplayType); err != nil {
			return nil, nil, err
		}
		result = append(result, *group)
	}
	if len(result) == 0 {
		return nil, nil, fmt.Errorf("no screenshots match the requested device types in %q", path)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Locale != result[j].Locale {
			return result[i].Locale < result[j].Locale
		}
		return result[i].DisplayType < result[j].DisplayType
	})
	sort.Strings(skipped)
	return result, skipped, nil
}

func screenshotPlanFileLocale(path string) (string, string, error) {
	parts := strings.Split(filepath.ToSlash(path), "/")
	locale, raw := "", ""
	for _, part := range parts[:min(2, len(parts)-1)] {
		candidate, err := shared.CanonicalizeAppStoreLocalizationLocale(part)
		if err != nil || !isKnownAppStoreLocalizationLocale(candidate) {
			continue
		}
		if locale != "" {
			return "", "", fmt.Errorf("ambiguous locale directories in %q", path)
		}
		locale, raw = candidate, part
	}
	if locale == "" {
		return "", "", fmt.Errorf("screenshot %q requires a locale-first or device/locale folder layout", path)
	}
	return locale, raw, nil
}

func readScreenshotPlanImage(root rootfs.Root, path string, types []string) (string, image.Image, error) {
	f, err := root.OpenFile(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", nil, err
	}
	if err := asc.ValidateAssetFileInfo(path, info); err != nil {
		return "", nil, err
	}
	if err := validateOpenedScreenshotFileFormat(path, f); err != nil {
		return "", nil, err
	}
	dimensions, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", nil, err
	}
	selected := ""
	for _, display := range types {
		if asc.ValidateScreenshotDimensionsForSize(path, dimensions.Width, dimensions.Height, display) == nil {
			if selected != "" {
				return "", nil, fmt.Errorf("screenshot %q matches both %s and %s; select one device type", path, selected, display)
			}
			selected = display
		}
	}
	if selected == "" {
		return "", nil, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", nil, err
	}
	img, _, err := image.Decode(f)
	return selected, img, err
}
