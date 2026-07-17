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

func TestChanged441IAPCommandsRejectPositionalArgsBeforeAuth(t *testing.T) {
	isolateIAPAuth(t)

	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "list", command: IAPListCommand, args: []string{"--app", "app-1", "unexpected"}},
		{name: "view", command: IAPGetCommand, args: []string{"--id", "iap-1", "unexpected"}},
		{name: "localization update", command: IAPLocalizationsUpdateCommand, args: []string{"--localization-id", "loc-1", "--name", "Name", "unexpected"}},
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
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "unexpected argument(s): unexpected") {
				t.Fatalf("error = %v, want positional usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before positional-argument validation")
			}
		})
	}
}

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

func isolateIAPAuth(t *testing.T) {
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
