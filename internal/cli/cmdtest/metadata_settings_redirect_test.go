package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestMetadataSettingsRefuseRedirectBeforeMutation(t *testing.T) {
	tests := []struct {
		name       string
		include    string
		fileName   string
		fileBody   string
		targetPath string
	}{
		{
			name:       "age rating declaration",
			include:    "age-rating",
			fileName:   "age-rating.json",
			fileBody:   `{"gambling":true}`,
			targetPath: "/v1/appInfos/appinfo-1/ageRatingDeclaration",
		},
		{
			name:       "review information detail",
			include:    "review-information",
			fileName:   "review-information.json",
			fileBody:   `{"notes":"updated"}`,
			targetPath: "/v1/appStoreVersions/version-1/appStoreReviewDetail",
		},
		{
			name:       "categories relationship",
			include:    "categories",
			fileName:   "categories.json",
			fileBody:   `{"primaryCategory":"GAMES"}`,
			targetPath: "/v1/appInfos/appinfo-1/relationships/primaryCategory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			var mu sync.Mutex
			var requests []string
			targetReached := false
			sentinelReached := false
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				switch {
				case r.URL.Path == "/fixture/redirect-sentinel":
					sentinelReached = true
					http.Error(w, "unexpected redirected read", http.StatusInternalServerError)
				case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/123456789/appInfos":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/123456789/appStoreVersions":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`))
				case r.Method == http.MethodGet && r.URL.Path == test.targetPath:
					targetReached = true
					http.Redirect(w, r, "/fixture/redirect-sentinel", http.StatusTemporaryRedirect)
				default:
					if r.Method != http.MethodGet {
						writes++
					}
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			t.Cleanup(server.Close)

			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, test.fileName), []byte(test.fileBody), 0o644); err != nil {
				t.Fatal(err)
			}

			stdout, stderr, code := runWithDeps(t, stubServerClient(t, server),
				"metadata", "apply",
				"--app", "123456789",
				"--version", "1.2.3",
				"--dir", dir,
				"--include", test.include,
				"--output", "json",
			)
			mu.Lock()
			defer mu.Unlock()
			if code != rootcmd.ExitError || stdout != "" {
				t.Fatalf("redirected %s apply exit=%d stdout=%q stderr=%q, want error and no output", test.name, code, stdout, stderr)
			}
			if !targetReached {
				t.Fatalf("did not reach the selected %s read: %v", test.name, requests)
			}
			if sentinelReached {
				t.Fatalf("redirected %s read followed the off-scope sentinel: %v", test.name, requests)
			}
			if writes != 0 {
				t.Fatalf("redirected %s read sent %d writes: %v", test.name, writes, requests)
			}
			for _, request := range requests {
				if strings.HasPrefix(request, "PATCH ") || strings.HasPrefix(request, "POST ") {
					t.Fatalf("redirected %s read sent a mutation: %v", test.name, requests)
				}
			}
		})
	}
}
