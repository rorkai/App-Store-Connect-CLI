package cmdtest

import "testing"

func TestChanged441CommandValidationReturnsUsageExit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "iap list positional", args: []string{"iap", "list", "--app", "app-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "iap view positional", args: []string{"iap", "view", "--id", "iap-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "iap localization update positional", args: []string{"iap", "localizations", "update", "--localization-id", "loc-1", "--name", "Name", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "subscription groups list positional", args: []string{"subscriptions", "groups", "list", "--app", "app-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "subscription groups view positional", args: []string{"subscriptions", "groups", "view", "--id", "group-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "subscriptions list positional", args: []string{"subscriptions", "list", "--group-id", "group-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "subscriptions view positional", args: []string{"subscriptions", "view", "--id", "sub-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "subscription price-points list positional", args: []string{"subscriptions", "pricing", "price-points", "list", "--subscription-id", "sub-1", "unexpected"}, wantErr: "unexpected argument(s): unexpected"},
		{name: "iap list versions limit", args: []string{"iap", "list", "--app", "app-1", "--versions-limit", "51"}, wantErr: "iap list: --versions-limit must be between 1 and 50"},
		{name: "iap view versions limit", args: []string{"iap", "view", "--id", "iap-1", "--versions-limit", "51"}, wantErr: "iap view: --versions-limit must be between 1 and 50"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}
