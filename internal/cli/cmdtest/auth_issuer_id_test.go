package cmdtest

import (
	"encoding/json"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestAuthIssuerIDOutput(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, stdout string)
	}{
		{
			name: "json",
			args: []string{"auth", "issuer-id", "--output", "json"},
			check: func(t *testing.T, stdout string) {
				var payload struct {
					IssuerID string `json:"issuerId"`
					Profile  string `json:"profile"`
				}
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("failed to unmarshal auth issuer-id json: %v; stdout=%q", err, stdout)
				}
				if payload.IssuerID != "ISS456" || payload.Profile != "default" {
					t.Fatalf("expected issuerId ISS456 and profile default, got %+v", payload)
				}
			},
		},
		{
			name: "text",
			args: []string{"auth", "issuer-id"},
			check: func(t *testing.T, stdout string) {
				if stdout != "ISS456" {
					t.Fatalf("expected raw issuer id, got %q", stdout)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeDefaultAuthConfig(t)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(test.args, "1.0.0")
			})
			if code != cmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			test.check(t, stdout)
		})
	}
}
