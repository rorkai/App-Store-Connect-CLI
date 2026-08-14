package shared

import (
	"context"
	"fmt"
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
	resolvedAppInfoID, err := ResolveOwnedAppInfoID(ctx, client, appID, appInfoID)
	if err != nil {
		return nil, err
	}

	localizations, err := client.GetAppInfoLocalizations(
		ctx,
		resolvedAppInfoID,
		asc.WithAppInfoLocalizationsLimit(200),
		asc.WithAppInfoLocalizationLocales([]string{locale}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app info localizations: %w", err)
	}
	if localizations == nil {
		return nil, fmt.Errorf("empty app info localizations response")
	}

	plan := &AppInfoLocalizationUpsertPlan{
		AppInfoID: resolvedAppInfoID,
		Locale:    locale,
	}
	switch len(localizations.Data) {
	case 0:
		if strings.TrimSpace(values["name"]) == "" {
			return nil, UsageError("--name is required when creating an app info localization")
		}
		plan.Create = true
		plan.Attributes = buildAppInfoLocalizationAttributes(locale, values, true)
	case 1:
		plan.LocalizationID = strings.TrimSpace(localizations.Data[0].ID)
		if plan.LocalizationID == "" {
			return nil, fmt.Errorf("localization id is empty")
		}
		plan.Attributes = buildAppInfoLocalizationAttributes(locale, values, false)
	default:
		return nil, fmt.Errorf("multiple app info localizations found for locale %q", locale)
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
