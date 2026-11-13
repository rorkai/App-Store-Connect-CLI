package cmdtest

import (
	"testing"
)

func TestTestFlightBetaNotificationsCreateValidationErrors(t *testing.T) {
	assertUsageExit(t, []string{"testflight", "notifications", "send"}, "Error: --build-id is required")
}
