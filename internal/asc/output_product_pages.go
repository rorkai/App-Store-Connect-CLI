package asc

import (
	"fmt"
)

func appCustomProductPagesRows(resp *AppCustomProductPagesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Visible", "URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			boolValue(item.Attributes.Visible),
			compactWhitespace(item.Attributes.URL),
		})
	}
	return headers, rows
}

func appCustomProductPageVersionsRows(resp *AppCustomProductPageVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State", "Deep Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Version),
			compactWhitespace(item.Attributes.State),
			compactWhitespace(item.Attributes.DeepLink),
		})
	}
	return headers, rows
}

func appCustomProductPageLocalizationsRows(resp *AppCustomProductPageLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Promotional Text"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Locale),
			compactWhitespace(item.Attributes.PromotionalText),
		})
	}
	return headers, rows
}

func appKeywordsRows(resp *AppKeywordsResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func appStoreVersionExperimentsRows(resp *AppStoreVersionExperimentsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Traffic Proportion", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			formatOptionalInt(item.Attributes.TrafficProportion),
			compactWhitespace(item.Attributes.State),
		})
	}
	return headers, rows
}

func appStoreVersionExperimentsV2Rows(resp *AppStoreVersionExperimentsV2Response) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Platform", "Traffic Proportion", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			string(item.Attributes.Platform),
			formatOptionalInt(item.Attributes.TrafficProportion),
			compactWhitespace(item.Attributes.State),
		})
	}
	return headers, rows
}

func appStoreVersionExperimentTreatmentsRows(resp *AppStoreVersionExperimentTreatmentsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "App Icon Name", "Promoted Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.AppIconName),
			compactWhitespace(item.Attributes.PromotedDate),
		})
	}
	return headers, rows
}

func appStoreVersionExperimentTreatmentLocalizationsRows(resp *AppStoreVersionExperimentTreatmentLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Locale),
		})
	}
	return headers, rows
}

func appCustomProductPageDeleteResultRows(result *AppCustomProductPageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appCustomProductPageLocalizationDeleteResultRows(result *AppCustomProductPageLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appStoreVersionExperimentDeleteResultRows(result *AppStoreVersionExperimentDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appStoreVersionExperimentTreatmentDeleteResultRows(result *AppStoreVersionExperimentTreatmentDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appStoreVersionExperimentTreatmentLocalizationDeleteResultRows(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
