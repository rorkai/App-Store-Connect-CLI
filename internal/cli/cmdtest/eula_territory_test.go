package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"
)

func TestEULANormalizesTerritories(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		method string
		path   string
		status int
	}{
		{
			name:   "create",
			args:   []string{"eula", "create", "--app", "app-1", "--agreement-text", "Terms", "--territory", "US,France"},
			method: http.MethodPost,
			path:   "/v1/endUserLicenseAgreements",
			status: http.StatusCreated,
		},
		{
			name:   "update",
			args:   []string{"eula", "update", "--id", "eula-1", "--territory", "US,France"},
			method: http.MethodPatch,
			path:   "/v1/endUserLicenseAgreements/eula-1",
			status: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				var payload struct {
					Data struct {
						Relationships struct {
							Territories struct {
								Data []struct {
									ID string `json:"id"`
								} `json:"data"`
							} `json:"territories"`
						} `json:"relationships"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request payload: %v", err)
				}
				var got []string
				for _, territory := range payload.Data.Relationships.Territories.Data {
					got = append(got, territory.ID)
				}
				if want := []string{"USA", "FRA"}; !slices.Equal(got, want) {
					t.Fatalf("territories = %v, want %v", got, want)
				}

				return jsonHTTPResponse(test.status, `{"data":{"type":"endUserLicenseAgreements","id":"eula-1","attributes":{"agreementText":"Terms"}}}`), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
		})
	}
}
