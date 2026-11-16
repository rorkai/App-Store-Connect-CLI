package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type customAppUsersCLIHTTPFixture struct {
	users             map[string]string
	nextID            string
	writeCount        int
	deleteCount       int
	postReadMismatch  bool
	malformedCreate   bool
	appDistribution   string
	requests          []string
	mutationHeadersOK bool
}

func setupCustomAppUsersCLIHTTP(t *testing.T, fixture *customAppUsersCLIHTTPFixture) {
	t.Helper()
	if fixture.users == nil {
		fixture.users = make(map[string]string)
	}
	if fixture.nextID == "" {
		fixture.nextID = "recipient-new"
	}
	if fixture.appDistribution == "" {
		fixture.appDistribution = webcore.AppDistributionTypeCustom
	}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse fixture URL: %v", err)
	}
	transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
	session := &webcore.AuthSession{Client: &http.Client{Transport: transport}}

	originalResolve := resolveSessionFn
	originalNewClient := newWebClientFn
	originalGetDistribution := getWebAppDistributionFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return session, "fixture", nil
	}
	newWebClientFn = webcore.NewClient
	getWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppDistribution, error) {
		return client.GetAppDistribution(ctx, appID)
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalNewClient
		getWebAppDistributionFn = originalGetDistribution
	})
}

type customAppUsersRewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t customAppUsersRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	requestURL := *req.URL
	requestURL.Scheme = t.target.Scheme
	requestURL.Host = t.target.Host
	clone.URL = &requestURL
	return t.base.RoundTrip(clone)
}

func (f *customAppUsersCLIHTTPFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/iris/v1")
	switch {
	case r.Method == http.MethodGet && path == "/apps/app-1":
		_, _ = io.WriteString(w, `{"data":{"type":"apps","id":"app-1","attributes":{"distributionType":"`+f.appDistribution+`","educationDiscountType":"NOT_APPLICABLE"}}}`)
	case r.Method == http.MethodGet && path == "/apps/app-1/customAppUsers":
		f.writeUsersList(w)
	case r.Method == http.MethodPost && path == "/customAppUsers":
		f.writeCount++
		if r.Header.Get("X-CSRF-ITC") != "[asc-ui]" || r.Header.Get("Referer") != "https://appstoreconnect.apple.com/apps/app-1/distribution/pricing" {
			f.mutationHeadersOK = false
		} else {
			f.mutationHeadersOK = true
		}
		var payload struct {
			Data struct {
				Attributes struct {
					AppleID string `json:"appleId"`
				} `json:"attributes"`
				Relationships struct {
					App struct {
						Data struct {
							Type string `json:"type"`
							ID   string `json:"id"`
						} `json:"data"`
					} `json:"app"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if payload.Data.Relationships.App.Data.Type != "apps" || payload.Data.Relationships.App.Data.ID != "app-1" {
			http.Error(w, "wrong app", http.StatusBadRequest)
			return
		}
		if f.malformedCreate {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"type":"customAppUsers","id":"accepted-id","attributes":null}}`)
			return
		}
		if !f.postReadMismatch {
			f.users[f.nextID] = payload.Data.Attributes.AppleID
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"customAppUsers","id":"`+f.nextID+`","attributes":{"appleId":"`+payload.Data.Attributes.AppleID+`"},"future":{"kept":true}}}`)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/customAppUsers/"):
		f.deleteCount++
		if r.Header.Get("X-CSRF-ITC") != "[asc-ui]" || r.Header.Get("Referer") != "https://appstoreconnect.apple.com/apps/app-1/distribution/pricing" {
			f.mutationHeadersOK = false
		} else {
			f.mutationHeadersOK = true
		}
		var payload struct {
			Data struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Data.Type != "customAppUsers" || payload.Data.ID == "" {
			http.Error(w, "bad delete body", http.StatusBadRequest)
			return
		}
		delete(f.users, payload.Data.ID)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected request", http.StatusNotFound)
	}
}

func (f *customAppUsersCLIHTTPFixture) writeUsersList(w http.ResponseWriter) {
	data := make([]map[string]any, 0, len(f.users))
	for id, appleID := range f.users {
		data = append(data, map[string]any{
			"type": "customAppUsers",
			"id":   id,
			"attributes": map[string]any{
				"appleId": appleID,
				"unknown": true,
			},
			"links": map[string]any{"self": "https://appstoreconnect.apple.com/iris/v1/customAppUsers/" + id},
		})
	}
	envelope := map[string]any{
		"data": data,
		"links": map[string]any{
			"self": "https://appstoreconnect.apple.com/iris/v1/apps/app-1/customAppUsers",
		},
		"meta":            map[string]any{"paging": map[string]any{"total": len(data), "limit": 100}},
		"unknownTopLevel": map[string]any{"keep": true},
	}
	encoded, _ := json.Marshal(envelope)
	_, _ = w.Write(encoded)
}

func TestWebAppsDistributionUsersCreateLifecycleUsesOneWriteAndReadback(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "new@example.com", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("execute command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt asc.WebAppDistributionUserMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.AppID != "app-1" || receipt.RecipientID != fixture.nextID || receipt.AppleID != "new@example.com" || receipt.Changed == nil || !*receipt.Changed || !receipt.Verified || receipt.Status != "created" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if fixture.writeCount != 1 || !fixture.mutationHeadersOK {
		t.Fatalf("write count=%d headersOK=%v; requests=%v", fixture.writeCount, fixture.mutationHeadersOK, fixture.requests)
	}
}

func TestWebAppsDistributionUsersCreateSkipsExactExistingAccount(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{users: map[string]string{"recipient-1": "existing@example.com"}}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "existing@example.com", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("execute command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt asc.WebAppDistributionUserMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Status != "unchanged" || receipt.Changed == nil || *receipt.Changed || !receipt.Verified || receipt.RecipientID != "recipient-1" {
		t.Fatalf("unexpected no-op receipt: %+v", receipt)
	}
	if fixture.writeCount != 0 {
		t.Fatalf("write count = %d, want no POST", fixture.writeCount)
	}
}

func TestWebAppsDistributionUsersDeleteLifecycleUsesOneWriteAndReadback(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{users: map[string]string{"recipient-1": "existing@example.com"}}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersDeleteCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--id", "recipient-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("execute command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt asc.WebAppDistributionUserMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Status != "deleted" || receipt.Changed == nil || !*receipt.Changed || !receipt.Verified || receipt.AppleID != "existing@example.com" {
		t.Fatalf("unexpected delete receipt: %+v", receipt)
	}
	if fixture.deleteCount != 1 || !fixture.mutationHeadersOK {
		t.Fatalf("delete count=%d headersOK=%v; requests=%v", fixture.deleteCount, fixture.mutationHeadersOK, fixture.requests)
	}
}

func TestWebAppsDistributionUsersDeleteAbsentIsVerifiedNoOp(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersDeleteCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--id", "missing", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("execute command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt asc.WebAppDistributionUserMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Status != "unchanged" || receipt.Changed == nil || *receipt.Changed || !receipt.Verified || receipt.RecipientID != "missing" {
		t.Fatalf("unexpected absent receipt: %+v", receipt)
	}
	if fixture.deleteCount != 0 {
		t.Fatalf("delete count = %d, want no DELETE", fixture.deleteCount)
	}
}

func TestWebAppsDistributionUsersCreateMalformedAcceptedResponseIsUncertainWithoutRetry(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{malformedCreate: true}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "new@example.com", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = command.Exec(context.Background(), nil) })
	if runErr == nil {
		t.Fatal("expected uncertain create error")
	}
	var receipt struct {
		RecipientID string `json:"recipientId"`
		Changed     *bool  `json:"changed"`
		Verified    bool   `json:"verified"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode uncertain receipt %q: %v", stdout, err)
	}
	if receipt.RecipientID != "accepted-id" || receipt.Changed != nil || receipt.Verified || receipt.Status != "uncertain" {
		t.Fatalf("unexpected uncertain receipt: %+v", receipt)
	}
	if fixture.writeCount != 1 {
		t.Fatalf("write count = %d, want exactly one POST", fixture.writeCount)
	}
}

func TestWebAppsDistributionUsersPostReadMismatchIsUncertain(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{postReadMismatch: true}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "new@example.com", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = command.Exec(context.Background(), nil) })
	if runErr == nil {
		t.Fatal("expected post-read verification error")
	}
	var receipt asc.WebAppDistributionUserMutationResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Status != "uncertain" || receipt.Changed != nil || receipt.Verified || receipt.RecipientID != fixture.nextID {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if fixture.writeCount != 1 {
		t.Fatalf("write count = %d, want one POST", fixture.writeCount)
	}
}

func TestWebAppsDistributionUsersRejectsNonCustomAppBeforeMutation(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{appDistribution: webcore.AppDistributionTypeAppStore}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "new@example.com", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	var runErr error
	_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = command.Exec(context.Background(), nil) })
	if runErr == nil || !strings.Contains(runErr.Error(), "must use CUSTOM") {
		t.Fatalf("error = %v, want CUSTOM precondition", runErr)
	}
	if fixture.writeCount != 0 || fixture.deleteCount != 0 {
		t.Fatalf("mutations write=%d delete=%d, want no mutation", fixture.writeCount, fixture.deleteCount)
	}
}

func TestWebAppsDistributionUsersListPreservesJSONAndProjectsHumanFormats(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			fixture := &customAppUsersCLIHTTPFixture{users: map[string]string{"recipient-1": "account@example.com"}}
			setupCustomAppUsersCLIHTTP(t, fixture)
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebAppsDistributionUsersListCommand(d)
			if err := command.FlagSet.Parse([]string{"--app", "app-1", "--output", format}); err != nil {
				t.Fatalf("parse command: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("execute command: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "recipient-1") || !strings.Contains(stdout, "account@example.com") {
				t.Fatalf("stdout = %q, want recipient identity", stdout)
			}
			if format == "json" && (!strings.Contains(stdout, "unknownTopLevel") || !strings.Contains(stdout, `"unknown":true`)) {
				t.Fatalf("JSON output lost raw members: %q", stdout)
			}
		})
	}
}

func TestWebAppsDistributionUsersValidatesBeforeSession(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	var resolveCalls int
	originalResolve := resolveSessionFn
	originalNewClient := newWebClientFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalNewClient
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	d, _, _ := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "account@example.com", "--confirm", "--output", "yaml"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if err := command.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "must be one of") {
		t.Fatalf("error = %v, want output validation error", err)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestCustomAppUserUncertainErrorPreservesAPIError(t *testing.T) {
	apiErr := &webcore.APIError{Status: http.StatusRequestTimeout}
	uncertain := &webcore.CustomAppUserUnverifiedError{Err: apiErr}
	if !webcore.IsCustomAppUserWriteUncertain(uncertain) {
		t.Fatal("wrapped API timeout should be uncertain")
	}
	var got *webcore.APIError
	if !errors.As(uncertain, &got) || got != apiErr {
		t.Fatalf("errors.As() = %v, want original API error", got)
	}
}

func TestWebAppsDistributionUsersCreateRejectsAmbiguousExistingAccount(t *testing.T) {
	fixture := &customAppUsersCLIHTTPFixture{users: map[string]string{"one": "same@example.com", "two": "same@example.com"}}
	setupCustomAppUsersCLIHTTP(t, fixture)
	d, _, _ := newWebTestDeps()
	command := newWebAppsDistributionUsersCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--app", "app-1", "--recipient-apple-id", "same@example.com", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "multiple recipients") || fixture.writeCount != 0 {
		t.Fatalf("writes=%d err=%v", fixture.writeCount, err)
	}
}
