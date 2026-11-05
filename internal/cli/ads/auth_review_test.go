package ads

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
)

func TestAuthPlatformEndpointSpecReportsMissingCommand(t *testing.T) {
	_, err := authPlatformEndpointSpec("missing", "view")
	if err == nil || !strings.Contains(err.Error(), "internal error: missing Apple Ads Platform endpoint spec for command \"missing view\"") {
		t.Fatalf("authPlatformEndpointSpec() error = %v, want a clear missing-endpoint programming error", err)
	}
}

func TestAuthPlatformEndpointSpecResolvesAuthEndpoints(t *testing.T) {
	for _, path := range [][]string{{"me", "view"}, {"acls", "list"}} {
		spec, err := authPlatformEndpointSpec(path...)
		if err != nil {
			t.Fatalf("authPlatformEndpointSpec(%q) error: %v", strings.Join(path, " "), err)
		}
		if spec.Version != appleads.APIVersionPlatformV1 {
			t.Fatalf("authPlatformEndpointSpec(%q) version = %q, want Platform API v1", strings.Join(path, " "), spec.Version)
		}
	}
}

func TestSummarizePlatformACLAccountsUsesAdAccountSelection(t *testing.T) {
	accounts, err := summarizePlatformACLAccounts(appleads.RawResponse(`{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Primary"},"roles":["Admin"]},{"adAccount":{"id":222,"orgId":987654,"name":"Secondary"},"roles":["ReadOnly"]}]}}`), "222")
	if err != nil {
		t.Fatalf("summarizePlatformACLAccounts() error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("accounts = %+v, want two ACL rows", accounts)
	}
	if accounts[0].AdAccountID != "111" || accounts[0].Active {
		t.Fatalf("first account = %+v, want account 111 inactive", accounts[0])
	}
	if accounts[1].AdAccountID != "222" || !accounts[1].Active {
		t.Fatalf("second account = %+v, want account 222 active", accounts[1])
	}
}

func TestPlatformDiscoveryRejectsMissingOrNullResults(t *testing.T) {
	for _, raw := range []appleads.RawResponse{
		appleads.RawResponse(`{}`),
		appleads.RawResponse(`{"result":null}`),
	} {
		if _, err := platformEnvelopeResult(raw); err == nil || !strings.Contains(err.Error(), "missing a non-null result") {
			t.Fatalf("platformEnvelopeResult(%s) error = %v, want missing-result error", raw, err)
		}
		if _, err := summarizePlatformACLAccounts(raw, ""); err == nil || !strings.Contains(err.Error(), "missing a non-null result") {
			t.Fatalf("summarizePlatformACLAccounts(%s) error = %v, want missing-result error", raw, err)
		}
	}
}

func TestRawPlatformRequestEndpointSpecPreservesOnlyReadOnlyQueryRetrySafety(t *testing.T) {
	query, ok := rawPlatformRequestEndpointSpec("POST", "v1/eligibilities/apps/query")
	if !ok {
		t.Fatal("rawPlatformRequestEndpointSpec() did not resolve known query endpoint")
	}
	if !query.RetrySafe {
		t.Fatal("known read-only query POST must preserve RetrySafe metadata")
	}

	for _, testCase := range []struct {
		method string
		path   string
	}{
		{method: "POST", path: "v1/campaigns"},
		{method: "PUT", path: "v1/campaigns/campaign-1"},
		{method: "DELETE", path: "v1/campaigns/campaign-1"},
	} {
		spec, ok := rawPlatformRequestEndpointSpec(testCase.method, testCase.path)
		if !ok {
			t.Fatalf("rawPlatformRequestEndpointSpec(%s %s) did not resolve known mutation", testCase.method, testCase.path)
		}
		if spec.RetrySafe {
			t.Fatalf("mutation %s %s unexpectedly marked retry-safe", testCase.method, testCase.path)
		}
	}
}
