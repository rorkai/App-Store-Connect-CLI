package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMetadataPushApplyDeleteGuardrails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}],"links":{"next":""}}`))

	t.Run("requires allow-deletes", func(t *testing.T) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", runErr)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "Error: --allow-deletes is required to apply delete operations") {
			t.Fatalf("expected allow-deletes error, got %q", stderr)
		}
	})

	t.Run("requires confirm", func(t *testing.T) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
				"--allow-deletes",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", runErr)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "Error: --confirm is required when applying delete operations") {
			t.Fatalf("expected confirm error, got %q", stderr)
		}
	})
}

func TestMetadataPushDryRunBuildsPlanWithoutMutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description","keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write version en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"日本語説明"}`), 0o644); err != nil {
		t.Fatalf("write version ja file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-1","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}},
					{"type":"appInfoLocalizations","id":"loc-app-2","attributes":{"locale":"fr","name":"App FR"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-1","attributes":{"locale":"en-US","description":"Remote description","marketingUrl":"https://example.com/remote"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "creating locale ja would make it participate in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	for _, want := range []string{"keywords, supportUrl", "Fill the remaining metadata before submission."} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}

	var payload struct {
		Adds []struct {
			Key string `json:"key"`
		} `json:"adds"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	if len(payload.Adds) != 2 {
		t.Fatalf("expected 2 adds, got %d (%+v)", len(payload.Adds), payload.Adds)
	}
	if len(payload.Updates) != 2 {
		t.Fatalf("expected 2 updates, got %d (%+v)", len(payload.Updates), payload.Updates)
	}
	if len(payload.Deletes) != 1 {
		t.Fatalf("expected 1 delete, got %d (%+v)", len(payload.Deletes), payload.Deletes)
	}
}

func TestMetadataPushVersionLocaleCreateWarnings(t *testing.T) {
	const (
		versionList  = `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
		emptyList    = `{"data":[],"links":{"next":""}}`
		releasedList = `{"data":[{"type":"appStoreVersions","id":"released-version","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{"next":""}}`
	)
	tests := []struct {
		name            string
		localeFile      string
		dryRun          bool
		releasedBody    string
		createdAttrs    string
		wantStderr      []string
		wantEmptyStderr bool
		check           func(t *testing.T, stdout string)
	}{
		{
			name:            "dry run complete create does not warn",
			localeFile:      `{"description":"日本語説明","keywords":"一,二","supportUrl":"https://example.com/ja"}`,
			dryRun:          true,
			releasedBody:    emptyList,
			wantEmptyStderr: true,
			check: func(t *testing.T, stdout string) {
				var payload struct {
					Adds []struct {
						Key string `json:"key"`
					} `json:"adds"`
				}
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
				}
				if len(payload.Adds) != 3 {
					t.Fatalf("expected three create-plan fields, got %+v", payload.Adds)
				}
			},
		},
		{
			name:         "apply incomplete create warns",
			localeFile:   `{"description":"日本語説明"}`,
			releasedBody: versionList,
			createdAttrs: `{"locale":"ja","description":"日本語説明"}`,
			wantStderr:   []string{"created locale ja now participates in submission validation", "keywords, supportUrl", "Fill the remaining metadata before submission."},
			check: func(t *testing.T, stdout string) {
				var payload struct {
					Applied bool `json:"applied"`
					Actions []struct {
						Action string `json:"action"`
						Scope  string `json:"scope"`
						Locale string `json:"locale"`
					} `json:"actions"`
				}
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
				}
				if !payload.Applied {
					t.Fatalf("expected applied result, got %+v", payload)
				}
				if len(payload.Actions) != 1 || payload.Actions[0].Action != "create" || payload.Actions[0].Locale != "ja" || payload.Actions[0].Scope != "version" {
					t.Fatalf("expected single version create action, got %+v", payload.Actions)
				}
			},
		},
		{
			name:         "apply update create requires whats new",
			localeFile:   `{"description":"日本語説明","keywords":"日本語,キーワード","supportUrl":"https://example.com/support-ja"}`,
			releasedBody: releasedList,
			createdAttrs: `{"locale":"ja","description":"日本語説明","keywords":"日本語,キーワード","supportUrl":"https://example.com/support-ja"}`,
			wantStderr:   []string{"created locale ja now participates in submission validation", "whatsNew"},
			check: func(t *testing.T, stdout string) {
				var payload struct {
					Applied bool `json:"applied"`
				}
				if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
					t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
				}
				if !payload.Applied {
					t.Fatalf("expected applied result, got %+v", payload)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
				t.Fatalf("mkdir version: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(test.localeFile), 0o644); err != nil {
				t.Fatalf("write version ja file: %v", err)
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"), asctest.JSON(http.StatusOK, versionList))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, emptyList))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
				asctest.JSON(http.StatusOK, test.releasedBody))
			if test.releasedBody == emptyList {
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
					asctest.JSON(http.StatusOK, test.releasedBody))
			}
			if !test.dryRun {
				reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":`+test.createdAttrs+`,"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-ja","attributes":`+test.createdAttrs+`}}`))
			}

			args := []string{"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--dir", dir}
			if test.dryRun {
				args = append(args, "--dry-run")
			}
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			wantCreates := 1
			if test.dryRun {
				wantCreates = 0
			}
			createCount := 0
			for _, req := range reg.Requests() {
				if req.Method == http.MethodPost {
					createCount++
				}
			}
			if createCount != wantCreates {
				t.Fatalf("expected %d create API calls, got %d", wantCreates, createCount)
			}
			if test.wantEmptyStderr && stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
			test.check(t, stdout)
		})
	}
}

func TestMetadataPushRejectsDuplicateCanonicalLocaleFiles(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "de-DE.json"), []byte(`{"keywords":"eins,zwei"}`), 0o644); err != nil {
		t.Fatalf("write de-DE file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "de.json"), []byte(`{"keywords":"drei,vier"}`), 0o644); err != nil {
		t.Fatalf("write de file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`duplicate canonical locale "de-DE"`, `"de-DE.json"`, `"de.json"`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestMetadataPushDryRunOmittedFieldsDoNotPlanDeletes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}
				],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	for _, item := range payload.Deletes {
		if item.Key == "app-info:en-US:subtitle" {
			t.Fatalf("expected omitted subtitle to be no-op, got deletes=%+v", payload.Deletes)
		}
	}
}

func TestMetadataPushDryRunAcceptsCaseInsensitiveKeys(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"Name":"App Name","SubTitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"Description":"Local description","Whatsnew":"Local whats new"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}
				],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Adds []struct {
			Key string `json:"key"`
		} `json:"adds"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	containsKey := func(items []struct {
		Key string `json:"key"`
	}, want string,
	) bool {
		for _, item := range items {
			if item.Key == want {
				return true
			}
		}
		return false
	}

	if !containsKey(payload.Updates, "app-info:en-US:subtitle") {
		t.Fatalf("expected app-info subtitle update with canonical key, got updates=%+v", payload.Updates)
	}
	if !containsKey(payload.Updates, "version:1.2.3:en-US:description") {
		t.Fatalf("expected version description update with canonical key, got updates=%+v", payload.Updates)
	}
	if !containsKey(payload.Adds, "version:1.2.3:en-US:whatsNew") {
		t.Fatalf("expected version whatsNew add with canonical key, got adds=%+v", payload.Adds)
	}
}

func TestMetadataPushDryRunUsesDefaultLocaleFallback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "default.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"Remote EN"}},
					{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote EN description"}}
				],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Adds    []struct{} `json:"adds"`
		Updates []struct{} `json:"updates"`
		Deletes []struct{} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Deletes) != 0 {
		t.Fatalf("expected default fallback to prevent deletes, got %+v", payload.Deletes)
	}
	if len(payload.Updates) == 0 {
		t.Fatalf("expected updates from default fallback, got %+v", payload)
	}
}

func TestMetadataPushDryRunAllowDeletesDisablesDefaultLocaleFallback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "default.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"Remote EN"}},
					{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote EN description"}}
				],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
			"--allow-deletes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Deletes []struct {
			Key string `json:"key"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	foundAppInfoDelete := false
	for _, item := range payload.Deletes {
		if item.Key == "app-info:fr:name" {
			foundAppInfoDelete = true
			break
		}
	}
	if !foundAppInfoDelete {
		t.Fatalf("expected delete for remote-only locale when allow-deletes is set, got %+v", payload.Deletes)
	}
}

// appleWhatsNewStateError is App Store Connect's answer, captured live on
// 2026-11-22, to a PATCH that includes whatsNew on an app's first version
// (PREPARE_FOR_SUBMISSION, never released). POST accepts whatsNew there.
const appleWhatsNewStateError = `{
  "errors" : [ {
    "id" : "2e1ef0a6-606d-463c-954f-26c2b8c567a9",
    "status" : "409",
    "code" : "STATE_ERROR",
    "title" : "The request cannot be fulfilled because of the state of another resource.",
    "detail" : "Attribute 'whatsNew' cannot be edited at this time"
  } ]
}`

// An app's first version can hold whatsNew (written when the locale was
// created) that App Store Connect refuses to PATCH. Editing another field
// must not resend the unchanged whatsNew, or the whole update fails with 409.
func TestMetadataPushPatchesOnlyChangedFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.0"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.0", "it.json"), []byte(`{"description":"Nuova descrizione","whatsNew":"Prima versione."}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"locale":"it","description":"Descrizione","whatsNew":"Prima versione."}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ver-it"), func(req *http.Request) (*http.Response, error) {
		requests := reg.Requests()
		if strings.Contains(string(requests[len(requests)-1].Body), "whatsNew") {
			return asctest.JSON(http.StatusConflict, appleWhatsNewStateError)(req)
		}
		return asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"locale":"it","description":"Nuova descrizione","whatsNew":"Prima versione."}}}`)(req)
	})

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "metadata", "push", "--app", "app-1", "--version", "1.0", "--dir", dir, "--output", "json")
	if runErr != nil {
		t.Fatalf("run error: %v\nstdout=%s\nstderr=%s", runErr, stdout, stderr)
	}
	var patches []string
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patches = append(patches, string(req.Body))
		}
	}
	want := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"description":"Nuova descrizione"}}}`
	if len(patches) != 1 || strings.TrimSpace(patches[0]) != want {
		t.Fatalf("PATCH bodies = %q, want [%q]", patches, want)
	}
	var payload struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied || payload.Succeeded != 1 {
		t.Fatalf("applied=%t succeeded=%d, want applied with 1 success; stdout=%s", payload.Applied, payload.Succeeded, stdout)
	}
}

// appleLiveVersionStateError has the shape of App Store Connect's 409 for a
// PATCH on a live version, built from the details agents received: every
// locked field is its own STATE_ERROR entry.
const appleLiveVersionStateError = `{"errors":[
  {"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Attribute 'description' cannot be edited at this time"},
  {"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Attribute 'whatsNew' cannot be edited at this time"}
]}`

// App Store Connect has no What's New for an app's first version and refuses
// a PATCH that sets it. Push leaves whatsNew out, sends the other fields, and
// says so, instead of failing the locale.
func TestMetadataPushLeavesOutRefusedFirstVersionWhatsNew(t *testing.T) {
	const descriptionOnly = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"description":"Nuova descrizione"}}}`
	const withWhatsNew = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"description":"Nuova descrizione","whatsNew":"Prima versione."}}}`
	const whatsNewOnly = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"whatsNew":"Prima versione."}}}`
	const onlyFirstVersion = `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`
	const warning = "Warning: whatsNew was not set for locale it of version 1.0: App Store Connect does not allow What's New on an app's first version (409 STATE_ERROR). The other fields were pushed.\n"

	tests := []struct {
		name        string
		local       string
		conflict    string
		wantErr     bool
		wantPatches []string
		wantStatus  string
		wantSkipped []string
		wantStderr  string
		// state is the version's appVersionState; empty means PREPARE_FOR_SUBMISSION.
		state string
		// appVersions answers the one read of the app's versions push makes to
		// confirm a first version; empty means push must not read them.
		appVersions string
		// appVersionsStatus is the status of that read; zero means 200.
		appVersionsStatus int
		// retryConflict answers the PATCH sent again without whatsNew.
		retryConflict string
		// wantActionError is a substring the failed action's error must contain.
		wantActionError string
	}{
		{
			name:        "other fields are pushed without whatsNew",
			local:       `{"description":"Nuova descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			wantPatches: []string{withWhatsNew, descriptionOnly},
			wantStatus:  "succeeded",
			wantSkipped: []string{"whatsNew"},
			wantStderr:  warning,
			appVersions: onlyFirstVersion,
		},
		{
			name:        "only whatsNew changed",
			local:       `{"description":"Descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			wantPatches: []string{whatsNewOnly},
			wantStatus:  "skipped",
			wantSkipped: []string{"whatsNew"},
			wantStderr:  strings.Replace(warning, " The other fields were pushed.", " Nothing else changed.", 1),
			appVersions: onlyFirstVersion,
		},
		{
			name:        "other locked fields still fail",
			local:       `{"description":"Nuova descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleLiveVersionStateError,
			wantErr:     true,
			wantPatches: []string{withWhatsNew},
			wantStatus:  "failed",
		},
		{
			// A live later version locks whatsNew with the same 409; that edit must fail.
			name:        "live version after a released one",
			local:       `{"description":"Descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			state:       "READY_FOR_SALE",
			appVersions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_SALE"}},{"type":"appStoreVersions","id":"version-0","attributes":{"versionString":"0.9","platform":"IOS","appVersionState":"REPLACED_WITH_NEW_VERSION"}}],"links":{"next":""}}`,
			wantErr:     true,
			wantPatches: []string{whatsNewOnly},
			wantStatus:  "failed",
		},
		{
			// A live first version still has no What's New; the local file
			// keeps the whatsNew the remote never stores, so other edits
			// must still go through.
			name:        "live first version",
			local:       `{"description":"Nuova descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			state:       "READY_FOR_SALE",
			appVersions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_SALE"}}],"links":{"next":""}}`,
			wantPatches: []string{withWhatsNew, descriptionOnly},
			wantStatus:  "succeeded",
			wantSkipped: []string{"whatsNew"},
			wantStderr:  warning,
		},
		{
			name:        "editable version after a released one",
			local:       `{"description":"Descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			appVersions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}},{"type":"appStoreVersions","id":"version-0","attributes":{"versionString":"0.9","platform":"IOS","appVersionState":"READY_FOR_SALE"}}],"links":{"next":""}}`,
			wantErr:     true,
			wantPatches: []string{whatsNewOnly},
			wantStatus:  "failed",
		},
		{
			name:              "versions read fails",
			local:             `{"description":"Descrizione","whatsNew":"Prima versione."}`,
			conflict:          appleWhatsNewStateError,
			appVersions:       `{"errors":[{"status":"400","code":"PARAMETER_ERROR.INVALID","title":"A parameter has an invalid value","detail":"bad"}]}`,
			appVersionsStatus: http.StatusBadRequest,
			wantErr:           true,
			wantPatches:       []string{whatsNewOnly},
			wantStatus:        "failed",
			wantActionError:   "could not confirm that version 1.0 is the app's first version",
		},
		{
			name:        "more versions than one page",
			local:       `{"description":"Descrizione","whatsNew":"Prima versione."}`,
			conflict:    appleWhatsNewStateError,
			appVersions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=x"}}`,
			wantErr:     true,
			wantPatches: []string{whatsNewOnly},
			wantStatus:  "failed",
		},
		{
			name:          "other fields fail without whatsNew",
			local:         `{"description":"Nuova descrizione","whatsNew":"Prima versione."}`,
			conflict:      appleWhatsNewStateError,
			retryConflict: appleLiveVersionStateError,
			appVersions:   onlyFirstVersion,
			wantErr:       true,
			wantPatches:   []string{withWhatsNew, descriptionOnly},
			wantStatus:    "failed",
			wantSkipped:   []string{"whatsNew"},
			wantStderr:    strings.Replace(warning, " The other fields were pushed.", " Pushing the other fields failed; see the action error.", 1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Chdir(t.TempDir())

			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "version", "1.0"), 0o755); err != nil {
				t.Fatalf("mkdir version: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "version", "1.0", "it.json"), []byte(test.local), 0o644); err != nil {
				t.Fatalf("write version file: %v", err)
			}

			state := test.state
			if state == "" {
				state = "PREPARE_FOR_SUBMISSION"
			}
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.0", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"`+state+`"}}],"links":{"next":""}}`))
			if test.appVersions != "" {
				status := test.appVersionsStatus
				if status == 0 {
					status = http.StatusOK
				}
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "limit=200"), asctest.JSON(status, test.appVersions))
			}
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"locale":"it","description":"Descrizione"}}],"links":{"next":""}}`))
			apple := func(req *http.Request) (*http.Response, error) {
				requests := reg.Requests()
				if strings.Contains(string(requests[len(requests)-1].Body), "whatsNew") {
					return asctest.JSON(http.StatusConflict, test.conflict)(req)
				}
				if test.retryConflict != "" {
					return asctest.JSON(http.StatusConflict, test.retryConflict)(req)
				}
				return asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"locale":"it","description":"Nuova descrizione"}}}`)(req)
			}
			for range test.wantPatches {
				reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ver-it"), apple)
			}
			if test.wantErr {
				// After a failed PATCH, push reads the locale back to learn whether it applied.
				reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-it","attributes":{"locale":"it","description":"Descrizione"}}],"links":{"next":""}}`))
			}

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "metadata", "push", "--app", "app-1", "--version", "1.0", "--dir", dir, "--output", "json")
			if (runErr != nil) != test.wantErr {
				t.Fatalf("run error = %v, wantErr %t\nstdout=%s\nstderr=%s", runErr, test.wantErr, stdout, stderr)
			}
			var patches []string
			for _, req := range reg.Requests() {
				if req.Method == http.MethodPatch {
					patches = append(patches, strings.TrimSpace(string(req.Body)))
				}
			}
			if !slices.Equal(patches, test.wantPatches) {
				t.Fatalf("PATCH bodies = %q, want %q", patches, test.wantPatches)
			}
			var payload struct {
				Actions []struct {
					Status        string   `json:"status"`
					SkippedFields []string `json:"skippedFields"`
					Error         string   `json:"error"`
				} `json:"actions"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
			}
			if len(payload.Actions) != 1 || payload.Actions[0].Status != test.wantStatus || !slices.Equal(payload.Actions[0].SkippedFields, test.wantSkipped) {
				t.Fatalf("actions = %+v, want status %q skippedFields %q", payload.Actions, test.wantStatus, test.wantSkipped)
			}
			if !strings.Contains(payload.Actions[0].Error, test.wantActionError) {
				t.Fatalf("action error = %q, want it to contain %q", payload.Actions[0].Error, test.wantActionError)
			}
			if (test.wantErr && !strings.HasPrefix(stderr, test.wantStderr)) || (!test.wantErr && stderr != test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if test.wantErr && test.wantStderr == "" && strings.Contains(stderr, "Warning: whatsNew") {
				t.Fatalf("stderr = %q, want no whatsNew warning", stderr)
			}
		})
	}
}

// A failed read of the app's versions is not remembered: the next locale
// that hits the whatsNew 409 reads again.
func TestMetadataPushRetriesFailedFirstVersionRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Chdir(t.TempDir())

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.0"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	for _, locale := range []string{"fr-FR", "it"} {
		if err := os.WriteFile(filepath.Join(dir, "version", "1.0", locale+".json"), []byte(`{"description":"Descrizione","whatsNew":"Prima versione."}`), 0o644); err != nil {
			t.Fatalf("write version file: %v", err)
		}
	}
	remote := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Descrizione"}},{"type":"appStoreVersionLocalizations","id":"loc-it","attributes":{"locale":"it","description":"Descrizione"}}],"links":{"next":""}}`

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, remote))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-fr"), asctest.JSON(http.StatusConflict, appleWhatsNewStateError))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-it"), asctest.JSON(http.StatusConflict, appleWhatsNewStateError))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusBadRequest, `{"errors":[{"status":"400","code":"PARAMETER_ERROR.INVALID","title":"A parameter has an invalid value","detail":"bad"}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`))
	// Read-back of the locale that failed.
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, remote))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "metadata", "push", "--app", "app-1", "--version", "1.0", "--dir", dir, "--output", "json")
	if runErr == nil {
		t.Fatalf("expected the locale whose read failed to fail\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	var payload struct {
		Actions []struct {
			Status string `json:"status"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	statuses := []string{}
	for _, action := range payload.Actions {
		statuses = append(statuses, action.Status)
	}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []string{"failed", "skipped"}) {
		t.Fatalf("action statuses = %q, want one failed and one skipped; stdout=%s", statuses, stdout)
	}
}

func TestMetadataPushApplyExecutesUpdates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-app-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"subtitle":"Local subtitle"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ver-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"description":"Local description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Local description"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	updateCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			updateCount++
		}
	}
	if updateCount != 2 {
		t.Fatalf("expected 2 update API calls, got %d", updateCount)
	}

	var payload struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Action string `json:"action"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied=true, got %+v", payload)
	}
	if len(payload.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %+v", payload.Actions)
	}
}

func TestMetadataPushApplyDeletesWhenConfirmed(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-fr","attributes":{"locale":"fr","name":"Remote FR"}}],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/appInfoLocalizations").JSONBody(`{"data":{"type":"appInfoLocalizations","attributes":{"locale":"en-US","name":"App Name"},"relationships":{"appInfo":{"data":{"type":"appInfos","id":"appinfo-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name"}}}`))
	reg.Register(asctest.DELETE("/v1/appInfoLocalizations/loc-app-fr"),
		asctest.JSON(http.StatusNoContent, ""))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--allow-deletes",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	deleteCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodDelete {
			deleteCount++
		}
	}
	if deleteCount != 1 {
		t.Fatalf("expected one delete API call, got %d", deleteCount)
	}

	var payload struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatalf("expected applied=true, got %+v", payload)
	}
}

func TestMetadataPushDryRunPlanIsDeterministic(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-1","attributes":{"locale":"en-US","name":"Remote Name"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-1","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-1","attributes":{"locale":"en-US","name":"Remote Name"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-1","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`))

	runPush := func() string {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"metadata", "push",
				"--app", "app-1",
				"--version", "1.2.3",
				"--dir", dir,
				"--dry-run",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		return stdout
	}

	first := runPush()
	second := runPush()
	if first != second {
		t.Fatalf("expected deterministic JSON plan output,\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestMetadataPushRejectsAmbiguousVersionWithoutPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersions","id":"version-ios","attributes":{"versionString":"1.2.3","platform":"IOS"}},
					{"type":"appStoreVersions","id":"version-mac","attributes":{"versionString":"1.2.3","platform":"MAC_OS"}}
				],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: 2 app store versions match version "1.2.3"; pass --platform with one of:`) {
		t.Fatalf("expected ambiguous-version error, got %q", stderr)
	}
}

func TestMetadataPushRejectsAmbiguousAppInfoWithActionableRemediation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/100000001/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/apps/100000001/appInfos"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
					{"type":"appInfos","id":"appinfo-2","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "100000001",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: 2 app infos match app "100000001"; pass --app-info with one of:`) {
		t.Fatalf("expected ambiguous app-info error, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc apps info list --app "100000001"`) {
		t.Fatalf("expected remediation to mention apps info list, got %q", stderr)
	}
	if !strings.Contains(stderr, `--app-info "appinfo-1"`) {
		t.Fatalf("expected remediation example with --app-info, got %q", stderr)
	}
	if !strings.Contains(stderr, "appinfo-2") {
		t.Fatalf("expected all candidate app info ids in remediation, got %q", stderr)
	}
}

func TestMetadataPushDryRunUsesExplicitAppInfoOverride(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/100000001/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-override", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"appinfo-override","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-override/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "100000001",
			"--app-info", "appinfo-override",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		AppInfoID string `json:"appInfoId"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.AppInfoID != "appinfo-override" {
		t.Fatalf("expected appInfoId appinfo-override, got %+v", payload)
	}
}

func TestMetadataPushDryRunAutoResolvesAppInfoByVersionState(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"appinfo-ready","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
					{"type":"appInfos","id":"appinfo-target","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-target/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		AppInfoID string `json:"appInfoId"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.AppInfoID != "appinfo-target" {
		t.Fatalf("expected auto-resolved appInfoId appinfo-target, got %+v", payload)
	}
}

func TestMetadataPushApplyFailsOnPartialMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Chdir(t.TempDir())

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-app-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"subtitle":"Local subtitle"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ver-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"description":"Local description"}}}`),
		asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Error","detail":"boom"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "push",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected reported partial failure, got %T: %v", runErr, runErr)
	}
	patchCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patchCount++
		}
	}
	if patchCount != 2 {
		t.Fatalf("expected two patch attempts before failure, got %d", patchCount)
	}
	if !strings.Contains(runErr.Error(), "metadata push: 1 localization(s) failed") {
		t.Fatalf("expected batch failure summary, got %v", runErr)
	}
	var result struct {
		Succeeded           int    `json:"succeeded"`
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse partial output: %v\nstdout=%q", err, stdout)
	}
	if result.Succeeded != 1 || result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("unexpected partial summary: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on failure, got %q", stderr)
	}
}

func TestMetadataPushSupportsTableAndMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		outputFlag string
		wantText   string
	}{
		{name: "table", outputFlag: "table", wantText: "Dry Run: true"},
		{name: "markdown", outputFlag: "markdown", wantText: "**Dry Run:** true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
				t.Fatalf("mkdir app-info: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
				t.Fatalf("write app-info file: %v", err)
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
			reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name"}}],"links":{"next":""}}`))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"metadata", "push",
					"--app", "app-1",
					"--version", "1.2.3",
					"--dir", dir,
					"--dry-run",
					"--output", test.outputFlag,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantText) {
				t.Fatalf("expected %q in output, got %q", test.wantText, stdout)
			}
		})
	}
}
