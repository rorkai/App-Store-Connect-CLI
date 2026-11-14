package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewSubmissionsListSelectorRequests(t *testing.T) {
	tests := []struct {
		name       string
		envAppID   string
		args       []string
		wantPath   string
		wantQuery  map[string]string
		body       string
		wantStdout string
	}{
		{
			name:       "global with app",
			args:       []string{"review", "submissions-list", "--global", "--app", "app-1"},
			wantPath:   "/v1/reviewSubmissions",
			wantQuery:  map[string]string{"filter[app]": "app-1"},
			body:       `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}],"links":{"self":"/v1/reviewSubmissions"}}`,
			wantStdout: `"id":"rs-1"`,
		},
		{
			name:       "global with ASC_APP_ID",
			envAppID:   "app-from-env",
			args:       []string{"review", "submissions-list", "--global"},
			wantPath:   "/v1/reviewSubmissions",
			wantQuery:  map[string]string{"filter[app]": "app-from-env"},
			body:       `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`,
			wantStdout: `"data":[]`,
		},
		{
			name:     "global with filters",
			args:     []string{"review", "submissions-list", "--global", "--app", "app-1", "--platform", "IOS", "--state", "READY_FOR_REVIEW"},
			wantPath: "/v1/reviewSubmissions",
			wantQuery: map[string]string{
				"filter[app]":      "app-1",
				"filter[platform]": "IOS",
				"filter[state]":    "READY_FOR_REVIEW",
			},
			body: `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`,
		},
		{
			name:       "app scoped",
			args:       []string{"review", "submissions-list", "--app", "app-1"},
			wantPath:   "/v1/apps/app-1/reviewSubmissions",
			body:       `{"data":[{"type":"reviewSubmissions","id":"rs-scoped","attributes":{"platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`,
			wantStdout: `"id":"rs-scoped"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", test.envAppID)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			query := []string{}
			for key, want := range test.wantQuery {
				query = append(query, key+"="+want)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.wantPath, query...), asctest.JSON(http.StatusOK, test.body))

			stdout, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.wantStdout, stdout)
			}
		})
	}
}

func TestReviewSubmissionsItemFieldsAutomaticallyIncludeItems(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		path       string
		wantFields string
		extraQuery []string
	}{
		{
			name:       "global list",
			args:       []string{"review", "submissions-list", "--global", "--app", "app-1", "--item-fields", "subscriptionVersion"},
			path:       "/v1/reviewSubmissions",
			wantFields: "subscriptionVersion",
			extraQuery: []string{"filter[app]=app-1"},
		},
		{
			name:       "detail",
			args:       []string{"review", "submissions-get", "--id", "submission-1", "--item-fields", "subscriptionGroupVersion"},
			path:       "/v1/reviewSubmissions/submission-1",
			wantFields: "subscriptionGroupVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			body := `{"data":{"type":"reviewSubmissions","id":"submission-1"}}`
			if strings.Contains(test.name, "list") {
				body = `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.path, append([]string{"include=items", "fields[reviewSubmissionItems]=" + test.wantFields}, test.extraQuery...)...), asctest.JSON(http.StatusOK, body))

			_, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestReviewItemsVersionFieldsAutomaticallyIncludeRelatedResources(t *testing.T) {
	tests := []struct {
		name           string
		flag           string
		field          string
		itemFields     string
		include        string
		wantInclude    string
		wantQueryName  string
		wantItemFields string
	}{
		{
			name:           "in-app purchase version",
			flag:           "--iap-version-fields",
			field:          "inAppPurchase",
			itemFields:     "state",
			wantInclude:    "inAppPurchaseVersion",
			wantQueryName:  "fields[inAppPurchaseVersions]",
			wantItemFields: "state,inAppPurchaseVersion",
		},
		{
			name:           "subscription version",
			flag:           "--subscription-version-fields",
			field:          "subscription",
			itemFields:     "state",
			wantInclude:    "subscriptionVersion",
			wantQueryName:  "fields[subscriptionVersions]",
			wantItemFields: "state,subscriptionVersion",
		},
		{
			name:           "subscription group version",
			flag:           "--subscription-group-version-fields",
			field:          "subscriptionGroup",
			itemFields:     "state",
			wantInclude:    "subscriptionGroupVersion",
			wantQueryName:  "fields[subscriptionGroupVersions]",
			wantItemFields: "state,subscriptionGroupVersion",
		},
		{
			name:           "without item sparse fields",
			flag:           "--subscription-version-fields",
			field:          "version",
			wantInclude:    "subscriptionVersion",
			wantQueryName:  "fields[subscriptionVersions]",
			wantItemFields: "",
		},
		{
			name:           "preserves explicit includes without duplicates",
			flag:           "--iap-version-fields",
			field:          "version",
			itemFields:     "state",
			include:        "appStoreVersion,inAppPurchaseVersion",
			wantInclude:    "appStoreVersion,inAppPurchaseVersion",
			wantQueryName:  "fields[inAppPurchaseVersions]",
			wantItemFields: "state,inAppPurchaseVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			query := []string{"include=" + test.wantInclude, test.wantQueryName + "=" + test.field}
			if test.wantItemFields != "" {
				query = append(query, "fields[reviewSubmissionItems]="+test.wantItemFields)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/reviewSubmissions/submission-1/items", query...),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items"}}`))

			args := []string{"review", "items", "list", "--submission", "submission-1"}
			if test.itemFields != "" {
				args = append(args, "--fields", test.itemFields)
			}
			if test.include != "" {
				args = append(args, "--include", test.include)
			}
			args = append(args, test.flag, test.field)
			_, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestReviewSubmissionsListGlobalPaginate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/reviewSubmissions", "filter[app]=app-1", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS"}}],"links":{"self":"/v1/reviewSubmissions","next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=page2"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions", "cursor=page2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"rs-2","attributes":{"platform":"MAC_OS"}}],"links":{"self":"/v1/reviewSubmissions"}}`))

	stdout, stderr, err := runCommand(t, []string{"review", "submissions-list", "--global", "--app", "app-1", "--paginate"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"rs-1"`) {
		t.Fatalf("expected first page data in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"rs-2"`) {
		t.Fatalf("expected second page data in output, got %q", stdout)
	}
}
