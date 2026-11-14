package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
			query := []string{"fields[ageRatingDeclarations]=gracRatingClassificationNumber"}
			if tt.included {
				query = append(query, "include=ageRatingDeclaration")
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.path, query...), asctest.JSON(http.StatusOK, tt.body))

			out, stderr, runErr := runCommand(t, append(tt.args, "--output", "json"))
			calls := len(reg.Requests())
			if runErr != nil || stderr != "" || calls != 1 || !strings.Contains(out, `"gracRatingClassificationNumber":"GRAC-45"`) {
				t.Fatalf("run=%v calls=%d stdout=%s stderr=%s", runErr, calls, out, stderr)
			}
		})
	}
}
