package shared

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AppInfoLocalizationUpsertPlan describes a resolved app-info localization write.
// Planning resolves and validates the target before callers perform any mutations.
type AppInfoLocalizationUpsertPlan struct {
	AppInfoID      string
	LocalizationID string
	Locale         string
	Attributes     asc.AppInfoLocalizationAttributes
	Create         bool
}

// PlanAppInfoLocalizationUpsert resolves one locale and validates whether it can
// be created or updated without performing a mutation.
func PlanAppInfoLocalizationUpsert(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	locale string,
	values map[string]string,
) (*AppInfoLocalizationUpsertPlan, error) {
	return planAppInfoLocalizationUpsertTo(ctx, os.Stderr, client, appID, appInfoID, locale, values)
}

// PlanAppInfoLocalizationUpsert is PlanAppInfoLocalizationUpsert with the
// app info auto-selection notice written to d's stderr.
func (d *Deps) PlanAppInfoLocalizationUpsert(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	locale string,
	values map[string]string,
) (*AppInfoLocalizationUpsertPlan, error) {
	return planAppInfoLocalizationUpsertTo(ctx, d.IO.ErrOut, client, appID, appInfoID, locale, values)
}

func planAppInfoLocalizationUpsertTo(
	ctx context.Context,
	stderr io.Writer,
	client *asc.Client,
	appID string,
	appInfoID string,
	locale string,
	values map[string]string,
) (*AppInfoLocalizationUpsertPlan, error) {
	resolvedAppInfoID, err := resolveOwnedAppInfoIDTo(ctx, stderr, client, appID, appInfoID)
	if err != nil {
		return nil, err
	}

	localizations, err := client.GetAppInfoLocalizations(
		ctx,
		resolvedAppInfoID,
		asc.Limit[asc.AppInfoLocalizationsOption](200),
		asc.WithAppInfoLocalizationLocales([]string{locale}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app info localizations: %w", err)
	}
	if localizations == nil {
		return nil, fmt.Errorf("empty app info localizations response")
	}
	pageHasNext := strings.TrimSpace(localizations.Links.Next) != ""

	plan := &AppInfoLocalizationUpsertPlan{
		AppInfoID: resolvedAppInfoID,
		Locale:    locale,
	}
	switch len(localizations.Data) {
	case 0:
		if pageHasNext {
			return nil, MarkAmbiguousSelectionSample(AmbiguousLocalizationError("app info localization", locale, nil))
		}
		if strings.TrimSpace(values["name"]) == "" {
			return nil, UsageError("--name is required when creating an app info localization")
		}
		plan.Create = true
		plan.Attributes = buildAppInfoLocalizationAttributes(locale, values, true)
	case 1:
		if pageHasNext {
			return nil, MarkAmbiguousSelectionSample(AmbiguousLocalizationError("app info localization", locale, LocalizationCandidates(localizations.Data, func(attributes asc.AppInfoLocalizationAttributes) string { return attributes.Locale })))
		}
		plan.LocalizationID = strings.TrimSpace(localizations.Data[0].ID)
		if plan.LocalizationID == "" {
			return nil, fmt.Errorf("localization id is empty")
		}
		plan.Attributes = buildAppInfoLocalizationAttributes(locale, values, false)
	default:
		ambiguous := AmbiguousLocalizationError("app info localization", locale, LocalizationCandidates(localizations.Data, func(attributes asc.AppInfoLocalizationAttributes) string { return attributes.Locale }))
		if pageHasNext {
			return nil, MarkAmbiguousSelectionSample(ambiguous)
		}
		return nil, ambiguous
	}

	return plan, nil
}

// ApplyAppInfoLocalizationUpsert performs a previously planned localization write.
func ApplyAppInfoLocalizationUpsert(
	ctx context.Context,
	client *asc.Client,
	plan *AppInfoLocalizationUpsertPlan,
) (*asc.AppInfoLocalizationResponse, string, error) {
	if plan == nil {
		return nil, "", fmt.Errorf("app info localization plan is required")
	}
	if plan.Create {
		resp, err := client.CreateAppInfoLocalization(ctx, plan.AppInfoID, plan.Attributes)
		return resp, "create", err
	}
	resp, err := client.UpdateAppInfoLocalization(ctx, plan.LocalizationID, plan.Attributes)
	return resp, "update", err
}
