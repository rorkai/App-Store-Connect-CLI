package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	appclipscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/appclips"
)

func TestAppClipsAdvancedExperiencesCreateValidatesSelectorBeforeClient(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		selector   []string
		wantStderr string
	}{
		{
			name:       "missing app clip and bundle selectors",
			wantStderr: "Error: --app-clip-id or --bundle-id is required\n",
		},
		{
			name:       "bundle selector missing app",
			selector:   []string{"--bundle-id", "com.example.clip"},
			wantStderr: "Error: --app is required with --bundle-id\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertAppClipAdvancedExperienceCreateUsageBeforeClient(t, test.selector, test.wantStderr)
		})
	}
}

func assertAppClipAdvancedExperienceCreateUsageBeforeClient(t *testing.T, selector []string, wantStderr string) {
	t.Helper()

	clientFactoryCalls := 0
	t.Cleanup(appclipscli.SetClientFactory(func() (*asc.Client, error) {
		clientFactoryCalls++
		return nil, errors.New("client factory must not run")
	}))

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run(appClipAdvancedExperienceCreateArgs(selector...), "1.2.3")
	})

	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	diagnostic, usage, found := strings.Cut(stderr, "DESCRIPTION\n")
	if !found || usage == "" {
		t.Fatalf("stderr is missing command usage: %q", stderr)
	}
	if diagnostic != wantStderr {
		t.Fatalf("stderr diagnostic = %q, want %q", diagnostic, wantStderr)
	}
	if clientFactoryCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
	}
}

func appClipAdvancedExperienceCreateArgs(selector ...string) []string {
	args := []string{
		"app-clips", "advanced-experiences", "create",
		"--link", "https://example.com",
		"--default-language", "EN",
		"--is-powered-by",
		"--header-image-id", "img-1",
		"--language", "EN",
		"--title", "Order ahead",
	}
	return append(args, selector...)
}
