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
	DefaultEndpoint = "https://telemetry.rork.com/asc/v1/events"
	endpointEnvVar  = "ASC_TELEMETRY_ENDPOINT"
	maxSendDuration = 250 * time.Millisecond
)

var sendHTTP = sendHTTPEvent

func Emit(args []string, commandName, version string, duration time.Duration, exitCode int) {
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

	ev, ok := BuildEvent(args, commandName, version, duration, exitCode)
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
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
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

	client := *http.DefaultClient
	checkRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "https" {
			return fmt.Errorf("non-HTTPS telemetry redirect")
		}
		if checkRedirect != nil {
			return checkRedirect(request, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
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
