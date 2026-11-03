package cmdtest

import "testing"

// TestIAPInputValidationReturnsUsageExitCode locks the usage-error contract for
// in-app purchase flag validation: every pre-request flag check must print
// "Error: <message>" to stderr and exit with code 2, not the generic runtime
// failure code.
//
// Some diagnostics name a shorter command path than the invocation ("iap
// price-points list" for `asc iap pricing price-points list`). That spelling is
// pre-existing and deliberately preserved here so the exit-code fix stays
// stderr-neutral; changing it belongs in a separate message change.
func TestIAPInputValidationReturnsUsageExitCode(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list limit above maximum",
			args:    []string{"iap", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "iap list: --limit must be between 1 and 200",
		},
		{
			name:    "list limit below minimum",
			args:    []string{"iap", "list", "--app", "123456789", "--limit", "-1"},
			wantErr: "iap list: --limit must be between 1 and 200",
		},
		{
			name:    "list non-App-Store-Connect next",
			args:    []string{"iap", "list", "--next", "http://api.appstoreconnect.apple.com/v1/inAppPurchases"},
			wantErr: "iap list: --next must be an App Store Connect URL",
		},
		{
			name:    "list malformed next",
			args:    []string{"iap", "list", "--next", malformedNextURL},
			wantErr: "iap list: --next must be a valid URL: " + malformedNextURLParseError,
		},
		{
			name:    "offer-codes list limit above maximum",
			args:    []string{"iap", "offer-codes", "list", "--iap-id", "IAP_ID", "--limit", "201"},
			wantErr: "iap offer-codes list: --limit must be between 1 and 200",
		},
		{
			name:    "offer-codes custom-codes list limit above maximum",
			args:    []string{"iap", "offer-codes", "custom-codes", "list", "--offer-code-id", "OFFER_CODE_ID", "--limit", "201"},
			wantErr: "iap offer-codes custom-codes list: --limit must be between 1 and 200",
		},
		{
			name:    "offer-codes one-time-codes list limit above maximum",
			args:    []string{"iap", "offer-codes", "one-time-codes", "list", "--offer-code-id", "OFFER_CODE_ID", "--limit", "201"},
			wantErr: "iap offer-codes one-time-codes list: --limit must be between 1 and 200",
		},
		{
			name:    "offer-codes custom-codes update missing id",
			args:    []string{"iap", "offer-codes", "custom-codes", "update", "--active", "false"},
			wantErr: "--custom-code-id is required",
		},
		{
			name:    "offer-codes custom-codes update missing active",
			args:    []string{"iap", "offer-codes", "custom-codes", "update", "--custom-code-id", "CODE_ID"},
			wantErr: "--active is required",
		},
		{
			name:    "offer-codes custom-codes update invalid active",
			args:    []string{"iap", "offer-codes", "custom-codes", "update", "--custom-code-id", "CODE_ID", "--active", "maybe"},
			wantErr: "iap offer-codes custom-codes update: --active must be true or false",
		},
		{
			name:    "offer-codes one-time-codes update missing id",
			args:    []string{"iap", "offer-codes", "one-time-codes", "update", "--active", "false"},
			wantErr: "--one-time-code-id is required",
		},
		{
			name:    "offer-codes one-time-codes update missing active",
			args:    []string{"iap", "offer-codes", "one-time-codes", "update", "--one-time-code-id", "CODE_ID"},
			wantErr: "--active is required",
		},
		{
			name:    "offer-codes one-time-codes update invalid active",
			args:    []string{"iap", "offer-codes", "one-time-codes", "update", "--one-time-code-id", "CODE_ID", "--active", "maybe"},
			wantErr: "iap offer-codes one-time-codes update: --active must be true or false",
		},
		{
			name:    "offer-codes prices limit above maximum",
			args:    []string{"iap", "offer-codes", "prices", "--offer-code-id", "OFFER_CODE_ID", "--limit", "201"},
			wantErr: "iap offer-codes prices: --limit must be between 1 and 200",
		},
		{
			name:    "offer-codes prices invalid next",
			args:    []string{"iap", "offer-codes", "prices", "--offer-code-id", "OFFER_CODE_ID", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "iap offer-codes prices: --next must be an App Store Connect URL",
		},
		{
			name:    "offer-codes prices invalid next without offer code id",
			args:    []string{"iap", "offer-codes", "prices", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "iap offer-codes prices: --next must be an App Store Connect URL",
		},
		{
			name:    "pricing price-points list limit above maximum",
			args:    []string{"iap", "pricing", "price-points", "list", "--limit", "201"},
			wantErr: "iap price-points list: --limit must be between 1 and 200",
		},
		{
			name:    "pricing schedules manual-prices limit above maximum",
			args:    []string{"iap", "pricing", "schedules", "manual-prices", "--limit", "201"},
			wantErr: "iap pricing schedules manual-prices: --limit must be between 1 and 200",
		},
		{
			name:    "pricing schedules automatic-prices limit above maximum",
			args:    []string{"iap", "pricing", "schedules", "automatic-prices", "--limit", "201"},
			wantErr: "iap pricing schedules automatic-prices: --limit must be between 1 and 200",
		},
		{
			name:    "pricing availabilities available-territories limit above maximum",
			args:    []string{"iap", "pricing", "availabilities", "available-territories", "--limit", "201"},
			wantErr: "iap availabilities available-territories: --limit must be between 1 and 200",
		},
		{
			name:    "versions images view without image-id",
			args:    []string{"iap", "versions", "images", "view"},
			wantErr: "--image-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}
