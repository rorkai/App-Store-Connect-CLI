package reviews

import (
	"strings"
	"testing"
)

func TestNormalizeRatingsOutput(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		pretty     bool
		wantFormat string
		wantErr    string
	}{
		{name: "default json", input: "", pretty: false, wantFormat: "json"},
		{name: "markdown alias md", input: "md", pretty: false, wantFormat: "markdown"},
		{name: "trim and lowercase", input: "  TABLE  ", pretty: false, wantFormat: "table"},
		{name: "pretty table rejected", input: "table", pretty: true, wantErr: "--pretty is only valid with JSON output"},
		{name: "unsupported format rejected", input: "yaml", pretty: false, wantErr: "unsupported format: yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeRatingsOutput(tc.input, tc.pretty)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantFormat {
				t.Fatalf("expected format %q, got %q", tc.wantFormat, got)
			}
		})
	}
}
