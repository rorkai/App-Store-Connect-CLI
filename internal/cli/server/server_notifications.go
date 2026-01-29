package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ServerNotificationsCommand returns the notifications command group.
func ServerNotificationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("notifications", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "notifications",
		ShortUsage: "asc server notifications <subcommand> [flags]",
		ShortHelp:  "Diagnostics for App Store Server Notifications.",
		LongHelp: `Diagnostics for App Store Server Notifications.

Examples:
  asc server notifications request-test
  asc server notifications status --token "TEST_TOKEN"
  asc server notifications history --start "2026-01-01T00:00:00Z" --end "2026-01-02T00:00:00Z"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ServerNotificationsRequestTestCommand(),
			ServerNotificationsStatusCommand(),
			ServerNotificationsHistoryCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ServerNotificationsRequestTestCommand requests a test notification.
func ServerNotificationsRequestTestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("request-test", flag.ExitOnError)

	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "request-test",
		ShortUsage: "asc server notifications request-test [flags]",
		ShortHelp:  "Request a test notification.",
		LongHelp: `Request a test notification.

Examples:
  asc server notifications request-test`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server notifications request-test: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.RequestTestNotification(requestCtx)
			if err != nil {
				return fmt.Errorf("server notifications request-test: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ServerNotificationsStatusCommand checks test notification status.
func ServerNotificationsStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	token := fs.String("token", "", "Test notification token")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc server notifications status --token \"TEST_TOKEN\" [flags]",
		ShortHelp:  "Check the status of a test notification.",
		LongHelp: `Check the status of a test notification.

Examples:
  asc server notifications status --token "TEST_TOKEN"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedToken := strings.TrimSpace(*token)
			if trimmedToken == "" {
				fmt.Fprintln(os.Stderr, "Error: --token is required")
				return flag.ErrHelp
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server notifications status: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetTestNotificationStatus(requestCtx, trimmedToken)
			if err != nil {
				return fmt.Errorf("server notifications status: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ServerNotificationsHistoryCommand returns notification history.
func ServerNotificationsHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("history", flag.ExitOnError)

	start := fs.String("start", "", "Start time (RFC3339 or unix ms)")
	end := fs.String("end", "", "End time (RFC3339 or unix ms)")
	transactionID := fs.String("transaction-id", "", "Filter by transaction ID")
	notificationType := fs.String("notification-type", "", "Filter by notification type: "+strings.Join(serverNotificationTypeList(), ", "))
	notificationSubtype := fs.String("notification-subtype", "", "Filter by notification subtype: "+strings.Join(serverNotificationSubtypeList(), ", "))
	next := fs.String("next", "", "Fetch next page using a pagination token")
	limit := fs.Int("limit", 0, "Maximum notifications to return")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	watch := fs.Bool("watch", false, "Poll for new notifications")
	interval := fs.Duration("interval", 10*time.Second, "Poll interval when watching")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	var onlyFailures shared.OptionalBool
	fs.Var(&onlyFailures, "only-failures", "Only failed notifications (true/false)")

	return &ffcli.Command{
		Name:       "history",
		ShortUsage: "asc server notifications history --start RFC3339 --end RFC3339 [flags]",
		ShortHelp:  "Get notification history.",
		LongHelp: `Get notification history.

Examples:
  asc server notifications history --start "2026-01-01T00:00:00Z" --end "2026-01-02T00:00:00Z"
  asc server notifications history --start "2026-01-01T00:00:00Z" --end "2026-01-31T00:00:00Z" --paginate
  asc server notifications history --start "2026-01-01T00:00:00Z" --watch --interval 30s`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit < 0 {
				return fmt.Errorf("server notifications history: --limit must be positive")
			}
			if *watch && *interval <= 0 {
				return fmt.Errorf("server notifications history: --interval must be greater than 0")
			}
			if *watch && strings.TrimSpace(*next) != "" {
				return fmt.Errorf("server notifications history: --next is not supported with --watch")
			}

			startValue, err := parseServerTimestamp(*start, "--start")
			if err != nil {
				return fmt.Errorf("server notifications history: %w", err)
			}
			if startValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --start is required")
				return flag.ErrHelp
			}

			endValue, err := parseServerTimestamp(*end, "--end")
			if err != nil {
				return fmt.Errorf("server notifications history: %w", err)
			}
			if !*watch && endValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --end is required")
				return flag.ErrHelp
			}
			if startValue != nil && endValue != nil && *startValue > *endValue {
				return fmt.Errorf("server notifications history: --start must be before --end")
			}

			notificationTypeValue, err := normalizeServerNotificationType(*notificationType)
			if err != nil {
				return fmt.Errorf("server notifications history: %w", err)
			}
			subtypeValue, err := normalizeServerNotificationSubtype(*notificationSubtype)
			if err != nil {
				return fmt.Errorf("server notifications history: %w", err)
			}
			if subtypeValue != nil && notificationTypeValue == nil {
				return fmt.Errorf("server notifications history: --notification-subtype requires --notification-type")
			}

			trimmedTransactionID := strings.TrimSpace(*transactionID)
			if trimmedTransactionID != "" && notificationTypeValue != nil {
				return fmt.Errorf("server notifications history: use --transaction-id or --notification-type, not both")
			}
			if trimmedTransactionID == "" && notificationTypeValue == nil {
				return fmt.Errorf("server notifications history: --transaction-id or --notification-type is required")
			}

			var onlyFailuresValue *bool
			if onlyFailures.IsSet() {
				value := onlyFailures.Value()
				onlyFailuresValue = &value
			}

			request := asc.NotificationHistoryRequest{
				StartDate:           startValue,
				EndDate:             endValue,
				NotificationType:    notificationTypeValue,
				NotificationSubtype: subtypeValue,
				OnlyFailures:        onlyFailuresValue,
			}
			if trimmedTransactionID != "" {
				request.TransactionID = &trimmedTransactionID
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server notifications history: %w", err)
			}

			if *watch {
				return watchNotificationHistory(ctx, client, request, *paginate, *limit, *interval, *output, *pretty)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetNotificationHistory(requestCtx, request, *next)
			if err != nil {
				return fmt.Errorf("server notifications history: failed to fetch: %w", err)
			}

			if *paginate {
				aggregated, err := paginateNotificationHistory(requestCtx, client, request, resp, *limit)
				if err != nil {
					return fmt.Errorf("server notifications history: %w", err)
				}
				return printOutput(aggregated, *output, *pretty)
			}

			if *limit > 0 && len(resp.NotificationHistory) > *limit {
				resp.NotificationHistory = resp.NotificationHistory[:*limit]
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

func paginateNotificationHistory(ctx context.Context, client *asc.ServerAPIClient, request asc.NotificationHistoryRequest, first *asc.NotificationHistoryResponse, limit int) (*asc.NotificationHistoryResponse, error) {
	aggregated := &asc.NotificationHistoryResponse{
		HasMore:             first.HasMore,
		PaginationToken:     first.PaginationToken,
		NotificationHistory: append([]asc.NotificationHistoryResponseItem{}, first.NotificationHistory...),
	}

	truncated := false
	current := first
	for hasMore(current.HasMore) && current.PaginationToken != nil && strings.TrimSpace(*current.PaginationToken) != "" {
		if limit > 0 && len(aggregated.NotificationHistory) >= limit {
			truncated = true
			break
		}
		next, err := client.GetNotificationHistory(ctx, request, *current.PaginationToken)
		if err != nil {
			return nil, err
		}
		aggregated.NotificationHistory = append(aggregated.NotificationHistory, next.NotificationHistory...)
		aggregated.PaginationToken = next.PaginationToken
		aggregated.HasMore = next.HasMore
		current = next
	}

	if limit > 0 && len(aggregated.NotificationHistory) > limit {
		aggregated.NotificationHistory = aggregated.NotificationHistory[:limit]
		truncated = true
	}

	if truncated {
		value := true
		aggregated.HasMore = &value
	} else {
		value := false
		aggregated.HasMore = &value
	}

	return aggregated, nil
}

func watchNotificationHistory(ctx context.Context, client *asc.ServerAPIClient, request asc.NotificationHistoryRequest, paginate bool, limit int, interval time.Duration, outputFormat string, pretty bool) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	currentStart := request.StartDate
	originalEndDate := request.EndDate
	for {
		endValue := originalEndDate
		if endValue == nil {
			now := time.Now().UnixMilli()
			endValue = &now
		}
		request.StartDate = currentStart
		request.EndDate = endValue

		if request.StartDate != nil && request.EndDate != nil && *request.StartDate > *request.EndDate {
			return fmt.Errorf("server notifications history: --start must be before --end")
		}

		requestCtx, cancel := contextWithTimeout(ctx)
		resp, err := client.GetNotificationHistory(requestCtx, request, "")
		if err != nil {
			cancel()
			return err
		}

		if paginate {
			resp, err = paginateNotificationHistory(requestCtx, client, request, resp, limit)
			if err != nil {
				cancel()
				return err
			}
		} else if limit > 0 && len(resp.NotificationHistory) > limit {
			resp.NotificationHistory = resp.NotificationHistory[:limit]
		}
		cancel()

		if err := printOutput(resp, outputFormat, pretty); err != nil {
			return err
		}

		if endValue != nil {
			nextStart := *endValue
			currentStart = &nextStart
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
