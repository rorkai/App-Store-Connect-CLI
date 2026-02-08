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

func printAppClipsTable(resp *AppClipsResponse) error {
	h, r := appClipsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipsMarkdown(resp *AppClipsResponse) error {
	h, r := appClipsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appClipDefaultExperiencesRows(resp *AppClipDefaultExperiencesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Action"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, string(item.Attributes.Action)})
	}
	return headers, rows
}

func printAppClipDefaultExperiencesTable(resp *AppClipDefaultExperiencesResponse) error {
	h, r := appClipDefaultExperiencesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipDefaultExperiencesMarkdown(resp *AppClipDefaultExperiencesResponse) error {
	h, r := appClipDefaultExperiencesRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipDefaultExperienceLocalizationsTable(resp *AppClipDefaultExperienceLocalizationsResponse) error {
	h, r := appClipDefaultExperienceLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipDefaultExperienceLocalizationsMarkdown(resp *AppClipDefaultExperienceLocalizationsResponse) error {
	h, r := appClipDefaultExperienceLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipAdvancedExperiencesTable(resp *AppClipAdvancedExperiencesResponse) error {
	h, r := appClipAdvancedExperiencesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipAdvancedExperiencesMarkdown(resp *AppClipAdvancedExperiencesResponse) error {
	h, r := appClipAdvancedExperiencesRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printBetaAppClipInvocationLocalizationsTable(resp *BetaAppClipInvocationLocalizationsResponse) error {
	h, r := betaAppClipInvocationLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaAppClipInvocationLocalizationsMarkdown(resp *BetaAppClipInvocationLocalizationsResponse) error {
	h, r := betaAppClipInvocationLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipAdvancedExperienceImageUploadResultTable(result *AppClipAdvancedExperienceImageUploadResult) error {
	h, r := appClipAdvancedExperienceImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipAdvancedExperienceImageUploadResultMarkdown(result *AppClipAdvancedExperienceImageUploadResult) error {
	h, r := appClipAdvancedExperienceImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipAdvancedExperienceImageTable(resp *AppClipAdvancedExperienceImageResponse) error {
	h, r := appClipAdvancedExperienceImageRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipAdvancedExperienceImageMarkdown(resp *AppClipAdvancedExperienceImageResponse) error {
	h, r := appClipAdvancedExperienceImageRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipHeaderImageUploadResultTable(result *AppClipHeaderImageUploadResult) error {
	h, r := appClipHeaderImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipHeaderImageUploadResultMarkdown(result *AppClipHeaderImageUploadResult) error {
	h, r := appClipHeaderImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
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

func printAppClipHeaderImageTable(resp *AppClipHeaderImageResponse) error {
	h, r := appClipHeaderImageRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipHeaderImageMarkdown(resp *AppClipHeaderImageResponse) error {
	h, r := appClipHeaderImageRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appClipDefaultExperienceDeleteResultRows(result *AppClipDefaultExperienceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppClipDefaultExperienceDeleteResultTable(result *AppClipDefaultExperienceDeleteResult) error {
	h, r := appClipDefaultExperienceDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipDefaultExperienceDeleteResultMarkdown(result *AppClipDefaultExperienceDeleteResult) error {
	h, r := appClipDefaultExperienceDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appClipDefaultExperienceLocalizationDeleteResultRows(result *AppClipDefaultExperienceLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppClipDefaultExperienceLocalizationDeleteResultTable(result *AppClipDefaultExperienceLocalizationDeleteResult) error {
	h, r := appClipDefaultExperienceLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipDefaultExperienceLocalizationDeleteResultMarkdown(result *AppClipDefaultExperienceLocalizationDeleteResult) error {
	h, r := appClipDefaultExperienceLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appClipAdvancedExperienceDeleteResultRows(result *AppClipAdvancedExperienceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppClipAdvancedExperienceDeleteResultTable(result *AppClipAdvancedExperienceDeleteResult) error {
	h, r := appClipAdvancedExperienceDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipAdvancedExperienceDeleteResultMarkdown(result *AppClipAdvancedExperienceDeleteResult) error {
	h, r := appClipAdvancedExperienceDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appClipAdvancedExperienceImageDeleteResultRows(result *AppClipAdvancedExperienceImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppClipAdvancedExperienceImageDeleteResultTable(result *AppClipAdvancedExperienceImageDeleteResult) error {
	h, r := appClipAdvancedExperienceImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipAdvancedExperienceImageDeleteResultMarkdown(result *AppClipAdvancedExperienceImageDeleteResult) error {
	h, r := appClipAdvancedExperienceImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appClipHeaderImageDeleteResultRows(result *AppClipHeaderImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppClipHeaderImageDeleteResultTable(result *AppClipHeaderImageDeleteResult) error {
	h, r := appClipHeaderImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppClipHeaderImageDeleteResultMarkdown(result *AppClipHeaderImageDeleteResult) error {
	h, r := appClipHeaderImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaAppClipInvocationDeleteResultRows(result *BetaAppClipInvocationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaAppClipInvocationDeleteResultTable(result *BetaAppClipInvocationDeleteResult) error {
	h, r := betaAppClipInvocationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaAppClipInvocationDeleteResultMarkdown(result *BetaAppClipInvocationDeleteResult) error {
	h, r := betaAppClipInvocationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaAppClipInvocationLocalizationDeleteResultRows(result *BetaAppClipInvocationLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaAppClipInvocationLocalizationDeleteResultTable(result *BetaAppClipInvocationLocalizationDeleteResult) error {
	h, r := betaAppClipInvocationLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaAppClipInvocationLocalizationDeleteResultMarkdown(result *BetaAppClipInvocationLocalizationDeleteResult) error {
	h, r := betaAppClipInvocationLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appClipAppStoreReviewDetailRows(resp *AppClipAppStoreReviewDetailResponse) ([]string, [][]string) {
	headers := []string{"ID", "Invocation URLs"}
	urls := strings.Join(resp.Data.Attributes.InvocationURLs, ", ")
	rows := [][]string{{resp.Data.ID, compactWhitespace(urls)}}
	return headers, rows
}

func printAppClipAppStoreReviewDetailTable(resp *AppClipAppStoreReviewDetailResponse) error {
	h, r := appClipAppStoreReviewDetailRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppClipAppStoreReviewDetailMarkdown(resp *AppClipAppStoreReviewDetailResponse) error {
	h, r := appClipAppStoreReviewDetailRows(resp)
	RenderMarkdown(h, r)
	return nil
}
