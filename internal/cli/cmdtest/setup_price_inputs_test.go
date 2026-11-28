package cmdtest

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSetupRejectsExplicitInvalidInputsBeforeRequests(t *testing.T) {
	type setupInput struct {
		name  string
		flags []string
		want  string
	}
	commands := []struct {
		name          string
		args          []string
		territoryFlag string
		extra         []setupInput
	}{
		{"iap", []string{"iap", "setup", "--app", "APP_ID", "--type", "NON_CONSUMABLE", "--reference-name", "Pro", "--product-id", "pro"}, "--base-territory", nil},
		{"subscription", []string{"subscriptions", "setup", "--group-id", "GROUP_ID", "--reference-name", "Pro", "--product-id", "pro", "--subscription-period", "ONE_MONTH"}, "--price-territory", []setupInput{
			{"empty_group_locale", []string{"--group-locale", ""}, "--group-locale is required when group localization flags are provided"},
			{"empty_group_name", []string{"--group-display-name", ""}, "--group-locale is required when group localization flags are provided"},
		}},
	}
	inputs := []setupInput{
		{"empty_price_point", []string{"--price-point-id", ""}, "--price-point-id cannot be empty"},
		{"empty_price", []string{"--price", ""}, "--price cannot be empty"},
		{"blank_price", []string{"--price", " "}, "--price cannot be empty"},
		{"zero_tier", []string{"--tier", "0"}, "--tier must be a positive integer"},
		{"empty_locale", []string{"--locale", ""}, "--locale is required when localization flags are provided"},
		{"empty_localized_name", []string{"--display-name", ""}, "--locale is required when localization flags are provided"},
		{"empty_localized_name_alias", []string{"--name", ""}, "--locale is required when localization flags are provided"},
		{"negative_tier", []string{"--tier", "-1"}, "--tier must be a positive integer"},
	}
	for _, command := range commands {
		cases := append([]setupInput(nil), inputs...)
		cases = append(cases, command.extra...)
		cases = append(cases, setupInput{"empty_territory", []string{command.territoryFlag, ""}, command.territoryFlag + " cannot be empty"})
		for _, input := range cases {
			t.Run(command.name+"/"+input.name, func(t *testing.T) {
				requests := 0
				client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					t.Logf("unexpected request %s %s", req.Method, req.URL.Path)
					if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/GROUP_ID/subscriptions" {
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`))}, nil
					}
					return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"errors":[{"status":"400","code":"FIXTURE","title":"Unexpected request"}]}`))}, nil
				}))
				args := append(append([]string(nil), command.args...), input.flags...)
				stdout, stderr, code := runWithDeps(t, client, args...)
				if code != 2 || stdout != "" || !strings.Contains(stderr, input.want) || requests != 0 {
					t.Fatalf("want usage error %q before requests, got code=%d requests=%d stdout=%q stderr=%q", input.want, code, requests, stdout, stderr)
				}
			})
		}
	}
}
