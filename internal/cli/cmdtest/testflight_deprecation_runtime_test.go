package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRootFeedbackAndCrashesWarnBeforeAuthFailure(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		warning string
	}{
		{
			name:    "feedback",
			args:    []string{"feedback", "--app", "123"},
			warning: feedbackRootDeprecationWarning,
		},
		{
			name:    "crashes",
			args:    []string{"crashes", "--app", "123"},
			warning: crashesRootDeprecationWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_PROFILE", "")
			t.Setenv("ASC_KEY_ID", "")
			t.Setenv("ASC_ISSUER_ID", "")
			t.Setenv("ASC_PRIVATE_KEY_PATH", "")
			t.Setenv("ASC_PRIVATE_KEY", "")
			t.Setenv("ASC_PRIVATE_KEY_B64", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.2.3")
				if code != cmd.ExitAuth {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitAuth, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			requireStderrContainsWarning(t, stderr, test.warning)
			if !strings.Contains(stderr, "missing authentication") {
				t.Fatalf("expected missing auth error, got %q", stderr)
			}
		})
	}
}

func TestTestFlightGroupsDeleteUsesCanonicalSuccessMessage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1" {
			t.Fatalf("expected path /v1/betaGroups/group-1, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "delete", "--id", "group-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully deleted group group-1") {
		t.Fatalf("expected canonical delete message, got %q", stderr)
	}
	if strings.Contains(stderr, "beta group") {
		t.Fatalf("expected canonical message without beta terminology, got %q", stderr)
	}
}
