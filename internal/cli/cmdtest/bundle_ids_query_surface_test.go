package cmdtest

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBundleIDsListEmitsQuerySurface(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	client := bundleIDPlatformTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected GET /v1/bundleIds, got %s %s", req.Method, req.URL.Path)
		}
		want := map[string]string{
			"filter[name]":                 "Example,Other",
			"filter[platform]":             "IOS,UNIVERSAL",
			"filter[identifier]":           "com.example.app,com.example.other",
			"filter[seedId]":               "seed-1,seed-2",
			"filter[id]":                   "bundle-1,bundle-2",
			"sort":                         "name,-identifier",
			"fields[bundleIds]":            "name,identifier,app",
			"fields[profiles]":             "name,expirationDate",
			"fields[bundleIdCapabilities]": "capabilityType,settings",
			"fields[apps]":                 "name,bundleId",
			"include":                      "profiles,bundleIdCapabilities,app",
			"limit[profiles]":              "7",
			"limit[bundleIdCapabilities]":  "8",
			"limit":                        "25",
		}
		values := req.URL.Query()
		for key, expected := range want {
			if got := values.Get(key); got != expected {
				t.Errorf("%s = %q, want %q", key, got, expected)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"bundleIds","id":"bundle-1"}]}`)
	})

	args := []string{
		"bundle-ids", "list",
		"--name", "Example,Other",
		"--platform", "ios,universal",
		"--identifier", "com.example.app,com.example.other",
		"--seed-id", "seed-1,seed-2",
		"--id", "bundle-1,bundle-2",
		"--sort", "name,-identifier",
		"--fields", "name,identifier,app",
		"--profile-fields", "name,expirationDate",
		"--capability-fields", "capabilityType,settings",
		"--app-fields", "name,bundleId",
		"--include", "profiles,bundleIdCapabilities,app",
		"--profiles-limit", "7",
		"--capabilities-limit", "8",
		"--limit", "25",
		"--output", "json",
	}
	stdout, stderr, err := runRootWithDeps(t, client, args...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"bundle-1"`) {
		t.Fatalf("stdout missing bundle ID: %q", stdout)
	}
}

func TestBundleIDsListRejectsInvalidQueryBeforeClient(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	clientFactoryCalls := 0

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "platform", args: []string{"--platform", "TV_OS"}, want: "--platform must be one of"},
		{name: "sort", args: []string{"--sort", "name,createdDate"}, want: "--sort must be one of"},
		{name: "bundle fields", args: []string{"--fields", "invalid"}, want: "--fields must be one of"},
		{name: "include", args: []string{"--include", "invalid"}, want: "--include must be one of"},
		{name: "profile fields require include", args: []string{"--profile-fields", "name"}, want: "--profile-fields requires --include profiles"},
		{name: "capability fields require include", args: []string{"--capability-fields", "settings"}, want: "--capability-fields requires --include bundleIdCapabilities"},
		{name: "app fields require include", args: []string{"--app-fields", "name"}, want: "--app-fields requires --include app"},
		{name: "profile limit zero", args: []string{"--profiles-limit", "0"}, want: "--profiles-limit must be between 1 and 50"},
		{name: "profile limit range", args: []string{"--profiles-limit", "51"}, want: "--profiles-limit must be between 1 and 50"},
		{name: "capability limit zero", args: []string{"--capabilities-limit", "0"}, want: "--capabilities-limit must be between 1 and 50"},
		{name: "capability limit range", args: []string{"--capabilities-limit", "51"}, want: "--capabilities-limit must be between 1 and 50"},
		{name: "oversized non-splittable query", args: []string{"--name", strings.Repeat("x", 5000)}, want: "request exceeds 3900-byte URL limit and cannot be split"},
		{name: "split identifiers require pagination", args: []string{"--identifier", makeLongBundleIDIdentifierFilter()}, want: "split identifier filter requires --paginate"},
		{name: "next conflict", args: []string{"--next", "https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=AQ", "--name", "Example"}, want: "--next cannot be combined with --name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, errors.New("client factory must not run during validation")
			}
			stdout, stderr, err := runRootWithDeps(t, client, append([]string{"bundle-ids", "list"}, test.args...)...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
	if clientFactoryCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
	}
}

func TestBundleIDsListIncludeRequirementsUseConciseUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	clientFactoryCalls := 0

	tests := []struct {
		name      string
		args      []string
		parameter string
		want      string
	}{
		{
			name:      "profile fields require include",
			args:      []string{"--profile-fields", "name"},
			parameter: "--profile-fields",
			want:      "Error: --profile-fields requires --include profiles\n",
		},
		{
			name:      "capability fields require include",
			args:      []string{"--capability-fields", "settings"},
			parameter: "--capability-fields",
			want:      "Error: --capability-fields requires --include bundleIdCapabilities\n",
		},
		{
			name:      "app fields require include",
			args:      []string{"--app-fields", "name"},
			parameter: "--app-fields",
			want:      "Error: --app-fields requires --include app\n",
		},
		{
			name:      "profile limit requires include",
			args:      []string{"--profiles-limit", "1"},
			parameter: "--profiles-limit",
			want:      "Error: --profiles-limit requires --include profiles\n",
		},
		{
			name:      "capability limit requires include",
			args:      []string{"--capabilities-limit", "1"},
			parameter: "--capabilities-limit",
			want:      "Error: --capabilities-limit requires --include bundleIdCapabilities\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, errors.New("client factory must not run during validation")
			}
			stdout, stderr, runErr := runRootWithDeps(t, client, append([]string{"bundle-ids", "list"}, test.args...)...)

			if runErr == nil || !shared.IsReportedUsageError(runErr) || errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want reported usage error without flag.ErrHelp", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("run error is missing validation diagnostic")
			}
			if diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != test.parameter {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticInvalidInput, test.parameter)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.want {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if clientFactoryCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}
