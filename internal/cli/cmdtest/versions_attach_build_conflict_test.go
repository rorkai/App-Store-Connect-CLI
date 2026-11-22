package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// App Store Connect answers PATCH /v1/appStoreVersions/{id}/relationships/build
// with "The specified pre-release build could not be added." both when the
// version is in review and when the build already ships with another version.
// Titles and details are the ones App Store Connect sends for each case.
func TestVersionsAttachBuildConflictNamesTheBlocker(t *testing.T) {
	const (
		inReviewBody  = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"A relationship value is not acceptable for the current resource state.","detail":"The specified pre-release build could not be added.","source":{"pointer":"/data"}}]}`
		usedBuildBody = `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The specified pre-release build could not be added.","source":{"pointer":"/data"}}]}`
	)
	version := func(state string) string {
		return `{"data":{"type":"appStoreVersions","id":"version-2","attributes":{"platform":"IOS","versionString":"1.0.1","appStoreState":"` + state + `","appVersionState":"` + state + `"}}}`
	}
	tests := []struct {
		name       string
		body       string
		state      string
		holder     string
		wantStderr string
	}{
		{
			name:       "version in review",
			body:       inReviewBody,
			state:      "WAITING_FOR_REVIEW",
			wantStderr: "Hint: Version 1.0.1 (IOS) is WAITING_FOR_REVIEW, and App Store Connect changes a version's build only while it is PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED, REJECTED, METADATA_REJECTED, INVALID_BINARY. To change the build of a version in review, cancel its review submission first",
		},
		{
			name:       "build already ships with another version",
			body:       usedBuildBody,
			state:      "PREPARE_FOR_SUBMISSION",
			holder:     `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.0.0","appVersionState":"READY_FOR_DISTRIBUTION"}}}`,
			wantStderr: "Hint: Build build-1 is already the build of version 1.0.0 (READY_FOR_DISTRIBUTION, id version-1), and a build ships with one version only. Attach a build no version uses yet (`asc builds list --app APP_ID --version 1.0.1`), or upload a new build for 1.0.1.\n",
		},
		{
			name:       "editable version and an unused build keep Apple's error",
			body:       usedBuildBody,
			state:      "PREPARE_FOR_SUBMISSION",
			holder:     `{"data":null}`,
			wantStderr: "Error: versions attach-build: The provided entity includes a relationship with an invalid value: The specified pre-release build could not be added.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.PATCH("/v1/appStoreVersions/version-2/relationships/build"), asctest.JSON(http.StatusConflict, test.body))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-2"), asctest.JSON(http.StatusOK, version(test.state)))
			if test.holder != "" {
				reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"), asctest.JSON(http.StatusOK, test.holder))
			}

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "versions", "attach-build", "--version-id", "version-2", "--build-id", "build-1")

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "The specified pre-release build could not be added.") {
				t.Fatalf("stderr %q dropped Apple's detail", stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
			}
		})
	}
}
