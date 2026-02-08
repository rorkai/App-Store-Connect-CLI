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

func printAppCustomProductPagesTable(resp *AppCustomProductPagesResponse) error {
	h, r := appCustomProductPagesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppCustomProductPagesMarkdown(resp *AppCustomProductPagesResponse) error {
	h, r := appCustomProductPagesRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppCustomProductPageVersionsTable(resp *AppCustomProductPageVersionsResponse) error {
	h, r := appCustomProductPageVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppCustomProductPageVersionsMarkdown(resp *AppCustomProductPageVersionsResponse) error {
	h, r := appCustomProductPageVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppCustomProductPageLocalizationsTable(resp *AppCustomProductPageLocalizationsResponse) error {
	h, r := appCustomProductPageLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppCustomProductPageLocalizationsMarkdown(resp *AppCustomProductPageLocalizationsResponse) error {
	h, r := appCustomProductPageLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appKeywordsRows(resp *AppKeywordsResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func printAppKeywordsTable(resp *AppKeywordsResponse) error {
	h, r := appKeywordsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppKeywordsMarkdown(resp *AppKeywordsResponse) error {
	h, r := appKeywordsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppStoreVersionExperimentsTable(resp *AppStoreVersionExperimentsResponse) error {
	h, r := appStoreVersionExperimentsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentsMarkdown(resp *AppStoreVersionExperimentsResponse) error {
	h, r := appStoreVersionExperimentsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppStoreVersionExperimentsV2Table(resp *AppStoreVersionExperimentsV2Response) error {
	h, r := appStoreVersionExperimentsV2Rows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentsV2Markdown(resp *AppStoreVersionExperimentsV2Response) error {
	h, r := appStoreVersionExperimentsV2Rows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppStoreVersionExperimentTreatmentsTable(resp *AppStoreVersionExperimentTreatmentsResponse) error {
	h, r := appStoreVersionExperimentTreatmentsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentTreatmentsMarkdown(resp *AppStoreVersionExperimentTreatmentsResponse) error {
	h, r := appStoreVersionExperimentTreatmentsRows(resp)
	RenderMarkdown(h, r)
	return nil
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

func printAppStoreVersionExperimentTreatmentLocalizationsTable(resp *AppStoreVersionExperimentTreatmentLocalizationsResponse) error {
	h, r := appStoreVersionExperimentTreatmentLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationsMarkdown(resp *AppStoreVersionExperimentTreatmentLocalizationsResponse) error {
	h, r := appStoreVersionExperimentTreatmentLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appCustomProductPageDeleteResultRows(result *AppCustomProductPageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppCustomProductPageDeleteResultTable(result *AppCustomProductPageDeleteResult) error {
	h, r := appCustomProductPageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppCustomProductPageDeleteResultMarkdown(result *AppCustomProductPageDeleteResult) error {
	h, r := appCustomProductPageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appCustomProductPageLocalizationDeleteResultRows(result *AppCustomProductPageLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppCustomProductPageLocalizationDeleteResultTable(result *AppCustomProductPageLocalizationDeleteResult) error {
	h, r := appCustomProductPageLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppCustomProductPageLocalizationDeleteResultMarkdown(result *AppCustomProductPageLocalizationDeleteResult) error {
	h, r := appCustomProductPageLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appStoreVersionExperimentDeleteResultRows(result *AppStoreVersionExperimentDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppStoreVersionExperimentDeleteResultTable(result *AppStoreVersionExperimentDeleteResult) error {
	h, r := appStoreVersionExperimentDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentDeleteResultMarkdown(result *AppStoreVersionExperimentDeleteResult) error {
	h, r := appStoreVersionExperimentDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appStoreVersionExperimentTreatmentDeleteResultRows(result *AppStoreVersionExperimentTreatmentDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppStoreVersionExperimentTreatmentDeleteResultTable(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	h, r := appStoreVersionExperimentTreatmentDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentTreatmentDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	h, r := appStoreVersionExperimentTreatmentDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appStoreVersionExperimentTreatmentLocalizationDeleteResultRows(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultTable(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	h, r := appStoreVersionExperimentTreatmentLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	h, r := appStoreVersionExperimentTreatmentLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
