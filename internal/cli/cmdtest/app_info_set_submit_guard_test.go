package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

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

const appInfoSetVersionLocalizationRelationship = `"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}`

func registerAppInfoSetVersionReads(reg *asctest.Registry, version, released string) {
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1"), asctest.JSON(http.StatusOK, version))
	if released != "" {
		reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, version))
		reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
			asctest.JSON(http.StatusOK, released))
	}
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

			reg := asctest.NewRegistry(t)
			reg.Install()
			registerAppInfoSetVersionReads(reg, test.version, test.released)
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=fr-FR,en-US", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}]}`))
			reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":`+test.created+`,`+appInfoSetVersionLocalizationRelationship+`}}`),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":`+test.created+`}}`))

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

			requests := reg.Requests()
			createBody := string(requests[len(requests)-1].Body)
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	version := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1"), asctest.JSON(http.StatusOK, version))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, version))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=fr-FR,en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

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
		localeFilter  string
		write         asctest.Matcher
		writeStatus   int
		writeResponse string
		wantStderr    []string
	}{
		{
			name:          "created locale misses base fields",
			args:          []string{"--locale", "fr-FR", "--whats-new", "Nouveautes"},
			version:       `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`,
			localizations: `{"data":[]}`,
			localeFilter:  "fr-FR",
			write:         asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"fr-FR","whatsNew":"Nouveautes"},` + appInfoSetVersionLocalizationRelationship + `}}`),
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
			localeFilter:  "en-US",
			write:         asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"Updated description"}}}`),
			writeStatus:   http.StatusOK,
			writeResponse: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Updated description","keywords":"english,keywords","supportUrl":"https://example.com/support"}}}`,
			wantStderr:    []string{"locale en-US is missing submit-required fields: whatsNew"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAppInfoSetAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			registerAppInfoSetVersionReads(reg, test.version, test.released)
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]="+test.localeFilter, "limit=200"),
				asctest.JSON(http.StatusOK, test.localizations))
			reg.Register(test.write, asctest.JSON(test.writeStatus, test.writeResponse))

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=fr-FR,en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"mots,cles","supportUrl":"https://example.com/support-fr"}},{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description EN","keywords":"english,keywords","supportUrl":"https://example.com/support-en"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-fr").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"whatsNew":"Nouveautes"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"mots,cles","supportUrl":"https://example.com/support-fr","whatsNew":"Nouveautes"}}}`))

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

	updateBody := string(reg.Requests()[2].Body)
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
