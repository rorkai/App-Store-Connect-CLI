package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	DefaultEndpoint = "https://rork.com/cf-api/asc/v1/events"
	endpointEnvVar  = "ASC_TELEMETRY_ENDPOINT"
	maxSendDuration = 3 * time.Second
)

func Emit(commandName, version string, duration time.Duration, exitCode int) {
	EmitWithContext(commandName, version, duration, exitCode, EventContext{InvocationShape: InvocationShapeLeaf})
}

func EmitWithContext(
	commandName, version string,
	duration time.Duration,
	exitCode int,
	eventContext EventContext,
) {
	commandPath := sanitizeCommandName(commandName)
	if reason := environmentOptOutReason(); reason != "" {
		if err := purgeDefaultSpool(); err != nil {
			debugf("telemetry spool purge failed after %s: %v", reason, err)
		}
		debugf("telemetry disabled by %s", reason)
		return
	}
	if shouldSkipCommand(commandPath) {
		return
	}

	st, err := loadCurrentState()
	if err != nil {
		debugf("telemetry disabled: %v", err)
		return
	}
	enabled, reason := enabledFromState(st)
	if !enabled {
		if err := purgeDefaultSpool(); err != nil {
			debugf("telemetry spool purge failed after %s: %v", reason, err)
		}
		debugf("telemetry disabled by %s", reason)
		return
	}

	ev, ok := BuildEventWithContext(commandPath, version, duration, exitCode, eventContext)
	if !ok {
		return
	}
	endpointURL := endpoint()
	if err := validateEndpoint(endpointURL); err != nil {
		debugf("telemetry send failed: %v", err)
		return
	}

	store, err := defaultSpoolStore()
	if err != nil {
		debugf("telemetry spool unavailable: %v", err)
		return
	}
	spooledRecords, err := store.appendAndCount(spoolRecord{Event: ev, Endpoint: endpointURL})
	if err != nil {
		debugf("telemetry spool append failed: %v", err)
		return
	}
	maybeStartMaintenanceWorker(store, spooledRecords)
}

func loadCurrentState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}
	return loadState(path)
}

func sendHTTPEventToEndpoint(ev Event, endpointURL string) error {
	if err := validateEndpoint(endpointURL); err != nil {
		return err
	}
	if endpointURL == "" {
		return nil
	}

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	configuredCtx, cancelConfigured := shared.ContextWithTimeout(context.Background())
	defer cancelConfigured()
	ctx, cancel := context.WithTimeout(configuredCtx, maxSendDuration)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	transport := http.DefaultClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected telemetry status %d", resp.StatusCode)
	}
	return nil
}

func validateEndpoint(endpointURL string) error {
	if endpointURL == "" {
		return nil
	}
	parsed, err := url.Parse(endpointURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("invalid telemetry endpoint")
	}
	return nil
}

func endpoint() string {
	if raw := strings.TrimSpace(os.Getenv(endpointEnvVar)); raw != "" {
		return raw
	}
	return DefaultEndpoint
}

func debugf(format string, args ...any) {
	if !debugEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func debugEnabled() bool {
	debug := strings.ToLower(strings.TrimSpace(os.Getenv("ASC_DEBUG")))
	return debug != "" && debug != "0" && debug != "false" && debug != "off"
}
