package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSandboxViewByEmailStopsOnRepeatedNextURL(t *testing.T) {
	setupAuth(t)
	requests := 0
	const repeatedNext = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=repeat"
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		switch requests {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"`+repeatedNext+`"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"`+repeatedNext+`"}}`), nil
		default:
			return jsonHTTPResponse(http.StatusBadRequest, `{"errors":[{"status":"400","title":"unexpected repeated request"}]}`), nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "tester@example.com"}, "1.2.3"); code == 0 {
			t.Fatal("expected repeated pagination failure")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requests != 2 {
		t.Fatalf("expected repeated next URL to stop before third request, got %d", requests)
	}
	if !strings.Contains(stderr, "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination error, got %q", stderr)
	}
}

func TestSandboxUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "ViewMissingIDAndEmail",
			args:    []string{"sandbox", "view"},
			wantErr: "--id or --email is required",
		},
		{
			name:    "UpdateMissingIDAndEmail",
			args:    []string{"sandbox", "update", "--territory", "USA"},
			wantErr: "--id or --email is required",
		},
		{
			name:    "UpdateMissingUpdateFields",
			args:    []string{"sandbox", "update", "--id", "tester-1"},
			wantErr: "--territory, --interrupt-purchases, or --subscription-renewal-rate is required",
		},
		{
			name:    "ClearHistoryMissingConfirm",
			args:    []string{"sandbox", "clear-history", "--id", "tester-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "ClearHistoryMissingIDAndEmail",
			args:    []string{"sandbox", "clear-history", "--confirm"},
			wantErr: "--id or --email is required",
		},
	})
}
