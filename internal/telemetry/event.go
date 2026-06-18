package telemetry

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	EventID          string           `json:"event_id"`
	SchemaVersion    uint8            `json:"schema_version"`
	ASCVersion       string           `json:"asc_version"`
	OS               string           `json:"os"`
	Arch             string           `json:"arch"`
	CommandPath      string           `json:"command_path"`
	CommandFamily    string           `json:"command_family"`
	DurationMS       uint32           `json:"duration_ms"`
	DurationBucket   string           `json:"duration_bucket"`
	ExitCode         int              `json:"exit_code"`
	Success          bool             `json:"success"`
	RuntimeContext   RuntimeContext   `json:"runtime_context"`
	InvocationSource InvocationSource `json:"invocation_source"`
	InstallID        *string          `json:"install_id"`
	SessionID        string           `json:"session_id"`
}

// processSessionID groups events from one CLI process without linking separate
// invocations of the executable.
var processSessionID = uuid.NewString()

func BuildEvent(commandName, version string, duration time.Duration, exitCode int) (Event, bool) {
	commandPath := sanitizeCommandName(commandName)
	if commandPath == "" {
		return Event{}, false
	}
	if shouldSkipCommand(commandPath) {
		return Event{}, false
	}

	runtimeContext := DetectRuntimeContext()
	invocationSource := DetectInvocationSource()
	var installID *string
	if shouldAttachInstallID(runtimeContext) {
		id, err := ensureInstallID(0)
		if err == nil && id != "" {
			installID = &id
		}
	}

	return Event{
		EventID:          uuid.NewString(),
		SchemaVersion:    1,
		ASCVersion:       strings.TrimSpace(version),
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		CommandPath:      commandPath,
		CommandFamily:    commandFamily(commandPath),
		DurationMS:       durationMillis(duration),
		DurationBucket:   durationBucket(duration),
		ExitCode:         exitCode,
		Success:          exitCode == 0,
		RuntimeContext:   runtimeContext,
		InvocationSource: invocationSource,
		InstallID:        installID,
		SessionID:        processSessionID,
	}, true
}

func sanitizeCommandName(commandName string) string {
	commandName = strings.ToLower(strings.Join(strings.Fields(commandName), " "))
	if commandName == "" {
		return ""
	}
	if commandName == "asc" || strings.HasPrefix(commandName, "asc ") {
		return commandName
	}
	return "asc " + commandName
}

func shouldSkipCommand(commandPath string) bool {
	switch commandPath {
	case "", "asc", "asc completion", "asc version", "asc telemetry", "asc telemetry status", "asc telemetry enable", "asc telemetry disable", "asc telemetry reset-id":
		return true
	default:
		return false
	}
}

func commandFamily(commandPath string) string {
	parts := strings.Fields(commandPath)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func durationMillis(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	ms := d.Milliseconds()
	if ms > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(ms)
}

func durationBucket(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	switch {
	case ms < 100:
		return "lt_100ms"
	case ms < 500:
		return "100ms_500ms"
	case ms < 1000:
		return "500ms_1s"
	case ms < 5000:
		return "1s_5s"
	case ms < 30000:
		return "5s_30s"
	default:
		return "gte_30s"
	}
}

func RedactedSummary(ev Event) map[string]string {
	installID := ""
	if ev.InstallID != nil {
		installID = *ev.InstallID
	}
	return map[string]string{
		"command_path":      ev.CommandPath,
		"command_family":    ev.CommandFamily,
		"runtime_context":   string(ev.RuntimeContext),
		"invocation_source": string(ev.InvocationSource),
		"duration_ms":       strconv.FormatUint(uint64(ev.DurationMS), 10),
		"install_id":        installID,
	}
}
