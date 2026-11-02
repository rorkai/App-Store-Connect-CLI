package users

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

func normalizeUsersFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, usersFieldsList(), flagName)
}

func normalizeUsersAppFields(value, flagName string) ([]string, error) {
	return shared.NormalizeSelection(value, shared.AppFieldsList(), flagName)
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
