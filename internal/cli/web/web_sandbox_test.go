package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebSandboxCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing first name",
			args:    []string{"--last-name", "Tester", "--email", "jane@example.com", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--first-name is required",
		},
		{
			name:    "invalid email",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "bad-email", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--email must be a valid email address",
		},
		{
			name:    "display name email is rejected",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "Jane Tester <jane@example.com>", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--email must be a valid email address",
		},
		{
			name:    "invalid password",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "jane@example.com", "--password", "password", "--territory", "USA"},
			wantErr: "--password must be at least 8 characters and include uppercase, lowercase, and numeric characters",
		},
		{
			name:    "invalid territory",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "jane@example.com", "--password", "Passwordtest1", "--territory", "ZZZ"},
			wantErr: "--territory must be a valid App Store territory code",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebSandboxCreateCommand(d)
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestNormalizeWebSandboxPasswordCountsCharactersNotBytes(t *testing.T) {
	_, err := normalizeWebSandboxPassword("Aéééé1b")
	if err == nil {
		t.Fatal("expected too-short password error")
	}
	if !strings.Contains(err.Error(), "at least 8 characters") {
		t.Fatalf("expected length error, got %v", err)
	}
}

func TestWebSandboxCreateResolvesSessionBeforeTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, "", resolveErr
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxCreateCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", "Jane",
		"--last-name", "Tester",
		"--email", "jane@example.com",
		"--password", "Passwordtest1",
		"--territory", "USA",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
	}
	if hadDeadline {
		t.Fatal("expected resolveSession to run before timeout context creation")
	}
}

func TestWebSandboxCreateSubmitsNormalizedRequest(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origCreate := createWebSandboxTesterFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		createWebSandboxTesterFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var received webcore.SandboxAccountCreateAttributes
	createWebSandboxTesterFn = func(ctx context.Context, client *webcore.Client, attrs webcore.SandboxAccountCreateAttributes) error {
		received = attrs
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebSandboxCreateCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", " Jane ",
		"--last-name", " Tester ",
		"--email", "jane+sandbox@example.com",
		"--password", "Passwordtest1",
		"--territory", "usa",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result webSandboxCreateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !result.Submitted {
		t.Fatal("expected submitted=true")
	}
	if result.Territory != "USA" {
		t.Fatalf("expected territory USA, got %q", result.Territory)
	}
	if result.Email != "jane+sandbox@example.com" {
		t.Fatalf("expected email preserved, got %q", result.Email)
	}

	if received.FirstName != "Jane" {
		t.Fatalf("expected first name Jane, got %q", received.FirstName)
	}
	if received.LastName != "Tester" {
		t.Fatalf("expected last name Tester, got %q", received.LastName)
	}
	if received.AccountName != "jane+sandbox@example.com" {
		t.Fatalf("expected account name email, got %q", received.AccountName)
	}
	if received.AccountPassword != "Passwordtest1" {
		t.Fatalf("expected password preserved, got %q", received.AccountPassword)
	}
	if received.StoreFront != "USA" {
		t.Fatalf("expected storefront USA, got %q", received.StoreFront)
	}
}

func TestWebSandboxCreateWrapsAuthErrors(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origCreate := createWebSandboxTesterFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		createWebSandboxTesterFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	createWebSandboxTesterFn = func(ctx context.Context, client *webcore.Client, attrs webcore.SandboxAccountCreateAttributes) error {
		return &webcore.APIError{Status: 401}
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxCreateCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", "Jane",
		"--last-name", "Tester",
		"--email", "jane@example.com",
		"--password", "Passwordtest1",
		"--territory", "USA",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected auth-wrapped error")
	}
	if !strings.Contains(err.Error(), "web sandbox create failed: web session is unauthorized or expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebSandboxDeleteRequiresConfirmBeforeResolvingSession(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-resource-id"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil || !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected missing confirm error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirm diagnostic, got %q", stderr)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebSandboxDeleteRefusesFamilyMemberBeforeMutation(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := true
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 1,
			Accounts:      []webcore.SandboxAccount{{ID: "family-id", IsInFamily: &family}},
		}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "family-id", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "family member") {
		t.Fatalf("expected family-member refusal, got %v", err)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", listCalls)
	}
	if deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want 0", deleteCalls)
	}
}

func TestWebSandboxDeletePreflightsDeletesAndVerifies(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listResults := []*webcore.SandboxAccountListResponse{
		{
			TotalAccounts: 2,
			Accounts: []webcore.SandboxAccount{
				{ID: "tester-one", IsInFamily: &family},
				{ID: "tester-two", IsInFamily: &family},
			},
		},
		{TotalAccounts: 0, Accounts: []webcore.SandboxAccount{}},
	}
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		if listCalls >= len(listResults) {
			t.Fatalf("unexpected list call %d", listCalls+1)
		}
		result := listResults[listCalls]
		listCalls++
		return result, nil
	}
	var deletedIDs []string
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deletedIDs = append([]string(nil), ids...)
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one,tester-two,tester-one", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !reflect.DeepEqual(deletedIDs, []string{"tester-one", "tester-two"}) {
		t.Fatalf("deleted IDs = %#v", deletedIDs)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2", listCalls)
	}
	var result asc.WebSandboxDeleteResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if !result.Deleted || !reflect.DeepEqual(result.IDs, []string{"tester-one", "tester-two"}) {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestWebSandboxDeleteRefusesIncompleteListBeforeMutation(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 2,
			Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
		}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "only 1 of 2") {
		t.Fatalf("expected incomplete-list refusal, got %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want 0", deleteCalls)
	}
}

func TestWebSandboxDeleteReportsUnknownTransportOutcome(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 1,
			Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
		}, nil
	}
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		return errors.New("request failed: context deadline exceeded")
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("expected unknown-outcome error, got %v", err)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1 after ambiguous delete", listCalls)
	}
}

func TestWebSandboxDeleteTreatsServerErrorAsUnknownOutcome(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 1,
			Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
		}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return &webcore.APIError{Status: 500}
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("expected unknown-outcome error for server failure, got %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", deleteCalls)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1; command must not retry or verify after ambiguous delete", listCalls)
	}
}

func TestWebSandboxDeleteTreatsRequestTimeoutAsUnknownOutcome(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 1,
			Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
		}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return &webcore.APIError{Status: http.StatusRequestTimeout}
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("expected unknown-outcome error for request timeout, got %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", deleteCalls)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1; command must not retry or verify after ambiguous delete", listCalls)
	}
}

func TestWebSandboxDeleteTreatsVisibleAccountAfterDeleteAsUnknownOutcome(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		return &webcore.SandboxAccountListResponse{
			TotalAccounts: 1,
			Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
		}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("expected unknown-outcome error for still-visible account, got %v", err)
	}
	if !strings.Contains(err.Error(), "tester-one") {
		t.Fatalf("expected still-visible account ID in error, got %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", deleteCalls)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2 for postcondition verification", listCalls)
	}
}

func TestWebSandboxDeleteReportsVerifiedStatusWhenOutputFails(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listWebSandboxAccountsFn
	origDelete := deleteWebSandboxAccountsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listWebSandboxAccountsFn = origList
		deleteWebSandboxAccountsFn = origDelete
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	family := false
	listCalls := 0
	listWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client) (*webcore.SandboxAccountListResponse, error) {
		listCalls++
		if listCalls == 1 {
			return &webcore.SandboxAccountListResponse{
				TotalAccounts: 1,
				Accounts:      []webcore.SandboxAccount{{ID: "tester-one", IsInFamily: &family}},
			}, nil
		}
		return &webcore.SandboxAccountListResponse{TotalAccounts: 0, Accounts: []webcore.SandboxAccount{}}, nil
	}
	deleteCalls := 0
	deleteWebSandboxAccountsFn = func(ctx context.Context, client *webcore.Client, ids []string) error {
		deleteCalls++
		return nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSandboxDeleteCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--id", "tester-one", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := wOut.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	d.IO.Out = wOut
	gotErr := cmd.Exec(context.Background(), nil)
	_ = rOut.Close()

	if gotErr == nil || !strings.Contains(gotErr.Error(), "completed and verified") {
		t.Fatalf("expected verified-status output error, got %v", gotErr)
	}
	if !strings.Contains(gotErr.Error(), "tester-one") || !strings.Contains(gotErr.Error(), "do not retry") {
		t.Fatalf("expected ID and no-retry guidance, got %v", gotErr)
	}
	if deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", deleteCalls)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2 for postcondition verification", listCalls)
	}
}
