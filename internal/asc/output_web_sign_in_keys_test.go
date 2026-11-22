package asc

import (
	"reflect"
	"testing"
)

func TestWebSignInKeyReceiptRows(t *testing.T) {
	for _, tc := range []struct {
		name        string
		receipt     WebSignInKeyReceipt
		wantHeaders []string
		wantRow     []string
	}{
		{
			name:        "Sign in with Apple key shows its bundle",
			receipt:     WebSignInKeyReceipt{KeyID: "KEY123", Name: "Sign in", BundleID: "BUNDLE1", P8Path: "/keys/AuthKey_KEY123.p8"},
			wantHeaders: []string{"Key ID", "Name", "Bundle ID", "P8 Path"},
			wantRow:     []string{"KEY123", "Sign in", "BUNDLE1", "/keys/AuthKey_KEY123.p8"},
		},
		{
			name:        "APNs key has no bundle column, like its JSON",
			receipt:     WebSignInKeyReceipt{KeyID: "KEY456", Name: "Push", P8Path: "/keys/AuthKey_KEY456.p8"},
			wantHeaders: []string{"Key ID", "Name", "P8 Path"},
			wantRow:     []string{"KEY456", "Push", "/keys/AuthKey_KEY456.p8"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers, rows := webSignInKeyReceiptRows(&tc.receipt)
			if !reflect.DeepEqual(headers, tc.wantHeaders) || len(rows) != 1 || !reflect.DeepEqual(rows[0], tc.wantRow) {
				t.Fatalf("got %q %q, want %q [%q]", headers, rows, tc.wantHeaders, tc.wantRow)
			}
		})
	}
}
