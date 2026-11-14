package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestVersionsCreateCopyMetadataAppliesSelectedFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreVersions").JSONBody(`{"data":{"type":"appStoreVersions","attributes":{"platform":"IOS","versionString":"2.4.0"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"ver-new","attributes":{"platform":"IOS","versionString":"2.4.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=2.3.2", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-source","attributes":{"platform":"IOS","versionString":"2.3.2","appVersionState":"READY_FOR_SALE"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-source/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"src-en","attributes":{"locale":"en-US","description":"English description","keywords":"en,keywords","whatsNew":"ignored"}},{"type":"appStoreVersionLocalizations","id":"src-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"fr,motscles","whatsNew":"ignored"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-new/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"dst-fr","attributes":{"locale":"fr-FR"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/dst-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"description":"English description","keywords":"en,keywords"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"locale":"en-US","description":"English description","keywords":"en,keywords"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/dst-fr").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"dst-fr","attributes":{"description":"Description FR","keywords":"fr,motscles"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"dst-fr","attributes":{"locale":"fr-FR","description":"Description FR","keywords":"fr,motscles"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"versions", "create",
		"--app", "app-1",
		"--version", "2.4.0",
		"--platform", "IOS",
		"--copy-metadata-from", "2.3.2",
		"--copy-fields", "description,keywords,whatsNew",
		"--exclude-fields", "whatsNew",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"ver-new"`) {
		t.Fatalf("expected created version id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"metadataCopy"`) {
		t.Fatalf("expected metadata copy summary in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"copiedLocales":2`) {
		t.Fatalf("expected copied locale count in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"copiedFieldUpdates":4`) {
		t.Fatalf("expected copied field count in output, got %q", stdout)
	}
}

func TestVersionsCreateCopyMetadataSkipsSourceLocalesMissingOnDestination(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreVersions").JSONBody(`{"data":{"type":"appStoreVersions","attributes":{"platform":"IOS","versionString":"2.4.0"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"ver-new","attributes":{"platform":"IOS","versionString":"2.4.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=2.3.2", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-source","attributes":{"platform":"IOS","versionString":"2.3.2","appVersionState":"READY_FOR_SALE"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-source/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"src-en","attributes":{"locale":"en-US","description":"English description"}},{"type":"appStoreVersionLocalizations","id":"src-fr","attributes":{"locale":"fr-FR","description":"Description FR"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-new/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/dst-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"description":"English description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"dst-en","attributes":{"locale":"en-US","description":"English description"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"versions", "create",
		"--app", "app-1",
		"--version", "2.4.0",
		"--platform", "IOS",
		"--copy-metadata-from", "2.3.2",
		"--copy-fields", "description",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Warning: skipped source locales not enabled on destination: fr-FR") {
		t.Fatalf("expected skip warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"copiedLocales":1`) {
		t.Fatalf("expected copied locale count in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"skippedLocales":["fr-FR"]`) {
		t.Fatalf("expected skipped locale summary in output, got %q", stdout)
	}
}

func TestVersionsCreateCopyMetadataRejectsInvalidCopyField(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "create",
			"--app", "app-1",
			"--version", "2.4.0",
			"--copy-metadata-from", "2.3.2",
			"--copy-fields", "description,invalidField",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--copy-fields must be one of:") {
		t.Fatalf("expected copy-fields usage error, got %q", stderr)
	}
}

func TestVersionsCreateCopyMetadataRequiresCopySource(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "create",
			"--app", "app-1",
			"--version", "2.4.0",
			"--copy-fields", "description",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --copy-metadata-from is required when using --copy-fields or --exclude-fields") {
		t.Fatalf("expected copy source usage error, got %q", stderr)
	}
}

func TestVersionsCreateCopyMetadataRejectsInvalidExcludeField(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "create",
			"--app", "app-1",
			"--version", "2.4.0",
			"--copy-metadata-from", "2.3.2",
			"--exclude-fields", "description,invalidField",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--exclude-fields must be one of:") {
		t.Fatalf("expected exclude-fields usage error, got %q", stderr)
	}
}

func TestVersionsCreateRejectsRepeatedCopyFieldFlags(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "copy-fields",
			args: []string{
				"versions", "create",
				"--app", "app-1",
				"--version", "2.4.0",
				"--copy-metadata-from", "2.3.2",
				"--copy-fields", "description",
				"--copy-fields", "keywords",
			},
			wantErr: `--copy-fields specified multiple times; pass one comma-separated list, for example --copy-fields "description,keywords"`,
		},
		{
			name: "exclude-fields",
			args: []string{
				"versions", "create",
				"--app", "app-1",
				"--version", "2.4.0",
				"--copy-metadata-from", "2.3.2",
				"--exclude-fields", "whatsNew",
				"--exclude-fields", "keywords",
			},
			wantErr: `--exclude-fields specified multiple times; pass one comma-separated list, for example --exclude-fields "whatsNew,keywords"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantErr)
			}
			if strings.Contains(stderr, "SUBCOMMANDS") {
				t.Fatalf("expected a usage error instead of a help dump, got %q", stderr)
			}
		})
	}
}
