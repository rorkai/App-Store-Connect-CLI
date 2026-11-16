package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAPIKeysCommandIncludesRevoke(t *testing.T) {
	d, _, _ := newWebTestDeps()
	for _, command := range newWebAPIKeysCommand(d).Subcommands {
		if command.Name == "revoke" {
			return
		}
	}
	t.Fatal("expected web api-keys command group to include revoke")
}

func TestWebAPIKeysRevokeValidatesBeforeSession(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing key id", args: []string{"--type", "team", "--confirm"}, want: "--key-id is required"},
		{name: "missing type", args: []string{"--key-id", "KEY123", "--confirm"}, want: "--type must be team or individual"},
		{name: "invalid type", args: []string{"--key-id", "KEY123", "--type", "other", "--confirm"}, want: "--type must be team or individual"},
		{name: "missing confirm", args: []string{"--key-id", "KEY123", "--type", "team"}, want: "--confirm is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveCalled := false
			restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
				resolveCalled = true
				return &webcore.AuthSession{}, "cache", nil
			})
			t.Cleanup(restoreSession)

			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAPIKeysRevokeCommand(d)
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var execErr error
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { execErr = cmd.Exec(context.Background(), nil) })
			if !errors.Is(execErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", execErr)
			}
			if !strings.Contains(stderr, tt.want) && !strings.Contains(execErr.Error(), tt.want) {
				t.Fatalf("expected %q in stderr/error, got %q / %v", tt.want, stderr, execErr)
			}
			if resolveCalled {
				t.Fatal("did not expect session resolution before revoke validation")
			}
		})
	}
}

func TestWebAPIKeysRevokeActiveKeyUsesPreflightPatchAndPostList(t *testing.T) {
	listCalls := 0
	revokeCalls := 0
	var gotKeyID, gotKind string
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			listCalls++
			if kind != webcore.APIKeyKindTeam {
				t.Fatalf("expected team preflight/post-list, got %q", kind)
			}
			active := listCalls == 1
			return []webcore.APIKeyListItem{{
				KeyID:  "KEY123",
				Kind:   webcore.APIKeyKindTeam,
				Active: active,
			}}, nil
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			gotKeyID, gotKind = keyID, kind
			return nil
		},
	)
	t.Cleanup(restore)

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--key-id", "KEY123",
		"--type", "team",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { execErr = cmd.Exec(context.Background(), nil) })
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if listCalls != 2 || revokeCalls != 1 {
		t.Fatalf("expected two type-specific lists and one revoke, got lists=%d revoke=%d", listCalls, revokeCalls)
	}
	if gotKeyID != "KEY123" || gotKind != webcore.APIKeyKindTeam {
		t.Fatalf("unexpected revoke arguments: key=%q kind=%q", gotKeyID, gotKind)
	}
	var result asc.WebAPIKeyRevokeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v\nstdout: %s", err, stdout)
	}
	if result.KeyID != "KEY123" || result.Kind != webcore.APIKeyKindTeam || !result.Changed || result.Active || result.Status != webAPIKeyRevokeStatusRevoked {
		t.Fatalf("unexpected revoke receipt: %#v", result)
	}
}

func TestWebAPIKeysRevokeInactiveKeyIsNoOp(t *testing.T) {
	listCalls := 0
	revokeCalls := 0
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			listCalls++
			return []webcore.APIKeyListItem{{KeyID: "KEY123", Kind: kind, Active: false}}, nil
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			return nil
		},
	)
	t.Cleanup(restore)

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY123", "--type", "individual", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { execErr = cmd.Exec(context.Background(), nil) })
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if listCalls != 1 || revokeCalls != 0 {
		t.Fatalf("expected one preflight list and no revoke, got lists=%d revoke=%d", listCalls, revokeCalls)
	}
	var result asc.WebAPIKeyRevokeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v\nstdout: %s", err, stdout)
	}
	if result.KeyID != "KEY123" || result.Kind != webcore.APIKeyKindIndividual || result.Changed || result.Active || result.Status != webAPIKeyRevokeStatusAlreadyInactive {
		t.Fatalf("unexpected no-op receipt: %#v", result)
	}
}

func TestWebAPIKeysRevokeFailsWhenPostListStillActive(t *testing.T) {
	listCalls := 0
	revokeCalls := 0
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			listCalls++
			return []webcore.APIKeyListItem{{KeyID: "KEY123", Kind: kind, Active: true}}, nil
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			return nil
		},
	)
	t.Cleanup(restore)

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY123", "--type", "team", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var execErr error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { execErr = cmd.Exec(context.Background(), nil) })
	if execErr == nil || !strings.Contains(execErr.Error(), "remains active") {
		t.Fatalf("expected post-state error, got %v (stderr %q)", execErr, stderr)
	}
	if listCalls != 2 || revokeCalls != 1 {
		t.Fatalf("expected two lists and one revoke, got lists=%d revoke=%d", listCalls, revokeCalls)
	}
}

func TestWebAPIKeysRevokeReportsUnknownForServerErrorWithoutRetry(t *testing.T) {
	listCalls := 0
	revokeCalls := 0
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			listCalls++
			return []webcore.APIKeyListItem{{KeyID: "KEY123", Kind: kind, Active: true}}, nil
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			return &webcore.APIError{Status: http.StatusBadGateway}
		},
	)
	t.Cleanup(restore)

	d, _, _ := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY123", "--type", "team", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome is unknown") || !strings.Contains(err.Error(), "no automatic retry was sent") {
		t.Fatalf("expected explicit unknown outcome, got %v", err)
	}
	if listCalls != 1 || revokeCalls != 1 {
		t.Fatalf("expected one preflight list and one revoke without retry/re-read, got lists=%d revoke=%d", listCalls, revokeCalls)
	}
}

func TestWebAPIKeysRevokeReportsUnknownForPostListError(t *testing.T) {
	listCalls := 0
	revokeCalls := 0
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			listCalls++
			if listCalls == 1 {
				return []webcore.APIKeyListItem{{KeyID: "KEY123", Kind: kind, Active: true}}, nil
			}
			return nil, errors.New("verification read failed")
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			return nil
		},
	)
	t.Cleanup(restore)

	d, _, _ := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY123", "--type", "individual", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome is unknown") || !strings.Contains(err.Error(), "no automatic retry was sent") {
		t.Fatalf("expected explicit unknown post-state outcome, got %v", err)
	}
	if listCalls != 2 || revokeCalls != 1 {
		t.Fatalf("expected one preflight, one post-list, and one revoke, got lists=%d revoke=%d", listCalls, revokeCalls)
	}
}

func TestWebAPIKeysRevokeRejectsWrongKindBeforePatch(t *testing.T) {
	revokeCalls := 0
	restore := installWebAPIKeyRevokeFakes(
		t,
		func(ctx context.Context, client *webcore.Client, kind string) ([]webcore.APIKeyListItem, error) {
			return []webcore.APIKeyListItem{{KeyID: "KEY123", Kind: webcore.APIKeyKindTeam, Active: true}}, nil
		},
		func(ctx context.Context, client *webcore.Client, keyID, kind string) error {
			revokeCalls++
			return nil
		},
	)
	t.Cleanup(restore)

	d, _, _ := newWebTestDeps()
	cmd := newWebAPIKeysRevokeCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY123", "--type", "individual", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "was listed as type") {
		t.Fatalf("expected kind mismatch error, got %v", err)
	}
	if revokeCalls != 0 {
		t.Fatal("did not expect revoke after kind mismatch")
	}
}

func installWebAPIKeyRevokeFakes(t *testing.T,
	listFn func(context.Context, *webcore.Client, string) ([]webcore.APIKeyListItem, error),
	revokeFn func(context.Context, *webcore.Client, string, string) error,
) func() {
	t.Helper()
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{TeamID: "TEAM123"}, "cache", nil
	})
	originalNewClient := newWebAPIKeyClientFn
	originalList := listWebAPIKeysByKindFn
	originalRevoke := revokeWebAPIKeyFn

	newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAPIKeysByKindFn = listFn
	revokeWebAPIKeyFn = revokeFn

	return func() {
		restoreSession()
		newWebAPIKeyClientFn = originalNewClient
		listWebAPIKeysByKindFn = originalList
		revokeWebAPIKeyFn = originalRevoke
	}
}
