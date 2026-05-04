package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type publishVersionMetadataOptions struct {
	VersionID string
	Version   string
	Dir       string
}

func applyPublishVersionMetadata(ctx context.Context, client *asc.Client, opts publishVersionMetadataOptions) ([]asc.LocalizationUploadLocaleResult, error) {
	versionID := strings.TrimSpace(opts.VersionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}

	valuesByLocale, err := loadPublishVersionMetadataValues(opts.Dir, opts.Version)
	if err != nil {
		return nil, err
	}

	results, warnings, err := shared.UploadVersionLocalizationsWithWarnings(ctx, client, versionID, valuesByLocale, false, shared.SubmitReadinessOptions{})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		if warnErr := shared.PrintSubmitReadinessCreateWarnings(os.Stderr, warnings); warnErr != nil {
			return nil, warnErr
		}
	}
	return results, nil
}

func loadPublishVersionMetadataValues(dir, version string) (map[string]map[string]string, error) {
	versionDir, err := publishVersionMetadataDir(dir, version)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", versionDir, err)
	}

	valuesByLocale := make(map[string]map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		locale := strings.TrimSpace(strings.TrimSuffix(entry.Name(), ".json"))
		if locale == "" {
			continue
		}
		path, err := metadata.VersionLocalizationFilePath(dir, version, locale)
		if err != nil {
			return nil, err
		}

		localization, err := metadata.ReadVersionLocalizationFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		values := shared.MapVersionLocalizationStrings(asc.AppStoreVersionLocalizationAttributes{
			Description:     localization.Description,
			Keywords:        localization.Keywords,
			MarketingURL:    localization.MarketingURL,
			PromotionalText: localization.PromotionalText,
			SupportURL:      localization.SupportURL,
			WhatsNew:        localization.WhatsNew,
		})
		if len(values) == 0 {
			return nil, fmt.Errorf("locale file %q has no version metadata values", entry.Name())
		}
		valuesByLocale[locale] = values
	}

	if len(valuesByLocale) == 0 {
		return nil, fmt.Errorf("no version metadata JSON files found in %q", versionDir)
	}
	return valuesByLocale, nil
}

func publishVersionMetadataDir(dir, version string) (string, error) {
	probePath, err := metadata.VersionLocalizationFilePath(dir, version, "en-US")
	if err != nil {
		return "", err
	}
	return filepath.Dir(probePath), nil
}
