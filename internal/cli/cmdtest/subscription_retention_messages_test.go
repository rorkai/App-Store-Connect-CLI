package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestRetentionMessagesRequestWiring(t *testing.T) {
	const (
		message      = `{"data":{"type":"retentionMessages","id":"msg-1","attributes":{"referenceName":"winter"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1"}}`
		messages     = `{"data":[{"type":"retentionMessages","id":"msg-1","attributes":{"referenceName":"winter"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/retentionMessages"}}`
		version      = `{"data":{"type":"retentionMessageVersions","id":"ver-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessageVersions/ver-1"}}`
		versions     = `{"data":[{"type":"retentionMessageVersions","id":"ver-1","attributes":{"state":"APPROVED"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1/versions"}}`
		localization = `{"data":{"type":"retentionMessageLocalizations","id":"loc-1","attributes":{"locale":"en-US","header":"","body":"Keep it."}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessageLocalizations/loc-1"}}`
		locales      = `{"data":[{"type":"retentionMessageLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessageVersions/ver-1/localizations"}}`
		subs         = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1/subscriptions"}}`
		placement    = `{"data":{"type":"appAssetLibraryPlacements","id":"pl-1","attributes":{"placementType":"RETENTION_MESSAGE_ASSET","state":"ACTIVE"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appAssetLibraryPlacements/pl-1"}}`
		linkages     = `{"data":[{"type":"retentionMessageVersions","id":"ver-1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1/relationships/versions"}}`
		toOne        = `{"data":{"type":"retentionMessages","id":"msg-1"},"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/relationships/retentionMessage"}}`
		unassigned   = `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionMessageVersions/ver-1/relationships/placement"}}`
	)
	tests := []struct {
		name, method, path, body, response, wantOutput string
		args                                           []string
		query                                          url.Values
	}{
		{
			name: "list", args: []string{"list", "--app", "app-1", "--include", "versions", "--retention-message-version-fields", "state", "--versions-limit", "5", "--limit", "10"},
			method: "GET", path: "/v1/apps/app-1/retentionMessages", response: messages,
			query: url.Values{"include": {"versions"}, "fields[retentionMessageVersions]": {"state"}, "limit[versions]": {"5"}, "limit": {"10"}},
		},
		{
			name: "view", args: []string{"view", "--id", "msg-1", "--fields", "referenceName"},
			method: "GET", path: "/v1/retentionMessages/msg-1", response: message, query: url.Values{"fields[retentionMessages]": {"referenceName"}},
		},
		{
			name: "create", args: []string{"create", "--app", "app-1", "--reference-name", "winter"},
			method: "POST", path: "/v1/retentionMessages", response: message,
			body: `{"data":{"type":"retentionMessages","attributes":{"referenceName":"winter"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`,
		},
		{
			name: "delete", args: []string{"delete", "--id", "msg-1", "--confirm"},
			method: "DELETE", path: "/v1/retentionMessages/msg-1", wantOutput: `{"id":"msg-1","deleted":true}`,
		},
		{
			name: "assigned", args: []string{"assigned", "--subscription-id", "sub-1", "--include", "subscriptions", "--subscription-fields", "name", "--subscriptions-limit", "2"},
			method: "GET", path: "/v1/subscriptions/sub-1/retentionMessage", response: message,
			query: url.Values{"include": {"subscriptions"}, "fields[subscriptions]": {"name"}, "limit[subscriptions]": {"2"}},
		},
		{
			name: "assigned unset", args: []string{"assigned", "--subscription-id", "sub-1"},
			method: "GET", path: "/v1/subscriptions/sub-1/retentionMessage",
			response: `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/retentionMessage"}}`,
		},
		{
			name: "assign", args: []string{"assign", "--subscription-id", "sub-1", "--message-id", "msg-1"},
			method: "PATCH", path: "/v1/subscriptions/sub-1/relationships/retentionMessage",
			body:       `{"data":{"type":"retentionMessages","id":"msg-1"}}`,
			wantOutput: `{"subscriptionId":"sub-1","retentionMessageId":"msg-1","assigned":true}`,
		},
		{
			name: "subscriptions list", args: []string{"subscriptions", "list", "--message-id", "msg-1", "--fields", "name", "--product-id", "com.example.monthly", "--name", "Monthly", "--state", "APPROVED", "--sort", "-name"},
			method: "GET", path: "/v1/retentionMessages/msg-1/subscriptions", response: subs,
			query: url.Values{"fields[subscriptions]": {"name"}, "filter[productId]": {"com.example.monthly"}, "filter[name]": {"Monthly"}, "filter[state]": {"APPROVED"}, "sort": {"-name"}},
		},
		{
			name: "versions list", args: []string{"versions", "list", "--message-id", "msg-1", "--include", "localizations", "--retention-message-localization-fields", "header", "--localizations-limit", "3"},
			method: "GET", path: "/v1/retentionMessages/msg-1/versions", response: versions,
			query: url.Values{"include": {"localizations"}, "fields[retentionMessageLocalizations]": {"header"}, "limit[localizations]": {"3"}},
		},
		{
			name: "versions view", args: []string{"versions", "view", "--id", "ver-1", "--include", "placement,retentionMessage", "--placement-fields", "state", "--retention-message-fields", "referenceName"},
			method: "GET", path: "/v1/retentionMessageVersions/ver-1", response: version,
			query: url.Values{"include": {"placement,retentionMessage"}, "fields[appAssetLibraryPlacements]": {"state"}, "fields[retentionMessages]": {"referenceName"}},
		},
		{
			name: "versions create", args: []string{"versions", "create", "--message-id", "msg-1", "--placement-id", "pl-1"},
			method: "POST", path: "/v1/retentionMessageVersions", response: version,
			body: `{"data":{"type":"retentionMessageVersions","relationships":{"retentionMessage":{"data":{"type":"retentionMessages","id":"msg-1"}},"placement":{"data":{"type":"appAssetLibraryPlacements","id":"pl-1"}}}}}`,
		},
		{
			name: "versions placement view", args: []string{"versions", "placement", "view", "--version-id", "ver-1", "--include", "image", "--asset-image-fields", "fileName", "--fields", "state"},
			method: "GET", path: "/v1/retentionMessageVersions/ver-1/placement", response: placement,
			query: url.Values{"include": {"image"}, "fields[appAssetLibraryImages]": {"fileName"}, "fields[appAssetLibraryPlacements]": {"state"}},
		},
		{
			name: "versions placement create", args: []string{"versions", "placement", "create", "--version-id", "ver-1", "--image-id", "img-1"},
			method: "POST", path: "/v1/appAssetLibraryPlacements", response: placement,
			body:       `{"data":{"type":"appAssetLibraryPlacements","attributes":{"placementType":"RETENTION_MESSAGE_ASSET","placementGroup":"DEFAULT_PROFILE"},"relationships":{"image":{"data":{"type":"appAssetLibraryImages","id":"img-1"}},"retentionMessageVersion":{"data":{"type":"retentionMessageVersions","id":"ver-1"}}}}}`,
			wantOutput: `{"id":"pl-1","created":true,"retentionMessageVersionId":"ver-1","imageId":"img-1","placementType":"RETENTION_MESSAGE_ASSET","placementGroup":"DEFAULT_PROFILE","state":"ACTIVE"}`,
		},
		{
			name: "localizations list", args: []string{"localizations", "list", "--version-id", "ver-1", "--include", "retentionMessageVersion", "--retention-message-version-fields", "state"},
			method: "GET", path: "/v1/retentionMessageVersions/ver-1/localizations", response: locales,
			query: url.Values{"include": {"retentionMessageVersion"}, "fields[retentionMessageVersions]": {"state"}},
		},
		{
			name: "localizations view", args: []string{"localizations", "view", "--id", "loc-1", "--fields", "locale,body"},
			method: "GET", path: "/v1/retentionMessageLocalizations/loc-1", response: localization, query: url.Values{"fields[retentionMessageLocalizations]": {"locale,body"}},
		},
		{
			name: "localizations create", args: []string{"localizations", "create", "--version-id", "ver-1", "--locale", "en-US", "--header", "Stay", "--body", "Keep it."},
			method: "POST", path: "/v1/retentionMessageLocalizations", response: localization,
			body: `{"data":{"type":"retentionMessageLocalizations","attributes":{"locale":"en-US","header":"Stay","body":"Keep it."},"relationships":{"retentionMessageVersion":{"data":{"type":"retentionMessageVersions","id":"ver-1"}}}}}`,
		},
		{
			name: "localizations update", args: []string{"localizations", "update", "--id", "loc-1", "--body", "Keep it.", "--clear-header"},
			method: "PATCH", path: "/v1/retentionMessageLocalizations/loc-1", response: localization,
			body: `{"data":{"type":"retentionMessageLocalizations","id":"loc-1","attributes":{"header":null,"body":"Keep it."}}}`,
		},
		{
			name: "localizations delete", args: []string{"localizations", "delete", "--id", "loc-1", "--confirm"},
			method: "DELETE", path: "/v1/retentionMessageLocalizations/loc-1", wantOutput: `{"id":"loc-1","deleted":true}`,
		},
		{name: "links messages", args: []string{"links", "messages", "--app", "app-1", "--limit", "5"}, method: "GET", path: "/v1/apps/app-1/relationships/retentionMessages", response: linkages, query: url.Values{"limit": {"5"}}},
		{name: "links versions", args: []string{"links", "versions", "--message-id", "msg-1"}, method: "GET", path: "/v1/retentionMessages/msg-1/relationships/versions", response: linkages},
		{name: "links subscriptions", args: []string{"links", "subscriptions", "--message-id", "msg-1"}, method: "GET", path: "/v1/retentionMessages/msg-1/relationships/subscriptions", response: linkages},
		{name: "links localizations", args: []string{"links", "localizations", "--version-id", "ver-1"}, method: "GET", path: "/v1/retentionMessageVersions/ver-1/relationships/localizations", response: linkages},
		{name: "links placement unset", args: []string{"links", "placement", "--version-id", "ver-1"}, method: "GET", path: "/v1/retentionMessageVersions/ver-1/relationships/placement", response: unassigned},
		{name: "links assigned", args: []string{"links", "assigned", "--subscription-id", "sub-1"}, method: "GET", path: "/v1/subscriptions/sub-1/relationships/retentionMessage", response: toOne},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			requests := 0
			useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != test.method || req.URL.Path != test.path {
					t.Errorf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.method, test.path)
				}
				if query := req.URL.Query(); len(query) != 0 || test.query != nil {
					if !reflect.DeepEqual(query, test.query) {
						t.Errorf("query = %v, want %v", query, test.query)
					}
				}
				if test.body != "" {
					assertJSONDocument(t, req.Body, test.body)
				} else if body, _ := io.ReadAll(req.Body); len(body) != 0 {
					t.Errorf("unexpected request body %s", body)
				}
				if test.response == "" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				status := http.StatusOK
				if test.method == "POST" {
					status = http.StatusCreated
				}
				writeSubscriptionVersionJSON(w, status, test.response)
			}))

			args := append(append([]string{"subscriptions", "retention-messages"}, test.args...), "--output", "json")
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
			want := test.wantOutput
			if want == "" {
				want = test.response
			}
			assertJSONDocument(t, strings.NewReader(stdout), want)
		})
	}
}

func TestRetentionMessagesVersionsListPaginates(t *testing.T) {
	setupAuth(t)
	const next = "https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1/versions?cursor=2"
	var queries []string
	useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		queries = append(queries, req.URL.RawQuery)
		if req.URL.Query().Get("cursor") == "" {
			writeSubscriptionVersionJSON(w, http.StatusOK, `{"data":[{"type":"retentionMessageVersions","id":"ver-1"}],"links":{"next":"`+next+`"}}`)
			return
		}
		writeSubscriptionVersionJSON(w, http.StatusOK, `{"data":[{"type":"retentionMessageVersions","id":"ver-2"}],"links":{}}`)
	}))

	stdout, _ := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"subscriptions", "retention-messages", "versions", "list", "--message-id", "msg-1", "--paginate", "--output", "json"}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Errorf("exit code = %d", code)
		}
	})
	if !reflect.DeepEqual(queries, []string{"limit=200", "cursor=2"}) {
		t.Fatalf("queries = %q, want first page limit=200 then the next cursor", queries)
	}
	var response asc.RetentionMessageVersionsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "ver-1" || response.Data[1].ID != "ver-2" {
		t.Fatalf("aggregated data = %+v, want ver-1 and ver-2", response.Data)
	}
}

func TestRetentionMessagesValidationBeforeClient(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	const next = "https://api.appstoreconnect.apple.com/v1/retentionMessages/msg-1/versions?cursor=2"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"list requires app", []string{"list"}, "Error: --app is required"},
		{"view requires id", []string{"view"}, "Error: --id is required"},
		{"create requires reference name", []string{"create", "--app", "app-1"}, "Error: --reference-name is required"},
		{"create limits reference name", []string{"create", "--app", "app-1", "--reference-name", strings.Repeat("é", 65)}, "--reference-name must be at most 64 characters"},
		{"delete requires confirm", []string{"delete", "--id", "msg-1"}, "Error: --confirm is required"},
		{"assign requires message", []string{"assign", "--subscription-id", "sub-1"}, "Error: --message-id is required"},
		{"versions create requires message", []string{"versions", "create"}, "Error: --message-id is required"},
		{"placement create requires image", []string{"versions", "placement", "create", "--version-id", "ver-1"}, "Error: --image-id is required"},
		{"localization create requires locale", []string{"localizations", "create", "--version-id", "ver-1"}, "Error: --locale is required"},
		{"localization create limits header", []string{"localizations", "create", "--version-id", "ver-1", "--locale", "en-US", "--header", strings.Repeat("a", 67)}, "--header must be at most 66 characters"},
		{"localization update limits body", []string{"localizations", "update", "--id", "loc-1", "--body", strings.Repeat("a", 145)}, "--body must be at most 144 characters"},
		{"localization update requires a change", []string{"localizations", "update", "--id", "loc-1"}, "Error: --header, --body, --clear-header, or --clear-body is required"},
		{"localization update rejects set and clear", []string{"localizations", "update", "--id", "loc-1", "--header", "Stay", "--clear-header"}, "--header cannot be used with --clear-header"},
		{"localization delete requires confirm", []string{"localizations", "delete", "--id", "loc-1"}, "Error: --confirm is required"},
		{"next rejects owner", []string{"versions", "list", "--message-id", "msg-2", "--next", next}, "--next cannot be combined with --message-id"},
		{"next rejects query flags", []string{"versions", "list", "--next", next, "--include", "localizations"}, "--next cannot be combined with --include"},
		{"include validated", []string{"versions", "view", "--id", "ver-1", "--include", "subscriptions"}, "--include must be one of"},
		{"included fields require include", []string{"versions", "view", "--id", "ver-1", "--placement-fields", "state"}, "--placement-fields requires --include placement"},
		{"relationship limit bounded", []string{"list", "--app", "app-1", "--versions-limit", "51"}, "--versions-limit must be between 1 and 50"},
		{"page limit bounded", []string{"links", "versions", "--message-id", "msg-1", "--limit", "201"}, "--limit must be between 1 and 200"},
		{"state filter validated", []string{"subscriptions", "list", "--message-id", "msg-1", "--state", "LIVE"}, "--state must be one of"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				t.Fatal("client factory must not run during validation")
				return nil, errors.New("unreachable")
			})
			defer restore()
			assertUsageExit(t, append([]string{"subscriptions", "retention-messages"}, test.args...), test.wantErr)
		})
	}
}
