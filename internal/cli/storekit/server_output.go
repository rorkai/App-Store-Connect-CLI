package storekit

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

type serverReadFlags struct {
	common commonFlags
	output shared.OutputFlags
	decode *bool // nil for responses without signed JWS fields
}

func bindServerReadFlags(fs *flag.FlagSet, signed bool) serverReadFlags {
	flags := serverReadFlags{common: bindCommonFlags(fs), output: shared.BindOutputFlags(fs)}
	if signed {
		flags.decode = fs.Bool("decode", false, "Add <field>Decoded siblings with signed JWS payloads decoded WITHOUT signature verification (JSON only)")
	}
	return flags
}

type tableBuilder func(json.RawMessage) ([]string, [][]string, error)

func printServerResponse(raw json.RawMessage, flags serverReadFlags, table tableBuilder) error {
	signed := flags.decode != nil
	if signed && *flags.decode {
		decoded, err := storekitapi.AddDecodedJWSPayloads(raw)
		if err != nil {
			return err
		}
		raw = decoded
	}
	var headers []string
	var rows [][]string
	tableOutput := shared.NormalizeOutputFormat(*flags.output.Output) != "json"
	if tableOutput {
		var err error
		if headers, rows, err = table(raw); err != nil {
			return err
		}
	}
	if signed && (*flags.decode || tableOutput) {
		fmt.Fprintln(os.Stderr, "Warning: decoded JWS payloads are not signature-verified")
	}
	return printOutput(raw, *flags.output.Output, *flags.output.Pretty, headers, rows)
}

func printServerPage(raw json.RawMessage, flags serverReadFlags, table tableBuilder, paginate bool, cursor storekitapi.PageCursor, tokenFlag string) error {
	if err := printServerResponse(raw, flags, table); err != nil {
		return err
	}
	if paginate {
		return nil
	}
	if hasMore, token, err := cursor.HasMore(raw); err == nil && hasMore && token != "" {
		fmt.Fprintf(os.Stderr, "Warning: more pages exist (use --paginate or %s %s)\n", tokenFlag, token)
	}
	return nil
}

type transactionClaims struct {
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ProductID             string `json:"productId"`
	Type                  string `json:"type"`
	PurchaseDate          int64  `json:"purchaseDate"`
	ExpiresDate           int64  `json:"expiresDate"`
	RevocationDate        int64  `json:"revocationDate"`
	InAppOwnershipType    string `json:"inAppOwnershipType"`
	RevocationType        string `json:"revocationType"`
}

var transactionHeaders = []string{"Transaction ID", "Original Transaction ID", "Product ID", "Type", "Purchase Date", "Expires Date", "Revocation Date", "Ownership", "Revocation Type"}

func transactionInfoTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		SignedTransactionInfo string `json:"signedTransactionInfo"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	rows, err := transactionRows([]string{response.SignedTransactionInfo})
	return transactionHeaders, rows, err
}

func signedTransactionsTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		SignedTransactions []string `json:"signedTransactions"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	rows, err := transactionRows(response.SignedTransactions)
	return transactionHeaders, rows, err
}

func transactionRows(tokens []string) ([][]string, error) {
	rows := make([][]string, 0, len(tokens))
	for _, token := range tokens {
		var claims transactionClaims
		if err := decodeClaims(token, &claims); err != nil {
			return nil, err
		}
		rows = append(rows, []string{
			claims.TransactionID, claims.OriginalTransactionID, claims.ProductID, claims.Type,
			formatMillis(claims.PurchaseDate), formatMillis(claims.ExpiresDate), formatMillis(claims.RevocationDate),
			claims.InAppOwnershipType, claims.RevocationType,
		})
	}
	return rows, nil
}

func orderLookupTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		Status             int      `json:"status"`
		SignedTransactions []string `json:"signedTransactions"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	transactions, err := transactionRows(response.SignedTransactions)
	if err != nil {
		return nil, nil, err
	}
	if len(transactions) == 0 {
		transactions = [][]string{make([]string, len(transactionHeaders))}
	}
	rows := make([][]string, 0, len(transactions))
	for _, row := range transactions {
		rows = append(rows, append([]string{strconv.Itoa(response.Status)}, row...))
	}
	return append([]string{"Order Status"}, transactionHeaders...), rows, nil
}

func subscriptionStatusTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		Data []struct {
			SubscriptionGroupIdentifier string `json:"subscriptionGroupIdentifier"`
			LastTransactions            []struct {
				OriginalTransactionID string `json:"originalTransactionId"`
				Status                int    `json:"status"`
				SignedTransactionInfo string `json:"signedTransactionInfo"`
				SignedRenewalInfo     string `json:"signedRenewalInfo"`
			} `json:"lastTransactions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	var rows [][]string
	for _, group := range response.Data {
		for _, item := range group.LastTransactions {
			var transaction transactionClaims
			if err := decodeClaims(item.SignedTransactionInfo, &transaction); err != nil {
				return nil, nil, err
			}
			var renewal struct {
				AutoRenewStatus *int `json:"autoRenewStatus"`
			}
			if err := decodeClaims(item.SignedRenewalInfo, &renewal); err != nil {
				return nil, nil, err
			}
			autoRenew := ""
			if renewal.AutoRenewStatus != nil {
				autoRenew = boolString(*renewal.AutoRenewStatus == 1)
			}
			rows = append(rows, []string{
				group.SubscriptionGroupIdentifier, item.OriginalTransactionID, subscriptionStatusName(item.Status),
				transaction.ProductID, formatMillis(transaction.ExpiresDate), autoRenew,
			})
		}
	}
	return []string{"Subscription Group", "Original Transaction ID", "Status", "Product ID", "Expires Date", "Auto Renew"}, rows, nil
}

func subscriptionStatusName(status int) string {
	names := map[int]string{1: "ACTIVE", 2: "EXPIRED", 3: "BILLING_RETRY", 4: "BILLING_GRACE_PERIOD", 5: "REVOKED"}
	if name, ok := names[status]; ok {
		return name
	}
	return strconv.Itoa(status)
}

type sendAttempt struct {
	AttemptDate       int64  `json:"attemptDate"`
	SendAttemptResult string `json:"sendAttemptResult"`
}

func notificationHistoryTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		NotificationHistory []struct {
			SignedPayload string        `json:"signedPayload"`
			SendAttempts  []sendAttempt `json:"sendAttempts"`
		} `json:"notificationHistory"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	rows := make([][]string, 0, len(response.NotificationHistory))
	for _, item := range response.NotificationHistory {
		var payload struct {
			NotificationUUID string `json:"notificationUUID"`
			NotificationType string `json:"notificationType"`
			Subtype          string `json:"subtype"`
			SignedDate       int64  `json:"signedDate"`
		}
		if err := decodeClaims(item.SignedPayload, &payload); err != nil {
			return nil, nil, err
		}
		lastResult := ""
		if len(item.SendAttempts) > 0 {
			lastResult = item.SendAttempts[len(item.SendAttempts)-1].SendAttemptResult
		}
		rows = append(rows, []string{payload.NotificationUUID, payload.NotificationType, payload.Subtype, formatMillis(payload.SignedDate), lastResult})
	}
	return []string{"Notification UUID", "Type", "Subtype", "Signed Date", "Last Attempt Result"}, rows, nil
}

func testNotificationStatusTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		SignedPayload string        `json:"signedPayload"`
		SendAttempts  []sendAttempt `json:"sendAttempts"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	var payload struct {
		NotificationType string `json:"notificationType"`
	}
	if err := decodeClaims(response.SignedPayload, &payload); err != nil {
		return nil, nil, err
	}
	rows := make([][]string, 0, len(response.SendAttempts))
	for _, attempt := range response.SendAttempts {
		rows = append(rows, []string{payload.NotificationType, formatMillis(attempt.AttemptDate), attempt.SendAttemptResult})
	}
	return []string{"Notification Type", "Attempt Date", "Result"}, rows, nil
}

func testNotificationTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		Token string `json:"testNotificationToken"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	return []string{"Test Notification Token"}, [][]string{{response.Token}}, nil
}

func customerGroupsTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		Groups []struct {
			GroupID   string `json:"groupId"`
			GroupType string `json:"groupType"`
			Roles     []struct {
				ProductID string `json:"productId"`
				Role      string `json:"role"`
			} `json:"roles"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	var rows [][]string
	for _, group := range response.Groups {
		if len(group.Roles) == 0 {
			rows = append(rows, []string{group.GroupID, group.GroupType, "", ""})
		}
		for _, role := range group.Roles {
			rows = append(rows, []string{group.GroupID, group.GroupType, role.ProductID, role.Role})
		}
	}
	return []string{"Group ID", "Group Type", "Product ID", "Role"}, rows, nil
}

func groupMembersTable(raw json.RawMessage) ([]string, [][]string, error) {
	var response struct {
		Members []struct {
			AppTransactionID string `json:"appTransactionId"`
		} `json:"members"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	rows := make([][]string, 0, len(response.Members))
	for _, member := range response.Members {
		rows = append(rows, []string{member.AppTransactionID})
	}
	return []string{"App Transaction ID"}, rows, nil
}

func decodeClaims(token string, target any) error {
	if token == "" {
		return nil
	}
	payload, err := storekitapi.DecodeJWSPayloadUnverified(token)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode JWS payload: %w", err)
	}
	return nil
}

func formatMillis(milliseconds int64) string {
	if milliseconds == 0 {
		return ""
	}
	return time.UnixMilli(milliseconds).UTC().Format(time.RFC3339)
}
