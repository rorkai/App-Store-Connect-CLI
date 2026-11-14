package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewDetailsCreateRejectsDemoAccountRequiredWithoutCredentials(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

	stdout, stderr, runErr := runCommand(t, []string{
		"review", "details-create",
		"--version-id", "version-1",
		"--demo-account-required=true",
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsCreateAllowsDemoAccountRequiredWithBothCredentials(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreReviewDetails").JSONBody(`{"data":{"type":"appStoreReviewDetails","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactPhone":"+1 408 555 0100","contactEmail":"dev@example.com","demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password","demoAccountRequired":true},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-create",
		"--version-id", "version-1",
		"--contact-first-name", "Dev",
		"--contact-last-name", "Support",
		"--contact-email", "dev@example.com",
		"--contact-phone", "+1 408 555 0100",
		"--demo-account-required=true",
		"--demo-account-name", "reviewer@example.com",
		"--demo-account-password", "app-specific-password",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestReviewDetailsUpdateRejectsDemoAccountRequiredWhenExistingCredentialsAreIncomplete(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false}}}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"review", "details-update",
		"--id", "detail-1",
		"--demo-account-required=true",
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsUpdateAcceptsMaxLengthDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1").JSONBody(`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountPassword":"`+strings.Repeat("p", 100)+`"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountPassword":"`+strings.Repeat("p", 100)+`"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-update",
		"--id", "detail-1",
		"--demo-account-password", strings.Repeat("p", 100),
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestRunReviewDetailsRejectsOverlongDemoAccountPasswordWithUsageExit(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "details-create",
			args: []string{
				"review", "details-create",
				"--version-id", "version-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
		{
			name: "details-update",
			args: []string{
				"review", "details-update",
				"--id", "detail-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--demo-account-password must be 100 characters or fewer") {
				t.Fatalf("expected local demo account password length error, got %q", stderr)
			}
		})
	}
}

func TestReviewDetailsUpdateAllowsDemoAccountRequiredWhenExistingCredentialsArePresent(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1").JSONBody(`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-update",
		"--id", "detail-1",
		"--demo-account-required=true",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestReviewDetailsForVersionReturnsNotConfiguredStateWhenUnset(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreReviewDetail"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0"}}}`))

	stdout, stderr, err := runCommand(t, []string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Warning: App Store review detail is not configured for version \"version-1\".") {
		t.Fatalf("expected not-configured warning, got %q", stderr)
	}

	var payload struct {
		VersionID  string `json:"versionId"`
		Configured bool   `json:"configured"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.Configured {
		t.Fatal("expected configured=false")
	}
	if payload.Message == "" {
		t.Fatal("expected message")
	}
}

func TestReviewDetailsForVersionPreservesErrorForUnknownVersionID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/missing-version/appStoreReviewDetail"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/missing-version"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{"review", "details-for-version", "--version-id", "missing-version", "--output", "json"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "review details-for-version: failed to fetch:") {
		t.Fatalf("expected wrapped fetch error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "not configured") {
		t.Fatalf("expected unknown version to remain a hard error, got %v", runErr)
	}
}
