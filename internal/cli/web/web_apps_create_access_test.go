package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const frozenWebAppCreateRequestBody = `{"data":{"type":"apps","attributes":{"sku":"SKU123","primaryLocale":"en-US","bundleId":"com.example.app"},"relationships":{"appStoreVersions":{"data":[{"type":"appStoreVersions","id":"${new-appStoreVersion}"}]},"appInfos":{"data":[{"type":"appInfos","id":"${new-appInfo}"}]}}},"included":[{"type":"appStoreVersions","id":"${new-appStoreVersion}","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"appStoreVersionLocalizations":{"data":[{"type":"appStoreVersionLocalizations","id":"${new-appStoreVersionLocalization}"}]}}},{"type":"appStoreVersionLocalizations","id":"${new-appStoreVersionLocalization}","attributes":{"locale":"en-US"}},{"type":"appInfos","id":"${new-appInfo}","relationships":{"appInfoLocalizations":{"data":[{"type":"appInfoLocalizations","id":"${new-appInfoLocalization}"}]}}},{"type":"appInfoLocalizations","id":"${new-appInfoLocalization}","attributes":{"locale":"en-US","name":"My App"}}]}`

func TestRunAppsCreateAccessLimitedWithoutUserMakesNoHTTP(t *testing.T) {
	createCalled := false
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			Access:   "limited",
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected limited-without-user error")
	}
	if !strings.Contains(errfmt.FormatStderr(err), "--access limited requires at least one --user") &&
		!strings.Contains(stderr, "--access limited requires at least one --user") {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
	if createCalled {
		t.Fatal("create should not run when --user is missing")
	}
}

func TestRunAppsCreateInvalidAccessFailsBeforeWizard(t *testing.T) {
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	origCanPrompt := appCreateCanPromptForFieldsFn
	origAskOne := appCreateAskOneFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
		appCreateCanPromptForFieldsFn = origCanPrompt
		appCreateAskOneFn = origAskOne
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		t.Fatal("did not expect create")
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}
	appCreateCanPromptForFieldsFn = func() bool { return true }
	appCreateAskOneFn = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
		t.Fatal("did not expect app-details wizard")
		return nil
	}

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Access: "limited",
			Output: "json",
		})
	})
	if err == nil {
		t.Fatal("expected limited-without-user error")
	}
	if !strings.Contains(errfmt.FormatStderr(err), "--access limited requires at least one --user") &&
		!strings.Contains(stderr, "--access limited requires at least one --user") {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
}

func TestRunAppsCreateAccessWithMissingNameMakesNoHTTP(t *testing.T) {
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	origCanPrompt := appCreateCanPromptForFieldsFn
	origClientFactory := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		t.Fatal("did not expect ASC client lookup")
		return nil, nil
	})
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
		appCreateCanPromptForFieldsFn = origCanPrompt
		origClientFactory()
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		t.Fatal("did not expect create")
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}
	appCreateCanPromptForFieldsFn = func() bool { return false }

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Access: "full",
			Output: "json",
		})
	})
	if err == nil {
		t.Fatal("expected missing required flags error")
	}
	if !strings.Contains(errfmt.FormatStderr(err), "missing required flags") &&
		!strings.Contains(stderr, "missing required flags") {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
}

func TestRunAppsCreateBlankUserMakesNoHTTP(t *testing.T) {
	createCalled := false
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			Access:   "limited",
			Users:    []string{"user-1", " "},
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected blank --user error")
	}
	if !strings.Contains(errfmt.FormatStderr(err), "--user must not be empty") &&
		!strings.Contains(stderr, "--user must not be empty") {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
	if createCalled {
		t.Fatal("create should not run when --user is blank")
	}
}

func TestRunAppsCreateUnknownUserMakesNoCreate(t *testing.T) {
	createCalled := false
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/missing-user" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			Access:   "limited",
			Users:    []string{"missing-user"},
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected unknown user error")
	}
	if !strings.Contains(stderr, `unknown user ID "missing-user"`) {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
	if createCalled {
		t.Fatal("create should not run for an unknown user")
	}
}

func TestRunAppsCreateAllAppsVisibleUserMakesNoCreate(t *testing.T) {
	createCalled := false
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/full-user" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"full-user","attributes":{"username":"full@example.com","allAppsVisible":true}}}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			Access:   "limited",
			Users:    []string{"full-user"},
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected all-apps-visible user error")
	}
	if !strings.Contains(stderr, `user ID "full-user" has access to all apps`) &&
		!strings.Contains(err.Error(), `user ID "full-user" has access to all apps`) {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
	if createCalled {
		t.Fatal("create should not run for a user with access to all apps")
	}
}

func TestRunAppsCreateFullAccessUnauthorizedMakesNoCreate(t *testing.T) {
	createCalled := false
	origCreate := createWebAppFn
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		createWebAppFn = origCreate
		resolveAppCreateSessionFn = origResolve
	})
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"status":"401","code":"NOT_AUTHORIZED","title":"Authentication credentials are missing or invalid."}]}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			Access:   "full",
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected unauthorized access probe to fail")
	}
	if !strings.Contains(err.Error(), "--access requires working App Store Connect API authentication") &&
		!strings.Contains(stderr, "--access requires working App Store Connect API authentication") {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
	if createCalled {
		t.Fatal("create should not run when the Users API probe fails")
	}
}

func TestRunAppsCreateOmittingAccessFlagsKeepsCreateBody(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
	})

	var requestBodies []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		requestBodies = append(requestBodies, string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	if err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
		AppleID:  "user@example.com",
		Output:   "json",
	}); err != nil {
		t.Fatalf("RunAppsCreate error: %v", err)
	}
	if len(requestBodies) != 1 {
		t.Fatalf("expected 1 create request, got %d", len(requestBodies))
	}
	if requestBodies[0] != frozenWebAppCreateRequestBody {
		t.Fatalf("create body changed\ngot:  %s\nwant: %s", requestBodies[0], frozenWebAppCreateRequestBody)
	}
}

func TestRunAppsCreateLimitedAccessReceiptUsesRereadNotRequest(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
	})

	var createBodies []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		createBodies = append(createBodies, string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	fixture := handlertest.New(t)
	var visibleAppsPosted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"username":"one@example.com","allAppsVisible":false}}}`))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/users/user-1/relationships/visibleApps":
			visibleAppsPosted = true
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users":
			if req.URL.Query().Get("filter[visibleApps]") != "app-123" {
				fixture.Respond(w, "unexpected users filter: %s", req.URL.RawQuery)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"type":"users","id":"user-9","attributes":{"username":"nine@example.com"}}],"links":{}}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	var err error
	stdout, _ := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			AppleID:  "user@example.com",
			Access:   "limited",
			Users:    []string{"user-1"},
			Output:   "json",
		})
	})
	if err != nil {
		t.Fatalf("RunAppsCreate error: %v", err)
	}
	if len(createBodies) != 1 || createBodies[0] != frozenWebAppCreateRequestBody {
		t.Fatalf("create body = %v, want captured contract", createBodies)
	}
	if !visibleAppsPosted {
		t.Fatal("expected POST visibleApps for the requested user")
	}

	var receipt asc.WebAppCreateResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.ID != "app-123" {
		t.Fatalf("id = %q, want app-123", receipt.ID)
	}
	if receipt.Access != "limited" {
		t.Fatalf("access = %q, want limited from re-read", receipt.Access)
	}
	if len(receipt.Users) != 1 || receipt.Users[0] != "user-9" {
		t.Fatalf("users = %v, want [user-9] from re-read", receipt.Users)
	}
}

func TestRunAppsCreateLimitedAccessRollsBackGrantedUsersWhenLaterGrantFails(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
	})

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	fixture := handlertest.New(t)
	posted := make([]string, 0, 2)
	deleted := make([]string, 0, 2)
	rereadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"username":"one@example.com"}}}`))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-2","attributes":{"username":"two@example.com"}}}`))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/users/user-1/relationships/visibleApps":
			posted = append(posted, "user-1")
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/users/user-2/relationships/visibleApps":
			posted = append(posted, "user-2")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred."}]}`))
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/users/user-1/relationships/visibleApps":
			deleted = append(deleted, "user-1")
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/users/user-2/relationships/visibleApps":
			deleted = append(deleted, "user-2")
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users":
			rereadCalled = true
			fixture.Respond(w, "did not expect access re-read after a failed grant")
		default:
			fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			AppleID:  "user@example.com",
			Access:   "limited",
			Users:    []string{"user-1", "user-2"},
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected later grant failure")
	}
	if len(posted) != 2 || posted[0] != "user-1" || posted[1] != "user-2" {
		t.Fatalf("posted grants = %v, want user-1 then user-2", posted)
	}
	if len(deleted) != 2 || deleted[0] != "user-1" || deleted[1] != "user-2" {
		t.Fatalf("rolled back users = %v, want [user-1 user-2]", deleted)
	}
	if rereadCalled {
		t.Fatal("did not expect access re-read after a failed grant")
	}
	if !strings.Contains(err.Error(), `grant app access to user "user-2"`) &&
		!strings.Contains(stderr, `grant app access to user "user-2"`) {
		t.Fatalf("stderr = %q err = %v", stderr, err)
	}
}

func TestRunAppsCreateLimitedAccessJoinsRollbackErrorWhenCompensationFails(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
	})

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-1","attributes":{"username":"one@example.com"}}}`))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"user-2","attributes":{"username":"two@example.com"}}}`))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/users/user-1/relationships/visibleApps":
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/users/user-2/relationships/visibleApps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred."}]}`))
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/users/user-1/relationships/visibleApps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred."}]}`))
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/users/user-2/relationships/visibleApps":
			w.WriteHeader(http.StatusNoContent)
		default:
			fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	var err error
	_, stderr := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			AppleID:  "user@example.com",
			Access:   "limited",
			Users:    []string{"user-1", "user-2"},
			Output:   "json",
		})
	})
	if err == nil {
		t.Fatal("expected grant failure joined with rollback failure")
	}
	combined := err.Error() + "\n" + stderr
	if !strings.Contains(combined, `grant app access to user "user-2"`) {
		t.Fatalf("missing original grant error: stderr = %q err = %v", stderr, err)
	}
	if !strings.Contains(combined, "manual access repair may be required") {
		t.Fatalf("missing compensation failure: stderr = %q err = %v", stderr, err)
	}
}

func TestRunAppsCreateFullAccessDoesNotPostVisibleApps(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
	})

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"links":{}}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	var err error
	stdout, _ := captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), AppsCreateRunOptions{
			Name:     "My App",
			BundleID: "com.example.app",
			SKU:      "SKU123",
			AppleID:  "user@example.com",
			Access:   "full",
			Output:   "json",
		})
	})
	if err != nil {
		t.Fatalf("RunAppsCreate error: %v", err)
	}

	var receipt asc.WebAppCreateResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	if receipt.Access != "full" {
		t.Fatalf("access = %q, want full from re-read", receipt.Access)
	}
	if receipt.Users == nil {
		t.Fatal("expected users array, got null")
	}
	if len(receipt.Users) != 0 {
		t.Fatalf("users = %v, want empty from re-read", receipt.Users)
	}
}

func TestRollbackAppCreateVisibleAppsUsesFreshTimeoutWhenParentCanceled(t *testing.T) {
	fixture := handlertest.New(t)
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodDelete && req.URL.Path == "/v1/users/user-1/relationships/visibleApps" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	client := newAppCreateTestClient(t, server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := rollbackAppCreateVisibleApps(ctx, client, "app-123", []string{"user-1"}); err != nil {
		t.Fatalf("rollback error: %v", err)
	}
	if !deleted {
		t.Fatal("expected compensating DELETE despite canceled parent context")
	}
}

func setAppCreateASCClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := newAppCreateTestClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func stubBundleIDPreflight(t *testing.T) {
	t.Helper()
	orig := ensureBundleIDFn
	t.Cleanup(func() { ensureBundleIDFn = orig })
	ensureBundleIDFn = func(context.Context, string, string, string) (bool, error) {
		return false, nil
	}
}

func newAppCreateTestClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeAppCreateTestPEM(t, keyPath)
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	return client
}

func writeAppCreateTestPEM(t *testing.T, path string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}
