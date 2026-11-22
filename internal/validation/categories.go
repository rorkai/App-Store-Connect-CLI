package validation

import "strings"

func categoryChecks(appInfoID string, primaryCategoryID string) []CheckResult {
	if strings.TrimSpace(primaryCategoryID) != "" {
		return nil
	}
	return []CheckResult{
		{
			ID:           "categories.primary_missing",
			Severity:     SeverityError,
			Field:        "primaryCategory",
			ResourceType: "appInfo",
			ResourceID:   strings.TrimSpace(appInfoID),
			Message:      "primary category is not set",
			Remediation:  "Set a primary category (list category IDs with asc categories list)",
		},
	}
}
