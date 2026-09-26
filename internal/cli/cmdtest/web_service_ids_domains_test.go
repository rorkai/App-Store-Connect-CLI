package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebServiceIDsDomainsSetValidatesBeforeSession(t *testing.T) {
	setCmdtestHome(t)
	t.Cleanup(webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("unexpected session resolution")
		return nil, "", nil
	}))
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"--service-id", "service-1", "--domain", "example.com", "--return-url", "https://example.com/cb"}, "--confirm is required"},
		{[]string{"--service-id", "service-1", "--domain", "https://example.com", "--return-url", "https://example.com/cb", "--confirm"}, "--domain"},
		{[]string{"--service-id", "service-1", "--domain", "example.com,", "--return-url", "https://example.com/cb", "--confirm"}, "--domain"},
		{[]string{"--service-id", "service-1", "--domain", "example.com", "--return-url", "http://example.com/cb", "--confirm"}, "--return-url"},
		{[]string{"--service-id", "service-1", "--domain", "example.com", "--return-url", "https://example.com/cb", "--confirm", "--output", "table", "--pretty"}, "--pretty"},
	} {
		assertUsageExit(t, append([]string{"web", "service-ids", "domains", "set"}, tc.args...), tc.message)
	}
}

func TestWebServiceIDsDomainsSetReportsVerifiedReceiptAndUnknownOutcome(t *testing.T) {
	setCmdtestHome(t)
	t.Cleanup(webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}))
	t.Cleanup(webcmd.SetPersistWebSession(func(*webcore.AuthSession) error { return nil }))
	var writeErr error
	var calls int
	t.Cleanup(webcmd.SetSetDeveloperServiceIDDomains(func(_ context.Context, _ *webcore.Client, r webcore.DeveloperServiceIDDomainsSetRequest) (*asc.WebServiceIDMutationResult, error) {
		calls++
		if r.ServiceID != "service-1" || !reflect.DeepEqual(r.Domains, []string{"example.com", "login.example.com"}) || !reflect.DeepEqual(r.ReturnURLs, []string{"https://example.com/cb", "https://login.example.com/cb"}) {
			t.Fatalf("unexpected request: %+v", r)
		}
		if writeErr != nil {
			return nil, writeErr
		}
		return &asc.WebServiceIDMutationResult{Operation: "domains-set", ServiceID: r.ServiceID, Changed: true, Verified: true, Status: "updated"}, nil
	}))
	args := []string{"web", "service-ids", "domains", "set", "--service-id", "service-1", "--domain", "login.example.com, example.com", "--return-url", "https://login.example.com/cb,https://example.com/cb", "--confirm", "--developer-team", "TEAM123456", "--output", "json"}
	var code int
	stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
	if code != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var receipt asc.WebServiceIDMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil || !receipt.Verified || !receipt.Changed || receipt.ServiceID != "service-1" {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
	writeErr = &webcore.DeveloperServiceIDUnverifiedError{Err: errors.New("domain update outcome is unknown; inspect before retrying")}
	stdout, stderr = captureOutput(t, func() { code = cmd.Run(args, "test") })
	if code != cmd.ExitError || stdout != "" || strings.Count(stderr, "domain update outcome is unknown") != 1 || calls != 2 {
		t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout, stderr, calls)
	}
}
