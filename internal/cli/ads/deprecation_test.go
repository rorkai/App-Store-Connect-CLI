package ads

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAdsLegacyMigrationLedgerMatchesAllV5EndpointSpecs(t *testing.T) {
	specs := appleads.EndpointSpecs()
	if got, want := len(specs), 73; got != want {
		t.Fatalf("EndpointSpecs() count = %d, want %d", got, want)
	}

	counts := map[adsLegacyMigrationKind]int{}
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		migration, ok := adsLegacyMigrationForSpec(spec)
		if !ok {
			t.Fatalf("missing migration entry for %q", spec.Name)
		}
		if seen[spec.Name] {
			t.Fatalf("duplicate migration entry for %q", spec.Name)
		}
		seen[spec.Name] = true
		counts[migration.kind]++
		switch migration.kind {
		case adsLegacyNone:
			if len(migration.replacement) != 0 || strings.TrimSpace(migration.guidance) == "" {
				t.Fatalf("%q no-replacement entry = %+v, want guidance without replacement", spec.Name, migration)
			}
		default:
			if len(migration.replacement) == 0 {
				t.Fatalf("%q migration has no replacement path", spec.Name)
			}
			if !adsLegacyReplacementRegistered(migration.replacement) {
				t.Fatalf("%q replacement path %q is not registered", spec.Name, strings.Join(migration.replacement, " "))
			}
		}
	}
	for name := range adsLegacyMigrations {
		if !seen[name] {
			t.Fatalf("migration ledger has extra entry %q", name)
		}
	}
	if got, want := counts[adsLegacyDirect], 29; got != want {
		t.Fatalf("direct migration count = %d, want %d", got, want)
	}
	if got, want := counts[adsLegacyBreaking], 37; got != want {
		t.Fatalf("breaking migration count = %d, want %d", got, want)
	}
	if got, want := counts[adsLegacyNone], 7; got != want {
		t.Fatalf("no-replacement migration count = %d, want %d", got, want)
	}
}

func TestAdsLegacyProductPageRejectionMigrationsUseAppEndpoints(t *testing.T) {
	tests := map[string]string{
		"find-ad-creative-rejection-reasons": "rejection-reasons apps find",
		"gets-a-product-page-reason":         "rejection-reasons apps view",
	}
	for name, want := range tests {
		migration, ok := adsLegacyMigrations[name]
		if !ok {
			t.Fatalf("missing migration for %q", name)
		}
		if migration.kind != adsLegacyBreaking {
			t.Errorf("%q migration kind = %q, want %q", name, migration.kind, adsLegacyBreaking)
		}
		if got := strings.Join(migration.replacement, " "); got != want {
			t.Errorf("%q replacement = %q, want %q", name, got, want)
		}
	}
}

func TestAdsLegacyCommandWarningIsEmittedOnceBeforeExistingExecution(t *testing.T) {
	// Isolate host credentials so the alias deterministically fails at
	// credential resolution instead of reaching the live Apple Ads API.
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	setAdsResolverTestEnv(t)

	command := findCommand(AdsCommand(), "v5", "campaigns")
	if command == nil || command.Exec == nil {
		t.Fatal("missing campaigns legacy alias")
	}
	stdout, stderr := captureAdsDeprecationStreams(t, func() {
		err := command.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("campaigns alias unexpectedly succeeded without credentials")
		}
	})
	if stdout != "" {
		t.Fatalf("campaigns alias stdout = %q, want empty output", stdout)
	}
	want := "Warning: `asc ads v5 campaigns` is deprecated and retires on January 26, 2027. Use `asc ads campaigns find`.\n"
	if stderr != want {
		t.Fatalf("campaigns alias warning = %q, want %q", stderr, want)
	}
	if !strings.Contains(command.ShortHelp, adsLegacyRetirementNotice) || !strings.Contains(command.LongHelp, adsLegacyRetirementNotice) {
		t.Fatalf("campaigns alias help missing retirement notice: short=%q long=%q", command.ShortHelp, command.LongHelp)
	}
}

func TestAdsLegacyWarningsDoNotWrapPlatformOrAuthGroups(t *testing.T) {
	root := AdsCommand()
	for _, path := range [][]string{{"auth", "discover"}, {"campaigns", "find"}, {"api", "request"}} {
		command := findCommand(root, path...)
		if command == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(path, " "))
		}
		if strings.HasPrefix(command.ShortHelp, "DEPRECATED:") || strings.Contains(command.LongHelp, "Campaign Management API v5 is deprecated") {
			t.Fatalf("asc ads %s unexpectedly marked deprecated: short=%q long=%q", strings.Join(path, " "), command.ShortHelp, command.LongHelp)
		}
	}
}

func TestAdsLegacyCommandInventoryIncludesEndpointLeavesAliasesAndWorkflows(t *testing.T) {
	count := 0
	var visit func(*ffcli.Command)
	visit = func(command *ffcli.Command) {
		if strings.HasPrefix(command.ShortHelp, "DEPRECATED:") {
			count++
		}
		for _, child := range command.Subcommands {
			visit(child)
		}
	}
	visit(AdsCommand())
	if got, want := count, 90; got != want {
		t.Fatalf("deprecated Apple Ads command count = %d, want %d (73 endpoints + 12 aliases + 4 workflows + v5 group)", got, want)
	}
}

func captureAdsDeprecationStreams(t *testing.T, run func()) (string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	run()
	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = oldStdout, oldStderr
	t.Cleanup(func() {
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
	})
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr)
}
