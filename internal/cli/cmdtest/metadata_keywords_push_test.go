package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestRunMetadataKeywordsPushInvalidContinueOnErrorReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":"alpha,beta"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "push",
			"--version-id", "ver-1",
			"--input", inputPath,
			"--continue-on-error", "maybe",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--continue-on-error must be true or false") {
		t.Fatalf("expected invalid boolean stderr, got %q", stderr)
	}
}

func TestMetadataKeywordsPushCreatesUpdatesAndWritesFailureArtifact(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	inputPath := filepath.Join(workDir, "keywords.json")
	input := `{
		"en-US": "alpha,beta",
		"de-DE": "eins,zwei",
		"ja": "nihon,go"
	}`
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"appStoreVersionLocalizations",
					"id":"loc-en",
					"attributes":{"locale":"en-US","keywords":"old,keywords"}
				}
			],
			"links":{}
		}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","keywords":"eins,zwei"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{
			"data":{
				"type":"appStoreVersionLocalizations",
				"id":"loc-de",
				"attributes":{"locale":"de-DE","keywords":"eins,zwei"}
			}
		}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"keywords":"alpha,beta"}}}`),
		asctest.JSON(http.StatusOK, `{
			"data":{
				"type":"appStoreVersionLocalizations",
				"id":"loc-en",
				"attributes":{"locale":"en-US","keywords":"alpha,beta"}
			}
		}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"ja","keywords":"nihon,go"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{
			"errors":[
				{
					"status":"422",
					"code":"ENTITY_ERROR.ATTRIBUTE.INVALID",
					"title":"Invalid",
					"detail":"ja keywords failed validation"
				}
			]
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "push",
			"--version-id", "ver-1",
			"--input", inputPath,
			"--continue-on-error=true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	var patchBody string
	createBodies := make([]string, 0, 2)
	for _, req := range reg.Requests() {
		switch req.Method {
		case http.MethodPatch:
			patchBody = string(req.Body)
		case http.MethodPost:
			createBodies = append(createBodies, string(req.Body))
		}
	}
	if !strings.Contains(patchBody, `"keywords":"alpha,beta"`) {
		t.Fatalf("expected PATCH body to include keywords, got %s", patchBody)
	}
	if strings.Contains(patchBody, `"locale":"en-US"`) {
		t.Fatalf("expected PATCH body to omit locale, got %s", patchBody)
	}
	if len(createBodies) != 2 {
		t.Fatalf("expected 2 create attempts, got %d", len(createBodies))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if intValue(payload["total"]) != 3 {
		t.Fatalf("expected total=3, got %v", payload["total"])
	}
	if intValue(payload["succeeded"]) != 2 {
		t.Fatalf("expected succeeded=2, got %v", payload["succeeded"])
	}
	if intValue(payload["failed"]) != 1 {
		t.Fatalf("expected failed=1, got %v", payload["failed"])
	}

	results, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", payload["results"])
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	byLocale := map[string]map[string]any{}
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected object result item, got %T", item)
		}
		byLocale[asString(entry["locale"])] = entry
	}

	if byLocale["en-US"]["action"] != "update" || byLocale["en-US"]["status"] != "succeeded" {
		t.Fatalf("expected en-US update success, got %+v", byLocale["en-US"])
	}
	if byLocale["de-DE"]["action"] != "create" || byLocale["de-DE"]["status"] != "succeeded" {
		t.Fatalf("expected de-DE create success, got %+v", byLocale["de-DE"])
	}
	if byLocale["ja"]["action"] != "create" || byLocale["ja"]["status"] != "failed" {
		t.Fatalf("expected ja create failure, got %+v", byLocale["ja"])
	}
	if !strings.Contains(asString(byLocale["ja"]["error"]), "ja keywords failed validation") {
		t.Fatalf("expected ja error details, got %+v", byLocale["ja"])
	}

	artifactPath := asString(payload["failureArtifactPath"])
	if artifactPath == "" {
		t.Fatalf("expected failureArtifactPath, got %+v", payload)
	}
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", artifactPath, err)
	}
	if !strings.Contains(string(artifactData), `"locale": "ja"`) && !strings.Contains(string(artifactData), `"locale":"ja"`) {
		t.Fatalf("expected failure artifact to include failed locale, got %s", string(artifactData))
	}
}

func TestRunMetadataKeywordsPushPartialFailureReturnsExitError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	inputPath := filepath.Join(workDir, "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":"alpha,beta","de-DE":"eins,zwei"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","keywords":"eins,zwei"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid","detail":"de-DE failed"}]}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","keywords":"alpha,beta"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "push",
			"--version-id", "ver-1",
			"--input", inputPath,
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if want := "Error: metadata keywords push: 1 locale(s) failed\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if intValue(payload["failed"]) != 1 {
		t.Fatalf("expected failed=1 in output, got %v", payload["failed"])
	}
}

func TestRunMetadataKeywordsPushTableOutputIncludesSummaryAndFailureArtifact(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	inputPath := filepath.Join(workDir, "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":"alpha,beta","de-DE":"eins,zwei"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","keywords":"eins,zwei"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid","detail":"de-DE failed"}]}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","keywords":"alpha,beta"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"metadata", "keywords", "push",
			"--version-id", "ver-1",
			"--input", inputPath,
			"--output", "table",
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if want := "Error: metadata keywords push: 1 locale(s) failed\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	for _, want := range []string{
		"Version ID",
		"Failure Artifact",
		".asc/reports/metadata-keywords-push/failures-",
		"Locale",
		"de-DE",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestMetadataKeywordsPushStopsOnFirstFailureWhenContinueOnErrorFalse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	inputPath := filepath.Join(workDir, "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"de-DE":"eins,zwei","ja":"nihon,go"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","keywords":"eins,zwei"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid","detail":"de-DE failed"}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "push",
			"--version-id", "ver-1",
			"--input", inputPath,
			"--continue-on-error=false",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	postCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			postCount++
		}
	}
	if postCount != 1 {
		t.Fatalf("expected one create attempt before stopping, got %d", postCount)
	}
	artifacts, err := filepath.Glob(filepath.Join(workDir, ".asc", "reports", "metadata-keywords-push", "failures-*.json"))
	if err != nil {
		t.Fatalf("Glob() error: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one failure artifact in temp work dir, got %d (%v)", len(artifacts), artifacts)
	}
}
