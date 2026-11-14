package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// A command prints the response type it decoded, so an attribute that type
// does not name never reaches JSON output.
func TestJSONOutputKeepsAttributesFromTheAPISchema(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name      string
		args      []string
		request   asctest.Matcher
		resource  string
		attribute string
		value     string
		deps      bool // the command is built on shared.Deps: stub its client and read its deps streams
	}{
		{
			name:      "video preview poster frame image",
			args:      []string{"video-previews", "set-poster-frame", "--id", "preview-1", "--time-code", "00:00:05:00"},
			request:   asctest.PATCH("/v1/appPreviews/preview-1"),
			resource:  "appPreviews",
			attribute: "previewFrameImage",
			value:     `{"image":{"templateUrl":"https://example.test/{w}x{h}.{f}","width":1290,"height":2796},"state":{"state":"COMPLETE"}}`,
		},
		{
			name:      "experiment treatment app icon",
			args:      []string{"product-pages", "experiments", "treatments", "view", "--treatment-id", "treatment-1"},
			request:   asctest.GET("/v1/appStoreVersionExperimentTreatments/treatment-1"),
			resource:  "appStoreVersionExperimentTreatments",
			attribute: "appIcon",
			value:     `{"templateUrl":"https://example.test/{w}x{h}.{f}","width":1024,"height":1024}`,
			deps:      true,
		},
		{
			name:      "subscription review screenshot asset type",
			args:      []string{"subscriptions", "review", "screenshots", "view", "--screenshot-id", "screenshot-1"},
			request:   asctest.GET("/v1/subscriptionAppStoreReviewScreenshots/screenshot-1"),
			resource:  "subscriptionAppStoreReviewScreenshots",
			attribute: "assetType",
			value:     `"SCREENSHOT"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(tt.request, asctest.JSON(http.StatusOK,
				`{"data":{"type":"`+tt.resource+`","id":"1","attributes":{"`+tt.attribute+`":`+tt.value+`}}}`))

			args := append(append([]string{}, tt.args...), "--output", "json")
			var stdout string
			var err error
			if tt.deps {
				stdout, _, err = runRootWithDeps(t, stubClient(t, reg), args...)
			} else {
				reg.Install()
				stdout, _, err = runCommand(t, args)
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var printed struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
				t.Fatalf("decode stdout %q: %v", stdout, err)
			}
			var want any
			if err := json.Unmarshal([]byte(tt.value), &want); err != nil {
				t.Fatalf("decode wanted %s: %v", tt.attribute, err)
			}
			if got := printed.Data.Attributes[tt.attribute]; !reflect.DeepEqual(got, want) {
				t.Fatalf("%s in JSON output = %v, want %v (stdout: %s)", tt.attribute, got, want, stdout)
			}
		})
	}
}
