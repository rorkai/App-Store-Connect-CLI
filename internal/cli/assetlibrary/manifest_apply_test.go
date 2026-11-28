package assetlibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestCreativeManifestApplyGroupedRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                                                                                                                                                                                                                   string
		replace, occupied, mismatch, unknownReserve, changedSlot, video, explicitPoster, journalRace, posterChanged, wrongReference, videoNull, missingPosterCode, processingPlacement, failedPlacement, rejectTransfer, table bool
		journal                                                                                                                                                                                                                string
	}{
		{name: "lost assignment response"},
		{name: "journal parser cause retained", journal: `{"schemaVersion":]`},
		{name: "null journal refused explicitly", journal: "null"},
		{name: "video default poster", video: true},
		{name: "video explicit poster", video: true, explicitPoster: true},
		{name: "video explicit null URI", video: true, explicitPoster: true, videoNull: true},
		{name: "video missing poster observation", video: true, missingPosterCode: true},
		{name: "placement processing stays partial", processingPlacement: true},
		{name: "placement failure stays partial", failedPlacement: true},
		{name: "reserved-only receipt cannot prove transferred bytes", rejectTransfer: true},
		{name: "retained video poster changed", video: true, posterChanged: true},
		{name: "concurrent journal edit", journalRace: true},
		{name: "equal metadata wrong transaction reference", wrongReference: true},
		{name: "partial table retains uploaded source", wrongReference: true, table: true},
		{name: "explicit same-media replacement", replace: true, occupied: true},
		{name: "preserve occupied slots", occupied: true},
		{name: "replacement expected asset mismatch", replace: true, occupied: true, mismatch: true},
		{name: "unknown reserve outcome", unknownReserve: true},
		{name: "slot filled after uploads", changedSlot: true},
	} {
		name := tc.name
		if runtime.GOOS == "windows" && tc.name == "lost assignment response" {
			name = "unsupported journal leaves provider unchanged"
		}
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tc.name != "lost assignment response" {
				t.Skip("identity-checked journal replacement is unavailable on Windows")
			}
			if tc.video && runtime.GOOS == "windows" {
				t.Skip("POSIX ffprobe fixture")
			}
			t.Cleanup(asc.SetMediaPollIntervalsForTest(time.Nanosecond, time.Nanosecond))
			dir := t.TempDir()
			img := image.NewRGBA(image.Rect(0, 0, 100, 50))
			draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 20, G: 80, B: 140, A: 255}}, image.Point{}, draw.Src)
			var source bytes.Buffer
			if err := png.Encode(&source, img); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "header.png"), source.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			manifest := `{"schemaVersion":1,"entries":[{"locale":"en-US","file":"header.png","placementType":"PRODUCT_PAGE_HEADER_ASSET"},{"locale":"de-DE","file":"header.png","placementType":"PRODUCT_PAGE_HEADER_ASSET"}]}`
			mediaType, resourceType, resourcePath, mediaRelationship := "IMAGE", "appAssetLibraryImages", "images", "image"
			if tc.video {
				mediaType, resourceType, resourcePath, mediaRelationship = "VIDEO", "appAssetLibraryVideos", "videos", "video"
				b, err := os.ReadFile(filepath.Join("testdata", "video.mp4"))
				if err != nil {
					t.Fatal(err)
				}
				source.Reset()
				source.Write(b)
				if err = os.WriteFile(filepath.Join(dir, "clip.mp4"), b, 0o600); err != nil {
					t.Fatal(err)
				}
				probe := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":176,"height":144,"avg_frame_rate":"30/1"}],"format":{"duration":"5","format_name":"mov,mp4,m4a,3gp,3g2,mj2","tags":{"major_brand":"isom"}}}'
`
				if err = os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				manifest = strings.ReplaceAll(manifest, "header.png", "clip.mp4")
				if tc.explicitPoster {
					manifest = strings.ReplaceAll(manifest, `"placementType":`, `"previewFrameTimeCode":"00:00:00:00","placementType":`)
				}
			}

			if tc.replace {
				var m map[string]any
				_ = json.Unmarshal([]byte(manifest), &m)
				for i, raw := range m["entries"].([]any) {
					e := raw.(map[string]any)
					loc := "en"
					if i == 1 {
						loc = "de"
					}
					e["expectedPlacementId"] = "old-loc-" + loc
					e["expectedAssetId"] = "old-asset"
					if tc.mismatch {
						e["expectedAssetId"] = "different-asset"
					}
				}
				b, _ := json.Marshal(m)
				manifest = string(b)
			}
			path := filepath.Join(dir, "creative.json")
			if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.journal != "" {
				if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte(tc.journal), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			uploads, posts := 0, 0
			var transfers atomic.Int32
			placements := map[string]string{}
			if tc.occupied {
				placements["loc-en"] = "old-loc-en"
				placements["loc-de"] = "old-loc-de"
			}
			failSecond := true
			placementState := "ACTIVE"
			if tc.processingPlacement {
				placementState = "ASSET_PROCESSING"
			}
			if tc.failedPlacement {
				placementState = "FAILED"
			}
			videoPolls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				transfers.Add(1)
				b, _ := io.ReadAll(r.Body)
				if r.Method != "PUT" || !bytes.Equal(b, source.Bytes()) {
					t.Errorf("unexpected transfer %s %d bytes", r.Method, len(b))
				}
				if tc.rejectTransfer {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			ready := fmt.Sprintf(`{"type":"appAssetLibraryImages","id":"image-1","attributes":{"category":"CREATIVE_ASSETS","fileSize":%d,"state":"PREPARE_FOR_SUBMISSION","specId":"image-spec","imageAsset":{"width":100,"height":50}}}`, source.Len())
			if tc.video {
				ready = fmt.Sprintf(`{"type":"appAssetLibraryVideos","id":"image-1","attributes":{"category":"CREATIVE_ASSETS","fileSize":%d,"state":"PREPARE_FOR_SUBMISSION","specId":"video-spec","previewFrameTimeCode":"00:00:00:00","videoAsset":"https://example.invalid/video.mp4","previewFrameImage":{"state":{"state":"COMPLETE"},"image":{"width":176,"height":144}}}}`, source.Len())
			}
			if tc.videoNull {
				ready = strings.ReplaceAll(ready, `"videoAsset":"https://example.invalid/video.mp4"`, `"videoAsset":null`)
			}
			if tc.missingPosterCode {
				ready = strings.ReplaceAll(ready, `"previewFrameTimeCode":"00:00:00:00",`, "")
			}
			uploadedReference := ""
			client := manifestTestClient(t, manifestRoundTrip(func(r *http.Request) (*http.Response, error) {
				response := ""
				switch {
				case r.Method == "GET" && r.URL.Path == "/v1/appStoreVersions/version-1":
					response = `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/appStoreVersionLocalizations"):
					response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}]}`
				case r.Method == "GET" && r.URL.Path == "/v1/apps/123456789/assetLibrary":
					response = `{"data":{"type":"appAssetLibraries","id":"library-1"}}`
				case r.Method == "GET" && r.URL.Path == "/v1/appAssetLibraryRefData":
					response = manifestPlanReference
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/placements"):
					loc := "loc-en"
					if strings.Contains(r.URL.Path, "loc-de") {
						loc = "loc-de"
					}
					response = `{"data":[]}`
					if id := placements[loc]; id != "" {
						asset := "image-1"
						if strings.HasPrefix(id, "old-") {
							asset = "old-asset"
						}
						response = fmt.Sprintf(`{"data":[{"type":"appAssetLibraryPlacements","id":%q,"attributes":{"mediaType":%q,"placementType":"PRODUCT_PAGE_HEADER_ASSET","placementGroup":"DEFAULT_PROFILE","state":%q},"relationships":{%q:{"data":{"id":%q,"type":%q}}}}]}`, id, mediaType, placementState, mediaRelationship, asset, resourceType)
					}
				case r.Method == "POST" && r.URL.Path == "/v1/"+resourceType:
					uploads++
					var reserve struct {
						Data struct {
							Attributes struct{ ReferenceName string }
						}
					}
					if err := json.NewDecoder(r.Body).Decode(&reserve); err != nil {
						t.Fatal(err)
					}
					uploadedReference = reserve.Data.Attributes.ReferenceName
					if tc.journalRace {
						journalPath := filepath.Join(dir, "receipt.json")
						b, err := os.ReadFile(journalPath)
						if err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(journalPath, append(b, ' '), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					if tc.unknownReserve {
						return nil, io.ErrUnexpectedEOF
					}
					if tc.changedSlot {
						placements["loc-de"] = "old-loc-de"
					}
					response = fmt.Sprintf(`{"data":{"type":%q,"id":"image-1","attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":%q,"offset":0,"length":%d}]}}}`, resourceType, server.URL, source.Len())
				case r.Method == "PATCH" && r.URL.Path == "/v1/"+resourceType+"/image-1":
					response = `{"data":` + ready + `}`
				case r.Method == "GET" && r.URL.Path == "/v1/"+resourceType+"/image-1":
					videoPolls++
					observed := ready
					if tc.videoNull && videoPolls > 1 {
						observed = strings.ReplaceAll(ready, `"videoAsset":null`, `"videoAsset":"https://example.invalid/video.mp4"`)
					}
					response = `{"data":` + observed + `}`
				case r.Method == "GET" && r.URL.Path == "/v1/appAssetLibraries/library-1/"+resourcePath:
					if r.URL.Query().Get("filter[id]") != "image-1" {
						t.Fatalf("unscoped reuse %s", r.URL)
					}
					var media map[string]any
					_ = json.Unmarshal([]byte(ready), &media)
					reference := uploadedReference
					if tc.wrongReference {
						reference += "-other-upload"
					}
					media["attributes"].(map[string]any)["referenceName"] = reference
					encoded, _ := json.Marshal(media)
					response = `{"data":[` + string(encoded) + `]}`
				case r.Method == "POST" && r.URL.Path == "/v1/appAssetLibraryPlacements":
					posts++
					var payload struct {
						Data struct {
							Relationships map[string]struct{ Data struct{ ID string } }
						}
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					loc := payload.Data.Relationships["appStoreVersionLocalization"].Data.ID
					previous := payload.Data.Relationships["placementToSwapOut"].Data.ID
					if tc.replace && previous != "old-"+loc {
						t.Fatalf("swap missing exact previous identity: %q", previous)
					}
					if !tc.replace && previous != "" {
						t.Fatalf("unexpected swap: %q", previous)
					}
					if payload.Data.Relationships[mediaRelationship].Data.ID != "image-1" {
						t.Fatal("wrong assigned asset")
					}
					placements[loc] = "placement-" + loc
					if loc == "loc-de" && failSecond {
						return nil, io.ErrUnexpectedEOF
					}
					response = fmt.Sprintf(`{"data":{"type":"appAssetLibraryPlacements","id":%q,"attributes":{"state":"ACTIVE"}}}`, placements[loc])
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
			}))
			attempts := 2
			if (tc.occupied && !tc.replace) || tc.mismatch || tc.changedSlot || tc.journalRace || tc.wrongReference || tc.missingPosterCode || tc.journal != "" {
				attempts = 1
			}
			for attempt := 0; attempt < attempts; attempt++ {
				d := shared.DefaultDeps()
				d.Client = func() (*asc.Client, error) { return client, nil }
				var out, stderr bytes.Buffer
				d.IO.Out = &out
				d.IO.ErrOut = &stderr
				cmd := NewCommand(d)
				cmd.FlagSet.Init("asset-library", flag.ContinueOnError)
				args := []string{"manifest", "apply", "--app", "123456789", "--version-id", "version-1", "--manifest", path, "--journal", filepath.Join(dir, "receipt.json"), "--confirm", "--output", "json"}
				if tc.table {
					args[len(args)-1] = "table"
				}
				if tc.replace {
					args = append(args, "--replace")
				}
				if err := cmd.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := cmd.Run(context.Background())
				if runtime.GOOS == "windows" {
					if !errors.Is(err, rootfs.ErrFileIdentityMutationUnsupported) || uploads != 0 || posts != 0 || transfers.Load() != 0 {
						t.Fatalf("unsupported journal err=%v uploads=%d posts=%d", err, uploads, posts)
					}
					return
				}
				if tc.journal != "" {
					if err == nil || uploads != 0 || posts != 0 || transfers.Load() != 0 {
						t.Fatalf("journal error=%v uploads=%d posts=%d", err, uploads, posts)
					}
					if tc.journal == "null" {
						if !strings.Contains(err.Error(), "expected JSON object") {
							t.Fatalf("null journal error=%v", err)
						}
					} else {
						var syntax *json.SyntaxError
						if !errors.As(err, &syntax) {
							t.Fatalf("journal parser cause lost: %v", err)
						}
					}
					continue
				}
				if tc.rejectTransfer {
					if err == nil || posts != 0 || uploads != 1 || transfers.Load() != 1 {
						t.Fatalf("unproven transfer err=%v uploads=%d transfers=%d posts=%d", err, uploads, transfers.Load(), posts)
					}
					if attempt == 1 && !strings.Contains(err.Error(), "durable transfer") {
						t.Fatalf("reserved receipt error=%v", err)
					}
					continue
				}
				if tc.missingPosterCode {
					if err == nil || !strings.Contains(err.Error(), "poster identity") || posts != 0 {
						t.Fatalf("poster identity err=%v posts=%d", err, posts)
					}
					continue
				}
				if tc.failedPlacement {
					if err == nil || !strings.Contains(err.Error(), "state FAILED") || posts != 1 {
						t.Fatalf("failed placement err=%v posts=%d", err, posts)
					}
					continue
				}
				if tc.processingPlacement && attempt == 0 && (err == nil || !strings.Contains(err.Error(), "state ASSET_PROCESSING") || posts != 1) {
					t.Fatalf("processing placement err=%v posts=%d", err, posts)
				}
				if tc.wrongReference {
					if err == nil || !strings.Contains(err.Error(), "referenceName differs") || uploads != 1 || posts != 0 {
						t.Fatalf("wrong reference err=%v uploads=%d posts=%d", err, uploads, posts)
					}
					if tc.table {
						for _, want := range []string{"header.png", "image-1", "transferred"} {
							if !strings.Contains(out.String(), want) {
								t.Fatalf("partial table omitted %q: %s", want, out.String())
							}
						}
					}
					continue
				}
				if tc.journalRace {
					if err == nil || uploads != 1 || transfers.Load() != 0 || posts != 0 || !strings.Contains(err.Error(), "journal") {
						t.Fatalf("journal race err=%v uploads=%d transfers=%d posts=%d", err, uploads, transfers.Load(), posts)
					}
					continue
				}
				if tc.posterChanged && attempt == 1 {
					if err == nil || !strings.Contains(err.Error(), "poster differs") || uploads != 1 || posts != 2 {
						t.Fatalf("poster change err=%v uploads=%d posts=%d", err, uploads, posts)
					}
					continue
				}
				if (tc.occupied && !tc.replace) || tc.mismatch {
					if err == nil || uploads != 0 || posts != 0 {
						t.Fatalf("preflight err=%v uploads=%d posts=%d", err, uploads, posts)
					}
					continue
				}
				if tc.unknownReserve {
					if err == nil || uploads != 1 || transfers.Load() != 0 || posts != 0 {
						t.Fatalf("uncertain reserve err=%v uploads=%d transfers=%d posts=%d", err, uploads, transfers.Load(), posts)
					}
					continue
				}
				if tc.changedSlot {
					if err == nil || !strings.Contains(err.Error(), "changed after planning") || posts != 1 {
						t.Fatalf("target race err=%v posts=%d", err, posts)
					}
					continue
				}
				if attempt == 0 && err == nil {
					t.Fatal("lost placement response must fail with partial receipt")
				}
				if attempt == 1 && err != nil {
					t.Fatalf("reconcile: %v stderr=%s", err, stderr.String())
				}
				var receipt map[string]any
				if e := json.Unmarshal(out.Bytes(), &receipt); e != nil {
					t.Fatalf("partial receipt missing: %v out=%s", e, out.String())
				}
				failSecond = false
				if tc.processingPlacement {
					placementState = "ACTIVE"
				}
				if tc.posterChanged {
					ready = strings.ReplaceAll(ready, "00:00:00:00", "00:00:01:00")
				}
			}
			if tc.unknownReserve || tc.changedSlot || tc.journalRace || tc.posterChanged || tc.wrongReference || tc.missingPosterCode || tc.failedPlacement || tc.rejectTransfer || tc.journal != "" || (tc.occupied && !tc.replace) || tc.mismatch {
				return
			}
			if tc.videoNull && videoPolls != 1 {
				t.Fatalf("assignment waited for download URI: %d polls", videoPolls)
			}
			if uploads != 1 || transfers.Load() != 1 || posts != 2 || len(placements) != 2 {
				t.Fatalf("uploads=%d transfers=%d posts=%d placements=%v", uploads, transfers.Load(), posts, placements)
			}
		})
	}
}

func TestCreativeManifestApplyValidationBeforeNetwork(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"invalid output", []string{"--output", "invalid"}, "output"},
		{"no confirmation", []string{"--app", "123456789", "--version-id", "v", "--manifest", "m", "--journal", "j"}, "--confirm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { t.Fatal("network setup for invalid arguments"); return nil, nil }
			var stderr bytes.Buffer
			d.IO.ErrOut = &stderr
			cmd := manifestApplyCommand(d)
			cmd.FlagSet.Init("apply", flag.ContinueOnError)
			if err := cmd.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error()+stderr.String(), tc.want) {
				t.Fatalf("err=%v stderr=%s", err, stderr.String())
			}
		})
	}
}
