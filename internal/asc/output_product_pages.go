package asc

import (
	"fmt"
	"os"
)

func printAppCustomProductPagesTable(resp *AppCustomProductPagesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppCustomProductPagesMarkdown(resp *AppCustomProductPagesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Visible | URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(boolValue(item.Attributes.Visible)),
			escapeMarkdown(item.Attributes.URL),
		)
	}
	return nil
}

func printAppCustomProductPageVersionsTable(resp *AppCustomProductPageVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppCustomProductPageVersionsMarkdown(resp *AppCustomProductPageVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State | Deep Link |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.State),
			escapeMarkdown(item.Attributes.DeepLink),
		)
	}
	return nil
}

func printAppCustomProductPageLocalizationsTable(resp *AppCustomProductPageLocalizationsResponse) error {
	headers := []string{"ID", "Locale", "Promotional Text"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Locale),
			compactWhitespace(item.Attributes.PromotionalText),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppCustomProductPageLocalizationsMarkdown(resp *AppCustomProductPageLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Promotional Text |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.PromotionalText),
		)
	}
	return nil
}

func printAppKeywordsTable(resp *AppKeywordsResponse) error {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppKeywordsMarkdown(resp *AppKeywordsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(item.ID))
	}
	return nil
}

func printAppStoreVersionExperimentsTable(resp *AppStoreVersionExperimentsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentsMarkdown(resp *AppStoreVersionExperimentsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Traffic Proportion | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(formatOptionalInt(item.Attributes.TrafficProportion)),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printAppStoreVersionExperimentsV2Table(resp *AppStoreVersionExperimentsV2Response) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentsV2Markdown(resp *AppStoreVersionExperimentsV2Response) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Platform | Traffic Proportion | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(formatOptionalInt(item.Attributes.TrafficProportion)),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printAppStoreVersionExperimentTreatmentsTable(resp *AppStoreVersionExperimentTreatmentsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentTreatmentsMarkdown(resp *AppStoreVersionExperimentTreatmentsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | App Icon Name | Promoted Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.AppIconName),
			escapeMarkdown(item.Attributes.PromotedDate),
		)
	}
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationsTable(resp *AppStoreVersionExperimentTreatmentLocalizationsResponse) error {
	headers := []string{"ID", "Locale"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Locale),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationsMarkdown(resp *AppStoreVersionExperimentTreatmentLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
		)
	}
	return nil
}

func printAppCustomProductPageDeleteResultTable(result *AppCustomProductPageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppCustomProductPageDeleteResultMarkdown(result *AppCustomProductPageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppCustomProductPageLocalizationDeleteResultTable(result *AppCustomProductPageLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppCustomProductPageLocalizationDeleteResultMarkdown(result *AppCustomProductPageLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentDeleteResultTable(result *AppStoreVersionExperimentDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentDeleteResultMarkdown(result *AppStoreVersionExperimentDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentTreatmentDeleteResultTable(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentTreatmentDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultTable(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
