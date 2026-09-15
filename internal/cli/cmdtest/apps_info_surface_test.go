package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestAppsHelpShowsInfoSubcommand(t *testing.T) {
	root := RootCommand("1.2.3")
	var appsCmd any
	for _, sub := range root.Subcommands {
		if sub != nil && sub.Name == "apps" {
			appsCmd = sub
			break
		}
	}
	if appsCmd == nil {
		t.Fatal("expected apps command in root subcommands")
	}

	usage := appsCmd.(*ffcli.Command).UsageFunc(appsCmd.(*ffcli.Command))
	if !strings.Contains(usage, "info") {
		t.Fatalf("expected apps help to show info subcommand, got %q", usage)
	}
}

func TestRootHelpRemovesAppInfoRoots(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{}); err != nil {
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
	if strings.Contains(stderr, "  app-info:") || strings.Contains(stderr, "  app-infos:") {
		t.Fatalf("expected root help to remove app-info roots, got %q", stderr)
	}
}

func TestAppsInfoHelpShowsNewSurface(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "info"}); err != nil {
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
	for _, want := range []string{"list", "view", "edit"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected apps info help to contain %q, got %q", want, stderr)
		}
	}
}

func TestAppInfoRootsAreRemoved(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-info root",
			args:    []string{"app-info", "get"},
			wantErr: "Unknown command: app-info",
		},
		{
			name:    "app-infos root",
			args:    []string{"app-infos", "list"},
			wantErr: "Unknown command: app-infos",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppsInfoViewIncludeFailsWhenAppInfoIsAmbiguous(t *testing.T) {
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
		if req.URL.Path != "/v1/apps/app-1/appInfos" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"data":[
					{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
					{"type":"appInfos","id":"info-rejected","attributes":{"state":"REJECTED"}}
				]
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "info", "view", "--app", "app-1", "--include", "primaryCategory", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`2 app infos match app "app-1"; pass --info-id with one of:`,
		`asc apps info list --app "app-1"`,
		"READY_FOR_DISTRIBUTION",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}
