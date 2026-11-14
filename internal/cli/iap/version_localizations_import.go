package iap

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPVersionLocalizationsImportCommand creates or updates IAP version
// localizations from a JSON file.
func IAPVersionLocalizationsImportCommand() *ffcli.Command {
	return shared.NewVersionLocalizationImportCommand(shared.VersionLocalizationImportConfig{
		CommandPath:         "iap versions localizations import",
		VersionResourceType: "inAppPurchaseVersions",
		VersionFlagUsage:    "In-app purchase version ID",
		ResourceType:        "inAppPurchaseLocalizations",
		OptionalField:       "description",
		ExampleValues:       `{"en-US": {"name": "Pro", "description": "Remove limits"}, "de-DE": {"name": "Pro"}}`,
		NewClient:           func() (*asc.Client, error) { return iapVersionClientFactory() },
		Ops:                 iapVersionLocalizationImportOps,
	})
}

func iapVersionLocalizationImportOps(client *asc.Client) shared.VersionLocalizationImportOps {
	return shared.VersionLocalizationImportOps{
		List: func(ctx context.Context, versionID string) ([]shared.VersionLocalization, error) {
			first, err := client.GetInAppPurchaseVersionLocalizations(ctx, versionID, asc.Limit[asc.IAPVersionLocalizationsOption](200))
			if err != nil {
				return nil, err
			}
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetInAppPurchaseVersionLocalizations(ctx, versionID, asc.WithIAPVersionLocalizationsNextURL(nextURL))
			})
			if err != nil {
				return nil, err
			}
			resp := all.(*asc.InAppPurchaseLocalizationsResponse)
			existing := make([]shared.VersionLocalization, 0, len(resp.Data))
			for _, item := range resp.Data {
				existing = append(existing, shared.VersionLocalization{
					ID:     item.ID,
					Locale: item.Attributes.Locale,
					Values: map[string]string{"name": item.Attributes.Name, "description": item.Attributes.Description},
				})
			}
			return existing, nil
		},
		Create: func(ctx context.Context, versionID, locale string, values map[string]string) (string, error) {
			resp, err := client.CreateInAppPurchaseLocalizationV2(ctx, versionID, asc.InAppPurchaseLocalizationV2CreateAttributes{
				Name:        values["name"],
				Locale:      locale,
				Description: shared.VersionLocalizationValue(values, "description"),
			})
			if err != nil {
				return "", err
			}
			return resp.Data.ID, nil
		},
		Update: func(ctx context.Context, localizationID string, values map[string]string) error {
			_, err := client.UpdateInAppPurchaseLocalizationV2(ctx, localizationID, asc.InAppPurchaseLocalizationUpdateAttributes{
				Name:        shared.VersionLocalizationValue(values, "name"),
				Description: shared.VersionLocalizationValue(values, "description"),
			})
			return err
		},
	}
}
