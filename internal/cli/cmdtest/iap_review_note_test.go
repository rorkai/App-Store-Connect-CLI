package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

			attrs, err := json.Marshal(test.wantAttrs)
			if err != nil {
				t.Fatalf("marshal attributes: %v", err)
			}
			matcher := asctest.POST(test.wantPath).JSONBody(`{"data":{"type":"inAppPurchases","attributes":` + string(attrs) + `,"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
			if test.wantPath != "/v2/inAppPurchases" {
				matcher = asctest.PATCH(test.wantPath).JSONBody(`{"data":{"type":"inAppPurchases","id":"iap-1","attributes":` + string(attrs) + `}}`)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(matcher, asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro"}}}`))

			if _, _, err := runCommand(t, test.args); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}
}
