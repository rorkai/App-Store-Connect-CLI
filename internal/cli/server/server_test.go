package server

import (
	"context"
	"flag"
	"testing"
)

func TestServerStatusCommand_MissingTransactionID(t *testing.T) {
	cmd := ServerStatusCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --transaction-id is missing, got %v", err)
	}
}

func TestServerStatusCommand_InvalidStatus(t *testing.T) {
	cmd := ServerStatusCommand()
	if err := cmd.FlagSet.Parse([]string{"--transaction-id", "TX_ID", "--status", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --status, got %v", err)
	}
}

func TestServerHistoryCommand_MissingTransactionID(t *testing.T) {
	cmd := ServerHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --transaction-id is missing, got %v", err)
	}
}

func TestServerHistoryCommand_InvalidProductType(t *testing.T) {
	cmd := ServerHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--transaction-id", "TX_ID", "--product-type", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --product-type, got %v", err)
	}
}

func TestServerHistoryCommand_InvalidSort(t *testing.T) {
	cmd := ServerHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--transaction-id", "TX_ID", "--sort", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --sort, got %v", err)
	}
}

func TestServerHistoryCommand_InvalidLimit(t *testing.T) {
	cmd := ServerHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--transaction-id", "TX_ID", "--limit", "-1"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --limit, got %v", err)
	}
}

func TestServerRefundsCommand_MissingTransactionID(t *testing.T) {
	cmd := ServerRefundsCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --transaction-id is missing, got %v", err)
	}
}

func TestServerTransactionGetCommand_MissingTransactionID(t *testing.T) {
	cmd := ServerTransactionGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --transaction-id is missing, got %v", err)
	}
}

func TestServerOrderLookupCommand_MissingOrderID(t *testing.T) {
	cmd := ServerOrderLookupCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --order-id is missing, got %v", err)
	}
}

func TestServerNotificationsStatusCommand_MissingToken(t *testing.T) {
	cmd := ServerNotificationsStatusCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --token is missing, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_MissingStart(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--end", "2026-01-02T00:00:00Z", "--transaction-id", "TX_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --start is missing, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_MissingEnd(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--transaction-id", "TX_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --end is missing, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_InvalidNotificationType(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z", "--notification-type", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --notification-type, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_SubtypeWithoutType(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z", "--notification-subtype", "INITIAL_BUY"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for subtype without type, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_BothTransactionAndType(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z", "--transaction-id", "TX_ID", "--notification-type", "SUBSCRIBED"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for mutually exclusive flags, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_MissingFilter(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error when filter is missing, got %v", err)
	}
}

func TestServerNotificationsHistoryCommand_InvalidInterval(t *testing.T) {
	cmd := ServerNotificationsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--start", "2026-01-01T00:00:00Z", "--transaction-id", "TX_ID", "--watch", "--interval", "0s"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err == nil || err == flag.ErrHelp {
		t.Fatalf("expected validation error for invalid --interval, got %v", err)
	}
}
