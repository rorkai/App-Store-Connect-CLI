package migrate

import (
	"fmt"
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

type ScreenshotPlan struct {
	Locale      string   `json:"locale"`
	DisplayType string   `json:"displayType"`
	Files       []string `json:"files"`
	sourceRoot  *screenshotSourceRoot
	sources     map[string]screenshotSource
}

type ScreenshotUploadResult struct {
	Locale      string                      `json:"locale"`
	DisplayType string                      `json:"displayType"`
	Uploaded    []asc.AssetUploadResultItem `json:"uploaded,omitempty"`
	Skipped     []SkippedItem               `json:"skipped,omitempty"`
	// createdSet records that this run created the screenshot set, so a later
	// failure still reports the App Store Connect state it left behind even
	// when no asset finished uploading.
	createdSet bool
}

const maxMigrateScreenshotFileSize = int64(1024 * 1024 * 1024)

// discoverScreenshotPlanForUpload pins discovery to one rooted directory
// handle and records each file's identity. Upload reopens one file at a time
// through that root, preventing path redirection without retaining an
// unbounded number of descriptors.
func discoverScreenshotPlanForUpload(screenshotsDir string) ([]ScreenshotPlan, []SkippedItem, error) {
	return discoverScreenshotPlanWithOpenFiles(screenshotsDir, true)
}

func discoverScreenshotPlanWithOpenFiles(screenshotsDir string, retainOpenFiles bool) ([]ScreenshotPlan, []SkippedItem, error) {
	// A screenshots directory inside the working directory ships with the
	// checkout, so refuse symlinked components before traversing it; an
	// operator-selected external directory remains its own trusted root.
	root, prefix, err := newMigrateContentRoot(screenshotsDir)
	if err != nil {
		return nil, nil, err
	}
	if err := checkContentRootContained(root, prefix); err != nil {
		return nil, nil, err
	}
	rooted, err := os.OpenRoot(root.Path())
	if err != nil {
		return nil, nil, err
	}
	defer rooted.Close()
	contentRoot, err := rooted.OpenRoot(filepath.ToSlash(prefix))
	if err != nil {
		return nil, nil, err
	}
	sourceRoot := &screenshotSourceRoot{root: contentRoot}
	keepSourceRoot := false
	defer func() {
		if !keepSourceRoot {
			_ = sourceRoot.root.Close()
		}
	}()
	entries, err := fs.ReadDir(contentRoot.FS(), ".")
	if err != nil {
		return nil, nil, err
	}

	type planKey struct {
		locale      string
		displayType string
	}
	type planFiles struct {
		paths   []string
		sources map[string]screenshotSource
	}
	plans := make(map[planKey]*planFiles)
	var skipped []SkippedItem

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			// A symlinked locale directory would read screenshots from outside
			// the screenshots root, so report it instead of silently ignoring
			// it, matching the metadata scan.
			skipped = append(skipped, SkippedItem{
				Path:   filepath.Join(screenshotsDir, entry.Name()),
				Reason: fmt.Sprintf("skipped symlinked screenshots entry %q", entry.Name()),
			})
			continue
		}
		if !entry.IsDir() {
			continue
		}
		localeName := entry.Name()
		if localeName == "default" {
			continue
		}
		locale, err := normalizeLocale(localeName)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid locale directory %q: %w", localeName, err)
		}
		localeDir := filepath.Join(screenshotsDir, entry.Name())
		files, localeSkipped, err := collectScreenshotFiles(contentRoot, entry.Name(), localeDir)
		if err != nil {
			return nil, nil, err
		}
		skipped = append(skipped, localeSkipped...)
		for _, candidate := range files {
			file, err := secureopen.OpenExistingNoFollowInRoot(contentRoot, candidate.relative)
			if err != nil {
				return nil, nil, fmt.Errorf("open screenshot file %q: %w", candidate.path, err)
			}
			dimensions, err := readOpenedImageDimensions(candidate.path, file)
			if err != nil {
				_ = file.Close()
				return nil, nil, fmt.Errorf("invalid screenshot file %q: %w", candidate.path, err)
			}
			if err := validateOpenedImageFormat(candidate.path, file); err != nil {
				_ = file.Close()
				return nil, nil, fmt.Errorf("invalid screenshot file %q: %w", candidate.path, err)
			}
			displayType, err := inferScreenshotDisplayTypeFromDimensions(candidate.path, dimensions.Width, dimensions.Height)
			if err != nil {
				_ = file.Close()
				return nil, nil, err
			}
			key := planKey{locale: locale, displayType: displayType}
			if plans[key] == nil {
				plans[key] = &planFiles{sources: make(map[string]screenshotSource)}
			}
			plans[key].paths = append(plans[key].paths, candidate.path)
			if retainOpenFiles {
				info, err := file.Stat()
				if err != nil {
					_ = file.Close()
					return nil, nil, err
				}
				plans[key].sources[candidate.path] = screenshotSource{
					relative: candidate.relative,
					info:     info,
				}
			}
			_ = file.Close()
		}
	}

	result := make([]ScreenshotPlan, 0, len(plans))
	for key, files := range plans {
		sort.Strings(files.paths)
		plan := ScreenshotPlan{
			Locale:      key.locale,
			DisplayType: key.displayType,
			Files:       files.paths,
		}
		if retainOpenFiles {
			plan.sourceRoot = sourceRoot
			plan.sources = files.sources
		}
		result = append(result, plan)
	}
	keepSourceRoot = retainOpenFiles && len(result) > 0

	sort.Slice(result, func(i, j int) bool {
		if result[i].Locale == result[j].Locale {
			return result[i].DisplayType < result[j].DisplayType
		}
		return result[i].Locale < result[j].Locale
	})

	return result, skipped, nil
}

func readOpenedImageDimensions(path string, file *os.File) (asc.ImageDimensions, error) {
	info, err := file.Stat()
	if err != nil {
		return asc.ImageDimensions{}, err
	}
	if !info.Mode().IsRegular() {
		return asc.ImageDimensions{}, fmt.Errorf("expected regular file: %q", path)
	}
	if info.Size() <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("file is empty: %q", path)
	}
	if info.Size() > maxMigrateScreenshotFileSize {
		return asc.ImageDimensions{}, fmt.Errorf("file size exceeds %d bytes: %q", maxMigrateScreenshotFileSize, path)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asc.ImageDimensions{}, err
	}
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return asc.ImageDimensions{}, fmt.Errorf("decode image dimensions for %q: %w", path, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("invalid image dimensions %dx%d for %q", cfg.Width, cfg.Height, path)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asc.ImageDimensions{}, err
	}
	return asc.ImageDimensions{Width: cfg.Width, Height: cfg.Height}, nil
}

// validateOpenedImageFormat rejects a discovered screenshot whose encoded
// format contradicts its file name, using the handle discovery already pinned.
// App Store Connect derives the asset content type from that name, so without
// this the import creates the localization and the screenshot set, reserves the
// asset, and uploads every byte before the mismatch is reported back.
func validateOpenedImageFormat(path string, file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	format, err := asc.ReadImageFormatFrom(file)
	if err != nil {
		return err
	}
	if err := asc.ValidateImageFormatMatchesExtension(path, format); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return nil
}

type screenshotSourceRoot struct {
	root *os.Root
}

type screenshotSource struct {
	relative string
	info     os.FileInfo
}

func (p ScreenshotPlan) openedFile(path string) (*os.File, bool, error) {
	source, ok := p.sources[path]
	if !ok || p.sourceRoot == nil || p.sourceRoot.root == nil {
		return nil, false, nil
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(p.sourceRoot.root, source.relative)
	if err != nil {
		return nil, true, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, true, err
	}
	if !os.SameFile(source.info, info) ||
		source.info.Size() != info.Size() ||
		!source.info.ModTime().Equal(info.ModTime()) {
		_ = file.Close()
		return nil, true, fmt.Errorf("screenshot file %q changed after discovery", path)
	}
	return file, true, nil
}

func closeScreenshotPlans(plans []ScreenshotPlan) {
	closed := make(map[*screenshotSourceRoot]struct{})
	for _, plan := range plans {
		if plan.sourceRoot == nil || plan.sourceRoot.root == nil {
			continue
		}
		if _, ok := closed[plan.sourceRoot]; ok {
			continue
		}
		closed[plan.sourceRoot] = struct{}{}
		_ = plan.sourceRoot.root.Close()
	}
}

type screenshotCandidate struct {
	path     string
	relative string
}

func collectScreenshotFiles(rooted *os.Root, localeRelative, localeDir string) ([]screenshotCandidate, []SkippedItem, error) {
	localeRoot, err := rooted.OpenRoot(filepath.ToSlash(localeRelative))
	if err != nil {
		return nil, nil, err
	}
	defer localeRoot.Close()

	var files []screenshotCandidate
	var skipped []SkippedItem
	err = fs.WalkDir(localeRoot.FS(), ".", func(relative string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		displayPath := localeDir
		if relative != "." {
			displayPath = filepath.Join(localeDir, filepath.FromSlash(strings.TrimPrefix(relative, "./")))
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to read symlink %q", displayPath)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if shouldSkipScreenshotFile(displayPath) {
			skipped = append(skipped, SkippedItem{
				Path:   displayPath,
				Reason: "unsupported screenshot file",
			})
			return nil
		}
		files = append(files, screenshotCandidate{
			path:     displayPath,
			relative: filepath.ToSlash(filepath.Join(localeRelative, relative)),
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		skipped = append(skipped, SkippedItem{
			Path:   localeDir,
			Reason: "no supported screenshots found",
		})
		return nil, skipped, nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})
	return files, skipped, nil
}

func inferScreenshotDisplayTypeFromDimensions(path string, width, height int) (string, error) {
	hint := inferDisplayTypeFromFilename(path)
	if hint != "" {
		if !asc.IsValidScreenshotDisplayType(hint) {
			return "", fmt.Errorf("unsupported screenshot display type %q for %s", hint, path)
		}
		return hint, nil
	}

	if displayType := inferDisplayTypeFromDimensions(width, height); displayType != "" {
		if !asc.IsValidScreenshotDisplayType(displayType) {
			return "", fmt.Errorf("unsupported screenshot display type %q for %s", displayType, path)
		}
		return displayType, nil
	}

	if candidates := ambiguousDisplayTypesForDimensions(width, height); len(candidates) > 0 {
		return "", fmt.Errorf(
			"ambiguous screenshot display type for %q: %dx%d matches %s; name the file after the device (for example %q or %q)",
			path, width, height, strings.Join(candidates, " and "), "apple tv", "vision pro",
		)
	}

	return "", fmt.Errorf("unable to infer screenshot display type for %q", path)
}

// ambiguousDisplayTypesForDimensions reports display types that share one size,
// so a screenshot is never routed to a guessed slot. Apple TV 4K and Apple
// Vision Pro both use 3840x2160, and only the file name can separate them.
func ambiguousDisplayTypesForDimensions(width, height int) []string {
	maxDim, minDim := orderedDimensions(width, height)
	if maxDim == 3840 && minDim == 2160 {
		return []string{"APP_APPLE_TV", "APP_APPLE_VISION_PRO"}
	}
	return nil
}

func inferDisplayTypeFromFilename(path string) string {
	name := strings.ToLower(filepath.Base(path))
	if strings.Contains(name, "app_ipad_pro_129") ||
		(strings.Contains(name, "12.9") && strings.Contains(name, "2nd generation")) {
		return "APP_IPAD_PRO_129"
	}

	replacements := map[string]string{
		"iphone duo":       "APP_IPHONE_DUO",
		"iphone_duo":       "APP_IPHONE_DUO",
		"iphoneduo":        "APP_IPHONE_DUO",
		"iphone 6.9":       "APP_IPHONE_69",
		"iphone6.9":        "APP_IPHONE_69",
		"iphone 6.7":       "APP_IPHONE_67",
		"iphone6.7":        "APP_IPHONE_67",
		"iphone 6.5":       "APP_IPHONE_65",
		"iphone6.5":        "APP_IPHONE_65",
		"iphone 6.1":       "APP_IPHONE_61",
		"iphone6.1":        "APP_IPHONE_61",
		"iphone 5.8":       "APP_IPHONE_58",
		"iphone5.8":        "APP_IPHONE_58",
		"iphone 5.5":       "APP_IPHONE_55",
		"iphone5.5":        "APP_IPHONE_55",
		"iphone 4.7":       "APP_IPHONE_47",
		"iphone4.7":        "APP_IPHONE_47",
		"iphone 4.0":       "APP_IPHONE_40",
		"iphone4.0":        "APP_IPHONE_40",
		"iphone 3.5":       "APP_IPHONE_35",
		"iphone3.5":        "APP_IPHONE_35",
		"ipad 11":          "APP_IPAD_PRO_3GEN_11",
		"ipad11":           "APP_IPAD_PRO_3GEN_11",
		"ipad 10.5":        "APP_IPAD_105",
		"ipad10.5":         "APP_IPAD_105",
		"ipad 9.7":         "APP_IPAD_97",
		"ipad9.7":          "APP_IPAD_97",
		"ipad pro 13":      "APP_IPAD_PRO_3GEN_129",
		"ipad pro 13-inch": "APP_IPAD_PRO_3GEN_129",
		"ipad pro 13 inch": "APP_IPAD_PRO_3GEN_129",
		"ipad-pro-13":      "APP_IPAD_PRO_3GEN_129",
		"ipad 12.9":        "APP_IPAD_PRO_3GEN_129",
		"ipad12.9":         "APP_IPAD_PRO_3GEN_129",
		"apple tv":         "APP_APPLE_TV",
		"appletv":          "APP_APPLE_TV",
		"apple_tv":         "APP_APPLE_TV",
		"vision pro":       "APP_APPLE_VISION_PRO",
		"visionpro":        "APP_APPLE_VISION_PRO",
		"vision_pro":       "APP_APPLE_VISION_PRO",
		"desktop":          "APP_DESKTOP",
		"mac":              "APP_DESKTOP",
		"watch ultra":      "APP_WATCH_ULTRA",
		"watch series 10":  "APP_WATCH_SERIES_10",
		"watch series 7":   "APP_WATCH_SERIES_7",
		"watch series 4":   "APP_WATCH_SERIES_4",
		"watch series 3":   "APP_WATCH_SERIES_3",
	}
	for key, value := range replacements {
		if strings.Contains(name, key) {
			return value
		}
	}
	return ""
}

func orderedDimensions(width, height int) (int, int) {
	if height > width {
		return height, width
	}
	return width, height
}

func inferDisplayTypeFromDimensions(width, height int) string {
	maxDim, minDim := orderedDimensions(width, height)
	switch {
	case (maxDim == 2034 && minDim == 1398) || (maxDim == 2853 && minDim == 2007):
		return "APP_IPHONE_DUO"
	case maxDim == 2688 && minDim == 1242:
		return "APP_IPHONE_65"
	case maxDim == 2778 && minDim == 1284:
		return "APP_IPHONE_65"
	case maxDim == 2868 && minDim == 1320:
		return "APP_IPHONE_69"
	case maxDim == 2736 && minDim == 1260:
		return "APP_IPHONE_69"
	case maxDim == 2796 && minDim == 1290:
		return "APP_IPHONE_67"
	case maxDim == 2622 && minDim == 1206:
		return "APP_IPHONE_61"
	case maxDim == 2556 && minDim == 1179:
		return "APP_IPHONE_61"
	case maxDim == 2532 && minDim == 1170:
		return "APP_IPHONE_58"
	case maxDim == 2436 && minDim == 1125:
		return "APP_IPHONE_58"
	case maxDim == 2340 && minDim == 1080:
		return "APP_IPHONE_58"
	case maxDim == 2208 && minDim == 1242:
		return "APP_IPHONE_55"
	case maxDim == 1334 && minDim == 750:
		return "APP_IPHONE_47"
	case maxDim == 1136 && minDim == 640:
		return "APP_IPHONE_40"
	case maxDim == 960 && minDim == 640:
		return "APP_IPHONE_35"
	case maxDim == 2732 && minDim == 2048:
		return "APP_IPAD_PRO_3GEN_129"
	case maxDim == 2752 && minDim == 2064:
		return "APP_IPAD_PRO_3GEN_129"
	case maxDim == 2420 && minDim == 1668:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2388 && minDim == 1668:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2360 && minDim == 1640:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2266 && minDim == 1488:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2224 && minDim == 1668:
		return "APP_IPAD_105"
	case maxDim == 2048 && minDim == 1536:
		return "APP_IPAD_97"
	case maxDim == 1920 && minDim == 1080:
		return "APP_APPLE_TV"
	// Apple Watch and Mac sizes, matching internal/screenshotcatalog. Without
	// them a watchOS or macOS fastlane tree aborts the whole import instead of
	// selecting a slot.
	case maxDim == 514 && minDim == 422:
		return "APP_WATCH_ULTRA"
	case maxDim == 502 && minDim == 410:
		return "APP_WATCH_ULTRA"
	case maxDim == 496 && minDim == 416:
		return "APP_WATCH_SERIES_10"
	case maxDim == 484 && minDim == 396:
		return "APP_WATCH_SERIES_7"
	case maxDim == 448 && minDim == 368:
		return "APP_WATCH_SERIES_4"
	case maxDim == 390 && minDim == 312:
		return "APP_WATCH_SERIES_3"
	case maxDim == 1280 && minDim == 800:
		return "APP_DESKTOP"
	case maxDim == 1440 && minDim == 900:
		return "APP_DESKTOP"
	case maxDim == 2560 && minDim == 1600:
		return "APP_DESKTOP"
	case maxDim == 2880 && minDim == 1800:
		return "APP_DESKTOP"
	default:
		return ""
	}
}

func shouldSkipScreenshotFile(path string) bool {
	name := filepath.Base(path)
	if strings.HasPrefix(name, ".") {
		return true
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg":
		return false
	default:
		return true
	}
}
