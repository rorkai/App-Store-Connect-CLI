package ascterritory

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "alpha3 pass through", input: "USA", want: "USA"},
		{name: "legacy alpha3 pass through", input: "ant", want: "ANT"},
		{name: "alpha2 maps to alpha3", input: "US", want: "USA"},
		{name: "exact english name", input: "France", want: "FRA"},
		{name: "alias maps", input: "UAE", want: "ARE"},
		{name: "punctuation-less us virgin islands maps", input: "US Virgin Islands", want: "VIR"},
		{name: "punctuation-less us outlying islands maps", input: "US Outlying Islands", want: "UMI"},
		{name: "whitespace and case normalize", input: "  united states  ", want: "USA"},
		{name: "unknown rejected", input: "ZZZ", wantErr: "could not be mapped"},
		{name: "unsupported region rejected", input: "Antarctica", wantErr: "could not be mapped"},
		{name: "ambiguous alias rejected", input: "Congo", wantErr: "is ambiguous"},
		{name: "empty rejected", input: "   ", wantErr: "territory is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Normalize(test.input)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", test.wantErr)
				}
				if got != "" {
					t.Fatalf("expected empty result on error, got %q", got)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error containing %q, got %q", test.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
