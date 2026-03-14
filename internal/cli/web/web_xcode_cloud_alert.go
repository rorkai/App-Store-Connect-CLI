package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const usageAlertSlackWebhookEnv = "ASC_SLACK_WEBHOOK"

type usageAlertSeverity string

const (
	usageAlertSeverityUnknown  usageAlertSeverity = "unknown"
	usageAlertSeverityOK       usageAlertSeverity = "ok"
	usageAlertSeverityWarning  usageAlertSeverity = "warning"
	usageAlertSeverityCritical usageAlertSeverity = "critical"
)

type usageAlertFailOn string

const (
	usageAlertFailOnNone     usageAlertFailOn = "none"
	usageAlertFailOnWarning  usageAlertFailOn = "warning"
	usageAlertFailOnCritical usageAlertFailOn = "critical"
)

type usageAlertNotifyOn string

const (
	usageAlertNotifyOnNone     usageAlertNotifyOn = "none"
	usageAlertNotifyOnWarning  usageAlertNotifyOn = "warning"
	usageAlertNotifyOnCritical usageAlertNotifyOn = "critical"
	usageAlertNotifyOnAlways   usageAlertNotifyOn = "always"
)

var usageAlertHTTPClientFn = func() *http.Client {
	return &http.Client{Timeout: asc.ResolveTimeout()}
}

var (
	sendUsageAlertSlackFn   = sendUsageAlertToSlack
	sendUsageAlertWebhookFn = sendUsageAlertToWebhook
)

// CIUsageAlertResult is the output payload for usage alert evaluation.
type CIUsageAlertResult struct {
	TeamID        string                     `json:"team_id"`
	EvaluatedAt   string                     `json:"evaluated_at"`
	Severity      usageAlertSeverity         `json:"severity"`
	Message       string                     `json:"message"`
	FailOn        usageAlertFailOn           `json:"fail_on"`
	NotifyOn      usageAlertNotifyOn         `json:"notify_on"`
	Thresholds    CIUsageAlertThresholds     `json:"thresholds"`
	Plan          CIUsageAlertPlan           `json:"plan"`
	Trend         *CIUsageAlertTrend         `json:"trend,omitempty"`
	Notifications []CIUsageAlertNotification `json:"notifications,omitempty"`
}

// CIUsageAlertThresholds captures warning and critical threshold percentages.
type CIUsageAlertThresholds struct {
	WarnAt     int `json:"warn_at"`
	CriticalAt int `json:"critical_at"`
}

// CIUsageAlertPlan captures plan quota and calculated usage percentage.
type CIUsageAlertPlan struct {
	Name          string `json:"name"`
	Used          int    `json:"used"`
	Available     int    `json:"available"`
	Total         int    `json:"total"`
	UsedPercent   int    `json:"used_percent"`
	ResetDate     string `json:"reset_date,omitempty"`
	ResetDateTime string `json:"reset_date_time,omitempty"`
	ManageURL     string `json:"manage_url,omitempty"`
}

// CIUsageAlertTrend carries monthly usage context.
type CIUsageAlertTrend struct {
	RequestedMonths   int                 `json:"requested_months"`
	Available         bool                `json:"available"`
	UnavailableReason string              `json:"unavailable_reason,omitempty"`
	AverageMinutes    int                 `json:"average_minutes,omitempty"`
	PeakMinutes       int                 `json:"peak_minutes,omitempty"`
	Months            []CIUsageAlertMonth `json:"months,omitempty"`
}

// CIUsageAlertMonth is one monthly usage datapoint.
type CIUsageAlertMonth struct {
	Year    int `json:"year"`
	Month   int `json:"month"`
	Minutes int `json:"minutes"`
	Builds  int `json:"builds"`
}

// CIUsageAlertNotification captures delivery status for outbound notifications.
type CIUsageAlertNotification struct {
	Channel    string `json:"channel"`
	Triggered  bool   `json:"triggered"`
	Delivered  bool   `json:"delivered"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

type usageAlertHeaderFlags []string

func (f *usageAlertHeaderFlags) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *usageAlertHeaderFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func webXcodeCloudUsageAlertCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage alert", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	warnAt := fs.Int("warn-at", 80, "Warning threshold percent (1-99)")
	criticalAt := fs.Int("critical-at", 95, "Critical threshold percent (1-100)")
	failOn := fs.String("fail-on", string(usageAlertFailOnCritical), "Exit non-zero when severity reaches: none, warning, critical")
	notifyOn := fs.String("notify-on", string(usageAlertNotifyOnWarning), "Send notifications when severity reaches: none, warning, critical, always")
	slackWebhook := fs.String("slack-webhook", "", "Slack incoming webhook URL (optional, or set ASC_SLACK_WEBHOOK)")
	webhook := fs.String("webhook", "", "Generic webhook URL for JSON alert payloads (optional)")
	trendMonths := fs.Int("trend-months", 6, "Monthly trend window in months (0 to disable, max 24)")

	var webhookHeaders usageAlertHeaderFlags
	fs.Var(&webhookHeaders, "webhook-header", "Header for --webhook in 'Key: Value' format (repeatable)")

	return &ffcli.Command{
		Name:       "alert",
		ShortUsage: "asc web xcode-cloud usage alert [flags]",
		ShortHelp:  "[experimental] Evaluate usage thresholds and send alerts.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Evaluate Xcode Cloud usage thresholds from plan quota, optionally include monthly trend context,
and optionally notify Slack/webhook endpoints.

Exit behavior:
  - Exit 0 when thresholds are not breached, or when --fail-on none
  - Exit 1 when severity meets --fail-on level (warning/critical)
  - Exit 2 for invalid flag usage

` + webWarningText + `

Examples:
  asc web xcode-cloud usage alert --apple-id "user@example.com"
  asc web xcode-cloud usage alert --warn-at 75 --critical-at 90 --fail-on warning --output table
  asc web xcode-cloud usage alert --slack-webhook "https://hooks.slack.com/services/..." --notify-on critical
  asc web xcode-cloud usage alert --webhook "https://example.com/alerts" --webhook-header "Authorization: Bearer TOKEN"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateUsageAlertThresholds(*warnAt, *criticalAt); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			failOnLevel, err := parseUsageAlertFailOn(*failOn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			notifyOnLevel, err := parseUsageAlertNotifyOn(*notifyOn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			if *trendMonths < 0 || *trendMonths > 24 {
				fmt.Fprintln(os.Stderr, "Error: --trend-months must be between 0 and 24")
				return flag.ErrHelp
			}
			normalizedSlackWebhook, err := resolveUsageAlertWebhookURL(
				resolveUsageAlertSlackWebhook(*slackWebhook),
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: --slack-webhook %s\n", err)
				return flag.ErrHelp
			}
			normalizedWebhookURL, err := resolveUsageAlertWebhookURL(*webhook)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: --webhook %s\n", err)
				return flag.ErrHelp
			}
			parsedHeaders, err := parseUsageAlertHeaders(webhookHeaders)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud usage alert failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			alertResult := &CIUsageAlertResult{}
			err = withWebSpinner("Loading Xcode Cloud usage alert data", func() error {
				summary, err := client.GetCIUsageSummary(requestCtx, teamID)
				if err != nil {
					return err
				}

				alertResult = buildCIUsageAlertResult(
					teamID,
					summary,
					*warnAt,
					*criticalAt,
					failOnLevel,
					notifyOnLevel,
				)
				if *trendMonths > 0 {
					alertResult.Trend = loadUsageAlertTrend(requestCtx, client, teamID, *trendMonths)
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud usage alert")
			}

			notifyErr := error(nil)
			if strings.TrimSpace(normalizedSlackWebhook) != "" || strings.TrimSpace(normalizedWebhookURL) != "" {
				notifyErr = withWebSpinner("Sending usage alert notifications", func() error {
					return deliverUsageAlertNotifications(
						requestCtx,
						alertResult,
						normalizedSlackWebhook,
						normalizedWebhookURL,
						parsedHeaders,
						notifyOnLevel,
					)
				})
			}

			if err := shared.PrintOutputWithRenderers(
				alertResult,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIUsageAlertTable(alertResult) },
				func() error { return renderCIUsageAlertMarkdown(alertResult) },
			); err != nil {
				return err
			}

			var resultErr error
			if notifyErr != nil {
				resultErr = fmt.Errorf("xcode-cloud usage alert notification failed: %w", notifyErr)
			}
			if shouldFailUsageAlert(alertResult.Severity, failOnLevel) {
				resultErr = errors.Join(
					resultErr,
					fmt.Errorf("xcode-cloud usage alert threshold breach: %s", alertResult.Message),
				)
			}
			return resultErr
		},
	}
}

func validateUsageAlertThresholds(warnAt, criticalAt int) error {
	if warnAt < 1 || warnAt > 99 {
		return fmt.Errorf("--warn-at must be between 1 and 99")
	}
	if criticalAt < 1 || criticalAt > 100 {
		return fmt.Errorf("--critical-at must be between 1 and 100")
	}
	if warnAt >= criticalAt {
		return fmt.Errorf("--warn-at must be less than --critical-at")
	}
	return nil
}

func parseUsageAlertFailOn(value string) (usageAlertFailOn, error) {
	switch usageAlertFailOn(strings.ToLower(strings.TrimSpace(value))) {
	case usageAlertFailOnNone, usageAlertFailOnWarning, usageAlertFailOnCritical:
		return usageAlertFailOn(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("--fail-on must be one of: none, warning, critical")
	}
}

func parseUsageAlertNotifyOn(value string) (usageAlertNotifyOn, error) {
	switch usageAlertNotifyOn(strings.ToLower(strings.TrimSpace(value))) {
	case usageAlertNotifyOnNone, usageAlertNotifyOnWarning, usageAlertNotifyOnCritical, usageAlertNotifyOnAlways:
		return usageAlertNotifyOn(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("--notify-on must be one of: none, warning, critical, always")
	}
}

func resolveUsageAlertSlackWebhook(flagValue string) string {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return flagValue
	}
	return strings.TrimSpace(os.Getenv(usageAlertSlackWebhookEnv))
}

func resolveUsageAlertWebhookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("must use http or https scheme")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("must include a hostname")
	}
	return parsed.String(), nil
}

func parseUsageAlertHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, entry := range values {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("--webhook-header must be in 'Key: Value' format")
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("--webhook-header key cannot be empty")
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("--webhook-header cannot contain newlines")
		}
		headers.Add(key, value)
	}
	return headers, nil
}

func buildCIUsageAlertResult(
	teamID string,
	summary *webcore.CIUsageSummary,
	warnAt, criticalAt int,
	failOn usageAlertFailOn,
	notifyOn usageAlertNotifyOn,
) *CIUsageAlertResult {
	if summary == nil {
		summary = &webcore.CIUsageSummary{}
	}
	used := summary.Plan.Used
	total := summary.Plan.Total
	percentUsed := calculateUsagePercent(used, total)
	severity := classifyUsageAlertSeverity(used, total, warnAt, criticalAt)

	result := &CIUsageAlertResult{
		TeamID:      teamID,
		EvaluatedAt: webNowFn().UTC().Format(time.RFC3339),
		Severity:    severity,
		FailOn:      failOn,
		NotifyOn:    notifyOn,
		Thresholds: CIUsageAlertThresholds{
			WarnAt:     warnAt,
			CriticalAt: criticalAt,
		},
		Plan: CIUsageAlertPlan{
			Name:          strings.TrimSpace(summary.Plan.Name),
			Used:          used,
			Available:     summary.Plan.Available,
			Total:         total,
			UsedPercent:   percentUsed,
			ResetDate:     strings.TrimSpace(summary.Plan.ResetDate),
			ResetDateTime: strings.TrimSpace(summary.Plan.ResetDateTime),
			ManageURL:     strings.TrimSpace(summary.Links["manage"]),
		},
	}
	result.Message = buildUsageAlertMessage(result)
	return result
}

func calculateUsagePercent(used, total int) int {
	if total <= 0 {
		return 0
	}
	if used < 0 {
		used = 0
	}
	if used > total {
		used = total
	}
	return (used*100 + total/2) / total
}

func classifyUsageAlertSeverity(used, total, warnAt, criticalAt int) usageAlertSeverity {
	if total <= 0 {
		return usageAlertSeverityUnknown
	}
	if used < 0 {
		used = 0
	}
	usedScaled := int64(used) * 100
	totalScaled := int64(total)
	if usedScaled >= int64(criticalAt)*totalScaled {
		return usageAlertSeverityCritical
	}
	if usedScaled >= int64(warnAt)*totalScaled {
		return usageAlertSeverityWarning
	}
	return usageAlertSeverityOK
}

func buildUsageAlertMessage(result *CIUsageAlertResult) string {
	if result == nil {
		return "xcode-cloud usage alert unavailable"
	}
	if result.Plan.Total <= 0 {
		return "xcode-cloud usage alert cannot evaluate thresholds because plan total is unavailable"
	}
	reset := strings.TrimSpace(result.Plan.ResetDate)
	if reset == "" {
		reset = "n/a"
	}
	return fmt.Sprintf(
		"xcode-cloud usage is %s at %d%% (%d/%dm); reset date: %s",
		result.Severity,
		result.Plan.UsedPercent,
		result.Plan.Used,
		result.Plan.Total,
		reset,
	)
}

func shouldFailUsageAlert(severity usageAlertSeverity, failOn usageAlertFailOn) bool {
	switch failOn {
	case usageAlertFailOnNone:
		return false
	case usageAlertFailOnWarning:
		return severity == usageAlertSeverityWarning || severity == usageAlertSeverityCritical
	case usageAlertFailOnCritical:
		return severity == usageAlertSeverityCritical
	default:
		return false
	}
}

func shouldNotifyUsageAlert(severity usageAlertSeverity, notifyOn usageAlertNotifyOn) bool {
	switch notifyOn {
	case usageAlertNotifyOnNone:
		return false
	case usageAlertNotifyOnAlways:
		return true
	case usageAlertNotifyOnWarning:
		return severity == usageAlertSeverityWarning || severity == usageAlertSeverityCritical
	case usageAlertNotifyOnCritical:
		return severity == usageAlertSeverityCritical
	default:
		return false
	}
}

func loadUsageAlertTrend(ctx context.Context, client *webcore.Client, teamID string, months int) *CIUsageAlertTrend {
	trend := &CIUsageAlertTrend{RequestedMonths: months}
	if months <= 0 || client == nil {
		trend.Available = false
		trend.UnavailableReason = "monthly trend disabled"
		return trend
	}

	now := webNowFn().UTC()
	startMonth, startYear, endMonth, endYear := usageAlertMonthWindow(now, months)
	response, err := client.GetCIUsageMonths(
		ctx,
		teamID,
		startMonth,
		startYear,
		endMonth,
		endYear,
	)
	if err != nil || response == nil {
		trend.Available = false
		trend.UnavailableReason = "monthly trend unavailable"
		return trend
	}

	usage := append([]webcore.CIMonthUsage(nil), response.Usage...)
	sort.Slice(usage, func(i, j int) bool {
		if usage[i].Year == usage[j].Year {
			return usage[i].Month < usage[j].Month
		}
		return usage[i].Year < usage[j].Year
	})
	if len(usage) > months {
		usage = usage[len(usage)-months:]
	}
	if len(usage) == 0 {
		trend.Available = false
		trend.UnavailableReason = "monthly trend unavailable"
		return trend
	}

	totalMinutes := 0
	peakMinutes := 0
	trend.Months = make([]CIUsageAlertMonth, 0, len(usage))
	for _, monthUsage := range usage {
		totalMinutes += monthUsage.Duration
		if monthUsage.Duration > peakMinutes {
			peakMinutes = monthUsage.Duration
		}
		trend.Months = append(trend.Months, CIUsageAlertMonth{
			Year:    monthUsage.Year,
			Month:   monthUsage.Month,
			Minutes: monthUsage.Duration,
			Builds:  monthUsage.NumberOfBuilds,
		})
	}
	trend.Available = true
	trend.AverageMinutes = totalMinutes / len(usage)
	trend.PeakMinutes = peakMinutes
	return trend
}

func usageAlertMonthWindow(now time.Time, months int) (startMonth, startYear, endMonth, endYear int) {
	if months < 1 {
		months = 1
	}
	now = now.UTC()
	endAnchor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	startAnchor := endAnchor.AddDate(0, -(months - 1), 0)
	return int(startAnchor.Month()), startAnchor.Year(), int(endAnchor.Month()), endAnchor.Year()
}

func deliverUsageAlertNotifications(
	ctx context.Context,
	result *CIUsageAlertResult,
	slackWebhook, webhookURL string,
	webhookHeaders http.Header,
	notifyOn usageAlertNotifyOn,
) error {
	shouldNotify := shouldNotifyUsageAlert(result.Severity, notifyOn)
	var notifyErr error

	if strings.TrimSpace(slackWebhook) != "" {
		delivery := CIUsageAlertNotification{
			Channel:   "slack",
			Triggered: shouldNotify,
		}
		if shouldNotify {
			statusCode, err := sendUsageAlertSlackFn(ctx, slackWebhook, result)
			delivery.StatusCode = statusCode
			delivery.Delivered = err == nil
			if err != nil {
				delivery.Error = err.Error()
				notifyErr = errors.Join(notifyErr, err)
			}
		}
		result.Notifications = append(result.Notifications, delivery)
	}

	if strings.TrimSpace(webhookURL) != "" {
		delivery := CIUsageAlertNotification{
			Channel:   "webhook",
			Triggered: shouldNotify,
		}
		if shouldNotify {
			statusCode, err := sendUsageAlertWebhookFn(ctx, webhookURL, webhookHeaders, result)
			delivery.StatusCode = statusCode
			delivery.Delivered = err == nil
			if err != nil {
				delivery.Error = err.Error()
				notifyErr = errors.Join(notifyErr, err)
			}
		}
		result.Notifications = append(result.Notifications, delivery)
	}

	return notifyErr
}

func sendUsageAlertToSlack(ctx context.Context, webhookURL string, result *CIUsageAlertResult) (int, error) {
	payload := map[string]any{
		"text": fmt.Sprintf(
			"Xcode Cloud usage alert: %s (team=%s, used=%d/%dm, threshold warn=%d%% critical=%d%%)",
			result.Severity,
			result.TeamID,
			result.Plan.Used,
			result.Plan.Total,
			result.Thresholds.WarnAt,
			result.Thresholds.CriticalAt,
		),
	}
	return postUsageAlertJSON(ctx, webhookURL, nil, payload)
}

func sendUsageAlertToWebhook(
	ctx context.Context,
	webhookURL string,
	headers http.Header,
	result *CIUsageAlertResult,
) (int, error) {
	payload := map[string]any{
		"event":   "xcode_cloud_usage_alert",
		"message": result.Message,
		"result":  result,
	}
	return postUsageAlertJSON(ctx, webhookURL, headers, payload)
}

func postUsageAlertJSON(
	ctx context.Context,
	endpoint string,
	headers http.Header,
	payload any,
) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to build notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := usageAlertHTTPClientFn()
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("notification request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("notification endpoint returned status %d (%s)", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return resp.StatusCode, nil
}

func renderCIUsageAlertTable(result *CIUsageAlertResult) error {
	if result == nil {
		result = &CIUsageAlertResult{}
	}

	asc.RenderTable(
		[]string{"Field", "Value"},
		buildCIUsageAlertOverviewRows(result, false),
	)

	if result.Trend != nil {
		fmt.Println()
		if result.Trend.Available {
			fmt.Printf("Trend window: %d months (avg=%dm, peak=%dm)\n\n", result.Trend.RequestedMonths, result.Trend.AverageMinutes, result.Trend.PeakMinutes)
			asc.RenderTable(
				[]string{"Year", "Month", "Minutes", "Builds", "Usage Bar (Plan)"},
				buildCIUsageAlertTrendRows(result.Trend, result.Plan.Total),
			)
		} else {
			fmt.Printf("Trend unavailable: %s\n", valueOrNA(result.Trend.UnavailableReason))
		}
	}

	if len(result.Notifications) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Channel", "Triggered", "Delivered", "Status", "Error"},
			buildCIUsageAlertNotificationRows(result.Notifications),
		)
	}

	return nil
}

func renderCIUsageAlertMarkdown(result *CIUsageAlertResult) error {
	if result == nil {
		result = &CIUsageAlertResult{}
	}

	asc.RenderMarkdown(
		[]string{"Field", "Value"},
		buildCIUsageAlertOverviewRows(result, true),
	)

	if result.Trend != nil {
		fmt.Println()
		if result.Trend.Available {
			fmt.Printf("**Trend window:** %d months (avg=%dm, peak=%dm)\n\n", result.Trend.RequestedMonths, result.Trend.AverageMinutes, result.Trend.PeakMinutes)
			asc.RenderMarkdown(
				[]string{"Year", "Month", "Minutes", "Builds", "Usage Bar (Plan)"},
				buildCIUsageAlertTrendRows(result.Trend, result.Plan.Total),
			)
		} else {
			fmt.Printf("**Trend unavailable:** %s\n", valueOrNA(result.Trend.UnavailableReason))
		}
	}

	if len(result.Notifications) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Channel", "Triggered", "Delivered", "Status", "Error"},
			buildCIUsageAlertNotificationRows(result.Notifications),
		)
	}

	return nil
}

func buildCIUsageAlertOverviewRows(result *CIUsageAlertResult, markdown bool) [][]string {
	if result == nil {
		result = &CIUsageAlertResult{}
	}
	usageBar := formatUsageBarWithValues(result.Plan.Used, result.Plan.Total)
	severity := string(result.Severity)
	if markdown {
		severity = strings.ToUpper(severity)
	}
	return [][]string{
		{"Severity", valueOrNA(severity)},
		{"Message", valueOrNA(result.Message)},
		{"Team ID", valueOrNA(result.TeamID)},
		{"Plan", valueOrNA(result.Plan.Name)},
		{"Usage", usageBar},
		{"Used %", fmt.Sprintf("%d%%", result.Plan.UsedPercent)},
		{"Used", fmt.Sprintf("%d", result.Plan.Used)},
		{"Available", fmt.Sprintf("%d", result.Plan.Available)},
		{"Total", fmt.Sprintf("%d", result.Plan.Total)},
		{"Thresholds", fmt.Sprintf("warn=%d%% critical=%d%%", result.Thresholds.WarnAt, result.Thresholds.CriticalAt)},
		{"Reset Date", valueOrNA(result.Plan.ResetDate)},
		{"Reset Date Time", valueOrNA(result.Plan.ResetDateTime)},
		{"Manage URL", valueOrNA(result.Plan.ManageURL)},
		{"Evaluated At", valueOrNA(result.EvaluatedAt)},
	}
}

func buildCIUsageAlertTrendRows(trend *CIUsageAlertTrend, planTotal int) [][]string {
	if trend == nil {
		return nil
	}
	rows := make([][]string, 0, len(trend.Months))
	for _, month := range trend.Months {
		rows = append(rows, []string{
			fmt.Sprintf("%d", month.Year),
			fmt.Sprintf("%02d", month.Month),
			fmt.Sprintf("%d", month.Minutes),
			fmt.Sprintf("%d", month.Builds),
			formatUsageBarWithValues(month.Minutes, planTotal),
		})
	}
	return rows
}

func buildCIUsageAlertNotificationRows(notifications []CIUsageAlertNotification) [][]string {
	rows := make([][]string, 0, len(notifications))
	for _, notification := range notifications {
		statusCode := "n/a"
		if notification.StatusCode > 0 {
			statusCode = fmt.Sprintf("%d", notification.StatusCode)
		}
		rows = append(rows, []string{
			valueOrNA(notification.Channel),
			fmt.Sprintf("%t", notification.Triggered),
			fmt.Sprintf("%t", notification.Delivered),
			statusCode,
			valueOrNA(notification.Error),
		})
	}
	return rows
}
