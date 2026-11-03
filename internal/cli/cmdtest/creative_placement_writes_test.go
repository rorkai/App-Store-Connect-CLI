package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreativePlacementCreate(t *testing.T) {
	for _, kind := range []string{"PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET"} {
		t.Run(kind, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "POST" || req.URL.Path != "/v1/appAssetLibraryPlacements" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
				var body struct {
					Data struct {
						Type          string
						Attributes    map[string]string
						Relationships map[string]struct {
							Data struct {
								Type string
								ID   string
							}
						}
					}
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Data.Type != "appAssetLibraryPlacements" || len(body.Data.Attributes) != 2 || body.Data.Attributes["placementType"] != kind || body.Data.Attributes["placementGroup"] != "DEFAULT_PROFILE" || len(body.Data.Relationships) != 2 {
					t.Fatalf("unexpected payload %+v", body)
				}
				image := body.Data.Relationships["image"].Data
				loc := body.Data.Relationships["appStoreVersionLocalization"].Data
				if image.Type != "appAssetLibraryImages" || image.ID != "image" || loc.Type != "appStoreVersionLocalizations" || loc.ID != "loc" {
					t.Fatalf("unexpected relationships %+v", body)
				}
				return jsonResponse(201, `{"data":{"id":"placement","type":"appAssetLibraryPlacements","attributes":{"state":"ACTIVE","future":null}}}`)
			})
			stdout, stderr, err := runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil {
				t.Fatalf("failed %s %s %v", stdout, stderr, err)
			}
			if receipt["id"] != "placement" || receipt["created"] != true || receipt["state"] != "ACTIVE" || receipt["localizationId"] != "loc" || receipt["imageId"] != "image" || receipt["placementType"] != kind || receipt["placementGroup"] != "DEFAULT_PROFILE" {
				t.Fatalf("receipt=%v", receipt)
			}
			for _, output := range []string{"table", "markdown"} {
				stdout, stderr, err = runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind, "--output", output)
				if err != nil || stderr != "" || !strings.Contains(stdout, kind) || !strings.Contains(stdout, "ACTIVE") {
					t.Fatalf("human receipt=%s %s %v", stdout, stderr, err)
				}
			}
		})
	}
}

func TestCreativePlacementCreateRefusesFalseSuccess(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, response := range []struct {
		status int
		body   string
	}{
		{403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`},
		{201, `{"data":{}}`},
	} {
		http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(response.status, response.body) })
		stdout, _, err := runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET")
		if err == nil || stdout != "" {
			t.Fatalf("false create success status=%d output=%s error=%v", response.status, stdout, err)
		}
	}
}

func TestCreativePlacementWriteUsageBeforeAuth(t *testing.T) {
	for _, args := range [][]string{
		{"create"},
		{"create", "--localization-id", "loc", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_SCREENSHOT"},
		{"create", "--localization-id", "../loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"delete", "--id", "placement"},
		{"delete", "--confirm"},
		{"delete", "--id", "../placement", "--confirm"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := runCreativePlacements(t, append([]string{"localizations", "placements"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("expected usage before auth: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

// Read filter support does not establish creation support, so the wider list
// filter must not widen create or swap.
func TestCreativePlacementWritesRejectFilterOnlyTypes(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		return nil, nil
	})
	for _, tc := range []struct{ command, operation, kind string }{
		{"localizations", "create", "SEARCH_RESULTS_ADS_ASSET"},
		{"app-events localizations", "create", "PRODUCT_PAGE_HEADER_ASSET"},
		{"product-pages custom-pages localizations", "create", "IMESSAGE_APP_SCREENSHOT"},
		{"product-pages experiments treatments localizations", "swap", "IMESSAGE_APP_SCREENSHOT"},
	} {
		t.Run(tc.command+" "+tc.operation, func(t *testing.T) {
			args := append(strings.Fields(tc.command), "placements", tc.operation, "--localization-id", "loc", "--image-id", "image", "--placement-type", tc.kind)
			stdout, stderr, err := runCreativePlacements(t, args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--placement-type must be one of") {
				t.Fatalf("expected placement type usage error: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestCreativePlacementDeleteReceiptAndFailure(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "DELETE" || req.URL.Path != "/v1/appAssetLibraryPlacements/placement" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	for _, output := range []string{"json", "table", "markdown"} {
		stdout, stderr, err := runCreativePlacements(t, "localizations", "placements", "delete", "--id", "placement", "--confirm", "--output", output)
		if err != nil || stderr != "" || !strings.Contains(stdout, "placement") {
			t.Fatalf("output=%s %s %v", stdout, stderr, err)
		}
		if output == "json" {
			var receipt map[string]any
			if json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["deleted"] != true {
				t.Fatalf("receipt=%s", stdout)
			}
		}
	}
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`)
	})
	stdout, _, err := runCreativePlacements(t, "localizations", "placements", "delete", "--id", "placement", "--confirm")
	if err == nil || stdout != "" {
		t.Fatalf("false success %s %v", stdout, err)
	}
}
