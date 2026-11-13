package iap

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestChanged441IAPVersionLimitErrorsAreUsageErrorsBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "list", command: IAPListCommand, args: []string{"--app", "app-1", "--versions-limit", "51"}},
		{name: "view", command: IAPGetCommand, args: []string{"--id", "iap-1", "--versions-limit", "51"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			previous := iapQueryClientFactory
			iapQueryClientFactory = func() (*asc.Client, error) {
				factoryCalled = true
				return &asc.Client{}, errors.New("poison IAP client factory called")
			}
			t.Cleanup(func() { iapQueryClientFactory = previous })

			err := test.command().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--versions-limit must be between 1 and 50") {
				t.Fatalf("error = %v, want versions-limit usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before versions-limit validation")
			}
		})
	}
}
