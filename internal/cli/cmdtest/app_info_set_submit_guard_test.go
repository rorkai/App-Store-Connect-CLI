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
	tests := []struct {
		name         string
		args         []string
		version      string
		released     string
		created      string
		wantStderr   []string
		rejectStderr string
	}{
		{
			name:         "new app with whats new",
			args:         []string{"--whats-new", "Nouveautes"},
			version:      `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`,
			created:      `{"locale":"fr-FR","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support","whatsNew":"Nouveautes"}`,
			rejectStderr: "submit-required fields",
		},
		{
			name:     "app update without whats new",
			version:  `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`,
			released: `{"data":[{"type":"appStoreVersions","id":"released-version","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{"next":""}}`,
			created:  `{"locale":"fr-FR","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support"}`,
			wantStderr: []string{
				"created locale fr-FR now participates in submission validation",
				"whatsNew",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAppInfoSetAuth(t)

			var createBody string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
					return appInfoSetJSONResponse(http.StatusOK, test.version)
				case test.released != "" && req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
					if !strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D=") {
						return nil, fmt.Errorf("unexpected app versions query: %s", req.URL.RawQuery)
					}
					return appInfoSetJSONResponse(http.StatusOK, test.released)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return appInfoSetJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}]}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
					body, _ := io.ReadAll(req.Body)
					createBody = string(body)
					return appInfoSetJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":`+test.created+`}}`)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append([]string{
					"apps", "info", "edit",
					"--version-id", "version-1",
					"--locale", "fr-FR",
					"--copy-from-locale", "en-US",
				}, test.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			for _, want := range []string{`"description":"English description"`, `"keywords":"english,keywords"`, `"supportUrl":"https://example.com/support"`} {
				if !strings.Contains(createBody, want) {
					t.Fatalf("expected copied %s in request body, got: %s", want, createBody)
				}
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got: %q", want, stderr)
				}
			}
			if test.rejectStderr != "" && strings.Contains(stderr, test.rejectStderr) {
				t.Fatalf("did not expect %q in stderr, got: %q", test.rejectStderr, stderr)
			}

			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
			}
		})
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
	tests := []struct {
		name          string
		args          []string
		version       string
		released      string
		localizations string
		writeMethod   string
		writePath     string
		writeStatus   int
		writeResponse string
		wantStderr    []string
	}{
		{
			name:          "created locale misses base fields",
			args:          []string{"--locale", "fr-FR", "--whats-new", "Nouveautes"},
			version:       `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`,
			localizations: `{"data":[]}`,
			writeMethod:   http.MethodPost,
			writePath:     "/v1/appStoreVersionLocalizations",
			writeStatus:   http.StatusCreated,
			writeResponse: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","whatsNew":"Nouveautes"}}}`,
			wantStderr:    []string{"created locale fr-FR now participates in submission validation", "description", "keywords", "supportUrl"},
		},
		{
			name:          "updated locale misses whats new on app update",
			args:          []string{"--locale", "en-US", "--description", "Updated description"},
			version:       `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`,
			released:      `{"data":[{"type":"appStoreVersions","id":"released-version","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{"next":""}}`,
			localizations: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}]}`,
			writeMethod:   http.MethodPatch,
			writePath:     "/v1/appStoreVersionLocalizations/loc-en",
			writeStatus:   http.StatusOK,
			writeResponse: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Updated description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}}`,
			wantStderr:    []string{"locale en-US is missing submit-required fields: whatsNew"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAppInfoSetAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
					return appInfoSetJSONResponse(http.StatusOK, test.version)
				case test.released != "" && req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
					return appInfoSetJSONResponse(http.StatusOK, test.released)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return appInfoSetJSONResponse(http.StatusOK, test.localizations)
				case req.Method == test.writeMethod && req.URL.Path == test.writePath:
					return appInfoSetJSONResponse(test.writeStatus, test.writeResponse)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(append([]string{"apps", "info", "edit", "--version-id", "version-1"}, test.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got: %q", want, stderr)
				}
			}
		})
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
