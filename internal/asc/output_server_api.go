package asc

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
)

func printServerStatusTable(resp *StatusResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Subscription Group ID\tOriginal Transaction ID\tStatus")
	for _, item := range resp.Data {
		groupID := serverStringValue(item.SubscriptionGroupIdentifier)
		if len(item.LastTransactions) == 0 {
			fmt.Fprintf(w, "%s\t\t\n", groupID)
			continue
		}
		for _, tx := range item.LastTransactions {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				groupID,
				serverStringValue(tx.OriginalTransactionID),
				formatServerStatus(tx.Status),
			)
		}
	}
	return w.Flush()
}

func printServerStatusMarkdown(resp *StatusResponse) error {
	fmt.Fprintln(os.Stdout, "| Subscription Group ID | Original Transaction ID | Status |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		groupID := serverStringValue(item.SubscriptionGroupIdentifier)
		if len(item.LastTransactions) == 0 {
			fmt.Fprintf(os.Stdout, "| %s |  |  |\n", escapeMarkdown(groupID))
			continue
		}
		for _, tx := range item.LastTransactions {
			fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
				escapeMarkdown(groupID),
				escapeMarkdown(serverStringValue(tx.OriginalTransactionID)),
				escapeMarkdown(formatServerStatus(tx.Status)),
			)
		}
	}
	return nil
}

func printServerHistoryTable(resp *HistoryResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Transactions\tHas More\tRevision")
	fmt.Fprintf(w, "%d\t%t\t%s\n",
		len(resp.SignedTransactions),
		serverBoolValue(resp.HasMore),
		serverStringValue(resp.Revision),
	)
	return w.Flush()
}

func printServerHistoryMarkdown(resp *HistoryResponse) error {
	fmt.Fprintln(os.Stdout, "| Transactions | Has More | Revision |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %d | %t | %s |\n",
		len(resp.SignedTransactions),
		serverBoolValue(resp.HasMore),
		escapeMarkdown(serverStringValue(resp.Revision)),
	)
	return nil
}

func printServerRefundHistoryTable(resp *RefundHistoryResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Transactions\tHas More\tRevision")
	fmt.Fprintf(w, "%d\t%t\t%s\n",
		len(resp.SignedTransactions),
		serverBoolValue(resp.HasMore),
		serverStringValue(resp.Revision),
	)
	return w.Flush()
}

func printServerRefundHistoryMarkdown(resp *RefundHistoryResponse) error {
	fmt.Fprintln(os.Stdout, "| Transactions | Has More | Revision |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %d | %t | %s |\n",
		len(resp.SignedTransactions),
		serverBoolValue(resp.HasMore),
		escapeMarkdown(serverStringValue(resp.Revision)),
	)
	return nil
}

func printServerTransactionInfoTable(resp *TransactionInfoResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Signed Transaction Info")
	fmt.Fprintf(w, "%s\n", compactWhitespace(serverStringValue(resp.SignedTransactionInfo)))
	return w.Flush()
}

func printServerTransactionInfoMarkdown(resp *TransactionInfoResponse) error {
	fmt.Fprintln(os.Stdout, "| Signed Transaction Info |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(compactWhitespace(serverStringValue(resp.SignedTransactionInfo))))
	return nil
}

func printServerOrderLookupTable(resp *OrderLookupResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Status\tTransactions")
	fmt.Fprintf(w, "%s\t%d\n",
		serverIntValue(resp.Status),
		len(resp.SignedTransactions),
	)
	return w.Flush()
}

func printServerOrderLookupMarkdown(resp *OrderLookupResponse) error {
	fmt.Fprintln(os.Stdout, "| Status | Transactions |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %d |\n",
		escapeMarkdown(serverIntValue(resp.Status)),
		len(resp.SignedTransactions),
	)
	return nil
}

func printServerSendTestNotificationTable(resp *SendTestNotificationResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Test Notification Token")
	fmt.Fprintf(w, "%s\n", compactWhitespace(serverStringValue(resp.TestNotificationToken)))
	return w.Flush()
}

func printServerSendTestNotificationMarkdown(resp *SendTestNotificationResponse) error {
	fmt.Fprintln(os.Stdout, "| Test Notification Token |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(compactWhitespace(serverStringValue(resp.TestNotificationToken))))
	return nil
}

func printServerCheckTestNotificationTable(resp *CheckTestNotificationResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Signed Payload\tSend Attempts")
	fmt.Fprintf(w, "%s\t%d\n",
		compactWhitespace(serverStringValue(resp.SignedPayload)),
		len(resp.SendAttempts),
	)
	return w.Flush()
}

func printServerCheckTestNotificationMarkdown(resp *CheckTestNotificationResponse) error {
	fmt.Fprintln(os.Stdout, "| Signed Payload | Send Attempts |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %d |\n",
		escapeMarkdown(compactWhitespace(serverStringValue(resp.SignedPayload))),
		len(resp.SendAttempts),
	)
	return nil
}

func printServerNotificationHistoryTable(resp *NotificationHistoryResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Notifications\tHas More\tPagination Token")
	fmt.Fprintf(w, "%d\t%t\t%s\n",
		len(resp.NotificationHistory),
		serverBoolValue(resp.HasMore),
		serverStringValue(resp.PaginationToken),
	)
	return w.Flush()
}

func printServerNotificationHistoryMarkdown(resp *NotificationHistoryResponse) error {
	fmt.Fprintln(os.Stdout, "| Notifications | Has More | Pagination Token |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %d | %t | %s |\n",
		len(resp.NotificationHistory),
		serverBoolValue(resp.HasMore),
		escapeMarkdown(serverStringValue(resp.PaginationToken)),
	)
	return nil
}

func formatServerStatus(status *ServerStatus) string {
	if status == nil {
		return ""
	}
	switch *status {
	case ServerStatusActive:
		return "ACTIVE"
	case ServerStatusExpired:
		return "EXPIRED"
	case ServerStatusBillingRetry:
		return "BILLING_RETRY"
	case ServerStatusBillingGracePeriod:
		return "BILLING_GRACE_PERIOD"
	case ServerStatusRevoked:
		return "REVOKED"
	}
	return strconv.Itoa(int(*status))
}

func serverStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func serverBoolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func serverIntValue(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
