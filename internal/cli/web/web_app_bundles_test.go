package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebAppBundleSession(t *testing.T) {
	t.Helper()
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
	})
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
}

func TestWebAppBundlesViewPrintsAppsInOrder(t *testing.T) {
	stubWebAppBundleSession(t)
	orig := getWebAppBundleFn
	t.Cleanup(func() { getWebAppBundleFn = orig })
	getWebAppBundleFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.AppBundleDetail, error) {
		if bundleID != "1501593771" {
			t.Fatalf("bundle ID = %q", bundleID)
		}
		return &webcore.AppBundleDetail{
			ID: bundleID, State: "Ready for Sale", CanSubmit: true,
			Apps: []webcore.AppBundleApp{{ID: "2", Name: "First"}, {ID: "1", Name: "Second"}},
		}, nil
	}

	for _, test := range []struct {
		output string
		check  func(t *testing.T, stdout string)
	}{
		{"json", func(t *testing.T, stdout string) {
			var got webcore.AppBundleDetail
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("stdout is not JSON: %v: %q", err, stdout)
			}
			if len(got.Apps) != 2 || got.Apps[0].ID != "2" {
				t.Fatalf("unexpected payload: %+v", got)
			}
		}},
		{"table", func(t *testing.T, stdout string) {
			first, second := strings.Index(stdout, "First"), strings.Index(stdout, "Second")
			if !strings.Contains(stdout, "Ready for Sale") || first < 0 || second < first {
				t.Fatalf("table must show state and apps in order: %q", stdout)
			}
		}},
	} {
		t.Run(test.output, func(t *testing.T) {
			cmd := WebAppBundlesViewCommand()
			if err := cmd.FlagSet.Parse([]string{"--id", "1501593771", "--output", test.output}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("Exec: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			test.check(t, stdout)
		})
	}
}

func TestWebAppBundlesListPrintsStatus(t *testing.T) {
	stubWebAppBundleSession(t)
	orig := listWebAppBundlesFn
	t.Cleanup(func() { listWebAppBundlesFn = orig })
	listWebAppBundlesFn = func(ctx context.Context, client *webcore.Client) (*webcore.AppBundlesListResponse, error) {
		return &webcore.AppBundlesListResponse{Data: []webcore.AppBundle{{
			ID: "1501593771", Name: "Colorway", Platforms: []string{"IOS", "MAC_OS"}, Status: "IN_REVIEW",
		}}}, nil
	}

	cmd := WebAppBundlesListCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec: %v", err)
		}
	})
	for _, want := range []string{"1501593771", "Colorway", "IOS,MAC_OS", "IN_REVIEW"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table missing %q: %q", want, stdout)
		}
	}
}

func TestWebAppBundlesRejectsBadUsage(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ffcli.Command
		args []string
		pos  []string
		want string
	}{
		{"view needs id", WebAppBundlesViewCommand(), nil, nil, "--id is required"},
		{"view rejects positional", WebAppBundlesViewCommand(), []string{"--id", "1"}, []string{"extra"}, "does not accept positional"},
		{"list rejects positional", WebAppBundlesListCommand(), nil, []string{"extra"}, "does not accept positional"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			var err error
			_, stderr := captureWebCommandOutput(t, func() {
				err = test.cmd.Exec(context.Background(), test.pos)
			})
			if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, test.want) {
				t.Fatalf("error = %v, stderr = %q, want usage error %q", err, stderr, test.want)
			}
		})
	}
}
