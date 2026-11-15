package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestScreenshotsReorder(t *testing.T) {
	const currentOrder = `{"data":[{"type":"appScreenshots","id":"a"},{"type":"appScreenshots","id":"b"},{"type":"appScreenshots","id":"c"}],"links":{}}`
	tests := []struct {
		name       string
		args       []string
		wantPatch  []string
		patchBody  string
		wantUsage  string
		wantErr    string
		wantNoCall bool
	}{
		{name: "full set in new order", args: []string{"--set-id", "set-1", "--ids", "c,a,b"}, wantPatch: []string{"c", "a", "b"}},
		{name: "set changed before PATCH", args: []string{"--set-id", "set-1", "--ids", "c,a,b"}, wantPatch: []string{"c", "a", "b"}, patchBody: `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Can't Add/Remove Relationship when reorder Set: appScreenshotSets"}]}`, wantErr: "screenshot set set-1 changed after it was read, so App Store Connect rejected the new order and left the set as it was; list it with: asc screenshots list --set-id set-1, then re-run with its current screenshot IDs"},
		{name: "other conflict", args: []string{"--set-id", "set-1", "--ids", "c,a,b"}, wantPatch: []string{"c", "a", "b"}, patchBody: `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The version is not editable in its current state."}]}`, wantErr: "screenshots reorder: The request cannot be fulfilled because of the state of another resource.: The version is not editable in its current state."},
		{name: "missing set member", args: []string{"--set-id", "set-1", "--ids", "c,a"}, wantErr: `missing from --ids: b`},
		{name: "unknown id", args: []string{"--set-id", "set-1", "--ids", "c,a,b,x"}, wantErr: `not in set set-1: x`},
		{name: "duplicate id", args: []string{"--set-id", "set-1", "--ids", "a,a,b,c"}, wantUsage: `Error: screenshots reorder: --ids contains duplicate ID "a"`, wantNoCall: true},
		{name: "empty id", args: []string{"--set-id", "set-1", "--ids", "a,,b,c"}, wantUsage: "Error: screenshots reorder: --ids contains an empty ID", wantNoCall: true},
		{name: "missing ids", args: []string{"--set-id", "set-1"}, wantUsage: "Error: screenshots reorder: --ids is required", wantNoCall: true},
		{name: "missing set id", args: []string{"--ids", "a,b,c"}, wantUsage: "Error: screenshots reorder: --set-id is required", wantNoCall: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			var patched []string
			client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.URL.Path != "/v1/appScreenshotSets/set-1/relationships/appScreenshots" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					http.Error(w, "unexpected", http.StatusNotFound)
					return
				}
				switch req.Method {
				case http.MethodGet:
					writeScreenshotsListJSON(t, w, currentOrder)
				case http.MethodPatch:
					if tt.wantPatch == nil {
						t.Errorf("unexpected PATCH")
					}
					var body struct {
						Data []struct{ Type, ID string } `json:"data"`
					}
					raw, _ := io.ReadAll(req.Body)
					if err := json.Unmarshal(raw, &body); err != nil {
						t.Errorf("decode PATCH body %s: %v", raw, err)
					}
					for _, item := range body.Data {
						if item.Type != "appScreenshots" {
							t.Errorf("PATCH item type = %q", item.Type)
						}
						patched = append(patched, item.ID)
					}
					if tt.patchBody != "" {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusConflict)
						_, _ = io.WriteString(w, tt.patchBody)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected method %s", req.Method)
				}
			})

			stdout, stderr, err := runRootWithDeps(t, client, append([]string{"screenshots", "reorder", "--output", "json"}, tt.args...)...)
			if tt.wantNoCall && calls != 0 {
				t.Fatalf("expected no requests, got %d", calls)
			}
			switch {
			case tt.wantUsage != "":
				if !isUsageClassError(err) || stdout != "" || !strings.HasPrefix(stderr, tt.wantUsage) {
					t.Fatalf("expected usage error, got err=%v stdout=%q stderr=%q", err, stdout, stderr)
				}
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || stdout != "" {
					t.Fatalf("expected error containing %q, got err=%v stdout=%q", tt.wantErr, err, stdout)
				}
				if !reflect.DeepEqual(patched, tt.wantPatch) {
					t.Fatalf("PATCH order = %v, want %v", patched, tt.wantPatch)
				}
			default:
				if err != nil {
					t.Fatalf("reorder error: %v (stderr %q)", err, stderr)
				}
				if !reflect.DeepEqual(patched, tt.wantPatch) {
					t.Fatalf("PATCH order = %v, want %v", patched, tt.wantPatch)
				}
				var receipt struct {
					SetID         string   `json:"setId"`
					Reordered     bool     `json:"reordered"`
					ScreenshotIDs []string `json:"screenshotIds"`
				}
				if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
					t.Fatalf("decode receipt %q: %v", stdout, err)
				}
				if receipt.SetID != "set-1" || !receipt.Reordered || !reflect.DeepEqual(receipt.ScreenshotIDs, tt.wantPatch) {
					t.Fatalf("receipt = %+v", receipt)
				}
			}
		})
	}
}
