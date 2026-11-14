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
			reg.Register(asctest.GET("/v1/apps/app-1/appInfos", "fields[appInfos]=state", "limit=200"), asctest.JSON(http.StatusOK, test.appInfos))
			if test.appInfoID != "" {
				reg.Register(asctest.GET("/v1/appInfos/"+test.appInfoID+"/ageRatingDeclaration"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-`+test.appInfoID+`","attributes":{}}}`))
			}

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "age-rating", "view", "--app", "app-1", "--output", "json")

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
