package storeassets

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// A verified rendition is reusable only while its version, locale and header
// still identify the resources that supplied the compared bytes.
func (p *ImportPlan) checkHeaderTargets(ctx context.Context, client *asc.Client) error {
	if p.Clip == nil || len(p.Clip.HeaderImages) == 0 {
		return nil
	}
	experience, err := request(ctx, func(c context.Context) (*asc.AppClipDefaultExperienceResponse, error) {
		return client.GetAppStoreVersionAppClipDefaultExperience(c, p.VersionID)
	})
	if err != nil && !asc.IsNotFound(err) {
		return err
	}
	var experienceID string
	if experience != nil {
		experienceID = experience.Data.ID
	}
	if experienceID != p.Experience.ID {
		return fmt.Errorf("app clip experience changed since preflight; retry")
	}
	if experienceID == "" {
		return nil
	}
	localizations, err := clipLocalizations(ctx, client, p.Experience.ID)
	if err != nil {
		return err
	}
	for locale := range p.Clip.HeaderImages {
		previous := p.Headers[locale]
		localeID := p.Localizations[locale].ID
		var currentLocaleID string
		for _, loc := range localizations {
			if loc.Attributes.Locale == locale {
				currentLocaleID = loc.ID
				break
			}
		}
		if currentLocaleID != localeID {
			return fmt.Errorf("app clip locale %s changed since preflight; retry", locale)
		}
		if localeID == "" {
			continue
		}
		current, err := request(ctx, func(c context.Context) (*asc.AppClipHeaderImageResponse, error) {
			return client.GetAppClipDefaultExperienceLocalizationHeaderImage(c, localeID)
		})
		if err != nil && !asc.IsNotFound(err) {
			return err
		}
		var currentID, checksum string
		if current != nil {
			currentID, checksum = current.Data.ID, current.Data.Attributes.SourceFileChecksum
		}
		if currentID != previous.ID || !strings.EqualFold(checksum, previous.Attributes.SourceFileChecksum) {
			return fmt.Errorf("app clip header for %s changed since preflight; retry", locale)
		}
	}
	return nil
}
