package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGRACAgeRatingSparseFields(t *testing.T) {
	const rating = `{"type":"ageRatingDeclarations","id":"rating-45","attributes":{"gracRatingClassificationNumber":"GRAC-45"}}`
	const info = `{"type":"appInfos","id":"info-45","relationships":{"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"rating-45"}}}}`
	for _, tt := range []struct {
		name       string
		args       []string
		path, body string
		included   bool
	}{
		{"direct age rating", []string{"age-rating", "view", "--app-info-id", "info-45", "--fields", "gracRatingClassificationNumber"}, "/v1/appInfos/info-45/ageRatingDeclaration", `{"data":` + rating + `}`, false},
		{"direct age rating self link", []string{"age-rating", "view", "--app-info-id", "https://api.appstoreconnect.apple.com/v1/appInfos/info-45", "--fields", "gracRatingClassificationNumber"}, "/v1/appInfos/info-45/ageRatingDeclaration", `{"data":` + rating + `}`, false},
		{"app info view", []string{"apps", "info", "view", "--info-id", "info-45", "--age-rating-fields", "gracRatingClassificationNumber"}, "/v1/appInfos/info-45", `{"data":` + info + `,"included":[` + rating + `]}`, true},
		{"app info list", []string{"apps", "info", "list", "--app", "app-45", "--age-rating-fields", "gracRatingClassificationNumber"}, "/v1/apps/app-45/appInfos", `{"data":[` + info + `],"included":[` + rating + `]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			calls := 0
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				query := r.URL.Query()
				if query.Get("fields[ageRatingDeclarations]") != "gracRatingClassificationNumber" {
					t.Errorf("fields=%v", query)
				}
				size := 1
				if tt.included {
					size = 2
					if query.Get("include") != "ageRatingDeclaration" {
						t.Errorf("include=%v", query)
					}
				}
				if len(query) != size {
					t.Errorf("unexpected query=%v", query)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			}))
			var runErr error
			args := append(tt.args, "--output", "json")
			out, stderr := captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			if runErr != nil || stderr != "" || calls != 1 || !strings.Contains(out, `"gracRatingClassificationNumber":"GRAC-45"`) {
				t.Fatalf("run=%v calls=%d stdout=%s stderr=%s", runErr, calls, out, stderr)
			}
		})
	}
}
