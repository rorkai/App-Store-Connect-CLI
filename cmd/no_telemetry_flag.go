package cmd

import (
	"strconv"
	"strings"
)

const noTelemetryFlagName = "no-telemetry"

// noTelemetryRequested reports whether args turn telemetry off for this
// invocation. Any `--no-telemetry` before `--` counts, wherever it sits, so a
// misplaced flag that fails to parse still sends nothing.
func noTelemetryRequested(args []string) bool {
	requested := false
	for _, token := range args {
		if token == "--" {
			break
		}
		if !strings.HasPrefix(token, "-") {
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(token, "-"), "=")
		if name != noTelemetryFlagName {
			continue
		}
		if !hasValue {
			requested = true
			continue
		}
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		requested = err != nil || enabled
	}
	return requested
}
