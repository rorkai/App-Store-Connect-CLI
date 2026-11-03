package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIAPReviewNoteRequestAttributes(t *testing.T) {
	const note = "Unlock with the demo account in the review notes."

	createArgs := []string{"--app", "app-1", "--type", "CONSUMABLE", "--product-id", "com.example.pro"}
	createAttrs := map[string]any{
		"name":              "Pro",
		"productId":         "com.example.pro",
		"inAppPurchaseType": "CONSUMABLE",
	}
	withNote := func(attrs map[string]any) map[string]any {
		merged := map[string]any{"reviewNote": note}
		for key, value := range attrs {
			merged[key] = value
		}
		return merged
	}

	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantAttrs map[string]any
	}{
		{
			name:      "create sends note",
			args:      append(append([]string{"iap", "create", "--ref-name", "Pro"}, createArgs...), "--review-note", note),
			wantPath:  "/v2/inAppPurchases",
			wantAttrs: withNote(createAttrs),
		},
		{
			name:      "create omits note without flag",
			args:      append([]string{"iap", "create", "--ref-name", "Pro"}, createArgs...),
			wantPath:  "/v2/inAppPurchases",
			wantAttrs: createAttrs,
		},
		{
			name:      "update accepts note as sole flag",
			args:      []string{"iap", "update", "--id", "iap-1", "--review-note", note},
			wantPath:  "/v2/inAppPurchases/iap-1",
			wantAttrs: map[string]any{"reviewNote": note},
		},
		{
			name:      "update omits note without flag",
			args:      []string{"iap", "update", "--id", "iap-1", "--ref-name", "Pro"},
			wantPath:  "/v2/inAppPurchases/iap-1",
			wantAttrs: map[string]any{"name": "Pro"},
		},
		{
			name:      "setup sends note",
			args:      append(append([]string{"iap", "setup", "--no-verify", "--reference-name", "Pro"}, createArgs...), "--review-note", note),
			wantPath:  "/v2/inAppPurchases",
			wantAttrs: withNote(createAttrs),
		},
		{
			name:      "setup omits note without flag",
			args:      append([]string{"iap", "setup", "--no-verify", "--reference-name", "Pro"}, createArgs...),
			wantPath:  "/v2/inAppPurchases",
			wantAttrs: createAttrs,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			var gotAttrs map[string]any
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != test.wantPath {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				var payload struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				gotAttrs = payload.Data.Attributes

				body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !reflect.DeepEqual(gotAttrs, test.wantAttrs) {
				t.Fatalf("expected attributes %v, got %v", test.wantAttrs, gotAttrs)
			}
		})
	}
}
