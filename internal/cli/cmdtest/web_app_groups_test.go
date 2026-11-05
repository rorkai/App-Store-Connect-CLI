package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppGroupsAuthenticationFailureUsesAuthExit(t *testing.T) {
	stubWebAppGroupsSession(t)
	restoreDelete := webcmd.SetDeleteDeveloperAppGroup(func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		return nil, fmt.Errorf("web session is unauthorized or expired for Developer Portal: %w", &webcore.APIError{Status: 401})
	})
	t.Cleanup(restoreDelete)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "app-groups", "delete", "--group-id", "GROUP1", "--confirm", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitAuth {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitAuth, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unauthorized or expired") {
		t.Fatalf("stderr lost authentication guidance: %q", stderr)
	}
}

func TestWebAppGroupsPortalRefusalSanitizesStderr(t *testing.T) {
	stubWebAppGroupsSession(t)
	restoreDelete := webcmd.SetDeleteDeveloperAppGroup(func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		return nil, &webcore.DeveloperPortalResultError{
			ResultCode: 35,
			RequestID:  "request-1\x1b[31m\r\nforged-request",
			Message:    "Denied\x1b]8;;https://example.invalid\a\r\nforged-message",
		}
	})
	t.Cleanup(restoreDelete)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "app-groups", "delete", "--group-id", "GROUP1", "--confirm", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, unsafe := range []string{"\x1b", "\r", "\nforged-request", "\nforged-message"} {
		if strings.Contains(stderr, unsafe) {
			t.Fatalf("stderr contains unsafe sequence %q: %q", unsafe, stderr)
		}
	}
	for _, preserved := range []string{"request-1", "forged-request", "Denied", "forged-message"} {
		if !strings.Contains(stderr, preserved) {
			t.Fatalf("stderr lost sanitized text %q: %q", preserved, stderr)
		}
	}
}

func stubWebAppGroupsSession(t *testing.T) {
	t.Helper()
	setCmdtestHome(t)
	restoreSession := webcmd.SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)
}

func TestWebAppGroupsMutationsRequireConfirmBeforeAnyRequest(t *testing.T) {
	setCmdtestHome(t)
	restoreSession := webcmd.SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("web session must not be resolved without --confirm")
		return nil, "", nil
	})
	t.Cleanup(restoreSession)

	assertUsageExit(t, []string{"web", "app-groups", "delete", "--group-id", "GROUP1"}, "Error: --confirm is required")
	assertUsageExit(t, []string{"web", "app-groups", "unassign", "--group-id", "GROUP1", "--bundle-id", "bundle-1"}, "Error: --confirm is required")
	assertUsageExit(t, []string{"web", "app-groups", "set", "--group", "GROUP1", "--bundle-id", "bundle-1"}, "Error: --confirm is required")
	assertUsageExit(t, []string{"web", "app-groups", "set", "--bundle-id", "bundle-1", "--confirm"}, "Error: at least one --group is required")
}

func TestWebAppGroupsSetRejectsBlankGroupBeforeAnyRequest(t *testing.T) {
	setCmdtestHome(t)
	restoreSession := webcmd.SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("web session must not be resolved when a --group value is blank")
		return nil, "", nil
	})
	t.Cleanup(restoreSession)
	restoreSet := webcmd.SetSetDeveloperAppGroups(func(context.Context, *webcore.Client, webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
		t.Fatal("set must not be called when a --group value is blank")
		return nil, nil
	})
	t.Cleanup(restoreSet)

	assertUsageExit(t, []string{"web", "app-groups", "set", "--bundle-id", "bundle-1", "--group", "GROUP1", "--group", "   ", "--confirm"}, "value cannot be empty")
	assertUsageExit(t, []string{"web", "app-groups", "set", "--bundle-id", "bundle-1", "--group", "", "--confirm"}, "value cannot be empty")
}

func TestWebAppGroupsDeleteAssignedGroupExitsNonZeroAndNamesAssignments(t *testing.T) {
	stubWebAppGroupsSession(t)
	restoreDelete := webcmd.SetDeleteDeveloperAppGroup(func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		return nil, &webcore.DeveloperAppGroupInUseError{
			GroupID:    request.GroupID,
			Identifier: "group.com.example.shared",
			Assignments: []webcore.DeveloperAppGroupAssignment{
				{BundleID: "bundle-1", Identifier: "com.example.app", Name: "Example"},
			},
		}
	})
	t.Cleanup(restoreDelete)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "app-groups", "delete", "--group-id", "GROUP1", "--confirm", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, expected := range []string{"still assigned", "group.com.example.shared", "com.example.app (bundle-1)", "asc web app-groups unassign"} {
		if !strings.Contains(stderr, expected) {
			t.Fatalf("stderr %q does not mention %q", stderr, expected)
		}
	}
}

func TestWebAppGroupsSetNoOpReportsUnchanged(t *testing.T) {
	stubWebAppGroupsSession(t)
	restoreSet := webcmd.SetSetDeveloperAppGroups(func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
		return &asc.WebAppGroupSetResult{BundleID: request.BundleID, GroupIDs: request.GroupIDs, Added: []string{}, Removed: []string{}, Changed: false, Status: "unchanged"}, nil
	})
	t.Cleanup(restoreSet)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "app-groups", "set", "--bundle-id", "bundle-1", "--group", "GROUP1", "--group", "GROUP2", "--confirm", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for a no-op set, got %q", stderr)
	}
	var result asc.WebAppGroupSetResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if result.Changed || result.Status != "unchanged" || strings.Join(result.GroupIDs, ",") != "GROUP1,GROUP2" || len(result.Added) != 0 || len(result.Removed) != 0 {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestWebAppGroupsUnassignReportsReceiptAndWarns(t *testing.T) {
	stubWebAppGroupsSession(t)
	restoreUnassign := webcmd.SetUnassignDeveloperAppGroup(func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
		return &asc.WebAppGroupUnassignResult{BundleID: request.BundleID, GroupID: request.GroupID, RemainingGroupIDs: []string{}, Changed: true, Status: "unassigned"}, nil
	})
	t.Cleanup(restoreUnassign)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "app-groups", "unassign", "--bundle-id", "bundle-1", "--group-id", "GROUP1", "--confirm", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	var result asc.WebAppGroupUnassignResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if !result.Changed || result.Status != "unassigned" || result.GroupID != "GROUP1" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("stderr %q is missing the provisioning profile warning", stderr)
	}
}
