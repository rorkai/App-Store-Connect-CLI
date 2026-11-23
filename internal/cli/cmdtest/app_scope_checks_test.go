package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestSubmitStatusAndVersionsUpdateScopeToApp covers --app on commands that
// select by submission or version ID: alone it picks the app's submission, and
// next to an ID it refuses a resource of another app before reading or writing
// it.
func TestSubmitStatusAndVersionsUpdateScopeToApp(t *testing.T) {
	type stub struct {
		matcher asctest.Matcher
		body    string
	}
	versionOf := func(versionID, appID string) stub {
		return stub{
			matcher: asctest.GET("/v1/appStoreVersions/"+versionID, "include=app"),
			body:    `{"data":{"type":"appStoreVersions","id":"` + versionID + `","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"},"relationships":{"app":{"data":{"type":"apps","id":"` + appID + `"}}}}}`,
		}
	}
	tests := []struct {
		name       string
		args       []string
		stubs      []stub
		wantStdout []string
		wantErr    string
	}{
		{
			name: "submit status app picks the submission in review",
			args: []string{"submit", "status", "--app", "100000001"},
			stubs: []stub{
				{
					matcher: asctest.GET("/v1/apps/100000001/reviewSubmissions", "limit=200"),
					body: `{"data":[` +
						`{"type":"reviewSubmissions","id":"sub-old","attributes":{"state":"COMPLETE","submittedDate":"2026-03-01T00:00:00Z"}},` +
						`{"type":"reviewSubmissions","id":"sub-live","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-01T00:00:00Z"}}]}`,
				},
				{
					matcher: asctest.GET("/v1/reviewSubmissions/sub-live", "include=app"),
					body:    `{"data":{"type":"reviewSubmissions","id":"sub-live","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-01T00:00:00Z"},"relationships":{"app":{"data":{"type":"apps","id":"100000001"}},"appStoreVersionForReview":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`,
				},
				{
					matcher: asctest.GET("/v1/appStoreVersions/version-1"),
					body:    `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}}`,
				},
			},
			wantStdout: []string{`"id":"sub-live"`, `"state":"WAITING_FOR_REVIEW"`, `"versionId":"version-1"`},
		},
		{
			name: "submit status refuses another app's submission",
			args: []string{"submit", "status", "--app", "100000001", "--id", "sub-9"},
			stubs: []stub{{
				matcher: asctest.GET("/v1/reviewSubmissions/sub-9", "include=app"),
				body:    `{"data":{"type":"reviewSubmissions","id":"sub-9","attributes":{"state":"WAITING_FOR_REVIEW"},"relationships":{"app":{"data":{"type":"apps","id":"200000002"}}}}}`,
			}},
			wantErr: `review submission "sub-9" belongs to app "200000002", not "100000001"`,
		},
		{
			name:    "submit status refuses another app's version",
			args:    []string{"submit", "status", "--app", "100000001", "--version-id", "version-9"},
			stubs:   []stub{versionOf("version-9", "200000002")},
			wantErr: `version "version-9" belongs to app "200000002", not "100000001"`,
		},
		{
			name: "versions update the app's version",
			args: []string{"versions", "update", "--app", "100000001", "--version-id", "version-1", "--copyright", "2026 Example"},
			stubs: []stub{
				versionOf("version-1", "100000001"),
				{
					matcher: asctest.PATCH("/v1/appStoreVersions/version-1"),
					body:    `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","copyright":"2026 Example"}}}`,
				},
			},
			wantStdout: []string{`"id":"version-1"`},
		},
		{
			name:    "versions update refuses another app's version",
			args:    []string{"versions", "update", "--app", "100000001", "--version-id", "version-9", "--copyright", "2026 Example"},
			stubs:   []stub{versionOf("version-9", "200000002")},
			wantErr: `version "version-9" belongs to app "200000002", not "100000001"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			reg := asctest.NewRegistry(t)
			for _, stub := range test.stubs {
				reg.Register(stub.matcher, asctest.JSON(http.StatusOK, stub.body))
			}

			args := append(append([]string{}, test.args...), "--output", "json")
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), args...)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("err = %v, want %q", err, test.wantErr)
				}
				if stdout != "" {
					t.Fatalf("expected no output, got %q", stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %s", stdout, want)
				}
			}
		})
	}
}
