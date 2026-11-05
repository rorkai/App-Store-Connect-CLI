package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type subscriptionReadParameter struct {
	Name   string `json:"name"`
	Schema struct {
		Items struct {
			Enum []string `json:"enum"`
		} `json:"items"`
	} `json:"schema"`
}

func subscriptionReadParameters(t *testing.T, paths ...string) map[string][]subscriptionReadParameter {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Paths map[string]struct {
			Get struct {
				Parameters []subscriptionReadParameter `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	parameters := make(map[string][]subscriptionReadParameter, len(paths))
	for _, path := range paths {
		parameters[path] = schema.Paths[path].Get.Parameters
		if len(parameters[path]) == 0 {
			t.Fatalf("no GET parameters for %s", path)
		}
	}
	return parameters
}

func stubSubscriptionRead(t *testing.T, wantPath, body string) *url.Values {
	t.Helper()
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	got := &url.Values{}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != wantPath {
			t.Errorf("unexpected request %s %s", req.Method, req.URL)
		}
		*got = req.URL.Query()
		return jsonResponse(http.StatusOK, body)
	})
	return got
}

func TestSubscriptionReadsSendAppleSparseFieldEnums(t *testing.T) {
	groupFlags := map[string]string{
		"fields[subscriptionGroups]":             "fields",
		"fields[subscriptions]":                  "subscription-fields",
		"fields[subscriptionGroupLocalizations]": "localization-fields",
		"fields[subscriptionGroupVersions]":      "version-fields",
	}
	subscriptionFlags := map[string]string{
		"fields[subscriptions]":                         "fields",
		"fields[subscriptionVersions]":                  "version-fields",
		"fields[subscriptionLocalizations]":             "localization-fields",
		"fields[subscriptionAppStoreReviewScreenshots]": "review-screenshot-fields",
		"fields[subscriptionGroups]":                    "group-fields",
		"fields[subscriptionIntroductoryOffers]":        "introductory-offer-fields",
		"fields[subscriptionPromotionalOffers]":         "promotional-offer-fields",
		"fields[subscriptionOfferCodes]":                "offer-code-fields",
		"fields[subscriptionPrices]":                    "price-fields",
		"fields[promotedPurchases]":                     "promoted-purchase-fields",
		"fields[subscriptionAvailabilities]":            "availability-fields",
		"fields[winBackOffers]":                         "win-back-offer-fields",
		"fields[subscriptionImages]":                    "image-fields",
		"fields[subscriptionPlanAvailabilities]":        "plan-availability-fields",
		"fields[retentionOffers]":                       "retention-offer-fields",
		"fields[retentionMessages]":                     "retention-message-fields",
	}
	tests := []struct {
		name  string
		args  []string
		spec  string
		path  string
		body  string
		flags map[string]string
	}{
		{
			name: "groups list", args: []string{"subscriptions", "groups", "list", "--app", "app-1"},
			spec: "/v1/apps/{id}/subscriptionGroups", path: "/v1/apps/app-1/subscriptionGroups",
			body: `{"data":[]}`, flags: groupFlags,
		},
		{
			name: "groups view", args: []string{"subscriptions", "groups", "view", "--id", "group-1"},
			spec: "/v1/subscriptionGroups/{id}", path: "/v1/subscriptionGroups/group-1",
			body: `{"data":{"type":"subscriptionGroups","id":"group-1"}}`, flags: groupFlags,
		},
		{
			name: "subscriptions list", args: []string{"subscriptions", "list", "--group-id", "group-1"},
			spec: "/v1/subscriptionGroups/{id}/subscriptions", path: "/v1/subscriptionGroups/group-1/subscriptions",
			body: `{"data":[]}`, flags: subscriptionFlags,
		},
		{
			name: "subscriptions view", args: []string{"subscriptions", "view", "--id", "sub-1"},
			spec: "/v1/subscriptions/{id}", path: "/v1/subscriptions/sub-1",
			body: `{"data":{"type":"subscriptions","id":"sub-1"}}`, flags: subscriptionFlags,
		},
	}
	specs := make([]string, 0, len(tests))
	for _, test := range tests {
		specs = append(specs, test.spec)
	}
	parameters := subscriptionReadParameters(t, specs...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stubSubscriptionRead(t, test.path, test.body)
			// --version-fields never added its own include, so versions stays explicit.
			// retentionMessageOffers shares fields[retentionOffers] with retentionOffers.
			include := "versions"
			if test.flags["fields[retentionOffers]"] != "" {
				include += ",retentionMessageOffers"
			}
			args := append(slices.Clone(test.args), "--include", include, "--output", "json")
			want := url.Values{}
			var wantInclude []string
			for _, parameter := range parameters[test.spec] {
				switch {
				case parameter.Name == "include":
					wantInclude = slices.Sorted(slices.Values(parameter.Schema.Items.Enum))
				case strings.HasPrefix(parameter.Name, "fields["):
					name, ok := test.flags[parameter.Name]
					if !ok {
						t.Fatalf("no flag sends %s", parameter.Name)
					}
					value := strings.Join(parameter.Schema.Items.Enum, ",")
					args = append(args, "--"+name, value)
					want.Set(parameter.Name, value)
				}
			}

			stdout, stderr, err := runCreativePlacements(t, args...)
			if err != nil || stderr != "" {
				t.Fatalf("error=%v stderr=%q stdout=%q", err, stderr, stdout)
			}
			gotInclude := strings.Split(got.Get("include"), ",")
			slices.Sort(gotInclude)
			if !slices.Equal(gotInclude, wantInclude) {
				t.Fatalf("include = %v, want %v", gotInclude, wantInclude)
			}
			got.Del("include")
			if got.Encode() != want.Encode() {
				t.Fatalf("query = %s, want %s", got.Encode(), want.Encode())
			}
		})
	}
}

func TestSubscriptionReadsRelatedFieldQueries(t *testing.T) {
	const (
		groupPath  = "/v1/subscriptionGroups/group-1"
		subPath    = "/v1/subscriptions/sub-1"
		listGroups = "/v1/apps/app-1/subscriptionGroups"
		listSubs   = "/v1/subscriptionGroups/group-1/subscriptions"
		list       = `{"data":[]}`
		group      = `{"data":{"type":"subscriptionGroups","id":"group-1"}}`
		sub        = `{"data":{"type":"subscriptions","id":"sub-1"}}`
	)
	tests := []struct {
		name string
		args []string
		path string
		body string
		want string
	}{
		{
			name: "subscriptions list omitted flags keep the query",
			args: []string{"subscriptions", "list", "--group-id", "group-1", "--fields", "name", "--include", "versions"},
			path: listSubs, body: list,
			want: "fields[subscriptions]=name&include=versions",
		},
		{
			name: "subscriptions view omitted flags keep the query",
			args: []string{"subscriptions", "view", "--id", "sub-1", "--fields", "name"},
			path: subPath, body: sub,
			want: "fields[subscriptions]=name",
		},
		{
			name: "groups list omitted flags keep the query",
			args: []string{"subscriptions", "groups", "list", "--app", "app-1", "--fields", "referenceName"},
			path: listGroups, body: list,
			want: "fields[subscriptionGroups]=referenceName",
		},
		{
			name: "groups view omitted flags keep the query",
			args: []string{"subscriptions", "groups", "view", "--id", "group-1", "--fields", "referenceName"},
			path: groupPath, body: group,
			want: "fields[subscriptionGroups]=referenceName",
		},
		{
			name: "subscriptions list includes the relationship and keeps its linkage",
			args: []string{"subscriptions", "list", "--group-id", "group-1", "--fields", "name", "--price-fields", "startDate"},
			path: listSubs, body: list,
			want: "fields[subscriptionPrices]=startDate&fields[subscriptions]=name,prices&include=prices",
		},
		{
			name: "subscriptions view does not repeat an explicit include",
			args: []string{"subscriptions", "view", "--id", "sub-1", "--include", "images", "--image-fields", "fileName"},
			path: subPath, body: sub,
			want: "fields[subscriptionImages]=fileName&include=images",
		},
		{
			name: "groups list includes the relationship and keeps its linkage",
			args: []string{"subscriptions", "groups", "list", "--app", "app-1", "--fields", "referenceName", "--subscription-fields", "name"},
			path: listGroups, body: list,
			want: "fields[subscriptionGroups]=referenceName,subscriptions&fields[subscriptions]=name&include=subscriptions",
		},
		{
			name: "groups view includes the relationship",
			args: []string{"subscriptions", "groups", "view", "--id", "group-1", "--localization-fields", "state"},
			path: groupPath, body: group,
			want: "fields[subscriptionGroupLocalizations]=state&include=subscriptionGroupLocalizations",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stubSubscriptionRead(t, test.path, test.body)
			stdout, stderr, err := runCreativePlacements(t, append(test.args, "--output", "json")...)
			if err != nil || stderr != "" {
				t.Fatalf("error=%v stderr=%q stdout=%q", err, stderr, stdout)
			}
			if decoded, _ := url.QueryUnescape(got.Encode()); decoded != test.want {
				t.Fatalf("query = %s, want %s", decoded, test.want)
			}
		})
	}
}

func TestSubscriptionReadsRejectRelatedFieldFlagsBeforeAuth(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=2"
	const nextGroups = "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=2"
	subscriptionFlags := map[string]string{
		"localization-fields":       "locale",
		"review-screenshot-fields":  "fileName",
		"group-fields":              "referenceName",
		"introductory-offer-fields": "startDate",
		"promotional-offer-fields":  "name",
		"offer-code-fields":         "name",
		"price-fields":              "startDate",
		"promoted-purchase-fields":  "state",
		"availability-fields":       "availableTerritories",
		"win-back-offer-fields":     "offerId",
		"image-fields":              "fileName",
		"plan-availability-fields":  "planType",
	}
	type usageCase struct {
		args []string
		want string
	}
	tests := []usageCase{
		{[]string{"subscriptions", "groups", "list", "--next", nextGroups, "--subscription-fields", "name"}, "--next cannot be combined"},
		{[]string{"subscriptions", "groups", "list", "--next", nextGroups, "--localization-fields", "name"}, "--next cannot be combined"},
		{[]string{"subscriptions", "groups", "list", "--app", "app-1", "--localization-fields", "version"}, "--localization-fields"},
		{[]string{"subscriptions", "groups", "view", "--id", "group-1", "--subscription-fields", "nope"}, "--subscription-fields"},
		{[]string{"subscriptions", "groups", "view", "--id", "group-1", "--localization-fields", ""}, "--localization-fields"},
		{[]string{"subscriptions", "view", "--id", "sub-1", "--image-fields", "version"}, "--image-fields"},
		{[]string{"subscriptions", "view", "--id", "sub-1", "--price-fields", ""}, "--price-fields"},
	}
	for flag, value := range subscriptionFlags {
		tests = append(tests,
			usageCase{[]string{"subscriptions", "list", "--next", next, "--" + flag, value}, "--next cannot be combined with --" + flag},
			usageCase{[]string{"subscriptions", "list", "--app", "app-1", "--" + flag, value}, "require --group-id"},
		)
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			stdout, stderr, err := runCreativePlacements(t, test.args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Fatalf("expected usage error before auth: stdout=%q stderr=%q error=%v", stdout, stderr, err)
			}
		})
	}
}
