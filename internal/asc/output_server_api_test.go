package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_ServerStatus(t *testing.T) {
	groupID := "group-1"
	originalID := "orig-1"
	status := ServerStatusActive
	resp := &StatusResponse{
		Data: []SubscriptionGroupIdentifierItem{
			{
				SubscriptionGroupIdentifier: &groupID,
				LastTransactions: []LastTransactionsItem{
					{
						OriginalTransactionID: &originalID,
						Status:                &status,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Subscription Group ID") || !strings.Contains(output, "Original Transaction ID") {
		t.Fatalf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "group-1") || !strings.Contains(output, "orig-1") || !strings.Contains(output, "ACTIVE") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerStatus(t *testing.T) {
	groupID := "group-1"
	originalID := "orig-1"
	status := ServerStatusActive
	resp := &StatusResponse{
		Data: []SubscriptionGroupIdentifierItem{
			{
				SubscriptionGroupIdentifier: &groupID,
				LastTransactions: []LastTransactionsItem{
					{
						OriginalTransactionID: &originalID,
						Status:                &status,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Subscription Group ID | Original Transaction ID | Status |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "group-1") || !strings.Contains(output, "orig-1") || !strings.Contains(output, "ACTIVE") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerHistory(t *testing.T) {
	hasMore := true
	revision := "rev-1"
	resp := &HistoryResponse{
		HasMore:            &hasMore,
		Revision:           &revision,
		SignedTransactions: []string{"one", "two"},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Transactions") || !strings.Contains(output, "Has More") {
		t.Fatalf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "2") || !strings.Contains(output, "true") || !strings.Contains(output, "rev-1") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerHistory(t *testing.T) {
	hasMore := false
	revision := "rev-2"
	resp := &HistoryResponse{
		HasMore:            &hasMore,
		Revision:           &revision,
		SignedTransactions: []string{"one"},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Transactions | Has More | Revision |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1") || !strings.Contains(output, "false") || !strings.Contains(output, "rev-2") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerRefundHistory(t *testing.T) {
	hasMore := false
	revision := "rev-3"
	resp := &RefundHistoryResponse{
		HasMore:            &hasMore,
		Revision:           &revision,
		SignedTransactions: []string{"one"},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Transactions") || !strings.Contains(output, "Has More") {
		t.Fatalf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "1") || !strings.Contains(output, "false") || !strings.Contains(output, "rev-3") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerRefundHistory(t *testing.T) {
	hasMore := true
	revision := "rev-4"
	resp := &RefundHistoryResponse{
		HasMore:            &hasMore,
		Revision:           &revision,
		SignedTransactions: []string{"one", "two"},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Transactions | Has More | Revision |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "2") || !strings.Contains(output, "true") || !strings.Contains(output, "rev-4") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerTransactionInfo(t *testing.T) {
	info := "signed-info"
	resp := &TransactionInfoResponse{
		SignedTransactionInfo: &info,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Signed Transaction Info") || !strings.Contains(output, "signed-info") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerTransactionInfo(t *testing.T) {
	info := "signed-info"
	resp := &TransactionInfoResponse{
		SignedTransactionInfo: &info,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Signed Transaction Info |") || !strings.Contains(output, "signed-info") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerOrderLookup(t *testing.T) {
	status := 0
	resp := &OrderLookupResponse{
		Status:             &status,
		SignedTransactions: []string{"one"},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Status") || !strings.Contains(output, "Transactions") {
		t.Fatalf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "0") || !strings.Contains(output, "1") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerOrderLookup(t *testing.T) {
	status := 1
	resp := &OrderLookupResponse{
		Status:             &status,
		SignedTransactions: []string{"one", "two"},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Status | Transactions |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1") || !strings.Contains(output, "2") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerSendTestNotification(t *testing.T) {
	token := "token-1"
	resp := &SendTestNotificationResponse{
		TestNotificationToken: &token,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Test Notification Token") || !strings.Contains(output, "token-1") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerSendTestNotification(t *testing.T) {
	token := "token-2"
	resp := &SendTestNotificationResponse{
		TestNotificationToken: &token,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Test Notification Token |") || !strings.Contains(output, "token-2") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerCheckTestNotification(t *testing.T) {
	payload := "signed-payload"
	resp := &CheckTestNotificationResponse{
		SignedPayload: &payload,
		SendAttempts:  []SendAttemptItem{{}},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Signed Payload") || !strings.Contains(output, "signed-payload") {
		t.Fatalf("expected values, got: %s", output)
	}
	if !strings.Contains(output, "1") {
		t.Fatalf("expected send attempts count, got: %s", output)
	}
}

func TestPrintMarkdown_ServerCheckTestNotification(t *testing.T) {
	payload := "signed-payload"
	resp := &CheckTestNotificationResponse{
		SignedPayload: &payload,
		SendAttempts:  []SendAttemptItem{{}},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Signed Payload | Send Attempts |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "signed-payload") || !strings.Contains(output, "1") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintTable_ServerNotificationHistory(t *testing.T) {
	hasMore := true
	token := "token-1"
	resp := &NotificationHistoryResponse{
		HasMore:             &hasMore,
		PaginationToken:     &token,
		NotificationHistory: []NotificationHistoryResponseItem{{}, {}},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Notifications") || !strings.Contains(output, "Pagination Token") {
		t.Fatalf("expected headers, got: %s", output)
	}
	if !strings.Contains(output, "2") || !strings.Contains(output, "token-1") {
		t.Fatalf("expected values, got: %s", output)
	}
}

func TestPrintMarkdown_ServerNotificationHistory(t *testing.T) {
	hasMore := false
	token := "token-2"
	resp := &NotificationHistoryResponse{
		HasMore:             &hasMore,
		PaginationToken:     &token,
		NotificationHistory: []NotificationHistoryResponseItem{{}},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Notifications | Has More | Pagination Token |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1") || !strings.Contains(output, "token-2") {
		t.Fatalf("expected values, got: %s", output)
	}
}
