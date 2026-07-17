package subscriptions

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestChanged441SubscriptionCommandsRejectPositionalArgsBeforeAuth(t *testing.T) {
	isolateSubscriptionAuth(t)

	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		group   bool
		pricing bool
	}{
		{name: "groups list", command: SubscriptionsGroupsListCommand, args: []string{"--app", "app-1", "unexpected"}, group: true},
		{name: "groups view", command: SubscriptionsGroupsGetCommand, args: []string{"--id", "group-1", "unexpected"}, group: true},
		{name: "list", command: SubscriptionsListCommand, args: []string{"--group-id", "group-1", "unexpected"}},
		{name: "view", command: SubscriptionsGetCommand, args: []string{"--id", "sub-1", "unexpected"}},
		{name: "price-points list", command: SubscriptionsPricePointsListCommand, args: []string{"--subscription-id", "sub-1", "unexpected"}, pricing: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			factory := &subscriptionQueryClientFactory
			if test.group {
				factory = &subscriptionGroupVersionClientFactory
			} else if test.pricing {
				factory = &subscriptionPricePointsClientFactory
			}
			previous := *factory
			*factory = func() (*asc.Client, error) {
				factoryCalled = true
				return &asc.Client{}, errors.New("poison subscription client factory called")
			}
			t.Cleanup(func() { *factory = previous })

			err := test.command().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "unexpected argument(s): unexpected") {
				t.Fatalf("error = %v, want positional usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before positional-argument validation")
			}
		})
	}
}

func isolateSubscriptionAuth(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")
	for _, key := range []string{
		"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
}
