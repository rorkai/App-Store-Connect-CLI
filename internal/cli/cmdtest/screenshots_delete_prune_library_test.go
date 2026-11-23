package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Deleting a screenshot removes its placement but leaves the Asset Library
// image with the same ID. --prune-library deletes that image only when
// nothing else places it.
func TestScreenshotsDeletePruneLibrary(t *testing.T) {
	const notFound = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAssetLibraryImages' with id 'shot-1'"}]}`
	const image = `{"data":{"type":"appAssetLibraryImages","id":"shot-1","attributes":{"fileName":"01.png","state":"PREPARE_FOR_SUBMISSION"}}}`
	tests := []struct {
		name       string
		image      asctest.Responder
		placements asctest.Responder
		deleted    bool
		wantStdout string
	}{
		{
			name:       "unplaced image is deleted",
			image:      asctest.JSON(http.StatusOK, image),
			placements: asctest.JSON(http.StatusOK, `{"data":[],"links":{},"meta":{"paging":{"total":0,"limit":1}}}`),
			deleted:    true,
			wantStdout: `{"id":"shot-1","deleted":true,"libraryImage":{"id":"shot-1","state":"deleted"}}`,
		},
		{
			name:       "image placed elsewhere is kept",
			image:      asctest.JSON(http.StatusOK, image),
			placements: asctest.JSON(http.StatusOK, `{"data":[{"type":"appAssetLibraryPlacements","id":"placement-2"}],"links":{},"meta":{"paging":{"total":1,"limit":1}}}`),
			wantStdout: `{"id":"shot-1","deleted":true,"libraryImage":{"id":"shot-1","state":"kept"}}`,
		},
		{
			// Apple lists no placements for an unknown image ID instead of
			// a 404, so only the image read can tell it is absent.
			name:       "screenshot without a library image",
			image:      asctest.JSON(http.StatusNotFound, notFound),
			wantStdout: `{"id":"shot-1","deleted":true,"libraryImage":{"id":"shot-1","state":"absent"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.DELETE("/v1/appScreenshots/shot-1"), asctest.JSON(http.StatusNoContent, ""))
			reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1"), test.image)
			if test.placements != nil {
				reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1/placements", "limit=1"), test.placements)
			}
			if test.deleted {
				reg.Register(asctest.DELETE("/v1/appAssetLibraryImages/shot-1"), asctest.JSON(http.StatusNoContent, ""))
			}

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "screenshots", "delete", "--id", "shot-1", "--confirm", "--prune-library", "--output", "json")
			if err != nil {
				t.Fatalf("screenshots delete: %v; stderr: %s", err, stderr)
			}
			if strings.TrimSpace(stdout) != test.wantStdout {
				t.Fatalf("stdout = %s, want %s", stdout, test.wantStdout)
			}
		})
	}
}

// If the screenshot is gone but the image delete fails, the command exits
// non-zero and says which half happened and how to finish it.
func TestScreenshotsDeletePruneLibraryReportsImageDeleteFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.DELETE("/v1/appScreenshots/shot-1"), asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"appAssetLibraryImages","id":"shot-1"}}`))
	reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1/placements", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.DELETE("/v1/appAssetLibraryImages/shot-1"), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The asset cannot be deleted","detail":"The asset cannot be deleted in its current state."}]}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "screenshots", "delete", "--id", "shot-1", "--confirm", "--prune-library", "--output", "json")
	if err == nil {
		t.Fatalf("screenshots delete succeeded, want the image delete failure; stdout: %s", stdout)
	}
	for _, want := range []string{"screenshot shot-1 was deleted", `asc asset-library images delete --id "shot-1" --confirm`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

// The image is deleted only on a clear "no placements" answer. A response
// that does not name the image or does not list placements keeps the image,
// exits non-zero, and points at the placements check rather than at an
// unchecked delete.
func TestScreenshotsDeletePruneLibraryKeepsImageOnUnclearAnswer(t *testing.T) {
	const image = `{"data":{"type":"appAssetLibraryImages","id":"shot-1"}}`
	tests := []struct {
		name       string
		image      string
		placements string
	}{
		{name: "placements without data", image: image, placements: `{}`},
		{name: "placements with null data", image: image, placements: `{"data":null}`},
		{name: "placements with only links", image: image, placements: `{"links":{}}`},
		{name: "placements with errors", image: image, placements: `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR"}]}`},
		{name: "placements data is not a list", image: image, placements: `{"data":{"type":"appAssetLibraryPlacements","id":"p-1"}}`},
		{name: "image read names another image", image: `{"data":{"type":"appAssetLibraryImages","id":"shot-2"}}`},
		{name: "image read names another type", image: `{"data":{"type":"appScreenshots","id":"shot-1"}}`},
		{name: "image read without data", image: `{}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.DELETE("/v1/appScreenshots/shot-1"), asctest.JSON(http.StatusNoContent, ""))
			reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1"), asctest.JSON(http.StatusOK, test.image))
			if test.placements != "" {
				reg.Register(asctest.GET("/v1/appAssetLibraryImages/shot-1/placements", "limit=1"), asctest.JSON(http.StatusOK, test.placements))
			}

			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "screenshots", "delete", "--id", "shot-1", "--confirm", "--prune-library", "--output", "json")
			if err == nil {
				t.Fatalf("screenshots delete succeeded, want it to keep the image and fail; stdout: %s", stdout)
			}
			if !strings.Contains(err.Error(), `asc asset-library images placements --id "shot-1"`) {
				t.Fatalf("error = %q, want it to point at the placements check", err.Error())
			}
			if strings.Contains(err.Error(), `images delete --id "shot-1" --confirm`) {
				t.Fatalf("error = %q, want no unchecked delete command", err.Error())
			}
		})
	}
}
