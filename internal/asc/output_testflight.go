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

func betaRecruitmentCriteriaRows(resp *BetaRecruitmentCriteriaResponse) ([]string, [][]string) {
	headers := []string{"ID", "Last Modified", "Filters"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(resp.Data.Attributes.LastModifiedDate),
		compactWhitespace(formatDeviceFamilyOsVersionFilters(resp.Data.Attributes.DeviceFamilyOsVersionFilters)),
	}}
	return headers, rows
}

func betaRecruitmentCriteriaDeleteResultRows(result *BetaRecruitmentCriteriaDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func betaGroupPublicLinkUsagesRows(resp *BetaGroupPublicLinkUsagesResponse) ([]string, [][]string) {
	headers := []string{"Start", "End", "Views", "Accepted", "Did Not Accept", "Did Not Meet Criteria", "Not Relevant", "Not Clear", "Not Interesting"}
	rows := make([][]string, 0, len(resp.Data))
	for _, metric := range resp.Data {
		for _, point := range metric.DataPoints {
			rows = append(rows, []string{
				point.Start,
				point.End,
				formatBetaGroupMetricCount(point.Values.ViewCount),
				formatBetaGroupMetricCount(point.Values.AcceptedCount),
				formatBetaGroupMetricCount(point.Values.DidNotAcceptCount),
				formatBetaGroupMetricCount(point.Values.DidNotMeetCriteriaCount),
				formatBetaGroupMetricRatio(point.Values.NotRelevantRatio),
				formatBetaGroupMetricRatio(point.Values.NotClearRatio),
				formatBetaGroupMetricRatio(point.Values.NotInterestingRatio),
			})
		}
	}
	return headers, rows
}

func betaGroupTesterUsagesRows(resp *BetaGroupTesterUsagesResponse) ([]string, [][]string) {
	headers := []string{"Tester ID", "Start", "End", "Sessions", "Crashes", "Feedback"}
	rows := make([][]string, 0, len(resp.Data))
	for _, metric := range resp.Data {
		testerID := ""
		if metric.Dimensions != nil && metric.Dimensions.BetaTesters != nil && metric.Dimensions.BetaTesters.Data != nil {
			testerID = metric.Dimensions.BetaTesters.Data.ID
		}
		for _, point := range metric.DataPoints {
			rows = append(rows, []string{
				testerID,
				point.Start,
				point.End,
				formatBetaGroupMetricCount(point.Values.SessionCount),
				formatBetaGroupMetricCount(point.Values.CrashCount),
				formatBetaGroupMetricCount(point.Values.FeedbackCount),
			})
		}
	}
	return headers, rows
}

func formatBetaGroupMetricCount(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func formatBetaGroupMetricRatio(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%g", *value)
}

func betaRecruitmentCompatibleBuildCheckRows(resp *BetaRecruitmentCriterionCompatibleBuildCheckResponse) ([]string, [][]string) {
	return []string{"ID", "Has Compatible Build"}, [][]string{{resp.Data.ID, fmt.Sprintf("%t", resp.Data.Attributes.HasCompatibleBuild)}}
}
