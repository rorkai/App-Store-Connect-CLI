package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type appInfoSetRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn appInfoSetRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupAppInfoSetAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func appInfoSetJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestAppInfoSetCopyFromLocaleBackfillsRequiredFields(t *testing.T) {
	setupAppInfoSetAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var createBody string
	http.DefaultTransport = appInfoSetRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body, _ := io.ReadAll(req.Body)
			createBody = string(body)
			return appInfoSetJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support","whatsNew":"Nouveautes"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--version-id", "version-1",
			"--locale", "fr-FR",
			"--copy-from-locale", "en-US",
			"--whats-new", "Nouveautes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(createBody, `"description":"English description"`) {
		t.Fatalf("expected copied description in request body, got: %s", createBody)
	}
	if !strings.Contains(createBody, `"keywords":"english,keywords"`) {
		t.Fatalf("expected copied keywords in request body, got: %s", createBody)
	}
	if !strings.Contains(createBody, `"supportUrl":"https://example.com/support"`) {
		t.Fatalf("expected copied supportUrl in request body, got: %s", createBody)
	}
	if strings.Contains(stderr, "submit-required fields") {
		t.Fatalf("did not expect submit-required warning for copied locale, got: %q", stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
}

func TestAppInfoSetCopyFromLocaleErrorsWhenSourceLocaleMissing(t *testing.T) {
	setupAppInfoSetAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = appInfoSetRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--version-id", "version-1",
			"--locale", "fr-FR",
			"--copy-from-locale", "en-US",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error when copy source locale does not exist")
	}
	if !strings.Contains(runErr.Error(), "copy-from-locale") || !strings.Contains(runErr.Error(), "en-US") {
		t.Fatalf("expected copy-from-locale error, got: %v", runErr)
	}
}

func TestAppInfoSetWarnsWhenLocaleRemainsSubmitIncomplete(t *testing.T) {
	setupAppInfoSetAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = appInfoSetRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			return appInfoSetJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","whatsNew":"Nouveautes"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--version-id", "version-1",
			"--locale", "fr-FR",
			"--whats-new", "Nouveautes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: locale fr-FR is missing submit-required fields") {
		t.Fatalf("expected submit-required warning in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "description") || !strings.Contains(stderr, "keywords") || !strings.Contains(stderr, "supportUrl") {
		t.Fatalf("expected missing field list in warning, got: %q", stderr)
	}
}

func TestAppInfoSetCopyFromLocaleDoesNotOverwriteExistingTargetFields(t *testing.T) {
	setupAppInfoSetAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var updateBody string
	http.DefaultTransport = appInfoSetRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return appInfoSetJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"mots,cles","supportUrl":"https://example.com/support-fr"}},{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description EN","keywords":"english,keywords","supportUrl":"https://example.com/support-en"}}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-fr":
			body, _ := io.ReadAll(req.Body)
			updateBody = string(body)
			return appInfoSetJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"mots,cles","supportUrl":"https://example.com/support-fr","whatsNew":"Nouveautes"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--version-id", "version-1",
			"--locale", "fr-FR",
			"--copy-from-locale", "en-US",
			"--whats-new", "Nouveautes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if strings.Contains(updateBody, `"description":"Description EN"`) {
		t.Fatalf("expected target description to be preserved, got update body: %s", updateBody)
	}
	if strings.Contains(updateBody, `"keywords":"english,keywords"`) {
		t.Fatalf("expected target keywords to be preserved, got update body: %s", updateBody)
	}
	if strings.Contains(updateBody, `"supportUrl":"https://example.com/support-en"`) {
		t.Fatalf("expected target supportUrl to be preserved, got update body: %s", updateBody)
	}
	if !strings.Contains(updateBody, `"whatsNew":"Nouveautes"`) {
		t.Fatalf("expected whatsNew update to be present, got update body: %s", updateBody)
	}
	if strings.Contains(stderr, "submit-required fields") {
		t.Fatalf("did not expect submit-required warning when target locale is complete, got: %q", stderr)
	}
}
