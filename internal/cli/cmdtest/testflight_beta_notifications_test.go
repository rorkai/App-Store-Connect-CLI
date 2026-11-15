package cmdtest

import (
	"testing"
)

func TestTestFlightBetaNotificationsCreateValidationErrors(t *testing.T) {
	assertUsageExitWithDeps(t, nil, []string{"testflight", "notifications", "send"}, "Error: --build-id is required")
}
