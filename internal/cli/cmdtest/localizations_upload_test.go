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
	"testing/synctest"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsUploadDryRunWarnsForPlannedCreate(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ja.strings"), []byte("\"description\" = \"日本語説明\";\n"), 0o644); err != nil {
		t.Fatalf("write strings file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "upload",
		"--version", "version-1",
		"--path", dir,
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "creating locale ja would make it participate in submission validation") {
		t.Fatalf("expected planned create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "keywords, supportUrl") {
		t.Fatalf("expected missing submit fields in warning, got %q", stderr)
	}

	var out struct {
		DryRun  bool `json:"dryRun"`
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if !out.DryRun {
		t.Fatalf("expected dryRun=true, got %+v", out)
	}
	if len(out.Results) != 1 || out.Results[0].Locale != "ja" || out.Results[0].Action != "create" {
		t.Fatalf("expected single create result, got %+v", out.Results)
	}
}

func TestLocalizationsUploadAppliedCreateWarns(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ja.strings"), []byte("\"description\" = \"日本語説明\";\n"), 0o644); err != nil {
		t.Fatalf("write strings file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"ja","description":"日本語説明"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"日本語説明"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "upload",
		"--version", "version-1",
		"--path", dir,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	createCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			createCount++
		}
	}
	if createCount != 1 {
		t.Fatalf("expected one create request, got %d", createCount)
	}
	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected applied create warning on stderr, got %q", stderr)
	}

	var out struct {
		DryRun  bool `json:"dryRun"`
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.DryRun {
		t.Fatalf("expected dryRun=false, got %+v", out)
	}
	if len(out.Results) != 1 || out.Results[0].Locale != "ja" || out.Results[0].Action != "create" {
		t.Fatalf("expected single create result, got %+v", out.Results)
	}
}

func TestLocalizationsUploadUpdateOnlyDoesNotWarn(t *testing.T) {
	setupAuth(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.strings"), []byte("\"description\" = \"Updated description\";\n"), 0o644); err != nil {
		t.Fatalf("write strings file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Old description"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"Updated description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Updated description"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "upload",
		"--version", "version-1",
		"--path", dir,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	updateCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			updateCount++
		}
	}
	if updateCount != 1 {
		t.Fatalf("expected one update request, got %d", updateCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if len(out.Results) != 1 || out.Results[0].Locale != "en-US" || out.Results[0].Action != "update" {
		t.Fatalf("expected single update result, got %+v", out.Results)
	}
}

func TestLocalizationsUploadPartialFailurePrintsSummaryAndArtifact(t *testing.T) {
	result, _, runErr := runPartialLocalizationsUpload(t, false, false, "json")
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 || result.FailureArtifactPath == "" || result.FailureArtifactError != "" {
		t.Fatalf("unexpected partial summary: %+v", result)
	}
	payload, err := os.ReadFile(result.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read failure artifact: %v", err)
	}
	if !strings.Contains(string(payload), `"schemaVersion": 1`) || !strings.Contains(string(payload), `"description": "New French"`) {
		t.Fatalf("failure artifact is not resumable: %s", payload)
	}
}

func TestLocalizationsUploadArtifactWriteFailureStillPrintsSummary(t *testing.T) {
	result, _, runErr := runPartialLocalizationsUpload(t, true, false, "json")
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 || result.FailureArtifactPath != "" || result.FailureArtifactError == "" {
		t.Fatalf("expected printed artifact failure summary: %+v", result)
	}
}

func TestAppSetupLocalizationsUploadPartialFailurePrintsSummaryAndArtifact(t *testing.T) {
	result, _, runErr := runPartialLocalizationsUpload(t, false, true, "json")
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("unexpected app-setup partial summary: %+v", result)
	}
}

func TestAppInfoLocalizationsUploadPartialFailureContinuesAndArtifacts(t *testing.T) {
	result, runErr := runPartialAppInfoLocalizationUpload(t)
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("unexpected app-info partial summary: %+v", result)
	}
	payload, err := os.ReadFile(result.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read app-info artifact: %v", err)
	}
	if !strings.Contains(string(payload), `"subtitle": "French subtitle"`) {
		t.Fatalf("app-info artifact missing desired fields: %s", payload)
	}
}

func TestAppInfoLocalizationsUploadUsesFreshReadbackAfterMutationTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "20ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.strings"), []byte("\"name\" = \"New Name\";\n"), 0o644); err != nil {
		t.Fatalf("write app-info strings: %v", err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	patches := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1":
			return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"appinfo-1","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			reads++
			name := "Old Name"
			if reads > 1 {
				name = "New Name"
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-id","attributes":{"locale":"en-US","name":"`+name+`"}}],"links":{"next":""}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appInfoLocalizations/loc-id":
			patches++
			<-req.Context().Done()
			return nil, req.Context().Err()
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("unexpected test transport request")
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "upload", "--type", "app-info", "--app", "app-1", "--app-info", "appinfo-1", "--path", dir, "--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if reads != 2 || patches != 1 || !strings.Contains(stdout, `"action":"reconcile"`) {
		t.Fatalf("unexpected app-info timeout recovery: reads=%d patches=%d stdout=%s", reads, patches, stdout)
	}
}

func TestLocalizationsUploadTableIncludesFailureArtifact(t *testing.T) {
	_, stdout, runErr := runPartialLocalizationsUpload(t, false, false, "table")
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	for _, expected := range []string{"Total", "Succeeded", "Failed", "Failure Artifact", ".asc/reports/localizations-upload/"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("table output missing %q: %s", expected, stdout)
		}
	}
}

func TestLocalizationsUploadMarkdownIncludesArtifactWriteFailure(t *testing.T) {
	_, stdout, runErr := runPartialLocalizationsUpload(t, true, false, "markdown")
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if !strings.Contains(stdout, "Failure Artifact Error") || !strings.Contains(stdout, "not a directory") {
		t.Fatalf("markdown output missing artifact write failure: %s", stdout)
	}
}

func TestLocalizationsUploadPartialBatchExitsOne(t *testing.T) {
	result, stdout, code := runPartialLocalizationsUploadExit(t)
	if code != cmd.ExitError {
		t.Fatalf("expected partial batch exit %d, got %d; stdout=%s", cmd.ExitError, code, stdout)
	}
	if result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("expected structured partial summary before exit: %+v", result)
	}
}

func TestLocalizationsUploadInitialReadFailureIsNotZeroFailureSummary(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.strings"), []byte("\"description\" = \"Desired\";\n\"whatsNew\" = \"Notes\";\n"), 0o644); err != nil {
		t.Fatalf("write strings: %v", err)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","detail":"conflicting state"}]}`))

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = cmd.Run([]string{"localizations", "upload", "--version", "version-1", "--path", dir, "--output", "json"}, "1.2.3")
	})
	if exitCode != cmd.ExitConflict {
		t.Fatalf("expected typed pre-batch exit %d, got %d; stderr=%s", cmd.ExitConflict, exitCode, stderr)
	}
	if stdout != "" || strings.Contains(stderr, "0 locale(s) failed") {
		t.Fatalf("unexpected zero-failure summary: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestLocalizationUploadCommandsRejectLateEmptyLocaleBeforeAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.strings"), []byte("\"description\" = \"Valid\";\n"), 0o644); err != nil {
		t.Fatalf("write valid locale: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fr-FR.strings"), []byte("// intentionally empty\n"), 0o644); err != nil {
		t.Fatalf("write empty locale: %v", err)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()

	for _, args := range [][]string{
		{"localizations", "upload", "--version", "version-1", "--path", dir},
		{"app-setup", "localizations", "upload", "--version", "version-1", "--path", dir},
	} {
		stdout, stderr := captureOutput(t, func() {
			if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
				t.Fatalf("expected exit %d for %v, got %d", cmd.ExitUsage, args, code)
			}
		})
		if stdout != "" || !strings.Contains(stderr, `no localization values for locale "fr-FR"`) {
			t.Fatalf("unexpected validation output for %v: stdout=%q stderr=%q", args, stdout, stderr)
		}
	}
	if requests := len(reg.Requests()); requests != 0 {
		t.Fatalf("expected zero network requests, got %d", requests)
	}
}

func TestLocalizationsUploadInvalidLocaleFlagExitsUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	path := filepath.Join(t.TempDir(), "input.strings")
	if err := os.WriteFile(path, []byte("\"description\" = \"Valid\";\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"localizations", "upload", "--version", "version-1", "--path", path, "--locale", "x"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit %d, got %d", cmd.ExitUsage, code)
		}
	})
	if stdout != "" || !strings.Contains(stderr, `invalid locale "x"`) {
		t.Fatalf("unexpected invalid-locale output: stdout=%q stderr=%q", stdout, stderr)
	}
}

type partialLocalizationUploadResult struct {
	Total                int    `json:"total"`
	Succeeded            int    `json:"succeeded"`
	Failed               int    `json:"failed"`
	FailureArtifactPath  string `json:"failureArtifactPath"`
	FailureArtifactError string `json:"failureArtifactError"`
}

func runPartialLocalizationsUpload(t *testing.T, blockArtifact, appSetup bool, output string) (partialLocalizationUploadResult, string, error) {
	result, stdout, _, runErr := runPartialLocalizationsUploadCore(t, blockArtifact, appSetup, output, false)
	return result, stdout, runErr
}

func runPartialLocalizationsUploadExit(t *testing.T) (partialLocalizationUploadResult, string, int) {
	result, stdout, code, _ := runPartialLocalizationsUploadCore(t, false, false, "json", true)
	return result, stdout, code
}

func runPartialLocalizationsUploadCore(t *testing.T, blockArtifact, appSetup bool, output string, viaEntryPoint bool) (partialLocalizationUploadResult, string, int, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	workDir := t.TempDir()
	t.Chdir(workDir)
	inputDir := filepath.Join(workDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "en-US.strings"), []byte("\"description\" = \"New English\";\n\"whatsNew\" = \"English notes\";\n"), 0o644); err != nil {
		t.Fatalf("write English: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "fr-FR.strings"), []byte("\"description\" = \"New French\";\n\"whatsNew\" = \"French notes\";\n"), 0o644); err != nil {
		t.Fatalf("write French: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "ja.strings"), []byte("\"description\" = \"New Japanese\";\n\"whatsNew\" = \"Japanese notes\";\n"), 0o644); err != nil {
		t.Fatalf("write Japanese: %v", err)
	}
	if blockArtifact {
		if err := os.WriteFile(".asc", []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("write artifact blocker: %v", err)
		}
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"Old English"}},{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","description":"Old French"}},{"type":"appStoreVersionLocalizations","id":"ja-id","attributes":{"locale":"ja","description":"Old Japanese"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/en-id").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"description":"New English","whatsNew":"English notes"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"New English","whatsNew":"English notes"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/fr-id").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"description":"New French","whatsNew":"French notes"}}}`),
		asctest.JSON(http.StatusBadRequest, `{"errors":[{"status":"400","code":"ENTITY_ERROR","detail":"French rejected"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"en-id","attributes":{"locale":"en-US","description":"Old English"}},{"type":"appStoreVersionLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","description":"Old French"}},{"type":"appStoreVersionLocalizations","id":"ja-id","attributes":{"locale":"ja","description":"Old Japanese"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/ja-id").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"ja-id","attributes":{"description":"New Japanese","whatsNew":"Japanese notes"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"ja-id","attributes":{"locale":"ja","description":"New Japanese","whatsNew":"Japanese notes"}}}`))

	var runErr error
	exitCode := cmd.ExitSuccess
	stdout, _ := captureOutput(t, func() {
		args := []string{"localizations", "upload", "--version", "version-1", "--path", inputDir, "--output", output}
		if appSetup {
			args = append([]string{"app-setup"}, args...)
		}
		if viaEntryPoint {
			exitCode = cmd.Run(args, "1.2.3")
			return
		}
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	patches := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patches++
		}
	}
	if patches != 3 {
		t.Fatalf("expected later locale after failure, patches=%d", patches)
	}
	var result partialLocalizationUploadResult
	if output == "json" {
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("parse summary: %v; stdout=%q", err, stdout)
		}
	}
	return result, stdout, exitCode, runErr
}

func runPartialAppInfoLocalizationUpload(t *testing.T) (partialLocalizationUploadResult, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	workDir := t.TempDir()
	t.Chdir(workDir)
	inputDir := filepath.Join(workDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir input: %v", err)
	}
	files := map[string]string{
		"en-US.strings": "\"name\" = \"English\";\n\"subtitle\" = \"English subtitle\";\n",
		"fr-FR.strings": "\"name\" = \"French\";\n\"subtitle\" = \"French subtitle\";\n",
		"ja.strings":    "\"name\" = \"Japanese\";\n\"subtitle\" = \"Japanese subtitle\";\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(inputDir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"appinfo-1","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"en-id","attributes":{"locale":"en-US","name":"Old English"}},{"type":"appInfoLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","name":"Old French"}},{"type":"appInfoLocalizations","id":"ja-id","attributes":{"locale":"ja","name":"Old Japanese"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/en-id").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"en-id","attributes":{"name":"English","subtitle":"English subtitle"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"en-id","attributes":{"locale":"en-US","name":"English","subtitle":"English subtitle"}}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/fr-id").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"fr-id","attributes":{"name":"French","subtitle":"French subtitle"}}}`),
		asctest.JSON(http.StatusBadRequest, `{"errors":[{"status":"400","code":"ENTITY_ERROR","detail":"French rejected"}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"en-id","attributes":{"locale":"en-US","name":"Old English"}},{"type":"appInfoLocalizations","id":"fr-id","attributes":{"locale":"fr-FR","name":"Old French"}},{"type":"appInfoLocalizations","id":"ja-id","attributes":{"locale":"ja","name":"Old Japanese"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/ja-id").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"ja-id","attributes":{"name":"Japanese","subtitle":"Japanese subtitle"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"ja-id","attributes":{"locale":"ja","name":"Japanese","subtitle":"Japanese subtitle"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "upload", "--type", "app-info", "--app", "app-1", "--app-info", "appinfo-1", "--path", inputDir, "--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	patches := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patches++
		}
	}
	if patches != 3 {
		t.Fatalf("expected app-info locale after failure, patches=%d", patches)
	}
	var result partialLocalizationUploadResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse app-info summary: %v; stdout=%q", err, stdout)
	}
	return result, runErr
}
