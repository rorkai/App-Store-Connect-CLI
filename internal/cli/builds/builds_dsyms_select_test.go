package builds

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestParseDSYMSelectionExactVersionDoesNotRequireLatest(t *testing.T) {
	d, _, _ := newBuildsTestDeps(nil)
	selection, err := parseDSYMSelection(d, dsymFlagInput{
		AppID:   "123",
		Version: "1.2.3",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.selectsByMarketingVersion() || selection.Resolve.Latest || selection.All || selection.Resolve.Version != "1.2.3" {
		t.Fatalf("selection = %+v, want exact version without --latest", selection)
	}
}

func TestParseDSYMSelectionVersionLatestMatchesLatestFlag(t *testing.T) {
	d, _, _ := newBuildsTestDeps(nil)
	selection, err := parseDSYMSelection(d, dsymFlagInput{
		AppID:   "123",
		Version: "latest",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.Resolve.Latest || selection.Resolve.Version != "" || selection.Live || selection.Multi {
		t.Fatalf("selection = %+v, want latest without a version filter", selection)
	}
}

func TestParseDSYMSelectionRejectsMalformedExactVersions(t *testing.T) {
	for _, version := range []string{"foo", "1..2", "-1.2"} {
		t.Run(version, func(t *testing.T) {
			d, _, _ := newBuildsTestDeps(nil)
			_, err := parseDSYMSelection(d, dsymFlagInput{AppID: "123", Version: version})
			if err == nil || !strings.Contains(err.Error(), "--version must be a dotted numeric version") {
				t.Fatalf("error = %v, want invalid exact version", err)
			}
		})
	}
}

func TestParseDSYMSelectionRejectsTimeoutWithoutWait(t *testing.T) {
	d, _, _ := newBuildsTestDeps(nil)
	_, err := parseDSYMSelection(d, dsymFlagInput{
		BuildID:    "build-1",
		Timeout:    time.Second,
		TimeoutSet: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--timeout and --poll-interval require --wait") {
		t.Fatalf("error = %v, want timeout requiring --wait", err)
	}
}

func TestChooseLiveVersionRequiresPlatformWhenSeveralAreLive(t *testing.T) {
	_, err := chooseLiveVersion([]liveAppVersion{
		{Version: "2.0", Platform: "IOS", CreatedDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{Version: "1.4", Platform: "MAC_OS", CreatedDate: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "--platform") || !strings.Contains(err.Error(), "IOS 2.0") || !strings.Contains(err.Error(), "MAC_OS 1.4") {
		t.Fatalf("error = %v, want both live platforms", err)
	}
}

func TestCompareMarketingVersions(t *testing.T) {
	got, err := compareMarketingVersions("1.2", "1.2.0")
	if err != nil || got != 0 {
		t.Fatalf("1.2 vs 1.2.0 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.10", "1.9")
	if err != nil || got <= 0 {
		t.Fatalf("1.10 vs 1.9 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.1.9", "1.2.0")
	if err != nil || got >= 0 {
		t.Fatalf("1.1.9 vs 1.2.0 = %d, %v", got, err)
	}
}

func TestSaveSingleDSYMVerifiesExistingFileContent(t *testing.T) {
	for _, remote := range []string{"mac", "ios"} {
		t.Run(remote, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "com.example.app-1.0-42.dSYM.zip")
			if err := os.WriteFile(path, []byte("ios"), 0o600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "3")
				_, _ = io.WriteString(w, remote)
			}))
			defer server.Close()
			target := dsymTarget{ID: "build", AppVersion: "1.0", BuildNumber: "42"}
			bundles := []dsymBundleInfo{{BundleID: "com.example.app", DSYMURL: &server.URL}}
			d, _, _ := newBuildsTestDeps(nil)
			files, err := saveDSYMBundles(t.Context(), d, bundles, target, dir, false)
			if remote != "ios" {
				if err == nil || !strings.Contains(err.Error(), "already exists") {
					t.Fatalf("different single-build artifact was trusted: files=%#v err=%v", files, err)
				}
				if len(files) != 0 {
					t.Fatalf("unexpected download receipt: %#v", files)
				}
			} else if err != nil || len(files) != 1 || !files[0].Skipped {
				t.Fatalf("identical existing artifact was not reused: files=%#v err=%v", files, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "ios" {
				t.Fatalf("existing file changed to %q", data)
			}
		})
	}
}

func TestResolveLiveDSYMTargetAttachmentValidation(t *testing.T) {
	for _, test := range []struct {
		name, link, attributes, wantError string
		exclude                           bool
	}{
		{"missing", `null`, ``, "no attached build", false},
		{"expired excluded", `{"type":"builds","id":"attached"}`, `,"expired":true`, "expired attached build", true},
		{"expired allowed", `{"type":"builds","id":"attached"}`, `,"expired":true`, "", false},
		{"unknown expiration", `{"type":"builds","id":"attached"}`, ``, "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/appStoreVersions/live/relationships/build":
					return buildsWaitJSONResponse(200, `{"data":`+test.link+`}`)
				case "/v1/builds/attached":
					return buildsWaitJSONResponse(200, `{"data":{"type":"builds","id":"attached","attributes":{"version":"20","uploadedDate":"2026-01-01T00:00:00Z"`+test.attributes+`}}}`)
				default:
					t.Fatalf("unexpected request: %s", req.URL.Path)
					return nil, nil
				}
			})
			d, _, _ := newBuildsTestDeps(nil)
			targets, err := resolveLiveDSYMTarget(t.Context(), d, client, liveAppVersion{ID: "live", Version: "2.0"}, test.exclude)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("targets=%v err=%v", targets, err)
				}
			} else if err != nil || len(targets) != 1 || targets[0].ID != "attached" || targets[0].AppVersion != "2.0" || targets[0].BuildNumber != "20" {
				t.Fatalf("targets=%v err=%v", targets, err)
			}
		})
	}
}

func TestResolveLiveDSYMTargetPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })
	d, _, _ := newBuildsTestDeps(nil)
	_, err := resolveLiveDSYMTarget(ctx, d, client, liveAppVersion{ID: "live"}, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveSelectedDSYMLiveRangesStillListBuilds(t *testing.T) {
	for _, after := range []*time.Time{nil, func() *time.Time { value := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); return &value }()} {
		client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/apps/123/appStoreVersions":
				return buildsWaitJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"live","attributes":{"platform":"IOS","versionString":"2.0","appStoreState":"READY_FOR_SALE","createdDate":"2026-01-01T00:00:00Z"}}]}`)
			case "/v1/builds":
				return buildsWaitJSONResponse(200, `{"data":[{"type":"builds","id":"newer","attributes":{"version":"21","uploadedDate":"2026-02-01T00:00:00Z"},"relationships":{"preReleaseVersion":{"data":{"id":"prv"}}}}],"included":[{"type":"preReleaseVersions","id":"prv","attributes":{"version":"2.0","platform":"IOS"}}]}`)
			default:
				t.Fatalf("range requested attachment: %s", req.URL.Path)
				return nil, nil
			}
		})
		d, _, _ := newBuildsTestDeps(nil)
		targets, err := resolveSelectedDSYMTargets(t.Context(), d, client, dsymSelection{Live: true, All: true, After: after, Resolve: ResolveBuildOptions{AppID: "123"}})
		if err != nil || len(targets) != 1 || targets[0].ID != "newer" {
			t.Fatalf("targets=%v err=%v", targets, err)
		}
	}
}

func TestResolveLiveDSYMTargetPreservesAPIErrors(t *testing.T) {
	failure := errors.New("provider sentinel")
	for _, failBuild := range []bool{false, true} {
		client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
			if failBuild && strings.HasSuffix(req.URL.Path, "/relationships/build") {
				return buildsWaitJSONResponse(200, `{"data":{"type":"builds","id":"attached"}}`)
			}
			return nil, failure
		})
		d, _, _ := newBuildsTestDeps(nil)
		_, err := resolveLiveDSYMTarget(t.Context(), d, client, liveAppVersion{ID: "live"}, false)
		if !errors.Is(err, failure) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestListDSYMTargetsDetectsRepeatedNextURL(t *testing.T) {
	for _, links := range [][2]string{
		{"https://api.appstoreconnect.apple.com/v1/builds?cursor=loop", "https://api.appstoreconnect.apple.com/v1/builds?cursor=loop"},
		{"https://api.appstoreconnect.apple.com/v1/builds?cursor=loop&limit=200", "/v1/builds?limit=200&cursor=loop"},
	} {
		t.Run(links[1], func(t *testing.T) {
			requests := 0
			client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
				requests++
				if requests == 2 && req.URL.RawQuery != strings.SplitN(links[0], "?", 2)[1] {
					t.Fatalf("raw next link changed: %s", req.URL.RawQuery)
				}
				if requests <= 2 {
					return buildsWaitJSONResponse(200, `{"data":[],"links":{"next":"`+links[requests-1]+`"}}`)
				}
				return buildsWaitJSONResponse(200, `{"data":[],"links":{}}`)
			})
			d, _, _ := newBuildsTestDeps(nil)
			_, err := listDSYMTargets(t.Context(), d, client, "123", "IOS", "", "", nil, false)
			if !errors.Is(err, asc.ErrRepeatedPaginationURL) || requests != 2 {
				t.Fatalf("requests=%d err=%v", requests, err)
			}
		})
	}
}

func TestListDSYMTargetsPaginationControls(t *testing.T) {
	for _, cutoff := range []bool{false, true} {
		requests := 0
		client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
			requests++
			if requests == 1 {
				return buildsWaitJSONResponse(200, `{"data":[{"type":"builds","id":"first","attributes":{"version":"20","uploadedDate":"2026-01-01T00:00:00Z"}}],"links":{"next":"/v1/builds?cursor=next"}}`)
			}
			if req.URL.RawQuery != "cursor=next" {
				t.Fatalf("query=%s", req.URL.RawQuery)
			}
			return buildsWaitJSONResponse(200, `{"data":[{"type":"builds","id":"second","attributes":{"version":"19","uploadedDate":"2025-01-01T00:00:00Z"}}]}`)
		})
		var after *time.Time
		if cutoff {
			value := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			after = &value
		}
		d, _, _ := newBuildsTestDeps(nil)
		targets, err := listDSYMTargets(t.Context(), d, client, "123", "", "", "", after, false)
		if err != nil {
			t.Fatal(err)
		}
		if cutoff {
			if requests != 1 || len(targets) != 0 {
				t.Fatalf("requests=%d targets=%v", requests, targets)
			}
		} else if requests != 2 || len(targets) != 2 || targets[0].ID != "first" || targets[1].ID != "second" {
			t.Fatalf("requests=%d targets=%v", requests, targets)
		}
	}
}

func TestListDSYMTargetsRejectsUntrustedNextURL(t *testing.T) {
	requests := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		return buildsWaitJSONResponse(200, `{"data":[],"links":{"next":"https://untrusted.example/v1/builds?cursor=next"}}`)
	})
	d, _, _ := newBuildsTestDeps(nil)
	_, err := listDSYMTargets(t.Context(), d, client, "123", "", "", "", nil, false)
	if err == nil || requests != 1 {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
}
