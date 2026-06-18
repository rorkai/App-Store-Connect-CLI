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
	maxSendDuration = 250 * time.Millisecond
)

var sendHTTP = sendHTTPEvent

func Emit(commandName, version string, duration time.Duration, exitCode int) {
	commandPath := sanitizeCommandName(commandName)
	if shouldSkipCommand(commandPath) {
		return
	}

	if reason := environmentOptOutReason(); reason != "" {
		debugf("telemetry disabled by %s", reason)
		return
	}

	st, err := loadCurrentState()
	if err != nil {
		debugf("telemetry disabled: %v", err)
		return
	}
	enabled, reason := enabledFromState(st)
	if !enabled {
		debugf("telemetry disabled by %s", reason)
		return
	}

	ev, ok := BuildEvent(commandPath, version, duration, exitCode)
	if !ok {
		return
	}

	if err := sendHTTP(ev); err != nil {
		debugf("telemetry send failed: %v", err)
	}
}

func loadCurrentState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}
	return loadState(path)
}

func sendHTTPEvent(ev Event) error {
	endpoint := endpoint()
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("invalid telemetry endpoint")
	}

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	configuredCtx, cancelConfigured := shared.ContextWithTimeout(context.Background())
	defer cancelConfigured()
	ctx, cancel := context.WithTimeout(configuredCtx, maxSendDuration)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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

func endpoint() string {
	if raw := strings.TrimSpace(os.Getenv(endpointEnvVar)); raw != "" {
		return raw
	}
	return DefaultEndpoint
}

func debugf(format string, args ...any) {
	debug := strings.ToLower(strings.TrimSpace(os.Getenv("ASC_DEBUG")))
	if debug == "" || debug == "0" || debug == "false" || debug == "off" {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
