package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsAgeRatings(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })

	for _, tc := range []struct {
		name, flag, id, path, response string
		query                          url.Values
		wantRow                        string
	}{
		{
			name: "list", flag: "--app", id: "app-1",
			path:    "/iris/v1/apps/app-1/appInfos",
			query:   url.Values{"include": {"ageRatingDeclaration"}},
			wantRow: "| info-1 | READY_FOR_SALE | FOUR_PLUS | declaration-1 |",
			response: `{
				"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreAgeRating":"FOUR_PLUS","state":"READY_FOR_SALE"},"relationships":{"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"declaration-1"}}}}],
				"included":[{"type":"ageRatingDeclarations","id":"declaration-1"}],
				"links":{"self":"appInfos"},
				"meta":{"paging":{"total":1,"limit":50}}
			}`,
		},
		{
			name: "territories", flag: "--app-info", id: "info-1",
			path:    "/iris/v1/appInfos/info-1/territoryAgeRatings",
			query:   url.Values{"include": {"territory"}, "limit": {"500"}},
			wantRow: "| USA | FOUR_PLUS |",
			response: `{
				"data":[{"type":"territoryAgeRatings","id":"rating-1","attributes":{"appStoreAgeRating":"FOUR_PLUS","tag":[]},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],
				"included":[{"type":"territories","id":"USA"}],
				"links":{"self":"territoryAgeRatings"},
				"meta":{"paging":{"total":1,"limit":500}}
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sessionResolutions int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil || len(body) != 0 {
					t.Errorf("expected an empty request body; body = %q, error = %v", body, err)
				}
				if req.Method != http.MethodGet || req.URL.Path != tc.path || req.URL.Query().Encode() != tc.query.Encode() {
					t.Errorf("request = %s %s, want GET %s with query %v", req.Method, req.URL, tc.path, tc.query)
					http.NotFound(w, req)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.response)
			}))
			t.Cleanup(server.Close)
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				sessionResolutions++
				return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "fixture", nil
			}

			d, stdout, stderr := newWebTestDeps()
			args := []string{"apps", "age-ratings", tc.name}
			err = NewWebCommand(d).ParseAndRun(context.Background(), append(args, tc.flag, tc.id, "--output", "json"))
			if err != nil || stderr.Len() != 0 {
				t.Fatalf("valid command: error = %v, stderr = %q", err, stderr)
			}
			var got, want any
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if err := json.Unmarshal([]byte(tc.response), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("stdout = %s, want Apple's complete response %s", stdout, tc.response)
			}

			d, stdout, stderr = newWebTestDeps()
			err = NewWebCommand(d).ParseAndRun(context.Background(), append(args, tc.flag, tc.id, "--output", "markdown"))
			if err != nil || stderr.Len() != 0 || !strings.Contains(strings.Join(strings.Fields(stdout.String()), " "), tc.wantRow) {
				t.Fatalf("markdown output = %q, want row %q; error = %v, stderr = %q", stdout, tc.wantRow, err, stderr)
			}

			d, stdout, stderr = newWebTestDeps()
			err = NewWebCommand(d).ParseAndRun(context.Background(), args)
			if shared.ClassifyUsageError(err) != shared.UsageErrorMissingRequired || !strings.HasPrefix(stderr.String(), fmt.Sprintf("Error: %s is required", tc.flag)) {
				t.Fatalf("missing %s: error = %v, stderr = %q", tc.flag, err, stderr)
			}
			if sessionResolutions != 2 || stdout.Len() != 0 {
				t.Fatalf("missing %s should be rejected before session resolution or stdout; session resolutions = %d, stdout = %q", tc.flag, sessionResolutions, stdout)
			}
		})
	}
}
