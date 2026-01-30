package asc

import (
	"fmt"
	"os"
	"text/tabwriter"
)

func printAppCustomProductPagesTable(resp *AppCustomProductPagesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tVisible\tURL")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			boolValue(item.Attributes.Visible),
			compactWhitespace(item.Attributes.URL),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tVersion\tState\tDeep Link")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Version),
			compactWhitespace(item.Attributes.State),
			compactWhitespace(item.Attributes.DeepLink),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLocale\tPromotional Text")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Locale),
			compactWhitespace(item.Attributes.PromotionalText),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\n", item.ID)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tTraffic Proportion\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			formatOptionalInt(item.Attributes.TrafficProportion),
			compactWhitespace(item.Attributes.State),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tPlatform\tTraffic Proportion\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			string(item.Attributes.Platform),
			formatOptionalInt(item.Attributes.TrafficProportion),
			compactWhitespace(item.Attributes.State),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tApp Icon Name\tPromoted Date")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.AppIconName),
			compactWhitespace(item.Attributes.PromotedDate),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLocale")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Locale),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printAppCustomProductPageDeleteResultMarkdown(result *AppCustomProductPageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppCustomProductPageLocalizationDeleteResultTable(result *AppCustomProductPageLocalizationDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printAppCustomProductPageLocalizationDeleteResultMarkdown(result *AppCustomProductPageLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentDeleteResultTable(result *AppStoreVersionExperimentDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printAppStoreVersionExperimentDeleteResultMarkdown(result *AppStoreVersionExperimentDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentTreatmentDeleteResultTable(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printAppStoreVersionExperimentTreatmentDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultTable(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printAppStoreVersionExperimentTreatmentLocalizationDeleteResultMarkdown(result *AppStoreVersionExperimentTreatmentLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
