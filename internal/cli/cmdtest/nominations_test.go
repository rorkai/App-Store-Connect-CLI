package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestNominationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "nominations list missing status",
			args:    []string{"nominations", "list"},
			wantErr: "--status is required",
		},
		{
			name:    "nominations create missing app",
			args:    []string{"nominations", "create", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--app is required",
		},
		{
			name:    "nominations create missing name",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--name is required",
		},
		{
			name:    "nominations create missing type",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--type is required",
		},
		{
			name:    "nominations create invalid type",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "INVALID", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--type must be one of",
		},
		{
			name:    "nominations create missing submitted",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--submitted is required",
		},
		{
			name:    "nominations create invalid publish date",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01"},
			wantErr: "--publish-start-date must be in RFC3339 format",
		},
		{
			name:    "nominations update missing id",
			args:    []string{"nominations", "update", "--name", "Updated"},
			wantErr: "--id is required",
		},
		{
			name:    "nominations update missing updates",
			args:    []string{"nominations", "update", "--id", "NOM_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "nominations update invalid type",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--type", "INVALID", "--submitted=false"},
			wantErr: "--type must be one of",
		},
		{
			name:    "nominations update invalid publish date",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--publish-start-date", "2026-02-01", "--submitted=false"},
			wantErr: "--publish-start-date must be in RFC3339 format",
		},
		{
			name:    "nominations update missing submitted or archived",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--notes", "Updated"},
			wantErr: "--submitted or --archived is required by Apple's live API",
		},
		{
			name:    "nominations delete missing id",
			args:    []string{"nominations", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "nominations delete missing confirm",
			args:    []string{"nominations", "delete", "--id", "NOM_ID"},
			wantErr: "--confirm is required to delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

// TestNominationsListHasInAppEventsQuery pins the raw query so false stays on
// the wire, the "--flag false" spelling is not read as bare true, and leaving
// the flag out adds nothing to the request.
func TestNominationsListHasInAppEventsQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name      string
		args      []string
		wantQuery string
	}{
		{name: "true", args: []string{"--has-in-app-events", "true"}, wantQuery: "filter%5BhasInAppEvents%5D=true&filter%5Bstate%5D=DRAFT"},
		{name: "false as separate value", args: []string{"--has-in-app-events", "false"}, wantQuery: "filter%5BhasInAppEvents%5D=false&filter%5Bstate%5D=DRAFT"},
		{name: "false with equals", args: []string{"--has-in-app-events=false"}, wantQuery: "filter%5BhasInAppEvents%5D=false&filter%5Bstate%5D=DRAFT"},
		{name: "omitted", wantQuery: "filter%5Bstate%5D=DRAFT"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int64
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				if req.Method != http.MethodGet || req.URL.Path != "/v1/nominations" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if req.URL.RawQuery != test.wantQuery {
					t.Errorf("query = %q, want %q", req.URL.RawQuery, test.wantQuery)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			var code int
			_, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append([]string{"nominations", "list", "--status", "DRAFT"}, test.args...), "1.2.3")
			})
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitSuccess, stderr)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
		})
	}
}

// TestNominationsListHasInAppEventsRejectsInvalidValues keeps a bad or missing
// value a usage error rather than an unfiltered list.
func TestNominationsListHasInAppEventsRejectsInvalidValues(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "not a boolean", args: []string{"--has-in-app-events", "maybe"}, wantErr: `invalid value "maybe" for flag -has-in-app-events: must be true or false`},
		{name: "empty", args: []string{"--has-in-app-events="}, wantErr: `invalid value "" for flag -has-in-app-events: must be true or false`},
		{name: "no value", args: []string{"--has-in-app-events"}, wantErr: "flag needs an argument: -has-in-app-events"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request: %s %s", req.Method, req.URL)
				return nil, errors.New("unexpected request")
			}))

			var code int
			_, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append([]string{"nominations", "list", "--status", "DRAFT"}, test.args...), "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
		})
	}
}
