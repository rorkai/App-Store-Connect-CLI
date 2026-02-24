package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	webhooksServeDefaultHost         = "127.0.0.1"
	webhooksServeDefaultPort         = 8787
	webhooksServeDefaultMaxBodyBytes = 1 << 20 // 1 MiB
)

var errWebhookPayloadTooLarge = errors.New("payload exceeds max body size")

var webhookExecCommand = func(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", command)
}

type webhookServeStartup struct {
	URL          string `json:"url"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Dir          string `json:"dir,omitempty"`
	ExecEnabled  bool   `json:"execEnabled"`
	MaxBodyBytes int64  `json:"maxBodyBytes"`
}

type webhookServeEvent struct {
	ReceivedAt time.Time
	Payload    []byte
	EventType  string
	EventID    string
}

type webhookServeRuntime struct {
	dir          string
	execCommand  string
	maxBodyBytes int64
	fileCounter  uint64
}

// WebhooksServeCommand returns the webhooks serve subcommand.
func WebhooksServeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	host := fs.String("host", webhooksServeDefaultHost, "Host to bind the local webhook receiver")
	port := fs.Int("port", webhooksServeDefaultPort, "Port to bind the local webhook receiver (0-65535)")
	dir := fs.String("dir", "", "Optional directory to write one JSON payload file per event")
	execCommand := fs.String("exec", "", "Optional command to execute per event (payload JSON is piped on stdin)")
	output := fs.String("output", "text", "Output format: text (default), json")
	maxBodyBytes := fs.Int64("max-body-bytes", webhooksServeDefaultMaxBodyBytes, "Maximum accepted request body size in bytes")

	return &ffcli.Command{
		Name:       "serve",
		ShortUsage: "asc webhooks serve [flags]",
		ShortHelp:  "Run a local webhook receiver for testing and automation.",
		LongHelp: `Run a local webhook receiver for testing and automation.

Examples:
  asc webhooks serve --port 8787
  asc webhooks serve --port 8787 --dir ./webhook-events
  asc webhooks serve --port 8787 --exec "./scripts/on-webhook.sh"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: webhooks serve does not accept positional arguments")
				return flag.ErrHelp
			}

			bindHost := strings.TrimSpace(*host)
			if bindHost == "" {
				fmt.Fprintln(os.Stderr, "Error: --host is required")
				return flag.ErrHelp
			}
			if *port < 0 || *port > 65535 {
				fmt.Fprintln(os.Stderr, "Error: --port must be between 0 and 65535")
				return flag.ErrHelp
			}
			if *maxBodyBytes < 1 {
				fmt.Fprintln(os.Stderr, "Error: --max-body-bytes must be greater than 0")
				return flag.ErrHelp
			}

			outputFormat := strings.ToLower(strings.TrimSpace(*output))
			if outputFormat == "" {
				outputFormat = "text"
			}
			if outputFormat != "text" && outputFormat != "json" {
				fmt.Fprintln(os.Stderr, "Error: --output must be one of: text, json")
				return flag.ErrHelp
			}

			eventsDir, err := prepareWebhookServeDirectory(*dir)
			if err != nil {
				return fmt.Errorf("webhooks serve: %w", err)
			}

			listener, err := net.Listen("tcp", net.JoinHostPort(bindHost, strconv.Itoa(*port)))
			if err != nil {
				return fmt.Errorf("webhooks serve: failed to listen on %s: %w", net.JoinHostPort(bindHost, strconv.Itoa(*port)), err)
			}
			defer func() { _ = listener.Close() }()

			tcpAddr, ok := listener.Addr().(*net.TCPAddr)
			if !ok {
				return fmt.Errorf("webhooks serve: unexpected listener address type %T", listener.Addr())
			}
			actualPort := tcpAddr.Port
			startup := webhookServeStartup{
				URL:          fmt.Sprintf("http://%s", net.JoinHostPort(bindHost, strconv.Itoa(actualPort))),
				Host:         bindHost,
				Port:         actualPort,
				Dir:          eventsDir,
				ExecEnabled:  strings.TrimSpace(*execCommand) != "",
				MaxBodyBytes: *maxBodyBytes,
			}

			runtime := &webhookServeRuntime{
				dir:          eventsDir,
				execCommand:  strings.TrimSpace(*execCommand),
				maxBodyBytes: *maxBodyBytes,
			}
			server := &http.Server{
				Handler:           runtime.newHandler(ctx),
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       15 * time.Second,
				WriteTimeout:      15 * time.Second,
				IdleTimeout:       60 * time.Second,
			}

			serveErrCh := make(chan error, 1)
			go func() {
				err := server.Serve(listener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					serveErrCh <- err
					return
				}
				serveErrCh <- nil
			}()

			if outputFormat == "json" {
				if err := asc.PrintJSON(startup); err != nil {
					return fmt.Errorf("webhooks serve: %w", err)
				}
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "Listening for webhook events on %s\n", startup.URL)
			}

			select {
			case err := <-serveErrCh:
				if err != nil {
					return fmt.Errorf("webhooks serve: %w", err)
				}
				return nil
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = server.Shutdown(shutdownCtx)
				if err := <-serveErrCh; err != nil {
					return fmt.Errorf("webhooks serve: %w", err)
				}
				return nil
			}
		},
	}
}

func (r *webhookServeRuntime) newHandler(ctx context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			writeWebhookServeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": "method not allowed",
			})
			return
		}

		payload, err := readWebhookServeJSONPayload(req.Body, r.maxBodyBytes)
		if err != nil {
			if errors.Is(err, errWebhookPayloadTooLarge) {
				writeWebhookServeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
					"error": "payload too large",
				})
				return
			}
			writeWebhookServeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON payload",
			})
			return
		}

		eventType, eventID := extractWebhookServeEventMetadata(req.Header, payload)
		event := webhookServeEvent{
			ReceivedAt: time.Now().UTC(),
			Payload:    payload,
			EventType:  eventType,
			EventID:    eventID,
		}

		fmt.Fprintf(
			os.Stderr,
			"webhooks serve: received event type=%s id=%s bytes=%d\n",
			firstNonEmpty(strings.TrimSpace(event.EventType), "unknown"),
			firstNonEmpty(strings.TrimSpace(event.EventID), "unknown"),
			len(payload),
		)

		go r.processEvent(ctx, event)

		writeWebhookServeJSON(w, http.StatusAccepted, map[string]any{
			"accepted": true,
		})
	})
}

func (r *webhookServeRuntime) processEvent(ctx context.Context, event webhookServeEvent) {
	if r.dir != "" {
		path, err := r.writeEventFile(event)
		if err != nil {
			fmt.Fprintf(os.Stderr, "webhooks serve: failed to persist event id=%s: %v\n", firstNonEmpty(event.EventID, "unknown"), err)
		} else {
			fmt.Fprintf(os.Stderr, "webhooks serve: wrote event payload to %s\n", path)
		}
	}

	if r.execCommand != "" {
		if err := runWebhookExecCommand(ctx, r.execCommand, event.Payload); err != nil {
			fmt.Fprintf(os.Stderr, "webhooks serve: exec failed for event id=%s: %v\n", firstNonEmpty(event.EventID, "unknown"), err)
		}
	}
}

func (r *webhookServeRuntime) writeEventFile(event webhookServeEvent) (string, error) {
	fileIndex := atomic.AddUint64(&r.fileCounter, 1)
	fileName := fmt.Sprintf(
		"%s-%06d-%s.json",
		event.ReceivedAt.Format("20060102T150405.000000000Z"),
		fileIndex,
		sanitizeWebhookServeFilenameSegment(event.EventType),
	)
	outputPath := filepath.Join(r.dir, fileName)
	if _, err := shared.WriteStreamToFile(outputPath, bytes.NewReader(event.Payload)); err != nil {
		return "", err
	}
	return outputPath, nil
}

func prepareWebhookServeDirectory(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	cleaned := filepath.Clean(trimmed)
	info, err := os.Lstat(cleaned)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing to use symlink directory %q", cleaned)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("--dir must point to a directory: %q", cleaned)
		}
		return cleaned, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(cleaned, 0o755); err != nil {
		return "", err
	}
	return cleaned, nil
}

func readWebhookServeJSONPayload(body io.ReadCloser, maxBodyBytes int64) ([]byte, error) {
	defer func() { _ = body.Close() }()

	limited := &io.LimitedReader{R: body, N: maxBodyBytes}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if limited.N == 0 {
		var probe [1]byte
		n, probeErr := body.Read(probe[:])
		if n > 0 {
			return nil, errWebhookPayloadTooLarge
		}
		if probeErr != nil && !errors.Is(probeErr, io.EOF) {
			return nil, probeErr
		}
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty request body")
	}

	compact := bytes.NewBuffer(nil)
	if err := json.Compact(compact, trimmed); err != nil {
		return nil, err
	}
	return compact.Bytes(), nil
}

func extractWebhookServeEventMetadata(header http.Header, payload []byte) (string, string) {
	eventType := strings.TrimSpace(firstNonEmpty(
		header.Get("X-Apple-Event-Type"),
		header.Get("X-ASC-Event-Type"),
		header.Get("X-Apple-Notification-Type"),
	))
	eventID := strings.TrimSpace(firstNonEmpty(
		header.Get("X-Request-ID"),
		header.Get("X-Apple-Request-ID"),
		header.Get("X-Apple-Notification-ID"),
	))

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err == nil {
		if eventType == "" {
			eventType = strings.TrimSpace(firstNonEmpty(
				stringValueAtPath(obj, "eventType"),
				stringValueAtPath(obj, "notificationType"),
				stringValueAtPath(obj, "type"),
				stringValueAtPath(obj, "data", "type"),
			))
		}
		if eventID == "" {
			eventID = strings.TrimSpace(firstNonEmpty(
				stringValueAtPath(obj, "id"),
				stringValueAtPath(obj, "notificationId"),
				stringValueAtPath(obj, "data", "id"),
			))
		}
	}

	return eventType, eventID
}

func runWebhookExecCommand(ctx context.Context, command string, payload []byte) error {
	cmd := webhookExecCommand(ctx, command)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}

	return nil
}

func writeWebhookServeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func stringValueAtPath(root map[string]any, path ...string) string {
	current := any(root)
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		next, ok := m[key]
		if !ok {
			return ""
		}
		current = next
	}

	value, ok := current.(string)
	if !ok {
		return ""
	}
	return value
}

func sanitizeWebhookServeFilenameSegment(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "event"
	}

	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "event"
	}
	if len(result) > 48 {
		return result[:48]
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
