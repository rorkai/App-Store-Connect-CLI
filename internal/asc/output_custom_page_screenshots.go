package asc

import "strconv"

// CustomPageScreenshotPlan describes a read-only screenshot target plan.
type CustomPageScreenshotPlan struct {
	AppID                string                          `json:"appId"`
	CustomPageID         string                          `json:"customPageId"`
	VersionID            string                          `json:"versionId"`
	VersionState         string                          `json:"versionState"`
	Path                 string                          `json:"path"`
	ReadOnly             bool                            `json:"readOnly"`
	Ready                bool                            `json:"ready"`
	Groups               []CustomPageScreenshotPlanGroup `json:"groups"`
	MissingLocalizations []string                        `json:"missingLocalizations"`
	SkippedFiles         []string                        `json:"skippedFiles"`
}

// CustomPageScreenshotPlanGroup resolves one locale and display type.
type CustomPageScreenshotPlanGroup struct {
	Locale         string   `json:"locale"`
	DisplayType    string   `json:"displayType"`
	LocalizationID string   `json:"localizationId"`
	Status         string   `json:"status"`
	Files          []string `json:"files"`
}

func customPageScreenshotPlanRows(plan *CustomPageScreenshotPlan) ([]string, [][]string) {
	rows := make([][]string, 0, len(plan.Groups))
	for _, group := range plan.Groups {
		rows = append(rows, []string{group.Locale, group.DisplayType, group.LocalizationID, strconv.Itoa(len(group.Files)), group.Status})
	}
	return []string{"Locale", "Display Type", "Localization ID", "Files", "Status"}, rows
}
