package asc

import (
	"fmt"
	"strings"
)

// ServerEnvironment is the App Store Server API environment.
type ServerEnvironment string

const (
	ServerEnvironmentProduction ServerEnvironment = "PRODUCTION"
	ServerEnvironmentSandbox    ServerEnvironment = "SANDBOX"
)

// ParseServerEnvironment parses a server API environment value.
func ParseServerEnvironment(value string) (ServerEnvironment, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	switch trimmed {
	case string(ServerEnvironmentProduction):
		return ServerEnvironmentProduction, nil
	case string(ServerEnvironmentSandbox):
		return ServerEnvironmentSandbox, nil
	}
	if trimmed == "" {
		return "", fmt.Errorf("server env is required")
	}
	return "", fmt.Errorf("server env must be SANDBOX or PRODUCTION")
}

// ServerStatus represents the subscription status.
type ServerStatus int

const (
	ServerStatusActive             ServerStatus = 1
	ServerStatusExpired            ServerStatus = 2
	ServerStatusBillingRetry       ServerStatus = 3
	ServerStatusBillingGracePeriod ServerStatus = 4
	ServerStatusRevoked            ServerStatus = 5
)

// ServerProductType represents the product type filter.
type ServerProductType string

const (
	ServerProductTypeAutoRenewable ServerProductType = "AUTO_RENEWABLE"
	ServerProductTypeConsumable    ServerProductType = "CONSUMABLE"
	ServerProductTypeNonConsumable ServerProductType = "NON_CONSUMABLE"
	ServerProductTypeNonRenewable  ServerProductType = "NON_RENEWABLE"
)

// ServerOrder represents sort order.
type ServerOrder string

const (
	ServerOrderAscending  ServerOrder = "ASCENDING"
	ServerOrderDescending ServerOrder = "DESCENDING"
)

// ServerInAppOwnershipType represents ownership type filter.
type ServerInAppOwnershipType string

const (
	ServerInAppOwnershipTypePurchased    ServerInAppOwnershipType = "PURCHASED"
	ServerInAppOwnershipTypeFamilyShared ServerInAppOwnershipType = "FAMILY_SHARED"
)

// ServerNotificationType represents notification types for history filtering.
type ServerNotificationType string

const (
	ServerNotificationTypeConsumptionRequest   ServerNotificationType = "CONSUMPTION_REQUEST"
	ServerNotificationTypeDidChangeRenewalPref ServerNotificationType = "DID_CHANGE_RENEWAL_PREF"
	ServerNotificationTypeDidChangeRenewal     ServerNotificationType = "DID_CHANGE_RENEWAL_STATUS"
	ServerNotificationTypeDidFailToRenew       ServerNotificationType = "DID_FAIL_TO_RENEW"
	ServerNotificationTypeDidRenew             ServerNotificationType = "DID_RENEW"
	ServerNotificationTypeExpired              ServerNotificationType = "EXPIRED"
	ServerNotificationTypeExternalPurchase     ServerNotificationType = "EXTERNAL_PURCHASE_TOKEN"
	ServerNotificationTypeGracePeriodExpired   ServerNotificationType = "GRACE_PERIOD_EXPIRED"
	ServerNotificationTypeOfferRedeemed        ServerNotificationType = "OFFER_REDEEMED"
	ServerNotificationTypeOneTimeCharge        ServerNotificationType = "ONE_TIME_CHARGE"
	ServerNotificationTypePriceIncrease        ServerNotificationType = "PRICE_INCREASE"
	ServerNotificationTypeRefund               ServerNotificationType = "REFUND"
	ServerNotificationTypeRefundDeclined       ServerNotificationType = "REFUND_DECLINED"
	ServerNotificationTypeRefundReversed       ServerNotificationType = "REFUND_REVERSED"
	ServerNotificationTypeRenewalExtended      ServerNotificationType = "RENEWAL_EXTENDED"
	ServerNotificationTypeRenewalExtension     ServerNotificationType = "RENEWAL_EXTENSION"
	ServerNotificationTypeRevoke               ServerNotificationType = "REVOKE"
	ServerNotificationTypeSubscribed           ServerNotificationType = "SUBSCRIBED"
	ServerNotificationTypeTest                 ServerNotificationType = "TEST"
)

// ServerNotificationSubtype represents notification subtype values.
type ServerNotificationSubtype string

const (
	ServerNotificationSubtypeInitialBuy        ServerNotificationSubtype = "INITIAL_BUY"
	ServerNotificationSubtypeResubscribe       ServerNotificationSubtype = "RESUBSCRIBE"
	ServerNotificationSubtypeDowngrade         ServerNotificationSubtype = "DOWNGRADE"
	ServerNotificationSubtypeUpgrade           ServerNotificationSubtype = "UPGRADE"
	ServerNotificationSubtypeAutoRenewEnabled  ServerNotificationSubtype = "AUTO_RENEW_ENABLED"
	ServerNotificationSubtypeAutoRenewDisabled ServerNotificationSubtype = "AUTO_RENEW_DISABLED"
	ServerNotificationSubtypeVoluntary         ServerNotificationSubtype = "VOLUNTARY"
	ServerNotificationSubtypeBillingRetry      ServerNotificationSubtype = "BILLING_RETRY"
	ServerNotificationSubtypePriceIncrease     ServerNotificationSubtype = "PRICE_INCREASE"
	ServerNotificationSubtypeGracePeriod       ServerNotificationSubtype = "GRACE_PERIOD"
	ServerNotificationSubtypePending           ServerNotificationSubtype = "PENDING"
	ServerNotificationSubtypeAccepted          ServerNotificationSubtype = "ACCEPTED"
	ServerNotificationSubtypeBillingRecovery   ServerNotificationSubtype = "BILLING_RECOVERY"
	ServerNotificationSubtypeProductNotForSale ServerNotificationSubtype = "PRODUCT_NOT_FOR_SALE"
	ServerNotificationSubtypeSummary           ServerNotificationSubtype = "SUMMARY"
	ServerNotificationSubtypeFailure           ServerNotificationSubtype = "FAILURE"
	ServerNotificationSubtypeUnreported        ServerNotificationSubtype = "UNREPORTED"
)

// StatusResponse contains subscription status information.
type StatusResponse struct {
	AppAppleID  *int64                            `json:"appAppleId,omitempty"`
	BundleID    *string                           `json:"bundleId,omitempty"`
	Data        []SubscriptionGroupIdentifierItem `json:"data,omitempty"`
	Environment *ServerEnvironment                `json:"environment,omitempty"`
}

// SubscriptionGroupIdentifierItem contains group status details.
type SubscriptionGroupIdentifierItem struct {
	LastTransactions            []LastTransactionsItem `json:"lastTransactions,omitempty"`
	SubscriptionGroupIdentifier *string                `json:"subscriptionGroupIdentifier,omitempty"`
}

// LastTransactionsItem contains the latest signed transaction and renewal info.
type LastTransactionsItem struct {
	OriginalTransactionID *string       `json:"originalTransactionId,omitempty"`
	SignedRenewalInfo     *string       `json:"signedRenewalInfo,omitempty"`
	SignedTransactionInfo *string       `json:"signedTransactionInfo,omitempty"`
	Status                *ServerStatus `json:"status,omitempty"`
}

// HistoryResponse contains transaction history.
type HistoryResponse struct {
	AppAppleID         *int64             `json:"appAppleId,omitempty"`
	BundleID           *string            `json:"bundleId,omitempty"`
	Environment        *ServerEnvironment `json:"environment,omitempty"`
	HasMore            *bool              `json:"hasMore,omitempty"`
	Revision           *string            `json:"revision,omitempty"`
	SignedTransactions []string           `json:"signedTransactions,omitempty"`
}

// RefundHistoryResponse contains refund history.
type RefundHistoryResponse struct {
	HasMore            *bool    `json:"hasMore,omitempty"`
	Revision           *string  `json:"revision,omitempty"`
	SignedTransactions []string `json:"signedTransactions,omitempty"`
}

// TransactionInfoResponse contains a signed transaction info payload.
type TransactionInfoResponse struct {
	SignedTransactionInfo *string `json:"signedTransactionInfo,omitempty"`
}

// OrderLookupResponse contains signed transactions for an order lookup.
type OrderLookupResponse struct {
	SignedTransactions []string `json:"signedTransactions,omitempty"`
	Status             *int     `json:"status,omitempty"`
}

// SendTestNotificationResponse contains a test notification token.
type SendTestNotificationResponse struct {
	TestNotificationToken *string `json:"testNotificationToken,omitempty"`
}

// CheckTestNotificationResponse contains test notification status details.
type CheckTestNotificationResponse struct {
	SendAttempts  []SendAttemptItem `json:"sendAttempts,omitempty"`
	SignedPayload *string           `json:"signedPayload,omitempty"`
}

// NotificationHistoryResponse contains notification history entries.
type NotificationHistoryResponse struct {
	HasMore             *bool                             `json:"hasMore,omitempty"`
	NotificationHistory []NotificationHistoryResponseItem `json:"notificationHistory,omitempty"`
	PaginationToken     *string                           `json:"paginationToken,omitempty"`
}

// NotificationHistoryResponseItem contains a signed payload and send attempts.
type NotificationHistoryResponseItem struct {
	SendAttempts  []SendAttemptItem `json:"sendAttempts,omitempty"`
	SignedPayload *string           `json:"signedPayload,omitempty"`
}

// SendAttemptItem describes a send attempt for server notifications.
type SendAttemptItem struct {
	AttemptDate       *int64  `json:"attemptDate,omitempty"`
	SendAttemptResult *string `json:"sendAttemptResult,omitempty"`
}

// TransactionHistoryRequest defines filters for transaction history.
type TransactionHistoryRequest struct {
	StartDate                    *int64                    `json:"startDate,omitempty"`
	EndDate                      *int64                    `json:"endDate,omitempty"`
	ProductIDs                   []string                  `json:"productIds,omitempty"`
	ProductTypes                 []ServerProductType       `json:"productTypes,omitempty"`
	SubscriptionGroupIdentifiers []string                  `json:"subscriptionGroupIdentifiers,omitempty"`
	InAppOwnershipType           *ServerInAppOwnershipType `json:"inAppOwnershipType,omitempty"`
	Revoked                      *bool                     `json:"revoked,omitempty"`
	Sort                         *ServerOrder              `json:"sort,omitempty"`
}

// NotificationHistoryRequest defines filters for notification history.
type NotificationHistoryRequest struct {
	StartDate           *int64                     `json:"startDate,omitempty"`
	EndDate             *int64                     `json:"endDate,omitempty"`
	NotificationType    *ServerNotificationType    `json:"notificationType,omitempty"`
	NotificationSubtype *ServerNotificationSubtype `json:"notificationSubtype,omitempty"`
	OnlyFailures        *bool                      `json:"onlyFailures,omitempty"`
	TransactionID       *string                    `json:"transactionId,omitempty"`
}
