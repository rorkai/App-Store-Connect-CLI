package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

var deleteOutputCommands = []struct {
	name       string
	args       []string
	path       string
	wantOutput string
	errPrefix  string
}{
	{
		name:       "localizations screenshot-sets delete",
		args:       []string{"localizations", "screenshot-sets", "delete", "--id", "set-1", "--confirm"},
		path:       "/v1/appScreenshotSets/set-1",
		wantOutput: `"id":"set-1"`,
		errPrefix:  "localizations screenshot-sets delete:",
	},
	{
		name:       "versions delete",
		args:       []string{"versions", "delete", "--version-id", "ver-1", "--confirm"},
		path:       "/v1/appStoreVersions/ver-1",
		wantOutput: `"versionId":"ver-1"`,
		errPrefix:  "versions delete:",
	},
}

func TestDeleteCommandsOutput(t *testing.T) {
	for _, test := range deleteOutputCommands {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodDelete || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want DELETE %s", req.Method, req.URL.Path, test.path)
				}
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range []string{test.wantOutput, `"deleted":true`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected %s in output, got %q", want, stdout)
				}
			}
		})
	}
}

func TestDeleteCommandsAPIError(t *testing.T) {
	for _, test := range deleteOutputCommands {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body: io.NopCloser(strings.NewReader(
						`{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found","detail":"Resource does not exist"}]}`,
					)),
					Header: http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.errPrefix) {
				t.Fatalf("expected error prefixed %q, got %v", test.errPrefix, runErr)
			}
			if !strings.Contains(strings.ToLower(runErr.Error()), "not found") {
				t.Fatalf("expected not-found details, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}
