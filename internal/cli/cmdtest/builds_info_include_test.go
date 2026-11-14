package cmdtest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestRun_BuildsInfoInclude(t *testing.T) {
	const included = `[{"type":"buildBetaDetails","id":"bbd-1","attributes":{"autoNotifyEnabled":false,"internalBuildState":"READY_FOR_BETA_TESTING","externalBuildState":"READY_FOR_BETA_SUBMISSION"}}]`
	detailBody := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}},"included":` + included + `}`
	listBody := `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}}],"links":{}}`

	tests := []struct {
		name         string
		args         []string
		register     func(reg *asctest.Registry)
		wantCode     int
		wantRequests []string
		wantStderr   string
	}{
		{
			name: "build id",
			args: []string{"--build-id", "build-1", "--include", "buildBetaDetail,betaGroups"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/builds/build-1", "include=preReleaseVersion,buildBetaDetail,betaGroups"), asctest.JSON(http.StatusOK, detailBody))
			},
			wantCode:     cmd.ExitSuccess,
			wantRequests: []string{"/v1/builds/build-1 include=preReleaseVersion,buildBetaDetail,betaGroups"},
		},
		{
			name: "app build number",
			args: []string{"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--include", "buildBetaDetail"},
			register: func(reg *asctest.Registry) {
				reg.Register(buildsByNumberIOS42, asctest.JSON(http.StatusOK, listBody))
				reg.Register(asctest.GET("/v1/builds/build-1", "include=preReleaseVersion,buildBetaDetail"), asctest.JSON(http.StatusOK, detailBody))
			},
			wantCode: cmd.ExitSuccess,
			wantRequests: []string{
				"/v1/builds",
				"/v1/builds/build-1 include=preReleaseVersion,buildBetaDetail",
			},
		},
		{
			name:       "unknown value",
			args:       []string{"--build-id", "build-1", "--include", "buildBetaDetail,bogus"},
			register:   func(*asctest.Registry) {},
			wantCode:   cmd.ExitUsage,
			wantStderr: `unknown value "bogus"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			test.register(reg)

			code, stdout, stderr := runBuildsInfoSelector(t, append(test.args, "--output", "json")...)

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, test.wantCode, stderr)
			}
			var requests []string
			for _, req := range reg.Requests() {
				request := req.URL.Path
				if req.URL.Path == "/v1/builds/build-1" {
					request += " include=" + req.URL.Query().Get("include")
				}
				requests = append(requests, request)
			}
			if !reflect.DeepEqual(requests, test.wantRequests) {
				t.Fatalf("requests = %q, want %q", requests, test.wantRequests)
			}
			if test.wantStderr != "" {
				if !strings.Contains(stderr, test.wantStderr) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
				}
				return
			}

			var got struct {
				Included any `json:"included"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("decode stdout %q: %v", stdout, err)
			}
			var wantIncluded any
			if err := json.Unmarshal([]byte(included), &wantIncluded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Included, wantIncluded) {
				t.Fatalf("included = %#v, want %s", got.Included, included)
			}
		})
	}
}
