package asc

import (
	"fmt"
	"strings"
)

func appClipsRows(resp *AppClipsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Bundle ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.BundleID})
	}
	return headers, rows
}

func appClipDefaultExperiencesRows(resp *AppClipDefaultExperiencesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Action"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, string(item.Attributes.Action)})
	}
	return headers, rows
}

func appClipDefaultExperienceLocalizationsRows(resp *AppClipDefaultExperienceLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Subtitle"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Subtitle),
		})
	}
	return headers, rows
}

func appClipAdvancedExperiencesRows(resp *AppClipAdvancedExperiencesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Action", "Status", "Business Category", "Default Language", "Powered By", "Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Action),
			item.Attributes.Status,
			string(item.Attributes.BusinessCategory),
			string(item.Attributes.DefaultLanguage),
			fmt.Sprintf("%t", item.Attributes.IsPoweredBy),
			item.Attributes.Link,
		})
	}
	return headers, rows
}

func betaAppClipInvocationLocalizationsRows(resp *BetaAppClipInvocationLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Title"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Title),
		})
	}
	return headers, rows
}

func appClipAdvancedExperienceImageUploadResultRows(result *AppClipAdvancedExperienceImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Experience ID", "File Name", "File Size", "State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.ExperienceID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func appClipAdvancedExperienceImageRows(resp *AppClipAdvancedExperienceImageResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		state,
	}}
	return headers, rows
}

func appClipHeaderImageUploadResultRows(result *AppClipHeaderImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func appClipHeaderImageRows(resp *AppClipHeaderImageResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		state,
	}}
	return headers, rows
}

func appClipDefaultExperienceDeleteResultRows(result *AppClipDefaultExperienceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appClipDefaultExperienceLocalizationDeleteResultRows(result *AppClipDefaultExperienceLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appClipAdvancedExperienceDeleteResultRows(result *AppClipAdvancedExperienceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appClipAdvancedExperienceImageDeleteResultRows(result *AppClipAdvancedExperienceImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appClipHeaderImageDeleteResultRows(result *AppClipHeaderImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func betaAppClipInvocationDeleteResultRows(result *BetaAppClipInvocationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func betaAppClipInvocationLocalizationDeleteResultRows(result *BetaAppClipInvocationLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appClipAppStoreReviewDetailRows(resp *AppClipAppStoreReviewDetailResponse) ([]string, [][]string) {
	headers := []string{"ID", "Invocation URLs"}
	urls := strings.Join(resp.Data.Attributes.InvocationURLs, ", ")
	rows := [][]string{{resp.Data.ID, compactWhitespace(urls)}}
	return headers, rows
}
