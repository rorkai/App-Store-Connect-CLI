package cmdtest

import "testing"

func TestValidateRemovedRemediationFlagsReturnUsageExitCode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "next removed",
			args:    []string{"validate", "--app", "app-1", "--version-id", "ver-1", "--next"},
			wantErr: "flag provided but not defined",
		},
		{
			name:    "fix-plan removed",
			args:    []string{"validate", "--app", "app-1", "--version-id", "ver-1", "--fix-plan"},
			wantErr: "flag provided but not defined",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestValidateSubcommandsRejectParentValidateFlagsExitCode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "version-id before subcommand",
			args:    []string{"validate", "--version-id", "ver-1", "testflight", "--app", "app-1", "--build", "build-1"},
			wantErr: "--version-id is only valid for asc validate",
		},
		{
			name:    "strict before subcommand",
			args:    []string{"validate", "--strict", "testflight", "--app", "app-1", "--build", "build-1"},
			wantErr: "--strict must be passed after the validate subcommand name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}
