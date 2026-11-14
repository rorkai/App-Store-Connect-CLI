package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPPOTreatmentCreateRelationshipVersion(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		name := "v1"
		if v2 {
			name = "v2"
		}
		t.Run(name, func(t *testing.T) {
			setupAuth(t)
			key := "appStoreVersionExperiment"
			if v2 {
				key += "V2"
			}
			reg := asctest.NewRegistry(t)
			called := false
			reg.Register(asctest.POST("/v1/appStoreVersionExperimentTreatments").JSONBody(`{"data":{"type":"appStoreVersionExperimentTreatments","attributes":{"name":"Header"},"relationships":{"`+key+`":{"data":{"type":"appStoreVersionExperiments","id":"exp-1"}}}}}`),
				func(req *http.Request) (*http.Response, error) {
					called = true
					return asctest.JSON(201, `{"data":{"type":"appStoreVersionExperimentTreatments","id":"treatment-1","attributes":{"name":"Header"}}}`)(req)
				})
			args := []string{"product-pages", "experiments", "treatments", "create", "--experiment-id", "exp-1", "--name", "Header"}
			if v2 {
				args = append(args, "--v2")
			}
			_, _, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil || !called {
				t.Fatalf("create failed: %v called=%v", err, called)
			}
		})
	}
}
