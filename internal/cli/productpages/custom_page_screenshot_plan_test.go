package productpages

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCustomPageScreenshotPlan(t *testing.T) {
	for _, tc := range []struct {
		name, layout, state, wantError                                                                                string
		wrongOwner, ambiguous, missingLocale, symlink, foreignNext, pagedVersions, duplicate, alias, table, phoneOnly bool
	}{
		{name: "locale-first", layout: "locale", state: "PREPARE_FOR_SUBMISSION"},
		{name: "device-first rejected", layout: "device", state: "REJECTED"},
		{name: "draft in submission before local path", state: "READY_FOR_REVIEW", wantError: "exactly one editable custom page version"},
		{name: "missing locale", layout: "locale", state: "PREPARE_FOR_SUBMISSION", missingLocale: true},
		{name: "version on later page", layout: "locale", state: "REJECTED", pagedVersions: true},
		{name: "table", layout: "locale", state: "PREPARE_FOR_SUBMISSION", table: true},
		{name: "selected phone only", layout: "locale", state: "PREPARE_FOR_SUBMISSION", phoneOnly: true},
		{name: "foreign continuation", foreignNext: true, wantError: "pagination"},
		{name: "duplicate content", layout: "locale", state: "PREPARE_FOR_SUBMISSION", duplicate: true, wantError: "decoded pixels identical"},
		{name: "duplicate locale aliases", layout: "locale", state: "PREPARE_FOR_SUBMISSION", alias: true, wantError: "duplicate locale"},
		{name: "wrong app before local path", wrongOwner: true, wantError: "does not belong to app"},
		{name: "ambiguous draft before local path", state: "PREPARE_FOR_SUBMISSION", ambiguous: true, wantError: "exactly one editable custom page version"},
		{name: "approved before local path", state: "APPROVED", wantError: "exactly one editable custom page version"},
		{name: "symlink", layout: "locale", state: "PREPARE_FOR_SUBMISSION", symlink: true, wantError: "symlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "missing")
			if tc.layout != "" {
				paths := []string{"en-US/iphone/01.png", "en-US/ipad/01.png", "de-DE/iphone/01.png"}
				if tc.layout == "device" {
					paths = []string{"iphone/en-US/01.png", "ipad/en-US/01.png", "iphone/de-DE/01.png"}
				}
				for i, p := range paths {
					w, h := 1242, 2688
					if i == 1 {
						w, h = 2048, 2732
					}
					writePagePlanPNG(t, filepath.Join(root, p), w, h)
				}
				if tc.duplicate || tc.alias {
					original := filepath.Join(root, paths[0])
					data, err := os.ReadFile(original)
					if err != nil {
						t.Fatal(err)
					}
					target := filepath.Join(root, "en-US/iphone/02.png")
					if tc.alias {
						target = filepath.Join(root, "en_US/iphone/02.png")
					}
					if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if tc.symlink {
					p := filepath.Join(root, paths[0])
					if err := os.Remove(p); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Join(root, paths[2]), p); err != nil {
						t.Fatal(err)
					}
				}
			}
			reads := []string{}
			transport := customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					t.Fatalf("unexpected mutation %s %s", req.Method, req.URL)
				}
				reads = append(reads, req.URL.Path+"?"+req.URL.RawQuery)
				switch req.URL.Path {
				case "/v1/apps/123456789/appCustomProductPages":
					if req.URL.Query().Get("cursor") == "second" {
						id := "page-1"
						if tc.wrongOwner {
							id = "page-other"
						}
						return customPageJSONResponse(200, fmt.Sprintf(`{"data":[{"type":"appCustomProductPages","id":"%s"}]}`, id))
					}
					next := "https://api.appstoreconnect.apple.com/v1/apps/123456789/appCustomProductPages?cursor=second"
					if tc.foreignNext {
						next = "https://api.appstoreconnect.apple.com/v1/apps/987654321/appCustomProductPages?cursor=second"
					}
					return customPageJSONResponse(200, fmt.Sprintf(`{"data":[],"links":{"next":%q}}`, next))
				case "/v1/appCustomProductPages/page-1/appCustomProductPageVersions":
					if tc.pagedVersions && req.URL.Query().Get("cursor") == "" {
						return customPageJSONResponse(200, `{"data":[{"type":"appCustomProductPageVersions","id":"approved","attributes":{"state":"APPROVED"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appCustomProductPages/page-1/appCustomProductPageVersions?cursor=second"}}`)
					}
					data := []map[string]any{{"type": "appCustomProductPageVersions", "id": "ver-1", "attributes": map[string]string{"state": tc.state}}}
					if tc.ambiguous {
						data = append(data, map[string]any{"type": "appCustomProductPageVersions", "id": "ver-2", "attributes": map[string]string{"state": "REJECTED"}})
					}
					b, _ := json.Marshal(map[string]any{"data": data})
					return customPageJSONResponse(200, string(b))
				case "/v1/appCustomProductPageVersions/ver-1/appCustomProductPageLocalizations":
					data := []map[string]any{{"type": "appCustomProductPageLocalizations", "id": "loc-en", "attributes": map[string]string{"locale": "en-US"}}}
					if !tc.missingLocale {
						data = append(data, map[string]any{"type": "appCustomProductPageLocalizations", "id": "loc-de", "attributes": map[string]string{"locale": "de-DE"}})
					}
					b, _ := json.Marshal(map[string]any{"data": data})
					return customPageJSONResponse(200, string(b))
				default:
					t.Fatalf("unexpected read: %s", req.URL)
					return nil, nil
				}
			})
			client := newCustomPageTestClientWithHTTPClient(t, &http.Client{Transport: transport})
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { return client, nil }
			var out, stderr bytes.Buffer
			d.IO.Out = &out
			d.IO.ErrOut = &stderr
			cmd := newCustomPagesCommand(d)
			cmd.FlagSet.Init("custom-pages", flag.ContinueOnError)
			deviceTypes := "IPHONE_65,IPAD_PRO_3GEN_129"
			if tc.phoneOnly {
				deviceTypes = "IPHONE_65"
			}
			format := "json"
			if tc.table {
				format = "table"
			}
			if err := cmd.Parse([]string{"screenshots", "plan", "--app", "123456789", "--custom-page-id", "page-1", "--path", root, "--device-type", deviceTypes, "--output", format}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error()+stderr.String(), tc.wantError) {
					t.Fatalf("error=%v stderr=%s, want=%q", err, stderr.String(), tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.table {
				for _, value := range []string{"Localization ID", "en-US", "APP_IPHONE_65", "APP_IPAD_PRO_3GEN_129", "loc-en", "resolved"} {
					if !strings.Contains(out.String(), value) {
						t.Fatalf("table missing %q: %s", value, out.String())
					}
				}
				return
			}
			var result struct {
				AppID, CustomPageID, VersionID, VersionState string
				ReadOnly, Ready                              bool
				MissingLocalizations                         []string
				SkippedFiles                                 []string
				Groups                                       []struct {
					Locale, DisplayType, LocalizationID, Status string
					Files                                       []string
				}
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			groupCount := 3
			if tc.phoneOnly {
				groupCount = 2
			}
			if result.AppID != "123456789" || result.CustomPageID != "page-1" || result.VersionID != "ver-1" || result.VersionState != tc.state || !result.ReadOnly || len(result.Groups) != groupCount {
				t.Fatalf("result=%+v", result)
			}
			if result.Ready == tc.missingLocale {
				t.Fatalf("ready=%v missing=%v", result.Ready, tc.missingLocale)
			}
			wantMissing := []string{}
			if tc.missingLocale {
				wantMissing = []string{"de-DE"}
			}
			if !reflect.DeepEqual(result.MissingLocalizations, wantMissing) {
				t.Fatalf("missing=%v", result.MissingLocalizations)
			}
			if tc.phoneOnly && (!reflect.DeepEqual(result.SkippedFiles, []string{"en-US/ipad/01.png"}) || !strings.Contains(stderr.String(), "en-US/ipad/01.png")) {
				t.Fatalf("skipped=%v stderr=%s", result.SkippedFiles, stderr.String())
			}
			keys := map[string]bool{}
			for _, g := range result.Groups {
				keys[g.Locale+"/"+g.DisplayType] = true
				want := "loc-en"
				if g.Locale == "de-DE" {
					want = "loc-de"
					if tc.missingLocale {
						want = ""
					}
				}
				status := "resolved"
				if want == "" {
					status = "missingLocalization"
				}
				if g.LocalizationID != want || len(g.Files) != 1 || g.Status != status {
					t.Fatalf("group=%+v", g)
				}
			}
			for _, key := range []string{"en-US/APP_IPHONE_65", "en-US/APP_IPAD_PRO_3GEN_129", "de-DE/APP_IPHONE_65"} {
				if tc.phoneOnly && key == "en-US/APP_IPAD_PRO_3GEN_129" {
					continue
				}
				if !keys[key] {
					t.Fatalf("missing group %s: %+v", key, result.Groups)
				}
			}
			expectedReads := 4
			if tc.pagedVersions {
				expectedReads++
			}
			if len(reads) != expectedReads {
				t.Fatalf("expected paginated owner then version and locales, got %v", reads)
			}
		})
	}
}

func writePagePlanPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 20, G: 80, B: 140, A: 255}}, image.Point{}, draw.Src)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestCustomPageScreenshotPlanFlagValidation(t *testing.T) {
	for _, tc := range []struct{ app, device, want string }{
		{"app-name", "IPHONE_65", "--app requires a canonical numeric app ID"},
		{"123456789", "unknown", "unsupported screenshot display type"},
		{"123456789", "IPHONE_65,APP_IPHONE_65", "duplicate device type"},
	} {
		t.Run(tc.app+"/"+tc.device, func(t *testing.T) {
			d := shared.DefaultDeps()
			var stderr bytes.Buffer
			d.IO.ErrOut = &stderr
			d.Client = func() (*asc.Client, error) { t.Fatal("client created before validating flags"); return nil, nil }
			cmd := newCustomPageScreenshotsPlanCommand(d)
			cmd.FlagSet.Init("plan", flag.ContinueOnError)
			if err := cmd.Parse([]string{"--app", tc.app, "--custom-page-id", "page-1", "--path", "missing", "--device-type", tc.device}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error()+stderr.String(), tc.want) {
				t.Fatalf("error=%v stderr=%s want=%q", err, stderr.String(), tc.want)
			}
		})
	}
}
