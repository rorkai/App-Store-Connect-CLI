package cmdtest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAssetLibraryLifecycle(t *testing.T) {
	for _, tc := range []struct {
		group, name, flag, value, attribute string
		expected                            any
	}{
		{"images", "rename", "name", "New name", "referenceName", "New name"},
		{"videos", "rename", "name", "Video name", "referenceName", "Video name"},
		{"images", "archive", "confirm", "true", "archived", true},
		{"videos", "unarchive", "", "", "archived", false},
		{"videos", "set-poster-frame", "time-code", "00:00:01:00", "previewFrameTimeCode", "00:00:01:00"},
		{"images", "delete", "confirm", "true", "", nil},
	} {
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			setupAuth(t)
			typ := "appAssetLibraryImages"
			if tc.group == "videos" {
				typ = "appAssetLibraryVideos"
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			if tc.name == "delete" {
				reg.Register(asctest.DELETE("/v1/"+typ+"/asset"), asctest.JSON(204, ""))
			} else {
				body, err := json.Marshal(map[string]any{"data": map[string]any{"type": typ, "id": "asset", "attributes": map[string]any{tc.attribute: tc.expected}}})
				if err != nil {
					t.Fatal(err)
				}
				reg.Register(asctest.PATCH("/v1/"+typ+"/asset").JSONBody(string(body)),
					asctest.JSON(200, `{"data":{"type":"`+typ+`","id":"asset","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}}`))
			}
			args := []string{"asset-library", tc.group, tc.name, "--id", "asset", "--output", "json"}
			if tc.flag != "" {
				args = append(args, "--"+tc.flag+"="+tc.value)
			}
			out, errout, err := runAssetLibrary(t, args...)
			if err != nil || errout != "" {
				t.Fatalf("err=%v stderr=%s", err, errout)
			}
			var receipt map[string]any
			if err := json.Unmarshal([]byte(out), &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt["assetId"] != "asset" || receipt["assetType"] != typ || receipt["action"] != tc.name {
				t.Fatalf("wrong receipt %s", out)
			}
		})
	}
}

func TestAssetLibraryLifecycleValidationBeforeAuth(t *testing.T) {
	for _, args := range [][]string{
		{"images", "archive", "--id", "asset"},
		{"videos", "delete", "--id", "asset"},
		{"images", "rename", "--id", "asset", "--name", "  "},
		{"videos", "set-poster-frame", "--id", "asset", "--time-code", "bad"},
	} {
		_, _, err := runAssetLibrary(t, append([]string{"asset-library"}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "time-code") {
			t.Fatalf("expected usage validation, got %v", err)
		}
	}
}

func TestAssetLibraryLifecycleVideoEnvelope(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appAssetLibraryVideos/video"),
		asctest.JSON(200, `{"data":{"type":"appAssetLibraryVideos","id":"video","attributes":{"future":{"opaque":true},"state":"ARCHIVED"}},"meta":{"future":null}}`))
	out, _, err := runAssetLibrary(t, "asset-library", "videos", "view", "--id", "https://api.appstoreconnect.apple.com/v1/appAssetLibraryVideos/video", "--output", "json")
	if err != nil || !strings.Contains(out, `"future":{"opaque":true}`) || !strings.Contains(out, `"meta":{"future":null}`) {
		t.Fatalf("envelope changed: %s %v", out, err)
	}
}

func TestAssetLibraryLifecycleProviderRefusal(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/appAssetLibraryImages/asset").JSONBody(`{"data":{"type":"appAssetLibraryImages","id":"asset","attributes":{"archived":true}}}`),
		asctest.JSON(409, `{"errors":[{"status":"409","code":"STATE_ERROR.INVALID_ASSET_STATE","title":"Invalid state","detail":"Only an approved asset can be archived"}]}`))
	out, _, err := runAssetLibrary(t, "asset-library", "images", "archive", "--id", "asset", "--confirm")
	if err == nil || !strings.Contains(err.Error(), "Only an approved asset can be archived") || out != "" {
		t.Fatalf("refusal lost: out=%s err=%v", out, err)
	}
}

func TestAssetLibraryLifecycleRenderedReceipt(t *testing.T) {
	for _, output := range []string{"table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH("/v1/appAssetLibraryImages/asset").JSONBody(`{"data":{"type":"appAssetLibraryImages","id":"asset","attributes":{"archived":false}}}`),
				asctest.JSON(200, `{"data":{"type":"appAssetLibraryImages","id":"asset"}}`))
			out, _, err := runAssetLibrary(t, "asset-library", "images", "unarchive", "--id", "asset", "--output", output)
			if err != nil || !strings.Contains(out, "Asset ID") || !strings.Contains(out, "false") {
				t.Fatalf("receipt not rendered: %s %v", out, err)
			}
		})
	}
}
