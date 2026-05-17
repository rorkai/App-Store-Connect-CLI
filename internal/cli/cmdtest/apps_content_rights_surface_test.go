package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAppsContentRightsCommandSurface(t *testing.T) {
	root := RootCommand("1.2.3")

	group := findSubcommand(root, "apps", "content-rights")
	if group == nil {
		t.Fatal("expected apps content-rights command")
		return
	}
	if findSubcommand(root, "apps", "content-rights", "view") == nil {
		t.Fatal("expected apps content-rights view command")
	}
	if findSubcommand(root, "apps", "content-rights", "edit") == nil {
		t.Fatal("expected apps content-rights edit command")
	}
	if findSubcommand(root, "apps", "content-rights", "get") != nil {
		t.Fatal("did not expect deprecated get alias")
	}
	if findSubcommand(root, "apps", "content-rights", "set") != nil {
		t.Fatal("did not expect deprecated set alias")
	}

	usage := group.UsageFunc(group)
	if !strings.Contains(usage, "view") || !strings.Contains(usage, "edit") {
		t.Fatalf("expected usage to show view/edit, got %q", usage)
	}
}

func TestAppsContentRightsEditRequiresExplicitValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "content-rights", "edit", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--uses-third-party-content is required") {
		t.Fatalf("expected missing value guidance, got %q", stderr)
	}
}

func TestAppsContentRightsViewTableShowsDeclaration(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1" {
			t.Fatalf("expected path /v1/apps/app-1, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"data":{
					"type":"apps",
					"id":"app-1",
					"attributes":{
						"name":"Test App",
						"bundleId":"com.example.test",
						"sku":"TESTSKU",
						"contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"
					}
				}
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "content-rights", "view", "--app", "app-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !strings.Contains(stdout, "content_rights_declaration") {
		t.Fatalf("expected declaration row header, got %q", stdout)
	}
	if !strings.Contains(stdout, "DOES_NOT_USE_THIRD_PARTY_CONTENT") {
		t.Fatalf("expected declaration value in table output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}
}
