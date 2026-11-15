package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestOutputFlagRejectionUsesEnumStyle pins the rejection of an unsupported
// output format to the same shape every other enumerated flag uses: one line
// naming the flag, its valid set, and the offending value, with no help dump.
func TestOutputFlagRejectionUsesEnumStyle(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "default output set",
			args: []string{"apps", "list", "--output", "yaml"},
			want: `Error: --output must be one of: json, table, markdown (got "yaml")` + "\n",
		},
		{
			name: "restricted output set",
			args: []string{"auth", "status", "--output", "yaml"},
			want: `Error: --output must be one of: table, json (got "yaml")` + "\n",
		},
		{
			name: "custom flag name",
			args: []string{"finance", "reports", "--output-format", "yaml"},
			want: `Error: --output-format must be one of: json, table, markdown (got "yaml")` + "\n",
		},
		{
			name: "alias normalizes to canonical name",
			args: []string{"auth", "status", "--output", "md"},
			want: `Error: --output must be one of: table, json (got "md")` + "\n",
		},
		{
			name: "rejected value preserves whitespace",
			args: []string{"apps", "list", "--output", " yaml "},
			want: `Error: --output must be one of: json, table, markdown (got " yaml ")` + "\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "1.2.3")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != test.want {
				t.Fatalf("stderr = %q, want exactly %q", stderr, test.want)
			}
			if strings.Contains(stderr, "USAGE") || strings.Contains(stderr, "DESCRIPTION") {
				t.Fatalf("expected no help dump, got %q", stderr)
			}
		})
	}
}

// TestValidOutputFormatsAreUnaffected keeps the guard from touching accepted
// values, including the markdown alias.
func TestValidOutputFormatsAreUnaffected(t *testing.T) {
	errClient := errors.New("client requested")
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return nil, errClient }))
	for _, format := range []string{"json", "table", "markdown", "md"} {
		t.Run(format, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				rootcmd.Run([]string{"apps", "list", "--output", format}, "1.2.3")
			})
			if strings.Contains(stderr, "must be one of") {
				t.Fatalf("format %q was rejected: %q", format, stderr)
			}
			if !strings.Contains(stderr, errClient.Error()) {
				t.Fatalf("format %q did not reach the client: %q", format, stderr)
			}
		})
	}
}
