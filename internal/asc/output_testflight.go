package asc

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func formatBetaReviewContactName(attr BetaAppReviewDetailAttributes) string {
	first := strings.TrimSpace(attr.ContactFirstName)
	last := strings.TrimSpace(attr.ContactLastName)
	switch {
	case first == "" && last == "":
		return ""
	case first == "":
		return last
	case last == "":
		return first
	default:
		return first + " " + last
	}
}

func printBetaAppReviewDetailsTable(resp *BetaAppReviewDetailsResponse) error {
	headers := []string{"ID", "Contact", "Email", "Phone", "Demo Required", "Demo Account", "Notes"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatBetaReviewContactName(item.Attributes)),
			compactWhitespace(item.Attributes.ContactEmail),
			compactWhitespace(item.Attributes.ContactPhone),
			fmt.Sprintf("%t", item.Attributes.DemoAccountRequired),
			compactWhitespace(item.Attributes.DemoAccountName),
			compactWhitespace(item.Attributes.Notes),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppReviewDetailTable(resp *BetaAppReviewDetailResponse) error {
	return printBetaAppReviewDetailsTable(&BetaAppReviewDetailsResponse{
		Data: []Resource[BetaAppReviewDetailAttributes]{resp.Data},
	})
}

func printBetaAppReviewDetailsMarkdown(resp *BetaAppReviewDetailsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Contact | Email | Phone | Demo Required | Demo Account | Notes |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(formatBetaReviewContactName(item.Attributes)),
			escapeMarkdown(item.Attributes.ContactEmail),
			escapeMarkdown(item.Attributes.ContactPhone),
			item.Attributes.DemoAccountRequired,
			escapeMarkdown(item.Attributes.DemoAccountName),
			escapeMarkdown(item.Attributes.Notes),
		)
	}
	return nil
}

func printBetaAppReviewDetailMarkdown(resp *BetaAppReviewDetailResponse) error {
	return printBetaAppReviewDetailsMarkdown(&BetaAppReviewDetailsResponse{
		Data: []Resource[BetaAppReviewDetailAttributes]{resp.Data},
	})
}

func printBetaAppReviewSubmissionsTable(resp *BetaAppReviewSubmissionsResponse) error {
	headers := []string{"ID", "State", "Submitted Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.BetaReviewState),
			compactWhitespace(item.Attributes.SubmittedDate),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppReviewSubmissionTable(resp *BetaAppReviewSubmissionResponse) error {
	return printBetaAppReviewSubmissionsTable(&BetaAppReviewSubmissionsResponse{
		Data: []Resource[BetaAppReviewSubmissionAttributes]{resp.Data},
	})
}

func printBetaAppReviewSubmissionsMarkdown(resp *BetaAppReviewSubmissionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Submitted Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.BetaReviewState),
			escapeMarkdown(item.Attributes.SubmittedDate),
		)
	}
	return nil
}

func printBetaAppReviewSubmissionMarkdown(resp *BetaAppReviewSubmissionResponse) error {
	return printBetaAppReviewSubmissionsMarkdown(&BetaAppReviewSubmissionsResponse{
		Data: []Resource[BetaAppReviewSubmissionAttributes]{resp.Data},
	})
}

func printBuildBetaDetailsTable(resp *BuildBetaDetailsResponse) error {
	headers := []string{"ID", "Auto Notify", "Internal State", "External State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.AutoNotifyEnabled),
			compactWhitespace(item.Attributes.InternalBuildState),
			compactWhitespace(item.Attributes.ExternalBuildState),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBuildBetaDetailTable(resp *BuildBetaDetailResponse) error {
	return printBuildBetaDetailsTable(&BuildBetaDetailsResponse{
		Data: []Resource[BuildBetaDetailAttributes]{resp.Data},
	})
}

func printBuildBetaDetailsMarkdown(resp *BuildBetaDetailsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Auto Notify | Internal State | External State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t | %s | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.AutoNotifyEnabled,
			escapeMarkdown(item.Attributes.InternalBuildState),
			escapeMarkdown(item.Attributes.ExternalBuildState),
		)
	}
	return nil
}

func printBuildBetaDetailMarkdown(resp *BuildBetaDetailResponse) error {
	return printBuildBetaDetailsMarkdown(&BuildBetaDetailsResponse{
		Data: []Resource[BuildBetaDetailAttributes]{resp.Data},
	})
}

func printBetaRecruitmentCriterionOptionsTable(resp *BetaRecruitmentCriterionOptionsResponse) error {
	headers := []string{"ID", "Device Family OS Versions"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatDeviceFamilyOsVersions(item.Attributes.DeviceFamilyOsVersions)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaRecruitmentCriterionOptionsMarkdown(resp *BetaRecruitmentCriterionOptionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Device Family OS Versions |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(formatDeviceFamilyOsVersions(item.Attributes.DeviceFamilyOsVersions)),
		)
	}
	return nil
}

func printBetaRecruitmentCriteriaTable(resp *BetaRecruitmentCriteriaResponse) error {
	headers := []string{"ID", "Last Modified", "Filters"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(resp.Data.Attributes.LastModifiedDate),
		compactWhitespace(formatDeviceFamilyOsVersionFilters(resp.Data.Attributes.DeviceFamilyOsVersionFilters)),
	}}
	RenderTable(headers, rows)
	return nil
}

func printBetaRecruitmentCriteriaMarkdown(resp *BetaRecruitmentCriteriaResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Last Modified | Filters |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.LastModifiedDate),
		escapeMarkdown(formatDeviceFamilyOsVersionFilters(resp.Data.Attributes.DeviceFamilyOsVersionFilters)),
	)
	return nil
}

func printBetaRecruitmentCriteriaDeleteResultTable(result *BetaRecruitmentCriteriaDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaRecruitmentCriteriaDeleteResultMarkdown(result *BetaRecruitmentCriteriaDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func formatDeviceFamilyOsVersions(items []BetaRecruitmentCriterionOptionDeviceFamily) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		family := string(item.DeviceFamily)
		versions := strings.Join(item.OSVersions, ",")
		if versions == "" {
			parts = append(parts, family)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", family, versions))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func formatDeviceFamilyOsVersionFilters(filters []DeviceFamilyOsVersionFilter) string {
	if len(filters) == 0 {
		return ""
	}
	parts := make([]string, 0, len(filters))
	for _, filter := range filters {
		family := string(filter.DeviceFamily)
		min := strings.TrimSpace(filter.MinimumOsInclusive)
		max := strings.TrimSpace(filter.MaximumOsInclusive)
		switch {
		case min != "" && max != "":
			parts = append(parts, fmt.Sprintf("%s:%s-%s", family, min, max))
		case min != "":
			parts = append(parts, fmt.Sprintf("%s:%s+", family, min))
		case max != "":
			parts = append(parts, fmt.Sprintf("%s:<=%s", family, max))
		default:
			parts = append(parts, family)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func formatMetricAttributes(attrs BetaGroupMetricAttributes) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, attrs[key]))
	}
	return strings.Join(parts, ", ")
}

func printBetaGroupMetricsTable(items []Resource[BetaGroupMetricAttributes]) error {
	headers := []string{"ID", "Attributes"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatMetricAttributes(item.Attributes)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaGroupMetricsMarkdown(items []Resource[BetaGroupMetricAttributes]) error {
	fmt.Fprintln(os.Stdout, "| ID | Attributes |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range items {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(formatMetricAttributes(item.Attributes)),
		)
	}
	return nil
}
