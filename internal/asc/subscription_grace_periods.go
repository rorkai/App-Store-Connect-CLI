package asc

// SubscriptionGracePeriodAttributes describes a subscription grace period resource.
type SubscriptionGracePeriodAttributes struct {
	OptIn        bool   `json:"optIn,omitempty"`
	SandboxOptIn bool   `json:"sandboxOptIn,omitempty"`
	Duration     string `json:"duration,omitempty"`
	RenewalType  string `json:"renewalType,omitempty"`
}

// SubscriptionGracePeriodResponse is the response for subscription grace period endpoints.
type SubscriptionGracePeriodResponse = SingleResponse[SubscriptionGracePeriodAttributes]
