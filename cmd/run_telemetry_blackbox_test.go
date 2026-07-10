package cmd

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRun_BuiltBinaryEmitsSchemaV4Payload(t *testing.T) {
	binaryPath := buildASCBlackboxBinary(t)
	home := t.TempDir()
	command := exec.Command(
		binaryPath,
		"versions",
		"attach-build",
		"--version-id",
		"VERSION_ID",
		"--build-id",
		"BUILD_ID",
	)
	command.Env = telemetryBlackboxEnv(home, false, "https://127.0.0.1:1/events")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != ExitUsage {
		t.Fatalf("built command error = %v, want exit %d; output=%s", err, ExitUsage, output)
	}

	spoolData, err := os.ReadFile(filepath.Join(home, ".asc", "telemetry-spool.jsonl"))
	if err != nil {
		t.Fatalf("read built CLI telemetry spool: %v", err)
	}
	var record struct {
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(spoolData, &record); err != nil {
		t.Fatalf("decode built CLI telemetry spool: %v", err)
	}
	var event telemetry.Event
	if err := json.Unmarshal(record.Event, &event); err != nil {
		t.Fatalf("decode built CLI telemetry event: %v", err)
	}
	if event.SchemaVersion != 4 || event.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected schema-v4 payload: %+v", event)
	}
	if event.FailureParameter == nil || *event.FailureParameter != "--build-id" {
		t.Fatalf("FailureParameter = %v, want --build-id", event.FailureParameter)
	}
	var payload map[string]any
	if err := json.Unmarshal(record.Event, &payload); err != nil {
		t.Fatalf("decode built CLI telemetry JSON: %v", err)
	}
	if value, exists := payload["http_status"]; !exists || value != nil {
		t.Fatalf("http_status = %v (exists=%t), want explicit null", value, exists)
	}
}

func TestRun_BuiltBinaryDoesNotWaitForBlockedTelemetryEndpoint(t *testing.T) {
	binaryPath := buildASCBlackboxBinary(t)
	blockedEndpoint, accepted, release := startBlockedTelemetryEndpoint(t)

	disabledHome := t.TempDir()
	runTimedTelemetryCommand(t, binaryPath, disabledHome, true, "")
	disabledDuration := runTimedTelemetryCommand(t, binaryPath, disabledHome, true, "")

	blockedHome := t.TempDir()
	blockedDuration := runTimedTelemetryCommand(t, binaryPath, blockedHome, false, blockedEndpoint)
	t.Logf("foreground timing: disabled=%s blocked=%s added=%s", disabledDuration, blockedDuration, blockedDuration-disabledDuration)
	if added := blockedDuration - disabledDuration; added >= 175*time.Millisecond {
		t.Fatalf(
			"blocked telemetry added %s to foreground runtime (disabled=%s blocked=%s), want less than 175ms",
			added,
			disabledDuration,
			blockedDuration,
		)
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("detached telemetry worker did not reach blocked endpoint")
	}
	release()
}

func runTimedTelemetryCommand(
	t *testing.T,
	binaryPath string,
	home string,
	disabled bool,
	endpoint string,
) time.Duration {
	t.Helper()
	command := exec.Command(binaryPath, "builds", "--definitely-invalid")
	command.Env = telemetryBlackboxEnv(home, disabled, endpoint)
	start := time.Now()
	output, err := command.CombinedOutput()
	duration := time.Since(start)

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != ExitUsage {
		t.Fatalf("built command error = %v, want exit %d; output=%s", err, ExitUsage, output)
	}
	return duration
}

func telemetryBlackboxEnv(home string, disabled bool, endpoint string) []string {
	environment := filterEnvVars(
		os.Environ(),
		"ASC_BYPASS_KEYCHAIN",
		"ASC_CONFIG_PATH",
		"ASC_DEBUG",
		"ASC_TELEMETRY_DISABLED",
		"ASC_TELEMETRY_ENDPOINT",
		"ASC_TELEMETRY_EPHEMERAL",
		"ASC_TIMEOUT",
		"ASC_TIMEOUT_SECONDS",
		"DO_NOT_TRACK",
		"HOME",
		"SSL_CERT_FILE",
	)
	disabledValue := ""
	if disabled {
		disabledValue = "1"
	}
	return append(
		environment,
		"ASC_BYPASS_KEYCHAIN=1",
		"ASC_CONFIG_PATH="+filepath.Join(home, "config.json"),
		"ASC_DEBUG=",
		"ASC_TELEMETRY_DISABLED="+disabledValue,
		"ASC_TELEMETRY_ENDPOINT="+endpoint,
		"ASC_TELEMETRY_EPHEMERAL=",
		"ASC_TIMEOUT=1s",
		"ASC_TIMEOUT_SECONDS=",
		"DO_NOT_TRACK=",
		"HOME="+home,
	)
}

func startBlockedTelemetryEndpoint(t *testing.T) (string, <-chan struct{}, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for blocked telemetry endpoint: %v", err)
	}
	accepted := make(chan struct{}, 1)
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		accepted <- struct{}{}
		<-release
		_ = connection.Close()
	}()

	released := false
	cleanup := func() {
		if !released {
			close(release)
			released = true
		}
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("blocked telemetry endpoint did not shut down")
		}
	}
	t.Cleanup(cleanup)
	return "https://" + listener.Addr().String() + "/events", accepted, cleanup
}
