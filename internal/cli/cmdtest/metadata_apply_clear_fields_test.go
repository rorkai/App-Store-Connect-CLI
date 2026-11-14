package cmdtest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func writeMetadataFile(t *testing.T, dir string, parts []string, body string) {
	t.Helper()
	target := filepath.Join(append([]string{dir}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func TestMetadataApplySendsNullForExplicitlyClearedAppInfoFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"name":"Outslept","subtitle":null,"privacyPolicyUrl":null}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker","privacyPolicyUrl":"https://example.com/privacy"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"name":"Outslept","privacyPolicyUrl":null,"subtitle":null}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":null,"privacyPolicyUrl":null}}}`))

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--confirm",
		"--output", "json",
	})

	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
		Updates   []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	patches := metadataMutationCount(reg)
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || patches != 1 {
		t.Fatalf("unexpected apply result: %+v patches=%d", result, patches)
	}
	if len(result.Updates) != 2 {
		t.Fatalf("expected planned clears for both fields, got %+v", result.Updates)
	}
}

func TestMetadataMutationRequiresConfirmForPlannedClear(t *testing.T) {
	for _, operation := range []string{"push", "apply"} {
		t.Run(operation, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			dir := t.TempDir()
			writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"subtitle":null}`)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
			reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker"}}],"links":{"next":""}}`))

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{"metadata", operation, "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir, "--output", "json"}, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected usage exit, got %d", code)
				}
			})
			if stdout != "" || !strings.Contains(stderr, "--confirm is required when applying field clear operations") {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
			}
			mutations := metadataMutationCount(reg)
			if mutations != 0 {
				t.Fatalf("mutations = %d, want zero", mutations)
			}
		})
	}
}

func TestMetadataApplySendsNullForExplicitlyClearedPromotionalText(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"version", "1.2.3", "en-US.json"}, `{"promotionalText":null}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","promotionalText":"Old promo"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"promotionalText":null}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","promotionalText":null}}}`))

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--confirm",
		"--output", "json",
	})

	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	patches := metadataMutationCount(reg)
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || patches != 1 {
		t.Fatalf("unexpected apply result: %+v patches=%d", result, patches)
	}
}

func TestMetadataApplySkipsClearWhenRemoteFieldIsAlreadyEmpty(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"name":"Outslept","subtitle":null}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept"}}],"links":{"next":""}}`))

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})

	var result struct {
		Total   int `json:"total"`
		Updates []struct {
			Key string `json:"key"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	mutations := metadataMutationCount(reg)
	if result.Total != 0 || len(result.Updates) != 0 || mutations != 0 {
		t.Fatalf("expected no-op apply, got %+v mutations=%d", result, mutations)
	}
}

func TestMetadataApplyOmitsNoopClearFromMixedPatchWithoutConfirm(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"app-info", "en-US.json"}, `{"subtitle":null,"privacyPolicyUrl":"https://example.com/new"}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","privacyPolicyUrl":"https://example.com/old"}}],"links":{"next":""}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"privacyPolicyUrl":"https://example.com/new"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","privacyPolicyUrl":"https://example.com/new"}}}`))

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})

	var result struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	patches := metadataMutationCount(reg)
	if !result.Applied || patches != 1 {
		t.Fatalf("unexpected mixed set/no-op clear apply: applied=%t patches=%d", result.Applied, patches)
	}
}

func TestMetadataClearOnlyFileRejectsMissingLocaleBeforeMutation(t *testing.T) {
	tests := []struct {
		name    string
		path    []string
		body    string
		wantErr string
	}{
		{
			name:    "version",
			path:    []string{"version", "1.2.3", "en-US.json"},
			body:    `{"promotionalText":null}`,
			wantErr: `version localization "en-US" cannot be cleared because no existing localization was found`,
		},
		{
			name:    "app-info",
			path:    []string{"app-info", "en-US.json"},
			body:    `{"subtitle":null}`,
			wantErr: `app-info localization "en-US" requires name when creating a new locale`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			dir := t.TempDir()
			writeMetadataFile(t, dir, test.path, test.body)

			reg := asctest.NewRegistry(t)
			reg.Install()
			for range []string{"plan", "apply"} {
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
				if test.name == "version" {
					reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
						asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
					continue
				}
				reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
				reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
			}

			for _, command := range []string{"plan", "apply"} {
				t.Run(command, func(t *testing.T) {
					args := []string{
						"metadata", command,
						"--app", "app-1",
						"--version", "1.2.3",
						"--platform", "IOS",
						"--dir", dir,
						"--output", "json",
					}
					if command == "plan" {
						args = append(args, "--review-dir", t.TempDir())
					}
					stdout, stderr := captureOutput(t, func() {
						if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
							t.Fatalf("exit code = %d, want usage exit code %d", code, cmd.ExitUsage)
						}
					})
					if stdout != "" || !strings.Contains(stderr, test.wantErr) {
						t.Fatalf("expected only the usage error on stderr, got stdout=%q stderr=%q", stdout, stderr)
					}
				})
			}
			mutations := metadataMutationCount(reg)
			if mutations != 0 {
				t.Fatalf("mutation count = %d, want 0", mutations)
			}
		})
	}
}

func TestMetadataPlanAndApplyRejectClearFieldsInDefaultFilesBeforeRequests(t *testing.T) {
	cases := []struct {
		name    string
		path    []string
		body    string
		wantErr string
	}{
		{
			name:    "app-info",
			path:    []string{"app-info", "default.json"},
			body:    `{"name":"Outslept","subtitle":null}`,
			wantErr: "clear fields in app-info/default.json are not supported",
		},
		{
			name:    "version",
			path:    []string{"version", "1.2.3", "default.json"},
			body:    `{"description":"English description","promotionalText":null}`,
			wantErr: "clear fields in version/1.2.3/default.json are not supported",
		},
	}

	for _, tc := range cases {
		for _, command := range []string{"plan", "apply"} {
			t.Run(tc.name+"/"+command, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				t.Setenv("ASC_APP_ID", "")

				dir := t.TempDir()
				writeMetadataFile(t, dir, tc.path, tc.body)
				reg := asctest.NewRegistry(t)
				reg.Install()

				args := []string{
					"metadata", command,
					"--app", "app-1",
					"--version", "1.2.3",
					"--platform", "IOS",
					"--dir", dir,
					"--output", "json",
				}
				if command == "plan" {
					args = append(args, "--review-dir", t.TempDir())
				}
				code := 0
				stdout, stderr := captureOutput(t, func() {
					code = cmd.Run(args, "1.2.3")
				})
				if code != cmd.ExitUsage {
					t.Errorf("exit code = %d, want usage exit code %d; stderr=%q", code, cmd.ExitUsage, stderr)
				}
				if stdout != "" || !strings.Contains(stderr, tc.wantErr) {
					t.Errorf("expected default clear usage error, got stdout=%q stderr=%q", stdout, stderr)
				}
				requests := len(reg.Requests())
				if requests != 0 {
					t.Errorf("default clear validation must happen before network requests, got %d", requests)
				}
			})
		}
	}
}

func TestMetadataApplyMixedSetAndClearCreatesWithoutNullAttribute(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	writeMetadataFile(t, dir, []string{"version", "1.2.3", "fr-FR.json"}, `{"description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version","promotionalText":null}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"fr-FR","description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Nouvelle description","keywords":"sommeil,sante","supportUrl":"https://example.com/support","whatsNew":"Premiere version"}}}`))

	stdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--output", "json",
	})
	var result struct {
		Applied   bool `json:"applied"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, stdout)
	}
	creates := metadataMutationCount(reg)
	if !result.Applied || result.Succeeded != 1 || result.Failed != 0 || creates != 1 {
		t.Fatalf("unexpected mixed create result: %+v creates=%d", result, creates)
	}
}
