package metadata

import (
	"context"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type categoryRelationshipReader func(ctx context.Context, client *asc.Client, appInfoID string) (string, error)

var categoryRelationshipReaders = map[string]categoryRelationshipReader{
	"primaryCategory": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoPrimaryCategoryRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	"primarySubcategoryOne": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoPrimarySubcategoryOneRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	"primarySubcategoryTwo": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoPrimarySubcategoryTwoRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	"secondaryCategory": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoSecondaryCategoryRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	"secondarySubcategoryOne": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoSecondarySubcategoryOneRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	"secondarySubcategoryTwo": func(ctx context.Context, client *asc.Client, appInfoID string) (string, error) {
		resp, err := client.GetAppInfoSecondarySubcategoryTwoRelationship(ctx, appInfoID)
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
}

var categoryFields = []metadataSettingField{
	{name: "primaryCategory"},
	{name: "primarySubcategoryOne"},
	{name: "primarySubcategoryTwo"},
	{name: "secondaryCategory"},
	{name: "secondarySubcategoryOne"},
	{name: "secondarySubcategoryTwo"},
}

var categoriesSettingScope = metadataSettingScope{
	name:         "categories",
	fileName:     "categories.json",
	fields:       categoryFields,
	needsAppInfo: true,
	fetch:        fetchAppInfoCategories,
	update:       updateAppInfoCategories,
}

func fetchAppInfoCategories(ctx context.Context, client *asc.Client, target metadataSettingTarget) (map[string]string, error) {
	values := map[string]string{}
	for _, field := range categoryFields {
		read := categoryRelationshipReaders[field.name]
		id, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
			id, err := read(requestCtx, client, target.appInfoID)
			if asc.IsNotFound(err) {
				return "", nil
			}
			return id, err
		})
		if err != nil {
			return nil, err
		}
		if id = strings.TrimSpace(id); id != "" {
			values[field.name] = id
		}
	}
	return values, nil
}

func updateAppInfoCategories(ctx context.Context, client *asc.Client, target metadataSettingTarget, changed map[string]string) error {
	_, err := client.UpdateAppInfoCategories(
		ctx,
		target.appInfoID,
		changed["primaryCategory"],
		changed["secondaryCategory"],
		changed["primarySubcategoryOne"],
		changed["primarySubcategoryTwo"],
		changed["secondarySubcategoryOne"],
		changed["secondarySubcategoryTwo"],
	)
	return err
}
