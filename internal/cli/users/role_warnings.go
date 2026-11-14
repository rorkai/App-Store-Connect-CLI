package users

import (
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const deprecatedAccessToReportsRole = "ACCESS_TO_REPORTS"

const deprecatedAccessToReportsWarning = "Warning: ACCESS_TO_REPORTS is deprecated in App Store Connect API 4.4; see UserRole documentation for alternatives."

func warnDeprecatedUserRoles(d *shared.Deps, roles []string) {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), deprecatedAccessToReportsRole) {
			fmt.Fprintln(d.IO.ErrOut, deprecatedAccessToReportsWarning)
			return
		}
	}
}
