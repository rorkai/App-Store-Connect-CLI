package asc

// SubscriptionGracePeriodDuration represents grace period duration values.
type SubscriptionGracePeriodDuration string

const (
	SubscriptionGracePeriodDurationThreeDays       SubscriptionGracePeriodDuration = "THREE_DAYS"
	SubscriptionGracePeriodDurationSixteenDays     SubscriptionGracePeriodDuration = "SIXTEEN_DAYS"
	SubscriptionGracePeriodDurationTwentyEightDays SubscriptionGracePeriodDuration = "TWENTY_EIGHT_DAYS"
)

// SubscriptionGracePeriodRenewalType represents grace period renewal type values.
type SubscriptionGracePeriodRenewalType string

const (
	SubscriptionGracePeriodRenewalTypeAllRenewals    SubscriptionGracePeriodRenewalType = "ALL_RENEWALS"
	SubscriptionGracePeriodRenewalTypePaidToPaidOnly SubscriptionGracePeriodRenewalType = "PAID_TO_PAID_ONLY"
)

// SubscriptionGracePeriodUpdateData is the data portion of an update request.
type SubscriptionGracePeriodUpdateData struct {
	Type       ResourceType                            `json:"type"`
	ID         string                                  `json:"id"`
	Attributes SubscriptionGracePeriodUpdateAttributes `json:"attributes"`
}
