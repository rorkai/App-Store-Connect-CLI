package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAgeRatingViewResolvesAppInfoWhileNewVersionIsPrepared(t *testing.T) {
	tests := []struct {
		name         string
		appInfos     string
		appInfoID    string
		wantStdout   string
		wantStderr   string
		wantErrParts []string
	}{
		{
			name:       "selects the editable app info",
			appInfos:   `{"data":[{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-edit","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`,
			appInfoID:  "info-edit",
			wantStdout: `"id":"decl-info-edit"`,
			wantStderr: "auto-selected info-edit (PREPARE_FOR_SUBMISSION)",
		},
		{
			name:         "names --app-info-id when no editable app info exists",
			appInfos:     `{"data":[{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-review","attributes":{"state":"WAITING_FOR_REVIEW"}}],"links":{}}`,
			wantErrParts: []string{"pass --app-info-id with one of:", "info-live", "info-review"},
		},
		{
			name:       "ignores historical app infos",
			appInfos:   `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}},{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`,
			appInfoID:  "info-live",
			wantStdout: `"id":"decl-info-live"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps/100000001/appInfos", "fields[appInfos]=state", "limit=200"), asctest.JSON(http.StatusOK, test.appInfos))
			if test.appInfoID != "" {
				reg.Register(asctest.GET("/v1/appInfos/"+test.appInfoID+"/ageRatingDeclaration"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-`+test.appInfoID+`","attributes":{}}}`))
			}

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "age-rating", "view", "--app", "100000001", "--output", "json")

			if len(test.wantErrParts) > 0 {
				if runErr == nil {
					t.Fatalf("expected error, got stdout %q", stdout)
				}
				for _, part := range test.wantErrParts {
					if !strings.Contains(runErr.Error(), part) {
						t.Fatalf("error %q missing %q", runErr.Error(), part)
					}
				}
				return
			}
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout %q missing %q", stdout, test.wantStdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr %q missing %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestAgeRatingViewResolvesConfiguredBundleBeforeAppInfo(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "com.example.owner")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=com.example.owner", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"100000001","attributes":{"bundleId":"com.example.owner"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/100000001/appInfos", "fields[appInfos]=state", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-edit","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-edit/ageRatingDeclaration"), asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-info-edit","attributes":{}}}`))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "age-rating", "view", "--output", "json")
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !strings.Contains(stdout, `"id":"decl-info-edit"`) || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q, want configured bundle success", stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 3 || requests[0].URL.Path != "/v1/apps" || requests[1].URL.Path != "/v1/apps/100000001/appInfos" || requests[2].URL.Path != "/v1/appInfos/info-edit/ageRatingDeclaration" {
		t.Fatalf("requests=%v, want exact app lookup then App Info and declaration reads", requests)
	}
}

func TestAgeRatingViewRejectsPartialOrAmbiguousAppLookupBeforeAppInfo(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		nextBody  string
		wantErr   string
		wantCalls int
	}{
		{
			name:      "prefix-only bundle match",
			body:      `{"data":[{"type":"apps","id":"100000001","attributes":{"bundleId":"com.example.owner.preview"}}],"links":{}}`,
			wantErr:   `app "com.example.owner" not found`,
			wantCalls: 3,
		},
		{
			name:      "ambiguous exact bundle matches across pages",
			body:      `{"data":[{"type":"apps","id":"100000001","attributes":{"bundleId":"com.example.owner"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=next"}}`,
			nextBody:  `{"data":[{"type":"apps","id":"100000002","attributes":{"bundleId":"com.example.owner"}}],"links":{}}`,
			wantErr:   "2 apps match",
			wantCalls: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "com.example.owner")
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=com.example.owner", "limit=200"), asctest.JSON(http.StatusOK, test.body))
			if test.nextBody != "" {
				reg.Register(asctest.GET("/v1/apps", "cursor=next"), asctest.JSON(http.StatusOK, test.nextBody))
			} else {
				reg.Register(asctest.GET("/v1/apps", "filter[name]=com.example.owner", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
				reg.Register(asctest.GET("/v1/apps", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
			}

			stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "age-rating", "view", "--output", "json")
			if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("run error=%v stdout=%q, want %q", runErr, stdout, test.wantErr)
			}
			requests := reg.Requests()
			if len(requests) != test.wantCalls {
				t.Fatalf("requests=%v, want %d app lookup calls and no App Info/declaration calls", requests, test.wantCalls)
			}
			for _, req := range requests {
				if req.URL.Path != "/v1/apps" {
					t.Fatalf("requests=%v, want only app lookup calls", requests)
				}
			}
		})
	}
}
