package cmdtest

import "testing"

func TestWinBackOffersCreateInvalidPricePointExitUsage(t *testing.T) {
	assertUsageExit(t, []string{
		"subscriptions", "offers", "win-back", "create",
		"--subscription-id", "sub-1",
		"--reference-name", "spring-2026",
		"--offer-id", "OFFER-1",
		"--duration", "ONE_MONTH",
		"--offer-mode", "PAY_AS_YOU_GO",
		"--period-count", "1",
		"--eligibility-paid-months", "6",
		"--eligibility-last-subscribed-min", "3",
		"--start-date", "2026-02-01",
		"--priority", "HIGH",
		"--price", "not-a-price-point!!",
	}, "is not a subscription price point ID")
}
