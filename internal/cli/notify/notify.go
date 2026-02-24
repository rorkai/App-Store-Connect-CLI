package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	slackWebhookEnvVar               = "ASC_SLACK_WEBHOOK"
	slackWebhookAllowLocalEnv        = "ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST"
	slackWebhookHost                 = "hooks.slack.com"
	slackWebhookGovHost              = "hooks.slack-gov.com"
	slackWebhookPathPrefix           = "/services/"
	slackWebhookMaxResponseBodyBytes = 4096
)

var slackThreadTSPattern = regexp.MustCompile(`^\d+\.\d+$`)

var slackHTTPClient = func() *http.Client {
	return &http.Client{Timeout: asc.ResolveTimeout()}
}

func slackFlags(fs *flag.FlagSet) (
	webhook *string,
	channel *string,
	message *string,
	threadTS *string,
	blocksJSON *string,
	blocksFile *string,
	payloadJSON *string,
	payloadFile *string,
	pretext *string,
	success *bool,
) {
	webhook = fs.String("webhook", "", "Slack webhook URL (https://hooks.slack.com/... or https://hooks.slack-gov.com/...; or set "+slackWebhookEnvVar+" env var)")
	channel = fs.String("channel", "", "Slack channel override (#channel or @username); incoming webhooks may ignore this based on app config")
	message = fs.String("message", "", "Message to send to Slack")
	threadTS = fs.String("thread-ts", "", "Parent message timestamp (thread_ts) for posting a threaded reply")
	blocksJSON = fs.String("blocks-json", "", "Slack Block Kit JSON array")
	blocksFile = fs.String("blocks-file", "", "Path to Slack Block Kit JSON array file")
	payloadJSON = fs.String("payload-json", "", "JSON object of release fields to include as Slack attachment fields")
	payloadFile = fs.String("payload-file", "", "Path to JSON object file for release payload fields")
	pretext = fs.String("pretext", "", "Optional text shown above attachment payload fields (requires --payload-json/--payload-file)")
	success = fs.Bool("success", true, "Set attachment color to success (true) or failure (false); requires --payload-json/--payload-file")
	return
}

func NotifyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("notify", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "notify",
		ShortUsage: "asc notify <subcommand> [flags]",
		ShortHelp:  "Send notifications to external services.",
		LongHelp: `Send notifications to external services.

Examples:
  asc notify slack --webhook $WEBHOOK --message "Build uploaded"
  ASC_SLACK_WEBHOOK=$WEBHOOK asc notify slack --message "Done"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SlackCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func SlackCommand() *ffcli.Command {
	fs := flag.NewFlagSet("notify slack", flag.ExitOnError)

	webhook, channel, message, threadTS, blocksJSON, blocksFile, payloadJSON, payloadFile, pretext, success := slackFlags(fs)

	return &ffcli.Command{
		Name:       "slack",
		ShortUsage: "asc notify slack --webhook URL --message TEXT",
		ShortHelp:  "Send a message to Slack via webhook.",
		LongHelp: `Send a message to Slack via incoming webhook.

This command sends a JSON payload to a Slack incoming webhook URL.
The webhook URL can be provided via --webhook flag or ASC_SLACK_WEBHOOK env var.
When using blocks, keep --message as the top-level text fallback.
Slack may ignore channel overrides for incoming webhooks based on app settings.
For --thread-ts, use the parent message ts from Slack APIs/events (webhook POST returns only "ok").
--pretext and --success are attachment options and require --payload-json/--payload-file.

Examples:
  asc notify slack --webhook "https://hooks.slack.com/..." --message "Build uploaded"
  asc notify slack --message "Done" --channel "#deployments"
  ASC_SLACK_WEBHOOK=$WEBHOOK asc notify slack --message "Release v2.1 ready"
  asc notify slack --message "Release ready" --blocks-json '[{"type":"section","text":{"type":"mrkdwn","text":"*Release* ready"}}]'
  asc notify slack --message "Release ready" --blocks-file ./blocks.json
  asc notify slack --message "Release update" --thread-ts "1733977745.12345"
  asc notify slack --message "Release submitted" --payload-json '{"app":"MyApp","version":"1.2.3","build":"42"}'`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			webhookURL := resolveWebhook(*webhook)
			if webhookURL == "" {
				fmt.Fprintf(os.Stderr, "Error: --webhook is required or set %s env var\n", slackWebhookEnvVar)
				return flag.ErrHelp
			}
			if err := validateSlackWebhookURL(webhookURL); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			msg := strings.TrimSpace(*message)
			if msg == "" {
				fmt.Fprintln(os.Stderr, "Error: --message is required")
				return flag.ErrHelp
			}

			blocks, err := parseSlackBlocks(*blocksJSON, *blocksFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			releasePayload, err := parseSlackPayload(*payloadJSON, *payloadFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if releasePayload == nil && (visited["pretext"] || visited["success"]) {
				fmt.Fprintln(os.Stderr, "Error: --pretext and --success require --payload-json or --payload-file")
				return flag.ErrHelp
			}

			payload := map[string]any{}
			payload["text"] = msg

			if ch := strings.TrimSpace(*channel); ch != "" {
				payload["channel"] = ch
			}
			if ts := strings.TrimSpace(*threadTS); ts != "" {
				if !slackThreadTSPattern.MatchString(ts) {
					fmt.Fprintln(os.Stderr, "Error: --thread-ts must be in Slack ts format (e.g. 1733977745.12345)")
					return flag.ErrHelp
				}
				payload["thread_ts"] = ts
			}
			if blocks != nil {
				payload["blocks"] = blocks
			}
			if releasePayload != nil {
				payload["attachments"] = []map[string]any{
					buildSlackAttachment(msg, strings.TrimSpace(*pretext), releasePayload, *success),
				}
			}

			body, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("notify slack: failed to marshal payload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			req, err := http.NewRequestWithContext(requestCtx, "POST", webhookURL, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("notify slack: failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			client := slackHTTPClient()
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("notify slack: failed to send: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				limited := io.LimitReader(resp.Body, slackWebhookMaxResponseBodyBytes)
				respBody, readErr := io.ReadAll(limited)
				if readErr != nil {
					return fmt.Errorf("notify slack: failed to read response: %w", readErr)
				}
				message := strings.TrimSpace(string(respBody))
				if message == "" {
					return fmt.Errorf("notify slack: unexpected response %d", resp.StatusCode)
				}
				return fmt.Errorf("notify slack: unexpected response %d: %s", resp.StatusCode, message)
			}

			fmt.Fprintln(os.Stderr, "Message sent to Slack successfully")
			return nil
		},
	}
}

func resolveWebhook(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(slackWebhookEnvVar)); v != "" {
		return v
	}
	return ""
}

func parseSlackBlocks(blocksJSON string, blocksFile string) ([]json.RawMessage, error) {
	blocksJSON = strings.TrimSpace(blocksJSON)
	blocksFile = strings.TrimSpace(blocksFile)

	if blocksJSON != "" && blocksFile != "" {
		return nil, fmt.Errorf("only one of --blocks-json or --blocks-file may be set")
	}
	if blocksJSON == "" && blocksFile == "" {
		return nil, nil
	}

	source := "--blocks-json"
	if blocksFile != "" {
		data, err := os.ReadFile(blocksFile)
		if err != nil {
			return nil, fmt.Errorf("--blocks-file must be readable: %w", err)
		}
		blocksJSON = strings.TrimSpace(string(data))
		source = "--blocks-file"
	}
	if blocksJSON == "" {
		return nil, fmt.Errorf("%s must contain a JSON array", source)
	}

	var blocks []json.RawMessage
	if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
		return nil, fmt.Errorf("%s must contain a JSON array: %w", source, err)
	}

	return blocks, nil
}

func parseSlackPayload(payloadJSON string, payloadFile string) (map[string]any, error) {
	payloadJSON = strings.TrimSpace(payloadJSON)
	payloadFile = strings.TrimSpace(payloadFile)

	if payloadJSON != "" && payloadFile != "" {
		return nil, fmt.Errorf("only one of --payload-json or --payload-file may be set")
	}
	if payloadJSON == "" && payloadFile == "" {
		return nil, nil
	}

	source := "--payload-json"
	if payloadFile != "" {
		data, err := os.ReadFile(payloadFile)
		if err != nil {
			return nil, fmt.Errorf("--payload-file must be readable: %w", err)
		}
		payloadJSON = strings.TrimSpace(string(data))
		source = "--payload-file"
	}
	if payloadJSON == "" {
		return nil, fmt.Errorf("%s must contain a JSON object", source)
	}

	decoder := json.NewDecoder(strings.NewReader(payloadJSON))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s must contain a JSON object: %w", source, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%s must contain a single JSON object", source)
	}
	if payload == nil {
		return nil, fmt.Errorf("%s must contain a JSON object", source)
	}
	return payload, nil
}

func buildSlackAttachment(message string, pretext string, payload map[string]any, success bool) map[string]any {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, map[string]any{
			"title": key,
			"value": formatPayloadValue(payload[key]),
			"short": false,
		})
	}

	color := "danger"
	if success {
		color = "good"
	}

	attachment := map[string]any{
		"fallback":  message,
		"color":     color,
		"mrkdwn_in": []string{"pretext", "fields"},
		"fields":    fields,
	}
	if pretext != "" {
		attachment["pretext"] = pretext
	}
	return attachment
}

func formatPayloadValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func validateSlackWebhookURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("--webhook must be a valid Slack webhook URL (https://hooks.slack.com/... or https://hooks.slack-gov.com/...)")
	}
	host := strings.ToLower(parsed.Hostname())
	if allowLocalSlackWebhook() && isLocalhost(host) {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("--webhook must use http or https")
		}
		return nil
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("--webhook must use https")
	}
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("--webhook must target %s", slackWebhookAllowedHostsLabel())
	}
	if !isSlackWebhookHost(host) {
		return fmt.Errorf("--webhook must target %s", slackWebhookAllowedHostsLabel())
	}
	if !strings.HasPrefix(parsed.Path, slackWebhookPathPrefix) {
		return fmt.Errorf("--webhook must start with %s", slackWebhookPathPrefix)
	}
	return nil
}

func isSlackWebhookHost(host string) bool {
	return host == slackWebhookHost || host == slackWebhookGovHost
}

func slackWebhookAllowedHostsLabel() string {
	return slackWebhookHost + " or " + slackWebhookGovHost
}

func allowLocalSlackWebhook() bool {
	value := strings.TrimSpace(os.Getenv(slackWebhookAllowLocalEnv))
	return value == "1" || strings.EqualFold(value, "true")
}

func isLocalhost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
