package cmdtest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Resource ID flags accept the API self-link that JSON output and webhook
// deliveries carry, so links.self values pipe straight back into asc. These
// tests pin the contract through the real command tree: the request path must
// use the extracted ID, and a self-link of the wrong type is a usage failure
// before authentication or any request.

const selfLinkTestBase = "https://api.appstoreconnect.apple.com"

func TestSelfLinkWrongTypeIsUsageErrorBeforeAuthOrRequest(t *testing.T) {
	resetCmdtestState()
	setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	asctest.NewRegistry(t).Install()

	tests := []struct {
		name    string
		args    []string
		flag    string
		value   string
		wantErr string
	}{
		{
			name:    "builds info rejects apps link",
			args:    []string{"builds", "info"},
			flag:    "build-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type builds, got apps",
		},
		{
			name:    "builds info rejects encoded query delimiter",
			args:    []string{"builds", "info"},
			flag:    "build-id",
			value:   selfLinkTestBase + "/v1/builds/build-1%3Finclude=app",
			wantErr: "self-link resource ID must not contain URL delimiters",
		},
		{
			name:    "builds info rejects encoded fragment delimiter",
			args:    []string{"builds", "info"},
			flag:    "build-id",
			value:   selfLinkTestBase + "/v1/builds/build-1%23suffix",
			wantErr: "self-link resource ID must not contain URL delimiters",
		},
		{
			name:    "versions view rejects relationship path",
			args:    []string{"versions", "view"},
			flag:    "version-id",
			value:   selfLinkTestBase + "/v1/appStoreVersions/version-1/relationships/build",
			wantErr: "/v1/<type>/<id>",
		},
		{
			name:    "iap view rejects subscriptions link",
			args:    []string{"iap", "view"},
			flag:    "id",
			value:   selfLinkTestBase + "/v1/subscriptions/sub-1",
			wantErr: "expected a self-link of type inAppPurchases, got subscriptions",
		},
		{
			name:    "iap promoted-purchases view rejects subscriptions link",
			args:    []string{"iap", "promoted-purchases", "view"},
			flag:    "iap-id",
			value:   selfLinkTestBase + "/v1/subscriptions/sub-1",
			wantErr: "expected a self-link of type inAppPurchases, got subscriptions",
		},
		{
			name:    "iap versions links versions rejects version link",
			args:    []string{"iap", "versions", "links", "versions"},
			flag:    "iap-id",
			value:   selfLinkTestBase + "/v1/inAppPurchaseVersions/ver-1",
			wantErr: "expected a self-link of type inAppPurchases, got inAppPurchaseVersions",
		},
		{
			name:    "testflight groups view rejects other host",
			args:    []string{"testflight", "groups", "view"},
			flag:    "id",
			value:   "https://appstoreconnect.apple.com/v1/betaGroups/group-1",
			wantErr: "api.appstoreconnect.apple.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string(nil), test.args...), "--"+test.flag, test.value)
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run(args, "1.2.3")
				if code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			wantPrefix := fmt.Sprintf("Error: invalid value %q for flag -%s: ", test.value, test.flag)
			if !strings.HasPrefix(firstLine, wantPrefix) || !strings.Contains(firstLine, test.wantErr) {
				t.Fatalf("first stderr line = %q, want prefix %q containing %q", firstLine, wantPrefix, test.wantErr)
			}
		})
	}
}
