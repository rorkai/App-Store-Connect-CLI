package assets

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestGetOrderedAppScreenshotIDsScopesContinuation(t *testing.T) {
	const path = "/v1/appScreenshotSets/set-1/relationships/appScreenshots"
	tests := []struct {
		name    string
		nextURL string
		wantErr bool
	}{
		{
			name:    "same-origin absolute",
			nextURL: asc.BaseURL + path + "?cursor=next",
		},
		{
			name:    "uppercase HTTPS canonicalizes",
			nextURL: "HTTPS://API.APPSTORECONNECT.APPLE.COM" + path + "?cursor=next",
		},
		{
			name:    "root-relative",
			nextURL: path + "?cursor=next",
		},
		{
			name:    "off-collection",
			nextURL: "/v1/appScreenshotSets/other/relationships/appScreenshots?cursor=next",
			wantErr: true,
		},
		{
			name:    "wrong origin",
			nextURL: "https://evil.example" + path + "?cursor=next",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.URL.Path != path {
					t.Errorf("request path = %q, want %q", req.URL.Path, path)
				}
				if requestCount == 1 {
					writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{"next":%q}}`, test.nextURL))
					return
				}
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-2"}],"links":{}}`)
			}))

			ids, err := GetOrderedAppScreenshotIDs(asc.WithNoRedirectReads(context.Background()), client, "set-1")
			if test.wantErr {
				if err == nil {
					t.Fatal("expected scoped continuation error")
				}
				if requestCount != 1 {
					t.Fatalf("request count = %d, want 1 before refusal", requestCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetOrderedAppScreenshotIDs() error: %v", err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
			if len(ids) != 2 || ids[0] != "shot-1" || ids[1] != "shot-2" {
				t.Fatalf("ordered IDs = %#v, want [shot-1 shot-2]", ids)
			}
		})
	}
}
