package validate

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBuildReadinessReport_VerifiesExplicitVersionBindingBeforeReadiness(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		platform  string
		wantError string
	}{
		{
			name:      "version belongs to another app",
			version:   `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}}`,
			wantError: `version "ver-1" belongs to app "app-2", not "app-1"`,
		},
		{
			name:      "version platform does not match",
			version:   `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS","versionString":"1.0"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`,
			platform:  "IOS",
			wantError: `version "ver-1" is on platform "MAC_OS", not "IOS"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			var unexpectedRequests []string
			var versionQueries []string
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/appStoreVersions/ver-1" {
					query := req.URL.Query().Encode()
					mu.Lock()
					versionQueries = append(versionQueries, query)
					mu.Unlock()
					if query == "include=app" {
						return buildsJSONResponse(http.StatusOK, test.version)
					}
				}

				mu.Lock()
				unexpectedRequests = append(unexpectedRequests, req.URL.RequestURI())
				mu.Unlock()
				return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_READINESS_REQUEST"}]}`)
			}))
			restoreClient := SetClientFactory(func() (*asc.Client, error) { return client, nil })
			t.Cleanup(restoreClient)

			_, err := BuildReadinessReport(context.Background(), ReadinessOptions{
				AppID:     "app-1",
				VersionID: "ver-1",
				Platform:  test.platform,
			})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("BuildReadinessReport() error = %v, want %q", err, test.wantError)
			}

			mu.Lock()
			defer mu.Unlock()
			if len(unexpectedRequests) != 0 {
				t.Fatalf("readiness requests started before version binding: %v", unexpectedRequests)
			}
			if len(versionQueries) != 1 || versionQueries[0] != "include=app" {
				t.Fatalf("version verification queries = %v, want [include=app]", versionQueries)
			}
		})
	}
}
