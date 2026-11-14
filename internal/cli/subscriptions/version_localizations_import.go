package subscriptions

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsVersionLocalizationsImportCommand creates or updates
// subscription version localizations from a JSON file.
func SubscriptionsVersionLocalizationsImportCommand() *ffcli.Command {
	return shared.NewVersionLocalizationImportCommand(shared.VersionLocalizationImportConfig{
		CommandPath:         "subscriptions versions localizations import",
		VersionResourceType: "subscriptionVersions",
		VersionFlagUsage:    "Subscription version ID",
		ResourceType:        "subscriptionLocalizations",
		OptionalField:       "description",
		ExampleValues:       `{"en-US": {"name": "Premium", "description": "All features"}, "de-DE": {"name": "Premium"}}`,
		NewClient:           shared.GetASCClient,
		Ops:                 subscriptionVersionLocalizationImportOps,
	})
}

// SubscriptionsGroupsVersionLocalizationsImportCommand creates or updates
// subscription group version localizations from a JSON file.
func SubscriptionsGroupsVersionLocalizationsImportCommand() *ffcli.Command {
	return shared.NewVersionLocalizationImportCommand(shared.VersionLocalizationImportConfig{
		CommandPath:         "subscriptions groups versions localizations import",
		VersionResourceType: "subscriptionGroupVersions",
		VersionFlagUsage:    "Subscription group version ID",
		ResourceType:        "subscriptionGroupLocalizations",
		OptionalField:       "customAppName",
		ExampleValues:       `{"en-US": {"name": "Premium", "customAppName": "Premium App"}, "de-DE": {"name": "Premium"}}`,
		NewClient:           func() (*asc.Client, error) { return subscriptionGroupVersionClientFactory() },
		Ops:                 subscriptionGroupVersionLocalizationImportOps,
	})
}

func subscriptionVersionLocalizationImportOps(client *asc.Client) shared.VersionLocalizationImportOps {
	return shared.VersionLocalizationImportOps{
		List: func(ctx context.Context, versionID string) ([]shared.VersionLocalization, error) {
			first, err := client.GetSubscriptionVersionLocalizations(ctx, versionID, asc.Limit[asc.SubscriptionVersionLocalizationsOption](200))
			if err != nil {
				return nil, err
			}
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetSubscriptionVersionLocalizations(ctx, versionID, asc.WithSubscriptionVersionLocalizationsNextURL(nextURL))
			})
			if err != nil {
				return nil, err
			}
			resp := all.(*asc.SubscriptionLocalizationsV2Response)
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
			resp, err := client.CreateSubscriptionLocalizationV2(ctx, versionID, asc.SubscriptionLocalizationV2CreateAttributes{
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
			_, err := client.UpdateSubscriptionLocalizationV2(ctx, localizationID, asc.SubscriptionLocalizationV2UpdateAttributes{
				Name:        shared.VersionLocalizationValue(values, "name"),
				Description: shared.VersionLocalizationValue(values, "description"),
			})
			return err
		},
	}
}

func subscriptionGroupVersionLocalizationImportOps(client *asc.Client) shared.VersionLocalizationImportOps {
	return shared.VersionLocalizationImportOps{
		List: func(ctx context.Context, versionID string) ([]shared.VersionLocalization, error) {
			first, err := client.GetSubscriptionGroupVersionLocalizations(ctx, versionID, asc.Limit[asc.SubscriptionGroupVersionLocalizationsOption](200))
			if err != nil {
				return nil, err
			}
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetSubscriptionGroupVersionLocalizations(ctx, versionID, asc.WithSubscriptionGroupVersionLocalizationsNextURL(nextURL))
			})
			if err != nil {
				return nil, err
			}
			resp := all.(*asc.SubscriptionGroupLocalizationsV2Response)
			existing := make([]shared.VersionLocalization, 0, len(resp.Data))
			for _, item := range resp.Data {
				existing = append(existing, shared.VersionLocalization{
					ID:     item.ID,
					Locale: item.Attributes.Locale,
					Values: map[string]string{"name": item.Attributes.Name, "customAppName": item.Attributes.CustomAppName},
				})
			}
			return existing, nil
		},
		Create: func(ctx context.Context, versionID, locale string, values map[string]string) (string, error) {
			resp, err := client.CreateSubscriptionGroupLocalizationV2(ctx, versionID, asc.SubscriptionGroupLocalizationV2CreateAttributes{
				Name:          values["name"],
				Locale:        locale,
				CustomAppName: shared.VersionLocalizationValue(values, "customAppName"),
			})
			if err != nil {
				return "", err
			}
			return resp.Data.ID, nil
		},
		Update: func(ctx context.Context, localizationID string, values map[string]string) error {
			_, err := client.UpdateSubscriptionGroupLocalizationV2(ctx, localizationID, asc.SubscriptionGroupLocalizationV2UpdateAttributes{
				Name:          shared.VersionLocalizationValue(values, "name"),
				CustomAppName: shared.VersionLocalizationValue(values, "customAppName"),
			})
			return err
		},
	}
}
