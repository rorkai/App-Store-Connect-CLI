package cmdtest

import (
	"testing"
)

func TestWebPrivacyUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "PullRequiresApp",
			args:    []string{"web", "privacy", "pull"},
			wantErr: "--app is required",
			deps:    true,
		},
		{
			name:    "CatalogRejectsPositionalArgs",
			args:    []string{"web", "privacy", "catalog", "extra"},
			wantErr: `unexpected argument "extra"`,
			deps:    true,
		},
		{
			name:    "PlanRequiresFile",
			args:    []string{"web", "privacy", "plan", "--app", "123456789"},
			wantErr: "--file is required",
			deps:    true,
		},
		{
			name:    "ApplyRequiresConfirmWhenAllowDeletesSet",
			args:    []string{"web", "privacy", "apply", "--app", "123456789", "--file", "privacy.json", "--allow-deletes"},
			wantErr: "--confirm is required when --allow-deletes is set",
			deps:    true,
		},
		{
			name:    "PublishRequiresConfirm",
			args:    []string{"web", "privacy", "publish", "--app", "123456789"},
			wantErr: "--confirm is required",
			deps:    true,
		},
	})
}
