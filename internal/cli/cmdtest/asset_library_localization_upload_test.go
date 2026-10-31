package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const libraryUploadRefData = `{"data":[{"type":"appAssetLibraryRefData","id":"1","attributes":{
"features":[{"featureId":"APP_STORE_VERSIONS","placementPolicies":[
  {"placementType":"APP_SCREENSHOT","groupLimits":[{"groupIds":["IPHONE_DUO_PROFILE","IPHONE_DYNAMIC_ISLAND_LARGE_PROFILE"],"maxCount":3}]},
  {"placementType":"APP_PREVIEW","groupLimits":[{"groupIds":["IPHONE_DUO_PROFILE"],"maxCount":3}]}]}],
"placementProfileGroups":[
  {"placementProfileGroupId":"IPHONE_DYNAMIC_ISLAND_LARGE_PROFILE","platform":"IPHONE_APP_STORE","displayClassId":"IPHONE_DYNAMIC_ISLAND_LARGE_DISPLAY"},
  {"placementProfileGroupId":"IPHONE_DUO_PROFILE","platform":"IPHONE_APP_STORE","displayClassId":"IPHONE_DUO"},
  {"placementProfileGroupId":"IMESSAGE_IPHONE_DUO_PROFILE","platform":"IMESSAGE_APP_STORE","displayClassId":"IPHONE_DUO"}],
"placementTypes":[{"placementTypeId":"APP_PREVIEW","specMappings":[{"placementGroupId":"IPHONE_DUO_PROFILE","specs":["duo-preview"]},{"placementGroupId":"IPAD_13_PROFILE","specs":["ipad-preview"]}]}],
"videoSpecs":[
  {"specId":"duo-preview","dimensions":{"minWidth":1398,"maxWidth":1398,"minHeight":2034,"maxHeight":2034},"compatiblePlacementTypes":["APP_PREVIEW"],"frameRates":[{"minFps":30,"maxFps":30}],"duration":{"min":"PT5S","max":"PT30S"},"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000},
  {"specId":"ipad-preview","dimensions":{"minWidth":1200,"maxWidth":1200,"minHeight":1600,"maxHeight":1600},"compatiblePlacementTypes":["APP_PREVIEW"],"frameRates":[{"minFps":30,"maxFps":30}],"duration":{"min":"PT5S","max":"PT30S"},"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000}]}}]}`

// libraryUploadFake answers the read chain and records every write. Assets are
// numbered in reservation order so tests can assert ordering end to end.
type libraryUploadFake struct {
	t                *testing.T
	existing         []string
	failUploadOf     string
	failPlacementFor string
	failOrdering     bool
	writes           []string
	reservedNames    []string
	placedAssets     []string
	placedGroups     []string
	placedTypes      []string
	orderedGroup     string
	orderedIDs       []string
	placementQuery   string
}

func (f *libraryUploadFake) install() {
	f.t.Helper()
	original := http.DefaultTransport
	f.t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(f.roundTrip)
}

func (f *libraryUploadFake) roundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Path
	if req.URL.Host == "storage.example" {
		return jsonResponse(200, `{}`)
	}
	if req.Method != http.MethodGet {
		f.writes = append(f.writes, key)
	}
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	switch {
	case key == "GET /v1/appStoreVersionLocalizations/LOC_1":
		return jsonResponse(200, `{"data":{"type":"appStoreVersionLocalizations","id":"LOC_1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VER_1"}}}}}`)
	case key == "GET /v1/appStoreVersions/VER_1":
		return jsonResponse(200, `{"data":{"type":"appStoreVersions","id":"VER_1","relationships":{"app":{"data":{"type":"apps","id":"APP_1"}}}}}`)
	case key == "GET /v1/apps/APP_1/assetLibrary":
		return jsonResponse(200, `{"data":{"type":"appAssetLibraries","id":"LIB_1"}}`)
	case key == "GET /v1/appAssetLibraryRefData":
		return jsonResponse(200, libraryUploadRefData)
	case key == "GET /v1/appStoreVersionLocalizations/LOC_1/placements":
		f.placementQuery = req.URL.Query().Encode()
		items := make([]string, len(f.existing))
		for i, id := range f.existing {
			items[i] = fmt.Sprintf(`{"type":"appAssetLibraryPlacements","id":%q}`, id)
		}
		return jsonResponse(200, `{"data":[`+strings.Join(items, ",")+`],"links":{}}`)
	case key == "POST /v1/appAssetLibraryImages" || key == "POST /v1/appAssetLibraryVideos":
		kind := strings.TrimPrefix(req.URL.Path, "/v1/")
		var payload struct {
			Data struct {
				Attributes struct {
					FileName string `json:"fileName"`
					FileSize int64  `json:"fileSize"`
					Category string `json:"category"`
				} `json:"attributes"`
				Relationships struct {
					AssetLibrary struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"assetLibrary"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			f.t.Fatal(err)
		}
		if payload.Data.Attributes.Category != "APP_SCREENSHOTS_AND_PREVIEWS" || payload.Data.Relationships.AssetLibrary.Data.ID != "LIB_1" {
			f.t.Fatalf("reservation = %s", body)
		}
		f.reservedNames = append(f.reservedNames, payload.Data.Attributes.FileName)
		if payload.Data.Attributes.FileName == f.failUploadOf {
			return jsonResponse(422, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"rejected","detail":"rejected"}]}`)
		}
		return jsonResponse(201, fmt.Sprintf(`{"data":{"type":%q,"id":"asset-%d","attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":"https://storage.example/part","offset":0,"length":%d}]}}}`, kind, len(f.reservedNames), payload.Data.Attributes.FileSize))
	case req.Method == http.MethodPatch && (strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryImages/") || strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryVideos/")):
		return jsonResponse(200, `{"data":{}}`)
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryImages/"):
		id := strings.TrimPrefix(req.URL.Path, "/v1/appAssetLibraryImages/")
		return jsonResponse(200, fmt.Sprintf(`{"data":{"type":"appAssetLibraryImages","id":%q,"attributes":{"state":"PREPARE_FOR_SUBMISSION","imageAsset":{"width":1398,"height":2034}}}}`, id))
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryVideos/"):
		id := strings.TrimPrefix(req.URL.Path, "/v1/appAssetLibraryVideos/")
		return jsonResponse(200, fmt.Sprintf(`{"data":{"type":"appAssetLibraryVideos","id":%q,"attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"spec-1","videoAsset":"https://cdn.example.test/video.mp4"}}}`, id))
	case key == "POST /v1/appAssetLibraryPlacements":
		var payload struct {
			Data struct {
				Attributes struct {
					PlacementType  string `json:"placementType"`
					PlacementGroup string `json:"placementGroup"`
				} `json:"attributes"`
				Relationships struct {
					Image, Video, AppStoreVersionLocalization struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					}
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			f.t.Fatal(err)
		}
		relationships := payload.Data.Relationships
		if relationships.AppStoreVersionLocalization.Data.ID != "LOC_1" {
			f.t.Fatalf("placement = %s", body)
		}
		asset := relationships.Image.Data.ID + relationships.Video.Data.ID
		if asset == f.failPlacementFor {
			return jsonResponse(409, `{"errors":[{"status":"409","code":"STATE_ERROR.INVALID_STATE","title":"not editable","detail":"not editable"}]}`)
		}
		f.placedAssets = append(f.placedAssets, asset)
		f.placedGroups = append(f.placedGroups, payload.Data.Attributes.PlacementGroup)
		f.placedTypes = append(f.placedTypes, payload.Data.Attributes.PlacementType)
		return jsonResponse(201, fmt.Sprintf(`{"data":{"type":"appAssetLibraryPlacements","id":"placement-%s","attributes":{"state":"PARENT_PREPARE_FOR_SUBMISSION"}}}`, asset))
	case key == "POST /v1/appAssetLibraryPlacementOrderingRequests":
		if f.failOrdering {
			return jsonResponse(409, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"no","detail":"no"}]}`)
		}
		var payload struct {
			Data struct {
				Attributes struct {
					PlacementGroup string `json:"placementGroup"`
				} `json:"attributes"`
				Relationships struct {
					OrderedPlacements struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"orderedPlacements"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			f.t.Fatal(err)
		}
		f.orderedGroup = payload.Data.Attributes.PlacementGroup
		for _, item := range payload.Data.Relationships.OrderedPlacements.Data {
			f.orderedIDs = append(f.orderedIDs, item.ID)
		}
		return jsonResponse(201, `{"data":{"type":"appAssetLibraryPlacementOrderingRequests","id":"order-1"}}`)
	}
	f.t.Fatalf("unexpected request %s", key)
	return nil, nil
}

func duoScreenshotFolder(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		writePNG(t, filepath.Join(dir, name), 1398, 2034)
	}
	return dir
}

func libraryScreenshotArgs(path string, extra ...string) []string {
	args := []string{"screenshots", "upload", "--version-localization", "LOC_1", "--path", path, "--device-type", "IPHONE_DUO", "--asset-library", "--output", "json"}
	return append(args, extra...)
}

func decodeLibraryReceipt(t *testing.T, stdout string) asc.AssetLibraryLocalizationUploadResult {
	t.Helper()
	var receipt asc.AssetLibraryLocalizationUploadResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout %q: %v", stdout, err)
	}
	return receipt
}

func libraryItemStates(receipt asc.AssetLibraryLocalizationUploadResult) []string {
	states := make([]string, len(receipt.Items))
	for i, item := range receipt.Items {
		states[i] = item.State
	}
	return states
}

func TestScreenshotsUploadAssetLibraryPlacesDuoInFileOrderAfterExistingPlacements(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_UPLOAD_TIMEOUT", "3s")
	fake := &libraryUploadFake{t: t, existing: []string{"placement-old"}}
	fake.install()
	stdout, stderr, err := runRootCommand(t, libraryScreenshotArgs(duoScreenshotFolder(t, "02.png", "01.png")))
	if err != nil {
		t.Fatalf("upload: %v; stderr: %s", err, stderr)
	}
	if want := []string{"01.png", "02.png"}; !reflect.DeepEqual(fake.reservedNames, want) {
		t.Fatalf("reserved %v, want %v", fake.reservedNames, want)
	}
	if want := []string{"asset-1", "asset-2"}; !reflect.DeepEqual(fake.placedAssets, want) {
		t.Fatalf("placed %v, want %v", fake.placedAssets, want)
	}
	if want := []string{"IPHONE_DUO_PROFILE", "IPHONE_DUO_PROFILE"}; !reflect.DeepEqual(fake.placedGroups, want) {
		t.Fatalf("groups %v, want %v", fake.placedGroups, want)
	}
	if fake.placedTypes[0] != "APP_SCREENSHOT" {
		t.Fatalf("placement type %q", fake.placedTypes[0])
	}
	if want := []string{"placement-old", "placement-asset-1", "placement-asset-2"}; fake.orderedGroup != "IPHONE_DUO_PROFILE" || !reflect.DeepEqual(fake.orderedIDs, want) {
		t.Fatalf("ordered %s %v, want %v", fake.orderedGroup, fake.orderedIDs, want)
	}
	for _, want := range []string{"filter%5BplacementGroup%5D=IPHONE_DUO_PROFILE", "filter%5BplacementType%5D=APP_SCREENSHOT", "sort=placementGroupPosition"} {
		if !strings.Contains(fake.placementQuery, want) {
			t.Fatalf("placement query %q lacks %q", fake.placementQuery, want)
		}
	}
	receipt := decodeLibraryReceipt(t, stdout)
	if !receipt.Complete || !receipt.Ordered || receipt.OrderingRequestID != "order-1" || receipt.LibraryID != "LIB_1" || receipt.PlacementGroup != "IPHONE_DUO_PROFILE" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if want := []string{"placed", "placed"}; !reflect.DeepEqual(libraryItemStates(receipt), want) {
		t.Fatalf("states = %v", libraryItemStates(receipt))
	}
}

func TestScreenshotsUploadAssetLibraryDryRunWritesNothing(t *testing.T) {
	setupAuth(t)
	fake := &libraryUploadFake{t: t}
	fake.install()
	stdout, stderr, err := runRootCommand(t, libraryScreenshotArgs(duoScreenshotFolder(t, "01.png"), "--dry-run"))
	if err != nil {
		t.Fatalf("dry run: %v; stderr: %s", err, stderr)
	}
	if len(fake.writes) != 0 {
		t.Fatalf("dry run wrote %v", fake.writes)
	}
	receipt := decodeLibraryReceipt(t, stdout)
	if !receipt.DryRun || receipt.Complete || receipt.PlacementGroup != "IPHONE_DUO_PROFILE" || receipt.Items[0].State != "would-upload" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestScreenshotsUploadAssetLibraryPartialFailureKeepsReceipt(t *testing.T) {
	tests := []struct {
		name     string
		fake     libraryUploadFake
		states   []string
		placed   int
		ordered  bool
		wantErrs []string
	}{
		{name: "upload fails before any placement", fake: libraryUploadFake{failUploadOf: "02.png"}, states: []string{"uploaded", "upload-failed", "pending"}, wantErrs: []string{"02.png", "1 of 3 file(s) uploaded", "0 placed"}},
		{name: "placement fails midway", fake: libraryUploadFake{failPlacementFor: "asset-2"}, states: []string{"placed", "placement-failed", "uploaded"}, placed: 1, wantErrs: []string{"02.png", "3 of 3 file(s) uploaded", "1 placed"}},
		{name: "ordering fails after placement", fake: libraryUploadFake{failOrdering: true}, states: []string{"placed", "placed", "placed"}, placed: 3, wantErrs: []string{"3 placed", "order"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_UPLOAD_TIMEOUT", "3s")
			fake := test.fake
			fake.t = t
			fake.install()
			stdout, _, err := runRootCommand(t, libraryScreenshotArgs(duoScreenshotFolder(t, "01.png", "02.png", "03.png")))
			if err == nil {
				t.Fatal("expected failure")
			}
			for _, want := range test.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q lacks %q", err, want)
				}
			}
			if errors.Is(err, flag.ErrHelp) {
				t.Fatalf("partial failure reported as usage: %v", err)
			}
			if len(fake.placedAssets) != test.placed {
				t.Fatalf("placed %v, want %d", fake.placedAssets, test.placed)
			}
			receipt := decodeLibraryReceipt(t, stdout)
			if receipt.Complete || receipt.Ordered {
				t.Fatalf("receipt claims success: %+v", receipt)
			}
			if !reflect.DeepEqual(libraryItemStates(receipt), test.states) {
				t.Fatalf("states = %v, want %v", libraryItemStates(receipt), test.states)
			}
			if receipt.Items[0].AssetID != "asset-1" {
				t.Fatalf("receipt lost the uploaded asset: %+v", receipt.Items[0])
			}
		})
	}
}

func TestScreenshotsUploadAssetLibraryRejectsBeforeAnyWrite(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		extra    []string
		want     string
	}{
		{name: "group capacity", existing: []string{"a", "b"}, want: "IPHONE_DUO_PROFILE allows 3 placement(s)"},
		{name: "group the version does not offer", extra: []string{"--placement-group", "IMESSAGE_IPHONE_DUO_PROFILE"}, want: `placement group "IMESSAGE_IPHONE_DUO_PROFILE" is not offered`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			fake := &libraryUploadFake{t: t, existing: test.existing}
			fake.install()
			stdout, _, err := runRootCommand(t, libraryScreenshotArgs(duoScreenshotFolder(t, "01.png", "02.png"), test.extra...))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
			if len(fake.writes) != 0 || stdout != "" {
				t.Fatalf("writes %v, stdout %q", fake.writes, stdout)
			}
		})
	}
}

func TestLibraryUploadFlagConflicts(t *testing.T) {
	file := filepath.Join(t.TempDir(), "duo.png")
	writePNG(t, file, 1398, 2034)
	base := []string{"--version-localization", "LOC_1", "--path", file, "--device-type", "IPHONE_DUO"}
	screenshots := func(extra ...string) []string {
		return append(append([]string{"screenshots", "upload"}, base...), extra...)
	}
	previews := func(extra ...string) []string {
		return append(append([]string{"video-previews", "upload"}, base...), extra...)
	}
	tests := []struct {
		args []string
		want string
	}{
		{screenshots("--placement-group", "G"), "--placement-group requires --asset-library"},
		{screenshots("--asset-library", "--skip-existing"), "--asset-library cannot be combined with --skip-existing"},
		{screenshots("--asset-library", "--replace", "--confirm"), "--asset-library cannot be combined with --replace"},
		{screenshots("--asset-library", "--concurrency", "2"), "--asset-library cannot be combined with --concurrency"},
		{[]string{"screenshots", "upload", "--asset-library", "--app", "1", "--version", "1.0", "--locale", "en-US", "--path", file, "--device-type", "IPHONE_DUO"}, "--asset-library requires --version-localization"},
		{[]string{"screenshots", "upload", "--asset-library", "--resume", "failures.json"}, "--resume cannot be combined with"},
		{previews("--placement-group", "G"), "--placement-group requires --asset-library"},
		{previews("--asset-library", "--skip-existing"), "--asset-library cannot be combined with --skip-existing"},
		{previews("--asset-library", "--replace", "--confirm"), "--asset-library cannot be combined with --replace"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				return nil, nil
			})
			_, stderr, err := runRootCommand(t, test.args)
			if err == nil || !strings.Contains(err.Error()+stderr, test.want) {
				t.Fatalf("err = %v, stderr = %q, want %q", err, stderr, test.want)
			}
		})
	}
}

// libraryPreviewFile writes a video and an ffprobe that reports the dimensions.
func libraryPreviewFile(t *testing.T, width, height int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ffprobe fixture")
	}
	file := filepath.Join(t.TempDir(), "preview.mp4")
	if err := os.WriteFile(file, []byte("bounded video fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := fmt.Sprintf(`#!/bin/sh
printf '%%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":%d,"height":%d,"avg_frame_rate":"30/1"}],"format":{"duration":"10","format_name":"mov,mp4,m4a,3gp,3g2,mj2","tags":{"major_brand":"isom"}}}'
`, width, height)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "ffprobe"), []byte(probe), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return file
}

func TestVideoPreviewsUploadAssetLibraryRejectsVideoFromAnotherGroupBeforeUpload(t *testing.T) {
	setupAuth(t)
	file := libraryPreviewFile(t, 1200, 1600)
	fake := &libraryUploadFake{t: t}
	fake.install()
	_, _, err := runRootCommand(t, []string{"video-previews", "upload", "--version-localization", "LOC_1", "--path", file, "--device-type", "IPHONE_DUO", "--asset-library", "--dry-run", "--output", "json"})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("err = %v", err)
	}
	if len(fake.writes) != 0 {
		t.Fatalf("writes = %v", fake.writes)
	}
}

func TestVideoPreviewsUploadAssetLibraryPlacesDuoPreview(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_UPLOAD_TIMEOUT", "3s")
	file := libraryPreviewFile(t, 1398, 2034)
	fake := &libraryUploadFake{t: t}
	fake.install()
	stdout, stderr, err := runRootCommand(t, []string{"video-previews", "upload", "--version-localization", "LOC_1", "--path", file, "--device-type", "IPHONE_DUO", "--asset-library", "--output", "json"})
	if err != nil {
		t.Fatalf("upload: %v; stderr: %s", err, stderr)
	}
	if want := []string{"POST /v1/appAssetLibraryVideos", "PATCH /v1/appAssetLibraryVideos/asset-1", "POST /v1/appAssetLibraryPlacements"}; !reflect.DeepEqual(fake.writes, want) {
		t.Fatalf("writes = %v, want %v", fake.writes, want)
	}
	if fake.placedTypes[0] != "APP_PREVIEW" || fake.placedGroups[0] != "IPHONE_DUO_PROFILE" {
		t.Fatalf("placed %v %v", fake.placedTypes, fake.placedGroups)
	}
	receipt := decodeLibraryReceipt(t, stdout)
	if !receipt.Complete || !receipt.Ordered || receipt.OrderingRequestID != "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}
