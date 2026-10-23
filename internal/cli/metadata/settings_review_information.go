package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var reviewInformationFields = []metadataSettingField{
	{name: "contactFirstName"},
	{name: "contactLastName"},
	{name: "contactPhone"},
	{name: "contactEmail"},
	{name: "demoAccountRequired", kind: metadataSettingBool},
	{name: "demoAccountName"},
	{name: "demoAccountPassword", secretEnv: "ASC_DEMO_ACCOUNT_PASSWORD"},
	{name: "notes"},
}

var reviewInformationSettingScope = metadataSettingScope{
	name:     "review-information",
	fileName: "review-information.json",
	fields:   reviewInformationFields,
	fetch:    fetchReviewInformationSetting,
	update:   updateReviewInformationSetting,
}

func fetchReviewDetail(ctx context.Context, client *asc.Client, versionID string) (*asc.AppStoreReviewDetailResponse, error) {
	resp, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.AppStoreReviewDetailResponse, error) {
		return client.GetAppStoreReviewDetailForVersion(requestCtx, versionID)
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*asc.APIError](err); asc.IsNotFound(err) || (ok && apiErr.StatusCode == http.StatusNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}

func fetchReviewInformationSetting(ctx context.Context, client *asc.Client, target metadataSettingTarget) (map[string]string, error) {
	resp, err := fetchReviewDetail(ctx, client, target.versionID)
	if err != nil || resp == nil {
		return map[string]string{}, err
	}
	attrs := resp.Data.Attributes
	values := map[string]string{"demoAccountRequired": strconv.FormatBool(attrs.DemoAccountRequired)}
	for name, value := range map[string]string{
		"contactFirstName": attrs.ContactFirstName,
		"contactLastName":  attrs.ContactLastName,
		"contactPhone":     attrs.ContactPhone,
		"contactEmail":     attrs.ContactEmail,
		"demoAccountName":  attrs.DemoAccountName,
		"notes":            attrs.Notes,
	} {
		if value = strings.TrimSpace(value); value != "" {
			values[name] = value
		}
	}
	if strings.TrimSpace(attrs.DemoAccountPassword) != "" {
		values["demoAccountPassword"] = attrs.DemoAccountPassword
	}
	return values, nil
}

func updateReviewInformationSetting(ctx context.Context, client *asc.Client, target metadataSettingTarget, changed map[string]string) error {
	existing, err := fetchReviewDetail(ctx, client, target.versionID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(metadataSettingObject(reviewInformationFields, changed))
	if err != nil {
		return err
	}
	if existing == nil {
		var attrs asc.AppStoreReviewDetailCreateAttributes
		if err := json.Unmarshal(payload, &attrs); err != nil {
			return err
		}
		_, err = client.CreateAppStoreReviewDetail(ctx, target.versionID, &attrs)
		return err
	}
	var attrs asc.AppStoreReviewDetailUpdateAttributes
	if err := json.Unmarshal(payload, &attrs); err != nil {
		return err
	}
	_, err = client.UpdateAppStoreReviewDetail(ctx, existing.Data.ID, attrs)
	return err
}
