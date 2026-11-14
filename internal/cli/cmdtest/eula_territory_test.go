package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestEULANormalizesTerritories(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		request asctest.Matcher
		status  int
	}{
		{
			name:    "create",
			args:    []string{"eula", "create", "--app", "app-1", "--agreement-text", "Terms", "--territory", "US,France"},
			request: asctest.POST("/v1/endUserLicenseAgreements").JSONBody(`{"data":{"type":"endUserLicenseAgreements","attributes":{"agreementText":"Terms"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"territories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}]}}}}`),
			status:  http.StatusCreated,
		},
		{
			name:    "update",
			args:    []string{"eula", "update", "--id", "eula-1", "--territory", "US,France"},
			request: asctest.PATCH("/v1/endUserLicenseAgreements/eula-1").JSONBody(`{"data":{"type":"endUserLicenseAgreements","id":"eula-1","relationships":{"territories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}]}}}}`),
			status:  http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(test.request, asctest.JSON(test.status, `{"data":{"type":"endUserLicenseAgreements","id":"eula-1","attributes":{"agreementText":"Terms"}}}`))

			if _, _, err := runRootWithDeps(t, stubClient(t, reg), test.args...); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}
}
