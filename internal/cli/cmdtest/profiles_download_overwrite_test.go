package cmdtest

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestProfilesDownloadExistingOutput(t *testing.T) {
	newContent := []byte("new profile")
	tests := []struct {
		name      string
		overwrite bool
		symlink   bool
		wantCalls int
		wantErr   string
		wantData  string
	}{
		{name: "refused before request without overwrite", wantErr: "pass --overwrite to replace it", wantData: "old profile"},
		{name: "replaced with overwrite", overwrite: true, wantCalls: 1, wantData: string(newContent)},
		{name: "symlink refused with overwrite", overwrite: true, symlink: true, wantErr: "refusing to overwrite symlink", wantData: "old profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			dir := t.TempDir()
			target := filepath.Join(dir, "target.mobileprovision")
			if err := os.WriteFile(target, []byte("old profile"), 0o600); err != nil {
				t.Fatal(err)
			}
			outputPath := target
			if tt.symlink {
				outputPath = filepath.Join(dir, "link.mobileprovision")
				if err := os.Symlink(target, outputPath); err != nil {
					t.Fatal(err)
				}
			}

			calls := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				body := `{"data":{"type":"profiles","id":"PROFILE_ID","attributes":{"name":"Dev","profileContent":"` + base64.StdEncoding.EncodeToString(newContent) + `"}}}`
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))

			args := []string{"profiles", "download", "--id", "PROFILE_ID", "--output", outputPath, "--output-format", "json"}
			if tt.overwrite {
				args = append(args, "--overwrite")
			}
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if calls != tt.wantCalls {
				t.Fatalf("requests = %d, want %d", calls, tt.wantCalls)
			}
			if tt.wantErr == "" {
				if runErr != nil {
					t.Fatalf("unexpected error: %v", runErr)
				}
			} else {
				if runErr == nil || !strings.Contains(runErr.Error(), tt.wantErr) || !strings.Contains(runErr.Error(), outputPath) {
					t.Fatalf("error = %v, want %q naming %q", runErr, tt.wantErr, outputPath)
				}
				if !shared.IsValidationError(runErr) || cmd.ExitCodeFromError(runErr) != cmd.ExitError {
					t.Fatalf("error not classified as validation: %v (exit %d)", runErr, cmd.ExitCodeFromError(runErr))
				}
				if diag, ok := shared.DiagnosticFromError(runErr); !tt.symlink && (!ok || diag.Code != shared.DiagnosticResourceConflict || diag.Parameter != "--output") {
					t.Fatalf("diagnostic = %+v, %v", diag, ok)
				}
			}
			data, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.wantData {
				t.Fatalf("target content = %q, want %q", data, tt.wantData)
			}
			if info, err := os.Lstat(target); err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("target mode = %v, %v; want 0600", info.Mode().Perm(), err)
			}
		})
	}
}
