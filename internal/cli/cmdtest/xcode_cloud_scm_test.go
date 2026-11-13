package cmdtest

import (
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
			args:    []string{"xcode-cloud", "scm", "providers", "view"},
			wantErr: "--provider-id is required",
		},
		{
			name:    "providers repositories missing provider id",
			args:    []string{"xcode-cloud", "scm", "providers", "repositories"},
			wantErr: "--provider-id is required",
		},
		{
			name:    "repositories get missing id",
			args:    []string{"xcode-cloud", "scm", "repositories", "view"},
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
			args:    []string{"xcode-cloud", "scm", "git-references", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "pull requests get missing id",
			args:    []string{"xcode-cloud", "scm", "pull-requests", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}
