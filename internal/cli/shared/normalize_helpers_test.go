package shared

import "testing"

func TestNormalizeEnumToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "  ", want: ""},
		{name: "already normalized", input: "FREE_TRIAL", want: "FREE_TRIAL"},
		{name: "hyphenated", input: "free-trial", want: "FREE_TRIAL"},
		{name: "spaced", input: "free trial", want: "FREE_TRIAL"},
		{name: "trimmed", input: "  pay-as-you-go  ", want: "PAY_AS_YOU_GO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeEnumToken(tt.input)
			if got != tt.want {
				t.Fatalf("NormalizeEnumToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseBoolFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "true word", input: "true", want: true},
		{name: "true number", input: "1", want: true},
		{name: "true yes", input: "yes", want: true},
		{name: "false word", input: "false", want: false},
		{name: "false number", input: "0", want: false},
		{name: "false no", input: "no", want: false},
		{name: "invalid", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBoolFlag(tt.input, "--flag")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseBoolFlag(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBoolFlag(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseBoolFlag(%q) = %t, want %t", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
		{
			name:  "single id",
			input: "tester-1",
			want:  []string{"tester-1"},
		},
		{
			name:  "comma separated",
			input: "tester-1, tester-2, tester-3",
			want:  []string{"tester-1", "tester-2", "tester-3"},
		},
		{
			name:  "drops empty entries",
			input: "tester-1,, ,tester-2",
			want:  []string{"tester-1", "tester-2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := SplitCSV(test.input)
			if len(got) != len(test.want) {
				t.Fatalf("expected %d ids, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Fatalf("expected %q at index %d, got %q", want, i, got[i])
				}
			}
		})
	}
}
