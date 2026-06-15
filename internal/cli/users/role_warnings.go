package users

import (
	"fmt"
	"os"
	"strings"
)

const deprecatedAccessToReportsRole = "ACCESS_TO_REPORTS"

const deprecatedAccessToReportsWarning = "Warning: ACCESS_TO_REPORTS is deprecated in App Store Connect API 4.4; see UserRole documentation for alternatives."

func warnDeprecatedUserRoles(roles []string) {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), deprecatedAccessToReportsRole) {
			fmt.Fprintln(os.Stderr, deprecatedAccessToReportsWarning)
			return
		}
	}
}
