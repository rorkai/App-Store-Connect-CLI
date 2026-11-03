package users

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

func normalizeUsersFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, usersFieldsList(), flagName)
}

func normalizeUsersAppFields(value, flagName string) ([]string, error) {
	// Apple production rejects these on GET /v1/users with 400
	// "is not a valid field name" (Nov 2026), although the spec lists them.
	allowed := shared.AppFieldsList(
		"appStoreIcon",
		"performanceOverviews",
		"customerReviewSummarizations",
		"analyticsReportRequests",
		"betaFeedbackScreenshotSubmissions",
		"betaFeedbackCrashSubmissions",
		"webhooks",
		"androidToIosAppMappingDetails",
	)
	return shared.NormalizeSelection(value, allowed, flagName)
}

func normalizeUsersSort(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, usersSortList(), flagName)
}

func usersSortList() []string {
	return []string{"username", "-username", "lastName", "-lastName"}
}

func usersFieldsList() []string {
	return []string{
		"username",
		"firstName",
		"lastName",
		"roles",
		"allAppsVisible",
		"provisioningAllowed",
		"visibleApps",
	}
}
