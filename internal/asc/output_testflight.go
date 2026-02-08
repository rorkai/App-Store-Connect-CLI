package asc

import (
	"fmt"
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

func betaAppReviewDetailsRows(resp *BetaAppReviewDetailsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printBetaAppReviewDetailsTable(resp *BetaAppReviewDetailsResponse) error {
	h, r := betaAppReviewDetailsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaAppReviewDetailTable(resp *BetaAppReviewDetailResponse) error {
	return printBetaAppReviewDetailsTable(&BetaAppReviewDetailsResponse{
		Data: []Resource[BetaAppReviewDetailAttributes]{resp.Data},
	})
}

func printBetaAppReviewDetailsMarkdown(resp *BetaAppReviewDetailsResponse) error {
	h, r := betaAppReviewDetailsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printBetaAppReviewDetailMarkdown(resp *BetaAppReviewDetailResponse) error {
	return printBetaAppReviewDetailsMarkdown(&BetaAppReviewDetailsResponse{
		Data: []Resource[BetaAppReviewDetailAttributes]{resp.Data},
	})
}

func betaAppReviewSubmissionsRows(resp *BetaAppReviewSubmissionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Submitted Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.BetaReviewState),
			compactWhitespace(item.Attributes.SubmittedDate),
		})
	}
	return headers, rows
}

func printBetaAppReviewSubmissionsTable(resp *BetaAppReviewSubmissionsResponse) error {
	h, r := betaAppReviewSubmissionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaAppReviewSubmissionTable(resp *BetaAppReviewSubmissionResponse) error {
	return printBetaAppReviewSubmissionsTable(&BetaAppReviewSubmissionsResponse{
		Data: []Resource[BetaAppReviewSubmissionAttributes]{resp.Data},
	})
}

func printBetaAppReviewSubmissionsMarkdown(resp *BetaAppReviewSubmissionsResponse) error {
	h, r := betaAppReviewSubmissionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printBetaAppReviewSubmissionMarkdown(resp *BetaAppReviewSubmissionResponse) error {
	return printBetaAppReviewSubmissionsMarkdown(&BetaAppReviewSubmissionsResponse{
		Data: []Resource[BetaAppReviewSubmissionAttributes]{resp.Data},
	})
}

func buildBetaDetailsRows(resp *BuildBetaDetailsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printBuildBetaDetailsTable(resp *BuildBetaDetailsResponse) error {
	h, r := buildBetaDetailsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBuildBetaDetailTable(resp *BuildBetaDetailResponse) error {
	return printBuildBetaDetailsTable(&BuildBetaDetailsResponse{
		Data: []Resource[BuildBetaDetailAttributes]{resp.Data},
	})
}

func printBuildBetaDetailsMarkdown(resp *BuildBetaDetailsResponse) error {
	h, r := buildBetaDetailsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printBuildBetaDetailMarkdown(resp *BuildBetaDetailResponse) error {
	return printBuildBetaDetailsMarkdown(&BuildBetaDetailsResponse{
		Data: []Resource[BuildBetaDetailAttributes]{resp.Data},
	})
}

func betaRecruitmentCriterionOptionsRows(resp *BetaRecruitmentCriterionOptionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Device Family OS Versions"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatDeviceFamilyOsVersions(item.Attributes.DeviceFamilyOsVersions)),
		})
	}
	return headers, rows
}

func printBetaRecruitmentCriterionOptionsTable(resp *BetaRecruitmentCriterionOptionsResponse) error {
	h, r := betaRecruitmentCriterionOptionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaRecruitmentCriterionOptionsMarkdown(resp *BetaRecruitmentCriterionOptionsResponse) error {
	h, r := betaRecruitmentCriterionOptionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaRecruitmentCriteriaRows(resp *BetaRecruitmentCriteriaResponse) ([]string, [][]string) {
	headers := []string{"ID", "Last Modified", "Filters"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(resp.Data.Attributes.LastModifiedDate),
		compactWhitespace(formatDeviceFamilyOsVersionFilters(resp.Data.Attributes.DeviceFamilyOsVersionFilters)),
	}}
	return headers, rows
}

func printBetaRecruitmentCriteriaTable(resp *BetaRecruitmentCriteriaResponse) error {
	h, r := betaRecruitmentCriteriaRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaRecruitmentCriteriaMarkdown(resp *BetaRecruitmentCriteriaResponse) error {
	h, r := betaRecruitmentCriteriaRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaRecruitmentCriteriaDeleteResultRows(result *BetaRecruitmentCriteriaDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaRecruitmentCriteriaDeleteResultTable(result *BetaRecruitmentCriteriaDeleteResult) error {
	h, r := betaRecruitmentCriteriaDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaRecruitmentCriteriaDeleteResultMarkdown(result *BetaRecruitmentCriteriaDeleteResult) error {
	h, r := betaRecruitmentCriteriaDeleteResultRows(result)
	RenderMarkdown(h, r)
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

func betaGroupMetricsRows(items []Resource[BetaGroupMetricAttributes]) ([]string, [][]string) {
	headers := []string{"ID", "Attributes"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(formatMetricAttributes(item.Attributes)),
		})
	}
	return headers, rows
}

func printBetaGroupMetricsTable(items []Resource[BetaGroupMetricAttributes]) error {
	h, r := betaGroupMetricsRows(items)
	RenderTable(h, r)
	return nil
}

func printBetaGroupMetricsMarkdown(items []Resource[BetaGroupMetricAttributes]) error {
	h, r := betaGroupMetricsRows(items)
	RenderMarkdown(h, r)
	return nil
}
