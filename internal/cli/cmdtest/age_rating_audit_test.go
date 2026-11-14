package cmdtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const ageRatingAuditApps = `{"data":[
	{"type":"apps","id":"app-ready","attributes":{"name":"Ready App","bundleId":"com.example.ready"}},
	{"type":"apps","id":"app-social","attributes":{"name":"Social App","bundleId":"com.example.social"}},
	{"type":"apps","id":"app-unset","attributes":{"name":"Unset App","bundleId":"com.example.unset"}}
],"links":{}}`

var ageRatingAuditDeclarations = map[string]string{
	"app-ready":  `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`,
	"app-social": `{"data":{"type":"ageRatingDeclarations","id":"decl-2","attributes":{"socialMedia":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":true}}}`,
	"app-unset":  `{"data":{"type":"ageRatingDeclarations","id":"decl-3","attributes":{}}}`,
}

func registerAgeRatingAuditApps(reg *asctest.Registry, appIDs ...string) {
	for _, appID := range appIDs {
		reg.Register(asctest.GET("/v1/apps/"+appID+"/appInfos", "fields[appInfos]=state", "limit=200"),
			asctest.JSON(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appInfos","id":"info-%s"}],"links":{}}`, appID)))
		reg.Register(asctest.GET("/v1/appInfos/info-"+appID+"/ageRatingDeclaration"),
			asctest.JSON(http.StatusOK, ageRatingAuditDeclarations[appID]))
	}
}

func TestAgeRatingAuditReportsMissingSocialMediaResponses(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"), asctest.JSON(http.StatusOK, ageRatingAuditApps))
	registerAgeRatingAuditApps(reg, "app-ready", "app-social", "app-unset")

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--output", "json"}...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		Apps []struct {
			AppID                string   `json:"appId"`
			SocialMedia          string   `json:"socialMedia"`
			UserGeneratedContent string   `json:"userGeneratedContent"`
			MissingResponses     []string `json:"missingResponses"`
			Ready                bool     `json:"ready"`
		} `json:"apps"`
		ReadyCount   int `json:"readyCount"`
		MissingCount int `json:"missingCount"`
		ErrorCount   int `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.ReadyCount != 1 || result.MissingCount != 2 || result.ErrorCount != 0 {
		t.Fatalf("counts = ready %d missing %d error %d, want 1/2/0", result.ReadyCount, result.MissingCount, result.ErrorCount)
	}
	rows := map[string][]string{}
	userGeneratedContent := map[string]string{}
	for _, row := range result.Apps {
		rows[row.AppID] = row.MissingResponses
		userGeneratedContent[row.AppID] = row.UserGeneratedContent
	}
	if got := userGeneratedContent["app-social"]; got != "true" {
		t.Fatalf("app-social user-generated content = %q, want true", got)
	}
	if got := userGeneratedContent["app-unset"]; got != "UNSET" {
		t.Fatalf("app-unset user-generated content = %q, want UNSET", got)
	}
	if len(rows["app-ready"]) != 0 {
		t.Fatalf("app-ready missing = %v, want none", rows["app-ready"])
	}
	if got := strings.Join(rows["app-social"], ","); got != "socialMediaAgeRestricted" {
		t.Fatalf("app-social missing = %q, want socialMediaAgeRestricted", got)
	}
	if got := strings.Join(rows["app-unset"], ","); got != "socialMedia,messagingAndChat" {
		t.Fatalf("app-unset missing = %q, want socialMedia,messagingAndChat", got)
	}
	if !strings.Contains(stderr, "September 2026") {
		t.Fatalf("stderr missing deadline notice, got %q", stderr)
	}
}

func TestAgeRatingAuditAppFilterRestrictsSweep(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"), asctest.JSON(http.StatusOK, ageRatingAuditApps))
	registerAgeRatingAuditApps(reg, "app-social")

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--app", "app-social", "--output", "json"}...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		Apps []struct {
			AppID string `json:"appId"`
			Name  string `json:"name"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || result.Apps[0].AppID != "app-social" || result.Apps[0].Name != "Social App" {
		t.Fatalf("unexpected filtered rows: %+v", result.Apps)
	}
}

func TestAgeRatingAuditSelectsCurrentAppInfo(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Current App"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos", "fields[appInfos]=state", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}},{"type":"appInfos","id":"info-current","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-current/ageRatingDeclaration"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-current","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--output", "json"}...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		Apps []struct {
			Ready bool `json:"ready"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || !result.Apps[0].Ready {
		t.Fatalf("unexpected audit result: %+v", result.Apps)
	}
}

func TestAgeRatingAuditChecksEveryCurrentAppInfo(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Multi-platform App"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos", "fields[appInfos]=state", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-ios","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-mac","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-ios/ageRatingDeclaration"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-ios","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-mac/ageRatingDeclaration"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-mac","attributes":{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":true}}}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--output", "json"}...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		Apps []struct {
			AppInfoID    string `json:"appInfoId"`
			AppInfoState string `json:"appInfoState"`
			Ready        bool   `json:"ready"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 2 {
		t.Fatalf("audit rows = %+v, want one row per current app info", result.Apps)
	}
	want := []struct {
		id    string
		state string
	}{{"info-ios", "READY_FOR_DISTRIBUTION"}, {"info-mac", "PREPARE_FOR_SUBMISSION"}}
	for i, row := range result.Apps {
		if row.AppInfoID != want[i].id || row.AppInfoState != want[i].state || !row.Ready {
			t.Fatalf("audit row %d = %+v, want app info %q in state %q and ready", i, row, want[i].id, want[i].state)
		}
	}
}

func TestAgeRatingAuditReportsMissingReadinessResponses(t *testing.T) {
	tests := []struct {
		name        string
		attributes  string
		wantMissing string
	}{
		{
			name:        "requires age assurance",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":true}`,
			wantMissing: "ageAssurance",
		},
		{
			name:        "rejects restriction without social media",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":true}`,
			wantMissing: "socialMedia",
		},
		{
			name:        "social media requires user-generated content",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false,"userGeneratedContent":false}`,
			wantMissing: "userGeneratedContent",
		},
		{
			name:        "age-restricted social media requires user-generated content",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true,"userGeneratedContent":false}`,
			wantMissing: "socialMedia,userGeneratedContent",
		},
		{
			name:        "social media with user-generated content unset",
			attributes:  `{"socialMedia":true,"socialMediaAgeRestricted":false,"messagingAndChat":true,"ageAssurance":false}`,
			wantMissing: "userGeneratedContent",
		},
		{
			name:        "age-restricted social media with user-generated content unset",
			attributes:  `{"socialMedia":false,"socialMediaAgeRestricted":true,"messagingAndChat":true,"ageAssurance":true}`,
			wantMissing: "socialMedia,userGeneratedContent",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appInfos", "fields[appInfos]=state", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`))
			reg.Register(asctest.GET("/v1/appInfos/info-1/ageRatingDeclaration"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":`+test.attributes+`}}`))

			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--output", "json"}...)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var result struct {
				Apps []struct {
					MissingResponses []string `json:"missingResponses"`
					Ready            bool     `json:"ready"`
				} `json:"apps"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal output: %v (%q)", err, stdout)
			}
			if len(result.Apps) != 1 || result.Apps[0].Ready || strings.Join(result.Apps[0].MissingResponses, ",") != test.wantMissing {
				t.Fatalf("unexpected readiness result: %+v", result.Apps)
			}
		})
	}
}

func TestAgeRatingAuditReturnsFailureAfterRenderingRowErrors(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos", "fields[appInfos]=state", "limit=200"), asctest.JSON(http.StatusForbidden, `{"errors":[{"status":"403","title":"Forbidden"}]}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "age-rating", "audit", "--output", "json")
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	var result struct {
		Apps []struct {
			Error string `json:"error"`
		} `json:"apps"`
		ErrorCount int `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Apps) != 1 || result.Apps[0].Error == "" || result.ErrorCount != 1 {
		t.Fatalf("unexpected error result: %+v", result)
	}
	if !strings.Contains(stderr, "age-rating audit: 1 app info record could not be audited") {
		t.Fatalf("stderr = %q, want partial-audit diagnostic", stderr)
	}
}

func TestAgeRatingAuditRejectsExplicitEmptyApp(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	stdout, stderr, code := runWithDeps(t, stubClient(t, asctest.NewRegistry(t)), "age-rating", "audit", "--app", " , \t")
	if code != cmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--app must include at least one app ID") {
		t.Fatalf("stderr = %q, want explicit empty --app error", stderr)
	}
}

func TestAgeRatingAuditPaginationIsExplicit(t *testing.T) {
	for _, tt := range []struct {
		name           string
		args           []string
		wantApps       int
		wantSecondPage bool
	}{
		{name: "first page by default", args: []string{"age-rating", "audit", "--output", "json"}, wantApps: 1},
		{name: "all pages when requested", args: []string{"age-rating", "audit", "--paginate", "--output", "json"}, wantApps: 2, wantSecondPage: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			ready := `{"data":{"type":"ageRatingDeclarations","id":"decl-1","attributes":{"socialMedia":false,"socialMediaAgeRestricted":false,"messagingAndChat":false,"ageAssurance":false}}}`
			appIDs := []string{"app-1"}
			reg.Register(asctest.GET("/v1/apps", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=next"}}`))
			if tt.wantSecondPage {
				reg.Register(asctest.GET("/v1/apps", "cursor=next"), asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-2"}],"links":{}}`))
				appIDs = append(appIDs, "app-2")
			}
			for _, appID := range appIDs {
				reg.Register(asctest.GET("/v1/apps/"+appID+"/appInfos", "fields[appInfos]=state", "limit=200"),
					asctest.JSON(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appInfos","id":"info-%s","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`, appID)))
				reg.Register(asctest.GET("/v1/appInfos/info-"+appID+"/ageRatingDeclaration"), asctest.JSON(http.StatusOK, ready))
			}

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), tt.args...)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			secondPage := false
			for _, req := range reg.Requests() {
				if req.URL.Path == "/v1/apps" && req.URL.Query().Get("cursor") == "next" {
					secondPage = true
				}
			}
			var result struct {
				Apps []json.RawMessage `json:"apps"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal output: %v (%q)", err, stdout)
			}
			if len(result.Apps) != tt.wantApps {
				t.Fatalf("app count = %d, want %d", len(result.Apps), tt.wantApps)
			}
			if secondPage != tt.wantSecondPage {
				t.Fatalf("second page requested = %t, want %t", secondPage, tt.wantSecondPage)
			}
			if !tt.wantSecondPage && !strings.Contains(stderr, "use --paginate to audit every app") {
				t.Fatalf("stderr = %q, want pagination warning", stderr)
			}
		})
	}
}

func TestAgeRatingAuditRejectsRepeatedAppPaginationURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=repeat"}}`))
	reg.Register(asctest.GET("/v1/apps", "cursor=repeat"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=repeat"}}`))

	_, _, err := runRootWithDeps(t, stubClient(t, reg), []string{"age-rating", "audit", "--paginate", "--output", "json"}...)
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("run error = %v, want ErrRepeatedPaginationURL", err)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("app requests = %d, want 2 before rejecting repeated link", requests)
	}
}
