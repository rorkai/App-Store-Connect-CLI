package users

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func userRoleList() []string {
	return []string{
		"ADMIN",
		"FINANCE",
		"ACCOUNT_HOLDER",
		"SALES",
		"MARKETING",
		"APP_MANAGER",
		"DEVELOPER",
		"ACCESS_TO_REPORTS",
		"CUSTOMER_SUPPORT",
		"CREATE_APPS",
		"CLOUD_MANAGED_DEVELOPER_ID",
		"CLOUD_MANAGED_APP_DISTRIBUTION",
		"GENERATE_INDIVIDUAL_KEYS",
	}
}

func normalizeUserRoles(value string, flagName string) ([]string, error) {
	roles := shared.SplitCSVUpper(value)
	allowed := userRoleList()
	for _, role := range roles {
		if !slices.Contains(allowed, role) {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
	}
	return roles, nil
}

// rolePermitsProvisioning reports whether App Store Connect lets a user with
// role opt into provisioning access.
func rolePermitsProvisioning(role string) bool {
	return role == "APP_MANAGER" || role == "DEVELOPER"
}
