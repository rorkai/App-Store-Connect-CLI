package assetlibrary

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const manifestPlanReference = `{"data":[{"attributes":{"features":[{"featureId":"APP_STORE_VERSIONS","placementPolicies":[{"placementType":"PRODUCT_PAGE_HEADER_ASSET","groupLimits":[{"groupIds":["DEFAULT_PROFILE"],"maxCount":1}]},{"placementType":"APP_STORE_SEARCH_RESULTS_ASSET","groupLimits":[{"groupIds":["DEFAULT_PROFILE"],"maxCount":1}]}]}],"placementTypes":[{"placementTypeId":"PRODUCT_PAGE_HEADER_ASSET","specMappings":[{"placementGroupId":"DEFAULT_PROFILE","specs":["image-spec","video-spec"]}]},{"placementTypeId":"APP_STORE_SEARCH_RESULTS_ASSET","specMappings":[{"placementGroupId":"DEFAULT_PROFILE","specs":["image-spec","video-spec"]}]}],"imageSpecs":[{"specId":"image-spec","dimensions":{"minWidth":100,"maxWidth":100,"minHeight":50,"maxHeight":50},"fileExtensions":[".png"],"maxFileSize":1000000}],"videoSpecs":[{"specId":"video-spec","dimensions":{"minWidth":176,"maxWidth":176,"minHeight":144,"maxHeight":144},"compatiblePlacementTypes":["PRODUCT_PAGE_HEADER_ASSET","APP_STORE_SEARCH_RESULTS_ASSET"],"frameRates":[{"minFps":30,"maxFps":30}],"duration":{"min":"PT0S","max":"PT30S"},"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000}]}}]}`

type manifestRoundTrip func(*http.Request) (*http.Response, error)

func (f manifestRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCreativeManifestPlan(t *testing.T) {
	for _, tc := range []struct {
		name                                                                   string
		wrongOwner, missing, changed, defaultPoster, foreignNext, paged, table bool
		badCollection, wantCollectionError                                     string
	}{{name: "owned targets"}, {name: "missing placement collection", badCollection: "missing", wantCollectionError: "data array"}, {name: "null placement collection", badCollection: "null", wantCollectionError: "data array"}, {name: "repeated placement identity", badCollection: "repeat", wantCollectionError: "duplicate placement"}, {name: "table output", table: true}, {name: "different source bytes", changed: true}, {name: "default poster", defaultPoster: true}, {name: "locales on later page", paged: true}, {name: "missing locale", missing: true}, {name: "wrong owner before files", wrongOwner: true}, {name: "foreign localization cursor", foreignNext: true}} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && !tc.wrongOwner && !tc.foreignNext {
				t.Skip("POSIX ffprobe fixture")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "creative.json")
			entries := []map[string]any{{"locale": "en-US", "file": "header.png", "placementType": "PRODUCT_PAGE_HEADER_ASSET"}, {"locale": "de-DE", "file": "renamed.png", "placementType": "PRODUCT_PAGE_HEADER_ASSET"}, {"locale": "en-US", "file": "first.mp4", "placementType": "APP_STORE_SEARCH_RESULTS_ASSET", "previewFrameTimeCode": "00:00:00:00"}, {"locale": "de-DE", "file": "second.mp4", "placementType": "APP_STORE_SEARCH_RESULTS_ASSET", "previewFrameTimeCode": "00:00:01:00"}}
			if tc.defaultPoster {
				delete(entries[2], "previewFrameTimeCode")
				entries[3]["previewFrameTimeCode"] = "00:00:00:00"
			}
			body, _ := json.Marshal(map[string]any{"schemaVersion": 1, "entries": entries})
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if !tc.wrongOwner && !tc.foreignNext {
				img := image.NewRGBA(image.Rect(0, 0, 100, 50))
				draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 20, G: 80, B: 140, A: 255}}, image.Point{}, draw.Src)
				var data bytes.Buffer
				if err := png.Encode(&data, img); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"header.png", "renamed.png"} {
					if name == "renamed.png" && tc.changed {
						img.Set(0, 0, color.RGBA{R: 200, G: 80, B: 10, A: 255})
						data.Reset()
						if err := png.Encode(&data, img); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.WriteFile(filepath.Join(dir, name), data.Bytes(), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				video, err := os.ReadFile(filepath.Join("testdata", "video.mp4"))
				if err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"first.mp4", "second.mp4"} {
					if err := os.WriteFile(filepath.Join(dir, name), video, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				probe := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":176,"height":144,"avg_frame_rate":"30/1"}],"format":{"duration":"5","format_name":"mov,mp4,m4a,3gp,3g2,mj2","tags":{"major_brand":"isom"}}}'
`
				if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			reads := 0
			client := manifestTestClient(t, manifestRoundTrip(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					t.Fatalf("unexpected write %s %s", req.Method, req.URL)
				}
				reads++
				var response string
				switch req.URL.Path {
				case "/v1/appStoreVersions/version-1":
					app := "123456789"
					if tc.wrongOwner {
						app = "987654321"
					}
					response = fmt.Sprintf(`{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"%s"}}}}}`, app)
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					if tc.foreignNext {
						response = `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/other-version/appStoreVersionLocalizations?cursor=next"}}`
						break
					}
					if tc.paged {
						if req.URL.Query().Get("cursor") == "second" {
							response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}]}`
						} else {
							response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=second"}}`
						}
						break
					}
					response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}]}`
					if !tc.missing {
						response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}]}`
					}
				case "/v1/apps/123456789/assetLibrary":
					response = `{"data":{"type":"appAssetLibraries","id":"library-1"}}`
				case "/v1/appAssetLibraryRefData":
					response = manifestPlanReference
				case "/v1/appStoreVersionLocalizations/loc-en/placements":
					if req.URL.Query().Get("include") != "image,video" {
						t.Fatalf("placement read must request media linkage, got %q", req.URL.Query().Get("include"))
					}
					if req.URL.Query().Get("filter[placementGroup]") != "DEFAULT_PROFILE" {
						t.Fatalf("query=%s", req.URL.RawQuery)
					}
					response = fmt.Sprintf(`{"data":[{"type":"appAssetLibraryPlacements","id":"old-%s","attributes":{"mediaType":"IMAGE","placementType":%q,"placementGroup":"DEFAULT_PROFILE","state":"PREPARE_FOR_SUBMISSION"},"relationships":{"appStoreVersionLocalization":{"data":{"id":"loc-en","type":"appStoreVersionLocalizations"}},"image":{"data":{"id":"old-image","type":"appAssetLibraryImages"}}}}]}`, req.URL.Query().Get("filter[placementType]"), req.URL.Query().Get("filter[placementType]"))
					switch tc.badCollection {
					case "missing":
						response = `{}`
					case "null":
						response = `{"data":null}`
					case "repeat":
						var page struct{ Data []json.RawMessage }
						if err := json.Unmarshal([]byte(response), &page); err != nil {
							t.Fatal(err)
						}
						response = fmt.Sprintf(`{"data":[%s,%s]}`, page.Data[0], strings.Replace(string(page.Data[0]), "old-image", "different-image", 1))
					}
				case "/v1/appStoreVersionLocalizations/loc-de/placements":
					response = `{"data":[]}`
				default:
					t.Fatalf("unexpected read %s", req.URL)
					return nil, nil
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
			}))
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { return client, nil }
			var out, stderr bytes.Buffer
			d.IO.Out = &out
			d.IO.ErrOut = &stderr
			cmd := NewCommand(d)
			cmd.FlagSet.Init("asset-library", flag.ContinueOnError)
			format := "json"
			if tc.table {
				format = "table"
			}
			if err := cmd.Parse([]string{"manifest", "plan", "--app", "123456789", "--version-id", "version-1", "--manifest", path, "--output", format}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if tc.wantCollectionError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantCollectionError) || out.Len() != 0 {
					t.Fatalf("expected incomplete collection error %q and no plan, got %v and %d output bytes", tc.wantCollectionError, err, out.Len())
				}
				return
			}
			if tc.wrongOwner || tc.foreignNext {
				want, expectedReads := "belongs to app", 1
				if tc.foreignNext {
					want, expectedReads = "manifest pagination", 2
				}
				if err == nil || !strings.Contains(err.Error(), want) || reads != expectedReads {
					t.Fatalf("err=%v reads=%d", err, reads)
				}
				return
			}
			if err != nil {
				t.Fatalf("err=%v stderr=%s", err, stderr.String())
			}
			if tc.table {
				for _, want := range []string{"Source Key (short)", "header.png", "mediaTypeConflict", "sha256:"} {
					if !strings.Contains(out.String(), want) {
						t.Fatalf("missing %q in table: %s", want, out.String())
					}
				}
				return
			}
			var result struct {
				AppID, VersionID, LibraryID string
				ReadOnly                    bool
				RemoteReuseVerified         *bool
				UniqueSources               int
				Entries                     []struct {
					Locale, LocalizationID, ContentSHA256, SourceKey, TargetStatus, MediaType string
					ExistingPlacements                                                        []struct{ ID, MediaType string }
				}
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			expectedSources := 3
			if tc.changed {
				expectedSources++
			}
			if result.AppID != "123456789" || result.VersionID != "version-1" || result.LibraryID != "library-1" || !result.ReadOnly || (result.RemoteReuseVerified == nil || *result.RemoteReuseVerified) || result.UniqueSources != expectedSources || len(result.Entries) != 4 {
				t.Fatalf("result=%+v", result)
			}
			if (result.Entries[0].SourceKey == result.Entries[1].SourceKey) == tc.changed || result.Entries[2].ContentSHA256 != result.Entries[3].ContentSHA256 || result.Entries[2].SourceKey == result.Entries[3].SourceKey {
				t.Fatalf("source grouping=%+v", result.Entries)
			}
			if result.Entries[0].TargetStatus != "occupied" || result.Entries[2].TargetStatus != "mediaTypeConflict" || result.Entries[0].ExistingPlacements[0].ID != "old-PRODUCT_PAGE_HEADER_ASSET" {
				t.Fatalf("target status=%+v", result.Entries)
			}
			for _, i := range []int{1, 3} {
				status := "empty"
				id := "loc-de"
				if tc.missing {
					status = "missingLocalization"
					id = ""
				}
				if result.Entries[i].TargetStatus != status || result.Entries[i].LocalizationID != id {
					t.Fatalf("entry=%+v", result.Entries[i])
				}
			}
		})
	}
}

func manifestTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test-key.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY", "ISSUER", path, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestCreativeManifestValidationBeforeAPI(t *testing.T) {
	for _, tc := range []struct{ name, manifest, want string }{
		{"unknown field", `{"schemaVersion":1,"entries":[{"locale":"en-US","file":"header.png","placementType":"PRODUCT_PAGE_HEADER_ASSET","unexpected":true}]}`, `unknown field "unexpected"`},
		{"path escape", `{"schemaVersion":1,"entries":[{"locale":"en-US","file":"../header.png","placementType":"PRODUCT_PAGE_HEADER_ASSET"}]}`, "path escapes trusted root"},
		{"image poster", `{"schemaVersion":1,"entries":[{"locale":"en-US","file":"header.png","placementType":"PRODUCT_PAGE_HEADER_ASSET","previewFrameTimeCode":"00:00:00:00"}]}`, "previewFrameTimeCode requires video"},
		{"duplicate normalized slot", `{"schemaVersion":1,"entries":[{"locale":"en-US","file":"first.png","placementType":"PRODUCT_PAGE_HEADER_ASSET"},{"locale":"en_US","file":"second.png","placementType":"PRODUCT_PAGE_HEADER_ASSET"}]}`, "duplicate target slot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "creative.json")
			if err := os.WriteFile(path, []byte(tc.manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			d := shared.DefaultDeps()
			var stderr bytes.Buffer
			d.IO.ErrOut = &stderr
			d.Client = func() (*asc.Client, error) { t.Fatal("client created for invalid manifest"); return nil, nil }
			cmd := manifestPlanCommand(d)
			cmd.FlagSet.Init("plan", flag.ContinueOnError)
			if err := cmd.Parse([]string{"--app", "123456789", "--version-id", "version-1", "--manifest", path}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error()+stderr.String(), tc.want) {
				t.Fatalf("error=%v stderr=%s want=%q", err, stderr.String(), tc.want)
			}
		})
	}
}
