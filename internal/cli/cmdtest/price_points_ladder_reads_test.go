package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPricePointLadderReadsSendTerritoryFiltersIncludesAndReadEveryPageForPriceFilters(t *testing.T) {
	const page2 = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/pricePoints?cursor=Mg"
	for _, test := range []struct {
		name       string
		args       []string
		path       string
		query      []string
		nextQuery  []string
		pages      []string
		wantStdout string
	}{
		{
			name:       "iap list preserves a comma within a territory name",
			args:       []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "Moldova, Republic of"},
			path:       "/v2/inAppPurchases/9000000001/pricePoints",
			query:      []string{"filter[territory]=MDA"},
			pages:      []string{`{"data":[],"links":{}}`},
			wantStdout: `"data":[]`,
		},
		{
			name:       "app list preserves a comma within a territory name",
			args:       []string{"pricing", "price-points", "--app", "app-1", "--territory", "Moldova, Republic of"},
			path:       "/v1/apps/app-1/appPricePoints",
			query:      []string{"filter[territory]=MDA"},
			pages:      []string{`{"data":[],"links":{}}`},
			wantStdout: `"data":[]`,
		},
		{
			name:       "iap list takes several territories and includes territory",
			args:       []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "DEU,GB", "--include", "territory"},
			path:       "/v2/inAppPurchases/9000000001/pricePoints",
			query:      []string{"filter[territory]=DEU,GBR", "include=territory"},
			pages:      []string{`{"data":[],"included":[{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}],"links":{}}`},
			wantStdout: `"included":[{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}]`,
		},
		{
			name:       "app list takes several territories and includes territory",
			args:       []string{"pricing", "price-points", "--app", "app-1", "--territory", "DEU,GB", "--include", "territory"},
			path:       "/v1/apps/app-1/appPricePoints",
			query:      []string{"filter[territory]=DEU,GBR", "include=territory"},
			pages:      []string{`{"data":[],"included":[{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}}],"links":{}}`},
			wantStdout: `"id":"GBR"`,
		},
		{
			name:       "iap equalizations include territory",
			args:       []string{"iap", "pricing", "price-points", "equalizations", "--id", "pp-1", "--include", "territory"},
			path:       "/v1/inAppPurchasePricePoints/pp-1/equalizations",
			query:      []string{"include=territory"},
			pages:      []string{`{"data":[{"type":"inAppPurchasePricePoints","id":"eq-1","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{}}`},
			wantStdout: `"territory":{"data":{"type":"territories","id":"FRA"}}`,
		},
		{
			name:       "app equalizations include territory",
			args:       []string{"pricing", "price-points", "equalizations", "--price-point", "pp-1", "--include", "territory"},
			path:       "/v3/appPricePoints/pp-1/equalizations",
			query:      []string{"include=territory"},
			pages:      []string{`{"data":[{"type":"appPricePoints","id":"eq-1","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{}}`},
			wantStdout: `"id":"FRA"`,
		},
		{
			name:      "iap price filter reads past the first page without paginate",
			args:      []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "USA", "--price", "4.99"},
			path:      "/v2/inAppPurchases/9000000001/pricePoints",
			query:     []string{"filter[territory]=USA", "limit=200"},
			nextQuery: []string{"cursor=Mg"},
			pages: []string{
				`{"data":[{"type":"inAppPurchasePricePoints","id":"usa-099","attributes":{"customerPrice":"0.99"}}],"links":{"next":"` + page2 + `"}}`,
				`{"data":[{"type":"inAppPurchasePricePoints","id":"usa-499","attributes":{"customerPrice":"4.99"}}],"links":{}}`,
			},
			wantStdout: `{"data":[{"type":"inAppPurchasePricePoints","id":"usa-499","attributes":{"customerPrice":"4.99"}}]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			reg := asctest.NewRegistry(t)
			reg.Install()
			for i, body := range test.pages {
				query := test.query
				if i > 0 {
					query = test.nextQuery
				}
				reg.Register(asctest.GET(test.path, query...), asctest.JSON(http.StatusOK, body))
			}

			stdout, _, err := runRootCommand(t, append(test.args, "--output", "json"))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if requests := len(reg.Requests()); requests != len(test.pages) {
				t.Fatalf("requests = %d, want %d", requests, len(test.pages))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout, test.wantStdout)
			}
		})
	}
}
