package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func runAssetLibrary(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("dev")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestAssetLibraryReads(t *testing.T) {
	for _, tc := range []struct {
		args          []string
		path, payload string
	}{
		{[]string{"view", "--app", "app-1"}, "/v1/apps/app-1/assetLibrary", `{"data":{"type":"appAssetLibraries","id":"lib-1","relationships":{"images":{"links":{"related":"new"}}}},"future":null}`},
		{[]string{"images", "list", "--library-id", "lib-1", "--limit", "1"}, "/v1/appAssetLibraries/lib-1/images", `{"data":[{"type":"appAssetLibraryImages","id":"img-1","attributes":{"referenceName":"Header","state":"APPROVED","future":{"new":true}}}],"links":{},"meta":{"paging":{"total":1}}}`},
		{[]string{"images", "view", "--id", "img-1"}, "/v1/appAssetLibraryImages/img-1", `{"data":{"type":"appAssetLibraryImages","id":"img-1","attributes":{"imageAsset":null}}}`},
		{[]string{"images", "placements", "--id", "img-1"}, "/v1/appAssetLibraryImages/img-1/placements", `{"data":[{"type":"appAssetLibraryPlacements","id":"place-1","attributes":{"placementType":"PRODUCT_PAGE_HEADER_ASSET"}}]}`},
		{[]string{"videos", "list", "--library-id", "lib-1"}, "/v1/appAssetLibraries/lib-1/videos", `{"data":[]}`},
		{[]string{"specs"}, "/v1/appAssetLibraryRefData", `{"data":[{"type":"appAssetLibraryRefData","id":"1","attributes":{"imageSpecs":[],"videoSpecs":[],"future":1}}]}`},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" || req.URL.Path != tc.path {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
				if strings.Contains(strings.Join(tc.args, " "), "--limit") && req.URL.Query().Get("limit") != "1" {
					t.Fatal("limit omitted")
				}
				return jsonResponse(200, tc.payload)
			})
			stdout, stderr, err := runAssetLibrary(t, append([]string{"asset-library"}, tc.args...)...)
			if err != nil || stderr != "" {
				t.Fatalf("error=%v stderr=%q", err, stderr)
			}
			var got, want any
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal([]byte(tc.payload), &want)
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("envelope changed: %s", stdout)
			}
		})
	}
}

func TestAssetLibraryValidationBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	for _, args := range [][]string{
		{"view"},
		{"images", "list"},
		{"images", "view"},
		{"images", "list", "--library-id", "lib", "--limit", "-1"},
		{"images", "list", "--library-id", "lib", "--limit", "201"},
		{"videos", "list", "--library-id", "lib", "--limit", "201"},
		{"images", "placements", "--id", "img", "--limit", "201"},
		{"images", "placements", "--id", "img", "--placement-type", "HEADER"},
		{"videos", "placements", "--id", "vid", "--placement-group", "a/b"},
		{"images", "placements", "--id", "img", "--sort", "referenceName"},
		{"videos", "placements", "--id", "vid", "--state", "failed"},
		{"images", "placements", "--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-type", "APP_SCREENSHOT"},
		{"videos", "placements", "--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-group", "DEFAULT_PROFILE"},
		{"images", "placements", "--next", "https://api.appstoreconnect.apple.com/v1/apps", "--state", "FAILED"},
		{"videos", "placements", "--next", "https://api.appstoreconnect.apple.com/v1/apps", "--sort", "createdDate"},
		{"images", "placements", "--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-id", "place"},
		{"images", "list", "--next", "https://evil.example/v1/apps"},
		{"images", "list", "--library-id", "lib", "--next", "https://api.appstoreconnect.apple.com/v1/apps"},
		{"images", "view", "--id", "../apps"},
		{"specs", "unexpected"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, err := runAssetLibrary(t, append([]string{"asset-library"}, args...)...)
			if !isUsageClassError(err) || stderr == "" {
				t.Fatalf("expected reported usage error before auth, got %v, stderr=%q", err, stderr)
			}
		})
	}
}

func TestAssetLibraryPaginationAndTable(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(200, `{"data":[{"type":"appAssetLibraryImages","id":"first","attributes":{"referenceName":"First","state":"APPROVED"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib/images?cursor=next"},"future":42}`)
		}
		if req.URL.Query().Get("cursor") != "next" {
			t.Fatal("next link not followed")
		}
		return jsonResponse(200, `{"data":[{"type":"appAssetLibraryImages","id":"second","attributes":{"referenceName":"Second","state":"PREPARE_FOR_SUBMISSION"}}]}`)
	})
	stdout, _, err := runAssetLibrary(t, "asset-library", "images", "list", "--library-id", "lib", "--paginate", "--output", "table")
	if err != nil || calls != 2 || !strings.Contains(stdout, "First") || !strings.Contains(stdout, "Second") || !strings.Contains(stdout, "APPROVED") {
		t.Fatalf("stdout=%s calls=%d err=%v", stdout, calls, err)
	}
}

func TestAssetLibraryRejectsMalformedResponse(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(200, `{"oops":true}`) })
	stdout, _, err := runAssetLibrary(t, "asset-library", "view", "--app", "app")
	if err == nil || stdout != "" {
		t.Fatalf("malformed response accepted: %q %v", stdout, err)
	}
}

func TestAssetLibrarySpecsPreserveUnmodeledJSONAndRenderObservedDimensions(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	body := `{"data":[{"type":"appAssetLibraryRefData","id":"1","attributes":{"imageSpecs":[{"specId":"header-spec","dimensions":{"minWidth":3840,"maxWidth":3840,"minHeight":1646,"maxHeight":1646},"aspectRatio":"21:9","compatiblePlacementTypes":["PRODUCT_PAGE_HEADER_ASSET"]}],"videoSpecs":[]}}]}`
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(200, body) })
	stdout, _, err := runAssetLibrary(t, "asset-library", "specs", "--output", "table")
	if err != nil || !strings.Contains(stdout, "3840-3840 x 1646-1646") || !strings.Contains(stdout, "PRODUCT_PAGE_HEADER_ASSET") {
		t.Fatalf("observed specs not rendered: %s %v", stdout, err)
	}
	body = strings.Replace(body, `"aspectRatio":"21:9"`, `"aspectRatio":{"width":21,"height":9}`, 1)
	stdout, _, err = runAssetLibrary(t, "asset-library", "specs", "--output", "json")
	if err != nil || !strings.Contains(stdout, `"aspectRatio":{"width":21,"height":9}`) {
		t.Fatalf("unmodeled JSON lost: %s %v", stdout, err)
	}
}

func TestAssetLibrarySpecsRenderOpaqueAspectRatio(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	body := `{"data":[{"type":"appAssetLibraryRefData","id":"1","attributes":{"imageSpecs":[{"specId":"header","aspectRatio": { "width":21, "height":9 }}],"videoSpecs":[]}}]}`
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(200, body) })
	for _, format := range []string{"table", "markdown"} {
		stdout, _, err := runAssetLibrary(t, "asset-library", "specs", "--output", format)
		if err != nil || !strings.Contains(stdout, `{"width":21,"height":9}`) {
			t.Fatalf("opaque ratio not rendered (%s): %s %v", format, stdout, err)
		}
	}
}
