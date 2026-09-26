package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// App Store Connect rejects U+2764 and U+FE0F in whatsNew with "Text for
// whatsNew contains invalid characters". Every What to Test writer must refuse
// them locally, before any request that could leave a side effect such as an
// auto-created TestFlight app localization.
func TestTestNotesWritersRejectCharactersAppStoreConnectRefusesWithoutRequests(t *testing.T) {
	const notes = "Roll the die ❤️"

	tests := []struct {
		name string
		args func(t *testing.T) []string
	}{
		{
			name: "builds test-notes create",
			args: func(*testing.T) []string {
				return []string{
					"builds", "test-notes", "create",
					"--build-id", "build-1",
					"--locale", "en-US",
					"--whats-new", notes,
				}
			},
		},
		{
			name: "builds test-notes update",
			args: func(*testing.T) []string {
				return []string{
					"builds", "test-notes", "update",
					"--localization-id", "loc-1",
					"--whats-new", notes,
				}
			},
		},
		{
			name: "builds upload",
			args: func(t *testing.T) []string {
				return []string{
					"builds", "upload",
					"--app", "123456789",
					"--ipa", writeBuildUploadIPA(t, "com.example.demo"),
					"--version", "1.0.0",
					"--build-number", "42",
					"--test-notes", notes,
					"--locale", "en-US",
				}
			},
		},
		{
			name: "publish testflight",
			args: func(*testing.T) []string {
				return []string{
					"publish", "testflight",
					"--app", "123456789",
					"--ipa", "app.ipa",
					"--group", "GROUP_ID",
					"--test-notes", notes,
					"--locale", "en-US",
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, errors.New("unexpected request")
			})

			args := test.args(t)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage-class failure", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			for _, want := range []string{"What to Test notes", "U+2764", "U+FE0F"} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
			if strings.Contains(stderr, "Roll the die") {
				t.Fatalf("stderr = %q, must not echo the notes", stderr)
			}
			if strings.Contains(stderr, "created TestFlight app localization") {
				t.Fatalf("stderr = %q, want no localization side effect", stderr)
			}
		})
	}
}
