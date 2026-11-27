package cmdtest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightTesterDeviceView(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=next&fields%5BbetaTesters%5D=appDevices&filter%5Bapps%5D=app-1"
	const body = `{"data":[
		{"type":"betaTesters","id":"tester-1","attributes":{"email":"synthetic@example.invalid","firstName":"Synthetic","appDevices":[
			{"model":"iPhone17,1","platform":"IOS","osVersion":"18.0","appBuildVersion":"41"},
			{"model":"iPad16,3","platform":"IOS","osVersion":"18.1","appBuildVersion":"42"}],"futureField":null}},
		{"type":"betaTesters","id":"tester-empty","attributes":{"appDevices":[]}},
		{"type":"betaTesters","id":"tester-unknown","attributes":{}}
	],"links":{"next":"` + nextURL + `"},"meta":{"paging":{"total":4}}}`
	for _, tc := range []struct {
		name, format string
		paginate     bool
		fields       string
		standard     bool
		next         bool
	}{
		{name: "default table unchanged", format: "table", standard: true},
		{name: "table", format: "table"},
		{name: "markdown", format: "markdown"},
		{name: "json envelope", format: "json"},
		{name: "all pages", format: "table", paginate: true},
		{name: "explicit fields", format: "table", fields: "state"},
		{name: "continuation", format: "table", next: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			fields := "appDevices"
			if tc.fields != "" {
				fields = tc.fields + ",appDevices"
			}
			query := []string{"filter[apps]=app-1"}
			args := []string{"testflight", "testers", "list", "--app", "app-1", "--output", tc.format}
			if !tc.standard {
				query = append(query, "fields[betaTesters]="+fields)
				args = append(args, "--devices")
			}
			if tc.next {
				query = append(query, "cursor=next")
				args = append(args, "--next", nextURL)
			}
			if tc.paginate {
				query = append(query, "limit=200")
				args = append(args, "--paginate")
				reg.Register(asctest.GET("/v1/betaTesters", "cursor=next", "fields[betaTesters]=appDevices", "filter[apps]=app-1"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-page-two","attributes":{"appDevices":[{"model":"Mac14,2","platform":"MAC_OS","osVersion":"15.0","appBuildVersion":"43"}]}}],"links":{}}`))
			}
			if tc.fields != "" {
				args = append(args, "--fields", tc.fields)
			}
			reg.Register(asctest.GET("/v1/betaTesters", query...), asctest.JSON(http.StatusOK, body))
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil {
				t.Fatalf("device view failed: %v\n%s", err, stderr)
			}
			if tc.standard {
				if !strings.Contains(stdout, "synthetic@example.invalid") || strings.Contains(stdout, "iPhone17,1") || strings.Count(stdout, "tester-1") != 1 {
					t.Fatalf("default tester table changed:\n%s", stdout)
				}
				return
			}
			if tc.format == "json" {
				var got, want any
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, stdout)
				}
				if err := json.Unmarshal([]byte(body), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("device view changed Apple's JSON envelope: %s", stdout)
				}
				return
			}
			for _, want := range []string{"Tester ID", "Model", "Platform", "OS", "App Build", "iPhone17,1", "18.0", "41", "iPad16,3", "18.1", "42", "None reported", "Not reported"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("device view missing %q:\n%s", want, stdout)
				}
			}
			if strings.Count(stdout, "tester-1") != 2 || strings.Contains(stdout, "Synthetic") || strings.Contains(stdout, "synthetic@example.invalid") {
				t.Fatalf("expected one row per device without name/email columns:\n%s", stdout)
			}
			for model, build := range map[string]string{"iPhone17,1": "41", "iPad16,3": "42"} {
				for _, line := range strings.Split(stdout, "\n") {
					if strings.Contains(line, model) && (!strings.Contains(line, "tester-1") || !strings.Contains(line, build)) {
						t.Fatalf("device is not paired with its tester and reported build: %s", line)
					}
				}
			}
			if tc.paginate == strings.Contains(stderr, "more pages exist") {
				t.Fatalf("pagination notice disagrees with fetched pages: %s", stderr)
			}
			if tc.paginate && (!strings.Contains(stdout, "Mac14,2") || !strings.Contains(stdout, "tester-page-two") || len(reg.Requests()) != 2) {
				t.Fatalf("device view omitted next-page data:\n%s", stdout)
			}
		})
	}
}
