package shared

import (
	"errors"
	"flag"
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNormalizeBuildProcessingStateFilter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		options BuildProcessingStateFilterOptions
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "single state",
			input: "processing",
			want:  []string{asc.BuildProcessingStateProcessing},
		},
		{
			name:  "all expands",
			input: "all",
			want: []string{
				asc.BuildProcessingStateProcessing,
				asc.BuildProcessingStateFailed,
				asc.BuildProcessingStateInvalid,
				asc.BuildProcessingStateValid,
			},
		},
		{
			name:  "alias maps and dedupes",
			input: "processing,complete,valid",
			options: BuildProcessingStateFilterOptions{
				FlagName:          "--state",
				AllowedValuesHelp: "PROCESSING, FAILED, INVALID, VALID, COMPLETE, or all",
				Aliases: map[string]string{
					"COMPLETE": asc.BuildProcessingStateValid,
				},
			},
			want: []string{asc.BuildProcessingStateProcessing, asc.BuildProcessingStateValid},
		},
		{
			name:    "all mixed invalid",
			input:   "all,valid",
			options: BuildProcessingStateFilterOptions{FlagName: "--state"},
			wantErr: true,
		},
		{
			name:    "unknown invalid",
			input:   "bogus",
			options: BuildProcessingStateFilterOptions{FlagName: "--state"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DefaultDeps().NormalizeBuildProcessingStateFilter(tt.input, tt.options)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeBuildProcessingStateFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
