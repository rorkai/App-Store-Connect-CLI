package cmdtest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func installIndirectionTransport(t *testing.T, method, path, response string) *[]string {
	t.Helper()
	setupLocUpdateAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	bodies := &[]string{}
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != method || req.URL.Path != path {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		*bodies = append(*bodies, string(body))
		return locUpdateJSONResponse(response)
	})
	return bodies
}

func TestFlagIndirectionFileValueReachesRequestBody(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notesPath, []byte("Line one\nLine two\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	bodies := installIndirectionTransport(t, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-1",
		`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"localizations", "update", "--id", "loc-1", "--whats-new", "@file:" + notesPath}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(*bodies) != 1 {
		t.Fatalf("request count = %d, want 1", len(*bodies))
	}
	if !strings.Contains((*bodies)[0], `"whatsNew":"Line one\nLine two"`) {
		t.Fatalf("patch body = %s, want file contents with one trailing newline trimmed", (*bodies)[0])
	}
	if strings.Contains((*bodies)[0], "@file:") {
		t.Fatalf("patch body still carries the indirection token: %s", (*bodies)[0])
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("stdout = %q, want updated localization", stdout)
	}
}

func TestFlagIndirectionEnvValueReachesRequestBody(t *testing.T) {
	t.Setenv("ASC_TEST_WEBHOOK_SECRET", "hunter2-from-env")
	bodies := installIndirectionTransport(t, http.MethodPost, "/v1/webhooks",
		`{"data":{"type":"webhooks","id":"wh-1","attributes":{"name":"Build Updates","url":"https://example.com/webhook","enabled":true,"eventTypes":["BUILD_UPLOAD_STATE_UPDATED"]}}}`)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"webhooks", "create",
			"--app", "APP_ID",
			"--name", "Build Updates",
			"--url", "https://example.com/webhook",
			"--secret", "@env:ASC_TEST_WEBHOOK_SECRET",
			"--events", "BUILD_UPLOAD_STATE_UPDATED",
			"--enabled", "true",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(*bodies) != 1 {
		t.Fatalf("request count = %d, want 1", len(*bodies))
	}
	if !strings.Contains((*bodies)[0], `"secret":"hunter2-from-env"`) {
		t.Fatalf("post body = %s, want resolved secret", (*bodies)[0])
	}
	if !strings.Contains(stdout, `"id":"wh-1"`) {
		t.Fatalf("stdout = %q, want created webhook", stdout)
	}
}

func TestFlagIndirectionDoubleAtEscapesLiteralValue(t *testing.T) {
	bodies := installIndirectionTransport(t, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-1",
		`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"localizations", "update", "--id", "loc-1", "--whats-new", "@@env:NOT_A_VARIABLE"}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(*bodies) != 1 || !strings.Contains((*bodies)[0], `"whatsNew":"@env:NOT_A_VARIABLE"`) {
		t.Fatalf("bodies = %q, want literal @env:NOT_A_VARIABLE", *bodies)
	}
}

func TestFlagIndirectionErrorsAreUsageErrorsBeforeAnyRequest(t *testing.T) {
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")
	t.Setenv("ASC_TEST_INDIRECT_EMPTY", "")
	missingPath := filepath.Join(t.TempDir(), "missing.txt")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unset environment variable",
			args: []string{"webhooks", "create", "--app", "APP_ID", "--name", "n", "--url", "https://example.com", "--secret", "@env:ASC_TEST_INDIRECT_UNSET", "--events", "BUILD_UPLOAD_STATE_UPDATED", "--enabled", "true"},
			want: "Error: --secret: environment variable ASC_TEST_INDIRECT_UNSET is not set\n",
		},
		{
			name: "empty environment variable",
			args: []string{"webhooks", "create", "--app", "APP_ID", "--secret=@env:ASC_TEST_INDIRECT_EMPTY"},
			want: "Error: --secret: environment variable ASC_TEST_INDIRECT_EMPTY is empty\n",
		},
		{
			name: "missing file",
			args: []string{"localizations", "update", "--id", "loc-1", "--whats-new", "@file:" + missingPath},
			want: "Error: --whats-new: cannot read file " + missingPath + ": file does not exist\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bodies := installIndirectionTransport(t, "", "", "{}")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(test.args, "1.2.3")
			})
			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.want {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if len(*bodies) != 0 {
				t.Fatalf("requests = %q, want none", *bodies)
			}
		})
	}
}

func TestFlagIndirectionSkipsOutputAndProfileFlags(t *testing.T) {
	t.Setenv("ASC_TEST_OUTPUT_FORMAT", "json")
	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"apps", "list", "--output", "@env:ASC_TEST_OUTPUT_FORMAT"}, "1.2.3"); code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := `Error: --output must be one of: json, table, markdown (got "@env:ASC_TEST_OUTPUT_FORMAT")` + "\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
