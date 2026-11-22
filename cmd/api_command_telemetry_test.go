package cmd

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// TestRunAPITelemetryNeverCarriesRequestDetails pins the passthrough command's
// telemetry contract: the command path stops at `asc api`, the event carries
// the method, the path as an embedded schema template, and the schema names of
// the query parameters, and no identifier, value, or body ever reaches it.
func TestRunAPITelemetryNeverCarriesRequestDetails(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secretPath = "/v1/apps/SECRET-APP-ID/builds"
	const secretQuery = "filter[version]=SECRET-VERSION"
	const secretBody = `{"data":{"type":"apps","id":"SECRET-BODY-ID"}}`
	method := func(value string) *string { return &value }
	names := func(values ...string) *[]string { return &values }

	tests := []struct {
		name          string
		args          []string
		wantExit      int
		wantParameter string
		wantMethod    *string
		wantTemplate  *string
		wantQuery     *[]string
	}{
		{
			name:         "usage error with unknown path",
			args:         []string{"api", "GET", secretPath + "z", "--query", secretQuery},
			wantExit:     ExitUsage,
			wantMethod:   method("GET"),
			wantTemplate: method("other"),
			wantQuery:    names("filter[version]"),
		},
		{
			name:          "mutation without confirm",
			args:          []string{"api", "PATCH", "/v1/apps/SECRET-APP-ID", "--body", secretBody},
			wantExit:      ExitUsage,
			wantParameter: "--confirm",
			wantMethod:    method("PATCH"),
			wantTemplate:  method("/v1/apps/{id}"),
		},
		{
			name:         "request fails without credentials",
			args:         []string{"api", "GET", "/v1/builds", "--query", "filter[app]=SECRET-APP-ID", "--query", secretQuery},
			wantExit:     ExitAuth,
			wantMethod:   method("GET"),
			wantTemplate: method("/v1/builds"),
			wantQuery:    names("filter[app]", "filter[version]"),
		},
		{
			name:         "full URL with an undocumented query parameter",
			args:         []string{"api", "GET", "https://api.appstoreconnect.apple.com/v1/apps/SECRET-APP-ID?cursor=SECRET-CURSOR&limit=5&SECRET-NAME=SECRET-VALUE"},
			wantExit:     ExitUsage,
			wantMethod:   method("GET"),
			wantTemplate: method("/v1/apps/{id}"),
			wantQuery:    names("cursor", "limit", "other"),
		},
		{
			name:     "missing PATH",
			args:     []string{"api", "GET"},
			wantExit: ExitUsage,
		},
		{
			name:     "unsupported method",
			args:     []string{"api", "PUT", secretPath},
			wantExit: ExitUsage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var gotCommand string
			var gotContext telemetry.EventContext
			emitTelemetry = func(commandName string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotCommand = commandName
				gotContext = eventContext
			}

			captureCommandOutput(t, func() {
				if code := Run(test.args, "1.2.3"); code != test.wantExit {
					t.Fatalf("Run(%v) exit code = %d, want %d", test.args, code, test.wantExit)
				}
			})

			if gotCommand != "asc api" {
				t.Fatalf("telemetry command path = %q, want %q", gotCommand, "asc api")
			}
			if gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("failure parameter = %q, want %q", gotContext.FailureParameter, test.wantParameter)
			}
			event, ok := telemetry.BuildEventWithContext(gotCommand, "1.2.3", 0, test.wantExit, gotContext)
			if !ok {
				t.Fatal("BuildEventWithContext() returned no event")
			}
			if !reflect.DeepEqual(event.APIMethod, test.wantMethod) ||
				!reflect.DeepEqual(event.APIPathTemplate, test.wantTemplate) ||
				!reflect.DeepEqual(event.APIQueryParams, test.wantQuery) {
				t.Fatalf("api fields = %v %v %v, want %v %v %v",
					deref(event.APIMethod), deref(event.APIPathTemplate), event.APIQueryParams,
					deref(test.wantMethod), deref(test.wantTemplate), test.wantQuery)
			}
			payload, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			for _, secret := range []string{"SECRET", "api.appstoreconnect.apple.com", "{\"data\""} {
				if strings.Contains(string(payload), secret) {
					t.Fatalf("event %s leaks request details (%q)", payload, secret)
				}
			}
		})
	}
}

func deref(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

// TestNormalizeSpacedBooleanFlagsPreservesAPIPositionals keeps a leading bool
// flag from swallowing the METHOD positional as its value.
func TestNormalizeSpacedBooleanFlagsPreservesAPIPositionals(t *testing.T) {
	args := []string{"api", "--paginate", "GET", "/v1/apps"}
	root := rootCommandForArgs("1.0.0", shared.DefaultDeps(), args)
	got := normalizeSpacedBooleanFlags(root, args)
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("normalizeSpacedBooleanFlags() = %#v, want positional args preserved %#v", got, args)
	}
}
