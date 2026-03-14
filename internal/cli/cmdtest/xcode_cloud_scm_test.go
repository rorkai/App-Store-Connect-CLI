package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestXcodeCloudScmValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "providers get missing provider id",
			args:    []string{"xcode-cloud", "scm", "providers", "get"},
			wantErr: "--provider-id is required",
		},
		{
			name:    "providers repositories missing provider id",
			args:    []string{"xcode-cloud", "scm", "providers", "repositories"},
			wantErr: "--provider-id is required",
		},
		{
			name:    "repositories get missing id",
			args:    []string{"xcode-cloud", "scm", "repositories", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "repositories git references missing repo id",
			args:    []string{"xcode-cloud", "scm", "repositories", "git-references"},
			wantErr: "--repo-id is required",
		},
		{
			name:    "repositories pull requests missing repo id",
			args:    []string{"xcode-cloud", "scm", "repositories", "pull-requests"},
			wantErr: "--repo-id is required",
		},
		{
			name:    "repositories relationships git references missing repo id",
			args:    []string{"xcode-cloud", "scm", "repositories", "links", "git-references"},
			wantErr: "--repo-id is required",
		},
		{
			name:    "repositories relationships pull requests missing repo id",
			args:    []string{"xcode-cloud", "scm", "repositories", "links", "pull-requests"},
			wantErr: "--repo-id is required",
		},
		{
			name:    "git references get missing id",
			args:    []string{"xcode-cloud", "scm", "git-references", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "pull requests get missing id",
			args:    []string{"xcode-cloud", "scm", "pull-requests", "get"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
