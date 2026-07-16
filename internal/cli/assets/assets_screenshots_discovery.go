package assets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func collectScreenshotUploadFiles(path string, maxScreenshots int) ([]string, error) {
	files, err := collectAssetPaths(path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in %q", path)
	}

	files, err = limitScreenshotUploadFiles(files, maxScreenshots, path)
	if err != nil {
		return nil, err
	}
	for _, filePath := range files {
		if err := asc.ValidateImageFile(filePath); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func limitScreenshotUploadFiles(files []string, maxScreenshots int, source string) ([]string, error) {
	if maxScreenshots < 0 || maxScreenshots > appScreenshotSetMaxScreenshots {
		return nil, fmt.Errorf("--max-screenshots must be between 0 and %d", appScreenshotSetMaxScreenshots)
	}
	if maxScreenshots > 0 {
		if len(files) > maxScreenshots {
			return append([]string(nil), files[:maxScreenshots]...), nil
		}
		return files, nil
	}

	if len(files) > appScreenshotSetMaxScreenshots {
		return nil, fmt.Errorf(
			"%q contains %d screenshots for one screenshot set; App Store screenshot sets allow at most %d images. Remove extra files or pass --max-screenshots %d to upload the first %d sorted files",
			source,
			len(files),
			appScreenshotSetMaxScreenshots,
			appScreenshotSetMaxScreenshots,
			appScreenshotSetMaxScreenshots,
		)
	}
	return files, nil
}

func limitScreenshotFanoutUploadFiles(localeAssets []screenshotLocaleAssetFiles, maxScreenshots int) ([]screenshotLocaleAssetFiles, error) {
	limited := make([]screenshotLocaleAssetFiles, 0, len(localeAssets))
	for _, item := range localeAssets {
		files, err := limitScreenshotUploadFiles(item.Files, maxScreenshots, item.Locale)
		if err != nil {
			return nil, err
		}
		item.Files = files
		limited = append(limited, item)
	}
	return limited, nil
}

func limitScreenshotUploadFilesForExistingSet(files []string, maxScreenshots int, existingScreenshots []asc.Resource[asc.AppScreenshotAttributes], replace bool, setID string) ([]string, error) {
	setLabel := strings.TrimSpace(setID)
	if setLabel == "" {
		setLabel = "target set"
	}
	if replace {
		return limitScreenshotUploadFiles(files, maxScreenshots, setLabel)
	}

	if maxScreenshots < 0 || maxScreenshots > appScreenshotSetMaxScreenshots {
		return nil, fmt.Errorf("--max-screenshots must be between 0 and %d", appScreenshotSetMaxScreenshots)
	}
	if maxScreenshots <= 0 {
		total := len(existingScreenshots) + len(files)
		if total > appScreenshotSetMaxScreenshots {
			return nil, fmt.Errorf(
				"%s already has %d screenshot(s); uploading %d more would exceed App Store screenshot set limit %d. Pass --replace to replace existing screenshots or --max-screenshots %d to upload only the remaining slot(s)",
				setLabel,
				len(existingScreenshots),
				len(files),
				appScreenshotSetMaxScreenshots,
				max(0, appScreenshotSetMaxScreenshots-len(existingScreenshots)),
			)
		}
		return files, nil
	}

	remaining := maxScreenshots - len(existingScreenshots)
	if remaining <= 0 {
		if len(files) == 0 {
			return files, nil
		}
		return nil, fmt.Errorf(
			"%s already has %d screenshot(s); --max-screenshots %d leaves no upload slots. Pass --replace to replace existing screenshots or choose a higher limit up to %d",
			setLabel,
			len(existingScreenshots),
			maxScreenshots,
			appScreenshotSetMaxScreenshots,
		)
	}
	if len(files) > remaining {
		return append([]string(nil), files[:remaining]...), nil
	}
	return files, nil
}

func uploadScreenshotsFanout(ctx context.Context, cfg screenshotUploadFanoutConfig) (asc.AppScreenshotFanoutUploadResult, error) {
	var zero asc.AppScreenshotFanoutUploadResult

	if cfg.Client == nil {
		return zero, fmt.Errorf("client is required")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = shared.ContextWithTimeout
	}
	cfg.ExecuteUpload = resolveScreenshotUploadExecutor(cfg.ExecuteUpload, cfg.UploadScreenshot)

	localeAssets := cfg.LocaleAssets
	var err error
	if localeAssets == nil {
		localeAssets, err = collectLocaleAssetFilesWithLimit(cfg.RootPath, cfg.DisplayType, cfg.MaxScreenshots)
		if err != nil {
			return zero, err
		}
		cfg.LocaleAssetsCanonical = true
	}
	if !cfg.LocaleAssetsCanonical {
		localeAssets, err = canonicalizeUniqueScreenshotFanoutLocaleAssets(localeAssets)
		if err != nil {
			return zero, err
		}
	}

	requestCtx, cancel := cfg.RequestContext(ctx)
	localizationsResp, err := cfg.Client.GetAppStoreVersionLocalizations(requestCtx, cfg.VersionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	cancel()
	if err != nil {
		return zero, fmt.Errorf("fetch version localizations: %w", err)
	}

	localizationIDsByLocale := make(map[string]string, len(localizationsResp.Data))
	for _, item := range localizationsResp.Data {
		localeKey := normalizeFanoutLocaleKey(item.Attributes.Locale)
		if localeKey == "" {
			continue
		}
		localizationIDsByLocale[localeKey] = strings.TrimSpace(item.ID)
	}

	missingLocales := make([]string, 0)
	for _, item := range localeAssets {
		if localizationIDsByLocale[normalizeFanoutLocaleKey(item.Locale)] == "" {
			missingLocales = append(missingLocales, item.Locale)
		}
	}
	if len(missingLocales) > 0 {
		sort.Strings(missingLocales)
		return zero, fmt.Errorf("no matching App Store version localizations found for locales: %s", strings.Join(missingLocales, ", "))
	}

	result := asc.AppScreenshotFanoutUploadResult{
		AppID:         cfg.AppID,
		Version:       cfg.Version,
		VersionID:     cfg.VersionID,
		Platform:      cfg.Platform,
		DisplayType:   cfg.DisplayType,
		DryRun:        cfg.DryRun,
		Localizations: make([]asc.AppScreenshotLocalizationUploadResult, 0, len(localeAssets)),
	}

	for _, item := range localeAssets {
		localizationID := localizationIDsByLocale[normalizeFanoutLocaleKey(item.Locale)]
		uploadResult, err := cfg.ExecuteUpload(ctx, screenshotUploadConfig[asc.AppScreenshotUploadResult]{
			Client:         cfg.Client,
			LocalizationID: localizationID,
			DisplayType:    cfg.DisplayType,
			Files:          item.Files,
			SkipExisting:   cfg.SkipExisting,
			Replace:        cfg.Replace,
			DryRun:         cfg.DryRun,
			MaxScreenshots: cfg.MaxScreenshots,
			RequestContext: cfg.RequestContext,
			UploadContext:  contextWithAssetUploadTimeout,
			Access:         appStoreVersionScreenshotSetAccess,
		}, "")
		if err != nil {
			if hasAppScreenshotUploadResultOutput(uploadResult) {
				result.Localizations = append(result.Localizations, buildFanoutLocalizationUploadResult(item.Locale, uploadResult))
			}
			return result, fmt.Errorf("upload locale %s: %w", item.Locale, err)
		}
		result.Localizations = append(result.Localizations, buildFanoutLocalizationUploadResult(item.Locale, uploadResult))
	}

	return result, nil
}

func collectLocaleAssetFiles(rootPath, displayType string) ([]screenshotLocaleAssetFiles, error) {
	return collectLocaleAssetFilesWithLimit(rootPath, displayType, 0)
}

func collectLocaleAssetFilesWithLimit(rootPath, displayType string, maxScreenshots int) ([]screenshotLocaleAssetFiles, error) {
	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", rootPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fan-out upload path %q must be a directory containing locale subdirectories", rootPath)
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	results := make([]screenshotLocaleAssetFiles, 0, len(entries))
	seenLocales := make(map[string]string, len(entries))
	for _, entry := range entries {
		if shouldIgnoreFanoutEntryName(entry.Name()) {
			continue
		}

		entryPath := filepath.Join(rootPath, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to read symlink %q", entryPath)
		}
		if !info.IsDir() {
			continue
		}

		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(entry.Name())
		if err != nil {
			hasMatchingFiles, matchErr := directoryContainsMatchingScreenshotFiles(entryPath, displayType)
			if matchErr != nil {
				return nil, matchErr
			}
			if !hasMatchingFiles {
				continue
			}
			return nil, fmt.Errorf("invalid locale directory %q: %w", entry.Name(), err)
		}
		if !isKnownAppStoreLocalizationLocale(locale) {
			hasMatchingFiles, matchErr := directoryContainsMatchingScreenshotFiles(entryPath, displayType)
			if matchErr != nil {
				return nil, matchErr
			}
			if !hasMatchingFiles {
				continue
			}
		}
		files, err := collectLocaleAssetFilesRecursiveWithLimit(entryPath, displayType, maxScreenshots)
		if err != nil {
			return nil, fmt.Errorf("locale %s: %w", locale, err)
		}
		if err := registerUniqueCanonicalFanoutLocale(locale, entry.Name(), "fan-out path", "dirs", seenLocales); err != nil {
			return nil, err
		}
		results = append(results, screenshotLocaleAssetFiles{
			Locale: locale,
			Files:  files,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no locale directories found in %q", rootPath)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Locale) < strings.ToLower(results[j].Locale)
	})
	return results, nil
}

type screenshotMatchWalkOptions struct {
	ignoreInvalidFiles bool
	ignoreSymlinks     bool
	onMatch            func(path string) error
}

func walkMatchingScreenshotFiles(rootPath, displayType string, opts screenshotMatchWalkOptions) error {
	return filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != rootPath && shouldIgnoreFanoutEntryName(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if opts.ignoreSymlinks {
				return nil
			}
			return fmt.Errorf("refusing to read symlink %q", path)
		}
		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !isSupportedScreenshotUploadFile(path) {
			return nil
		}
		if err := asc.ValidateImageFile(path); err != nil {
			if opts.ignoreInvalidFiles {
				return nil
			}
			return err
		}
		isMatch, err := screenshotMatchesDisplayType(path, displayType)
		if err != nil {
			if opts.ignoreInvalidFiles {
				return nil
			}
			return err
		}
		if !isMatch || opts.onMatch == nil {
			return nil
		}
		if err := opts.onMatch(path); err != nil {
			return err
		}
		return nil
	})
}

func collectLocaleAssetFilesRecursive(rootPath, displayType string) ([]string, error) {
	return collectLocaleAssetFilesRecursiveWithLimit(rootPath, displayType, 0)
}

func collectLocaleAssetFilesRecursiveWithLimit(rootPath, displayType string, maxScreenshots int) ([]string, error) {
	if maxScreenshots > 0 {
		return collectLimitedLocaleAssetFilesRecursive(rootPath, displayType, maxScreenshots)
	}

	files := make([]string, 0)
	err := walkMatchingScreenshotFiles(rootPath, displayType, screenshotMatchWalkOptions{
		onMatch: func(path string) error {
			files = append(files, path)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no screenshot files matching %s found in %q", displayType, rootPath)
	}
	sort.Strings(files)
	if err := validateUniqueScreenshotUploadFileNames(files); err != nil {
		return nil, err
	}
	return files, nil
}

func collectLimitedLocaleAssetFilesRecursive(rootPath, displayType string, maxScreenshots int) ([]string, error) {
	candidates, err := collectSupportedScreenshotCandidateFiles(rootPath)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, min(maxScreenshots, len(candidates)))
	for _, path := range candidates {
		if len(files) >= maxScreenshots {
			break
		}
		if err := asc.ValidateImageFile(path); err != nil {
			return nil, err
		}
		matches, err := screenshotMatchesDisplayType(path, displayType)
		if err != nil {
			return nil, err
		}
		if matches {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no screenshot files matching %s found in %q", displayType, rootPath)
	}
	if err := validateUniqueScreenshotUploadFileNames(files); err != nil {
		return nil, err
	}
	return files, nil
}

func validateUniqueScreenshotUploadFileNames(files []string) error {
	seen := make(map[string]string, len(files))
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		key := strings.ToLower(fileName)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf(
				"duplicate screenshot file name %q (%q, %q); App Store screenshot ordering uses uploaded file names, so rename one file before uploading",
				fileName,
				previous,
				filePath,
			)
		}
		seen[key] = filePath
	}
	return nil
}

func collectSupportedScreenshotCandidateFiles(rootPath string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != rootPath && shouldIgnoreFanoutEntryName(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to read symlink %q", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !isSupportedScreenshotUploadFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func directoryContainsMatchingScreenshotFiles(rootPath, displayType string) (bool, error) {
	found := false
	err := walkMatchingScreenshotFiles(rootPath, displayType, screenshotMatchWalkOptions{
		ignoreInvalidFiles: true,
		ignoreSymlinks:     true,
		onMatch: func(path string) error {
			found = true
			return filepath.SkipAll
		},
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return false, err
	}
	return found, nil
}

func canonicalizeUniqueScreenshotFanoutLocaleAssets(localeAssets []screenshotLocaleAssetFiles) ([]screenshotLocaleAssetFiles, error) {
	result := make([]screenshotLocaleAssetFiles, 0, len(localeAssets))
	seen := make(map[string]string, len(localeAssets))
	for _, item := range localeAssets {
		canonicalLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(item.Locale)
		if err != nil {
			return nil, fmt.Errorf("invalid locale %q in fan-out upload: %w", item.Locale, err)
		}
		if err := registerUniqueCanonicalFanoutLocale(canonicalLocale, item.Locale, "fan-out upload", "inputs", seen); err != nil {
			return nil, err
		}
		if err := validateUniqueScreenshotUploadFileNames(item.Files); err != nil {
			return nil, fmt.Errorf("locale %s: %w", canonicalLocale, err)
		}
		result = append(result, screenshotLocaleAssetFiles{
			Locale: canonicalLocale,
			Files:  item.Files,
		})
	}
	return result, nil
}

func registerUniqueCanonicalFanoutLocale(canonicalLocale, source, scope, itemLabel string, seen map[string]string) error {
	localeKey := normalizeFanoutLocaleKey(canonicalLocale)
	if previous, ok := seen[localeKey]; ok {
		return fmt.Errorf("duplicate locale %q in %s (%s: %q, %q)", canonicalLocale, scope, itemLabel, previous, source)
	}
	seen[localeKey] = source
	return nil
}

func normalizeFanoutLocaleKey(locale string) string {
	return strings.ToLower(shared.NormalizeLocaleCode(locale))
}

func isKnownAppStoreLocalizationLocale(locale string) bool {
	_, ok := knownAppStoreLocalizationLocales[normalizeFanoutLocaleKey(locale)]
	return ok
}

func screenshotMatchesDisplayType(path, displayType string) (bool, error) {
	allowed, ok := asc.ScreenshotDimensions(displayType)
	if !ok {
		return false, fmt.Errorf("unsupported screenshot display type %q", displayType)
	}

	dims, err := asc.ReadImageDimensions(path)
	if err != nil {
		return false, err
	}

	for _, dim := range allowed {
		if dim.Width == dims.Width && dim.Height == dims.Height {
			return true, nil
		}
	}
	return false, nil
}

func shouldIgnoreFanoutEntryName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isSupportedScreenshotUploadFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}
