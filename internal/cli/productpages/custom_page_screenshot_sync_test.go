package productpages

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCustomPageScreenshotIncrementalSync(t *testing.T) {
	for _, tc := range []struct {
		name                                                                    string
		dryRun, noOp, changed, pending, failUpload, full, duplicate, failCommit bool
		wantError                                                               string
	}{
		{name: "preview", dryRun: true},
		{name: "apply"},
		{name: "unchanged", noOp: true},
		{name: "processing asset", pending: true, wantError: "incremental sync requires COMPLETE assets with source checksums"},
		{name: "inventory changed", changed: true, wantError: "screenshot set changed before incremental deletion"},
		{name: "partial upload", failUpload: true, wantError: "upload rejected"},
		{name: "partial commit", failCommit: true, wantError: "commit rejected"},
		{name: "full set", full: true},
		{name: "full set preview", full: true, dryRun: true},
		{name: "full set upload failure", full: true, failUpload: true, wantError: "upload rejected"},
		{name: "duplicate local files", duplicate: true, wantError: "decoded pixels identical"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			keep := writeCustomPageTestPNG(t, dir, "02-settings.png")
			added := writeCustomPageTestPNG(t, dir, "01-home.png")
			content, err := os.ReadFile(keep)
			if err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", md5.Sum(content))
			addedContent, err := os.ReadFile(added)
			if err != nil {
				t.Fatal(err)
			}
			addedChecksum := fmt.Sprintf("%x", md5.Sum(addedContent))
			if tc.duplicate {
				if err := os.WriteFile(added, content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ids := []string{"obsolete", "keep"}
			if tc.noOp {
				ids = []string{"new", "keep"}
			}
			if tc.full {
				for i := 0; i < 8; i++ {
					ids = append(ids, fmt.Sprintf("extra-%d", i))
				}
			}
			inventoryReads := 0
			mutations := []string{}
			transport := customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				path := req.URL.Path
				switch {
				case req.Method == "GET" && path == "/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets":
					return customPageJSONResponse(200, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
				case req.Method == "GET" && path == "/v1/appScreenshotSets/set-1/appScreenshots":
					inventoryReads++
					items := []map[string]any{}
					for _, id := range ids {
						hash := checksum
						if id == "obsolete" || strings.HasPrefix(id, "extra-") {
							hash = "00000000000000000000000000000000"
						}
						if id == "new" {
							hash = addedChecksum
						}
						if tc.changed && inventoryReads > 1 && id == "obsolete" {
							hash = "11111111111111111111111111111111"
						}
						state := "COMPLETE"
						if tc.pending && id == "obsolete" {
							state = "UPLOAD_COMPLETE"
						}
						items = append(items, map[string]any{"type": "appScreenshots", "id": id, "attributes": map[string]any{"fileName": id + ".png", "sourceFileChecksum": hash, "assetDeliveryState": map[string]string{"state": state}}})
					}
					b, _ := json.Marshal(map[string]any{"data": items})
					return customPageJSONResponse(200, string(b))
				case req.Method == "GET" && path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					data := []map[string]string{}
					for _, id := range ids {
						data = append(data, map[string]string{"type": "appScreenshots", "id": id})
					}
					b, _ := json.Marshal(map[string]any{"data": data})
					return customPageJSONResponse(200, string(b))
				case req.Method == "DELETE" && (path == "/v1/appScreenshots/obsolete" || strings.HasPrefix(path, "/v1/appScreenshots/extra-")):
					id := strings.TrimPrefix(path, "/v1/appScreenshots/")
					mutations = append(mutations, "delete "+id)
					remaining := []string{}
					for _, current := range ids {
						if current != id {
							remaining = append(remaining, current)
						}
					}
					ids = remaining
					return customPageJSONResponse(204, "")
				case req.Method == "POST" && path == "/v1/appScreenshots":
					if tc.failUpload {
						return customPageJSONResponse(422, `{"errors":[{"status":"422","code":"UPLOAD_REJECTED","title":"upload rejected"}]}`)
					}
					body, _ := io.ReadAll(req.Body)
					if !bytes.Contains(body, []byte("01-home.png")) {
						t.Fatalf("unexpected upload: %s", body)
					}
					mutations = append(mutations, "upload new")
					ids = append(ids, "new")
					return customPageJSONResponse(201, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new","length":%d,"offset":0}]}}}`, customPageFileSize(t, added)))
				case req.Method == "PUT" && req.URL.Host == "upload.example":
					return customPageJSONResponse(200, `{}`)
				case req.Method == "PATCH" && path == "/v1/appScreenshots/new":
					if tc.failCommit {
						return customPageJSONResponse(422, `{"errors":[{"status":"422","code":"COMMIT_REJECTED","title":"commit rejected"}]}`)
					}
					return customPageJSONResponse(200, `{"data":{"type":"appScreenshots","id":"new","attributes":{"uploaded":true}}}`)
				case req.Method == "GET" && path == "/v1/appScreenshots/new":
					return customPageJSONResponse(200, `{"data":{"type":"appScreenshots","id":"new","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
				case req.Method == "PATCH" && path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					var body asc.RelationshipRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					order := []string{}
					for _, r := range body.Data {
						order = append(order, r.ID)
					}
					if !reflect.DeepEqual(order, []string{"new", "keep"}) {
						t.Fatalf("order=%v", order)
					}
					mutations = append(mutations, "order")
					ids = order
					return customPageJSONResponse(204, "")
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					return nil, nil
				}
			})
			original := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = original })
			client := newCustomPageTestClientWithHTTPClient(t, &http.Client{Transport: transport})
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { return client, nil }
			var out, stderr bytes.Buffer
			d.IO.Out = &out
			d.IO.ErrOut = &stderr
			cmd := newCustomPageLocalizationsScreenshotSetsSyncCommand(d)
			cmd.FlagSet.Init("sync", flag.ContinueOnError)
			args := []string{"--localization-id", "loc-1", "--path", dir, "--device-type", "IPHONE_65", "--incremental", "--output", "json"}
			if tc.dryRun {
				args = append(args, "--dry-run")
			} else {
				args = append(args, "--confirm")
			}
			if err := cmd.Parse(args); err != nil {
				t.Fatal(err)
			}
			runErr := cmd.Run(context.Background())
			if tc.wantError != "" {
				if runErr == nil || !strings.Contains(runErr.Error(), tc.wantError) {
					t.Fatalf("error=%v, want %q", runErr, tc.wantError)
				}
				if !tc.failUpload && !tc.failCommit {
					if len(mutations) != 0 {
						t.Fatalf("unexpected mutations=%v", mutations)
					}
					return
				}
			} else if runErr != nil {
				t.Fatalf("sync: %v; stderr=%s", runErr, stderr.String())
			}
			var result asc.CustomProductPageScreenshotUploadResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			states := map[string]string{}
			for _, r := range result.Results {
				states[r.AssetID] = r.State
				if r.AssetID == "" {
					states[r.FileName] = r.State
				}
			}
			if states["keep"] != "skipped" {
				t.Fatalf("retained receipt=%v", states)
			}
			if tc.failCommit {
				if states["new"] != "UPLOADED" || states["obsolete"] != "" || !reflect.DeepEqual(mutations, []string{"upload new"}) {
					t.Fatalf("partial commit receipt=%v mutations=%v", states, mutations)
				}
			} else if tc.failUpload {
				expected := []string{}
				if tc.full {
					expected = []string{"delete obsolete"}
				}
				if (tc.full && states["obsolete"] != "deleted") || (!tc.full && states["obsolete"] != "") || !reflect.DeepEqual(mutations, expected) {
					t.Fatalf("partial receipt=%v mutations=%v", states, mutations)
				}
			} else if tc.noOp {
				if len(mutations) != 0 || states["new"] != "skipped" {
					t.Fatalf("no-op receipt=%v mutations=%v", states, mutations)
				}
			} else if tc.dryRun {
				expected := []string{"would-upload 01-home.png", "would-delete obsolete"}
				if tc.full {
					expected = []string{"would-delete obsolete", "would-upload 01-home.png"}
					for i := 0; i < 8; i++ {
						expected = append(expected, fmt.Sprintf("would-delete extra-%d", i))
					}
				}
				planned := []string{}
				for _, item := range result.Results {
					if strings.HasPrefix(item.State, "would-") {
						planned = append(planned, item.State+" "+item.FileName)
					}
				}
				for i := range planned {
					planned[i] = strings.TrimSuffix(planned[i], ".png")
				}
				for i := range expected {
					expected[i] = strings.TrimSuffix(expected[i], ".png")
				}
				if !reflect.DeepEqual(planned, expected) {
					t.Fatalf("planned order=%v want=%v", planned, expected)
				}
				if len(mutations) != 0 || states["obsolete"] != "would-delete" || states["01-home.png"] != "would-upload" {
					t.Fatalf("preview=%v mutations=%v", states, mutations)
				}
			} else {
				expected := []string{"upload new", "delete obsolete", "order"}
				if tc.full {
					expected = []string{"delete obsolete", "upload new"}
					for i := 0; i < 8; i++ {
						expected = append(expected, fmt.Sprintf("delete extra-%d", i))
					}
					expected = append(expected, "order")
				}
				if !reflect.DeepEqual(ids, []string{"new", "keep"}) || !reflect.DeepEqual(mutations, expected) {
					t.Fatalf("ids=%v mutations=%v want=%v", ids, mutations, expected)
				}
			}
		})
	}
}

func TestCustomPageScreenshotIncrementalSyncRequiresSafeFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--dry-run"}, "--dry-run requires --incremental"},
		{[]string{"--incremental"}, "--confirm"},
		{[]string{"--incremental", "--confirm", "--no-wait"}, "--incremental cannot be used with --no-wait"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			d := shared.DefaultDeps()
			var stderr bytes.Buffer
			d.IO.ErrOut = &stderr
			d.Client = func() (*asc.Client, error) { t.Fatal("unexpected client creation"); return nil, nil }
			cmd := newCustomPageLocalizationsScreenshotSetsSyncCommand(d)
			cmd.FlagSet.Init("sync", flag.ContinueOnError)
			args := append([]string{"--localization-id", "loc-1", "--path", "missing", "--device-type", "IPHONE_65"}, tc.args...)
			if err := cmd.Parse(args); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error()+stderr.String(), tc.want) {
				t.Fatalf("error=%v stderr=%q want=%q", err, stderr.String(), tc.want)
			}
		})
	}
}
