package systemstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	statusDataURL          = "https://www.apple.com/support/systemstatus/data/developer/system_status_en_US.js"
	statusPageURL          = "https://developer.apple.com/system-status/"
	maxStatusResponseBytes = 2 << 20
	maxWatchFailures       = 3
)

type statusFeed struct {
	DRMessage string              `json:"drMessage"`
	Services  []statusFeedService `json:"services"`
}

type statusFeedService struct {
	ServiceName string             `json:"serviceName"`
	RedirectURL string             `json:"redirectUrl"`
	Events      *[]statusFeedEvent `json:"events"`
}

type statusFeedEvent struct {
	MessageID        string          `json:"messageId"`
	StatusType       string          `json:"statusType"`
	EventStatus      string          `json:"eventStatus"`
	Message          string          `json:"message"`
	DatePosted       string          `json:"datePosted"`
	StartDate        string          `json:"startDate"`
	EndDate          string          `json:"endDate"`
	EpochStartDate   int64           `json:"epochStartDate"`
	EpochEndDate     *int64          `json:"epochEndDate"`
	UsersAffected    string          `json:"usersAffected"`
	AffectedServices json.RawMessage `json:"affectedServices"`
}

// Command returns the unauthenticated Apple Developer system status command.
func Command() *ffcli.Command {
	return commandWithClient(http.DefaultClient, statusDataURL)
}

func commandWithClient(client *http.Client, endpoint string) *ffcli.Command {
	fs := flag.NewFlagSet("system-status", flag.ExitOnError)
	services := shared.BindOnceCSVFlag(fs, "service", "Filter by service name substring(s), comma-separated")
	issuesOnly := fs.Bool("issues-only", false, "Show only services with active incidents")
	watch := fs.Bool("watch", false, "Poll and emit snapshots when the selected report changes")
	pollInterval := fs.Duration("poll-interval", 30*time.Second, "Polling interval for --watch")
	maxPolls := fs.Int("max-polls", 0, "Maximum polls for --watch (0 = unlimited)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "system-status",
		ShortUsage: "asc system-status [flags]",
		ShortHelp:  "[experimental] Check Apple Developer service health.",
		LongHelp: `[experimental] Check Apple's public Developer System Status feed without authentication.

Use --service to match one or more service-name substrings. Watch mode emits
the initial snapshot, then emits again only when the selected report changes.
JSON watch output is newline-delimited and does not support --pretty.

Examples:
  asc system-status
  asc system-status --service "App Store Connect API"
  asc system-status --service "App Store Connect,TestFlight" --issues-only
  asc system-status --watch --poll-interval 30s
  asc system-status --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("system-status does not accept positional arguments")
			}
			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *maxPolls < 0 {
				return shared.UsageError("--max-polls must be greater than or equal to 0")
			}
			if *maxPolls > 0 && !*watch {
				return shared.UsageError("--max-polls requires --watch")
			}
			pollIntervalSet := false
			fs.Visit(func(current *flag.Flag) {
				if current.Name == "poll-interval" {
					pollIntervalSet = true
				}
			})
			if pollIntervalSet && !*watch {
				return shared.UsageError("--poll-interval requires --watch")
			}
			normalizedOutput, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *watch && *output.Pretty && normalizedOutput == "json" {
				return shared.UsageError("--pretty is not supported with --watch JSON output")
			}

			serviceFilterSet := false
			fs.Visit(func(current *flag.Flag) {
				if current.Name == "service" {
					serviceFilterSet = true
				}
			})
			if serviceFilterSet {
				for _, service := range strings.Split(services.String(), ",") {
					if strings.TrimSpace(service) == "" {
						return shared.UsageError("--service must not contain empty service names")
					}
				}
			}
			filters := shared.SplitCSV(services.String())

			if *watch {
				return watchDeveloperSystemStatus(ctx, client, endpoint, filters, *issuesOnly, *output.Output, *output.Pretty, *pollInterval, *maxPolls)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			report, err := fetchDeveloperSystemStatus(requestCtx, client, endpoint)
			cancel()
			if err != nil {
				return fmt.Errorf("system-status: %w", err)
			}
			report, err = selectDeveloperSystemStatus(report, filters, *issuesOnly)
			if err != nil {
				return fmt.Errorf("system-status: %w", err)
			}
			return shared.PrintOutput(report, *output.Output, *output.Pretty)
		},
	}
}

func fetchDeveloperSystemStatus(ctx context.Context, client *http.Client, endpoint string) (*asc.DeveloperSystemStatusReport, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json, application/javascript;q=0.9")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Apple Developer System Status: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("developer system status feed returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxStatusResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxStatusResponseBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxStatusResponseBytes)
	}
	return decodeDeveloperSystemStatus(body)
}

func decodeDeveloperSystemStatus(body []byte) (*asc.DeveloperSystemStatusReport, error) {
	payload, err := unwrapStatusPayload(body)
	if err != nil {
		return nil, err
	}

	var feed statusFeed
	if err := json.Unmarshal(payload, &feed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(feed.Services) == 0 {
		return nil, errors.New("response contained no services")
	}

	report := &asc.DeveloperSystemStatusReport{
		Source:   statusPageURL,
		Message:  strings.TrimSpace(feed.DRMessage),
		Services: make([]asc.DeveloperSystemStatusService, 0, len(feed.Services)),
	}
	for index, rawService := range feed.Services {
		name := strings.TrimSpace(rawService.ServiceName)
		if name == "" {
			return nil, fmt.Errorf("service %d has no name", index+1)
		}
		if rawService.Events == nil {
			return nil, fmt.Errorf("service %q is missing events", name)
		}

		service := asc.DeveloperSystemStatusService{
			Name:   name,
			Status: "operational",
			URL:    strings.TrimSpace(rawService.RedirectURL),
			Events: make([]asc.DeveloperSystemStatusEvent, 0, len(*rawService.Events)),
		}
		for eventIndex, rawEvent := range *rawService.Events {
			active, err := statusEventActive(rawEvent)
			if err != nil {
				return nil, fmt.Errorf("service %q event %d has %w", name, eventIndex+1, err)
			}
			if active {
				service.Status = "issues"
			}
			service.Events = append(service.Events, asc.DeveloperSystemStatusEvent{
				MessageID:        strings.TrimSpace(rawEvent.MessageID),
				StatusType:       strings.TrimSpace(rawEvent.StatusType),
				EventStatus:      strings.TrimSpace(rawEvent.EventStatus),
				Message:          strings.TrimSpace(rawEvent.Message),
				DatePosted:       strings.TrimSpace(rawEvent.DatePosted),
				StartDate:        strings.TrimSpace(rawEvent.StartDate),
				EndDate:          strings.TrimSpace(rawEvent.EndDate),
				EpochStartDate:   rawEvent.EpochStartDate,
				EpochEndDate:     rawEvent.EpochEndDate,
				UsersAffected:    strings.TrimSpace(rawEvent.UsersAffected),
				AffectedServices: normalizeAffectedServices(rawEvent.AffectedServices),
				Active:           active,
			})
		}
		report.Services = append(report.Services, service)
	}
	report.Summary = summarizeDeveloperSystemStatus(report.Services, report.Message)
	return report, nil
}

func unwrapStatusPayload(body []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 {
		return nil, errors.New("empty response")
	}
	if trimmed[0] == '{' {
		return trimmed, nil
	}

	const callback = "jsonCallback"
	if !bytes.HasPrefix(trimmed, []byte(callback)) {
		return nil, errors.New("unsupported response wrapper")
	}
	wrapped := bytes.TrimSpace(trimmed[len(callback):])
	if len(wrapped) < 2 || wrapped[0] != '(' {
		return nil, errors.New("unsupported response wrapper")
	}
	wrapped = bytes.TrimSpace(wrapped[1:])
	wrapped = bytes.TrimSpace(bytes.TrimSuffix(wrapped, []byte(";")))
	if len(wrapped) == 0 || wrapped[len(wrapped)-1] != ')' {
		return nil, errors.New("unsupported response wrapper")
	}
	return bytes.TrimSpace(wrapped[:len(wrapped)-1]), nil
}

func statusEventActive(event statusFeedEvent) (bool, error) {
	status := strings.TrimSpace(event.EventStatus)
	switch {
	case status == "":
		return strings.TrimSpace(event.EndDate) == "" && event.EpochEndDate == nil, nil
	case strings.EqualFold(status, "ongoing"):
		return true, nil
	case strings.EqualFold(status, "resolved"), strings.EqualFold(status, "completed"):
		return false, nil
	default:
		return false, fmt.Errorf("unknown eventStatus %q", status)
	}
}

func normalizeAffectedServices(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		return strings.Join(values, ", ")
	}
	return strings.TrimSpace(string(raw))
}

func selectDeveloperSystemStatus(report *asc.DeveloperSystemStatusReport, filters []string, issuesOnly bool) (*asc.DeveloperSystemStatusReport, error) {
	for _, filter := range filters {
		filterMatched := false
		for _, service := range report.Services {
			if matchesServiceFilters(service.Name, []string{filter}) {
				filterMatched = true
				break
			}
		}
		if !filterMatched {
			return nil, shared.UsageErrorf("no services matched --service %q", filter)
		}
	}

	matched := make([]asc.DeveloperSystemStatusService, 0, len(report.Services))
	for _, service := range report.Services {
		if matchesServiceFilters(service.Name, filters) {
			matched = append(matched, service)
		}
	}
	if len(matched) == 0 && len(filters) > 0 {
		return nil, shared.UsageErrorf("no services matched --service %q", strings.Join(filters, ","))
	}

	selected := matched
	if issuesOnly {
		selected = make([]asc.DeveloperSystemStatusService, 0, len(matched))
		for _, service := range matched {
			if service.Status == "issues" {
				selected = append(selected, service)
			}
		}
	}
	result := &asc.DeveloperSystemStatusReport{
		Source:   report.Source,
		Message:  report.Message,
		Summary:  summarizeDeveloperSystemStatus(matched, report.Message),
		Services: selected,
	}
	return result, nil
}

func matchesServiceFilters(name string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	name = strings.ToLower(name)
	for _, filter := range filters {
		if strings.Contains(name, strings.ToLower(strings.TrimSpace(filter))) {
			return true
		}
	}
	return false
}

func summarizeDeveloperSystemStatus(services []asc.DeveloperSystemStatusService, message string) asc.DeveloperSystemStatusSummary {
	summary := asc.DeveloperSystemStatusSummary{
		Status:        "operational",
		TotalServices: len(services),
	}
	for _, service := range services {
		if service.Status == "issues" {
			summary.Status = "issues"
			summary.AffectedServices++
		} else {
			summary.OperationalServices++
		}
		for _, event := range service.Events {
			if event.Active {
				summary.ActiveIncidents++
			}
		}
	}
	if strings.TrimSpace(message) != "" {
		summary.Status = "issues"
	}
	return summary
}

func watchDeveloperSystemStatus(ctx context.Context, client *http.Client, endpoint string, filters []string, issuesOnly bool, output string, pretty bool, pollInterval time.Duration, maxPolls int) error {
	previous := ""
	consecutiveFailures := 0
	for poll := 1; maxPolls == 0 || poll <= maxPolls; poll++ {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		report, err := fetchDeveloperSystemStatus(requestCtx, client, endpoint)
		cancel()
		if err != nil {
			if contextDone(ctx) {
				return nil
			}
			consecutiveFailures++
			if consecutiveFailures >= maxWatchFailures || (maxPolls > 0 && poll >= maxPolls) {
				return fmt.Errorf("system-status: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Warning: system-status poll %d failed (%v); retrying in %s\n", poll, err, pollInterval)
			if err := waitForPoll(ctx, pollInterval); err != nil {
				if contextDone(ctx) {
					return nil
				}
				return err
			}
			continue
		}
		consecutiveFailures = 0
		report, err = selectDeveloperSystemStatus(report, filters, issuesOnly)
		if err != nil {
			return fmt.Errorf("system-status: %w", err)
		}

		snapshot, err := json.Marshal(report)
		if err != nil {
			return fmt.Errorf("system-status: encode watch snapshot: %w", err)
		}
		if string(snapshot) != previous {
			if err := printWatchSnapshot(report, output, pretty, previous != ""); err != nil {
				return err
			}
			previous = string(snapshot)
		}
		if maxPolls > 0 && poll >= maxPolls {
			return nil
		}
		if err := waitForPoll(ctx, pollInterval); err != nil {
			if contextDone(ctx) {
				return nil
			}
			return err
		}
	}
	return nil
}

func printWatchSnapshot(report *asc.DeveloperSystemStatusReport, output string, pretty bool, separator bool) error {
	format := strings.ToLower(strings.TrimSpace(output))
	if format == "" {
		format = shared.DefaultOutputFormat()
	}
	if format == "json" {
		return shared.PrintOutput(report, format, pretty)
	}
	if separator {
		if format == "markdown" || format == "md" {
			fmt.Fprintln(os.Stdout, "\n---")
		} else {
			fmt.Fprintln(os.Stdout)
		}
	}
	return shared.PrintOutput(report, output, pretty)
}

func waitForPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func contextDone(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)
}
