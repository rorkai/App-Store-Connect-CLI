package asc

import (
	"fmt"
	"strings"
)

// WebAppComplianceRequirement is one App Compliance requirement from
// `asc web apps compliance list`.
type WebAppComplianceRequirement struct {
	AppID              string   `json:"appId"`
	RequirementID      string   `json:"requirementId"`
	RequirementName    string   `json:"requirementName"`
	FormName           string   `json:"formName"`
	Status             string   `json:"status,omitempty"`
	Required           bool     `json:"required"`
	CountriesOrRegions []string `json:"countriesOrRegions,omitempty"`
	FormID             string   `json:"formId,omitempty"`
	DateSigned         string   `json:"dateSigned,omitempty"`
	LastUpdatedDate    string   `json:"lastUpdatedDate,omitempty"`
	RequiredActions    []string `json:"requiredActions,omitempty"`
}

// WebAppComplianceRequirementList is the computed listing for
// `asc web apps compliance list`. JSON is a bare array.
type WebAppComplianceRequirementList []WebAppComplianceRequirement

func webAppComplianceRequirementListRows(result *WebAppComplianceRequirementList) ([]string, [][]string) {
	headers := []string{"Requirement", "Countries/Regions", "Status", "Required", "Last Updated", "Required Actions", "Requirement ID"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(*result))
	for _, requirement := range *result {
		rows = append(rows, []string{
			requirement.RequirementName,
			webDeclarationText(strings.Join(requirement.CountriesOrRegions, ",")),
			webDeclarationText(requirement.Status),
			fmt.Sprintf("%t", requirement.Required),
			webDeclarationText(requirement.LastUpdatedDate),
			webDeclarationText(strings.Join(requirement.RequiredActions, ",")),
			webDeclarationText(requirement.RequirementID),
		})
	}
	return headers, rows
}
