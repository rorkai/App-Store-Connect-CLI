package cmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// TestRunAPITelemetryNeverCarriesRequestDetails pins the passthrough command's
// telemetry contract: the command path stops at `asc api`, and the method,
// path, query, and body never appear in any event field.
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

	tests := []struct {
		name          string
		args          []string
		wantExit      int
		wantParameter string
	}{
		{
			name:     "usage error with unknown path",
			args:     []string{"api", "GET", secretPath + "z", "--query", secretQuery},
			wantExit: ExitUsage,
		},
		{
			name:          "mutation without confirm",
			args:          []string{"api", "PATCH", "/v1/apps/SECRET-APP-ID", "--body", secretBody},
			wantExit:      ExitUsage,
			wantParameter: "--confirm",
		},
		{
			name:     "request fails without credentials",
			args:     []string{"api", "GET", secretPath, "--query", secretQuery},
			wantExit: ExitAuth,
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
			serialized := reflect.ValueOf(gotContext)
			for i := 0; i < serialized.NumField(); i++ {
				field := serialized.Field(i)
				if field.Kind() != reflect.String {
					continue
				}
				for _, secret := range []string{"SECRET", "/v1/", "filter[", "{\"data\""} {
					if strings.Contains(field.String(), secret) {
						t.Fatalf("telemetry field %s = %q leaks request details", serialized.Type().Field(i).Name, field.String())
					}
				}
			}
		})
	}
}

// TestNormalizeSpacedBooleanFlagsPreservesAPIPositionals keeps a leading bool
// flag from swallowing the METHOD positional as its value.
func TestNormalizeSpacedBooleanFlagsPreservesAPIPositionals(t *testing.T) {
	args := []string{"api", "--paginate", "GET", "/v1/apps"}
	root := rootCommandForArgs("1.0.0", args)
	got := normalizeSpacedBooleanFlags(root, args)
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("normalizeSpacedBooleanFlags() = %#v, want positional args preserved %#v", got, args)
	}
}
