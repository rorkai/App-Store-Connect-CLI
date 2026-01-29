package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func serverStatusList() []string {
	return []string{"ACTIVE", "EXPIRED", "BILLING_RETRY", "BILLING_GRACE_PERIOD", "REVOKED"}
}

func normalizeServerStatuses(value string) ([]asc.ServerStatus, error) {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}
	mapping := map[string]asc.ServerStatus{
		"ACTIVE":               asc.ServerStatusActive,
		"EXPIRED":              asc.ServerStatusExpired,
		"BILLING_RETRY":        asc.ServerStatusBillingRetry,
		"BILLING_GRACE_PERIOD": asc.ServerStatusBillingGracePeriod,
		"REVOKED":              asc.ServerStatusRevoked,
	}
	statuses := make([]asc.ServerStatus, 0, len(values))
	for _, item := range values {
		key := strings.ToUpper(strings.TrimSpace(item))
		status, ok := mapping[key]
		if !ok {
			return nil, fmt.Errorf("--status must be one of: %s", strings.Join(serverStatusList(), ", "))
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func serverProductTypeList() []string {
	return []string{"AUTO_RENEWABLE", "CONSUMABLE", "NON_CONSUMABLE", "NON_RENEWABLE"}
}

func normalizeServerProductTypes(value string) ([]asc.ServerProductType, error) {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}
	mapping := map[string]asc.ServerProductType{
		"AUTO_RENEWABLE": asc.ServerProductTypeAutoRenewable,
		"CONSUMABLE":     asc.ServerProductTypeConsumable,
		"NON_CONSUMABLE": asc.ServerProductTypeNonConsumable,
		"NON_RENEWABLE":  asc.ServerProductTypeNonRenewable,
	}
	types := make([]asc.ServerProductType, 0, len(values))
	for _, item := range values {
		key := strings.ToUpper(strings.TrimSpace(item))
		productType, ok := mapping[key]
		if !ok {
			return nil, fmt.Errorf("--product-type must be one of: %s", strings.Join(serverProductTypeList(), ", "))
		}
		types = append(types, productType)
	}
	return types, nil
}

func serverOrderList() []string {
	return []string{"ASCENDING", "DESCENDING"}
}

func normalizeServerOrder(value string) (*asc.ServerOrder, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil, nil
	}
	switch trimmed {
	case string(asc.ServerOrderAscending):
		order := asc.ServerOrderAscending
		return &order, nil
	case string(asc.ServerOrderDescending):
		order := asc.ServerOrderDescending
		return &order, nil
	}
	return nil, fmt.Errorf("--sort must be one of: %s", strings.Join(serverOrderList(), ", "))
}

func serverOwnershipTypeList() []string {
	return []string{"PURCHASED", "FAMILY_SHARED"}
}

func normalizeServerOwnershipType(value string) (*asc.ServerInAppOwnershipType, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil, nil
	}
	switch trimmed {
	case string(asc.ServerInAppOwnershipTypePurchased):
		ownership := asc.ServerInAppOwnershipTypePurchased
		return &ownership, nil
	case string(asc.ServerInAppOwnershipTypeFamilyShared):
		ownership := asc.ServerInAppOwnershipTypeFamilyShared
		return &ownership, nil
	}
	return nil, fmt.Errorf("--ownership-type must be one of: %s", strings.Join(serverOwnershipTypeList(), ", "))
}

func serverNotificationTypeList() []string {
	return []string{
		"CONSUMPTION_REQUEST",
		"DID_CHANGE_RENEWAL_PREF",
		"DID_CHANGE_RENEWAL_STATUS",
		"DID_FAIL_TO_RENEW",
		"DID_RENEW",
		"EXPIRED",
		"EXTERNAL_PURCHASE_TOKEN",
		"GRACE_PERIOD_EXPIRED",
		"OFFER_REDEEMED",
		"ONE_TIME_CHARGE",
		"PRICE_INCREASE",
		"REFUND",
		"REFUND_DECLINED",
		"REFUND_REVERSED",
		"RENEWAL_EXTENDED",
		"RENEWAL_EXTENSION",
		"REVOKE",
		"SUBSCRIBED",
		"TEST",
	}
}

func normalizeServerNotificationType(value string) (*asc.ServerNotificationType, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil, nil
	}
	mapping := map[string]asc.ServerNotificationType{
		"CONSUMPTION_REQUEST":       asc.ServerNotificationTypeConsumptionRequest,
		"DID_CHANGE_RENEWAL_PREF":   asc.ServerNotificationTypeDidChangeRenewalPref,
		"DID_CHANGE_RENEWAL_STATUS": asc.ServerNotificationTypeDidChangeRenewal,
		"DID_FAIL_TO_RENEW":         asc.ServerNotificationTypeDidFailToRenew,
		"DID_RENEW":                 asc.ServerNotificationTypeDidRenew,
		"EXPIRED":                   asc.ServerNotificationTypeExpired,
		"EXTERNAL_PURCHASE_TOKEN":   asc.ServerNotificationTypeExternalPurchase,
		"GRACE_PERIOD_EXPIRED":      asc.ServerNotificationTypeGracePeriodExpired,
		"OFFER_REDEEMED":            asc.ServerNotificationTypeOfferRedeemed,
		"ONE_TIME_CHARGE":           asc.ServerNotificationTypeOneTimeCharge,
		"PRICE_INCREASE":            asc.ServerNotificationTypePriceIncrease,
		"REFUND":                    asc.ServerNotificationTypeRefund,
		"REFUND_DECLINED":           asc.ServerNotificationTypeRefundDeclined,
		"REFUND_REVERSED":           asc.ServerNotificationTypeRefundReversed,
		"RENEWAL_EXTENDED":          asc.ServerNotificationTypeRenewalExtended,
		"RENEWAL_EXTENSION":         asc.ServerNotificationTypeRenewalExtension,
		"REVOKE":                    asc.ServerNotificationTypeRevoke,
		"SUBSCRIBED":                asc.ServerNotificationTypeSubscribed,
		"TEST":                      asc.ServerNotificationTypeTest,
	}
	notificationType, ok := mapping[trimmed]
	if !ok {
		return nil, fmt.Errorf("--notification-type must be one of: %s", strings.Join(serverNotificationTypeList(), ", "))
	}
	return &notificationType, nil
}

func serverNotificationSubtypeList() []string {
	return []string{
		"INITIAL_BUY",
		"RESUBSCRIBE",
		"DOWNGRADE",
		"UPGRADE",
		"AUTO_RENEW_ENABLED",
		"AUTO_RENEW_DISABLED",
		"VOLUNTARY",
		"BILLING_RETRY",
		"PRICE_INCREASE",
		"GRACE_PERIOD",
		"PENDING",
		"ACCEPTED",
		"BILLING_RECOVERY",
		"PRODUCT_NOT_FOR_SALE",
		"SUMMARY",
		"FAILURE",
		"UNREPORTED",
	}
}

func normalizeServerNotificationSubtype(value string) (*asc.ServerNotificationSubtype, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil, nil
	}
	mapping := map[string]asc.ServerNotificationSubtype{
		"INITIAL_BUY":          asc.ServerNotificationSubtypeInitialBuy,
		"RESUBSCRIBE":          asc.ServerNotificationSubtypeResubscribe,
		"DOWNGRADE":            asc.ServerNotificationSubtypeDowngrade,
		"UPGRADE":              asc.ServerNotificationSubtypeUpgrade,
		"AUTO_RENEW_ENABLED":   asc.ServerNotificationSubtypeAutoRenewEnabled,
		"AUTO_RENEW_DISABLED":  asc.ServerNotificationSubtypeAutoRenewDisabled,
		"VOLUNTARY":            asc.ServerNotificationSubtypeVoluntary,
		"BILLING_RETRY":        asc.ServerNotificationSubtypeBillingRetry,
		"PRICE_INCREASE":       asc.ServerNotificationSubtypePriceIncrease,
		"GRACE_PERIOD":         asc.ServerNotificationSubtypeGracePeriod,
		"PENDING":              asc.ServerNotificationSubtypePending,
		"ACCEPTED":             asc.ServerNotificationSubtypeAccepted,
		"BILLING_RECOVERY":     asc.ServerNotificationSubtypeBillingRecovery,
		"PRODUCT_NOT_FOR_SALE": asc.ServerNotificationSubtypeProductNotForSale,
		"SUMMARY":              asc.ServerNotificationSubtypeSummary,
		"FAILURE":              asc.ServerNotificationSubtypeFailure,
		"UNREPORTED":           asc.ServerNotificationSubtypeUnreported,
	}
	subtype, ok := mapping[trimmed]
	if !ok {
		return nil, fmt.Errorf("--notification-subtype must be one of: %s", strings.Join(serverNotificationSubtypeList(), ", "))
	}
	return &subtype, nil
}

func parseServerTimestamp(value, flagName string) (*int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	if digitsOnly(trimmed) {
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must be a positive unix millisecond timestamp", flagName)
		}
		return &parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		ms := parsed.UnixMilli()
		return &ms, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		ms := parsed.UnixMilli()
		return &ms, nil
	}
	return nil, fmt.Errorf("%s must be RFC3339 or a unix millisecond timestamp", flagName)
}

func digitsOnly(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
