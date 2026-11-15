package cmdtest

import (
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestWebhooksListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"webhooks", "list"}, wantErr: "webhooks list: --next", deps: true},
		{args: []string{"webhooks", "deliveries", "links"}, wantErr: "webhooks deliveries links: --next", deps: true},
	})
}

func TestWebhooksListRejectsNextQueryFlagConflicts(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-2/webhooks?cursor=AQ"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "limit after next",
			args:    []string{"webhooks", "list", "--next", nextURL, "--limit", "5"},
			wantErr: "webhooks list: --next cannot be combined with --limit",
		},
		{
			name:    "out of range limit before next",
			args:    []string{"webhooks", "list", "--limit", "201", "--next", nextURL},
			wantErr: "webhooks list: --limit must be between 1 and 200",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestWebhooksListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"webhooks", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/webhooks?cursor=AQ&limit=200",
			dataType: "webhooks",
			deps:     true,
		},
		{
			args:     []string{"webhooks", "deliveries", "--webhook-id", "wh-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/deliveries?cursor=AQ&limit=200",
			dataType: "webhookDeliveries",
			deps:     true,
		},
		{
			args:     []string{"webhooks", "deliveries", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/webhooks/wh-1/relationships/deliveries?cursor=AQ&limit=200",
			dataType: "webhookDeliveries",
			deps:     true,
		},
	})
}
